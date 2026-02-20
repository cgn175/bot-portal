package test

import (
	"testing"
	"time"

	"github.com/zeroclaw/bot-portal/internal/models"
	"github.com/zeroclaw/bot-portal/internal/store"
)

func TestAgentRegistration(t *testing.T) {
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	agentStore := store.NewAgentStore(db)

	agent := &store.Agent{
		ID:          "test-agent",
		Name:        "Test Agent",
		Image:       "test:latest",
		Endpoint:    "http://localhost:8080",
		Status:      "stopped",
		BearerToken: "test-token",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = agentStore.Create(agent)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	retrieved, err := agentStore.GetByID("test-agent")
	if err != nil {
		t.Fatalf("Failed to get agent: %v", err)
	}

	if retrieved.ID != "test-agent" {
		t.Errorf("Expected ID test-agent, got %s", retrieved.ID)
	}
}

func TestChannelCreation(t *testing.T) {
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	channelStore := store.NewChannelStore(db)

	channel := &models.Channel{
		ID:        "agent1::agent2",
		Members:   []string{"agent1", "agent2"},
		CreatedAt: time.Now(),
	}

	err = channelStore.Create(channel)
	if err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	retrieved, err := channelStore.GetByID("agent1::agent2")
	if err != nil {
		t.Fatalf("Failed to get channel: %v", err)
	}

	if len(retrieved.Members) != 2 {
		t.Errorf("Expected 2 members, got %d", len(retrieved.Members))
	}
}

func TestMessageLogging(t *testing.T) {
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	messageStore := store.NewMessageStore(db)

	taskLog := &store.TaskLog{
		ID:          "task-123",
		ChannelID:   "agent1::agent2",
		SenderID:    "agent1",
		RecipientID: "agent2",
		Status:      "pending",
		Direction:   "outbound",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = messageStore.Create(taskLog)
	if err != nil {
		t.Fatalf("Failed to create task log: %v", err)
	}

	log, err := messageStore.GetByID("task-123")
	if err != nil {
		t.Fatalf("Failed to get task log: %v", err)
	}

	if log.SenderID != "agent1" {
		t.Errorf("Expected sender agent1, got %s", log.SenderID)
	}
}

func TestA2AMessageRouting(t *testing.T) {
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	agentStore := store.NewAgentStore(db)

	// Create mock agents
	agent1 := &store.Agent{
		ID:          "agent1",
		Name:        "Agent 1",
		Endpoint:    "http://localhost:8081",
		BearerToken: "token1",
		Status:      "running",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	agent2 := &store.Agent{
		ID:          "agent2",
		Name:        "Agent 2",
		Endpoint:    "http://localhost:8082",
		BearerToken: "token2",
		Status:      "running",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	agentStore.Create(agent1)
	agentStore.Create(agent2)

	agents, err := agentStore.List()
	if err != nil {
		t.Fatalf("Failed to list agents: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("Expected 2 agents, got %d", len(agents))
	}
}

func TestBroadcastChannel(t *testing.T) {
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	channelStore := store.NewChannelStore(db)

	// Create general broadcast channel
	err = channelStore.EnsureGeneralChannel()
	if err != nil {
		t.Fatalf("Failed to create general channel: %v", err)
	}

	channel, err := channelStore.GetByID("general")
	if err != nil {
		t.Fatalf("Failed to get general channel: %v", err)
	}

	if channel.ID != "general" {
		t.Errorf("Expected channel ID general, got %s", channel.ID)
	}
}

