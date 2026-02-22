package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zeroclaw/bot-portal/internal/crypto"
	"github.com/zeroclaw/bot-portal/internal/models"
)

// AuthConfigStore provides database operations for authentication configurations
type AuthConfigStore struct {
	db *sql.DB
}

// NewAuthConfigStore creates a new AuthConfigStore instance
func NewAuthConfigStore(db *sql.DB) *AuthConfigStore {
	return &AuthConfigStore{db: db}
}

// Create inserts a new authentication configuration with encrypted credentials
func (s *AuthConfigStore) Create(config *models.AuthConfig) error {
	// Parse credentials JSON to validate and prepare for encryption
	var credsMap map[string]string
	if err := json.Unmarshal([]byte(config.Credentials), &credsMap); err != nil {
		return fmt.Errorf("invalid credentials JSON format: %w", err)
	}

	// Encrypt the credentials
	encryptedCreds, err := crypto.EncryptCredentials(credsMap)
	if err != nil {
		return fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	query := `INSERT INTO auth_configs (id, name, provider, auth_type, credentials, endpoint_url, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = s.db.Exec(query, config.ID, config.Name, config.Provider, config.AuthType,
		encryptedCreds, config.EndpointURL, config.CreatedAt, config.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create auth config: %w", err)
	}

	return nil
}

// GetByID retrieves an authentication configuration by ID with decrypted credentials
func (s *AuthConfigStore) GetByID(id string) (*models.AuthConfig, error) {
	query := `SELECT id, name, provider, auth_type, credentials, endpoint_url, created_at, updated_at
			  FROM auth_configs WHERE id = ?`

	var config models.AuthConfig
	var encryptedCreds string

	err := s.db.QueryRow(query, id).Scan(
		&config.ID, &config.Name, &config.Provider, &config.AuthType,
		&encryptedCreds, &config.EndpointURL, &config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("auth config not found with id: %s", id)
		}
		return nil, fmt.Errorf("failed to get auth config: %w", err)
	}

	// Decrypt the credentials
	credsMap, err := crypto.DecryptCredentials(encryptedCreds)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	// Convert back to JSON string
	credsJSON, err := json.Marshal(credsMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credentials: %w", err)
	}

	config.Credentials = string(credsJSON)

	return &config, nil
}

// List retrieves all authentication configurations with decrypted credentials
func (s *AuthConfigStore) List() ([]*models.AuthConfig, error) {
	query := `SELECT id, name, provider, auth_type, credentials, endpoint_url, created_at, updated_at
			  FROM auth_configs ORDER BY created_at DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query auth configs: %w", err)
	}
	defer rows.Close()

	var configs []*models.AuthConfig

	for rows.Next() {
		var config models.AuthConfig
		var encryptedCreds string

		err := rows.Scan(
			&config.ID, &config.Name, &config.Provider, &config.AuthType,
			&encryptedCreds, &config.EndpointURL, &config.CreatedAt, &config.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan auth config: %w", err)
		}

		// Decrypt the credentials
		credsMap, err := crypto.DecryptCredentials(encryptedCreds)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
		}

		// Convert back to JSON string
		credsJSON, err := json.Marshal(credsMap)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal credentials: %w", err)
		}

		config.Credentials = string(credsJSON)
		configs = append(configs, &config)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating auth configs: %w", err)
	}

	return configs, nil
}

// ListMasked retrieves all authentication configurations with credentials masked for API responses
func (s *AuthConfigStore) ListMasked() ([]*models.AuthConfig, error) {
	query := `SELECT id, name, provider, auth_type, credentials, endpoint_url, created_at, updated_at
			  FROM auth_configs ORDER BY created_at DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query auth configs: %w", err)
	}
	defer rows.Close()

	var configs []*models.AuthConfig

	for rows.Next() {
		var config models.AuthConfig
		var encryptedCreds string

		err := rows.Scan(
			&config.ID, &config.Name, &config.Provider, &config.AuthType,
			&encryptedCreds, &config.EndpointURL, &config.CreatedAt, &config.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan auth config: %w", err)
		}

		// Decrypt the credentials to get the structure
		credsMap, err := crypto.DecryptCredentials(encryptedCreds)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
		}

		// Mask the credential values
		maskedCreds := make(map[string]string)
		for key := range credsMap {
			maskedCreds[key] = "****"
		}

		// Convert back to JSON string
		credsJSON, err := json.Marshal(maskedCreds)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal masked credentials: %w", err)
		}

		config.Credentials = string(credsJSON)
		configs = append(configs, &config)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating auth configs: %w", err)
	}

	return configs, nil
}

// Update updates an existing authentication configuration with encrypted credentials
func (s *AuthConfigStore) Update(config *models.AuthConfig) error {
	// Parse credentials JSON to validate and prepare for encryption
	var credsMap map[string]string
	if err := json.Unmarshal([]byte(config.Credentials), &credsMap); err != nil {
		return fmt.Errorf("invalid credentials JSON format: %w", err)
	}

	// Encrypt the credentials
	encryptedCreds, err := crypto.EncryptCredentials(credsMap)
	if err != nil {
		return fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	// Update the updated_at timestamp
	config.UpdatedAt = time.Now()

	query := `UPDATE auth_configs SET name = ?, provider = ?, auth_type = ?, credentials = ?,
			  endpoint_url = ?, updated_at = ? WHERE id = ?`

	result, err := s.db.Exec(query, config.Name, config.Provider, config.AuthType,
		encryptedCreds, config.EndpointURL, config.UpdatedAt, config.ID)
	if err != nil {
		return fmt.Errorf("failed to update auth config: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("auth config not found with id: %s", config.ID)
	}

	return nil
}

// Delete removes an authentication configuration by ID
func (s *AuthConfigStore) Delete(id string) error {
	query := `DELETE FROM auth_configs WHERE id = ?`

	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete auth config: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("auth config not found with id: %s", id)
	}

	return nil
}