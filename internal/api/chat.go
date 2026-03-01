package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zeroclaw/bot-portal/internal/models"
	"github.com/zeroclaw/bot-portal/internal/provider"
)

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatMessage represents a single message in the conversation
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// handleChatCompletions proxies chat requests to the model's endpoint
func (r *Router) handleChatCompletions(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var chatReq ChatRequest
	if err := json.NewDecoder(req.Body).Decode(&chatReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get model details
	model, err := r.modelStore.GetByID(chatReq.Model)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get model: %v", err), http.StatusInternalServerError)
		return
	}
	if model == nil {
		http.Error(w, "Model not found", http.StatusNotFound)
		return
	}

	// Check if this is a Claude model
	if isClaudeModel(model.ModelIdentifier) {
		log.Printf("[Adaptive Chat] Detected Claude model: %s", model.ModelIdentifier)

		// Transform OpenAI → Claude
		claudeReq := transformToClaudeFormat(chatReq)

		log.Printf("[Format Transform] OpenAI → Claude: system=%v, messages=%d",
			len(claudeReq.System) > 0, len(claudeReq.Messages))

		// TODO: Proxy as Claude request
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Claude model proxy not yet implemented",
		})
		return
	}

	log.Printf("[Adaptive Chat] Using OpenAI format for model: %s", model.ModelIdentifier)

	// Find auth config for this model's provider
	authConfigs, err := r.authConfigStore.List()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get auth configs: %v", err), http.StatusInternalServerError)
		return
	}

	var token, baseURL string
	var copilotAuth *models.AuthConfig // tracked for 401/403 retry
	for _, auth := range authConfigs {
		if auth.Provider == model.Provider || (model.Provider == "copilot" && auth.AuthType == "github_copilot_oauth") {
			baseURL = auth.EndpointURL

			if auth.AuthType == "github_copilot_oauth" {
				copilotAuth = auth
				var refreshErr error
				token, refreshErr = r.ensureFreshCopilotToken(auth)
				if refreshErr != nil {
					// Fall back to stored values
					var creds map[string]string
					if err := json.Unmarshal([]byte(auth.Credentials), &creds); err == nil {
						token = creds["copilot_api_key"]
						if token == "" {
							token = creds["access_token"]
						}
					}
				}
				if baseURL == "" {
					baseURL = "https://api.githubcopilot.com"
				}
			} else {
				var creds map[string]string
				if err := json.Unmarshal([]byte(auth.Credentials), &creds); err != nil {
					continue
				}
				token = creds["api_key"]
				baseURL = strings.TrimRight(auth.EndpointURL, "/")
			}
			break
		}
	}

	if token == "" {
		http.Error(w, "No auth config found for model provider", http.StatusBadRequest)
		return
	}

	// Build the request to the model endpoint
	endpointURL := baseURL + "/chat/completions"

	// Create request payload
	payload := map[string]interface{}{
		"model":    model.ModelIdentifier,
		"messages": chatReq.Messages,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "Failed to marshal request", http.StatusInternalServerError)
		return
	}

	proxyReq, err := http.NewRequest(http.MethodPost, endpointURL, bytes.NewReader(payloadBytes))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
		return
	}

	// Set auth header using provider registry (respects x-api-key, Bearer, custom styles)
	authHeaderName, authHeaderValue := provider.GetAuthHeader(model.Provider, token)
	proxyReq.Header.Set(authHeaderName, authHeaderValue)
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("Accept", "application/json")

	// Apply provider-specific headers from registry
	applyProviderHeaders(proxyReq, model.Provider, baseURL)

	// Make the request
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to make request: %v", err), http.StatusBadGateway)
		return
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	// On 401/403 for Copilot, refresh the token and retry once
	if copilotAuth != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
		var creds map[string]string
		if err := json.Unmarshal([]byte(copilotAuth.Credentials), &creds); err == nil {
			if newToken, err := r.refreshCopilotToken(copilotAuth, creds["access_token"]); err == nil {
				retryReq, _ := http.NewRequest(http.MethodPost, endpointURL, bytes.NewReader(payloadBytes))
				h, v := provider.GetAuthHeader(model.Provider, newToken)
				retryReq.Header.Set(h, v)
				retryReq.Header.Set("Content-Type", "application/json")
				retryReq.Header.Set("Accept", "application/json")
				applyProviderHeaders(retryReq, model.Provider, baseURL)

				if retryResp, err := client.Do(retryReq); err == nil {
					retryBody, readErr := io.ReadAll(retryResp.Body)
					retryResp.Body.Close()
					if readErr == nil {
						body = retryBody
						resp = retryResp
					}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}
