package agents

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/zeroclaw/bot-portal/internal/a2a"
	"github.com/zeroclaw/bot-portal/internal/docker"
	"github.com/zeroclaw/bot-portal/internal/store"
)

// ValidAgentIDPattern defines allowed characters in agent IDs
// Allows: alphanumeric, hyphens, underscores
var ValidAgentIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

const (
	MaxAgentIDLength = 64
	MinAgentIDLength = 1

	// TokenBytes is the number of random bytes for bearer tokens
	// 64 bytes = 512 bits of entropy = 128 hex characters
	// This provides ~2^256 security against brute force
	TokenBytes = 64

	// Expected token length in hex encoding
	ExpectedTokenLength = TokenBytes * 2
)

// ============================================================================
// Types
// ============================================================================

// Handler holds dependencies for agent HTTP handlers.
type Handler struct {
	DB              *sql.DB
	DockerMgr       *docker.Manager
	AgentStore      *store.AgentStore
	ModelStore      *store.ModelStore
	AuthConfigStore *store.AuthConfigStore
	MessageStore    *store.MessageStore

	// CreateTaskWithID is a callback used by chat to create a task.
	CreateTaskWithID func(id, channelID, senderID, recipientID string, message a2a.TaskMessage) (string, error)
	// SubscribeToAgentSSE is a callback used by chat to subscribe to SSE updates.
	SubscribeToAgentSSE func(agent *store.Agent, taskID string)
}

// NewHandler creates a new agents Handler with the given dependencies.
func NewHandler(
	db *sql.DB,
	dockerMgr *docker.Manager,
	agentStore *store.AgentStore,
	modelStore *store.ModelStore,
	authConfigStore *store.AuthConfigStore,
	messageStore *store.MessageStore,
) *Handler {
	return &Handler{
		DB:              db,
		DockerMgr:       dockerMgr,
		AgentStore:      agentStore,
		ModelStore:      modelStore,
		AuthConfigStore: authConfigStore,
		MessageStore:    messageStore,
	}
}

// AgentIdentityFilesResponse represents the response for listing agent identity files.
type AgentIdentityFilesResponse struct {
	Files []AgentIdentityFileInfo `json:"files"`
}

// AgentIdentityFileInfo represents metadata for an identity file (without content).
type AgentIdentityFileInfo struct {
	Filename  string    `json:"filename"`
	CharCount int       `json:"charCount"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// UpdateIdentityFileRequest represents a request to update an identity file.
type UpdateIdentityFileRequest struct {
	Content string `json:"content"`
}

// ============================================================================
// Routing / Dispatch
// ============================================================================

// HandleAgents dispatches GET/POST /api/agents.
func (h *Handler) HandleAgents(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		h.listAgents(w, req)
	case http.MethodPost:
		h.createAgent(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleAgentDetail dispatches /api/agents/{id} and action sub-routes.
func (h *Handler) HandleAgentDetail(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if path == "/api/agents/" {
		http.Error(w, "Agent ID required", http.StatusBadRequest)
		return
	}

	agentID, err := h.validateAndExtractAgentID(path)
	if err != nil {
		http.Error(w, "Invalid agent ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Handle sub-routes
	switch req.URL.Query().Get("action") {
	case "start":
		h.startAgent(w, req, agentID)
	case "stop":
		h.stopAgent(w, req, agentID)
	case "restart":
		h.restartAgent(w, req, agentID)
	case "recreate":
		h.recreateAgent(w, req, agentID)
	case "ping":
		h.pingAgent(w, req, agentID)
	case "chat":
		h.handleAgentChat(w, req, agentID)
	case "logs":
		h.streamAgentLogs(w, req, agentID)
	case "sync-peers":
		h.syncAgentPeers(w, req, agentID)
	default:
		switch req.Method {
		case http.MethodGet:
			h.getAgent(w, req, agentID)
		case http.MethodPut:
			h.updateAgent(w, req, agentID)
		case http.MethodDelete:
			h.deleteAgent(w, req, agentID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// StreamAgents streams agent list updates via SSE.
func (h *Handler) StreamAgents(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var flusher http.Flusher
	if f, ok := w.(http.Flusher); ok {
		flusher = f
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
			agents, err := h.AgentStore.List()
			if err != nil {
				continue
			}
			data, _ := json.Marshal(agents)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// validateAndExtractAgentID extracts and validates an agent ID from the URL path
// Returns an error if the ID is invalid or potentially malicious
func (h *Handler) validateAndExtractAgentID(path string) (string, error) {
	prefix := "/api/agents/"
	agentID := path[len(prefix):]

	// Check length constraints
	if len(agentID) < MinAgentIDLength || len(agentID) > MaxAgentIDLength {
		return "", fmt.Errorf("agent ID must be between %d and %d characters", MinAgentIDLength, MaxAgentIDLength)
	}

	// Validate against allowed pattern
	if !ValidAgentIDPattern.MatchString(agentID) {
		return "", errors.New("agent ID contains invalid characters (allowed: alphanumeric, hyphens, underscores)")
	}

	return agentID, nil
}

// ============================================================================
// CRUD Handlers
// ============================================================================

func (h *Handler) listAgents(w http.ResponseWriter, req *http.Request) {
	agents, err := h.AgentStore.List()
	if err != nil {
		log.Printf("[error] listAgents: %v", err)
		http.Error(w, "Failed to list agents", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

func (h *Handler) createAgent(w http.ResponseWriter, req *http.Request) {
	var agent struct {
		ID           string   `json:"id"`
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		Image        string   `json:"image"`
		AgentType    string   `json:"agentType"`
		Endpoint     string   `json:"endpoint"`
		ModelID      string   `json:"modelId"`
		AuthConfigID string   `json:"authConfigId"`
		PeerAgentIDs []string `json:"peerAgentIds"`
	}

	if err := json.NewDecoder(req.Body).Decode(&agent); err != nil {
		log.Printf("[error] createAgent decode: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
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

	// Generate bearer token with high entropy (512 bits)
	b := make([]byte, TokenBytes)
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
		if agent.Endpoint != "" {
			if u, err := url.Parse(agent.Endpoint); err == nil {
				if p := u.Port(); p != "" {
					listenPort, _ = strconv.Atoi(p)
				}
			}
		}
		if listenPort == 0 {
			count := 0
			agents, err := h.AgentStore.List()
			if err == nil {
				count = len(agents)
			}
			listenPort = 17000 + count
		}
		agent.Endpoint = fmt.Sprintf("http://127.0.0.1:%d", listenPort)
	}

	peerAgentIDsJSON, _ := json.Marshal(agent.PeerAgentIDs)
	if agent.PeerAgentIDs == nil {
		peerAgentIDsJSON = []byte("[]")
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
		PeerAgentIDs: peerAgentIDsJSON,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := h.AgentStore.Create(newAgent); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Sync symmetric peer relationships
	if len(agent.PeerAgentIDs) > 0 {
		h.syncSymmetricPeers(agent.ID, nil, agent.PeerAgentIDs)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newAgent)
}

func (h *Handler) getAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	agent, err := h.AgentStore.GetByID(agentID)
	if err != nil {
		log.Printf("[error] getAgent %s: %v", agentID, err)
		http.Error(w, "Failed to get agent", http.StatusInternalServerError)
		return
	}
	if agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agent)
}

func (h *Handler) updateAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	agent, err := h.AgentStore.GetByID(agentID)
	if err != nil || agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	// Capture old peer IDs before applying updates
	var oldPeerIDs []string
	if len(agent.PeerAgentIDs) > 0 {
		json.Unmarshal(agent.PeerAgentIDs, &oldPeerIDs)
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&updates); err != nil {
		log.Printf("[error] updateAgent decode %s: %v", agentID, err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
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
			agents, err := h.AgentStore.List()
			if err == nil {
				count = len(agents)
			}
			agent.ListenPort = 17000 + count
		}
		agent.Endpoint = fmt.Sprintf("http://127.0.0.1:%d", agent.ListenPort)
		needsNewContainer = true
	}

	if mID, ok := updates["modelId"].(string); ok {
		agent.ModelID = mID
		needsNewContainer = true
	}
	if aID, ok := updates["authConfigId"].(string); ok {
		agent.AuthConfigID = aID
		needsNewContainer = true
	}
	if peerIDs, ok := updates["peerAgentIds"].([]interface{}); ok {
		ids := make([]string, 0, len(peerIDs))
		for _, id := range peerIDs {
			if s, ok := id.(string); ok {
				ids = append(ids, s)
			}
		}
		peerJSON, _ := json.Marshal(ids)
		agent.PeerAgentIDs = peerJSON
	}

	// Remove stale container so startAgent creates a fresh one
	if needsNewContainer && agent.ContainerID != "" {
		ctx := req.Context()
		h.DockerMgr.StopContainer(ctx, agent.ContainerID)
		h.DockerMgr.RemoveContainer(ctx, agent.ContainerID)
		agent.ContainerID = ""
		agent.Status = "stopped"
	}
	if at, ok := updates["agentType"].(string); ok {
		agent.AgentType = at
	}

	if err := h.AgentStore.Update(agent); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If peers changed, sync symmetric relationships and regenerate configs
	if _, ok := updates["peerAgentIds"]; ok {
		var newPeerIDs []string
		json.Unmarshal(agent.PeerAgentIDs, &newPeerIDs)
		h.syncSymmetricPeers(agentID, oldPeerIDs, newPeerIDs)

		// Regenerate this agent's own config if running
		if agent.Status == "running" && agent.ContainerID != "" {
			if err := h.regenerateAgentConfig(agent); err != nil {
				fmt.Printf("Warning: failed to sync peers config for %s: %v\n", agentID, err)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agent)
}

func (h *Handler) deleteAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	agent, err := h.AgentStore.GetByID(agentID)
	if err == nil && agent != nil {
		// Remove this agent from all its peers' lists
		var peerIDs []string
		if len(agent.PeerAgentIDs) > 0 {
			json.Unmarshal(agent.PeerAgentIDs, &peerIDs)
		}
		for _, peerID := range peerIDs {
			h.removePeerFromAgent(peerID, agentID)
		}

		if agent.ContainerID != "" {
			ctx := req.Context()
			h.DockerMgr.StopContainer(ctx, agent.ContainerID)
			h.DockerMgr.RemoveContainer(ctx, agent.ContainerID)
		}
	}

	if err := h.AgentStore.Delete(agentID); err != nil {
		log.Printf("[error] deleteAgent %s: %v", agentID, err)
		http.Error(w, "Failed to delete agent", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetAgentStoreForRouting exposes agent listing for A2A routing.
func (h *Handler) GetAgentStoreForRouting() *store.AgentStore {
	return h.AgentStore
}

// DoStartAgent exposes the start logic for use by the Router (e.g. background tasks).
func (h *Handler) DoStartAgent(agentID string) error {
	return h.doStartAgent(agentID)
}

// DoStopAgent exposes the stop logic for use by the Router.
func (h *Handler) DoStopAgent(agentID string) {
	h.doStopAgent(agentID)
}
