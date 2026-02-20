package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type AgentCard struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type TaskRequest struct {
	ChannelID string `json:"channel_id"`
	SenderID  string `json:"sender_id"`
	Message   struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
}

func main() {
	agentID := os.Getenv("AGENT_ID")
	if agentID == "" {
		agentID = "mock-agent"
	}

	http.HandleFunc("/.well-known/agent.json", func(w http.ResponseWriter, r *http.Request) {
		card := AgentCard{
			Name:        agentID,
			Description: "Mock agent for testing",
			Version:     "1.0.0",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(card)
	})

	http.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req TaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("Received task from %s: %s", req.SenderID, req.Message.Content)

		response := map[string]string{
			"task_id": "mock-task-123",
			"status":  "accepted",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Mock agent %s starting on port %s", agentID, port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
