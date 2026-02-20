package store

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/zeroclaw/bot-portal/internal/models"
)

// ChannelStore handles channel persistence
type ChannelStore struct {
	db *sql.DB
}

// NewChannelStore creates a new channel store
func NewChannelStore(db *sql.DB) *ChannelStore {
	return &ChannelStore{db: db}
}

// Create creates a new channel
func (s *ChannelStore) Create(channel *models.Channel) error {
	membersJSON, _ := json.Marshal(channel.Members)

	_, err := s.db.Exec(`
		INSERT INTO channels (id, members, created_at)
		VALUES (?, ?, ?)`,
		channel.ID, membersJSON, channel.CreatedAt)
	return err
}

// GetByID retrieves a channel by ID
func (s *ChannelStore) GetByID(id string) (*models.Channel, error) {
	var channel models.Channel
	var membersJSON []byte

	err := s.db.QueryRow(`
		SELECT id, members, created_at
		FROM channels WHERE id = ?`, id).Scan(
		&channel.ID, &membersJSON, &channel.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if len(membersJSON) > 0 {
		json.Unmarshal(membersJSON, &channel.Members)
	}

	return &channel, nil
}

// List retrieves all channels
func (s *ChannelStore) List() ([]*models.Channel, error) {
	rows, err := s.db.Query(`
		SELECT id, members, created_at
		FROM channels`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []*models.Channel
	for rows.Next() {
		var channel models.Channel
		var membersJSON []byte

		err := rows.Scan(&channel.ID, &membersJSON, &channel.CreatedAt)
		if err != nil {
			return nil, err
		}

		if len(membersJSON) > 0 {
			json.Unmarshal(membersJSON, &channel.Members)
		}

		channels = append(channels, &channel)
	}

	return channels, nil
}

// Delete deletes a channel
func (s *ChannelStore) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM channels WHERE id = ?", id)
	return err
}

// EnsureGeneralChannel ensures the general broadcast channel exists
func (s *ChannelStore) EnsureGeneralChannel() error {
	channel := &models.Channel{
		ID:        "general",
		Members:   []string{},
		CreatedAt: time.Now(),
	}
	return s.Create(channel)
}
