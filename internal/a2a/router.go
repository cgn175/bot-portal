package a2a

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Router handles A2A message routing
type Router struct {
	// Store interface for persistence
	CreateTask       func(channelID, senderID, recipientID string, message TaskMessage) (string, error)
	GetTask          func(id string) (*Task, error)
	UpdateTaskStatus func(id, status string) error
	AppendMessage    func(id string, message TaskMessage) error
	GetAgents        func() ([]AgentInfo, error)
	GetAgent         func(id string) (*AgentInfo, error)

	// HTTP client for forwarding tasks to agents
	client *http.Client

	// HTTP client for SSE streaming (no timeout)
	sseClient *http.Client

	// Semaphore to limit concurrent outgoing requests
	sendSem chan struct{}

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
		sseClient: &http.Client{
			Timeout: 0,
		},
		sendSem:     make(chan struct{}, 10),
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
		resp := CreateTaskResponse{}
		resp.Task.ID = taskID
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Fallback if no store configured
	http.Error(w, "store not configured", http.StatusInternalServerError)
}

// HandleTaskGet handles GET /tasks/{id}
func (r *Router) HandleTaskGet(w http.ResponseWriter, req *http.Request) {
	taskID, _ := req.Context().Value("taskID").(string)

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
	taskID, _ := req.Context().Value("taskID").(string)

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
	taskID, _ := req.Context().Value("taskID").(string)

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

// UpdateTaskRequest represents a request to update a task with a response
type UpdateTaskRequest struct {
	Message TaskMessage `json:"message"`
	Status  TaskStatus  `json:"status,omitempty"`
}

// HandleTaskUpdate handles POST /tasks/{id} - agents update tasks with responses
func (r *Router) HandleTaskUpdate(w http.ResponseWriter, req *http.Request) {
	taskID, _ := req.Context().Value("taskID").(string)

	var updateReq UpdateTaskRequest
	if err := json.NewDecoder(req.Body).Decode(&updateReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get sender from header
	senderID := req.Header.Get("X-Agent-ID")
	if senderID == "" {
		senderID = "unknown"
	}

	// Append the message to the task
	if r.AppendMessage != nil && updateReq.Message.Content != "" {
		if err := r.AppendMessage(taskID, updateReq.Message); err != nil {
			http.Error(w, fmt.Sprintf("failed to append message: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Update status if provided
	if r.UpdateTaskStatus != nil && updateReq.Status != "" {
		if err := r.UpdateTaskStatus(taskID, string(updateReq.Status)); err != nil {
			http.Error(w, fmt.Sprintf("failed to update status: %v", err), http.StatusInternalServerError)
			return
		}

		// Broadcast status update via SSE
		r.broadcastUpdate(taskID, &TaskUpdate{
			TaskID:  taskID,
			Status:  updateReq.Status,
			Message: &updateReq.Message,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"task_id": taskID,
		"status":  "updated",
	})
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

		agent := agent // capture loop variable
		r.sendSem <- struct{}{}
		go func() {
			defer func() { <-r.sendSem }()
			r.sendTaskToAgent(&agent, taskID, req)
		}()
	}
}

// sendTaskToAgent sends a task to a specific agent and subscribes to its SSE stream
func (r *Router) sendTaskToAgent(agent *AgentInfo, taskID string, req CreateTaskRequest) {
	// Step 1: Send the task to the agent
	url := fmt.Sprintf("%s/tasks", agent.Endpoint)

	body, _ := json.Marshal(req)
	log.Printf("Sending task to %s: %s", agent.ID, body)

	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Bearer "+agent.BearerToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Agent-ID", "portal")
	httpReq.Header.Set("X-Channel-ID", ChannelID("portal", agent.ID))

	resp, err := r.client.Do(httpReq)
	if err != nil {
		log.Printf("Failed to send task %s to agent %s: %v", taskID, agent.ID, err)
		return
	}
	defer resp.Body.Close()

	log.Printf("Task %s forwarded to agent %s, status: %d", taskID, agent.ID, resp.StatusCode)

	// Read response body for debugging
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("Agent %s returned error: %s", agent.ID, string(respBody))
		if r.UpdateTaskStatus != nil {
			r.UpdateTaskStatus(taskID, string(TaskStatusFailed))
		}
		return
	}

	// Step 2: Subscribe to the agent's SSE stream to receive responses
	go r.subscribeToAgentStream(agent, taskID)
}

// subscribeToAgentStream connects to the agent's SSE stream and processes updates
func (r *Router) subscribeToAgentStream(agent *AgentInfo, taskID string) {
	streamURL := fmt.Sprintf("%s/tasks/stream/%s", agent.Endpoint, taskID)
	log.Printf("Subscribing to SSE stream for task %s from agent %s", taskID, agent.ID)

	// Create request with headers
	req, err := http.NewRequest("GET", streamURL, nil)
	if err != nil {
		log.Printf("Failed to create SSE request for task %s: %v", taskID, err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+agent.BearerToken)
	req.Header.Set("Accept", "text/event-stream")

	// Make the request using the SSE client (no timeout)
	resp, err := r.sseClient.Do(req)
	if err != nil {
		log.Printf("Failed to connect to SSE stream for task %s: %v", taskID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("SSE stream returned status %d for task %s", resp.StatusCode, taskID)
		return
	}

	// Read SSE events
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("SSE stream error for task %s: %v", taskID, err)
			}
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue // Empty line between events
		}

		// Parse SSE data line
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			log.Printf("Received SSE event for task %s: %s", taskID, data)

			// Try to parse as TaskUpdate
			var update TaskUpdate
			if err := json.Unmarshal([]byte(data), &update); err == nil {
				// Update task status if provided
				if r.UpdateTaskStatus != nil && update.Status != "" {
					if err := r.UpdateTaskStatus(taskID, string(update.Status)); err != nil {
						log.Printf("Failed to update task status: %v", err)
					}
				}

				// Append message if provided
				if r.AppendMessage != nil && update.Message != nil && update.Message.Content != "" {
					if err := r.AppendMessage(taskID, *update.Message); err != nil {
						log.Printf("Failed to append message: %v", err)
					}
				}

				// Broadcast to portal's SSE clients
				r.broadcastUpdate(taskID, &update)

				// Stop if task is completed or failed
				if update.Status == TaskStatusCompleted || update.Status == TaskStatusFailed {
					log.Printf("Task %s finished with status: %s", taskID, update.Status)
					return
				}
			} else {
				log.Printf("Failed to parse SSE event: %v", err)
			}
		}
	}
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

// broadcastUpdate broadcasts an update to all SSE connections for a task
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
	parts := strings.SplitN(channelID, "::", 2)
	for _, part := range parts {
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
