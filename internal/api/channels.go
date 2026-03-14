package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

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

	remainder := path[len("/api/channels/"):]

	// Check if this is a messages request: /api/channels/{id}/messages
	if strings.HasSuffix(remainder, "/messages") {
		channelID := strings.TrimSuffix(remainder, "/messages")
		r.getChannelMessages(w, req, channelID)
		return
	}

	channelID := remainder

	// Also support query param: /api/channels/{id}?messages=true
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
// Message Stream (SSE)
// ============================================================================

func (r *Router) handleMessageStream(w http.ResponseWriter, req *http.Request) {
	channelID := req.URL.Query().Get("channel_id")

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

	if flusher == nil {
		return
	}

	// Subscribe to notifications for this channel
	ch := make(chan string, 4)
	addr := req.RemoteAddr
	r.addMsgStreamConn(channelID, addr, ch)
	defer r.removeMsgStreamConn(channelID, addr)

	notify := req.Context().Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-notify:
			return
		case updatedChannelID := <-ch:
			if channelID != "" {
				// Per-channel subscriber: send full message list
				r.sendChannelMessages(w, flusher, channelID)
			} else {
				// Global subscriber: send lightweight notification with channel_id
				data, _ := json.Marshal(map[string]string{"channel_id": updatedChannelID})
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func (r *Router) sendChannelMessages(w http.ResponseWriter, flusher http.Flusher, channelID string) {
	messages, err := r.messageStore.ListByChannel(channelID, 100)
	if err != nil {
		return
	}
	data, err := json.Marshal(messages)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func (r *Router) addMsgStreamConn(channelID, addr string, ch chan string) {
	r.msgStreamMu.Lock()
	defer r.msgStreamMu.Unlock()
	if r.msgStreamConns[channelID] == nil {
		r.msgStreamConns[channelID] = make(map[string]chan string)
	}
	r.msgStreamConns[channelID][addr] = ch
}

func (r *Router) removeMsgStreamConn(channelID, addr string) {
	r.msgStreamMu.Lock()
	defer r.msgStreamMu.Unlock()
	if conns, ok := r.msgStreamConns[channelID]; ok {
		delete(conns, addr)
		if len(conns) == 0 {
			delete(r.msgStreamConns, channelID)
		}
	}
}

// notifyMsgStream signals all SSE clients subscribed to a channel that new data is available.
func (r *Router) notifyMsgStream(channelID string) {
	r.msgStreamMu.RLock()
	defer r.msgStreamMu.RUnlock()

	// Notify per-channel subscribers
	if conns, ok := r.msgStreamConns[channelID]; ok {
		for _, ch := range conns {
			select {
			case ch <- channelID:
			default:
			}
		}
	}

	// Notify global subscribers (no channel_id filter) with the channelID that changed
	if channelID != "" {
		if conns, ok := r.msgStreamConns[""]; ok {
			for _, ch := range conns {
				select {
				case ch <- channelID:
				default:
				}
			}
		}
	}
}
