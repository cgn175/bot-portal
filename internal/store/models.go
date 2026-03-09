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
		INSERT INTO models (id, name, auth_config_id, model_identifier, endpoint_url, default_params, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.Exec(query,
		model.ID,
		model.Name,
		model.AuthConfigID,
		model.ModelIdentifier,
		model.EndpointURL,
		model.DefaultParams,
		model.IsDefault,
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
		SELECT id, name, auth_config_id, model_identifier, endpoint_url, default_params, is_default, created_at, updated_at
		FROM models
		WHERE id = ?
	`
	row := s.db.QueryRow(query, id)

	var model models.Model
	var authConfigID sql.NullString
	var endpointURL, defaultParams sql.NullString
	var isDefault sql.NullInt64
	err := row.Scan(
		&model.ID,
		&model.Name,
		&authConfigID,
		&model.ModelIdentifier,
		&endpointURL,
		&defaultParams,
		&isDefault,
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
	if authConfigID.Valid {
		model.AuthConfigID = authConfigID.String
	}
	if endpointURL.Valid {
		model.EndpointURL = endpointURL.String
	}
	if defaultParams.Valid {
		model.DefaultParams = defaultParams.String
	}
	if isDefault.Valid {
		model.IsDefault = isDefault.Int64 == 1
	}

	return &model, nil
}

// List retrieves all models ordered by created_at DESC
func (s *ModelStore) List() ([]models.Model, error) {
	query := `
		SELECT id, name, auth_config_id, model_identifier, endpoint_url, default_params, is_default, created_at, updated_at
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
		var authConfigID sql.NullString
		var endpointURL, defaultParams sql.NullString
		var isDefault sql.NullInt64

		err := rows.Scan(
			&model.ID,
			&model.Name,
			&authConfigID,
			&model.ModelIdentifier,
			&endpointURL,
			&defaultParams,
			&isDefault,
			&model.CreatedAt,
			&model.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan model: %w", err)
		}

		// Handle nullable fields
		if authConfigID.Valid {
			model.AuthConfigID = authConfigID.String
		}
		if endpointURL.Valid {
			model.EndpointURL = endpointURL.String
		}
		if defaultParams.Valid {
			model.DefaultParams = defaultParams.String
		}
		if isDefault.Valid {
			model.IsDefault = isDefault.Int64 == 1
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
		SET name = ?, auth_config_id = ?, model_identifier = ?, endpoint_url = ?, default_params = ?, is_default = ?, updated_at = ?
		WHERE id = ?
	`
	result, err := s.db.Exec(query,
		model.Name,
		model.AuthConfigID,
		model.ModelIdentifier,
		model.EndpointURL,
		model.DefaultParams,
		model.IsDefault,
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

// DeleteByAuthConfigID removes all models for a given auth config ID
func (s *ModelStore) DeleteByAuthConfigID(authConfigID string) error {
	query := `DELETE FROM models WHERE auth_config_id = ?`
	_, err := s.db.Exec(query, authConfigID)
	if err != nil {
		return fmt.Errorf("failed to delete models by auth config ID: %w", err)
	}
	return nil
}

// DeleteAll removes all models from the database
func (s *ModelStore) DeleteAll() error {
	query := `DELETE FROM models`
	_, err := s.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to delete all models: %w", err)
	}
	return nil
}

// GetDefault returns the model marked as default, or the first model if none is marked.
// Returns nil if no models exist.
func (s *ModelStore) GetDefault() (*models.Model, error) {
	// First try to find a model flagged as default
	query := `
		SELECT id, name, auth_config_id, model_identifier, endpoint_url, default_params, is_default, created_at, updated_at
		FROM models
		WHERE is_default = 1
		LIMIT 1
	`
	row := s.db.QueryRow(query)
	model, err := s.scanModel(row)
	if err == nil {
		return model, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get default model: %w", err)
	}

	// Fallback: return the first model ordered by created_at
	query = `
		SELECT id, name, auth_config_id, model_identifier, endpoint_url, default_params, is_default, created_at, updated_at
		FROM models
		ORDER BY created_at ASC
		LIMIT 1
	`
	row = s.db.QueryRow(query)
	model, err = s.scanModel(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get first model: %w", err)
	}
	return model, nil
}

// SetDefault marks a model as the default, clearing the flag on all other models.
func (s *ModelStore) SetDefault(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clear all defaults
	if _, err := tx.Exec(`UPDATE models SET is_default = 0 WHERE is_default = 1`); err != nil {
		return fmt.Errorf("failed to clear defaults: %w", err)
	}

	// Set the new default
	result, err := tx.Exec(`UPDATE models SET is_default = 1, updated_at = ? WHERE id = ?`, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to set default: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return tx.Commit()
}

// scanModel scans a single model row including the is_default field.
func (s *ModelStore) scanModel(row *sql.Row) (*models.Model, error) {
	var model models.Model
	var authConfigID sql.NullString
	var endpointURL, defaultParams sql.NullString
	var isDefault sql.NullInt64
	err := row.Scan(
		&model.ID,
		&model.Name,
		&authConfigID,
		&model.ModelIdentifier,
		&endpointURL,
		&defaultParams,
		&isDefault,
		&model.CreatedAt,
		&model.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if authConfigID.Valid {
		model.AuthConfigID = authConfigID.String
	}
	if endpointURL.Valid {
		model.EndpointURL = endpointURL.String
	}
	if defaultParams.Valid {
		model.DefaultParams = defaultParams.String
	}
	if isDefault.Valid {
		model.IsDefault = isDefault.Int64 == 1
	}
	return &model, nil
}
