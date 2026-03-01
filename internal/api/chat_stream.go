package api

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
)

// streamChatResponse pipes an SSE stream from the upstream provider to the client.
// The upstream response body must be a text/event-stream (OpenAI-compatible SSE format).
// Each SSE event is flushed immediately so the client receives incremental chunks.
func (r *Router) streamChatResponse(w http.ResponseWriter, upstreamResp *http.Response) error {
	defer upstreamResp.Body.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported: ResponseWriter does not implement http.Flusher")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering if proxied
	w.WriteHeader(http.StatusOK)

	scanner := bufio.NewScanner(upstreamResp.Body)
	// Increase buffer size for large tool_calls chunks
	scanner.Buffer(make([]byte, 0, 64*1024), 512*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Write the line followed by newline (SSE format)
		if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
			return fmt.Errorf("failed to write SSE line: %w", err)
		}

		// SSE events are delimited by blank lines — flush after each blank line
		// to send the complete event to the client immediately
		if line == "" {
			flusher.Flush()
		}

		// Stop on the SSE termination signal
		if line == "data: [DONE]" {
			fmt.Fprintf(w, "\n")
			flusher.Flush()
			break
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("SSE stream scanner error: %v", err)
		return fmt.Errorf("SSE stream read error: %w", err)
	}

	return nil
}
