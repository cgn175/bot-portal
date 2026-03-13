package agents

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/zeroclaw/bot-portal/internal/store"
)

// ============================================================================
// Configuration Constants
// ============================================================================

const (
	// DefaultTailLines is the default number of log lines to stream
	DefaultTailLines = 100

	// MaxTailLines is the maximum number of log lines allowed in tail parameter
	MaxTailLines = 10000

	// HeartbeatInterval is the interval between SSE heartbeat comments
	HeartbeatInterval = 15 * time.Second

	// DefaultLogStreamMaxDuration is the default maximum duration for a log stream session
	DefaultLogStreamMaxDuration = 30 * time.Minute

	// DefaultLogStreamMaxBytes is the default maximum bytes per connection
	DefaultLogStreamMaxBytes = 10 * 1024 * 1024 // 10MB

	// DefaultMaxLogStreamsPerUser is the default maximum concurrent streams per user
	DefaultMaxLogStreamsPerUser = 5

	// DefaultMaxLogStreamsGlobal is the default maximum concurrent streams globally
	DefaultMaxLogStreamsGlobal = 20

	// LogStreamBufferSize is the buffer size for reading log lines
	LogStreamBufferSize = 64 * 1024

	// LogStreamMaxLineSize is the maximum size of a single log line
	LogStreamMaxLineSize = 256 * 1024
)

// ============================================================================
// Rate Limiting
// ============================================================================

var (
	// logStreamTracker tracks active log streams for rate limiting
	logStreamTracker = &streamTracker{
		userStreams:  make(map[string]int),
		globalCount:  0,
		maxPerUser:   getEnvInt("MAX_LOG_STREAMS_PER_USER", DefaultMaxLogStreamsPerUser),
		maxGlobal:    getEnvInt("MAX_LOG_STREAMS_GLOBAL", DefaultMaxLogStreamsGlobal),
	}

	// redactPatterns holds compiled regex patterns for secret redaction
	redactPatterns     []*regexp.Regexp
	redactPatternsOnce sync.Once
)

// streamTracker tracks active log streams for rate limiting
type streamTracker struct {
	mu          sync.RWMutex
	userStreams map[string]int
	globalCount int
	maxPerUser  int
	maxGlobal   int
}

// acquire attempts to acquire a stream slot for the given user
func (st *streamTracker) acquire(userID string) bool {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.globalCount >= st.maxGlobal {
		return false
	}

	if st.userStreams[userID] >= st.maxPerUser {
		return false
	}

	st.userStreams[userID]++
	st.globalCount++
	return true
}

// release releases a stream slot for the given user
func (st *streamTracker) release(userID string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.userStreams[userID] > 0 {
		st.userStreams[userID]--
		if st.userStreams[userID] == 0 {
			delete(st.userStreams, userID)
		}
	}

	if st.globalCount > 0 {
		st.globalCount--
	}
}

// getEnvInt gets an integer value from environment variable with a default
func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			return parsed
		}
	}
	return defaultVal
}

// getEnvDuration gets a duration value from environment variable with a default
func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil && parsed > 0 {
			return parsed
		}
	}
	return defaultVal
}

// ============================================================================
// Secret Redaction
// ============================================================================

// parseRedactPatterns parses comma-separated regex patterns from environment variable
func parseRedactPatterns() []*regexp.Regexp {
	redactPatternsOnce.Do(func() {
		patternsStr := os.Getenv("REDACT_PATTERNS")
		if patternsStr == "" {
			// Default patterns for common secrets
			patternsStr = `(?i)(api[_-]?key\s*[=:]\s*)["']?[a-zA-Z0-9_\-]{16,}["']?,(?i)(token\s*[=:]\s*)["']?[a-zA-Z0-9_\-]{16,}["']?,(?i)(password\s*[=:]\s*)["'][^"']{4,}["']?,(?i)(bearer\s+)[a-zA-Z0-9_\-\.]{20,},(?i)(authorization\s*[=:]\s*bearer\s+)[a-zA-Z0-9_\-\.]{20,}`
		}

		for _, pattern := range strings.Split(patternsStr, ",") {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				log.Printf("[warn] Invalid redact pattern %q: %v", pattern, err)
				continue
			}
			redactPatterns = append(redactPatterns, re)
		}
	})
	return redactPatterns
}

// redactSecrets applies redaction patterns to a log line
func redactSecrets(line string) string {
	patterns := parseRedactPatterns()
	for _, re := range patterns {
		line = re.ReplaceAllString(line, "${1}[REDACTED]")
	}
	return line
}

// ============================================================================
// Agent ID Validation
// ============================================================================

// isValidAgentID validates an agent ID against the allowed pattern
func isValidAgentID(agentID string) bool {
	if len(agentID) < MinAgentIDLength || len(agentID) > MaxAgentIDLength {
		return false
	}
	return ValidAgentIDPattern.MatchString(agentID)
}

// ============================================================================
// Log Stream Handler
// ============================================================================

// LogEvent represents a single log line event for SSE
type LogEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Stream    string    `json:"stream"`
	Line      string    `json:"line"`
}

// StreamContainerLogs handles SSE streaming of Docker container logs
// Endpoint: GET /api/agents/container-logs-stream?agentID=xxx
func (h *Handler) StreamContainerLogs(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	startTime := time.Now()

	// Extract and validate agent ID from query parameter
	agentID := req.URL.Query().Get("agentID")
	if agentID == "" {
		http.Error(w, "Missing required query parameter: agentID", http.StatusBadRequest)
		return
	}

	// Validate agent ID format
	if !isValidAgentID(agentID) {
		http.Error(w, "Invalid agent ID format", http.StatusBadRequest)
		return
	}

	// Extract user identifier (for rate limiting purposes only - auth disabled)
	userID := extractUserFromRequest(req)

	// Check rate limits
	if !logStreamTracker.acquire(userID) {
		http.Error(w, "Rate limit exceeded: too many concurrent log streams", http.StatusTooManyRequests)
		return
	}
	defer logStreamTracker.release(userID)

	// Get agent and verify it exists
	agent, err := h.AgentStore.GetByID(agentID)
	if err != nil {
		log.Printf("[error] Failed to get agent %s: %v", agentID, err)
		http.Error(w, "Failed to retrieve agent", http.StatusInternalServerError)
		return
	}
	if agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	// Verify agent type supports logs
	if agent.AgentType != "docker" {
		http.Error(w, "Logs only available for Docker agents", http.StatusBadRequest)
		return
	}

	// Permission check bypassed - logs are publicly accessible

	// Parse query parameters
	tail, since, err := parseLogQueryParams(req)
	if err != nil {
		http.Error(w, "Invalid query parameters: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Determine container ID to use
	containerID := agent.ContainerID
	if containerID == "" {
		http.Error(w, "Agent has no running container", http.StatusNotFound)
		return
	}

	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Get max duration and bytes from config
	maxDuration := getEnvDuration("LOG_STREAM_MAX_DURATION", DefaultLogStreamMaxDuration)
	maxBytes := int64(getEnvInt("LOG_STREAM_MAX_BYTES", DefaultLogStreamMaxBytes))

	// Create context with timeout
	streamCtx, cancel := context.WithTimeout(ctx, maxDuration)
	defer cancel()

	// Log audit event (NO tokens logged)
	log.Printf("[audit] Log stream started: agent=%s user=%s container=%s tail=%d since=%v",
		agentID, userID, containerID[:min(12, len(containerID))], tail, since)

	// Track bytes sent
	var bytesSent int64

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\n")
	fmt.Fprintf(w, "data: %s\n\n", mustJSON(map[string]interface{}{
		"agentId":     agentID,
		"containerId": containerID[:min(12, len(containerID))],
		"tail":        tail,
		"since":       since,
	}))
	flusher.Flush()

	// Start heartbeat goroutine
	heartbeatDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(HeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				select {
				case <-streamCtx.Done():
					return
				case <-heartbeatDone:
					return
				default:
					fmt.Fprintf(w, ":heartbeat\n\n")
					flusher.Flush()
				}
			case <-streamCtx.Done():
				return
			case <-heartbeatDone:
				return
			}
		}
	}()

	// Stream logs from Docker
	err = h.streamDockerLogs(streamCtx, w, flusher, containerID, tail, since, maxBytes, &bytesSent)

	// Signal heartbeat to stop
	close(heartbeatDone)

	// Log audit event for stream end (NO tokens logged)
	duration := time.Since(startTime)
	if err != nil {
		log.Printf("[audit] Log stream ended with error: agent=%s user=%s duration=%v bytes=%d error=%v",
			agentID, userID, duration, bytesSent, err)
		// Send error event to client
		fmt.Fprintf(w, "event: error\n")
		fmt.Fprintf(w, "data: %s\n\n", mustJSON(map[string]string{"error": err.Error()}))
		flusher.Flush()
	} else {
		log.Printf("[audit] Log stream ended: agent=%s user=%s duration=%v bytes=%d",
			agentID, userID, duration, bytesSent)
		// Send completion event
		fmt.Fprintf(w, "event: complete\n")
		fmt.Fprintf(w, "data: %s\n\n", mustJSON(map[string]interface{}{
			"bytesTransferred": bytesSent,
			"duration":         duration.String(),
		}))
		flusher.Flush()
	}
}

// streamDockerLogs streams logs from a Docker container
func (h *Handler) streamDockerLogs(
	ctx context.Context,
	w http.ResponseWriter,
	flusher http.Flusher,
	containerID string,
	tail int,
	since time.Time,
	maxBytes int64,
	bytesSent *int64,
) error {
	// Build log options
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
	}

	if tail > 0 {
		opts.Tail = strconv.Itoa(tail)
	}

	if !since.IsZero() {
		opts.Since = since.Format(time.RFC3339)
	}

	// Get log stream from Docker
	reader, err := h.DockerMgr.RawContainerLogs(ctx, containerID, opts)
	if err != nil {
		return fmt.Errorf("failed to get container logs: %w", err)
	}
	defer reader.Close()

	// Docker multiplexes stdout/stderr with an 8-byte header per frame.
	// We need to demux the stream to separate stdout and stderr.
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	// Channel to signal when copying is done
	done := make(chan error, 1)
	go func() {
		_, err := stdcopy.StdCopy(stdoutWriter, stderrWriter, reader)
		stdoutWriter.Close()
		stderrWriter.Close()
		done <- err
	}()

	// Create scanners for stdout and stderr
	stdoutScanner := bufio.NewScanner(stdoutReader)
	stderrScanner := bufio.NewScanner(stderrReader)
	stdoutScanner.Buffer(make([]byte, LogStreamBufferSize), LogStreamMaxLineSize)
	stderrScanner.Buffer(make([]byte, LogStreamBufferSize), LogStreamMaxLineSize)

	// Channel for log lines
	type logLine struct {
		stream string
		line   string
	}
	lines := make(chan logLine, 100)

	// Start goroutines to scan stdout and stderr
	var wg sync.WaitGroup
	wg.Add(2)

	scanFunc := func(scanner *bufio.Scanner, stream string) {
		defer wg.Done()
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case lines <- logLine{stream: stream, line: scanner.Text()}:
			}
		}
	}

	go scanFunc(stdoutScanner, "stdout")
	go scanFunc(stderrScanner, "stderr")

	// Close lines channel when both scanners are done
	go func() {
		wg.Wait()
		close(lines)
	}()

	// Process log lines
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case line, ok := <-lines:
			if !ok {
				// All lines processed
				return nil
			}

			// Parse Docker timestamp from line (format: 2024-01-15T10:30:00.123456789Z ...)
			timestamp, message := parseDockerLogLine(line.line)

			// Apply secret redaction
			message = redactSecrets(message)

			// Create log event
			event := LogEvent{
				Timestamp: timestamp,
				Stream:    line.stream,
				Line:      message,
			}

			// Serialize to JSON
			data, err := json.Marshal(event)
			if err != nil {
				log.Printf("[error] Failed to marshal log event: %v", err)
				continue
			}

			// Check byte limit
			eventBytes := int64(len(data) + 50) // Approximate SSE overhead
			if atomic.AddInt64(bytesSent, eventBytes) > maxBytes {
				return fmt.Errorf("maximum byte limit reached")
			}

			// Send SSE event
			fmt.Fprintf(w, "event: log\n")
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case err := <-done:
			if err != nil && err != io.EOF {
				return fmt.Errorf("error reading container logs: %w", err)
			}
			return nil
		}
	}
}

// parseDockerLogLine parses a Docker log line with timestamp
func parseDockerLogLine(line string) (time.Time, string) {
	// Docker timestamps are in RFC3339Nano format: 2024-01-15T10:30:00.123456789Z
	if idx := strings.IndexByte(line, ' '); idx > 0 {
		ts := line[:idx]
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			return t, line[idx+1:]
		}
	}
	return time.Now(), line
}

// ============================================================================
// Helper Functions
// ============================================================================

// extractUserFromRequest extracts a user identifier from the request for rate limiting
// Authentication is disabled - always returns an IP-based identifier
func extractUserFromRequest(req *http.Request) string {
	// Use IP address for rate limiting identifier
	ip := req.RemoteAddr
	if forwarded := req.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip = strings.Split(forwarded, ",")[0]
		ip = strings.TrimSpace(ip)
	}

	if ip == "" {
		return "anonymous"
	}

	return "ip:" + ip
}

// canViewAgentLogs checks if the requestor can view the agent's logs
// Authentication is disabled - always returns true to allow public access
func (h *Handler) canViewAgentLogs(req *http.Request, agent *store.Agent) bool {
	// Authentication disabled - logs are publicly accessible
	return true
}

// parseLogQueryParams parses the tail and since query parameters
func parseLogQueryParams(req *http.Request) (tail int, since time.Time, err error) {
	// Parse tail parameter
	tail = DefaultTailLines
	if tailStr := req.URL.Query().Get("tail"); tailStr != "" {
		parsed, err := strconv.Atoi(tailStr)
		if err != nil || parsed < 0 {
			return 0, time.Time{}, fmt.Errorf("invalid tail value")
		}
		if parsed > MaxTailLines {
			parsed = MaxTailLines
		}
		tail = parsed
	}

	// Parse since parameter
	if sinceStr := req.URL.Query().Get("since"); sinceStr != "" {
		parsed, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			return 0, time.Time{}, fmt.Errorf("invalid since format (use RFC3339)")
		}
		since = parsed
	}

	return tail, since, nil
}

// mustJSON marshals a value to JSON, panicking on error (safe for known types)
func mustJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
