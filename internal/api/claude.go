package api

import (
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

// isClaudeModel returns true if the model name indicates a Claude model
func isClaudeModel(modelName string) bool {
	return strings.HasPrefix(strings.ToLower(modelName), "claude-")
}

// transformToClaudeFormat converts OpenAI ChatRequest to Claude format
func transformToClaudeFormat(openAIReq ChatRequest) ClaudeRequest {
	claudeReq := ClaudeRequest{
		Model:       openAIReq.Model,
		MaxTokens:   openAIReq.MaxTokens,
		Messages:    []ClaudeMessage{},
		Stream:      openAIReq.Stream,
		Temperature: openAIReq.Temperature,
		TopP:        openAIReq.TopP,
	}

	// Extract system message and filter out from messages array
	for _, msg := range openAIReq.Messages {
		if msg.Role == "system" {
			// Use first system message only
			if claudeReq.System == "" {
				claudeReq.System = msg.Content
			}
		} else {
			// Keep user and assistant messages
			claudeReq.Messages = append(claudeReq.Messages, ClaudeMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	return claudeReq
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
