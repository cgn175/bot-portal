package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/zeroclaw/bot-portal/internal/models"
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

	// Return the config with masked credentials
	maskedConfig := maskCredentialsInConfig(config)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(maskedConfig)
}

func (r *Router) deleteAuthConfig(w http.ResponseWriter, req *http.Request, configID string) {
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
