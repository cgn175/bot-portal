package api

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zeroclaw/bot-portal/internal/a2a"
	"github.com/zeroclaw/bot-portal/internal/docker"
	"github.com/zeroclaw/bot-portal/internal/store"
)

// Router is the main HTTP router
type Router struct {
	db                 *sql.DB
	dockerMgr          *docker.Manager
	agentStore         *store.AgentStore
	channelStore       *store.ChannelStore
	messageStore       *store.MessageStore
	modelStore         *store.ModelStore
	authConfigStore    *store.AuthConfigStore
	a2aRouter          *a2a.Router
	skipModelDiscovery bool // For testing: skip async model discovery

	// SSE connections for frontend task streaming
	taskStreamMu    sync.RWMutex
	taskStreamConns map[string]map[string]chan *a2a.TaskUpdate // taskID -> addr -> channel
}

// NewRouter creates a new API router
func NewRouter(db *sql.DB, dockerMgr *docker.Manager) *Router {
	agentStore := store.NewAgentStore(db)
	channelStore := store.NewChannelStore(db)
	messageStore := store.NewMessageStore(db)
	modelStore := store.NewModelStore(db)
	authConfigStore := store.NewAuthConfigStore(db)

	router := &Router{
		db:              db,
		dockerMgr:       dockerMgr,
		agentStore:      agentStore,
		channelStore:    channelStore,
		messageStore:    messageStore,
		modelStore:      modelStore,
		authConfigStore: authConfigStore,
		taskStreamConns: make(map[string]map[string]chan *a2a.TaskUpdate),
	}

	// Initialize A2A router
	a2aRouter := a2a.NewRouter()
	a2aRouter.CreateTask = router.createTask
	a2aRouter.GetTask = router.getTask
	a2aRouter.UpdateTaskStatus = router.updateTaskStatus
	a2aRouter.AppendMessage = router.appendMessage
	a2aRouter.GetAgents = router.getAgentsForRouting
	router.a2aRouter = a2aRouter

	// Ensure general channel exists
	channelStore.EnsureGeneralChannel()

	// Ensure Docker network exists
	if dockerMgr != nil {
		ctx := context.Background()
		if err := dockerMgr.EnsureNetwork(ctx); err != nil {
			log.Printf("Warning: Failed to ensure Docker network: %v", err)
		}
	}

	return router
}

// Run starts the HTTP server
func (r *Router) Run(addr string) error {
	mux := http.NewServeMux()

	// A2A endpoints (Google A2A Protocol)
	mux.HandleFunc("/.well-known/agent.json", r.handleAgentCard)
	mux.HandleFunc("/tasks", r.requireBearerToken(r.handleTasks))
	mux.HandleFunc("/tasks/", r.requireBearerToken(r.handleTaskDetail))

	// REST API endpoints
	// Agent management
	mux.HandleFunc("/api/agents", r.handleAgents)
	mux.HandleFunc("/api/agents/", r.handleAgentDetail)
	mux.HandleFunc("/api/agents-stream", r.streamAgents)

	// Identity file endpoints
	mux.HandleFunc("/api/agents/", func(w http.ResponseWriter, req *http.Request) {
		// Check if this is an identity-files request
		if strings.Contains(req.URL.Path, "/identity-files/") {
			r.handleAgentIdentityFileDetail(w, req)
		} else if strings.HasSuffix(req.URL.Path, "/identity-files") {
			r.handleAgentIdentityFiles(w, req)
		} else {
			r.handleAgentDetail(w, req)
		}
	})

	// Channel management
	mux.HandleFunc("/api/channels", r.handleChannels)
	mux.HandleFunc("/api/channels/", r.handleChannelDetail)

	// Messages
	mux.HandleFunc("/api/messages/stream", r.handleMessageStream)

	// Task stream (frontend SSE for chat task updates)
	mux.HandleFunc("/api/tasks/", r.handleTaskStream)

	// Model management
	mux.HandleFunc("/api/models", r.handleModels)
	mux.HandleFunc("/api/models/", r.handleModelDetail)

	// Auth Config management (placeholder for task 7)
	mux.HandleFunc("/api/auth-configs", r.handleAuthConfigs)
	mux.HandleFunc("/api/auth-configs/", r.handleAuthConfigDetail)

	// GitHub Copilot OAuth Device Flow
	mux.HandleFunc("/api/auth/copilot/device-code", r.handleCopilotDeviceCode)
	mux.HandleFunc("/api/auth/copilot/token", r.handleCopilotToken)
	mux.HandleFunc("/api/auth/copilot/models", r.handleCopilotModels)

	// Generic model discovery (works for any auth config type)
	mux.HandleFunc("/api/auth/discover-models", r.handleDiscoverModels)

	// Chat completions endpoint for testing models
	mux.HandleFunc("/api/chat/completions", r.handleChatCompletions)

	// CopilotKit self-hosted runtime endpoints
	mux.HandleFunc("/api/copilotkit/chat/completions", r.handleChatCompletions)
	mux.HandleFunc("/api/copilotkit/info", r.handleCopilotKitInfo)

	// Provider registry
	mux.HandleFunc("/api/providers", r.handleProviders)
	mux.HandleFunc("/api/providers/", r.handleProviderDetail)

	// Claude API endpoint
	mux.HandleFunc("POST /api/claude", r.handleClaudeMessages)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("OK"))
	})

	// Wrap with middleware (CORS first, then logging)
	handler := r.corsMiddleware(mux)
	handler = r.loggingMiddleware(handler)

	log.Printf("Server starting on %s", addr)
	return http.ListenAndServe(addr, handler)
}

// ============================================================================
// Middleware
// ============================================================================

// corsMiddleware adds CORS headers for frontend dev
func (r *Router) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if req.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, req)
	})
}

// responseRecorder wraps http.ResponseWriter to capture status code
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.statusCode = code
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Flush() {
	if flusher, ok := rr.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// loggingMiddleware logs HTTP requests with method, path, status, and duration
func (r *Router) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		rr := newResponseRecorder(w)

		next.ServeHTTP(rr, req)

		duration := time.Since(start)
		log.Printf("[%s] %s %s - %d (%v)", req.Method, req.URL.Path, req.RemoteAddr, rr.statusCode, duration)
	})
}

// ============================================================================
// Authentication Middleware
// ============================================================================

func (r *Router) requireBearerToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		authHeader := req.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		const prefix = "Bearer "
		if len(authHeader) < len(prefix) || authHeader[:len(prefix)] != prefix {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		token := authHeader[len(prefix):]
		agents, err := r.agentStore.List()
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		valid := false
		for _, agent := range agents {
			if agent.BearerToken == token {
				valid = true
				break
			}
		}

		if !valid {
			http.Error(w, "Invalid bearer token", http.StatusUnauthorized)
			return
		}

		next(w, req)
	}
}
