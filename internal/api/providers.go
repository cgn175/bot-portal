package api

import (
	"encoding/json"
	"net/http"

	"github.com/zeroclaw/bot-portal/internal/provider"
)

// ============================================================================
// Provider Handlers
// ============================================================================

func (r *Router) handleProviders(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	providers := provider.List()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providers)
}

// handleProviderDetail returns details for a specific provider
func (r *Router) handleProviderDetail(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := req.URL.Path
	if path == "/api/providers/" {
		http.Error(w, "Provider ID required", http.StatusBadRequest)
		return
	}

	providerID := path[len("/api/providers/"):]

	p, ok := provider.Get(providerID)
	if !ok {
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}
