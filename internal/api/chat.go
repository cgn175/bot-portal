package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
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

	// Find auth config for this model's provider
	authConfigs, err := r.authConfigStore.List()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get auth configs: %v", err), http.StatusInternalServerError)
		return
	}

	var token, baseURL string
	for _, auth := range authConfigs {
		if auth.Provider == model.Provider || (model.Provider == "copilot" && auth.AuthType == "github_copilot_oauth") {
			var creds map[string]string
			if err := json.Unmarshal([]byte(auth.Credentials), &creds); err != nil {
				continue
			}

			baseURL = auth.EndpointURL

			if auth.AuthType == "github_copilot_oauth" {
				token = creds["copilot_api_key"]
				if token == "" {
					token = creds["access_token"]
				}
				if baseURL == "" {
					baseURL = "https://api.githubcopilot.com"
				}
			} else {
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

	// Set headers
	proxyReq.Header.Set("Authorization", "Bearer "+token)
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
	defer resp.Body.Close()

	// Read and proxy the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}
