package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/zeroclaw/bot-portal/internal/models"
	"github.com/zeroclaw/bot-portal/internal/store"
)

// ============================================================================
// Models Handlers
// ============================================================================

func (r *Router) handleModels(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.listModels(w, req)
	case http.MethodPost:
		r.createModel(w, req)
	case http.MethodDelete:
		r.deleteAllModels(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleModelDetail(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if path == "/api/models/" {
		http.Error(w, "Model ID required", http.StatusBadRequest)
		return
	}

	modelID := path[len("/api/models/"):]

	switch req.Method {
	case http.MethodGet:
		r.getModel(w, req, modelID)
	case http.MethodPut:
		r.updateModel(w, req, modelID)
	case http.MethodDelete:
		r.deleteModel(w, req, modelID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) listModels(w http.ResponseWriter, req *http.Request) {
	models, err := r.modelStore.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}

func (r *Router) createModel(w http.ResponseWriter, req *http.Request) {
	var requestModel struct {
		ID           string                 `json:"id"`
		Name         string                 `json:"name"`
		AuthConfigID string                 `json:"authConfigId"`
		ModelName    string                 `json:"modelName"`
		APIKeyConfig map[string]interface{} `json:"apiKeyConfig"`
		BaseURL      string                 `json:"baseUrl"`
	}

	if err := json.NewDecoder(req.Body).Decode(&requestModel); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	now := time.Now()
	model := &models.Model{
		ID:              requestModel.ID,
		Name:            requestModel.Name,
		AuthConfigID:    requestModel.AuthConfigID,
		ModelIdentifier: requestModel.ModelName,
		EndpointURL:     requestModel.BaseURL,
		DefaultParams:   "", // Convert APIKeyConfig appropriately
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// Handle API key configuration as default params for now
	if requestModel.APIKeyConfig != nil {
		configStr, err := json.Marshal(requestModel.APIKeyConfig)
		if err != nil {
			http.Error(w, "Invalid API key config", http.StatusBadRequest)
			return
		}
		model.DefaultParams = string(configStr)
	}

	if err := r.modelStore.Create(model); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(model)
}

func (r *Router) getModel(w http.ResponseWriter, req *http.Request, modelID string) {
	model, err := r.modelStore.GetByID(modelID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if model == nil {
		http.Error(w, "Model not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model)
}

func (r *Router) updateModel(w http.ResponseWriter, req *http.Request, modelID string) {
	model, err := r.modelStore.GetByID(modelID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if model == nil {
		http.Error(w, "Model not found", http.StatusNotFound)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&updates); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if name, ok := updates["name"].(string); ok {
		model.Name = name
	}
	if authConfigID, ok := updates["authConfigId"].(string); ok {
		model.AuthConfigID = authConfigID
	}
	if modelName, ok := updates["modelName"].(string); ok {
		model.ModelIdentifier = modelName
	}
	if baseURL, ok := updates["baseUrl"].(string); ok {
		model.EndpointURL = baseURL
	}
	if isDefault, ok := updates["isDefault"].(bool); ok {
		model.IsDefault = isDefault
	}
	if apiKeyConfig, ok := updates["apiKeyConfig"].(map[string]interface{}); ok {
		configBytes, err := json.Marshal(apiKeyConfig)
		if err != nil {
			http.Error(w, "Invalid API key config", http.StatusBadRequest)
			return
		}
		model.DefaultParams = string(configBytes)
	}

	model.UpdatedAt = time.Now()

	if err := r.modelStore.Update(model); err != nil {
		if err == store.ErrNotFound {
			http.Error(w, "Model not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if isDefault, ok := updates["isDefault"].(bool); ok {
		if isDefault == true {
			if err := r.modelStore.SetDefault(modelID); err != nil {
				if err == store.ErrNotFound {
					http.Error(w, "Model not found", http.StatusNotFound)
					return
				}
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model)
}

func (r *Router) deleteModel(w http.ResponseWriter, req *http.Request, modelID string) {
	if err := r.modelStore.Delete(modelID); err != nil {
		if err == store.ErrNotFound {
			http.Error(w, "Model not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (r *Router) deleteAllModels(w http.ResponseWriter, req *http.Request) {
	if err := r.modelStore.DeleteAll(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
