package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/zeroclaw/bot-portal/internal/models"
)

// ModelStore handles database operations for models
type ModelStore struct {
	db *sql.DB
}

// NewModelStore creates a new ModelStore instance
func NewModelStore(db *sql.DB) *ModelStore {
	return &ModelStore{db: db}
}

// Create inserts a new model into the database
func (s *ModelStore) Create(model *models.Model) error {
	query := `
		INSERT INTO models (id, name, provider, model_identifier, endpoint_url, default_params, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.Exec(query,
		model.ID,
		model.Name,
		model.Provider,
		model.ModelIdentifier,
		model.EndpointURL,
		model.DefaultParams,
		model.CreatedAt,
		model.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create model: %w", err)
	}
	return nil
}

// GetByID retrieves a model by its ID
func (s *ModelStore) GetByID(id string) (*models.Model, error) {
	query := `
		SELECT id, name, provider, model_identifier, endpoint_url, default_params, created_at, updated_at
		FROM models
		WHERE id = ?
	`
	row := s.db.QueryRow(query, id)

	var model models.Model
	var endpointURL, defaultParams sql.NullString
	err := row.Scan(
		&model.ID,
		&model.Name,
		&model.Provider,
		&model.ModelIdentifier,
		&endpointURL,
		&defaultParams,
		&model.CreatedAt,
		&model.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Return nil for not found, not an error
		}
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	// Handle nullable fields
	if endpointURL.Valid {
		model.EndpointURL = endpointURL.String
	}
	if defaultParams.Valid {
		model.DefaultParams = defaultParams.String
	}

	return &model, nil
}

// List retrieves all models ordered by created_at DESC
func (s *ModelStore) List() ([]models.Model, error) {
	query := `
		SELECT id, name, provider, model_identifier, endpoint_url, default_params, created_at, updated_at
		FROM models
		ORDER BY created_at DESC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer rows.Close()

	var modelsSlice []models.Model
	for rows.Next() {
		var model models.Model
		var endpointURL, defaultParams sql.NullString

		err := rows.Scan(
			&model.ID,
			&model.Name,
			&model.Provider,
			&model.ModelIdentifier,
			&endpointURL,
			&defaultParams,
			&model.CreatedAt,
			&model.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan model: %w", err)
		}

		// Handle nullable fields
		if endpointURL.Valid {
			model.EndpointURL = endpointURL.String
		}
		if defaultParams.Valid {
			model.DefaultParams = defaultParams.String
		}

		modelsSlice = append(modelsSlice, model)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate models: %w", err)
	}

	return modelsSlice, nil
}

// Update updates an existing model in the database
func (s *ModelStore) Update(model *models.Model) error {
	// Set updated timestamp
	model.UpdatedAt = time.Now()

	query := `
		UPDATE models
		SET name = ?, provider = ?, model_identifier = ?, endpoint_url = ?, default_params = ?, updated_at = ?
		WHERE id = ?
	`
	result, err := s.db.Exec(query,
		model.Name,
		model.Provider,
		model.ModelIdentifier,
		model.EndpointURL,
		model.DefaultParams,
		model.UpdatedAt,
		model.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update model: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// Delete removes a model from the database
func (s *ModelStore) Delete(id string) error {
	query := `DELETE FROM models WHERE id = ?`
	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete model: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteByProvider removes all models for a given provider
func (s *ModelStore) DeleteByProvider(provider string) error {
	query := `DELETE FROM models WHERE provider = ?`
	_, err := s.db.Exec(query, provider)
	if err != nil {
		return fmt.Errorf("failed to delete models by provider: %w", err)
	}
	return nil
}