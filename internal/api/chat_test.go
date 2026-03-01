package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zeroclaw/bot-portal/internal/models"
	"github.com/zeroclaw/bot-portal/internal/store"
)

func TestHandleChatCompletions_ClaudeModel(t *testing.T) {
	// Create test router with in-memory database
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create a Claude model in database
	modelStore := store.NewModelStore(db)
	model := &models.Model{
		ID:              "claude-test",
		Name:            "Claude Test",
		Provider:        "anthropic",
		ModelIdentifier: "claude-opus-4-6",
		EndpointURL:     "https://api.anthropic.com/v1",
	}
	if err := modelStore.Create(model); err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}

	router := &Router{
		modelStore:      modelStore,
		authConfigStore: store.NewAuthConfigStore(db),
	}

	// Send OpenAI-format request with Claude model
	chatReq := ChatRequest{
		Model: "claude-test",
		Messages: []ChatMessage{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: 100,
		Stream:    false,
	}

	body, _ := json.Marshal(chatReq)
	req := httptest.NewRequest("POST", "/api/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.handleChatCompletions(w, req)

	// For now, expect 501 (transformation logic not yet implemented)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusNotImplemented)
	}
}
