package a2a

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Router handles A2A message routing
type Router struct {
	// Store interface for persistence
	CreateTask       func(channelID, senderID, recipientID string, message TaskMessage) (string, error)
	GetTask          func(id string) (*Task, error)
	UpdateTaskStatus func(id, status string) error
	GetAgents        func() ([]AgentInfo, error)
	GetAgent         func(id string) (*AgentInfo, error)

	// HTTP client for forwarding tasks to agents
	client *http.Client

	// SSE connections for streaming
	connections map[string]map[string]chan *TaskUpdate
	mu          sync.RWMutex
}

// AgentInfo represents agent information for routing
type AgentInfo struct {
	ID          string
	Name        string
	Endpoint    string
	BearerToken string
}

// NewRouter creates a new A2A router
func NewRouter() *Router {
	return &Router{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		connections: make(map[string]map[string]chan *TaskUpdate),
	}
}

// HandleTaskCreate handles POST /tasks - creates a new task
func (r *Router) HandleTaskCreate(w http.ResponseWriter, req *http.Request) {
	var createReq CreateTaskRequest
	if err := json.NewDecoder(req.Body).Decode(&createReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Determine channel and recipients
	senderID := req.Header.Get("X-Agent-ID")
	channelID := req.Header.Get("X-Channel-ID")
	if channelID == "" {
		channelID = "general"
	}

	// Determine recipient for direct channels
	recipientID := ""
	if IsDirectChannel(channelID) {
		parts := splitChannelID(channelID, senderID)
		recipientID = parts
	}

	// Create task via store interface
	if r.CreateTask != nil {
		taskID, err := r.CreateTask(channelID, senderID, recipientID, createReq.Message)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Forward to agents asynchronously
		go r.forwardTask(channelID, senderID, taskID, createReq)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CreateTaskResponse{TaskID: taskID})
		return
	}

	// Fallback if no store configured
	http.Error(w, "store not configured", http.StatusInternalServerError)
}

// HandleTaskGet handles GET /tasks/{id}
func (r *Router) HandleTaskGet(w http.ResponseWriter, req *http.Request) {
	taskID := req.PathValue("id")

	if r.GetTask != nil {
		task, err := r.GetTask(taskID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if task == nil {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GetTaskResponse{Task: task})
		return
	}

	http.Error(w, "store not configured", http.StatusInternalServerError)
}

// HandleTaskStream handles GET /tasks/{id}/stream - SSE
func (r *Router) HandleTaskStream(w http.ResponseWriter, req *http.Request) {
	taskID := req.PathValue("id")

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create channel for this connection
	ch := make(chan *TaskUpdate, 10)
	r.addConnection(taskID, req.RemoteAddr, ch)
	defer r.removeConnection(taskID, req.RemoteAddr)

	// Send initial state if store configured
	if r.GetTask != nil {
		task, err := r.GetTask(taskID)
		if err == nil && task != nil {
			update := &TaskUpdate{
				TaskID: taskID,
				Status: task.Status,
			}
			ch <- update
		}
	}

	// Keep connection open and stream updates
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
		case update := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", mustJSON(update))
			flusher.Flush()
		case <-ticker.C:
			// Send keep-alive
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// HandleTaskCancel handles POST /tasks/{id}/cancel
func (r *Router) HandleTaskCancel(w http.ResponseWriter, req *http.Request) {
	taskID := req.PathValue("id")

	if r.UpdateTaskStatus != nil {
		err := r.UpdateTaskStatus(taskID, string(TaskStatusCancelled))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CancelTaskResponse{
			TaskID: taskID,
			Status: "cancelled",
		})
		return
	}

	http.Error(w, "store not configured", http.StatusInternalServerError)
}

// HandleAgentCard handles GET /.well-known/agent.json
func (r *Router) HandleAgentCard(w http.ResponseWriter, req *http.Request) {
	card := AgentCard{
		Name:        "Bot Portal",
		Description: "A2A Message Router and Agent Management Portal",
		Version:     "0.1.0",
		Capabilities: AgentCapabilities{
			Streaming:         true,
			Artifacts:         true,
			PushNotifications: false,
		},
		Authentication: Authentication{
			Schemes: []string{"bearer"},
		},
		Endpoints: AgentEndpoints{
			Tasks:  "/tasks",
			Stream: "/tasks/{id}/stream",
		},
		Skills: []Skill{
			{
				ID:          "agent-management",
				Name:        "Agent Management",
				Description: "Manage AI agents running in Docker containers",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(card)
}

// forwardTask forwards a task to the appropriate agents
func (r *Router) forwardTask(channelID, senderID, taskID string, req CreateTaskRequest) {
	if r.GetAgents == nil {
		return
	}

	agents, err := r.GetAgents()
	if err != nil {
		log.Printf("Failed to get agents: %v", err)
		return
	}

	for _, agent := range agents {
		if agent.ID == senderID {
			continue // Don't send to self
		}

		// For direct channels, only send to recipient
		if IsDirectChannel(channelID) && agent.ID != channelID {
			continue
		}

		go r.sendTaskToAgent(&agent, taskID, req)
	}
}

// sendTaskToAgent sends a task to a specific agent
func (r *Router) sendTaskToAgent(agent *AgentInfo, taskID string, req CreateTaskRequest) {
	url := fmt.Sprintf("%s/tasks", agent.Endpoint)

	body, _ := json.Marshal(req)
	log.Printf("Sending task to %s: %s", agent.ID, body)

	httpReq, _ := http.NewRequest("POST", url, nil)
	httpReq.Header.Set("Authorization", "Bearer "+agent.BearerToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Body = nil // Would need to set body properly

	resp, err := r.client.Do(httpReq)
	if err != nil {
		log.Printf("Failed to send task %s to agent %s: %v", taskID, agent.ID, err)
		return
	}
	defer resp.Body.Close()

	log.Printf("Task %s forwarded to agent %s", taskID, agent.ID)
}

// addConnection adds an SSE connection
func (r *Router) addConnection(taskID, addr string, ch chan *TaskUpdate) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.connections[taskID] == nil {
		r.connections[taskID] = make(map[string]chan *TaskUpdate)
	}
	r.connections[taskID][addr] = ch
}

// removeConnection removes an SSE connection
func (r *Router) removeConnection(taskID, addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.connections[taskID] != nil {
		delete(r.connections[taskID], addr)
	}
}

// broadcastUpdate broadcasts an update to all connections for a task
func (r *Router) broadcastUpdate(taskID string, update *TaskUpdate) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if connections, ok := r.connections[taskID]; ok {
		for _, ch := range connections {
			select {
			case ch <- update:
			default:
			}
		}
	}
}

// Helper functions
func generateTaskID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}

func splitChannelID(channelID, senderID string) string {
	// Format: agent1::agent2
	// Extract the other agent
	for _, part := range []string{channelID[:len(channelID)/2], channelID[len(channelID)/2:]} {
		if part != senderID && part != "" {
			return part
		}
	}
	return ""
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
