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

func TestModelsAPI(t *testing.T) {
	// Setup test database using store package
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Run migrations to create tables
	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create router with nil docker manager (we won't test Docker-related functionality)
	router := &Router{
		db:              db,
		agentStore:      store.NewAgentStore(db),
		channelStore:    store.NewChannelStore(db),
		messageStore:    store.NewMessageStore(db),
		modelStore:      store.NewModelStore(db),
		authConfigStore: store.NewAuthConfigStore(db),
	}

	// Test POST /api/models endpoint
	t.Run("POST /api/models", func(t *testing.T) {
		model := map[string]interface{}{
			"id":       "test-model",
			"name":     "Test Model",
			"provider": "openai",
			"modelName": "gpt-4",
			"apiKeyConfig": map[string]string{
				"OPENAI_API_KEY": "sk-test-key",
			},
			"baseUrl": "https://api.openai.com/v1",
		}

		body, _ := json.Marshal(model)
		req := httptest.NewRequest(http.MethodPost, "/api/models", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.handleModels(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", rr.Code)
		}

		// Verify response contains the created model
		var response models.Model
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}

		if response.ID != "test-model" {
			t.Errorf("Expected ID 'test-model', got %s", response.ID)
		}
		if response.Name != "Test Model" {
			t.Errorf("Expected name 'Test Model', got %s", response.Name)
		}
	})

	// Test GET /api/models endpoint
	t.Run("GET /api/models", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
		rr := httptest.NewRecorder()
		router.handleModels(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var response []models.Model
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}

		if len(response) != 1 {
			t.Errorf("Expected 1 model, got %d", len(response))
		}
	})

	// Test GET /api/models/{id} endpoint
	t.Run("GET /api/models/{id}", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/models/test-model", nil)
		rr := httptest.NewRecorder()
		router.handleModelDetail(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var response models.Model
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}

		if response.ID != "test-model" {
			t.Errorf("Expected ID 'test-model', got %s", response.ID)
		}
	})

	// Test PUT /api/models/{id} endpoint
	t.Run("PUT /api/models/{id}", func(t *testing.T) {
		updates := map[string]interface{}{
			"name": "Updated Test Model",
		}

		body, _ := json.Marshal(updates)
		req := httptest.NewRequest(http.MethodPut, "/api/models/test-model", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.handleModelDetail(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var response models.Model
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}

		if response.Name != "Updated Test Model" {
			t.Errorf("Expected name 'Updated Test Model', got %s", response.Name)
		}
	})

	// Test DELETE /api/models/{id} endpoint
	t.Run("DELETE /api/models/{id}", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/models/test-model", nil)
		rr := httptest.NewRecorder()
		router.handleModelDetail(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", rr.Code)
		}

		// Verify model is deleted
		req = httptest.NewRequest(http.MethodGet, "/api/models/test-model", nil)
		rr = httptest.NewRecorder()
		router.handleModelDetail(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for deleted model, got %d", rr.Code)
		}
	})
}
