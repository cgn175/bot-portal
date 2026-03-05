package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/zeroclaw/bot-portal/internal/docker"
	"github.com/zeroclaw/bot-portal/internal/models"
	"github.com/zeroclaw/bot-portal/internal/store"
)

func (h *Handler) syncSymmetricPeers(agentID string, oldPeerIDs, newPeerIDs []string) {
	oldSet := make(map[string]bool, len(oldPeerIDs))
	for _, id := range oldPeerIDs {
		oldSet[id] = true
	}
	newSet := make(map[string]bool, len(newPeerIDs))
	for _, id := range newPeerIDs {
		newSet[id] = true
	}

	for _, peerID := range newPeerIDs {
		if oldSet[peerID] {
			continue
		}
		h.addPeerToAgent(peerID, agentID)
	}

	for _, peerID := range oldPeerIDs {
		if newSet[peerID] {
			continue
		}
		h.removePeerFromAgent(peerID, agentID)
	}
}

func (h *Handler) addPeerToAgent(peerAgentID, targetID string) {
	peer, err := h.AgentStore.GetByID(peerAgentID)
	if err != nil || peer == nil {
		return
	}
	var ids []string
	if len(peer.PeerAgentIDs) > 0 {
		json.Unmarshal(peer.PeerAgentIDs, &ids)
	}
	for _, id := range ids {
		if id == targetID {
			return
		}
	}
	ids = append(ids, targetID)
	peerJSON, _ := json.Marshal(ids)
	peer.PeerAgentIDs = peerJSON
	h.AgentStore.Update(peer)

	if peer.AgentType == "docker" && peer.Status == "running" && peer.ContainerID != "" {
		if err := h.regenerateAgentConfig(peer); err != nil {
			log.Printf("Warning: failed to regen config for peer %s: %v", peerAgentID, err)
		}
	}
}

func (h *Handler) removePeerFromAgent(peerAgentID, targetID string) {
	peer, err := h.AgentStore.GetByID(peerAgentID)
	if err != nil || peer == nil {
		return
	}
	var ids []string
	if len(peer.PeerAgentIDs) > 0 {
		json.Unmarshal(peer.PeerAgentIDs, &ids)
	}
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != targetID {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) == len(ids) {
		return
	}
	peerJSON, _ := json.Marshal(filtered)
	peer.PeerAgentIDs = peerJSON
	h.AgentStore.Update(peer)

	if peer.AgentType == "docker" && peer.Status == "running" && peer.ContainerID != "" {
		if err := h.regenerateAgentConfig(peer); err != nil {
			log.Printf("Warning: failed to regen config for peer %s: %v", peerAgentID, err)
		}
	}
}

func (h *Handler) regenerateAgentConfig(agent *store.Agent) error {
	var peerIDs []string
	if len(agent.PeerAgentIDs) > 0 {
		json.Unmarshal(agent.PeerAgentIDs, &peerIDs)
	}
	var a2aPeers []docker.A2APeer
	for _, peerID := range peerIDs {
		peerAgent, err := h.AgentStore.GetByID(peerID)
		if err != nil || peerAgent == nil {
			continue
		}
		a2aPeers = append(a2aPeers, docker.A2APeer{
			ID:          peerID,
			BearerToken: peerAgent.BearerToken,
		})
	}
	a2aPeersJSON, _ := json.Marshal(a2aPeers)

	portalURL := os.Getenv("PORTAL_INTERNAL_URL")
	if portalURL == "" {
		portalURL = "http://host.docker.internal:8080"
	}

	var modelConfig *docker.ModelConfig
	if agent.ModelID != "" {
		model, err := h.ModelStore.GetByID(agent.ModelID)
		if err == nil && model != nil {
			modelConfig = &docker.ModelConfig{
				Provider: model.Provider,
				Name:     model.ModelIdentifier,
				Endpoint: model.EndpointURL,
			}
			if model.DefaultParams != "" {
				var params map[string]interface{}
				if err := json.Unmarshal([]byte(model.DefaultParams), &params); err == nil {
					if temp, ok := params["temperature"].(float64); ok {
						modelConfig.Temperature = &temp
					}
				}
			}
		}
	}

	gatewayPort := fmt.Sprintf("%d", agent.ListenPort)
	containerConfig := docker.ContainerConfig{
		AgentID:      agent.ID,
		AgentImage:   agent.Image,
		PortalURL:    portalURL,
		PortalToken:  agent.BearerToken,
		ListenPort:   agent.ListenPort,
		A2APeersJSON: string(a2aPeersJSON),
		ModelConfig:  modelConfig,
	}

	return h.DockerMgr.RegenerateConfig(containerConfig, gatewayPort)
}

func (h *Handler) syncAgentPeers(w http.ResponseWriter, req *http.Request, agentID string) {
	agent, err := h.AgentStore.GetByID(agentID)
	if err != nil || agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	if agent.AgentType == "docker" && agent.Status == "running" && agent.ContainerID != "" {
		if err := h.regenerateAgentConfig(agent); err != nil {
			http.Error(w, fmt.Sprintf("Failed to sync peers: %v", err), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "synced"})
}

func (h *Handler) injectDefaultAgentsMD(ctx context.Context, agent *store.Agent) {
	identityFileStore := store.NewIdentityFileStore(h.DB)
	existing, _ := identityFileStore.GetByAgentAndFilename(agent.ID, "AGENTS.md")
	if existing != nil && existing.Content != "" {
		return
	}

	var peerIDs []string
	if len(agent.PeerAgentIDs) > 0 {
		json.Unmarshal(agent.PeerAgentIDs, &peerIDs)
	}
	var peers []peerEntry
	for _, pid := range peerIDs {
		if pa, err := h.AgentStore.GetByID(pid); err == nil && pa != nil {
			peers = append(peers, peerEntry{ID: pid, Name: pa.Name})
		}
	}

	content := buildDefaultAgentsMD(agent.ID, peers)

	file := &models.AgentIdentityFile{
		AgentID:  agent.ID,
		Filename: "AGENTS.md",
		Content:  content,
	}
	if err := identityFileStore.CreateOrUpdate(file); err != nil {
		log.Printf("Warning: failed to save default AGENTS.md for %s: %v", agent.ID, err)
		return
	}

	if err := h.DockerMgr.WriteWorkspaceFile(ctx, agent.ID, "AGENTS.md", []byte(content)); err != nil {
		log.Printf("Warning: failed to inject AGENTS.md into container for %s: %v", agent.ID, err)
	}
}

type peerEntry struct {
	ID   string
	Name string
}

func buildDefaultAgentsMD(agentID string, peers []peerEntry) string {
	var sb strings.Builder
	sb.WriteString("# Agent Communication Guide\n\n")
	sb.WriteString("## Your Identity\n\n")
	sb.WriteString(fmt.Sprintf("- **Agent ID:** `%s`\n\n", agentID))

	sb.WriteString("## Communicating with Other Agents\n\n")
	sb.WriteString("Use the `a2a_send` tool to send messages to peer agents:\n\n")
	sb.WriteString("```\n")
	sb.WriteString("a2a_send(to: \"<peer-id>\", message: \"Your message here\")\n")
	sb.WriteString("```\n\n")
	sb.WriteString("Messages are delivered asynchronously via the A2A protocol. ")
	sb.WriteString("Responses will arrive through your A2A channel automatically.\n\n")

	if len(peers) > 0 {
		sb.WriteString("## Available Peers\n\n")
		for _, p := range peers {
			if p.Name != "" {
				sb.WriteString(fmt.Sprintf("- **%s** (ID: `%s`)\n", p.Name, p.ID))
			} else {
				sb.WriteString(fmt.Sprintf("- `%s`\n", p.ID))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Important Notes\n\n")
	sb.WriteString("- Messages are asynchronous — responses arrive via your A2A channel.\n")
	sb.WriteString("- Your memory system preserves conversation context across async turns.\n")
	sb.WriteString("- The portal mediates all communication between agents.\n")
	return sb.String()
}
