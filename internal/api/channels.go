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
// Message Stream
// ============================================================================

func (r *Router) handleMessageStream(w http.ResponseWriter, req *http.Request) {
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
