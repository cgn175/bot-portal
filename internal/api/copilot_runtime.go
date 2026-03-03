package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/zeroclaw/bot-portal/internal/models"
	"github.com/zeroclaw/bot-portal/internal/provider"
)

// CopilotChatRequest represents a CopilotKit chat completion request.
// It is a superset of the OpenAI chat completion format with tools support.
type CopilotChatRequest struct {
	Model      string          `json:"model,omitempty"`
	Messages   []ChatMessage   `json:"messages"`
	Stream     bool            `json:"stream,omitempty"`
	Tools      json.RawMessage `json:"tools,omitempty"`
	ToolChoice json.RawMessage `json:"tool_choice,omitempty"`
}

// handleCopilotChat handles POST /api/copilotkit/chat/completions
// It implements the CopilotKit self-hosted runtime protocol:
// 1. Resolves the model (requested or default)
// 2. Injects system prompt with Bot Portal context
// 3. Proxies the request to the upstream LLM provider with SSE streaming
func (r *Router) handleCopilotChat(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var copilotReq CopilotChatRequest
	if err := json.NewDecoder(req.Body).Decode(&copilotReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 1. Resolve model: use requested model or fall back to default
	model, err := r.resolveModelForCopilot(copilotReq.Model)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 2. Resolve auth config for the model's provider
	token, baseURL, copilotAuth, err := r.resolveAuthForModel(model)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 3. Inject system prompt if not already present
	messages := r.injectSystemPrompt(copilotReq.Messages)

	// 4. Build upstream request payload
	payload := map[string]interface{}{
		"model":    model.ModelIdentifier,
		"messages": messages,
		"stream":   true,
	}
	if len(copilotReq.Tools) > 0 {
		payload["tools"] = json.RawMessage(copilotReq.Tools)
	}
	if len(copilotReq.ToolChoice) > 0 {
		payload["tool_choice"] = json.RawMessage(copilotReq.ToolChoice)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "Failed to marshal request", http.StatusInternalServerError)
		return
	}

	endpointURL := strings.TrimRight(baseURL, "/") + "/chat/completions"

	proxyReq, err := http.NewRequest(http.MethodPost, endpointURL, bytes.NewReader(payloadBytes))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
		return
	}

	// Set auth and content headers
	authHeaderName, authHeaderValue := provider.GetAuthHeader(model.Provider, token)
	proxyReq.Header.Set(authHeaderName, authHeaderValue)
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("Accept", "text/event-stream")
	applyProviderHeaders(proxyReq, model.Provider, baseURL)

	// 5. Make the request and stream the response
	client := &http.Client{Timeout: 300 * time.Second} // Longer timeout for streaming
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to model provider: %v", err), http.StatusBadGateway)
		return
	}

	// On 401/403 for Copilot, refresh token and retry once
	if copilotAuth != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
		resp.Body.Close()
		var creds map[string]string
		if err := json.Unmarshal([]byte(copilotAuth.Credentials), &creds); err == nil {
			if newToken, err := r.refreshCopilotToken(copilotAuth, creds["access_token"]); err == nil {
				retryReq, _ := http.NewRequest(http.MethodPost, endpointURL, bytes.NewReader(payloadBytes))
				h, v := provider.GetAuthHeader(model.Provider, newToken)
				retryReq.Header.Set(h, v)
				retryReq.Header.Set("Content-Type", "application/json")
				retryReq.Header.Set("Accept", "text/event-stream")
				applyProviderHeaders(retryReq, model.Provider, baseURL)

				if retryResp, err := client.Do(retryReq); err == nil {
					resp = retryResp
				}
			}
		}
	}

	// Stream the response back to the client
	if resp.StatusCode == http.StatusOK {
		if err := r.streamChatResponse(w, resp); err != nil {
			log.Printf("Copilot streaming error: %v", err)
		}
		return
	}

	// Non-OK response — forward the error
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
}

// handleCopilotInfo handles GET /api/copilot/info — returns available models and features
func (r *Router) handleCopilotInfo(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	modelsList, _ := r.modelStore.List()
	availableModels := make([]map[string]string, 0, len(modelsList))
	for _, m := range modelsList {
		availableModels = append(availableModels, map[string]string{
			"id":       m.ID,
			"name":     m.Name,
			"provider": m.Provider,
		})
	}

	info := map[string]interface{}{
		"name":     "Bot Portal CopilotKit Runtime",
		"version":  "1.0.0",
		"features": []string{"streaming", "tools", "function_calling"},
		"models":   availableModels,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// ============================================================================
// Model Resolution
// ============================================================================

// resolveModelForCopilot resolves the model to use for a CopilotKit request.
// Fallback chain: requested model → settings.DefaultModel → COPILOT_DEFAULT_MODEL_ID env → is_default flag → first model → error
func (r *Router) resolveModelForCopilot(requestedModel string) (*models.Model, error) {
	// Step 1: If a model ID was explicitly requested, look it up
	if requestedModel != "" {
		model, err := r.modelStore.GetByID(requestedModel)
		if err != nil {
			return nil, fmt.Errorf("failed to get model: %v", err)
		}
		if model != nil {
			return model, nil
		}
		// Model ID not found — fall through to defaults
		log.Printf("Requested model %q not found, trying defaults", requestedModel)
	}

	// Step 2: Check CopilotKit settings (from /api/copilotkit/settings)
	copilotKitSettings.RLock()
	settingsModel := copilotKitSettings.DefaultModel
	copilotKitSettings.RUnlock()

	if settingsModel != "" {
		model, err := r.modelStore.GetByID(settingsModel)
		if err == nil && model != nil {
			return model, nil
		}
		log.Printf("CopilotKit settings model %q not found, trying env", settingsModel)
	}

	// Step 3: Check COPILOT_DEFAULT_MODEL_ID env var
	if envModelID := os.Getenv("COPILOT_DEFAULT_MODEL_ID"); envModelID != "" {
		model, err := r.modelStore.GetByID(envModelID)
		if err == nil && model != nil {
			return model, nil
		}
		log.Printf("COPILOT_DEFAULT_MODEL_ID=%q not found in database", envModelID)
	}

	// Step 3 + 4: Use GetDefault (is_default flag → first model)
	model, err := r.modelStore.GetDefault()
	if err != nil {
		return nil, fmt.Errorf("failed to get default model: %v", err)
	}
	if model != nil {
		return model, nil
	}

	return nil, fmt.Errorf("no models configured. Add a model in Settings → Models first")
}

// resolveAuthForModel finds the auth config and token for a given model's provider.
func (r *Router) resolveAuthForModel(model *models.Model) (token string, baseURL string, copilotAuth *models.AuthConfig, err error) {
	authConfigs, err := r.authConfigStore.List()
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to get auth configs: %v", err)
	}

	for _, auth := range authConfigs {
		if auth.Provider == model.Provider || (model.Provider == "copilot" && auth.AuthType == "github_copilot_oauth") {
			baseURL = auth.EndpointURL

			if auth.AuthType == "github_copilot_oauth" {
				copilotAuth = auth
				var refreshErr error
				token, refreshErr = r.ensureFreshCopilotToken(auth)
				if refreshErr != nil {
					var creds map[string]string
					if jsonErr := json.Unmarshal([]byte(auth.Credentials), &creds); jsonErr == nil {
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
				if jsonErr := json.Unmarshal([]byte(auth.Credentials), &creds); jsonErr != nil {
					continue
				}
				token = creds["api_key"]
				baseURL = strings.TrimRight(auth.EndpointURL, "/")
			}
			break
		}
	}

	if token == "" {
		return "", "", nil, fmt.Errorf("no auth config found for provider %q. Add one in Settings → Auth", model.Provider)
	}

	return token, baseURL, copilotAuth, nil
}

// ============================================================================
// System Prompt
// ============================================================================

// injectSystemPrompt prepends a Bot Portal system prompt if one is not already present.
func (r *Router) injectSystemPrompt(messages []ChatMessage) []ChatMessage {
	// Check if there's already a system message
	for _, msg := range messages {
		if msg.Role == "system" {
			return messages
		}
	}

	systemPrompt := r.buildSystemPrompt()
	contentJSON, _ := json.Marshal(systemPrompt)
	return append([]ChatMessage{{Role: "system", Content: json.RawMessage(contentJSON)}}, messages...)
}

// buildSystemPrompt creates a dynamic system prompt describing Bot Portal.
func (r *Router) buildSystemPrompt() string {
	// Allow override via env var
	if custom := os.Getenv("COPILOT_SYSTEM_PROMPT"); custom != "" {
		return custom
	}

	agents, _ := r.agentStore.List()
	modelsList, _ := r.modelStore.List()
	authConfigs, _ := r.authConfigStore.List()

	running := 0
	for _, a := range agents {
		if a.Status == "running" {
			running++
		}
	}

	return fmt.Sprintf(`You are the Bot Portal assistant. Bot Portal is a self-hosted AI agent management platform.

Current state:
- %d agents registered (%d running)
- %d models configured
- %d auth configurations

You can help users:
- List, create, start, stop, restart, and delete agents
- List, create, and delete models  
- List, create, and delete auth configurations
- Navigate to different pages (Agents, Models, Auth, Chat, Messages)

When asked to perform an action, use the available tool functions.
Always confirm destructive actions (delete, stop) before executing.
Be concise and helpful.`, len(agents), running, len(modelsList), len(authConfigs))
}
