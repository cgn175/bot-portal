package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zeroclaw/bot-portal/internal/a2a"
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

func (r *Router) createTaskWithID(id, channelID, senderID, recipientID string, message a2a.TaskMessage) (string, error) {
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

	return r.messageStore.Update(taskLog)
}

func (r *Router) appendMessage(id string, message a2a.TaskMessage) error {
	return r.messageStore.AppendMessage(id, message)
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
