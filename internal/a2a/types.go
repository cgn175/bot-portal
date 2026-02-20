package a2a

import (
	"encoding/json"
	"time"
)

// ============================================================================
// Google A2A Protocol Types
// ============================================================================

// AgentCard represents an agent's discovery document
// Corresponds to: GET /.well-known/agent.json
type AgentCard struct {
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Version        string            `json:"version"`
	Capabilities   AgentCapabilities `json:"capabilities"`
	Authentication Authentication    `json:"authentication"`
	Endpoints      AgentEndpoints    `json:"endpoints"`
	Skills         []Skill           `json:"skills"`
}

// AgentCapabilities represents what an agent can do
type AgentCapabilities struct {
	Streaming         bool `json:"streaming"`
	Artifacts         bool `json:"artifacts"`
	PushNotifications bool `json:"pushNotifications"`
}

// Authentication represents authentication requirements
type Authentication struct {
	Schemes []string `json:"schemes"`
}

// AgentEndpoints represents the HTTP endpoints for A2A
type AgentEndpoints struct {
	Tasks  string `json:"tasks"`
	Stream string `json:"stream"`
}

// Skill represents an agent skill
type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ============================================================================
// Task Types
// ============================================================================

// Task represents the core unit of work in A2A
type Task struct {
	ID        string        `json:"id"`
	Status    TaskStatus    `json:"status"`
	Messages  []TaskMessage `json:"messages"`
	Artifacts []Artifact    `json:"artifacts"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// TaskMessage represents a message in a task conversation
type TaskMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Artifact represents a file, image, or data produced by a task
type Artifact struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

// ============================================================================
// Task Update (SSE Event)
// ============================================================================

// TaskUpdate represents a Server-Sent Event for task updates
type TaskUpdate struct {
	TaskID   string       `json:"task_id"`
	Status   TaskStatus   `json:"status"`
	Message  *TaskMessage `json:"message,omitempty"`
	Artifact *Artifact    `json:"artifact,omitempty"`
}

// ============================================================================
// Request/Response Types
// ============================================================================

// CreateTaskRequest represents a request to create a new task
type CreateTaskRequest struct {
	Message  TaskMessage     `json:"message"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// CreateTaskResponse represents the response from creating a task
type CreateTaskResponse struct {
	TaskID string `json:"task_id"`
}

// GetTaskResponse represents the response for getting task status
type GetTaskResponse struct {
	Task *Task `json:"task"`
}

// CancelTaskRequest represents a request to cancel a task
type CancelTaskRequest struct{}

// CancelTaskResponse represents the response from cancelling a task
type CancelTaskResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

// ============================================================================
// A2A Peer Configuration
// ============================================================================

// A2APeer represents a peer agent in the A2A network
type A2APeer struct {
	ID          string `json:"id"`
	Endpoint    string `json:"endpoint"`
	BearerToken string `json:"bearer_token"`
	Enabled     bool   `json:"enabled"`
}

// A2AConfig represents the A2A configuration for an agent
type A2AConfig struct {
	Enabled        bool      `json:"enabled"`
	ListenPort     int       `json:"listen_port"`
	DiscoveryMode  string    `json:"discovery_mode"`
	AllowedPeerIDs []string  `json:"allowed_peer_ids"`
	AgentCard      AgentCard `json:"agent_card"`
	Peers          []A2APeer `json:"peers"`
	RateLimit      RateLimit `json:"rate_limit"`
}

// RateLimit represents rate limiting configuration
type RateLimit struct {
	RequestsPerMinute int `json:"requests_per_minute"`
	BurstSize         int `json:"burst_size"`
}

// ============================================================================
// Channel Types (Bot Portal specific)
// ============================================================================

// Channel represents a communication channel between agents
type Channel struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"` // "direct" or "general"
	Members []string `json:"members"`
}

// ChannelID generates a channel ID from sender and recipient
func ChannelID(senderID, recipientID string) string {
	if recipientID == "" {
		return "general"
	}
	// Sort IDs to ensure consistent channel ID regardless of direction
	if senderID < recipientID {
		return senderID + "::" + recipientID
	}
	return recipientID + "::" + senderID
}

// IsDirectChannel checks if a channel ID represents a direct message
func IsDirectChannel(channelID string) bool {
	return channelID != "general" && len(channelID) > 2
}
