package api

import (
	"encoding/json"
	"net/http"
	"sync"
)

// In-memory storage for CopilotKit settings
var (
	copilotKitSettings = struct {
		sync.RWMutex
		DefaultModel string
	}{}
)

// CopilotKitSettingsRequest represents settings update request
type CopilotKitSettingsRequest struct {
	DefaultModel string `json:"defaultModel"`
}

// CopilotKitSettingsResponse represents current settings
type CopilotKitSettingsResponse struct {
	DefaultModel string `json:"defaultModel"`
}

// handleCopilotKitSettings handles GET/PUT /api/copilotkit/settings
func (r *Router) handleCopilotKitSettings(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.getCopilotKitSettings(w, req)
	case http.MethodPut:
		r.updateCopilotKitSettings(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) getCopilotKitSettings(w http.ResponseWriter, req *http.Request) {
	copilotKitSettings.RLock()
	defaultModel := copilotKitSettings.DefaultModel
	copilotKitSettings.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CopilotKitSettingsResponse{
		DefaultModel: defaultModel,
	})
}

func (r *Router) updateCopilotKitSettings(w http.ResponseWriter, req *http.Request) {
	var settings CopilotKitSettingsRequest
	if err := json.NewDecoder(req.Body).Decode(&settings); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	copilotKitSettings.Lock()
	copilotKitSettings.DefaultModel = settings.DefaultModel
	copilotKitSettings.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CopilotKitSettingsResponse{
		DefaultModel: settings.DefaultModel,
	})
}
