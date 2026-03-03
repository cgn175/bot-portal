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

// chatProxyOptions holds parameters for proxying a chat completion request upstream.
type chatProxyOptions struct {
	model       *models.Model
	messages    []ChatMessage
	stream      bool
	tools       json.RawMessage
	toolChoice  json.RawMessage
	maxTokens   int
	temperature *float64
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model       string          `json:"model,omitempty"`
	Messages    []ChatMessage   `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Tools       json.RawMessage `json:"tools,omitempty"`
	ToolChoice  json.RawMessage `json:"tool_choice,omitempty"`
}

// ChatMessage represents a single message in the conversation.
// Uses json.RawMessage for Content and ToolCalls to transparently proxy
// all fields to upstream LLM providers without loss.
type ChatMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
}

// ContentString returns the content as a plain string, handling both
// JSON string values and other types (returns empty string for non-strings).
func (m ChatMessage) ContentString() string {
	if len(m.Content) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(m.Content, &s); err == nil {
		return s
	}
	return string(m.Content)
}

// NewChatMessage creates a ChatMessage with a string content value.
func NewChatMessage(role, content string) ChatMessage {
	contentJSON, _ := json.Marshal(content)
	return ChatMessage{Role: role, Content: json.RawMessage(contentJSON)}
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

	// Resolve model: for CopilotKit requests (identified by X-Inject-System-Prompt header),
	// always use backend-defined model, ignoring the request model
	requestedModel := chatReq.Model
	log.Printf("[Adaptive Chat] Request header: %s", req.Header.Get("X-Inject-System-Prompt"))

	if req.Header.Get("X-Inject-System-Prompt") == "true" {
		requestedModel = "" // Force fallback to backend-defined model
	}

	model, err := r.resolveModel(requestedModel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	// Inject system prompt if requested
	messages := chatReq.Messages
	if req.Header.Get("X-Inject-System-Prompt") == "true" {
		messages = r.injectSystemPrompt(messages)
	}

	r.proxyChatCompletion(w, chatProxyOptions{
		model:       model,
		messages:    messages,
		stream:      chatReq.Stream,
		tools:       chatReq.Tools,
		toolChoice:  chatReq.ToolChoice,
		maxTokens:   chatReq.MaxTokens,
		temperature: chatReq.Temperature,
	})
}

// proxyChatCompletion handles the common proxy logic for sending a chat completion
// request to an upstream LLM provider. It resolves auth, builds the payload,
// makes the request (with Copilot token refresh retry), and writes the response.
func (r *Router) proxyChatCompletion(w http.ResponseWriter, opts chatProxyOptions) {
	token, baseURL, copilotAuth, err := r.resolveAuthForModel(opts.model)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Build upstream request payload
	payload := map[string]interface{}{
		"model":    opts.model.ModelIdentifier,
		"messages": opts.messages,
		"stream":   opts.stream,
	}
	if len(opts.tools) > 0 {
		payload["tools"] = json.RawMessage(opts.tools)
	}
	if len(opts.toolChoice) > 0 {
		payload["tool_choice"] = json.RawMessage(opts.toolChoice)
	}
	if opts.maxTokens > 0 {
		payload["max_tokens"] = opts.maxTokens
	}
	if opts.temperature != nil {
		payload["temperature"] = *opts.temperature
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "Failed to marshal request", http.StatusInternalServerError)
		return
	}

	endpointURL := strings.TrimRight(baseURL, "/") + "/chat/completions"

	acceptHeader := "application/json"
	timeout := 120 * time.Second
	if opts.stream {
		acceptHeader = "text/event-stream"
		timeout = 300 * time.Second
	}

	proxyReq, err := http.NewRequest(http.MethodPost, endpointURL, bytes.NewReader(payloadBytes))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
		return
	}

	authHeaderName, authHeaderValue := provider.GetAuthHeader(opts.model.Provider, token)
	proxyReq.Header.Set(authHeaderName, authHeaderValue)
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("Accept", acceptHeader)
	applyProviderHeaders(proxyReq, opts.model.Provider, baseURL)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to model provider: %v", err), http.StatusBadGateway)
		return
	}

	// On 401/403 for Copilot, refresh the token and retry once
	if copilotAuth != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
		resp.Body.Close()
		var creds map[string]string
		if err := json.Unmarshal([]byte(copilotAuth.Credentials), &creds); err == nil {
			if newToken, err := r.refreshCopilotToken(copilotAuth, creds["access_token"]); err == nil {
				retryReq, _ := http.NewRequest(http.MethodPost, endpointURL, bytes.NewReader(payloadBytes))
				h, v := provider.GetAuthHeader(opts.model.Provider, newToken)
				retryReq.Header.Set(h, v)
				retryReq.Header.Set("Content-Type", "application/json")
				retryReq.Header.Set("Accept", acceptHeader)
				applyProviderHeaders(retryReq, opts.model.Provider, baseURL)

				if retryResp, err := client.Do(retryReq); err == nil {
					resp = retryResp
				}
			}
		}
	}

	// Stream or return full response
	if opts.stream {
		if resp.StatusCode == http.StatusOK {
			if err := r.streamChatResponse(w, resp); err != nil {
				log.Printf("Streaming error: %v", err)
			}
			return
		}
		// Non-OK streaming response — forward the error body
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		buf := make([]byte, 4096)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
			}
			if readErr != nil {
				break
			}
		}
		return
	}

	// Non-streaming: read full body and forward
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}
