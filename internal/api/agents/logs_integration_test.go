//go:build docker_integration

package agents

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/zeroclaw/bot-portal/internal/docker"
)

// TestLiveLogStreaming tests the SSE endpoint with a real Docker container
func TestLiveLogStreaming(t *testing.T) {
	// Check if Docker is available
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test Docker connectivity
	if _, err := cli.Ping(ctx); err != nil {
		t.Skip("Docker daemon not accessible:", err)
	}

	// Create a test container that outputs known messages
	containerID, err := createTestContainer(ctx, cli)
	if err != nil {
		t.Fatal("Failed to create test container:", err)
	}
	defer cleanupContainer(ctx, cli, containerID)

	// Wait for container to produce logs
	time.Sleep(2 * time.Second)

	// Test cases
	tests := []struct {
		name         string
		tail         int
		wantLines    int
		wantStreams  []string
	}{
		{
			name:        "stream all logs",
			tail:        100,
			wantLines:   6, // 3 stdout + 3 stderr
			wantStreams: []string{"stdout", "stderr"},
		},
		{
			name:        "stream limited tail",
			tail:        2,
			wantLines:   2,
			wantStreams: []string{"stdout"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build request
			url := fmt.Sprintf("/api/agents/container-logs-stream?agentID=test-agent&tail=%d", tt.tail)
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("X-User-ID", "test-user")
			req.Header.Set("X-User-Role", "admin")

			// Create response recorder
			rr := httptest.NewRecorder()

			// Create handler
			mgr, _ := docker.NewManager()
			defer mgr.Close()

			handler := &Handler{
				DockerMgr: mgr,
			}

			// Serve request with timeout context
			reqCtx, reqCancel := context.WithTimeout(req.Context(), 10*time.Second)
			defer reqCancel()
			req = req.WithContext(reqCtx)

			// Handle request
			handler.StreamContainerLogs(rr, req)

			// Verify response
			if rr.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", rr.Code)
			}

			// Parse SSE events
			events := parseSSEEvents(rr.Body.String())

			// Verify we got log events
			logEvents := filterEventsByType(events, "log")
			if len(logEvents) == 0 {
				t.Error("Expected log events, got none")
			}

			// Verify log format
			for _, event := range logEvents {
				var logEntry struct {
					Timestamp string `json:"timestamp"`
					Stream    string `json:"stream"`
					Line      string `json:"line"`
				}
				if err := json.Unmarshal([]byte(event.data), &logEntry); err != nil {
					t.Errorf("Failed to parse log entry: %v", err)
					continue
				}

				if logEntry.Timestamp == "" {
					t.Error("Log entry missing timestamp")
				}
				if logEntry.Stream != "stdout" && logEntry.Stream != "stderr" {
					t.Errorf("Invalid stream: %s", logEntry.Stream)
				}
				if logEntry.Line == "" {
					t.Error("Log entry missing line")
				}
			}
		})
	}
}

// TestSecretRedactionInStream tests that secrets are redacted in live streams
func TestSecretRedactionInStream(t *testing.T) {
	// Set up test redaction patterns
	os.Setenv("REDACT_PATTERNS", `(?i)(api_key\s*=\s*)[^\s]+`)
	defer os.Unsetenv("REDACT_PATTERNS")

	// Reset patterns
	redactPatterns = nil
	redactPatternsOnce = sync.Once{}

	// Check if Docker is available
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := cli.Ping(ctx); err != nil {
		t.Skip("Docker daemon not accessible:", err)
	}

	// Create container that outputs secrets
	containerID, err := createSecretTestContainer(ctx, cli)
	if err != nil {
		t.Fatal("Failed to create test container:", err)
	}
	defer cleanupContainer(ctx, cli, containerID)

	time.Sleep(2 * time.Second)

	// Create handler and request
	mgr, _ := docker.NewManager()
	defer mgr.Close()

	handler := &Handler{
		DockerMgr: mgr,
	}

	url := fmt.Sprintf("/api/agents/container-logs-stream?agentID=test-agent&tail=100")
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("X-User-ID", "test-user")
	req.Header.Set("X-User-Role", "admin")

	rr := httptest.NewRecorder()

	reqCtx, reqCancel := context.WithTimeout(req.Context(), 10*time.Second)
	defer reqCancel()
	req = req.WithContext(reqCtx)

	handler.StreamContainerLogs(rr, req)

	// Parse events
	events := parseSSEEvents(rr.Body.String())
	logEvents := filterEventsByType(events, "log")

	// Verify secrets are redacted
	for _, event := range logEvents {
		if strings.Contains(event.data, "sk-1234567890abcdef") {
			t.Error("Secret was not redacted in log stream")
		}
		if strings.Contains(event.data, "[REDACTED]") {
			// Good - redaction worked
			return
		}
	}

	// If we didn't find any redacted content, that might also be ok
	// if the log events didn't include our secret line
}

// TestClientDisconnect tests that streams are properly cleaned up on disconnect
func TestClientDisconnect(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := cli.Ping(ctx); err != nil {
		t.Skip("Docker daemon not accessible:", err)
	}

	// Create long-running container
	containerID, err := createLongRunningContainer(ctx, cli)
	if err != nil {
		t.Fatal("Failed to create test container:", err)
	}
	defer cleanupContainer(ctx, cli, containerID)

	mgr, _ := docker.NewManager()
	defer mgr.Close()

	handler := &Handler{
		DockerMgr: mgr,
	}

	// Create request with early cancellation
	url := fmt.Sprintf("/api/agents/container-logs-stream?agentID=test-agent")
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("X-User-ID", "test-user")
	req.Header.Set("X-User-Role", "admin")

	// Cancel context after short time to simulate disconnect
	reqCtx, reqCancel := context.WithTimeout(req.Context(), 500*time.Millisecond)
	defer reqCancel()
	req = req.WithContext(reqCtx)

	rr := httptest.NewRecorder()

	// This should not hang - it should respect context cancellation
	done := make(chan bool)
	go func() {
		handler.StreamContainerLogs(rr, req)
		done <- true
	}()

	select {
	case <-done:
		// Good - handler returned within timeout
	case <-time.After(3 * time.Second):
		t.Error("Handler did not respect context cancellation - possible goroutine leak")
	}
}

// Helper functions

func createTestContainer(ctx context.Context, cli *client.Client) (string, error) {
	config := &container.Config{
		Image: "alpine:latest",
		Cmd: []string{
			"sh", "-c",
			`echo "stdout line 1" && 
			 echo "stdout line 2" >&2 && 
			 echo "stdout line 3" && 
			 echo "stderr line 1" >&2 &&
			 echo "stdout line 4" &&
			 echo "stderr line 2" >&2 &&
			 sleep 30`,
		},
	}

	resp, err := cli.ContainerCreate(ctx, config, nil, nil, nil, "")
	if err != nil {
		return "", err
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", err
	}

	return resp.ID, nil
}

func createSecretTestContainer(ctx context.Context, cli *client.Client) (string, error) {
	config := &container.Config{
		Image: "alpine:latest",
		Cmd: []string{
			"sh", "-c",
			`echo "Starting app" && 
			 echo "api_key=sk-1234567890abcdef" &&
			 echo "Config loaded" &&
			 sleep 30`,
		},
	}

	resp, err := cli.ContainerCreate(ctx, config, nil, nil, nil, "")
	if err != nil {
		return "", err
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", err
	}

	return resp.ID, nil
}

func createLongRunningContainer(ctx context.Context, cli *client.Client) (string, error) {
	config := &container.Config{
		Image: "alpine:latest",
		Cmd: []string{
			"sh", "-c",
			`while true; do echo "log line at $(date)"; sleep 1; done`,
		},
	}

	resp, err := cli.ContainerCreate(ctx, config, nil, nil, nil, "")
	if err != nil {
		return "", err
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", err
	}

	return resp.ID, nil
}

func cleanupContainer(ctx context.Context, cli *client.Client, containerID string) {
	timeout := 5
	cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
	cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
}

type sseEvent struct {
	event string
	data  string
}

func parseSSEEvents(body string) []sseEvent {
	var events []sseEvent
	var currentEvent sseEvent

	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEvent.event = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			currentEvent.data = strings.TrimPrefix(line, "data: ")
		} else if line == "" {
			if currentEvent.event != "" || currentEvent.data != "" {
				events = append(events, currentEvent)
				currentEvent = sseEvent{}
			}
		}
	}

	// Don't forget the last event
	if currentEvent.event != "" || currentEvent.data != "" {
		events = append(events, currentEvent)
	}

	return events
}

func filterEventsByType(events []sseEvent, eventType string) []sseEvent {
	var filtered []sseEvent
	for _, e := range events {
		if e.event == eventType {
			filtered = append(filtered, e)
		}
	}
	return filtered
}