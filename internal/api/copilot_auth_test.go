package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/zeroclaw/bot-portal/internal/store"
)

func TestCopilotAuthAPI(t *testing.T) {
	// Set up test encryption key before running tests (must be exactly 32 bytes)
	os.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")

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

	// Test POST /api/auth/copilot/device-code endpoint (this will fail in tests without mocking GitHub)
	t.Run("POST /api/auth/copilot/device-code method not allowed", func(t *testing.T) {
		// Test with wrong method
		req := httptest.NewRequest(http.MethodGet, "/api/auth/copilot/device-code", nil)
		rr := httptest.NewRecorder()
		router.handleCopilotDeviceCode(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", rr.Code)
		}
	})

	t.Run("POST /api/auth/copilot/device-code calls GitHub", func(t *testing.T) {
		// The device-code handler does not parse the request body;
		// it calls GitHub directly and returns 500 if the call fails,
		// or 200 with the device code info on success.
		req := httptest.NewRequest(http.MethodPost, "/api/auth/copilot/device-code", nil)
		rr := httptest.NewRecorder()
		router.handleCopilotDeviceCode(rr, req)

		// Accept either 200 (GitHub reachable) or 500 (GitHub unreachable in test env)
		if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 200 or 500, got %d", rr.Code)
		}
	})

	t.Run("POST /api/auth/copilot/token method not allowed", func(t *testing.T) {
		// Test with wrong method
		req := httptest.NewRequest(http.MethodGet, "/api/auth/copilot/token", nil)
		rr := httptest.NewRecorder()
		router.handleCopilotToken(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", rr.Code)
		}
	})

	t.Run("POST /api/auth/copilot/token missing device_code", func(t *testing.T) {
		request := map[string]string{
			"configId": "test-config",
		}

		body, _ := json.Marshal(request)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/copilot/token", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.handleCopilotToken(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("POST /api/auth/copilot/token bad request", func(t *testing.T) {
		// Test with invalid JSON
		req := httptest.NewRequest(http.MethodPost, "/api/auth/copilot/token", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.handleCopilotToken(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rr.Code)
		}
	})
}

func TestRequestGitHubDeviceCode(t *testing.T) {
	// This test would require mocking the GitHub API
	// For now, we just verify the function exists and has proper signature
	// In a real test environment, you would use a mock server

	// Since we can't actually call GitHub's API in unit tests,
	// we'll skip this test unless explicitly enabled
	if os.Getenv("GITHUB_OAUTH_TEST") != "1" {
		t.Skip("Skipping GitHub OAuth test. Set GITHUB_OAUTH_TEST=1 to enable.")
	}

	resp, err := requestGitHubDeviceCode()
	if err != nil {
		t.Fatalf("Failed to request device code: %v", err)
	}

	if resp.DeviceCode == "" {
		t.Error("Expected device_code to be non-empty")
	}
	if resp.UserCode == "" {
		t.Error("Expected user_code to be non-empty")
	}
	if resp.VerificationURI == "" {
		t.Error("Expected verification_uri to be non-empty")
	}
	if resp.ExpiresIn == 0 {
		t.Error("Expected expires_in to be non-zero")
	}
	if resp.Interval == 0 {
		t.Error("Expected interval to be non-zero")
	}
}

func TestPollGitHubAccessToken(t *testing.T) {
	// This test would require mocking the GitHub API
	// For now, we just verify the function exists and has proper signature

	// Since we can't actually call GitHub's API in unit tests,
	// we'll skip this test unless explicitly enabled
	if os.Getenv("GITHUB_OAUTH_TEST") != "1" {
		t.Skip("Skipping GitHub OAuth test. Set GITHUB_OAUTH_TEST=1 to enable.")
	}

	// First get a device code
	deviceResp, err := requestGitHubDeviceCode()
	if err != nil {
		t.Fatalf("Failed to request device code: %v", err)
	}

	// Poll for token (this will likely return authorization_pending)
	tokenResp, err := pollGitHubAccessToken(deviceResp.DeviceCode)
	if err != nil {
		t.Fatalf("Failed to poll for token: %v", err)
	}

	// We expect either an error (authorization_pending) or a token
	if tokenResp.Error == "" && tokenResp.AccessToken == "" {
		t.Error("Expected either error or access_token to be set")
	}
}

func TestDeviceCodeRequestStructure(t *testing.T) {
	// Test that our request/response structures are properly defined
	req := DeviceCodeRequest{
		ConfigID: "test-config",
		Name:     "Test Config",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal DeviceCodeRequest: %v", err)
	}

	var decoded DeviceCodeRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal DeviceCodeRequest: %v", err)
	}

	if decoded.ConfigID != req.ConfigID {
		t.Errorf("Expected ConfigID %s, got %s", req.ConfigID, decoded.ConfigID)
	}
	if decoded.Name != req.Name {
		t.Errorf("Expected Name %s, got %s", req.Name, decoded.Name)
	}
}

func TestTokenRequestStructure(t *testing.T) {
	// Test that our request/response structures are properly defined
	req := TokenRequest{
		DeviceCode: "test-device-code",
		ConfigID:   "test-config",
		Name:       "Test Config",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal TokenRequest: %v", err)
	}

	var decoded TokenRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal TokenRequest: %v", err)
	}

	if decoded.DeviceCode != req.DeviceCode {
		t.Errorf("Expected DeviceCode %s, got %s", req.DeviceCode, decoded.DeviceCode)
	}
	if decoded.ConfigID != req.ConfigID {
		t.Errorf("Expected ConfigID %s, got %s", req.ConfigID, decoded.ConfigID)
	}
	if decoded.Name != req.Name {
		t.Errorf("Expected Name %s, got %s", req.Name, decoded.Name)
	}
}

func TestDeviceCodeResultStructure(t *testing.T) {
	// Test that our result structures are properly defined
	result := DeviceCodeResult{
		DeviceCode:      "test-device-code",
		UserCode:        "ABCD-EFGH",
		VerificationURI: "https://github.com/login/device",
		ExpiresIn:       900,
		Interval:        5,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal DeviceCodeResult: %v", err)
	}

	var decoded DeviceCodeResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal DeviceCodeResult: %v", err)
	}

	if decoded.DeviceCode != result.DeviceCode {
		t.Errorf("Expected DeviceCode %s, got %s", result.DeviceCode, decoded.DeviceCode)
	}
	if decoded.UserCode != result.UserCode {
		t.Errorf("Expected UserCode %s, got %s", result.UserCode, decoded.UserCode)
	}
	if decoded.VerificationURI != result.VerificationURI {
		t.Errorf("Expected VerificationURI %s, got %s", result.VerificationURI, decoded.VerificationURI)
	}
	if decoded.ExpiresIn != result.ExpiresIn {
		t.Errorf("Expected ExpiresIn %d, got %d", result.ExpiresIn, decoded.ExpiresIn)
	}
	if decoded.Interval != result.Interval {
		t.Errorf("Expected Interval %d, got %d", result.Interval, decoded.Interval)
	}
}

func TestTokenResultStructure(t *testing.T) {
	// Test that our result structures are properly defined
	// Note: TokenResult no longer includes AccessToken for security
	// The token is stored server-side in AuthConfig
	result := TokenResult{
		Success:  true,
		Message:  "Token saved successfully",
		ConfigID: "test-config",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal TokenResult: %v", err)
	}

	var decoded TokenResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal TokenResult: %v", err)
	}

	if decoded.Success != result.Success {
		t.Errorf("Expected Success %v, got %v", result.Success, decoded.Success)
	}
	if decoded.Message != result.Message {
		t.Errorf("Expected Message %s, got %s", result.Message, decoded.Message)
	}
	if decoded.ConfigID != result.ConfigID {
		t.Errorf("Expected ConfigID %s, got %s", result.ConfigID, decoded.ConfigID)
	}
}
