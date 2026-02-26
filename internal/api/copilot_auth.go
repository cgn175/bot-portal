package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zeroclaw/bot-portal/internal/models"
)

// GitHub OAuth Device Flow constants
const (
	GitHubDeviceCodeURL   = "https://github.com/login/device/code"
	GitHubAccessTokenURL  = "https://github.com/login/oauth/access_token"
	GitHubCopilotTokenURL = "https://api.github.com/copilot_internal/v2/token"
	GitHubCopilotClientID = "Iv1.b507a08c87ecfe98" // GitHub Copilot OAuth App client ID
	GitHubCopilotProvider = "github_copilot"
	GitHubCopilotAuthType = "github_copilot_oauth"
	DefaultCopilotAPIURL  = "https://api.githubcopilot.com"
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

// CopilotAPIKeyResponse represents the response from GitHub's Copilot token endpoint
type CopilotAPIKeyResponse struct {
	Token     string           `json:"token"`
	ExpiresAt int64            `json:"expires_at"`
	Endpoints CopilotEndpoints `json:"endpoints"`
}

// CopilotEndpoints represents the API endpoints in the Copilot token response
type CopilotEndpoints struct {
	API string `json:"api"`
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
	Success  bool   `json:"success"`
	Message  string `json:"message,omitempty"`
	ConfigID string `json:"configId,omitempty"`
}

// ============================================================================
// Copilot Auth Handlers
// ============================================================================

func (r *Router) handleCopilotDeviceCode(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
	// Poll GitHub for access token
	tokenResp, err := pollGitHubAccessToken(request.DeviceCode)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to poll for token: %v", err), http.StatusInternalServerError)
		return
	}

	// Check if we got a token
	if tokenResp.AccessToken == "" {
		if tokenResp.Error == "authorization_pending" || tokenResp.Error == "slow_down" {
			// Still waiting for user authorization, return 202 Accepted
			w.WriteHeader(http.StatusAccepted)
			return
		}

		// Other errors are terminal
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(TokenResult{
			Success: false,
			Message: fmt.Sprintf("GitHub OAuth error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc),
		})
		return
	}

	// Exchange the GitHub access token for a Copilot API key
	copilotKeyResp, err := exchangeForCopilotAPIKey(tokenResp.AccessToken)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to exchange for Copilot API key: %v", err), http.StatusInternalServerError)
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

	// Determine API endpoint (use provided or default)
	apiEndpoint := copilotKeyResp.Endpoints.API
	if apiEndpoint == "" {
		apiEndpoint = DefaultCopilotAPIURL
	}

	credentials := map[string]string{
		"copilot_api_key": copilotKeyResp.Token,
		"expires_at":      fmt.Sprintf("%d", copilotKeyResp.ExpiresAt),
		"access_token":    tokenResp.AccessToken, // Keep for potential refresh
		"token_type":      tokenResp.TokenType,
		"scope":           tokenResp.Scope,
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
		existingConfig.EndpointURL = apiEndpoint
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
			EndpointURL: apiEndpoint,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := r.authConfigStore.Create(config); err != nil {
			http.Error(w, fmt.Sprintf("Failed to create auth config: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Auto-discover and save models from copilot
	// Fetch from store to get decrypted credentials, then run discovery async
	if !r.skipModelDiscovery {
		go func(id string) {
			cfg, err := r.authConfigStore.GetByID(id)
			if err != nil || cfg == nil {
				log.Printf("[model-discovery] failed to get copilot auth config %s: %v", id, err)
				return
			}
			r.discoverAndSaveModels(cfg)
		}(configID)
	}

	// Return success response
	result := TokenResult{
		Success:  true,
		Message:  "Token saved successfully",
		ConfigID: configID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (r *Router) handleCopilotModels(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	configID := req.URL.Query().Get("configId")
	if configID == "" {
		http.Error(w, "configId query parameter is required", http.StatusBadRequest)
		return
	}

	// Get the auth config to retrieve the access token
	config, err := r.authConfigStore.GetByID(configID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get auth config: %v", err), http.StatusInternalServerError)
		return
	}
	if config == nil {
		http.Error(w, "Auth config not found", http.StatusNotFound)
		return
	}

	// Parse credentials to get Copilot API key
	var creds map[string]string
	if err := json.Unmarshal([]byte(config.Credentials), &creds); err != nil {
		http.Error(w, "Failed to parse credentials", http.StatusInternalServerError)
		return
	}

	// Use the Copilot API key (not the GitHub access token)
	apiKey := creds["copilot_api_key"]
	if apiKey == "" {
		// Fallback to access_token for backwards compatibility
		apiKey = creds["access_token"]
	}
	if apiKey == "" {
		http.Error(w, "No API key found in auth config", http.StatusBadRequest)
		return
	}

	// Determine API endpoint
	apiEndpoint := config.EndpointURL
	if apiEndpoint == "" {
		apiEndpoint = DefaultCopilotAPIURL
	}

	// Fetch models from GitHub Copilot API
	modelsReq, err := http.NewRequest(http.MethodGet, apiEndpoint+"/models", nil)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}
	modelsReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	modelsReq.Header.Set("Editor-Version", "vscode/1.85.1")
	modelsReq.Header.Set("Editor-Plugin-Version", "copilot/1.155.0")
	modelsReq.Header.Set("User-Agent", "GithubCopilot/1.155.0")
	modelsReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(modelsReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch models: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf("GitHub Copilot API returned status %d: %s", resp.StatusCode, string(body)), resp.StatusCode)
		return
	}

	// Proxy the response
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

// ============================================================================
// GitHub OAuth Device Flow Helpers
// ============================================================================

// requestGitHubDeviceCode requests a device code from GitHub's OAuth device flow endpoint
func requestGitHubDeviceCode() (*DeviceCodeResponse, error) {
	payload := map[string]string{
		"client_id": GitHubCopilotClientID,
		"scope":     "read:user",
	}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, GitHubDeviceCodeURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create device code request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make device code request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub returned status %d: %s", resp.StatusCode, string(body))
	}

	var deviceResp DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Set defaults if not provided
	if deviceResp.ExpiresIn == 0 {
		deviceResp.ExpiresIn = 900 // 15 minutes default
	}
	if deviceResp.Interval == 0 {
		deviceResp.Interval = 5 // 5 seconds default
	}

	return &deviceResp, nil
}

// pollGitHubAccessToken polls GitHub's access token endpoint
func pollGitHubAccessToken(deviceCode string) (*AccessTokenResponse, error) {
	payload := map[string]string{
		"client_id":   GitHubCopilotClientID,
		"device_code": deviceCode,
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, GitHubAccessTokenURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create access token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make access token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp AccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &tokenResp, nil
}

// exchangeForCopilotAPIKey exchanges a GitHub access token for a Copilot API key.
// This is required because the Copilot API uses different tokens than the GitHub OAuth tokens.
func exchangeForCopilotAPIKey(accessToken string) (*CopilotAPIKeyResponse, error) {
	req, err := http.NewRequest(http.MethodGet, GitHubCopilotTokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Copilot token request: %w", err)
	}

	// Required Copilot headers (mimics VS Code Copilot extension)
	req.Header.Set("Editor-Version", "vscode/1.85.1")
	req.Header.Set("Editor-Plugin-Version", "copilot/1.155.0")
	req.Header.Set("User-Agent", "GithubCopilot/1.155.0")
	req.Header.Set("Accept", "application/json")

	// Note: GitHub uses "token" prefix, NOT "Bearer" for this endpoint
	req.Header.Set("Authorization", fmt.Sprintf("token %s", accessToken))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange for Copilot API key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub Copilot token endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiKeyResp CopilotAPIKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiKeyResp); err != nil {
		return nil, fmt.Errorf("failed to decode Copilot API key response: %w", err)
	}

	return &apiKeyResp, nil
}
