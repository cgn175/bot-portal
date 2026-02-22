package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zeroclaw/bot-portal/internal/models"
	"github.com/zeroclaw/bot-portal/internal/store"
)

func TestAuthConfigsAPI(t *testing.T) {
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

	// Test POST /api/auth-configs endpoint
	t.Run("POST /api/auth-configs", func(t *testing.T) {
		config := map[string]interface{}{
			"id":          "test-auth-config",
			"name":        "Test Auth Config",
			"provider":    "openai",
			"authType":    "bearer_token",
			"credentials": map[string]string{
				"api_key": "sk-test-secret-key",
			},
			"endpointUrl": "https://api.openai.com/v1",
		}

		body, _ := json.Marshal(config)
		req := httptest.NewRequest(http.MethodPost, "/api/auth-configs", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.handleAuthConfigs(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d: %s", rr.Code, rr.Body.String())
		}

		// Verify response contains the created auth config with masked credentials
		var response models.AuthConfig
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}

		if response.ID != "test-auth-config" {
			t.Errorf("Expected ID 'test-auth-config', got %s", response.ID)
		}
		if response.Name != "Test Auth Config" {
			t.Errorf("Expected name 'Test Auth Config', got %s", response.Name)
		}

		// Verify credentials are masked in the response
		if !strings.Contains(response.Credentials, "***masked***") {
			t.Errorf("Expected credentials to be masked with '***masked***', got %s", response.Credentials)
		}
	})

	// Test GET /api/auth-configs endpoint
	t.Run("GET /api/auth-configs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/auth-configs", nil)
		rr := httptest.NewRecorder()
		router.handleAuthConfigs(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var response []models.AuthConfig
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}

		if len(response) != 1 {
			t.Errorf("Expected 1 auth config, got %d", len(response))
		}

		// Verify credentials are masked in list response
		if len(response) > 0 && !strings.Contains(response[0].Credentials, "***masked***") {
			t.Errorf("Expected credentials to be masked in list response, got %s", response[0].Credentials)
		}
	})

	// Test GET /api/auth-configs/{id} endpoint
	t.Run("GET /api/auth-configs/{id}", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/auth-configs/test-auth-config", nil)
		rr := httptest.NewRecorder()
		router.handleAuthConfigDetail(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var response models.AuthConfig
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}

		if response.ID != "test-auth-config" {
			t.Errorf("Expected ID 'test-auth-config', got %s", response.ID)
		}

		// Verify credentials are masked
		if !strings.Contains(response.Credentials, "***masked***") {
			t.Errorf("Expected credentials to be masked, got %s", response.Credentials)
		}
	})

	// Test PUT /api/auth-configs/{id} endpoint
	t.Run("PUT /api/auth-configs/{id}", func(t *testing.T) {
		updates := map[string]interface{}{
			"name": "Updated Auth Config",
			"credentials": map[string]string{
				"api_key": "sk-new-secret-key",
			},
		}

		body, _ := json.Marshal(updates)
		req := httptest.NewRequest(http.MethodPut, "/api/auth-configs/test-auth-config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.handleAuthConfigDetail(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var response models.AuthConfig
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}

		if response.Name != "Updated Auth Config" {
			t.Errorf("Expected name 'Updated Auth Config', got %s", response.Name)
		}

		// Verify credentials are masked in the response
		if !strings.Contains(response.Credentials, "***masked***") {
			t.Errorf("Expected credentials to be masked after update, got %s", response.Credentials)
		}
	})

	// Test DELETE /api/auth-configs/{id} endpoint
	t.Run("DELETE /api/auth-configs/{id}", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/auth-configs/test-auth-config", nil)
		rr := httptest.NewRecorder()
		router.handleAuthConfigDetail(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", rr.Code)
		}

		// Verify auth config is deleted
		req = httptest.NewRequest(http.MethodGet, "/api/auth-configs/test-auth-config", nil)
		rr = httptest.NewRecorder()
		router.handleAuthConfigDetail(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for deleted auth config, got %d", rr.Code)
		}
	})
}
