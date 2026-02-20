package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zeroclaw/bot-portal/internal/a2a"
)

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

// MessageStore handles task log persistence
type MessageStore struct {
	db *sql.DB
}

// NewMessageStore creates a new message store
func NewMessageStore(db *sql.DB) *MessageStore {
	return &MessageStore{db: db}
}

// Create creates a new task log
func (s *MessageStore) Create(log *TaskLog) error {
	messagesJSON, _ := json.Marshal(log.Messages)
	artifactsJSON, _ := json.Marshal(log.Artifacts)

	_, err := s.db.Exec(`
		INSERT INTO task_logs (id, channel_id, sender_id, recipient_id, status, messages, artifacts, direction, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.ID, log.ChannelID, log.SenderID, log.RecipientID, log.Status,
		messagesJSON, artifactsJSON, log.Direction, log.CreatedAt, log.UpdatedAt)
	return err
}

// GetByID retrieves a task log by ID
func (s *MessageStore) GetByID(id string) (*TaskLog, error) {
	var log TaskLog
	var messagesJSON, artifactsJSON []byte

	err := s.db.QueryRow(`
		SELECT id, channel_id, sender_id, recipient_id, status, messages, artifacts, direction, created_at, updated_at
		FROM task_logs WHERE id = ?`, id).Scan(
		&log.ID, &log.ChannelID, &log.SenderID, &log.RecipientID, &log.Status,
		&messagesJSON, &artifactsJSON, &log.Direction, &log.CreatedAt, &log.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	log.Messages = messagesJSON
	log.Artifacts = artifactsJSON

	return &log, nil
}

// ListByChannel retrieves task logs for a channel
func (s *MessageStore) ListByChannel(channelID string, limit int) ([]*TaskLog, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(`
		SELECT id, channel_id, sender_id, recipient_id, status, messages, artifacts, direction, created_at, updated_at
		FROM task_logs WHERE channel_id = ?
		ORDER BY created_at DESC LIMIT ?`, channelID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*TaskLog
	for rows.Next() {
		var log TaskLog
		var messagesJSON, artifactsJSON []byte

		err := rows.Scan(
			&log.ID, &log.ChannelID, &log.SenderID, &log.RecipientID, &log.Status,
			&messagesJSON, &artifactsJSON, &log.Direction, &log.CreatedAt, &log.UpdatedAt)
		if err != nil {
			return nil, err
		}

		log.Messages = messagesJSON
		log.Artifacts = artifactsJSON

		logs = append(logs, &log)
	}

	return logs, nil
}

// ListAll retrieves all task logs with limit
func (s *MessageStore) ListAll(limit int) ([]*TaskLog, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(`
		SELECT id, channel_id, sender_id, recipient_id, status, messages, artifacts, direction, created_at, updated_at
		FROM task_logs
		ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*TaskLog
	for rows.Next() {
		var log TaskLog
		var messagesJSON, artifactsJSON []byte

		err := rows.Scan(
			&log.ID, &log.ChannelID, &log.SenderID, &log.RecipientID, &log.Status,
			&messagesJSON, &artifactsJSON, &log.Direction, &log.CreatedAt, &log.UpdatedAt)
		if err != nil {
			return nil, err
		}

		log.Messages = messagesJSON
		log.Artifacts = artifactsJSON

		logs = append(logs, &log)
	}

	return logs, nil
}

// Update updates a task log
func (s *MessageStore) Update(log *TaskLog) error {
	log.UpdatedAt = time.Now()
	messagesJSON, _ := json.Marshal(log.Messages)
	artifactsJSON, _ := json.Marshal(log.Artifacts)

	_, err := s.db.Exec(`
		UPDATE task_logs SET channel_id = ?, sender_id = ?, recipient_id = ?, status = ?, messages = ?, artifacts = ?, direction = ?, updated_at = ?
		WHERE id = ?`,
		log.ChannelID, log.SenderID, log.RecipientID, log.Status,
		messagesJSON, artifactsJSON, log.Direction, log.UpdatedAt, log.ID)
	return err
}

// Delete deletes a task log
func (s *MessageStore) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM task_logs WHERE id = ?", id)
	return err
}

// AppendMessage appends a message to a task log
func (s *MessageStore) AppendMessage(id string, message a2a.TaskMessage) error {
	log, err := s.GetByID(id)
	if err != nil || log == nil {
		return err
	}

	// Unmarshal existing messages
	var messages []a2a.TaskMessage
	if len(log.Messages) > 0 {
		if err := json.Unmarshal(log.Messages, &messages); err != nil {
			return fmt.Errorf("failed to unmarshal messages: %w", err)
		}
	}

	// Append the new message
	messages = append(messages, message)
	messagesJSON, _ := json.Marshal(messages)

	_, err = s.db.Exec(`
		UPDATE task_logs SET messages = ?, updated_at = ?
		WHERE id = ?`, messagesJSON, time.Now(), id)
	return err
}
