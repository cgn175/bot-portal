package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/zeroclaw/bot-portal/internal/a2a"
	"github.com/zeroclaw/bot-portal/internal/docker"
	"github.com/zeroclaw/bot-portal/internal/store"
)

// Router is the main HTTP router
type Router struct {
	db           *sql.DB
	dockerMgr    *docker.Manager
	agentStore   *store.AgentStore
	channelStore *store.ChannelStore
	messageStore *store.MessageStore
	a2aRouter    *a2a.Router
}

// NewRouter creates a new API router
func NewRouter(db *sql.DB, dockerMgr *docker.Manager) *Router {
	agentStore := store.NewAgentStore(db)
	channelStore := store.NewChannelStore(db)
	messageStore := store.NewMessageStore(db)

	router := &Router{
		db:           db,
		dockerMgr:    dockerMgr,
		agentStore:   agentStore,
		channelStore: channelStore,
		messageStore: messageStore,
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

	return router
}

// Run starts the HTTP server
func (r *Router) Run(addr string) error {
	// A2A endpoints (Google A2A Protocol)
	http.HandleFunc("/.well-known/agent.json", r.handleAgentCard)
	http.HandleFunc("/tasks", r.handleTasks)
	http.HandleFunc("/tasks/", r.handleTaskDetail)

	// REST API endpoints
	// Agent management
	http.HandleFunc("/api/agents", r.handleAgents)
	http.HandleFunc("/api/agents/", r.handleAgentDetail)

	// Channel management
	http.HandleFunc("/api/channels", r.handleChannels)
	http.HandleFunc("/api/channels/", r.handleChannelDetail)

	// Messages
	http.HandleFunc("/api/messages/stream", r.handleMessageStream)

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("OK"))
	})

	log.Printf("Server starting on %s", addr)
	return http.ListenAndServe(addr, nil)
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
		ID       string `json:"id"`
		Name     string `json:"name"`
		Image    string `json:"image"`
		Endpoint string `json:"endpoint"`
	}

	if err := json.NewDecoder(req.Body).Decode(&agent); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate bearer token
	token, _ := r.agentStore.GenerateBearerToken(agent.ID)

	newAgent := &store.Agent{
		ID:          agent.ID,
		Name:        agent.Name,
		Image:       agent.Image,
		Endpoint:    agent.Endpoint,
		Status:      "stopped",
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

	if err := r.agentStore.Update(agent); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agent)
}

func (r *Router) deleteAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	// Stop container first
	r.stopAgent(w, req, agentID)

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

	// Create and start container (when Docker SDK is implemented)
	r.agentStore.UpdateStatus(agentID, "running")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (r *Router) stopAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	r.agentStore.UpdateStatus(agentID, "stopped")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func (r *Router) restartAgent(w http.ResponseWriter, req *http.Request, agentID string) {
	r.agentStore.UpdateStatus(agentID, "restarting")
	time.Sleep(500 * time.Millisecond)
	r.agentStore.UpdateStatus(agentID, "running")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "restarted"})
}

func (r *Router) streamAgentLogs(w http.ResponseWriter, req *http.Request, agentID string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send mock log data (when Docker SDK is implemented)
	fmt.Fprintf(w, "data: Log streaming not yet implemented\n\n")
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
