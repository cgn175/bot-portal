package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zeroclaw/bot-portal/internal/models"
	"github.com/zeroclaw/bot-portal/internal/provider"
	"github.com/zeroclaw/bot-portal/internal/store"
)

// ============================================================================
// Auth Configs Handlers
// ============================================================================

func (r *Router) handleAuthConfigs(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.listAuthConfigs(w, req)
	case http.MethodPost:
		r.createAuthConfig(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleAuthConfigDetail(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if path == "/api/auth-configs/" {
		http.Error(w, "Auth config ID required", http.StatusBadRequest)
		return
	}

	configID := path[len("/api/auth-configs/"):]

	switch req.Method {
	case http.MethodGet:
		r.getAuthConfig(w, req, configID)
	case http.MethodPut:
		r.updateAuthConfig(w, req, configID)
	case http.MethodDelete:
		r.deleteAuthConfig(w, req, configID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) listAuthConfigs(w http.ResponseWriter, req *http.Request) {
	// Use ListMasked to get configs with credentials masked for API responses
	configs, err := r.authConfigStore.ListMasked()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Apply custom masking format for API responses
	maskedConfigs := maskCredentialsInConfigs(configs)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(maskedConfigs)
}

func (r *Router) createAuthConfig(w http.ResponseWriter, req *http.Request) {
	var requestConfig struct {
		ID          string            `json:"id"`
		Name        string            `json:"name"`
		Provider    string            `json:"provider"`
		AuthType    string            `json:"authType"`
		Credentials map[string]string `json:"credentials"`
		EndpointURL string            `json:"endpointUrl"`
	}

	if err := json.NewDecoder(req.Body).Decode(&requestConfig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if requestConfig.ID == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}
	if requestConfig.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}
	if requestConfig.Provider == "" {
		http.Error(w, "Provider is required", http.StatusBadRequest)
		return
	}
	if requestConfig.AuthType == "" {
		http.Error(w, "AuthType is required", http.StatusBadRequest)
		return
	}

	// Convert credentials map to JSON string for storage
	credentialsJSON, err := json.Marshal(requestConfig.Credentials)
	if err != nil {
		http.Error(w, "Invalid credentials format", http.StatusBadRequest)
		return
	}

	now := time.Now()
	config := &models.AuthConfig{
		ID:          requestConfig.ID,
		Name:        requestConfig.Name,
		Provider:    requestConfig.Provider,
		AuthType:    requestConfig.AuthType,
		Credentials: string(credentialsJSON),
		EndpointURL: requestConfig.EndpointURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := r.authConfigStore.Create(config); err != nil {
		// Check for duplicate ID error
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "already exists") {
			http.Error(w, "Auth config with this ID already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Auto-discover and save models from this provider
	go r.discoverAndSaveModels(config)

	// Return the config with masked credentials
	maskedConfig := maskCredentialsInConfig(config)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(maskedConfig)
}

func (r *Router) getAuthConfig(w http.ResponseWriter, req *http.Request, configID string) {
	config, err := r.authConfigStore.GetByID(configID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if config == nil {
		http.Error(w, "Auth config not found", http.StatusNotFound)
		return
	}

	// Return the config with masked credentials
	maskedConfig := maskCredentialsInConfig(config)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(maskedConfig)
}

func (r *Router) updateAuthConfig(w http.ResponseWriter, req *http.Request, configID string) {
	config, err := r.authConfigStore.GetByID(configID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if config == nil {
		http.Error(w, "Auth config not found", http.StatusNotFound)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&updates); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Apply updates
	if name, ok := updates["name"].(string); ok {
		config.Name = name
	}
	if provider, ok := updates["provider"].(string); ok {
		config.Provider = provider
	}
	if authType, ok := updates["authType"].(string); ok {
		config.AuthType = authType
	}
	if endpointURL, ok := updates["endpointUrl"].(string); ok {
		config.EndpointURL = endpointURL
	}
	if credentials, ok := updates["credentials"].(map[string]interface{}); ok {
		// Convert credentials map to JSON string
		credsMap := make(map[string]string)
		for key, value := range credentials {
			if strValue, ok := value.(string); ok {
				credsMap[key] = strValue
			}
		}
		credsJSON, err := json.Marshal(credsMap)
		if err != nil {
			http.Error(w, "Invalid credentials format", http.StatusBadRequest)
			return
		}
		config.Credentials = string(credsJSON)
	}

	config.UpdatedAt = time.Now()

	if err := r.authConfigStore.Update(config); err != nil {
		// Check for not found error
		if errors.Is(err, store.ErrNotFound) || strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Auto-discover and save models from this provider
	go r.discoverAndSaveModels(config)

	// Return the config with masked credentials
	maskedConfig := maskCredentialsInConfig(config)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(maskedConfig)
}

func (r *Router) deleteAuthConfig(w http.ResponseWriter, req *http.Request, configID string) {
	// Get the auth config first to know which provider's models to delete
	config, err := r.authConfigStore.GetByID(configID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if config == nil {
		http.Error(w, "Auth config not found", http.StatusNotFound)
		return
	}

	// Delete all models associated with this provider
	provider := config.Provider
	if config.AuthType == "github_copilot_oauth" {
		provider = "copilot"
	}
	if err := r.modelStore.DeleteByProvider(provider); err != nil {
		log.Printf("[delete-auth-config] failed to delete models for provider %s: %v", provider, err)
		// Continue with deleting the auth config even if model deletion fails
	}

	if err := r.authConfigStore.Delete(configID); err != nil {
		// Check for not found error
		if errors.Is(err, store.ErrNotFound) || strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDiscoverModels fetches available models from an auth config's endpoint.
// For copilot configs, it uses the copilot API. For custom/bearer configs, it
// tries {endpointUrl}/models then {endpointUrl}/v1/models.
func (r *Router) handleDiscoverModels(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	configID := req.URL.Query().Get("configId")
	if configID == "" {
		http.Error(w, "configId query parameter is required", http.StatusBadRequest)
		return
	}

	config, err := r.authConfigStore.GetByID(configID)
	if err != nil {
		http.Error(w, "Failed to get auth config", http.StatusInternalServerError)
		return
	}
	if config == nil {
		http.Error(w, "Auth config not found", http.StatusNotFound)
		return
	}

	var creds map[string]string
	if err := json.Unmarshal([]byte(config.Credentials), &creds); err != nil {
		http.Error(w, "Failed to parse credentials", http.StatusInternalServerError)
		return
	}

	// Debug: log available credential keys
	var credKeys []string
	for k := range creds {
		credKeys = append(credKeys, k)
	}
	log.Printf("[discover-models] auth config %s has credential keys: %v", config.ID, credKeys)

	// Determine token and URL based on auth type
	var token, baseURL string
	if config.AuthType == "github_copilot_oauth" {
		// Use the Copilot API key (not the GitHub access token)
		token = creds["copilot_api_key"]
		if token == "" {
			// Fallback to access_token for backwards compatibility
			token = creds["access_token"]
		}
		baseURL = config.EndpointURL
		if baseURL == "" {
			baseURL = "https://api.githubcopilot.com"
		}
	} else {
		token = creds["api_key"]
		baseURL = strings.TrimRight(config.EndpointURL, "/")
	}

	if token == "" {
		log.Printf("[discover-models] no API key found for %s (checked api_key/access_token/copilot_api_key)", config.ID)
		http.Error(w, "No API key / token found in auth config", http.StatusBadRequest)
		return
	}
	if baseURL == "" {
		http.Error(w, "No endpoint URL configured", http.StatusBadRequest)
		return
	}

	// Debug: log token preview
	tokenPreview := ""
	if len(token) > 8 {
		tokenPreview = token[:4] + "..." + token[len(token)-4:] + fmt.Sprintf("(len=%d)", len(token))
	} else if len(token) > 0 {
		tokenPreview = fmt.Sprintf("[short token: len=%d]", len(token))
	} else {
		tokenPreview = "[empty]"
	}
	log.Printf("[discover-models] using token for %s: %s, baseURL: %s", config.ID, tokenPreview, baseURL)

	client := &http.Client{Timeout: 15 * time.Second}

	// Build candidate URLs to try
	var urls []string
	if config.AuthType == "github_copilot_oauth" {
		urls = []string{baseURL + "/models"}
	} else {
		// Try /models first (already includes /v1), then /v1/models as fallback
		urls = []string{baseURL + "/models", baseURL + "/v1/models"}
	}

	for _, u := range urls {
		modelsReq, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			continue
		}
		modelsReq.Header.Set("Authorization", "Bearer "+token)
		modelsReq.Header.Set("Accept", "application/json")

		// Apply provider-specific headers from registry
		applyProviderHeaders(modelsReq, config.Provider, baseURL)

		resp, err := client.Do(modelsReq)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			w.Header().Set("Content-Type", "application/json")
			io.Copy(w, resp.Body)
			return
		}
	}

	http.Error(w, "Could not fetch models from any known endpoint path", http.StatusBadGateway)
}

// ============================================================================
// Model Auto-Discovery
// ============================================================================

// discoverAndSaveModels fetches available models from an auth config's provider
// and saves them to the database. Duplicate model IDs are silently skipped.
func (r *Router) discoverAndSaveModels(config *models.AuthConfig) {
	log.Printf("[model-discovery] starting discovery for auth config %s (provider: %s, authType: %s)", config.ID, config.Provider, config.AuthType)

	var creds map[string]string
	if err := json.Unmarshal([]byte(config.Credentials), &creds); err != nil {
		log.Printf("[model-discovery] failed to parse credentials for %s: %v", config.ID, err)
		return
	}
	log.Printf("[model-discovery] credentials parsed successfully for %s, keys: %v", config.ID, getMapKeys(creds))

	// Debug: log all credential values (safely)
	for k, v := range creds {
		preview := "[empty]"
		if len(v) > 0 {
			if len(v) <= 8 {
				preview = "[short:" + fmt.Sprintf("%d", len(v)) + "]"
			} else {
				preview = v[:4] + "..." + v[len(v)-4:] + fmt.Sprintf("(%d)", len(v))
			}
		}
		log.Printf("[model-discovery] credential %s for %s: %s", k, config.ID, preview)
	}

	var token, baseURL, provider string
	if config.AuthType == "github_copilot_oauth" {
		// Use the Copilot API key (not the GitHub access token)
		token = creds["copilot_api_key"]
		if token == "" {
			// Fallback to access_token for backwards compatibility with old configs
			token = creds["access_token"]
		}
		baseURL = config.EndpointURL
		if baseURL == "" {
			baseURL = "https://api.githubcopilot.com"
		}
		provider = "copilot"
	} else {
		token = creds["api_key"]
		baseURL = strings.TrimRight(config.EndpointURL, "/")
		provider = config.Provider
	}

	if token == "" {
		log.Printf("[model-discovery] no API token found for %s (checked api_key/access_token)", config.ID)
		return
	}
	if baseURL == "" {
		log.Printf("[model-discovery] no endpoint URL configured for %s", config.ID)
		return
	}
	// Log first/last 4 chars of token for debugging (be careful not to log full token)
	tokenPreview := ""
	if len(token) > 8 {
		tokenPreview = token[:4] + "..." + token[len(token)-4:]
	} else if len(token) > 0 {
		tokenPreview = "[token too short]"
	}
	log.Printf("[model-discovery] config %s: baseURL=%s, provider=%s, token_length=%d, token_preview=%s", config.ID, baseURL, provider, len(token), tokenPreview)

	// Check if token already has bearer prefix (would cause "Bearer bearer ..." issue)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		log.Printf("[model-discovery] WARNING: token for %s appears to already have 'bearer ' prefix!", config.ID)
	}

	client := &http.Client{Timeout: 15 * time.Second}

	var urls []string
	if config.AuthType == "github_copilot_oauth" {
		urls = []string{baseURL + "/models"}
	} else {
		urls = []string{baseURL + "/models", baseURL + "/v1/models"}
	}
	log.Printf("[model-discovery] will try URLs for %s: %v", config.ID, urls)

	for _, u := range urls {
		log.Printf("[model-discovery] trying URL for %s: %s", config.ID, u)

		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			log.Printf("[model-discovery] failed to create request for %s (%s): %v", config.ID, u, err)
			continue
		}
		authHeader := fmt.Sprintf("Bearer %s", token)
		if config.AuthType == "github_copilot_oauth" {
			// Log what we're sending (mask the token)
			log.Printf("[model-discovery] setting Authorization header for %s: Bearer %s... (length: %d)", config.ID, token[:min(10, len(token))], len(authHeader))
		}
		req.Header.Set("Authorization", authHeader)
		req.Header.Set("Accept", "application/json")

		// Apply provider-specific headers from registry
		applyProviderHeaders(req, provider, baseURL)

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[model-discovery] HTTP request failed for %s (%s): %v", config.ID, u, err)
			continue
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("[model-discovery] non-OK status for %s (%s): %d, body: %s", config.ID, u, resp.StatusCode, string(bodyBytes))
			continue
		}

		log.Printf("[model-discovery] got OK response from %s (%s), body length: %d", config.ID, u, len(bodyBytes))

		var modelsResp struct {
			Data []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		if err := json.Unmarshal(bodyBytes, &modelsResp); err != nil {
			log.Printf("[model-discovery] failed to decode models response for %s (%s): %v, body: %s", config.ID, u, err, string(bodyBytes))
			continue
		}

		log.Printf("[model-discovery] parsed %d models from %s (%s)", len(modelsResp.Data), config.ID, u)

		now := time.Now()
		saved := 0
		for _, m := range modelsResp.Data {
			name := m.Name
			if name == "" {
				name = m.ID
			}
			model := &models.Model{
				ID:              m.ID,
				Name:            name,
				Provider:        provider,
				ModelIdentifier: m.ID,
				EndpointURL:     baseURL,
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			if err := r.modelStore.Create(model); err != nil {
				// Skip duplicates silently
				continue
			}
			saved++
		}
		log.Printf("[model-discovery] saved %d new models from %s (provider: %s)", saved, config.ID, provider)
		return
	}

	log.Printf("[model-discovery] could not fetch models for auth config %s - exhausted all URLs", config.ID)
}

// getMapKeys returns a slice of map keys for logging
func getMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// applyProviderHeaders sets provider-specific headers on the request
func applyProviderHeaders(req *http.Request, providerName, baseURL string) {
	// Try to get provider from registry
	if p, ok := provider.Get(providerName); ok {
		for key, value := range p.Headers {
			req.Header.Set(key, value)
		}
		return
	}

	// Fallback: detect by URL patterns for providers not in registry
	if strings.Contains(baseURL, "api.kimi.com") || strings.Contains(baseURL, "moonshot") {
		req.Header.Set("User-Agent", "KimiCLI/0.77")
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// maskCredentialsInConfig masks credentials in a single auth config for API responses
func maskCredentialsInConfig(config *models.AuthConfig) *models.AuthConfig {
	// Parse the credentials JSON
	var credsMap map[string]string
	if err := json.Unmarshal([]byte(config.Credentials), &credsMap); err != nil {
		// If parsing fails, set credentials to masked placeholder
		config.Credentials = `{"_masked":"***masked***"}`
		return config
	}

	// Mask each credential value
	maskedCreds := make(map[string]string)
	for key := range credsMap {
		maskedCreds[key] = "***masked***"
	}

	// Convert back to JSON
	maskedJSON, err := json.Marshal(maskedCreds)
	if err != nil {
		config.Credentials = `{"_masked":"***masked***"}`
		return config
	}

	config.Credentials = string(maskedJSON)
	return config
}

// maskCredentialsInConfigs masks credentials in a list of auth configs for API responses
func maskCredentialsInConfigs(configs []*models.AuthConfig) []*models.AuthConfig {
	for i := range configs {
		configs[i] = maskCredentialsInConfig(configs[i])
	}
	return configs
}
