package api

import (
	"encoding/json"
	"testing"
)

func TestClaudeRequest_Marshal(t *testing.T) {
	req := ClaudeRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []ClaudeMessage{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: 1024,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal ClaudeRequest: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if decoded["model"] != "claude-3-5-sonnet-20241022" {
		t.Errorf("Expected model=claude-3-5-sonnet-20241022, got %v", decoded["model"])
	}
	if decoded["max_tokens"] != float64(1024) {
		t.Errorf("Expected max_tokens=1024, got %v", decoded["max_tokens"])
	}
}

func TestClaudeResponse_Unmarshal(t *testing.T) {
	jsonData := `{
		"id": "msg_123",
		"type": "message",
		"role": "assistant",
		"content": [{"type": "text", "text": "Hello!"}],
		"model": "claude-3-5-sonnet-20241022",
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`

	var resp ClaudeResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("Failed to unmarshal ClaudeResponse: %v", err)
	}

	if resp.ID != "msg_123" {
		t.Errorf("Expected ID=msg_123, got %s", resp.ID)
	}
	if resp.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("Expected model=claude-3-5-sonnet-20241022, got %s", resp.Model)
	}
	if len(resp.Content) != 1 {
		t.Fatalf("Expected 1 content block, got %d", len(resp.Content))
	}
	if resp.Content[0].Text != "Hello!" {
		t.Errorf("Expected text='Hello!', got '%s'", resp.Content[0].Text)
	}
}

func TestIsClaudeModel(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		want      bool
	}{
		{
			name:      "claude-3-5-sonnet",
			modelName: "claude-3-5-sonnet-20241022",
			want:      true,
		},
		{
			name:      "claude-opus-4-6",
			modelName: "claude-opus-4-6",
			want:      true,
		},
		{
			name:      "uppercase CLAUDE",
			modelName: "CLAUDE-OPUS-4-6",
			want:      true,
		},
		{
			name:      "gpt-4",
			modelName: "gpt-4",
			want:      false,
		},
		{
			name:      "empty string",
			modelName: "",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isClaudeModel(tt.modelName)
			if got != tt.want {
				t.Errorf("isClaudeModel(%q) = %v, want %v", tt.modelName, got, tt.want)
			}
		})
	}
}

func TestTransformToClaudeFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   ChatRequest
		want    ClaudeRequest
	}{
		{
			name: "extract system message",
			input: ChatRequest{
				Model: "claude-3-5-sonnet-20241022",
				Messages: []ChatMessage{
					{Role: "system", Content: "You are a helpful assistant."},
					{Role: "user", Content: "Hello"},
					{Role: "assistant", Content: "Hi there!"},
				},
				MaxTokens: 1024,
				Stream:    false,
			},
			want: ClaudeRequest{
				Model:     "claude-3-5-sonnet-20241022",
				System:    "You are a helpful assistant.",
				Messages: []ClaudeMessage{
					{Role: "user", Content: "Hello"},
					{Role: "assistant", Content: "Hi there!"},
				},
				MaxTokens: 1024,
				Stream:    false,
			},
		},
		{
			name: "no system message",
			input: ChatRequest{
				Model: "claude-3-5-sonnet-20241022",
				Messages: []ChatMessage{
					{Role: "user", Content: "Hello"},
				},
				MaxTokens: 2048,
			},
			want: ClaudeRequest{
				Model: "claude-3-5-sonnet-20241022",
				Messages: []ClaudeMessage{
					{Role: "user", Content: "Hello"},
				},
				MaxTokens: 2048,
			},
		},
		{
			name: "multiple system messages",
			input: ChatRequest{
				Model: "claude-3-5-sonnet-20241022",
				Messages: []ChatMessage{
					{Role: "system", Content: "First system message"},
					{Role: "system", Content: "Second system message"},
					{Role: "user", Content: "Hello"},
				},
				MaxTokens: 1024,
			},
			want: ClaudeRequest{
				Model:  "claude-3-5-sonnet-20241022",
				System: "First system message",
				Messages: []ClaudeMessage{
					{Role: "user", Content: "Hello"},
				},
				MaxTokens: 1024,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transformToClaudeFormat(tt.input)

			if got.Model != tt.want.Model {
				t.Errorf("Model = %v, want %v", got.Model, tt.want.Model)
			}
			if got.System != tt.want.System {
				t.Errorf("System = %v, want %v", got.System, tt.want.System)
			}
			if got.MaxTokens != tt.want.MaxTokens {
				t.Errorf("MaxTokens = %v, want %v", got.MaxTokens, tt.want.MaxTokens)
			}
			if got.Stream != tt.want.Stream {
				t.Errorf("Stream = %v, want %v", got.Stream, tt.want.Stream)
			}
			if len(got.Messages) != len(tt.want.Messages) {
				t.Fatalf("Messages length = %v, want %v", len(got.Messages), len(tt.want.Messages))
			}
			for i := range got.Messages {
				if got.Messages[i].Role != tt.want.Messages[i].Role {
					t.Errorf("Messages[%d].Role = %v, want %v", i, got.Messages[i].Role, tt.want.Messages[i].Role)
				}
				if got.Messages[i].Content != tt.want.Messages[i].Content {
					t.Errorf("Messages[%d].Content = %v, want %v", i, got.Messages[i].Content, tt.want.Messages[i].Content)
				}
			}
		})
	}
}

func TestTransformClaudeResponseToOpenAI(t *testing.T) {
	tests := []struct {
		name    string
		input   ClaudeResponse
		want    OpenAIResponse
	}{
		{
			name: "basic response",
			input: ClaudeResponse{
				ID:    "msg_123",
				Model: "claude-3-5-sonnet-20241022",
				Content: []ContentBlock{
					{Type: "text", Text: "Hello!"},
				},
				StopReason: "end_turn",
				Usage: ClaudeUsage{
					InputTokens:  10,
					OutputTokens: 5,
				},
			},
			want: OpenAIResponse{
				ID:      "msg_123",
				Object:  "chat.completion",
				Model:   "claude-3-5-sonnet-20241022",
				Choices: []Choice{
					{
						Index: 0,
						Message: Message{
							Role:    "assistant",
							Content: "Hello!",
						},
						FinishReason: "stop",
					},
				},
				Usage: Usage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
		},
		{
			name: "max_tokens stop reason",
			input: ClaudeResponse{
				ID:    "msg_456",
				Model: "claude-opus-4-6",
				Content: []ContentBlock{
					{Type: "text", Text: "Response text"},
				},
				StopReason: "max_tokens",
				Usage: ClaudeUsage{
					InputTokens:  20,
					OutputTokens: 100,
				},
			},
			want: OpenAIResponse{
				ID:      "msg_456",
				Object:  "chat.completion",
				Model:   "claude-opus-4-6",
				Choices: []Choice{
					{
						Index: 0,
						Message: Message{
							Role:    "assistant",
							Content: "Response text",
						},
						FinishReason: "length",
					},
				},
				Usage: Usage{
					PromptTokens:     20,
					CompletionTokens: 100,
					TotalTokens:      120,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transformClaudeResponseToOpenAI(tt.input)

			if got.ID != tt.want.ID {
				t.Errorf("ID = %v, want %v", got.ID, tt.want.ID)
			}
			if got.Object != tt.want.Object {
				t.Errorf("Object = %v, want %v", got.Object, tt.want.Object)
			}
			if got.Model != tt.want.Model {
				t.Errorf("Model = %v, want %v", got.Model, tt.want.Model)
			}
			if len(got.Choices) != len(tt.want.Choices) {
				t.Fatalf("Choices length = %v, want %v", len(got.Choices), len(tt.want.Choices))
			}
			if got.Choices[0].Index != tt.want.Choices[0].Index {
				t.Errorf("Choices[0].Index = %v, want %v", got.Choices[0].Index, tt.want.Choices[0].Index)
			}
			if got.Choices[0].Message.Role != tt.want.Choices[0].Message.Role {
				t.Errorf("Choices[0].Message.Role = %v, want %v", got.Choices[0].Message.Role, tt.want.Choices[0].Message.Role)
			}
			if got.Choices[0].Message.Content != tt.want.Choices[0].Message.Content {
				t.Errorf("Choices[0].Message.Content = %v, want %v", got.Choices[0].Message.Content, tt.want.Choices[0].Message.Content)
			}
			if got.Choices[0].FinishReason != tt.want.Choices[0].FinishReason {
				t.Errorf("Choices[0].FinishReason = %v, want %v", got.Choices[0].FinishReason, tt.want.Choices[0].FinishReason)
			}
			if got.Usage.PromptTokens != tt.want.Usage.PromptTokens {
				t.Errorf("Usage.PromptTokens = %v, want %v", got.Usage.PromptTokens, tt.want.Usage.PromptTokens)
			}
			if got.Usage.CompletionTokens != tt.want.Usage.CompletionTokens {
				t.Errorf("Usage.CompletionTokens = %v, want %v", got.Usage.CompletionTokens, tt.want.Usage.CompletionTokens)
			}
			if got.Usage.TotalTokens != tt.want.Usage.TotalTokens {
				t.Errorf("Usage.TotalTokens = %v, want %v", got.Usage.TotalTokens, tt.want.Usage.TotalTokens)
			}
		})
	}
}

func TestExtractTextContent(t *testing.T) {
	tests := []struct {
		name    string
		input   []ContentBlock
		want    string
	}{
		{
			name: "single text block",
			input: []ContentBlock{
				{Type: "text", Text: "Hello, world!"},
			},
			want: "Hello, world!",
		},
		{
			name: "multiple blocks - returns first text",
			input: []ContentBlock{
				{Type: "text", Text: "First text"},
				{Type: "text", Text: "Second text"},
			},
			want: "First text",
		},
		{
			name: "empty blocks",
			input: []ContentBlock{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTextContent(tt.input)
			if got != tt.want {
				t.Errorf("extractTextContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMapStopReason(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
	}{
		{
			name:  "end_turn → stop",
			input: "end_turn",
			want:  "stop",
		},
		{
			name:  "max_tokens → length",
			input: "max_tokens",
			want:  "length",
		},
		{
			name:  "stop_sequence → stop",
			input: "stop_sequence",
			want:  "stop",
		},
		{
			name:  "unknown → stop",
			input: "unknown_reason",
			want:  "stop",
		},
		{
			name:  "empty → stop",
			input: "",
			want:  "stop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapStopReason(tt.input)
			if got != tt.want {
				t.Errorf("mapStopReason(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
