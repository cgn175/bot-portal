package a2a

import (
	"testing"
)

func TestSplitChannelID(t *testing.T) {
	tests := []struct {
		name      string
		channelID string
		senderID  string
		want      string
	}{
		{"agent1 sends to agent2", "agent1::agent2", "agent1", "agent2"},
		{"agent2 sends to agent1", "agent1::agent2", "agent2", "agent1"},
		{"empty sender", "agent1::agent2", "", "agent1"},
		{"no delimiter returns first part", "invalid", "agent1", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitChannelID(tt.channelID, tt.senderID)
			if got != tt.want {
				t.Errorf("splitChannelID(%q, %q) = %q, want %q", tt.channelID, tt.senderID, got, tt.want)
			}
		})
	}
}

func TestGenerateTaskID(t *testing.T) {
	id1 := generateTaskID()
	id2 := generateTaskID()

	if id1 == "" {
		t.Error("generateTaskID returned empty string")
	}

	if id1 == id2 {
		t.Error("generateTaskID should return unique IDs")
	}

	if len(id1) < 10 {
		t.Errorf("Expected task ID length >= 10, got %d", len(id1))
	}
}

func TestMustJSON(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{"simple object", map[string]string{"key": "value"}, `{"key":"value"}`},
		{"empty object", map[string]string{}, `{}`},
		{"nil", nil, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustJSON(tt.input)
			if got != tt.want {
				t.Errorf("mustJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTaskStatus(t *testing.T) {
	statuses := []TaskStatus{
		TaskStatusPending,
		TaskStatusRunning,
		TaskStatusCompleted,
		TaskStatusFailed,
		TaskStatusCancelled,
	}

	for _, status := range statuses {
		if string(status) == "" {
			t.Errorf("TaskStatus should not be empty: %v", status)
		}
	}
}
