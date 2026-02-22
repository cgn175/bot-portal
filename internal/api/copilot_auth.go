package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zeroclaw/bot-portal/internal/models"
)

// GitHub OAuth Device Flow constants
const (
	GitHubDeviceCodeURL     = "https://github.com/login/device/code"
	GitHubAccessTokenURL    = "https://github.com/login/oauth/access_token"
	GitHubCopilotClientID   = "Iv1.3f455342d838a8f6" // GitHub Copilot OAuth App client ID
	GitHubCopilotProvider   = "github_copilot"
	GitHubCopilotAuthType   = "github_copilot_oauth"
)

// DeviceCodeResponse represents the response from GitHub's device code endpoint
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// AccessTokenResponse represents the response from GitHub's access token endpoint
type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error,omitempty"`
	ErrorDesc   string `json:"error_description,omitempty"`
}

// DeviceCodeRequest represents the request to initiate device flow
type DeviceCodeRequest struct {
	ConfigID string `json:"configId"`
	Name     string `json:"name"`
}

// DeviceCodeResult represents the result returned to the client
type DeviceCodeResult struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenRequest represents the request to poll for token
type TokenRequest struct {
	DeviceCode string `json:"device_code"`
	ConfigID   string `json:"configId,omitempty"`
	Name       string `json:"name,omitempty"`
}

// TokenResult represents the result of the token polling
type TokenResult struct {
	Success     bool   `json:"success"`
	Message     string `json:"message,omitempty"`
	ConfigID    string `json:"configId,omitempty"`
	AccessToken string `json:"access_token,omitempty"`
}

// ============================================================================
// Copilot Auth Handlers
// ============================================================================

func (r *Router) handleCopilotDeviceCode(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request DeviceCodeRequest
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Step 1: Request device code from GitHub
	deviceResp, err := requestGitHubDeviceCode()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to request device code: %v", err), http.StatusInternalServerError)
		return
	}

	// Return device code info to client
	result := DeviceCodeResult{
		DeviceCode:      deviceResp.DeviceCode,
		UserCode:        deviceResp.UserCode,
		VerificationURI: deviceResp.VerificationURI,
		ExpiresIn:       deviceResp.ExpiresIn,
		Interval:        deviceResp.Interval,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (r *Router) handleCopilotToken(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request TokenRequest
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if request.DeviceCode == "" {
		http.Error(w, "device_code is required", http.StatusBadRequest)
		return
	}

	// Poll GitHub for access token
	// Use default interval of 5 seconds if not specified
	interval := 5
	maxAttempts := 60 // Max 5 minutes (60 * 5 seconds)

	var tokenResp *AccessTokenResponse
	var err error

	for i := 0; i < maxAttempts; i++ {
		tokenResp, err = pollGitHubAccessToken(request.DeviceCode)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to poll for token: %v", err), http.StatusInternalServerError)
			return
		}

		// Check if we got a token
		if tokenResp.AccessToken != "" {
			break
		}

		// Check for errors
		if tokenResp.Error != "" && tokenResp.Error != "authorization_pending" && tokenResp.Error != "slow_down" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(TokenResult{
				Success: false,
				Message: fmt.Sprintf("GitHub OAuth error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc),
			})
			return
		}

		// Wait before next poll
		time.Sleep(time.Duration(interval) * time.Second)
	}

	if tokenResp == nil || tokenResp.AccessToken == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestTimeout)
		json.NewEncoder(w).Encode(TokenResult{
			Success: false,
			Message: "Timeout waiting for user authorization",
		})
		return
	}

	// Save the token as an encrypted Auth Config
	configID := request.ConfigID
	configName := request.Name

	// Generate config ID if not provided
	if configID == "" {
		configID = fmt.Sprintf("copilot-%d", time.Now().Unix())
	}
	if configName == "" {
		configName = "GitHub Copilot OAuth"
	}

	// Check if config already exists
	existingConfig, err := r.authConfigStore.GetByID(configID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to check existing config: %v", err), http.StatusInternalServerError)
		return
	}

	credentials := map[string]string{
		"access_token": tokenResp.AccessToken,
		"token_type":   tokenResp.TokenType,
		"scope":        tokenResp.Scope,
	}

	credentialsJSON, err := json.Marshal(credentials)
	if err != nil {
		http.Error(w, "Failed to marshal credentials", http.StatusInternalServerError)
		return
	}

	now := time.Now()

	if existingConfig != nil {
		// Update existing config
		existingConfig.Credentials = string(credentialsJSON)
		existingConfig.UpdatedAt = now
		if err := r.authConfigStore.Update(existingConfig); err != nil {
			http.Error(w, fmt.Sprintf("Failed to update auth config: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		// Create new config
		config := &models.AuthConfig{
			ID:          configID,
			Name:        configName,
			Provider:    GitHubCopilotProvider,
			AuthType:    GitHubCopilotAuthType,
			Credentials: string(credentialsJSON),
			EndpointURL: "",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := r.authConfigStore.Create(config); err != nil {
			http.Error(w, fmt.Sprintf("Failed to create auth config: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Return success response
	result := TokenResult{
		Success:     true,
		Message:     "GitHub Copilot OAuth token saved successfully",
		ConfigID:    configID,
		AccessToken: tokenResp.AccessToken[:10] + "...", // Only return partial token for security
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// ============================================================================
// GitHub OAuth Device Flow Helpers
// ============================================================================

// requestGitHubDeviceCode requests a device code from GitHub's OAuth device flow endpoint
func requestGitHubDeviceCode() (*DeviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", GitHubCopilotClientID)
	data.Set("scope", "read:user")

	resp, err := http.Post(
		GitHubDeviceCodeURL,
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to make device code request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response (GitHub returns form-encoded data)
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for errors in the response
	if errorVal := values.Get("error"); errorVal != "" {
		return nil, fmt.Errorf("GitHub error: %s - %s", errorVal, values.Get("error_description"))
	}

	deviceResp := &DeviceCodeResponse{
		DeviceCode:      values.Get("device_code"),
		UserCode:        values.Get("user_code"),
		VerificationURI: values.Get("verification_uri"),
	}

	// Parse expires_in
	if expiresStr := values.Get("expires_in"); expiresStr != "" {
		fmt.Sscanf(expiresStr, "%d", &deviceResp.ExpiresIn)
	}

	// Parse interval
	if intervalStr := values.Get("interval"); intervalStr != "" {
		fmt.Sscanf(intervalStr, "%d", &deviceResp.Interval)
	}

	// Set defaults if not provided
	if deviceResp.ExpiresIn == 0 {
		deviceResp.ExpiresIn = 900 // 15 minutes default
	}
	if deviceResp.Interval == 0 {
		deviceResp.Interval = 5 // 5 seconds default
	}

	return deviceResp, nil
}

// pollGitHubAccessToken polls GitHub's access token endpoint
func pollGitHubAccessToken(deviceCode string) (*AccessTokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", GitHubCopilotClientID)
	data.Set("device_code", deviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	resp, err := http.Post(
		GitHubAccessTokenURL,
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to make access token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response (GitHub returns form-encoded data)
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	tokenResp := &AccessTokenResponse{
		AccessToken: values.Get("access_token"),
		TokenType:   values.Get("token_type"),
		Scope:       values.Get("scope"),
		Error:       values.Get("error"),
		ErrorDesc:   values.Get("error_description"),
	}

	return tokenResp, nil
}
