package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/zeroclaw/bot-portal/internal/a2a"
	"github.com/zeroclaw/bot-portal/internal/docker"
	"github.com/zeroclaw/bot-portal/internal/store"
)

// Router is the main HTTP router
type Router struct {
	db                *sql.DB
	dockerMgr         *docker.Manager
	agentStore        *store.AgentStore
	channelStore      *store.ChannelStore
	messageStore      *store.MessageStore
	modelStore        *store.ModelStore
	authConfigStore   *store.AuthConfigStore
	a2aRouter         *a2a.Router
}

// NewRouter creates a new API router
func NewRouter(db *sql.DB, dockerMgr *docker.Manager) *Router {
	agentStore := store.NewAgentStore(db)
	channelStore := store.NewChannelStore(db)
	messageStore := store.NewMessageStore(db)
	modelStore := store.NewModelStore(db)
	authConfigStore := store.NewAuthConfigStore(db)

	router := &Router{
		db:              db,
		dockerMgr:       dockerMgr,
		agentStore:      agentStore,
		channelStore:    channelStore,
		messageStore:    messageStore,
		modelStore:      modelStore,
		authConfigStore: authConfigStore,
	}

	// Initialize A2A router
	a2aRouter := a2a.NewRouter()
	a2aRouter.CreateTask = router.createTask
	a2aRouter.GetTask = router.getTask
	a2aRouter.UpdateTaskStatus = router.updateTaskStatus
	a2aRouter.GetAgents = router.getAgentsForRouting
	router.a2aRouter = a2aRouter

	// Ensure general channel exists
	channelStore.EnsureGeneralChannel()

	// Ensure Docker network exists
	ctx := context.Background()
	if err := dockerMgr.EnsureNetwork(ctx); err != nil {
		log.Printf("Warning: Failed to ensure Docker network: %v", err)
	}

	return router
}

// Run starts the HTTP server
func (r *Router) Run(addr string) error {
	mux := http.NewServeMux()

	// A2A endpoints (Google A2A Protocol)
	mux.HandleFunc("/.well-known/agent.json", r.handleAgentCard)
	mux.HandleFunc("/tasks", r.requireBearerToken(r.handleTasks))
	mux.HandleFunc("/tasks/", r.requireBearerToken(r.handleTaskDetail))

	// REST API endpoints
	// Agent management
	mux.HandleFunc("/api/agents", r.handleAgents)
	mux.HandleFunc("/api/agents/", r.handleAgentDetail)
	mux.HandleFunc("/api/agents-stream", r.streamAgents)

	// Channel management
	mux.HandleFunc("/api/channels", r.handleChannels)
	mux.HandleFunc("/api/channels/", r.handleChannelDetail)

	// Messages
	mux.HandleFunc("/api/messages/stream", r.handleMessageStream)

	// Model management
	mux.HandleFunc("/api/models", r.handleModels)
	mux.HandleFunc("/api/models/", r.handleModelDetail)

	// Auth Config management (placeholder for task 7)
	mux.HandleFunc("/api/auth-configs", r.handleAuthConfigs)
	mux.HandleFunc("/api/auth-configs/", r.handleAuthConfigDetail)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("OK"))
	})

	// Wrap with CORS middleware
	handler := r.corsMiddleware(mux)

	log.Printf("Server starting on %s", addr)
	return http.ListenAndServe(addr, handler)
}

// corsMiddleware adds CORS headers for frontend dev
func (r *Router) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if req.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, req)
	})
}

// ============================================================================
// Authentication Middleware
// ============================================================================

func (r *Router) requireBearerToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		authHeader := req.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		const prefix = "Bearer "
		if len(authHeader) < len(prefix) || authHeader[:len(prefix)] != prefix {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		token := authHeader[len(prefix):]
		agents, err := r.agentStore.List()
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		valid := false
		for _, agent := range agents {
			if agent.BearerToken == token {
				valid = true
				break
			}
		}

		if !valid {
			http.Error(w, "Invalid bearer token", http.StatusUnauthorized)
			return
		}

		next(w, req)
	}
}

// ============================================================================
// A2A Protocol Handlers
// ============================================================================

func (r *Router) handleAgentCard(w http.ResponseWriter, req *http.Request) {
	r.a2aRouter.HandleAgentCard(w, req)
}

func (r *Router) handleTasks(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		r.a2aRouter.HandleTaskCreate(w, req)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleTaskDetail(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if path == "/tasks/" {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}

	taskID := path[len("/tasks/"):]
	req = req.WithContext(context.WithValue(req.Context(), "taskID", taskID))

	if req.Method == http.MethodGet {
		r.a2aRouter.HandleTaskGet(w, req)
	} else if req.Method == http.MethodPost {
		// Check for cancel
		if req.URL.RawQuery == "cancel" {
			r.a2aRouter.HandleTaskCancel(w, req)
		}
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
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
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Image       string `json:"image"`
		AgentType   string `json:"agentType"`
		Endpoint    string `json:"endpoint"`
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
	if agent.AgentType == "docker" && agent.Endpoint != "" {
		if u, err := url.Parse(agent.Endpoint); err == nil {
			if p := u.Port(); p != "" {
				listenPort, _ = strconv.Atoi(p)
			}
		}
	}

	newAgent := &store.Agent{
		ID:          agent.ID,
		Name:        agent.Name,
		Description: agent.Description,
		Image:       agent.Image,
		AgentType:   agent.AgentType,
		Endpoint:    agent.Endpoint,
		Status:      status,
		ListenPort:  listenPort,
		BearerToken: token,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
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

func (r *Router) startAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	agent, err := r.agentStore.GetByID(agentID)
	if err != nil || agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	// Native agents are always running
	if agent.AgentType == "native" {
		r.agentStore.UpdateStatus(agentID, "running")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "running", "message": "Native agent marked as running"})
		return
	}

	ctx := req.Context()

	// Resolve listen port from endpoint if not already set
	listenPort := agent.ListenPort
	if listenPort == 0 && agent.Endpoint != "" {
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
		// Build container config
		containerConfig := docker.ContainerConfig{
			AgentID:     agent.ID,
			AgentImage:  agent.Image,
			PortalURL:   fmt.Sprintf("http://localhost:%d", 8080),
			PortalToken: agent.BearerToken,
			ListenPort:  listenPort,
		}

		// Fetch model config if specified
		if agent.ModelID != "" {
			model, err := r.modelStore.GetByID(agent.ModelID)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to fetch model config: %v", err), http.StatusInternalServerError)
				return
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
					if err := json.Unmarshal([]byte(model.DefaultParams), &params); err == nil {
						if temp, ok := params["temperature"].(float64); ok {
							modelConfig.Temperature = &temp
						}
						// Handle max_tokens as float64 (JSON numbers are float64 by default)
						if maxTokensFloat, ok := params["max_tokens"].(float64); ok {
							maxTokens := int(maxTokensFloat)
							modelConfig.MaxTokens = &maxTokens
						}
					}
				}
				containerConfig.ModelConfig = modelConfig
			}
		}

		// Fetch auth config if specified
		if agent.AuthConfigID != "" {
			auth, err := r.authConfigStore.GetByID(agent.AuthConfigID)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to fetch auth config: %v", err), http.StatusInternalServerError)
				return
			}
			if auth != nil {
				authConfig := &docker.AuthConfig{
					Type:     auth.AuthType,
					Endpoint: auth.EndpointURL,
				}
				// Parse api_key from credentials JSON
				if auth.Credentials != "" {
					var creds map[string]string
					if err := json.Unmarshal([]byte(auth.Credentials), &creds); err == nil {
						if apiKey, ok := creds["api_key"]; ok {
							authConfig.ApiKey = apiKey
						}
					}
				}
				containerConfig.AuthConfig = authConfig
			}
		}

		containerID, err := r.dockerMgr.CreateContainer(ctx, containerConfig)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create container: %v", err), http.StatusInternalServerError)
			return
		}
		agent.ContainerID = containerID
		r.agentStore.Update(agent)
	}

	// Start container
	if err := r.dockerMgr.StartContainer(ctx, agent.ContainerID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to start container: %v", err), http.StatusInternalServerError)
		return
	}

	r.agentStore.UpdateStatus(agentID, "running")

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
	go func() {
		ctx := context.Background()
		if err := r.dockerMgr.RestartContainer(ctx, agent.ContainerID); err != nil {
			log.Printf("Failed to restart container %s: %v", agent.ContainerID, err)
			r.agentStore.UpdateStatus(agentID, "stopped")
			return
		}
		r.agentStore.UpdateStatus(agentID, "running")
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "restarting"})
}

func (r *Router) streamAgentLogs(w http.ResponseWriter, req *http.Request, agentID string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send mock log data (when Docker SDK is implemented)
	fmt.Fprintf(w, "data: Log streaming not yet implemented\n\n")
}

func (r *Router) streamAgents(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
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
// Channel Handlers
// ============================================================================

func (r *Router) handleChannels(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.listChannels(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleChannelDetail(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if path == "/api/channels/" {
		http.Error(w, "Channel ID required", http.StatusBadRequest)
		return
	}

	channelID := path[len("/api/channels/"):]

	// Check if this is a messages request
	if req.URL.Query().Get("messages") == "true" {
		r.getChannelMessages(w, req, channelID)
		return
	}

	r.getChannel(w, req, channelID)
}

func (r *Router) listChannels(w http.ResponseWriter, req *http.Request) {
	channels, err := r.channelStore.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channels)
}

func (r *Router) getChannel(w http.ResponseWriter, req *http.Request, channelID string) {
	channel, err := r.channelStore.GetByID(channelID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if channel == nil {
		http.Error(w, "Channel not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channel)
}

func (r *Router) getChannelMessages(w http.ResponseWriter, req *http.Request, channelID string) {
	messages, err := r.messageStore.ListByChannel(channelID, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// ============================================================================
// Message Stream
// ============================================================================

func (r *Router) handleMessageStream(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	notify := req.Context().Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-notify:
			return
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// ============================================================================
// Helper methods for A2A Router
// ============================================================================

func (r *Router) createTask(channelID, senderID, recipientID string, message a2a.TaskMessage) (string, error) {
	taskLog := &store.TaskLog{
		ID:          fmt.Sprintf("task-%d", time.Now().UnixNano()),
		ChannelID:   channelID,
		SenderID:    senderID,
		RecipientID: recipientID,
		Status:      "pending",
		Direction:   "inbound",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	messages, _ := json.Marshal([]a2a.TaskMessage{message})
	taskLog.Messages = messages

	if err := r.messageStore.Create(taskLog); err != nil {
		return "", err
	}

	return taskLog.ID, nil
}

func (r *Router) getTask(id string) (*a2a.Task, error) {
	taskLog, err := r.messageStore.GetByID(id)
	if err != nil {
		return nil, err
	}
	if taskLog == nil {
		return nil, nil
	}

	task := &a2a.Task{
		ID:        taskLog.ID,
		Status:    a2a.TaskStatus(taskLog.Status),
		CreatedAt: taskLog.CreatedAt,
		UpdatedAt: taskLog.UpdatedAt,
	}
	json.Unmarshal(taskLog.Messages, &task.Messages)
	json.Unmarshal(taskLog.Artifacts, &task.Artifacts)

	return task, nil
}

func (r *Router) updateTaskStatus(id, status string) error {
	taskLog, err := r.messageStore.GetByID(id)
	if err != nil || taskLog == nil {
		return err
	}

	taskLog.Status = status
	taskLog.UpdatedAt = time.Now()

	return r.messageStore.Update(taskLog)
}

func (r *Router) getAgentsForRouting() ([]a2a.AgentInfo, error) {
	agents, err := r.agentStore.List()
	if err != nil {
		return nil, err
	}

	var result []a2a.AgentInfo
	for _, a := range agents {
		result = append(result, a2a.AgentInfo{
			ID:          a.ID,
			Name:        a.Name,
			Endpoint:    a.Endpoint,
			BearerToken: a.BearerToken,
		})
	}

	return result, nil
}


