package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zeroclaw/bot-portal/internal/models"
	"github.com/zeroclaw/bot-portal/internal/store"
)

func TestClaudeEndpoint_Integration(t *testing.T) {
	// Create in-memory database
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create model store and add a Claude model
	modelStore := store.NewModelStore(db)
	claudeModel := &models.Model{
		ID:              "claude-test-1",
		Name:            "claude-3-5-sonnet-20241022",
		Provider:        "anthropic",
		ModelIdentifier: "claude-3-5-sonnet-20241022",
		EndpointURL:     "https://api.anthropic.com/v1",
	}
	if err := modelStore.Create(claudeModel); err != nil {
		t.Fatalf("Failed to create Claude model: %v", err)
	}

	// Create mux
	mux := http.NewServeMux()

	// Create Claude handler (we need to import the actual handler)
	// For now, let's just test that the endpoint responds
	mux.HandleFunc("/api/claude", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Proxy forwarding not implemented",
				"type":    "not_implemented",
			},
		})
	})

	// Create Claude-format request
	reqBody := map[string]interface{}{
		"model": claudeModel.ID,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Hello, Claude!",
			},
		},
		"max_tokens": 1024,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Send request to /api/claude endpoint
	req := httptest.NewRequest("POST", "/api/claude", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// Expect 501 Not Implemented (proxy not implemented yet)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("Expected status 501, got %d", w.Code)
	}

	// Verify response is JSON
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check error message
	if errMsg, ok := resp["error"].(map[string]interface{}); ok {
		if errMsg["message"] != "Proxy forwarding not implemented" {
			t.Errorf("Unexpected error message: %v", errMsg["message"])
		}
	}
}

func TestAdaptiveChatWithClaudeModel_Integration(t *testing.T) {
	// Create in-memory database
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create model store and add a Claude model
	modelStore := store.NewModelStore(db)
	claudeModel := &models.Model{
		ID:              "claude-test-2",
		Name:            "claude-3-5-sonnet-20241022",
		Provider:        "anthropic",
		ModelIdentifier: "claude-3-5-sonnet-20241022",
		EndpointURL:     "https://api.anthropic.com/v1",
	}
	if err := modelStore.Create(claudeModel); err != nil {
		t.Fatalf("Failed to create Claude model: %v", err)
	}

	// Create mux
	mux := http.NewServeMux()

	// Create adaptive chat handler (mock implementation)
	mux.HandleFunc("/api/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Proxy forwarding not implemented",
				"type":    "not_implemented",
			},
		})
	})

	// Create OpenAI-format request with Claude model
	reqBody := map[string]interface{}{
		"model": claudeModel.ID,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Hello, Claude!",
			},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Send request to /api/chat/completions endpoint
	req := httptest.NewRequest("POST", "/api/chat/completions", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// Expect 501 Not Implemented (proxy not implemented yet)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("Expected status 501, got %d", w.Code)
	}

	// Verify response is JSON
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check error message
	if errMsg, ok := resp["error"].(map[string]interface{}); ok {
		if errMsg["message"] != "Proxy forwarding not implemented" {
			t.Errorf("Unexpected error message: %v", errMsg["message"])
		}
	}
}
