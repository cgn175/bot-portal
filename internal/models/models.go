package models

import (
	"encoding/json"
	"time"
)

// Agent represents a registered AI agent
type Agent struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Image       string            `json:"image"`
	AgentType   string            `json:"agentType"` // "docker" or "native"
	Status      string            `json:"status"`
	ContainerID string            `json:"containerId"`
	Endpoint    string            `json:"endpoint"`
	ListenPort  int               `json:"listenPort"`
	BearerToken string            `json:"-"`
	AgentCard   *AgentCard        `json:"agentCard"`
	Config      map[string]string `json:"config"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// AgentCard represents Google A2A AgentCard
type AgentCard struct {
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Version        string             `json:"version"`
	Capabilities   AgentCapabilities  `json:"capabilities"`
	Authentication AuthenticationInfo `json:"authentication"`
	Endpoints      AgentEndpoints     `json:"endpoints"`
	Skills         []Skill            `json:"skills"`
}

// AgentCapabilities represents agent capabilities
type AgentCapabilities struct {
	Streaming         bool `json:"streaming"`
	Artifacts         bool `json:"artifacts"`
	PushNotifications bool `json:"pushNotifications"`
}

// AuthenticationInfo represents authentication scheme
type AuthenticationInfo struct {
	Schemes []string `json:"schemes"`
}

// AgentEndpoints represents agent endpoints
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

// Channel represents a communication channel
type Channel struct {
	ID        string    `json:"id"`
	Members   []string  `json:"members"`
	CreatedAt time.Time `json:"createdAt"`
}

// TaskLog represents a logged A2A task
type TaskLog struct {
	ID          string          `json:"id"`
	ChannelID   string          `json:"channelId"`
	SenderID    string          `json:"senderId"`
	RecipientID string          `json:"recipientId"`
	Status      string          `json:"status"`
	Messages    json.RawMessage `json:"messages"`
	Artifacts   json.RawMessage `json:"artifacts"`
	Direction   string          `json:"direction"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// Task represents an A2A Task
type Task struct {
	ID        string        `json:"id"`
	Status    string        `json:"status"`
	Messages  []TaskMessage `json:"messages"`
	Artifacts []Artifact    `json:"artifacts"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

// TaskMessage represents a message in a task
type TaskMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Artifact represents a task artifact
type Artifact struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
}
