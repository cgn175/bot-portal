package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
)

// MaxIdentityFileSize is the maximum size for identity files (16KB)
const MaxIdentityFileSize = 16 * 1024

// Allowed identity files that can be read/written
var allowedIdentityFiles = map[string]bool{
	"IDENTITY.md": true,
	"SOUL.md":     true,
	"AGENTS.md":   true,
	"USER.md":     true,
	"TOOLS.md":    true,
}

// validateIdentityFilename checks if the filename is in the allowlist
// and prevents path traversal attacks
func validateIdentityFilename(filename string) error {
	// Check for empty filename
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	// Check for path traversal attempts
	cleanPath := filepath.Clean(filename)
	if cleanPath != filename {
		return fmt.Errorf("invalid filename: path cleaning changed the value")
	}

	// Check for directory traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return fmt.Errorf("invalid filename: path traversal not allowed")
	}

	// Check if file is in allowlist
	if !allowedIdentityFiles[filename] {
		return fmt.Errorf("invalid filename: %s is not an editable identity file", filename)
	}

	return nil
}

// ReadWorkspaceFile reads a file from an agent's workspace volume using Docker CP API
func (m *Manager) ReadWorkspaceFile(ctx context.Context, agentID string, filename string) ([]byte, error) {
	if err := validateIdentityFilename(filename); err != nil {
		return nil, err
	}

	// Find the container for this agent
	containerName := m.getContainerNameByAgentId(agentID)
	cont, err := m.getContainerByName(ctx, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to find container for agent %s: %w", agentID, err)
	}

	// Read the file using Docker CopyFromContainer API
	workspacePath := fmt.Sprintf("%s/workspace/%s", ZEROCLAW_WORK_DIR, filename)
	reader, _, err := m.cli.CopyFromContainer(ctx, cont.ID, workspacePath)
	if err != nil {
		// File doesn't exist in container
		if strings.Contains(err.Error(), "No such container path") || strings.Contains(err.Error(), "not found") {
			return []byte{}, nil
		}
		return nil, fmt.Errorf("failed to copy file from container: %w", err)
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	_, err = tr.Next()
	if err != nil {
		return nil, fmt.Errorf("failed to read tar header: %w", err)
	}

	content, err := io.ReadAll(tr)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	// Check size limit
	if len(content) > MaxIdentityFileSize {
		return nil, fmt.Errorf("file exceeds maximum size of %d bytes", MaxIdentityFileSize)
	}

	return content, nil
}

// WriteWorkspaceFile writes a file to an agent's workspace volume using Docker CP API
func (m *Manager) WriteWorkspaceFile(ctx context.Context, agentID string, filename string, content []byte) error {
	if err := validateIdentityFilename(filename); err != nil {
		return err
	}

	// Check size limit
	if len(content) > MaxIdentityFileSize {
		return fmt.Errorf("content exceeds maximum size of %d bytes", MaxIdentityFileSize)
	}

	// Find the container for this agent
	containerName := m.getContainerNameByAgentId(agentID)
	cont, err := m.getContainerByName(ctx, containerName)
	if err != nil {
		return fmt.Errorf("failed to find container for agent %s: %w", agentID, err)
	}

	// Create a tar archive with the file content
	workspacePath := fmt.Sprintf("/workspace/%s", filename)
	tarReader, err := createTarArchive(filename, content)
	if err != nil {
		return fmt.Errorf("failed to create tar archive: %w", err)
	}

	// Use docker cp API to copy file into container
	err = m.cli.CopyToContainer(ctx, cont.ID, ZEROCLAW_WORK_DIR+"/workspace", tarReader, container.CopyToContainerOptions{
		AllowOverwriteDirWithFile: true,
	})
	if err != nil {
		return fmt.Errorf("failed to copy file to container: %w", err)
	}

	// Ensure correct ownership (in case container runs as non-root)
	chownConfig := container.ExecOptions{
		Cmd:          []string{"chown", "claude:claude", workspacePath},
		AttachStdout: false,
		AttachStderr: false,
	}

	chownResp, err := m.cli.ContainerExecCreate(ctx, cont.ID, chownConfig)
	if err == nil {
		// Best effort - ignore errors
		_ = m.cli.ContainerExecStart(ctx, chownResp.ID, container.ExecStartOptions{})
	}

	return nil
}

// createTarArchive creates a tar archive containing a single file
func createTarArchive(filename string, content []byte) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	hdr := &tar.Header{
		Name: filename,
		Mode: 0644,
		Size: int64(len(content)),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}

	if _, err := tw.Write(content); err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	return &buf, nil
}

// ContainerSummary wraps the container information we need
type ContainerSummary struct {
	ID   string
	Name string
}

// getContainerByName finds a container by its name
func (m *Manager) getContainerByName(ctx context.Context, name string) (*ContainerSummary, error) {
	if m.cli == nil {
		return nil, fmt.Errorf("docker client not initialized")
	}

	containers, err := m.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	for _, c := range containers {
		for _, n := range c.Names {
			// Docker adds a leading slash to container names
			if n == "/"+name || n == name {
				return &ContainerSummary{
					ID:   c.ID,
					Name: name,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("container %s not found", name)
}

func (m *Manager) getContainerNameByAgentId(agentId string) string {
	return fmt.Sprintf("bot-portal-agent-%s", agentId)
}
