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
