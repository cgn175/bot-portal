package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/zeroclaw/bot-portal/internal/a2a"
	"github.com/zeroclaw/bot-portal/internal/docker"
	"github.com/zeroclaw/bot-portal/internal/models"
	"github.com/zeroclaw/bot-portal/internal/store"
)

// ============================================================================
// Types
// ============================================================================

// AgentIdentityFilesResponse represents the response for listing agent identity files
type AgentIdentityFilesResponse struct {
	Files []AgentIdentityFileInfo `json:"files"`
}

// AgentIdentityFileInfo represents metadata for an identity file (without content)
type AgentIdentityFileInfo struct {
	Filename  string    `json:"filename"`
	CharCount int       `json:"charCount"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// UpdateIdentityFileRequest represents a request to update an identity file
type UpdateIdentityFileRequest struct {
	Content string `json:"content"`
}

// ============================================================================
// Agent Handlers
// ============================================================================

func (r *Router) handleAgents(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.listAgents(w, req)
	case http.MethodPost:
		r.createAgent(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleAgentDetail(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if path == "/api/agents/" {
		http.Error(w, "Agent ID required", http.StatusBadRequest)
		return
	}

	agentID := path[len("/api/agents/"):]

	// Handle sub-routes
	switch req.URL.Query().Get("action") {
	case "start":
		r.startAgent(w, req, agentID)
	case "stop":
		r.stopAgent(w, req, agentID)
	case "restart":
		r.restartAgent(w, req, agentID)
	case "recreate":
		r.recreateAgent(w, req, agentID)
	case "ping":
		r.pingAgent(w, req, agentID)
	case "chat":
		r.handleAgentChat(w, req, agentID)
	case "logs":
		r.streamAgentLogs(w, req, agentID)
	default:
		switch req.Method {
		case http.MethodGet:
			r.getAgent(w, req, agentID)
		case http.MethodPut:
			r.updateAgent(w, req, agentID)
		case http.MethodDelete:
			r.deleteAgent(w, req, agentID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func (r *Router) listAgents(w http.ResponseWriter, req *http.Request) {
	agents, err := r.agentStore.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

func (r *Router) createAgent(w http.ResponseWriter, req *http.Request) {
	var agent struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Description  string `json:"description"`
		Image        string `json:"image"`
		AgentType    string `json:"agentType"`
		Endpoint     string `json:"endpoint"`
		ModelID      string `json:"modelId"`
		AuthConfigID string `json:"authConfigId"`
	}

	if err := json.NewDecoder(req.Body).Decode(&agent); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Default to docker if not specified
	if agent.AgentType == "" {
		agent.AgentType = "docker"
	}

	// Validate agent type
	if agent.AgentType != "docker" && agent.AgentType != "native" {
		http.Error(w, "agentType must be 'docker' or 'native'", http.StatusBadRequest)
		return
	}

	// For native agents, image is optional
	if agent.AgentType == "native" && agent.Image == "" {
		agent.Image = "native"
	}

	// Generate bearer token inline
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(b)

	status := "stopped"
	if agent.AgentType == "native" {
		status = "running"
	}

	// Extract listen port from the endpoint URL for Docker agents
	listenPort := 0
	if agent.AgentType == "docker" {
		// Use specific port if provided in image or default to random high port
		// For now, we'll try to find a free port or use a common range starting from 17000
		// Actually, let's let the user provide it via Endpoint if they want, but if it's empty, we'll assign one.
		if agent.Endpoint != "" {
			if u, err := url.Parse(agent.Endpoint); err == nil {
				if p := u.Port(); p != "" {
					listenPort, _ = strconv.Atoi(p)
				}
			}
		}
		if listenPort == 0 {
			count := 0
			agents, err := r.agentStore.List()
			if err == nil {
				count = len(agents)
			}
			listenPort = 17000 + count
		}
		agent.Endpoint = fmt.Sprintf("http://127.0.0.1:%d", listenPort)
	}

	newAgent := &store.Agent{
		ID:           agent.ID,
		Name:         agent.Name,
		Description:  agent.Description,
		Image:        agent.Image,
		AgentType:    agent.AgentType,
		Endpoint:     agent.Endpoint,
		Status:       status,
		ListenPort:   listenPort,
		BearerToken:  token,
		ModelID:      agent.ModelID,
		AuthConfigID: agent.AuthConfigID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := r.agentStore.Create(newAgent); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newAgent)
}

func (r *Router) getAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	agent, err := r.agentStore.GetByID(agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agent)
}

func (r *Router) updateAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	agent, err := r.agentStore.GetByID(agentID)
	if err != nil || agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&updates); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Apply updates
	if name, ok := updates["name"].(string); ok {
		agent.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		agent.Description = desc
	}
	needsNewContainer := false
	if img, ok := updates["image"].(string); ok && img != agent.Image {
		agent.Image = img
		needsNewContainer = true
	}
	if ep, ok := updates["endpoint"].(string); ok && ep != agent.Endpoint {
		agent.Endpoint = ep
		needsNewContainer = true
		// Re-derive listen port from the new endpoint
		if u, err := url.Parse(ep); err == nil {
			if p := u.Port(); p != "" {
				agent.ListenPort, _ = strconv.Atoi(p)
			}
		}
	} else if at, ok := updates["agentType"].(string); ok && at == "docker" && at != agent.AgentType {
		// Switching to docker, ensure endpoint is 127.0.0.1
		if agent.ListenPort == 0 {
			count := 0
			agents, err := r.agentStore.List()
			if err == nil {
				count = len(agents)
			}
			agent.ListenPort = 17000 + count
		}
		agent.Endpoint = fmt.Sprintf("http://127.0.0.1:%d", agent.ListenPort)
		needsNewContainer = true
	}

	// Remove stale container so startAgent creates a fresh one
	if needsNewContainer && agent.ContainerID != "" {
		ctx := req.Context()
		r.dockerMgr.StopContainer(ctx, agent.ContainerID)
		r.dockerMgr.RemoveContainer(ctx, agent.ContainerID)
		agent.ContainerID = ""
		agent.Status = "stopped"
	}
	if at, ok := updates["agentType"].(string); ok {
		agent.AgentType = at
	}
	if mID, ok := updates["modelId"].(string); ok {
		agent.ModelID = mID
		needsNewContainer = true
	}
	if aID, ok := updates["authConfigId"].(string); ok {
		agent.AuthConfigID = aID
		needsNewContainer = true
	}

	if err := r.agentStore.Update(agent); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agent)
}

func (r *Router) deleteAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	agent, err := r.agentStore.GetByID(agentID)
	if err == nil && agent != nil && agent.ContainerID != "" {
		ctx := req.Context()
		// Stop and remove container
		r.dockerMgr.StopContainer(ctx, agent.ContainerID)
		r.dockerMgr.RemoveContainer(ctx, agent.ContainerID)
	}

	if err := r.agentStore.Delete(agentID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// doStartAgent contains the core logic for starting an agent container.
// It can be called from both HTTP handlers and background goroutines.
func (r *Router) doStartAgent(agentID string) error {
	agent, err := r.agentStore.GetByID(agentID)
	if err != nil || agent == nil {
		return fmt.Errorf("agent not found")
	}

	// Native agents are always running
	if agent.AgentType == "native" {
		r.agentStore.UpdateStatus(agentID, "running")
		return nil
	}

	ctx := context.Background()

	// Verify container exists in Docker (it may have been removed externally)
	if agent.ContainerID != "" && !r.dockerMgr.ContainerExists(ctx, agent.ContainerID) {
		log.Printf("Container %s for agent %s no longer exists, will recreate", agent.ContainerID, agentID)
		agent.ContainerID = ""
		if err := r.agentStore.Update(agent); err != nil {
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
				r.agentStore.Update(agent)
			}
		}
	}

	// Create container if it doesn't exist
	if agent.ContainerID == "" {
		// Resolve portal URL for the container.
		// If running in Docker, localhost:8080 won't work.
		// For now, we use a special Docker DNS name 'host.docker.internal' which works
		// on Docker Desktop (Mac/Windows) and can be configured for Linux.
		// Podman also supports this or '172.17.0.1'.
		portalURL := os.Getenv("PORTAL_INTERNAL_URL")
		if portalURL == "" {
			// Fallback to host.docker.internal which is common for dev environments
			portalURL = "http://host.docker.internal:8080"
		}

		// Build container config
		containerConfig := docker.ContainerConfig{
			AgentID:     agent.ID,
			AgentImage:  agent.Image,
			PortalURL:   portalURL,
			PortalToken: agent.BearerToken,
			ListenPort:  listenPort,
		}

		// Fetch model config if specified
		if agent.ModelID != "" {
			model, err := r.modelStore.GetByID(agent.ModelID)
			if err != nil {
				return fmt.Errorf("failed to fetch model config: %w", err)
			}
			if model != nil {
				modelConfig := &docker.ModelConfig{
					Provider: model.Provider,
					Name:     model.ModelIdentifier,
					Endpoint: model.EndpointURL,
				}
				// Parse temperature and max_tokens from default_params JSON
				if model.DefaultParams != "" {
					var params map[string]interface{}
					if err := json.Unmarshal([]byte(model.DefaultParams), &params); err != nil {
						return fmt.Errorf("failed to parse model default params: %w", err)
					}
					if temp, ok := params["temperature"].(float64); ok {
						modelConfig.Temperature = &temp
					}
					// Handle max_tokens as float64 (JSON numbers are float64 by default)
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
			auth, err := r.authConfigStore.GetByID(agent.AuthConfigID)
			if err != nil {
				return fmt.Errorf("failed to fetch auth config: %w", err)
			}
			if auth != nil {
				authConfig := &docker.AuthConfig{
					Type:     auth.AuthType,
					Endpoint: auth.EndpointURL,
				}
				// Parse api_key or access_token from credentials JSON
				if auth.Credentials != "" {
					var creds map[string]string
					if err := json.Unmarshal([]byte(auth.Credentials), &creds); err != nil {
						return fmt.Errorf("failed to parse auth credentials: %w", err)
					}
					if apiKey, ok := creds["api_key"]; ok {
						authConfig.ApiKey = apiKey
					}
					// For GitHub Copilot OAuth, use access_token as the API key
					if auth.AuthType == "github_copilot_oauth" {
						if accessToken, ok := creds["access_token"]; ok {
							authConfig.ApiKey = accessToken
						}
					}
				}
				containerConfig.AuthConfig = authConfig
			}
		}

		containerID, err := r.dockerMgr.CreateContainer(ctx, containerConfig)
		if err != nil {
			return fmt.Errorf("failed to create container: %w", err)
		}

		// Update agent with container ID
		agent.ContainerID = containerID
		r.agentStore.Update(agent)
	}

	// Start container
	if err := r.dockerMgr.StartContainer(ctx, agent.ContainerID); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	r.agentStore.UpdateStatus(agentID, "running")
	return nil
}

func (r *Router) startAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	if err := r.doStartAgent(agentID); err != nil {
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

func (r *Router) doStopAgent(agentID string) {
	agent, err := r.agentStore.GetByID(agentID)
	if err != nil || agent == nil {
		r.agentStore.UpdateStatus(agentID, "stopped")
		return
	}

	// Native agents cannot be stopped
	if agent.AgentType == "native" {
		return
	}

	if agent.ContainerID == "" {
		r.agentStore.UpdateStatus(agentID, "stopped")
		return
	}

	ctx := context.Background()
	r.dockerMgr.StopContainer(ctx, agent.ContainerID)
	r.agentStore.UpdateStatus(agentID, "stopped")
}

func (r *Router) stopAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	agent, err := r.agentStore.GetByID(agentID)
	if err != nil || agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	if agent.AgentType == "native" {
		http.Error(w, "Cannot stop native agents", http.StatusBadRequest)
		return
	}

	r.doStopAgent(agentID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func (r *Router) restartAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	agent, err := r.agentStore.GetByID(agentID)
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

	r.agentStore.UpdateStatus(agentID, "restarting")

	// Perform restart asynchronously
	// Capture values to avoid race condition with the agent pointer
	containerID := agent.ContainerID
	go func(cid, aid string) {
		ctx := context.Background()
		if err := r.dockerMgr.RestartContainer(ctx, cid); err != nil {
			log.Printf("Failed to restart container %s: %v", cid, err)
			r.agentStore.UpdateStatus(aid, "stopped")
			return
		}
		r.agentStore.UpdateStatus(aid, "running")
	}(containerID, agentID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "restarting"})
}

func (r *Router) recreateAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	agent, err := r.agentStore.GetByID(agentID)
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

	r.agentStore.UpdateStatus(agentID, "recreating")

	// Perform recreate asynchronously
	// Capture values to avoid race condition with the agent pointer
	containerID := agent.ContainerID
	go func(cid, aid string) {
		ctx := context.Background()
		if err := r.dockerMgr.RemoveContainer(ctx, cid); err != nil {
			log.Printf("Failed to remove container %s: %v", cid, err)
			r.agentStore.UpdateStatus(aid, "stopped")
			return
		}

		// Clear container ID from agent record so doStartAgent creates a new one
		agent, err := r.agentStore.GetByID(aid)
		if err != nil || agent == nil {
			log.Printf("Failed to get agent %s after removing container: %v", aid, err)
			r.agentStore.UpdateStatus(aid, "stopped")
			return
		}
		agent.ContainerID = ""
		if err := r.agentStore.Update(agent); err != nil {
			log.Printf("Failed to update agent %s after clearing container ID: %v", aid, err)
			r.agentStore.UpdateStatus(aid, "stopped")
			return
		}

		// Start a new container
		if err := r.doStartAgent(aid); err != nil {
			log.Printf("Failed to start agent %s after recreating: %v", aid, err)
			r.agentStore.UpdateStatus(aid, "stopped")
			return
		}
	}(containerID, agentID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "recreating"})
}

func (r *Router) pingAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	agent, err := r.agentStore.GetByID(agentID)
	if err != nil || agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	endpoint := agent.Endpoint
	if endpoint == "" {
		http.Error(w, "Agent has no endpoint", http.StatusBadRequest)
		return
	}

	// Try to reach the agent's well-known card
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

func (r *Router) handleAgentChat(w http.ResponseWriter, req *http.Request, agentID string) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agent, err := r.agentStore.GetByID(agentID)
	if err != nil || agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	var chatReq ChatRequest
	if err := json.NewDecoder(req.Body).Decode(&chatReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(chatReq.Messages) == 0 {
		http.Error(w, "Messages cannot be empty", http.StatusBadRequest)
		return
	}

	lastMsg := chatReq.Messages[len(chatReq.Messages)-1]

	// Forward task to agent first to get the agent's task ID
	taskURL := fmt.Sprintf("%s/tasks", agent.Endpoint)
	createReq := a2a.CreateTaskRequest{
		Message: a2a.TaskMessage{
			Role:      lastMsg.Role,
			Content:   lastMsg.ContentString(),
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

	// Parse the agent's response to get its task ID
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

	// Save the task locally using the agent's task ID
	_, err = r.createTaskWithID(taskID, a2a.ChannelID("portal", agentID), "portal", agentID, a2a.TaskMessage{
		Role:      lastMsg.Role,
		Content:   lastMsg.ContentString(),
		Timestamp: time.Now(),
	})
	if err != nil {
		log.Printf("Failed to save task %s locally: %v", taskID, err)
		http.Error(w, fmt.Sprintf("Failed to save task: %v", err), http.StatusInternalServerError)
		return
	}

	// Subscribe to agent's SSE stream in the background
	go r.subscribeToAgentSSE(agent, taskID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"taskId": taskID})
}

func (r *Router) streamAgentLogs(w http.ResponseWriter, req *http.Request, agentID string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var flusher http.Flusher
	if f, ok := w.(http.Flusher); ok {
		flusher = f
	} else if rr, ok := w.(*responseRecorder); ok {
		if f, ok := rr.ResponseWriter.(http.Flusher); ok {
			flusher = f
		}
	}

	// Send mock log data (when Docker SDK is implemented)
	fmt.Fprintf(w, "data: Log streaming not yet implemented\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func (r *Router) streamAgents(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var flusher http.Flusher
	if f, ok := w.(http.Flusher); ok {
		flusher = f
	} else if rr, ok := w.(*responseRecorder); ok {
		if f, ok := rr.ResponseWriter.(http.Flusher); ok {
			flusher = f
		}
	}

	if flusher == nil {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	ctx := req.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			agents, err := r.agentStore.List()
			if err != nil {
				continue
			}
			data, _ := json.Marshal(agents)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// ============================================================================
// Identity File Handlers
// ============================================================================

// handleAgentIdentityFiles handles GET /api/agents/{id}/identity-files
func (r *Router) handleAgentIdentityFiles(w http.ResponseWriter, req *http.Request) {
	// Parse agent ID from path
	path := req.URL.Path
	prefix := "/api/agents/"
	suffix := "/identity-files"

	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	agentID := path[len(prefix) : len(path)-len(suffix)]
	if agentID == "" {
		http.Error(w, "Agent ID required", http.StatusBadRequest)
		return
	}

	// Verify agent exists
	agent, err := r.agentStore.GetByID(agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	switch req.Method {
	case http.MethodGet:
		r.listIdentityFiles(w, req, agentID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAgentIdentityFileDetail handles PUT /api/agents/{id}/identity-files/{filename}
func (r *Router) handleAgentIdentityFileDetail(w http.ResponseWriter, req *http.Request) {
	// Parse path: /api/agents/{id}/identity-files/{filename}
	path := req.URL.Path
	prefix := "/api/agents/"

	if !strings.HasPrefix(path, prefix) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Remove prefix and split remaining path
	remaining := path[len(prefix):]
	parts := strings.SplitN(remaining, "/identity-files/", 2)
	if len(parts) != 2 {
		http.Error(w, "Invalid path format", http.StatusBadRequest)
		return
	}

	agentID := parts[0]
	filename := parts[1]

	log.Printf("Identity file request: agentID=%q filename=%q method=%s", agentID, filename, req.Method)

	if agentID == "" {
		http.Error(w, "Agent ID required", http.StatusBadRequest)
		return
	}
	if filename == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	// Verify agent exists
	agent, err := r.agentStore.GetByID(agentID)
	if err != nil {
		log.Printf("Identity file: error looking up agent %q: %v", agentID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if agent == nil {
		log.Printf("Identity file: agent %q not found", agentID)
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	switch req.Method {
	case http.MethodGet:
		r.getIdentityFile(w, req, agentID, filename)
	case http.MethodPut:
		r.updateIdentityFile(w, req, agentID, filename)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listIdentityFiles lists all identity files for an agent
func (r *Router) listIdentityFiles(w http.ResponseWriter, req *http.Request, agentID string) {
	// Get files from database
	identityFileStore := store.NewIdentityFileStore(r.db)
	files, err := identityFileStore.ListByAgent(agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build response with metadata (without full content for list)
	response := AgentIdentityFilesResponse{
		Files: make([]AgentIdentityFileInfo, len(files)),
	}
	for i, f := range files {
		response.Files[i] = AgentIdentityFileInfo{
			Filename:  f.Filename,
			CharCount: f.CharCount,
			UpdatedAt: f.UpdatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// getIdentityFile reads the current content of an identity file
func (r *Router) getIdentityFile(w http.ResponseWriter, req *http.Request, agentID, filename string) {
	// If agent is running, read from Docker container
	agent, err := r.agentStore.GetByID(agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var content []byte
	if agent.Status == "running" && agent.ContainerID != "" {
		// Read from container
		ctx := req.Context()
		content, err = r.dockerMgr.ReadWorkspaceFile(ctx, agentID, filename)
		if err != nil {
			// If file doesn't exist in container, fall back to database
			if !strings.Contains(err.Error(), "not found") {
				log.Printf("Failed to read from container, falling back to DB: %v", err)
			}
		}
	}

	// If not running or container read failed, read from database
	if content == nil {
		identityFileStore := store.NewIdentityFileStore(r.db)
		file, err := identityFileStore.GetByAgentAndFilename(agentID, filename)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if file != nil {
			content = []byte(file.Content)
		}
	}

	// Return file content
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"filename": filename,
		"content":  string(content),
	})
}

// updateIdentityFile updates the content of an identity file
func (r *Router) updateIdentityFile(w http.ResponseWriter, req *http.Request, agentID, filename string) {
	// Parse request body
	var request UpdateIdentityFileRequest
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Check size limit (16KB)
	const maxSize = 16 * 1024
	if len(request.Content) > maxSize {
		http.Error(w, fmt.Sprintf("Content exceeds maximum size of %d bytes", maxSize), http.StatusBadRequest)
		return
	}

	// Save to database
	identityFileStore := store.NewIdentityFileStore(r.db)
	file := &models.AgentIdentityFile{
		AgentID:  agentID,
		Filename: filename,
		Content:  request.Content,
	}

	if err := identityFileStore.CreateOrUpdate(file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If agent is running, sync to container
	agent, err := r.agentStore.GetByID(agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if agent.Status == "running" && agent.ContainerID != "" {
		ctx := req.Context()
		if err := r.dockerMgr.WriteWorkspaceFile(ctx, agentID, filename, []byte(request.Content)); err != nil {
			// Log error but don't fail - DB is source of truth
			log.Printf("Warning: Failed to sync %s to container: %v", filename, err)
		}
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"filename":  filename,
		"charCount": utf8.RuneCountInString(request.Content),
	})
}
