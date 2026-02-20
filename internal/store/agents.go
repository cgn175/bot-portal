package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"time"
)

// Agent represents a registered AI agent
type Agent struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Image       string          `json:"image"`
	AgentType   string          `json:"agentType"`
	Status      string          `json:"status"`
	ContainerID string          `json:"containerId"`
	Endpoint    string          `json:"endpoint"`
	ListenPort  int             `json:"listenPort"`
	BearerToken string          `json:"-"`
	AgentCard   json.RawMessage `json:"agentCard"`
	Config      json.RawMessage `json:"config"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// AgentStore handles agent persistence
type AgentStore struct {
	db *sql.DB
}

// NewAgentStore creates a new agent store
func NewAgentStore(db *sql.DB) *AgentStore {
	return &AgentStore{db: db}
}

// Create creates a new agent
func (s *AgentStore) Create(agent *Agent) error {
	agentCardJSON, _ := json.Marshal(agent.AgentCard)
	configJSON, _ := json.Marshal(agent.Config)

	if agent.AgentType == "" {
		agent.AgentType = "docker"
	}

	_, err := s.db.Exec(`
		INSERT INTO agents (id, name, description, image, agent_type, status, container_id, endpoint, listen_port, bearer_token, agent_card, config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		agent.ID, agent.Name, agent.Description, agent.Image, agent.AgentType, agent.Status,
		agent.ContainerID, agent.Endpoint, agent.ListenPort, agent.BearerToken,
		agentCardJSON, configJSON, agent.CreatedAt, agent.UpdatedAt)
	return err
}

// GetByID retrieves an agent by ID
func (s *AgentStore) GetByID(id string) (*Agent, error) {
	var agent Agent
	var agentCardJSON, configJSON []byte

	err := s.db.QueryRow(`
		SELECT id, name, description, image, COALESCE(agent_type, 'docker'), status, container_id, endpoint, listen_port, bearer_token, agent_card, config, created_at, updated_at
		FROM agents WHERE id = ?`, id).Scan(
		&agent.ID, &agent.Name, &agent.Description, &agent.Image, &agent.AgentType, &agent.Status,
		&agent.ContainerID, &agent.Endpoint, &agent.ListenPort, &agent.BearerToken,
		&agentCardJSON, &configJSON, &agent.CreatedAt, &agent.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if len(agentCardJSON) > 0 {
		if err := json.Unmarshal(agentCardJSON, &agent.AgentCard); err != nil {
			log.Printf("Failed to unmarshal agent_card for %s: %v", agent.ID, err)
		}
	}
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &agent.Config); err != nil {
			log.Printf("Failed to unmarshal config for %s: %v", agent.ID, err)
		}
	}

	return &agent, nil
}

// List retrieves all agents
func (s *AgentStore) List() ([]*Agent, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, image, COALESCE(agent_type, 'docker'), status, container_id, endpoint, listen_port, bearer_token, agent_card, config, created_at, updated_at
		FROM agents`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		var agent Agent
		var agentCardJSON, configJSON []byte

		err := rows.Scan(
			&agent.ID, &agent.Name, &agent.Description, &agent.Image, &agent.AgentType, &agent.Status,
			&agent.ContainerID, &agent.Endpoint, &agent.ListenPort, &agent.BearerToken,
			&agentCardJSON, &configJSON, &agent.CreatedAt, &agent.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if len(agentCardJSON) > 0 {
			if err := json.Unmarshal(agentCardJSON, &agent.AgentCard); err != nil {
				log.Printf("Failed to unmarshal agent_card for %s: %v", agent.ID, err)
			}
		}
		if len(configJSON) > 0 {
			if err := json.Unmarshal(configJSON, &agent.Config); err != nil {
				log.Printf("Failed to unmarshal config for %s: %v", agent.ID, err)
			}
		}

		agents = append(agents, &agent)
	}

	return agents, nil
}

// Update updates an existing agent
func (s *AgentStore) Update(agent *Agent) error {
	agent.UpdatedAt = time.Now()
	agentCardJSON, _ := json.Marshal(agent.AgentCard)
	configJSON, _ := json.Marshal(agent.Config)

	_, err := s.db.Exec(`
		UPDATE agents SET name = ?, description = ?, image = ?, status = ?, container_id = ?, endpoint = ?, listen_port = ?, bearer_token = ?, agent_card = ?, config = ?, updated_at = ?
		WHERE id = ?`,
		agent.Name, agent.Description, agent.Image, agent.Status, agent.ContainerID,
		agent.Endpoint, agent.ListenPort, agent.BearerToken, agentCardJSON, configJSON,
		agent.UpdatedAt, agent.ID)
	return err
}

// Delete deletes an agent
func (s *AgentStore) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM agents WHERE id = ?", id)
	return err
}

// UpdateStatus updates agent status
func (s *AgentStore) UpdateStatus(id, status string) error {
	_, err := s.db.Exec("UPDATE agents SET status = ?, updated_at = ? WHERE id = ?", status, time.Now(), id)
	return err
}

// GenerateBearerToken generates a new bearer token for an agent
func (s *AgentStore) GenerateBearerToken(id string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	_, err := s.db.Exec("UPDATE agents SET bearer_token = ?, updated_at = ? WHERE id = ?", token, time.Now(), id)
	return token, err
}
