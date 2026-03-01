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
	// Fetch from store to get decrypted credentials, then run discovery async
	if !r.skipModelDiscovery {
		go func(configID string) {
			log.Printf("[model-discovery] goroutine started for %s", configID)
			cfg, err := r.authConfigStore.GetByID(configID)
			if err != nil {
				log.Printf("[model-discovery] failed to get auth config %s: %v", configID, err)
				return
			}
			if cfg == nil {
				log.Printf("[model-discovery] auth config %s not found (nil)", configID)
				return
			}
			log.Printf("[model-discovery] successfully fetched config %s, starting discovery", configID)
			r.discoverAndSaveModels(cfg)
		}(config.ID)
	}

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
	// Fetch from store to get decrypted credentials, then run discovery async
	if !r.skipModelDiscovery {
		go func(configID string) {
			log.Printf("[model-discovery] goroutine started for %s", configID)
			cfg, err := r.authConfigStore.GetByID(configID)
			if err != nil {
				log.Printf("[model-discovery] failed to get auth config %s: %v", configID, err)
				return
			}
			if cfg == nil {
				log.Printf("[model-discovery] auth config %s not found (nil)", configID)
				return
			}
			log.Printf("[model-discovery] successfully fetched config %s, starting discovery", configID)
			r.discoverAndSaveModels(cfg)
		}(config.ID)
	}

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

// modelsResponse represents the standard OpenAI-compatible /models response
type modelsResponse struct {
	Data []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"data"`
}

// discoverModels fetches available models from an auth config's provider endpoint.
// It handles auth type differences (github_copilot_oauth vs bearer_token) and
// tries multiple URL patterns (/models, /v1/models) to find the models endpoint.
// Returns the parsed response, the effective provider ID used, the baseURL used, and any error.
func (r *Router) discoverModels(config *models.AuthConfig) (*modelsResponse, string, string, error) {
	var creds map[string]string
	if err := json.Unmarshal([]byte(config.Credentials), &creds); err != nil {
		return nil, "", "", fmt.Errorf("failed to parse credentials: %w", err)
	}

	// Debug: log available credential keys
	var credKeys []string
	for k := range creds {
		credKeys = append(credKeys, k)
	}
	log.Printf("[discover-models] auth config %s has credential keys: %v", config.ID, credKeys)

	// Determine token, baseURL, and provider based on auth type
	var token, baseURL, providerID string
	if config.AuthType == "github_copilot_oauth" {
		var refreshErr error
		token, refreshErr = r.ensureFreshCopilotToken(config)
		if refreshErr != nil {
			token = creds["copilot_api_key"]
			if token == "" {
				token = creds["access_token"]
			}
		}
		baseURL = config.EndpointURL
		if baseURL == "" {
			baseURL = "https://api.githubcopilot.com"
		}
		providerID = "copilot"
	} else {
		token = creds["api_key"]
		baseURL = strings.TrimRight(config.EndpointURL, "/")
		providerID = config.Provider
	}

	if token == "" {
		log.Printf("[discover-models] no API key found for %s (checked api_key/access_token/copilot_api_key)", config.ID)
		return nil, "", "", fmt.Errorf("no API key / token found in auth config")
	}
	if baseURL == "" {
		return nil, "", "", fmt.Errorf("no endpoint URL configured")
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
		urls = []string{baseURL + "/models", baseURL + "/v1/models"}
	}

	for _, u := range urls {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			continue
		}
		authHeaderName, authHeaderValue := provider.GetAuthHeader(providerID, token)
		log.Printf("[discover-models] setting %s header for %s: %s... (length: %d)", authHeaderName, config.ID, token[:min(10, len(token))], len(authHeaderValue))
		req.Header.Set(authHeaderName, authHeaderValue)
		req.Header.Set("Accept", "application/json")

		// Apply provider-specific headers from registry
		applyProviderHeaders(req, providerID, baseURL)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("[discover-models] non-OK status for %s (%s): %d, body: %s", config.ID, u, resp.StatusCode, string(bodyBytes))
			continue
		}
		log.Printf("[discover-models] got OK response from %s (%s), body length: %d", config.ID, u, len(bodyBytes))

		var modelsResp modelsResponse
		if err := json.Unmarshal(bodyBytes, &modelsResp); err != nil {
			log.Printf("[discover-models] failed to decode models response for %s (%s): %v, body: %s", config.ID, u, err, string(bodyBytes))
			continue
		}

		return &modelsResp, providerID, baseURL, nil
	}

	log.Printf("[discover-models] could not fetch models for auth config %s - exhausted all URLs", config.ID)
	return nil, "", "", fmt.Errorf("could not fetch models from any known endpoint path")
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

	modelsResp, _, _, err := r.discoverModels(config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(modelsResp)
}

// ============================================================================
// Model Auto-Discovery
// ============================================================================

// discoverAndSaveModels fetches available models from an auth config's provider
// and saves them to the database. Duplicate model IDs are silently skipped.
// The config should already have decrypted credentials (fetched from store before calling this).
func (r *Router) discoverAndSaveModels(config *models.AuthConfig) {
	log.Printf("[model-discovery] starting discovery for auth config %s (provider: %s, authType: %s)", config.ID, config.Provider, config.AuthType)

	modelsResp, providerID, baseURL, err := r.discoverModels(config)
	if err != nil {
		log.Printf("[model-discovery] failed to discover models for %s: %v", config.ID, err)
		return
	}

	log.Printf("[model-discovery] parsed %d models from %s", len(modelsResp.Data), config.ID)

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
			Provider:        providerID,
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
	log.Printf("[model-discovery] saved %d new models from %s (provider: %s)", saved, config.ID, providerID)
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
