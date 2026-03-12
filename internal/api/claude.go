package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// ClaudeRequest represents a request to the Claude API
type ClaudeRequest struct {
	Model         string          `json:"model"`
	Messages      []ClaudeMessage `json:"messages"`
	MaxTokens     int             `json:"max_tokens"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	TopK          *int            `json:"top_k,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
	System        string          `json:"system,omitempty"`
}

// ClaudeMessage represents a message in the conversation
type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ClaudeResponse represents a response from the Claude API
type ClaudeResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason,omitempty"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        ClaudeUsage    `json:"usage"`
}

// ContentBlock represents a content block in the response
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ClaudeUsage represents token usage information
type ClaudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ClaudeStreamEvent represents a streaming event from the Claude API
type ClaudeStreamEvent struct {
	Type         string        `json:"type"`
	Message      *ClaudeResponse `json:"message,omitempty"`
	Index        int           `json:"index,omitempty"`
	ContentBlock *ContentBlock `json:"content_block,omitempty"`
	Delta        *ContentDelta `json:"delta,omitempty"`
	Usage        *ClaudeUsage  `json:"usage,omitempty"`
}

// ContentDelta represents incremental content in streaming
type ContentDelta struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ClaudeError represents an error response from the Claude API
type ClaudeError struct {
	Type  string             `json:"type"`
	Error ClaudeErrorDetail  `json:"error"`
}

// ClaudeErrorDetail contains error details
type ClaudeErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// OpenAIResponse represents a response in OpenAI format
type OpenAIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Message represents a message in a choice
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// transformClaudeResponseToOpenAI converts Claude response to OpenAI format
func transformClaudeResponseToOpenAI(claudeResp ClaudeResponse) OpenAIResponse {
	return OpenAIResponse{
		ID:      claudeResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   claudeResp.Model,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: extractTextContent(claudeResp.Content),
				},
				FinishReason: mapStopReason(claudeResp.StopReason),
			},
		},
		Usage: Usage{
			PromptTokens:     claudeResp.Usage.InputTokens,
			CompletionTokens: claudeResp.Usage.OutputTokens,
			TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		},
	}
}

// extractTextContent extracts text from content blocks
func extractTextContent(content []ContentBlock) string {
	for _, block := range content {
		if block.Type == "text" {
			return block.Text
		}
	}
	return ""
}

// mapStopReason maps Claude stop reason to OpenAI finish reason
func mapStopReason(claudeReason string) string {
	switch claudeReason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	default:
		return "stop"
	}
}

// handleClaudeMessages handles POST /api/claude
func (r *Router) handleClaudeMessages(w http.ResponseWriter, req *http.Request) {
	// Log request details (user requirement: "with log detail about the request of claude code")
	log.Printf("[Claude API] Request: method=%s, path=%s, remote=%s",
		req.Method, req.URL.Path, req.RemoteAddr)

	// Parse Claude request
	var claudeReq ClaudeRequest
	if err := json.NewDecoder(req.Body).Decode(&claudeReq); err != nil {
		log.Printf("[Claude API] Failed to parse request: %v", err)
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Log request details
	log.Printf("[Claude API] Request details: model=%s, messages=%d, stream=%v, max_tokens=%d",
		claudeReq.Model, len(claudeReq.Messages), claudeReq.Stream, claudeReq.MaxTokens)

	if claudeReq.System != "" {
		log.Printf("[Claude API] System message: %s", claudeReq.System)
	}

	// TODO: Implement provider proxy
	// For now, return 501 Not Implemented
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "Provider proxy not yet implemented",
	})
}

// transformClaudeStreamToOpenAI transforms Claude SSE stream to OpenAI format
func transformClaudeStreamToOpenAI(claudeStream io.Reader, w http.ResponseWriter) error {
	scanner := bufio.NewScanner(claudeStream)
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for scanner.Scan() {
		line := scanner.Text()

		// Skip event lines
		if strings.HasPrefix(line, "event:") {
			continue
		}

		// Process data lines
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data: ")

			var claudeEvent ClaudeStreamEvent
			if err := json.Unmarshal([]byte(data), &claudeEvent); err != nil {
				continue
			}

			// Only transform content_block_delta events
			if claudeEvent.Type == "content_block_delta" && claudeEvent.Delta != nil {
				openAIChunk := OpenAIStreamChunk{
					ID:      "chatcmpl-" + fmt.Sprintf("%d", time.Now().UnixNano()),
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   "claude", // Will be filled with actual model
					Choices: []StreamChoice{
						{
							Index: 0,
							Delta: Delta{
								Content: claudeEvent.Delta.Text,
							},
						},
					},
				}

				chunkJSON, _ := json.Marshal(openAIChunk)
				fmt.Fprintf(w, "data: %s\n\n", chunkJSON)
				flusher.Flush()
			}
		}
	}

	// Send [DONE]
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	return scanner.Err()
}

// OpenAIStreamChunk represents a streaming chunk in OpenAI format
type OpenAIStreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

// StreamChoice represents a choice in a streaming response
type StreamChoice struct {
	Index int   `json:"index"`
	Delta Delta `json:"delta"`
}

// Delta represents a delta in a streaming response
type Delta struct {
	Content string `json:"content,omitempty"`
	Role    string `json:"role,omitempty"`
}
