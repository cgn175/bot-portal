package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/zeroclaw/bot-portal/internal/models"
)

// handleCopilotKitInfo handles GET /api/copilot/info — returns available models and features
func (r *Router) handleCopilotKitInfo(w http.ResponseWriter, req *http.Request) {
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

// resolveModel resolves the model to use for a chat request.
// Fallback chain: requested model → settings.DefaultModel → COPILOT_DEFAULT_MODEL_ID env → is_default flag → first model → error
func (r *Router) resolveModel(requestedModel string) (*models.Model, error) {
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
