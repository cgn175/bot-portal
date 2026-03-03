package store

import (
	"database/sql"
	"time"

	"github.com/zeroclaw/bot-portal/internal/models"
)

// IdentityFileStore handles agent identity file persistence
type IdentityFileStore struct {
	db *sql.DB
}

// NewIdentityFileStore creates a new identity file store
func NewIdentityFileStore(db *sql.DB) *IdentityFileStore {
	return &IdentityFileStore{db: db}
}

// GetByAgentAndFilename retrieves an identity file by agent ID and filename
func (s *IdentityFileStore) GetByAgentAndFilename(agentID, filename string) (*models.AgentIdentityFile, error) {
	var file models.AgentIdentityFile
	err := s.db.QueryRow(`
		SELECT id, agent_id, filename, content, char_count, updated_at
		FROM agent_identity_files WHERE agent_id = ? AND filename = ?`,
		agentID, filename).Scan(
		&file.ID, &file.AgentID, &file.Filename, &file.Content, &file.CharCount, &file.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &file, nil
}

// ListByAgent retrieves all identity files for an agent
func (s *IdentityFileStore) ListByAgent(agentID string) ([]*models.AgentIdentityFile, error) {
	rows, err := s.db.Query(`
		SELECT id, agent_id, filename, content, char_count, updated_at
		FROM agent_identity_files WHERE agent_id = ? ORDER BY filename`,
		agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*models.AgentIdentityFile
	for rows.Next() {
		var file models.AgentIdentityFile
		err := rows.Scan(&file.ID, &file.AgentID, &file.Filename, &file.Content, &file.CharCount, &file.UpdatedAt)
		if err != nil {
			return nil, err
		}
		files = append(files, &file)
	}

	return files, nil
}

// Create creates a new identity file record
func (s *IdentityFileStore) Create(file *models.AgentIdentityFile) error {
	file.UpdatedAt = time.Now()
	file.CharCount = len(file.Content)

	result, err := s.db.Exec(`
		INSERT INTO agent_identity_files (agent_id, filename, content, char_count, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		file.AgentID, file.Filename, file.Content, file.CharCount, file.UpdatedAt)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err == nil {
		file.ID = id
	}

	return nil
}

// Update updates an existing identity file
func (s *IdentityFileStore) Update(file *models.AgentIdentityFile) error {
	file.UpdatedAt = time.Now()
	file.CharCount = len(file.Content)

	_, err := s.db.Exec(`
		UPDATE agent_identity_files SET content = ?, char_count = ?, updated_at = ?
		WHERE agent_id = ? AND filename = ?`,
		file.Content, file.CharCount, file.UpdatedAt, file.AgentID, file.Filename)

	return err
}

// CreateOrUpdate creates or updates an identity file
func (s *IdentityFileStore) CreateOrUpdate(file *models.AgentIdentityFile) error {
	existing, err := s.GetByAgentAndFilename(file.AgentID, file.Filename)
	if err != nil {
		return err
	}

	if existing == nil {
		return s.Create(file)
	}

	file.ID = existing.ID
	return s.Update(file)
}

// Delete deletes an identity file
func (s *IdentityFileStore) Delete(agentID, filename string) error {
	_, err := s.db.Exec(`
		DELETE FROM agent_identity_files WHERE agent_id = ? AND filename = ?`,
		agentID, filename)
	return err
}

// DeleteAllForAgent deletes all identity files for an agent
func (s *IdentityFileStore) DeleteAllForAgent(agentID string) error {
	_, err := s.db.Exec(`DELETE FROM agent_identity_files WHERE agent_id = ?`, agentID)
	return err
}
