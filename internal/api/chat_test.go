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
			NewChatMessage("system", "You are helpful"),
			NewChatMessage("user", "Hello"),
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

func TestChatCompletions_ToolsPassthrough(t *testing.T) {
	req := ChatRequest{
		Model:      "test-model",
		Messages:   []ChatMessage{NewChatMessage("user", "Hello")},
		Tools:      json.RawMessage(`[{"type":"function","function":{"name":"test"}}]`),
		ToolChoice: json.RawMessage(`"auto"`),
		Stream:     true,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["tools"] == nil {
		t.Error("tools field missing from serialized request")
	}
	if result["tool_choice"] == nil {
		t.Error("tool_choice field missing from serialized request")
	}
	if result["stream"] != true {
		t.Error("stream field should be true")
	}
}

func TestChatCompletions_SystemPromptInjection(t *testing.T) {
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	router := &Router{
		agentStore:      store.NewAgentStore(db),
		modelStore:      store.NewModelStore(db),
		authConfigStore: store.NewAuthConfigStore(db),
	}

	t.Run("injects when no system message", func(t *testing.T) {
		messages := []ChatMessage{
			NewChatMessage("user", "Hello"),
		}
		result := router.injectSystemPrompt(messages)
		if len(result) != 2 {
			t.Fatalf("Expected 2 messages, got %d", len(result))
		}
		if result[0].Role != "system" {
			t.Errorf("Expected first message role 'system', got '%s'", result[0].Role)
		}
	})

	t.Run("skips when system message exists", func(t *testing.T) {
		messages := []ChatMessage{
			NewChatMessage("system", "Custom prompt"),
			NewChatMessage("user", "Hello"),
		}
		result := router.injectSystemPrompt(messages)
		if len(result) != 2 {
			t.Fatalf("Expected 2 messages (unchanged), got %d", len(result))
		}
		if result[0].ContentString() != "Custom prompt" {
			t.Errorf("Expected original system prompt preserved, got '%s'", result[0].ContentString())
		}
	})
}

func TestChatCompletions_ModelFallbackResolution(t *testing.T) {
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	modelStore := store.NewModelStore(db)

	if err := modelStore.Create(&models.Model{
		ID:              "model-1",
		Name:            "Test Model",
		Provider:        "openai",
		ModelIdentifier: "gpt-4o",
		IsDefault:       true,
	}); err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}

	router := &Router{
		modelStore: modelStore,
	}

	t.Run("resolves by ID", func(t *testing.T) {
		model, err := router.resolveModel("model-1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if model.ID != "model-1" {
			t.Errorf("Expected model-1, got %s", model.ID)
		}
	})

	t.Run("falls back to default when ID not found", func(t *testing.T) {
		model, err := router.resolveModel("nonexistent")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if model.ID != "model-1" {
			t.Errorf("Expected fallback to model-1, got %s", model.ID)
		}
	})

	t.Run("falls back to default when empty", func(t *testing.T) {
		model, err := router.resolveModel("")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if model.ID != "model-1" {
			t.Errorf("Expected fallback to model-1, got %s", model.ID)
		}
	})
}
