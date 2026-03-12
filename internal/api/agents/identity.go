package agents

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/zeroclaw/bot-portal/internal/models"
	"github.com/zeroclaw/bot-portal/internal/store"
)

// HandleAgentIdentityFiles dispatches GET /api/agents/{id}/identity-files.
func (h *Handler) HandleAgentIdentityFiles(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	prefix := "/api/agents/"
	suffix := "/identity-files"

	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	agentID := path[len(prefix) : len(path)-len(suffix)]
	if agentID == "" {
		http.Error(w, "Agent ID required", http.StatusBadRequest)
		return
	}

	agent, err := h.AgentStore.GetByID(agentID)
	if err != nil {
		log.Printf("[error] HandleAgentIdentityFiles getAgent %s: %v", agentID, err)
		http.Error(w, "Failed to get agent", http.StatusInternalServerError)
		return
	}
	if agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	switch req.Method {
	case http.MethodGet:
		h.listIdentityFiles(w, req, agentID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleAgentIdentityFileDetail dispatches GET/PUT /api/agents/{id}/identity-files/{filename}.
func (h *Handler) HandleAgentIdentityFileDetail(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	prefix := "/api/agents/"

	if !strings.HasPrefix(path, prefix) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	remaining := path[len(prefix):]
	parts := strings.SplitN(remaining, "/identity-files/", 2)
	if len(parts) != 2 {
		http.Error(w, "Invalid path format", http.StatusBadRequest)
		return
	}

	agentID := parts[0]
	filename := parts[1]

	log.Printf("Identity file request: agentID=%q filename=%q method=%s", agentID, filename, req.Method)

	if agentID == "" {
		http.Error(w, "Agent ID required", http.StatusBadRequest)
		return
	}
	if filename == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	agent, err := h.AgentStore.GetByID(agentID)
	if err != nil {
		log.Printf("Identity file: error looking up agent %q: %v", agentID, err)
		http.Error(w, "Failed to get agent", http.StatusInternalServerError)
		return
	}
	if agent == nil {
		log.Printf("Identity file: agent %q not found", agentID)
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	switch req.Method {
	case http.MethodGet:
		h.getIdentityFile(w, req, agentID, filename)
	case http.MethodPut:
		h.updateIdentityFile(w, req, agentID, filename)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) listIdentityFiles(w http.ResponseWriter, req *http.Request, agentID string) {
	identityFileStore := store.NewIdentityFileStore(h.DB)
	files, err := identityFileStore.ListByAgent(agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := AgentIdentityFilesResponse{
		Files: make([]AgentIdentityFileInfo, len(files)),
	}
	for i, f := range files {
		response.Files[i] = AgentIdentityFileInfo{
			Filename:  f.Filename,
			CharCount: f.CharCount,
			UpdatedAt: f.UpdatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) getIdentityFile(w http.ResponseWriter, req *http.Request, agentID, filename string) {
	agent, err := h.AgentStore.GetByID(agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var content []byte
	if agent.Status == "running" && agent.ContainerID != "" {
		ctx := req.Context()
		content, err = h.DockerMgr.ReadWorkspaceFile(ctx, agentID, filename)
		if err != nil {
			if !strings.Contains(err.Error(), "not found") {
				log.Printf("Failed to read from container, falling back to DB: %v", err)
			}
		}
	}

	if content == nil {
		identityFileStore := store.NewIdentityFileStore(h.DB)
		file, err := identityFileStore.GetByAgentAndFilename(agentID, filename)
		if err != nil {
			log.Printf("[error] getIdentityFile from store %s/%s: %v", agentID, filename, err)
			http.Error(w, "Failed to get identity file", http.StatusInternalServerError)
			return
		}
		if file != nil {
			content = []byte(file.Content)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"filename": filename,
		"content":  string(content),
	})
}

func (h *Handler) updateIdentityFile(w http.ResponseWriter, req *http.Request, agentID, filename string) {
	var request UpdateIdentityFileRequest
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	const maxSize = 16 * 1024
	if len(request.Content) > maxSize {
		http.Error(w, fmt.Sprintf("Content exceeds maximum size of %d bytes", maxSize), http.StatusBadRequest)
		return
	}

	identityFileStore := store.NewIdentityFileStore(h.DB)
	file := &models.AgentIdentityFile{
		AgentID:  agentID,
		Filename: filename,
		Content:  request.Content,
	}

	if err := identityFileStore.CreateOrUpdate(file); err != nil {
		log.Printf("[error] updateIdentityFile store %s/%s: %v", agentID, filename, err)
		http.Error(w, "Failed to update identity file", http.StatusInternalServerError)
		return
	}

	agent, err := h.AgentStore.GetByID(agentID)
	if err != nil {
		log.Printf("[error] updateIdentityFile getAgent %s: %v", agentID, err)
		http.Error(w, "Failed to get agent", http.StatusInternalServerError)
		return
	}

	if agent.Status == "running" && agent.ContainerID != "" {
		ctx := req.Context()
		if err := h.DockerMgr.WriteWorkspaceFile(ctx, agentID, filename, []byte(request.Content)); err != nil {
			log.Printf("Warning: Failed to sync %s to container: %v", filename, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"filename":  filename,
		"charCount": utf8.RuneCountInString(request.Content),
	})
}
