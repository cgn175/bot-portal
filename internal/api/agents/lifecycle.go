package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/zeroclaw/bot-portal/internal/a2a"
	"github.com/zeroclaw/bot-portal/internal/docker"
)

// ============================================================================
// Chat types (local to avoid circular import with api package)
// ============================================================================

type chatRequest struct {
	Messages []chatMessage `json:"messages"`
	Model    string        `json:"model,omitempty"`
	Stream   bool          `json:"stream,omitempty"`
}

type chatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

func (m chatMessage) contentString() string {
	switch v := m.Content.(type) {
	case string:
		return v
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// ============================================================================
// Start / Stop / Restart / Recreate
// ============================================================================

func (h *Handler) doStartAgent(agentID string) error {
	agent, err := h.AgentStore.GetByID(agentID)
	if err != nil || agent == nil {
		return fmt.Errorf("agent not found")
	}

	// Native agents are always running
	if agent.AgentType == "native" {
		h.AgentStore.UpdateStatus(agentID, "running")
		return nil
	}

	ctx := context.Background()

	// Verify container exists in Docker (it may have been removed externally)
	if agent.ContainerID != "" && !h.DockerMgr.ContainerExists(ctx, agent.ContainerID) {
		log.Printf("Container %s for agent %s no longer exists, will recreate", agent.ContainerID, agentID)
		agent.ContainerID = ""
		if err := h.AgentStore.Update(agent); err != nil {
			return fmt.Errorf("failed to update agent after clearing container ID: %w", err)
		}
	}

	// Resolve listen port for native agents if not already set
	listenPort := agent.ListenPort
	if agent.AgentType == "native" && listenPort == 0 && agent.Endpoint != "" {
		if u, err := url.Parse(agent.Endpoint); err == nil {
			if p := u.Port(); p != "" {
				listenPort, _ = strconv.Atoi(p)
				agent.ListenPort = listenPort
				h.AgentStore.Update(agent)
			}
		}
	}

	// Create container if it doesn't exist
	if agent.ContainerID == "" {
		portalURL := os.Getenv("PORTAL_INTERNAL_URL")
		if portalURL == "" {
			portalURL = "http://host.docker.internal:8080"
		}

		// Build peer list from stored peer agent IDs
		var peerIDs []string
		if len(agent.PeerAgentIDs) > 0 {
			json.Unmarshal(agent.PeerAgentIDs, &peerIDs)
		}
		var a2aPeers []docker.A2APeer
		for _, peerID := range peerIDs {
			peerAgent, err := h.AgentStore.GetByID(peerID)
			if err != nil || peerAgent == nil {
				log.Printf("Skipping peer %s: not found", peerID)
				continue
			}
			a2aPeers = append(a2aPeers, docker.A2APeer{
				ID:          peerID,
				BearerToken: peerAgent.BearerToken,
			})
		}
		a2aPeersJSON, _ := json.Marshal(a2aPeers)

		// Build container config
		containerConfig := docker.ContainerConfig{
			AgentID:      agent.ID,
			AgentImage:   agent.Image,
			PortalURL:    portalURL,
			PortalToken:  agent.BearerToken,
			ListenPort:   listenPort,
			A2APeersJSON: string(a2aPeersJSON),
		}

		// Fetch model config if specified
		if agent.ModelID != "" {
			model, err := h.ModelStore.GetByID(agent.ModelID)
			if err != nil {
				return fmt.Errorf("failed to fetch model config: %w", err)
			}
			if model != nil {
				modelConfig := &docker.ModelConfig{
					Provider: model.Provider,
					Name:     model.ModelIdentifier,
					Endpoint: model.EndpointURL,
				}
				if model.DefaultParams != "" {
					var params map[string]interface{}
					if err := json.Unmarshal([]byte(model.DefaultParams), &params); err != nil {
						return fmt.Errorf("failed to parse model default params: %w", err)
					}
					if temp, ok := params["temperature"].(float64); ok {
						modelConfig.Temperature = &temp
					}
					if maxTokensFloat, ok := params["max_tokens"].(float64); ok {
						maxTokens := int(maxTokensFloat)
						modelConfig.MaxTokens = &maxTokens
					}
				}
				containerConfig.ModelConfig = modelConfig
			}
		}

		// Fetch auth config if specified
		if agent.AuthConfigID != "" {
			auth, err := h.AuthConfigStore.GetByID(agent.AuthConfigID)
			if err != nil {
				return fmt.Errorf("failed to fetch auth config: %w", err)
			}
			if auth != nil {
				authConfig := &docker.AuthConfig{
					Type:     auth.AuthType,
					Endpoint: auth.EndpointURL,
				}
				if auth.Credentials != "" {
					var creds map[string]string
					if err := json.Unmarshal([]byte(auth.Credentials), &creds); err != nil {
						return fmt.Errorf("failed to parse auth credentials: %w", err)
					}
					if apiKey, ok := creds["api_key"]; ok {
						authConfig.ApiKey = apiKey
					}
					if auth.AuthType == "github_copilot_oauth" {
						if accessToken, ok := creds["access_token"]; ok {
							authConfig.ApiKey = accessToken
						}
					}
				}
				containerConfig.AuthConfig = authConfig
			}
		}

		containerID, err := h.DockerMgr.CreateContainer(ctx, containerConfig)
		if err != nil {
			return fmt.Errorf("failed to create container: %w", err)
		}

		agent.ContainerID = containerID
		h.AgentStore.Update(agent)
	}

	// Start container
	if err := h.DockerMgr.StartContainer(ctx, agent.ContainerID); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Auto-inject AGENTS.md with A2A instructions if not already set
	h.injectDefaultAgentsMD(ctx, agent)

	h.AgentStore.UpdateStatus(agentID, "running")
	return nil
}

func (h *Handler) startAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	if err := h.doStartAgent(agentID); err != nil {
		if err.Error() == "agent not found" {
			http.Error(w, "Agent not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (h *Handler) doStopAgent(agentID string) {
	agent, err := h.AgentStore.GetByID(agentID)
	if err != nil || agent == nil {
		h.AgentStore.UpdateStatus(agentID, "stopped")
		return
	}

	if agent.AgentType == "native" {
		return
	}

	if agent.ContainerID == "" {
		h.AgentStore.UpdateStatus(agentID, "stopped")
		return
	}

	ctx := context.Background()
	h.DockerMgr.StopContainer(ctx, agent.ContainerID)
	h.AgentStore.UpdateStatus(agentID, "stopped")
}

func (h *Handler) stopAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	agent, err := h.AgentStore.GetByID(agentID)
	if err != nil || agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	if agent.AgentType == "native" {
		http.Error(w, "Cannot stop native agents", http.StatusBadRequest)
		return
	}

	h.doStopAgent(agentID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func (h *Handler) restartAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	agent, err := h.AgentStore.GetByID(agentID)
	if err != nil || agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	if agent.AgentType == "native" {
		http.Error(w, "Cannot restart native agents", http.StatusBadRequest)
		return
	}

	if agent.ContainerID == "" {
		http.Error(w, "No container to restart", http.StatusBadRequest)
		return
	}

	h.AgentStore.UpdateStatus(agentID, "restarting")

	containerID := agent.ContainerID
	go func(cid, aid string) {
		ctx := context.Background()
		if err := h.DockerMgr.RestartContainer(ctx, cid); err != nil {
			log.Printf("Failed to restart container %s: %v", cid, err)
			h.AgentStore.UpdateStatus(aid, "stopped")
			return
		}
		h.AgentStore.UpdateStatus(aid, "running")
	}(containerID, agentID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "restarting"})
}

func (h *Handler) recreateAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	agent, err := h.AgentStore.GetByID(agentID)
	if err != nil || agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	if agent.AgentType == "native" {
		http.Error(w, "Cannot recreate native agents", http.StatusBadRequest)
		return
	}

	if agent.ContainerID == "" {
		http.Error(w, "No container to recreate", http.StatusBadRequest)
		return
	}

	h.AgentStore.UpdateStatus(agentID, "recreating")

	containerID := agent.ContainerID
	go func(cid, aid string) {
		ctx := context.Background()
		if err := h.DockerMgr.RemoveContainer(ctx, cid); err != nil {
			log.Printf("Failed to remove container %s: %v", cid, err)
			h.AgentStore.UpdateStatus(aid, "stopped")
			return
		}

		agent, err := h.AgentStore.GetByID(aid)
		if err != nil || agent == nil {
			log.Printf("Failed to get agent %s after removing container: %v", aid, err)
			h.AgentStore.UpdateStatus(aid, "stopped")
			return
		}
		agent.ContainerID = ""
		if err := h.AgentStore.Update(agent); err != nil {
			log.Printf("Failed to update agent %s after clearing container ID: %v", aid, err)
			h.AgentStore.UpdateStatus(aid, "stopped")
			return
		}

		if err := h.doStartAgent(aid); err != nil {
			log.Printf("Failed to start agent %s after recreating: %v", aid, err)
			h.AgentStore.UpdateStatus(aid, "stopped")
			return
		}
	}(containerID, agentID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "recreating"})
}

// ============================================================================
// Ping / Chat / Logs
// ============================================================================

func (h *Handler) pingAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	agent, err := h.AgentStore.GetByID(agentID)
	if err != nil || agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	endpoint := agent.Endpoint
	if endpoint == "" {
		http.Error(w, "Agent has no endpoint", http.StatusBadRequest)
		return
	}

	client := http.Client{Timeout: 5 * time.Second}

	if endpoint[len(endpoint)-1] != '/' {
		endpoint += "/"
	}

	resp, err := client.Get(endpoint + ".well-known/agent.json")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"online": false,
			"error":  err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	online := resp.StatusCode == http.StatusOK
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"online": online,
		"status": resp.StatusCode,
	})
}

func (h *Handler) handleAgentChat(w http.ResponseWriter, req *http.Request, agentID string) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agent, err := h.AgentStore.GetByID(agentID)
	if err != nil || agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	var chatReq chatRequest
	if err := json.NewDecoder(req.Body).Decode(&chatReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(chatReq.Messages) == 0 {
		http.Error(w, "Messages cannot be empty", http.StatusBadRequest)
		return
	}

	lastMsg := chatReq.Messages[len(chatReq.Messages)-1]

	taskURL := fmt.Sprintf("%s/tasks", agent.Endpoint)
	createReq := a2a.CreateTaskRequest{
		Message: a2a.TaskMessage{
			Role:      lastMsg.Role,
			Content:   lastMsg.contentString(),
			Timestamp: time.Now(),
		},
	}
	body, _ := json.Marshal(createReq)

	httpReq, _ := http.NewRequest("POST", taskURL, bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Bearer "+agent.BearerToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Agent-ID", "portal")
	httpReq.Header.Set("X-Channel-ID", a2a.ChannelID("portal", agent.ID))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("Failed to forward chat task to agent %s: %v", agent.ID, err)
		http.Error(w, fmt.Sprintf("Failed to reach agent: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("Agent %s returned error: %s", agent.ID, string(respBody))
		http.Error(w, fmt.Sprintf("Agent returned error: %s", string(respBody)), resp.StatusCode)
		return
	}

	var agentResp struct {
		Task struct {
			ID string `json:"id"`
		} `json:"task"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&agentResp); err != nil || agentResp.Task.ID == "" {
		log.Printf("Failed to parse task ID from agent %s response: %v", agent.ID, err)
		http.Error(w, "Failed to parse agent response", http.StatusInternalServerError)
		return
	}

	taskID := agentResp.Task.ID
	log.Printf("Chat task %s created by agent %s", taskID, agent.ID)

	_, err = h.CreateTaskWithID(taskID, a2a.ChannelID("portal", agentID), "portal", agentID, a2a.TaskMessage{
		Role:      lastMsg.Role,
		Content:   lastMsg.contentString(),
		Timestamp: time.Now(),
	})
	if err != nil {
		log.Printf("Failed to save task %s locally: %v", taskID, err)
		http.Error(w, fmt.Sprintf("Failed to save task: %v", err), http.StatusInternalServerError)
		return
	}

	go h.SubscribeToAgentSSE(agent, taskID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"taskId": taskID})
}

func (h *Handler) streamAgentLogs(w http.ResponseWriter, req *http.Request, agentID string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var flusher http.Flusher
	if f, ok := w.(http.Flusher); ok {
		flusher = f
	}

	fmt.Fprintf(w, "data: Log streaming not yet implemented\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}
