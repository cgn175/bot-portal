package docker

import (
	"context"
	"testing"
)

func TestReadWorkspaceFile_AllowsOnlyAllowlistedFiles(t *testing.T) {
	m := &Manager{}
	_, err := m.ReadWorkspaceFile(context.Background(), "agent-1", "../../../etc/passwd")
	if err == nil {
		t.Error("Expected error for path traversal attempt")
	}
}

func TestValidateIdentityFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{"IDENTITY.md", "IDENTITY.md", false},
		{"SOUL.md", "SOUL.md", false},
		{"AGENTS.md", "AGENTS.md", false},
		{"USER.md", "USER.md", false},
		{"TOOLS.md", "TOOLS.md", false},
		{"HEARTBEAT.md", "HEARTBEAT.md", true}, // not in allowlist
		{"../etc/passwd", "../etc/passwd", true},
		{"/etc/passwd", "/etc/passwd", true},
		{"identity.md", "identity.md", true}, // case sensitive
		{"", "", true}, // empty filename
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIdentityFilename(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIdentityFilename(%q) error = %v, wantErr %v", tt.filename, err, tt.wantErr)
			}
		})
	}
}

func TestReadWorkspaceFile_SizeLimit(t *testing.T) {
	// This test validates that files over 16KB are rejected
	// Note: Full integration test would require a running container
	m := &Manager{}

	// Just verify the constant is set correctly
	if MaxIdentityFileSize != 16*1024 {
		t.Errorf("MaxIdentityFileSize = %d, want %d", MaxIdentityFileSize, 16*1024)
	}

	// Verify we get an error for size check with nil client (not a real test, just API validation)
	ctx := context.Background()
	_, err := m.ReadWorkspaceFile(ctx, "agent-1", "IDENTITY.md")
	// Should fail because client is nil
	if err == nil {
		t.Error("Expected error when manager has no client")
	}
}

func TestWriteWorkspaceFile_ValidatesContent(t *testing.T) {
	m := &Manager{}

	// Test with oversized content
	largeContent := make([]byte, MaxIdentityFileSize+1)
	ctx := context.Background()
	err := m.WriteWorkspaceFile(ctx, "agent-1", "IDENTITY.md", largeContent)
	if err == nil {
		t.Error("Expected error for content exceeding size limit")
	}

	// Test with invalid filename
	err = m.WriteWorkspaceFile(ctx, "agent-1", "INVALID.md", []byte("test"))
	if err == nil {
		t.Error("Expected error for invalid filename")
	}
}
