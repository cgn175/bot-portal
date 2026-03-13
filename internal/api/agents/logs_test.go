package agents

import (
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// Secret Redaction Tests
// ============================================================================

func TestRedactSecrets(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "API key in env format",
			input:    "API_KEY=sk-abc123456789abcdef",
			expected: "API_KEY=[REDACTED]",
		},
		{
			name:     "Token in JSON format",
			input:    `{"token": "ghp_12345678901234567890"}`,
			expected: `{"token": [REDACTED]}`,
		},
		{
			name:     "Bearer token in header",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: "Authorization: [REDACTED]",
		},
		{
			name:     "Password in config",
			input:    `password="supersecret123"`,
			expected: `password=[REDACTED]`,
		},
		{
			name:     "No secrets",
			input:    "This is a normal log line without secrets",
			expected: "This is a normal log line without secrets",
		},
		{
			name:     "Multiple secrets",
			input:    `API_KEY=secret123 TOKEN=token456`,
			expected: `API_KEY=[REDACTED] TOKEN=[REDACTED]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactSecrets(tt.input)
			if result != tt.expected {
				t.Errorf("redactSecrets() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseRedactPatterns(t *testing.T) {
	// Test with custom patterns
	customPattern := `(?i)(secret\s*=\s*)\w+`
	os.Setenv("REDACT_PATTERNS", customPattern)
	defer os.Unsetenv("REDACT_PATTERNS")

	// Reset the once to allow re-parsing
	redactPatterns = nil
	redactPatternsOnce = sync.Once{}

	patterns := parseRedactPatterns()
	if len(patterns) != 1 {
		t.Errorf("Expected 1 pattern, got %d", len(patterns))
	}

	// Test the pattern works
	testLine := "secret=hidden123"
	result := redactSecrets(testLine)
	if result != "secret=[REDACTED]" {
		t.Errorf("Custom pattern failed: got %q, want %q", result, "secret=[REDACTED]")
	}
}

// ============================================================================
// Agent ID Validation Tests
// ============================================================================

func TestIsValidAgentID(t *testing.T) {
	tests := []struct {
		name    string
		agentID string
		want    bool
	}{
		{
			name:    "valid simple ID",
			agentID: "my-agent",
			want:    true,
		},
		{
			name:    "valid with underscore",
			agentID: "my_agent_123",
			want:    true,
		},
		{
			name:    "valid alphanumeric",
			agentID: "agent123",
			want:    true,
		},
		{
			name:    "empty ID",
			agentID: "",
			want:    false,
		},
		{
			name:    "too long",
			agentID: strings.Repeat("a", MaxAgentIDLength+1),
			want:    false,
		},
		{
			name:    "invalid characters",
			agentID: "my@agent",
			want:    false,
		},
		{
			name:    "path traversal attempt",
			agentID: "../../../etc/passwd",
			want:    false,
		},
		{
			name:    "shell injection attempt",
			agentID: "agent; rm -rf /",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidAgentID(tt.agentID); got != tt.want {
				t.Errorf("isValidAgentID(%q) = %v, want %v", tt.agentID, got, tt.want)
			}
		})
	}
}

// ============================================================================
// Query Parameter Parsing Tests
// ============================================================================

func TestParseLogQueryParams(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		wantTail     int
		wantSinceSet bool
		wantErr      bool
	}{
		{
			name:     "default values",
			query:    "",
			wantTail: DefaultTailLines,
		},
		{
			name:     "valid tail",
			query:    "tail=500",
			wantTail: 500,
		},
		{
			name:     "tail clamped to max",
			query:    "tail=999999",
			wantTail: MaxTailLines,
		},
		{
			name:     "valid since",
			query:    "since=2024-01-15T10:30:00Z",
			wantTail: DefaultTailLines,
			wantSinceSet: true,
		},
		{
			name:    "invalid tail",
			query:   "tail=invalid",
			wantErr: true,
		},
		{
			name:    "negative tail",
			query:   "tail=-10",
			wantErr: true,
		},
		{
			name:    "invalid since format",
			query:   "since=not-a-date",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/test?"+tt.query, nil)
			tail, since, err := parseLogQueryParams(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLogQueryParams() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tail != tt.wantTail {
				t.Errorf("parseLogQueryParams() tail = %v, want %v", tail, tt.wantTail)
			}
			if tt.wantSinceSet && since.IsZero() {
				t.Errorf("parseLogQueryParams() since should be set")
			}
		})
	}
}

// ============================================================================
// Docker Log Line Parsing Tests
// ============================================================================

func TestParseDockerLogLine(t *testing.T) {
	tests := []struct {
		name            string
		line            string
		wantMessage     string
		wantTimePresent bool
	}{
		{
			name:            "standard Docker log line",
			line:            "2024-01-15T10:30:00.123456789Z This is a log message",
			wantMessage:     "This is a log message",
			wantTimePresent: true,
		},
		{
			name:            "log without timestamp",
			line:            "This is a raw log message",
			wantMessage:     "This is a raw log message",
			wantTimePresent: false,
		},
		{
			name:            "empty line",
			line:            "",
			wantMessage:     "",
			wantTimePresent: false,
		},
		{
			name:            "line with multiple spaces",
			line:            "2024-01-15T10:30:00Z message with spaces here",
			wantMessage:     "message with spaces here",
			wantTimePresent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, message := parseDockerLogLine(tt.line)
			if message != tt.wantMessage {
				t.Errorf("parseDockerLogLine() message = %q, want %q", message, tt.wantMessage)
			}
			if tt.wantTimePresent && ts.IsZero() {
				t.Errorf("parseDockerLogLine() timestamp should be present")
			}
		})
	}
}

// ============================================================================
// Environment Helper Tests
// ============================================================================

func TestGetEnvInt(t *testing.T) {
	// Test with set value
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	if got := getEnvInt("TEST_INT", 10); got != 42 {
		t.Errorf("getEnvInt() = %v, want %v", got, 42)
	}

	// Test with default value
	if got := getEnvInt("TEST_INT_UNSET", 10); got != 10 {
		t.Errorf("getEnvInt() = %v, want %v", got, 10)
	}

	// Test with invalid value
	os.Setenv("TEST_INT_INVALID", "not-a-number")
	defer os.Unsetenv("TEST_INT_INVALID")

	if got := getEnvInt("TEST_INT_INVALID", 10); got != 10 {
		t.Errorf("getEnvInt() = %v, want %v", got, 10)
	}

	// Test with negative value (should use default)
	os.Setenv("TEST_INT_NEGATIVE", "-5")
	defer os.Unsetenv("TEST_INT_NEGATIVE")

	if got := getEnvInt("TEST_INT_NEGATIVE", 10); got != 10 {
		t.Errorf("getEnvInt() = %v, want %v", got, 10)
	}
}

func TestGetEnvDuration(t *testing.T) {
	// Test with set value
	os.Setenv("TEST_DURATION", "5m")
	defer os.Unsetenv("TEST_DURATION")

	if got := getEnvDuration("TEST_DURATION", time.Minute); got != 5*time.Minute {
		t.Errorf("getEnvDuration() = %v, want %v", got, 5*time.Minute)
	}

	// Test with default value
	if got := getEnvDuration("TEST_DURATION_UNSET", time.Minute); got != time.Minute {
		t.Errorf("getEnvDuration() = %v, want %v", got, time.Minute)
	}

	// Test with invalid value
	os.Setenv("TEST_DURATION_INVALID", "not-a-duration")
	defer os.Unsetenv("TEST_DURATION_INVALID")

	if got := getEnvDuration("TEST_DURATION_INVALID", time.Minute); got != time.Minute {
		t.Errorf("getEnvDuration() = %v, want %v", got, time.Minute)
	}
}

// ============================================================================
// Stream Tracker Tests
// ============================================================================

func TestStreamTracker(t *testing.T) {
	st := &streamTracker{
		userStreams: make(map[string]int),
		globalCount: 0,
		maxPerUser:  2,
		maxGlobal:   3,
	}

	user1 := "user1"
	user2 := "user2"

	// Acquire for user1 - should succeed
	if !st.acquire(user1) {
		t.Error("First acquire for user1 should succeed")
	}

	// Second acquire for user1 - should succeed
	if !st.acquire(user1) {
		t.Error("Second acquire for user1 should succeed")
	}

	// Third acquire for user1 - should fail (per-user limit)
	if st.acquire(user1) {
		t.Error("Third acquire for user1 should fail (per-user limit)")
	}

	// Acquire for user2 - should succeed (global not reached)
	if !st.acquire(user2) {
		t.Error("First acquire for user2 should succeed")
	}

	// Second acquire for user2 - should fail (global limit)
	if st.acquire(user2) {
		t.Error("Second acquire for user2 should fail (global limit)")
	}

	// Release for user1
	st.release(user1)

	// Now acquire for user2 should succeed
	if !st.acquire(user2) {
		t.Error("Acquire for user2 should succeed after release")
	}

	// Clean up
	st.release(user1)
	st.release(user2)
}

// ============================================================================
// Utility Function Tests
// ============================================================================

func TestMustJSON(t *testing.T) {
	// Test with valid struct
	result := mustJSON(map[string]string{"key": "value"})
	if !strings.Contains(result, `"key":"value"`) {
		t.Errorf("mustJSON() = %q, expected to contain key:value", result)
	}

	// Test with valid slice
	result = mustJSON([]string{"a", "b", "c"})
	if result != `["a","b","c"]` {
		t.Errorf("mustJSON() = %q, want %q", result, `["a","b","c"]`)
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{1, 2, 1},
		{2, 1, 1},
		{5, 5, 5},
		{-1, 1, -1},
	}

	for _, tt := range tests {
		if got := min(tt.a, tt.b); got != tt.want {
			t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

// ============================================================================
// User Extraction Tests
// ============================================================================

func TestExtractUserFromRequest(t *testing.T) {
	tests := []struct {
		name       string
		setupReq   func(*http.Request)
		wantPrefix string
	}{
		{
			name:       "Fallback to IP",
			setupReq:   func(req *http.Request) {},
			wantPrefix: "ip:",
		},
		{
			name: "X-Forwarded-For header",
			setupReq: func(req *http.Request) {
				req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
			},
			wantPrefix: "ip:10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "127.0.0.1:12345"
			tt.setupReq(req)

			userID := extractUserFromRequest(req)
			if !strings.HasPrefix(userID, tt.wantPrefix) {
				t.Errorf("extractUserFromRequest() = %q, want prefix %q", userID, tt.wantPrefix)
			}
		})
	}
}

// ============================================================================
// Regex Pattern Tests
// ============================================================================

func TestValidAgentIDPattern(t *testing.T) {
	// Test the pattern directly
	validIDs := []string{
		"agent",
		"agent-1",
		"agent_1",
		"Agent123",
		"a",
		strings.Repeat("a", MaxAgentIDLength),
	}

	invalidIDs := []string{
		"",
		"agent@domain",
		"agent.name",
		"agent/name",
		"agent:name",
		"agent;cmd",
		"agent|pipe",
		"agent$(cmd)",
		`agent"quote`,
		"agent'quote",
		"agent space",
		strings.Repeat("a", MaxAgentIDLength+1),
	}

	for _, id := range validIDs {
		if !ValidAgentIDPattern.MatchString(id) {
			t.Errorf("ValidAgentIDPattern should match %q", id)
		}
	}

	for _, id := range invalidIDs {
		if ValidAgentIDPattern.MatchString(id) {
			t.Errorf("ValidAgentIDPattern should NOT match %q", id)
		}
	}
}
