package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zeroclaw/bot-portal/internal/a2a"
	"github.com/zeroclaw/bot-portal/internal/models"
	"github.com/zeroclaw/bot-portal/internal/store"
)

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
		} else {
			// Handle task update (agents sending responses)
			r.a2aRouter.HandleTaskUpdate(w, req)
		}
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ============================================================================
// Frontend Task Stream (SSE for chat task updates)
// ============================================================================

func (r *Router) handleTaskStream(w http.ResponseWriter, req *http.Request) {
	// Parse /api/tasks/{taskID}/stream
	path := strings.TrimPrefix(req.URL.Path, "/api/tasks/")
	if !strings.HasSuffix(path, "/stream") {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	taskID := strings.TrimSuffix(path, "/stream")
	if taskID == "" {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		if rr, ok := w.(*responseRecorder); ok {
			if f, ok := rr.ResponseWriter.(http.Flusher); ok {
				flusher = f
			}
		}
	}
	if flusher == nil {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := make(chan *a2a.TaskUpdate, 10)
	addr := req.RemoteAddr
	r.addTaskStreamConn(taskID, addr, ch)
	defer r.removeTaskStreamConn(taskID, addr)

	notify := req.Context().Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-notify:
			return
		case update := <-ch:
			data, _ := json.Marshal(update)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func (r *Router) addTaskStreamConn(taskID, addr string, ch chan *a2a.TaskUpdate) {
	r.taskStreamMu.Lock()
	defer r.taskStreamMu.Unlock()
	if r.taskStreamConns[taskID] == nil {
		r.taskStreamConns[taskID] = make(map[string]chan *a2a.TaskUpdate)
	}
	r.taskStreamConns[taskID][addr] = ch
}

func (r *Router) removeTaskStreamConn(taskID, addr string) {
	r.taskStreamMu.Lock()
	defer r.taskStreamMu.Unlock()
	if conns, ok := r.taskStreamConns[taskID]; ok {
		delete(conns, addr)
		if len(conns) == 0 {
			delete(r.taskStreamConns, taskID)
		}
	}
}

func (r *Router) broadcastTaskUpdate(taskID string, update *a2a.TaskUpdate) {
	r.taskStreamMu.RLock()
	defer r.taskStreamMu.RUnlock()
	if conns, ok := r.taskStreamConns[taskID]; ok {
		for _, ch := range conns {
			select {
			case ch <- update:
			default:
			}
		}
	}
}

// ============================================================================
// Helper methods for A2A Router
// ============================================================================

func (r *Router) createTask(channelID, senderID, recipientID string, message a2a.TaskMessage) (string, error) {
	id := fmt.Sprintf("task-%d", time.Now().UnixNano())
	return r.createTaskWithID(id, channelID, senderID, recipientID, message)
}

// ensureChannel creates the channel row if it doesn't exist yet,
// so the Messages page can list it.
func (r *Router) ensureChannel(channelID, senderID, recipientID string) {
	existing, _ := r.channelStore.GetByID(channelID)
	if existing != nil {
		return
	}
	members := []string{}
	if senderID != "" {
		members = append(members, senderID)
	}
	if recipientID != "" && recipientID != senderID {
		members = append(members, recipientID)
	}
	ch := &models.Channel{
		ID:        channelID,
		Members:   members,
		CreatedAt: time.Now(),
	}
	if err := r.channelStore.Create(ch); err != nil {
		log.Printf("Warning: failed to ensure channel %s: %v", channelID, err)
	}
}

func (r *Router) createTaskWithID(id, channelID, senderID, recipientID string, message a2a.TaskMessage) (string, error) {
	// Ensure the channel exists so it shows up in the Messages page
	r.ensureChannel(channelID, senderID, recipientID)

	taskLog := &store.TaskLog{
		ID:          id,
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

	r.notifyMsgStream(channelID)
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
	if err := json.Unmarshal(taskLog.Messages, &task.Messages); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task messages: %w", err)
	}
	if err := json.Unmarshal(taskLog.Artifacts, &task.Artifacts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task artifacts: %w", err)
	}

	return task, nil
}

func (r *Router) updateTaskStatus(id, status string) error {
	taskLog, err := r.messageStore.GetByID(id)
	if err != nil || taskLog == nil {
		return err
	}

	taskLog.Status = status
	taskLog.UpdatedAt = time.Now()

	if err := r.messageStore.Update(taskLog); err != nil {
		return err
	}
	r.notifyMsgStream(taskLog.ChannelID)
	return nil
}

func (r *Router) appendMessage(id string, message a2a.TaskMessage) error {
	taskLog, err := r.messageStore.GetByID(id)
	if err != nil || taskLog == nil {
		return err
	}
	if err := r.messageStore.AppendMessage(id, message); err != nil {
		return err
	}
	r.notifyMsgStream(taskLog.ChannelID)
	return nil
}

// subscribeToAgentSSE connects to the agent's SSE stream and processes updates
func (r *Router) subscribeToAgentSSE(agent *store.Agent, taskID string) {
	streamURL := fmt.Sprintf("%s/tasks/%s/stream", agent.Endpoint, taskID)
	log.Printf("Subscribing to SSE stream for task %s from agent %s", taskID, agent.ID)

	req, err := http.NewRequest("GET", streamURL, nil)
	if err != nil {
		log.Printf("Failed to create SSE request for task %s: %v", taskID, err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+agent.BearerToken)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 0} // No timeout for SSE
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to connect to SSE stream for task %s: %v", taskID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("SSE stream returned status %d for task %s", resp.StatusCode, taskID)
		return
	}

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
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			log.Printf("Received SSE event for task %s: %s", taskID, data)

			var update a2a.TaskUpdate
			if err := json.Unmarshal([]byte(data), &update); err == nil {
				if update.Status != "" {
					r.updateTaskStatus(taskID, string(update.Status))
				}
				if update.Message != nil && update.Message.Content != "" {
					r.appendMessage(taskID, *update.Message)
				}
				// Broadcast to frontend SSE clients
				r.broadcastTaskUpdate(taskID, &update)
				if update.Status == a2a.TaskStatusCompleted || update.Status == a2a.TaskStatusFailed {
					log.Printf("Task %s finished with status: %s", taskID, update.Status)
					return
				}
			}
		}
	}
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

// ============================================================================
// A2A Relay Handler — Hub-and-Spoke Inter-Agent Communication
// ============================================================================

// handleA2ARelay intercepts agent-to-agent messages so they flow through the portal.
// URL pattern: /a2a/relay/{recipientID}/tasks
//
// The agent's a2a_send tool thinks it's talking directly to a peer, but the
// peer endpoint in its config actually points here. The portal:
//  1. Identifies the sender (from bearer token, set by requireBearerToken middleware)
//  2. Extracts the recipient from the URL path
//  3. Logs the message in task_logs (visible in the UI)
//  4. Forwards the task to the real recipient agent
//  5. Subscribes to the recipient's SSE stream and relays responses back
func (r *Router) handleA2ARelay(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse /a2a/relay/{recipientID}/tasks
	path := strings.TrimPrefix(req.URL.Path, "/a2a/relay/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] != "tasks" {
		http.Error(w, "Invalid relay path. Expected /a2a/relay/{recipientID}/tasks", http.StatusBadRequest)
		return
	}
	recipientID := parts[0]

	// Sender is auto-set by requireBearerToken middleware
	senderID := req.Header.Get("X-Agent-ID")
	if senderID == "" {
		http.Error(w, "Could not identify sender", http.StatusUnauthorized)
		return
	}

	// Parse the task creation request
	var createReq a2a.CreateTaskRequest
	if err := json.NewDecoder(req.Body).Decode(&createReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Look up the recipient agent
	recipient, err := r.agentStore.GetByID(recipientID)
	if err != nil || recipient == nil {
		http.Error(w, fmt.Sprintf("Recipient agent %q not found", recipientID), http.StatusNotFound)
		return
	}

	// Build a direct channel ID between sender and recipient
	channelID := a2a.ChannelID(senderID, recipientID)

	log.Printf("[A2A Relay] %s → %s (channel: %s) message: %s",
		senderID, recipientID, channelID, createReq.Message.Content)

	// 1. Log the message in task_logs (this makes it visible in the frontend)
	taskID, err := r.createTask(channelID, senderID, recipientID, createReq.Message)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create task: %v", err), http.StatusInternalServerError)
		return
	}

	// 2. Look up sender agent so we can forward the response back later
	sender, err := r.agentStore.GetByID(senderID)
	if err != nil || sender == nil {
		log.Printf("[A2A Relay] Warning: sender %q not found, response won't be forwarded back", senderID)
	}

	// 3. Forward to the real recipient agent asynchronously.
	//    When agentB responds, the goroutine will forward the response back
	//    to agentA via its /tasks endpoint.
	go r.relayToRecipient(recipient, taskID, createReq, senderID, sender)

	// 4. Return task_id immediately to the sender (same response format as direct A2A)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a2a.CreateTaskResponse{TaskID: taskID})
}

// relayToRecipient forwards a relayed task to the actual recipient agent,
// subscribes to its SSE stream, and when the response arrives, forwards it
// back to the sender agent via its /tasks endpoint.
func (r *Router) relayToRecipient(recipient *store.Agent, portalTaskID string, req a2a.CreateTaskRequest, senderID string, sender *store.Agent) {
	if recipient.Endpoint == "" {
		log.Printf("[A2A Relay] Recipient %s has no endpoint, cannot forward", recipient.ID)
		r.updateTaskStatus(portalTaskID, string(a2a.TaskStatusFailed))
		return
	}

	url := fmt.Sprintf("%s/tasks", recipient.Endpoint)
	body, _ := json.Marshal(req)

	log.Printf("[A2A Relay] Forwarding task %s to %s at %s", portalTaskID, recipient.ID, url)

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		log.Printf("[A2A Relay] Failed to create request: %v", err)
		r.updateTaskStatus(portalTaskID, string(a2a.TaskStatusFailed))
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+recipient.BearerToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Agent-ID", senderID)
	httpReq.Header.Set("X-Channel-ID", a2a.ChannelID(senderID, recipient.ID))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[A2A Relay] Failed to forward task %s to %s: %v", portalTaskID, recipient.ID, err)
		r.updateTaskStatus(portalTaskID, string(a2a.TaskStatusFailed))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[A2A Relay] Recipient %s returned error %d: %s", recipient.ID, resp.StatusCode, string(respBody))
		r.updateTaskStatus(portalTaskID, string(a2a.TaskStatusFailed))
		return
	}

	// Parse the agent's response to get its task ID.
	// The agent returns {"task":{"id":"...","status":"pending",...}} (ZeroClaw A2A format),
	// not {"task_id":"..."} — we must extract the nested task.id.
	var agentResp struct {
		Task struct {
			ID string `json:"id"`
		} `json:"task"`
		TaskID string `json:"task_id"` // fallback: some agents may use flat format
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[A2A Relay] Failed to read response from %s: %v", recipient.ID, err)
		r.updateTaskStatus(portalTaskID, string(a2a.TaskStatusFailed))
		return
	}
	if err := json.Unmarshal(respBody, &agentResp); err != nil {
		log.Printf("[A2A Relay] Failed to parse response from %s: %s", recipient.ID, string(respBody))
		r.updateTaskStatus(portalTaskID, string(a2a.TaskStatusFailed))
		return
	}
	agentTaskID := agentResp.Task.ID
	if agentTaskID == "" {
		agentTaskID = agentResp.TaskID // fallback to flat format
	}
	if agentTaskID == "" {
		log.Printf("[A2A Relay] No task ID in response from %s: %s", recipient.ID, string(respBody))
		r.updateTaskStatus(portalTaskID, string(a2a.TaskStatusFailed))
		return
	}
	log.Printf("[A2A Relay] Task forwarded to %s: portal=%s agent=%s", recipient.ID, portalTaskID, agentTaskID)

	// Subscribe to the agent's SSE stream using the AGENT's task ID,
	// but write updates back to our portal DB using portalTaskID.
	r.subscribeToRelayedAgentSSE(recipient, agentTaskID, portalTaskID, sender)
}

// subscribeToRelayedAgentSSE connects to the agent's SSE stream using the agent's
// own task ID, but persists updates and broadcasts to the frontend using the portal's
// task ID. This bridges the two ID spaces.
// When the recipient completes, it forwards the response back to the sender agent.
func (r *Router) subscribeToRelayedAgentSSE(agent *store.Agent, agentTaskID, portalTaskID string, sender *store.Agent) {
	streamURL := fmt.Sprintf("%s/tasks/%s/stream", agent.Endpoint, agentTaskID)
	log.Printf("[A2A Relay] Subscribing to SSE: agent=%s agentTask=%s portalTask=%s", agent.ID, agentTaskID, portalTaskID)

	req, err := http.NewRequest("GET", streamURL, nil)
	if err != nil {
		log.Printf("[A2A Relay] Failed to create SSE request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+agent.BearerToken)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 0} // No timeout for SSE
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[A2A Relay] Failed to connect to SSE stream: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[A2A Relay] SSE stream returned status %d for agentTask=%s", resp.StatusCode, agentTaskID)
		return
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("[A2A Relay] SSE stream error for agentTask=%s: %v", agentTaskID, err)
			}
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			log.Printf("[A2A Relay] SSE event agentTask=%s → portalTask=%s: %s", agentTaskID, portalTaskID, data)

			var update a2a.TaskUpdate
			if err := json.Unmarshal([]byte(data), &update); err == nil {
				// Update status on the portal task
				if update.Status != "" {
					r.updateTaskStatus(portalTaskID, string(update.Status))
				}
				// Forward the response back to the sender agent.
				// forwardResponseToSender creates a new task_log entry for the
				// response so it appears as a visible message in the frontend.
				if update.Message != nil && update.Message.Content != "" {
					if sender != nil && sender.Endpoint != "" {
						r.forwardResponseToSender(sender, agent.ID, *update.Message)
					}
				}
				// Broadcast to frontend using portal's task ID
				update.TaskID = portalTaskID
				r.broadcastTaskUpdate(portalTaskID, &update)
				if update.Status == a2a.TaskStatusCompleted || update.Status == a2a.TaskStatusFailed {
					log.Printf("[A2A Relay] Task finished: portalTask=%s status=%s", portalTaskID, update.Status)
					return
				}
			}
		}
	}
}

// forwardResponseToSender sends a recipient's response back to the sender agent
// by POSTing a new task to the sender's /tasks endpoint, then subscribes to
// the sender's SSE stream so any follow-up reply is captured and relayed back.
func (r *Router) forwardResponseToSender(sender *store.Agent, recipientID string, message a2a.TaskMessage) {
	channelID := a2a.ChannelID(recipientID, sender.ID)

	// Log the response as a separate task_log entry so it appears
	// as a visible message in the frontend channel view.
	portalTaskID, err := r.createTask(channelID, recipientID, sender.ID, message)
	if err != nil {
		log.Printf("[A2A Relay] Failed to log response task: %v", err)
		return
	}

	// Forward to the sender agent
	url := fmt.Sprintf("%s/tasks", sender.Endpoint)
	createReq := a2a.CreateTaskRequest{Message: message}
	body, _ := json.Marshal(createReq)

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		log.Printf("[A2A Relay] Failed to create forward request to sender %s: %v", sender.ID, err)
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+sender.BearerToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Agent-ID", recipientID)
	httpReq.Header.Set("X-Channel-ID", channelID)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[A2A Relay] Failed to forward response to sender %s: %v", sender.ID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[A2A Relay] Sender %s returned error %d: %s", sender.ID, resp.StatusCode, string(respBody))
		return
	}

	// Parse the sender's task ID from the response
	var agentResp struct {
		Task struct {
			ID string `json:"id"`
		} `json:"task"`
		TaskID string `json:"task_id"`
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[A2A Relay] Failed to read response from sender %s: %v", sender.ID, err)
		return
	}
	if err := json.Unmarshal(respBody, &agentResp); err != nil {
		log.Printf("[A2A Relay] Failed to parse response from sender %s: %s", sender.ID, string(respBody))
		return
	}
	agentTaskID := agentResp.Task.ID
	if agentTaskID == "" {
		agentTaskID = agentResp.TaskID
	}
	if agentTaskID == "" {
		log.Printf("[A2A Relay] No task ID in response from sender %s: %s", sender.ID, string(respBody))
		return
	}

	log.Printf("[A2A Relay] Response forwarded to sender %s from %s (portalTask=%s agentTask=%s)", sender.ID, recipientID, portalTaskID, agentTaskID)

	// Subscribe to the sender's SSE stream to capture any follow-up reply.
	// The recipient becomes the "forward-to" agent for the next leg.
	recipient, _ := r.agentStore.GetByID(recipientID)
	r.subscribeToRelayedAgentSSE(sender, agentTaskID, portalTaskID, recipient)
}
