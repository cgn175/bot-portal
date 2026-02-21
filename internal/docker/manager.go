package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// Manager handles Docker container lifecycle
type Manager struct {
	cli *client.Client
}

// NewManager creates a new Docker manager
func NewManager() (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &Manager{cli: cli}, nil
}

// ContainerConfig holds configuration for creating a container
type ContainerConfig struct {
	AgentID      string
	AgentImage   string
	PortalURL    string
	PortalToken  string
	ListenPort   int
	A2APeersJSON string
}

// CreateContainer creates a new Docker container for an agent
func (m *Manager) CreateContainer(ctx context.Context, config ContainerConfig) (string, error) {
	containerName := fmt.Sprintf("bot-portal-agent-%s", config.AgentID)

	// Try to get the image ID to avoid Docker adding prefixes
	imageID := config.AgentImage
	images, err := m.cli.ImageList(ctx, image.ListOptions{})
	if err == nil {
		for _, img := range images {
			for _, tag := range img.RepoTags {
				if tag == config.AgentImage {
					imageID = img.ID
					fmt.Printf("Found local image %s with ID %s\n", config.AgentImage, imageID)
					break
				}
			}
		}
	}

	// Port binding
	port := nat.Port(fmt.Sprintf("%d/tcp", config.ListenPort))
	portBindings := nat.PortMap{
		port: []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", config.ListenPort)}},
	}

	// Container config - use image ID to avoid registry lookup
	containerConfig := &container.Config{
		Image: imageID,
		Env: []string{
			fmt.Sprintf("AGENT_ID=%s", config.AgentID),
			fmt.Sprintf("PORTAL_URL=%s", config.PortalURL),
			fmt.Sprintf("PORTAL_TOKEN=%s", config.PortalToken),
			fmt.Sprintf("LISTEN_PORT=%d", config.ListenPort),
			fmt.Sprintf("A2A_PEERS=%s", config.A2APeersJSON),
		},
		ExposedPorts: nat.PortSet{port: struct{}{}},
	}

	// Host config
	hostConfig := &container.HostConfig{
		PortBindings:    portBindings,
		NetworkMode:     "bot-portal",
		AutoRemove:      false,
		PublishAllPorts: false,
	}

	// Network config
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"bot-portal": {},
		},
	}

	resp, err := m.cli.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container with image '%s': %w", config.AgentImage, err)
	}

	fmt.Printf("Created container %s with image %s\n", resp.ID[:12], imageID)
	return resp.ID, nil
}

// StartContainer starts a Docker container
func (m *Manager) StartContainer(ctx context.Context, containerID string) error {
	return m.cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

// StopContainer stops a Docker container
func (m *Manager) StopContainer(ctx context.Context, containerID string) error {
	timeout := 10
	return m.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
}

// RestartContainer restarts a Docker container
func (m *Manager) RestartContainer(ctx context.Context, containerID string) error {
	timeout := 10
	return m.cli.ContainerRestart(ctx, containerID, container.StopOptions{Timeout: &timeout})
}

// RemoveContainer removes a Docker container
func (m *Manager) RemoveContainer(ctx context.Context, containerID string) error {
	return m.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
}

// GetContainerStatus returns the status of a container
func (m *Manager) GetContainerStatus(ctx context.Context, containerID string) (string, error) {
	inspect, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}
	return inspect.State.Status, nil
}

// ContainerInfo represents container information
type ContainerInfo struct {
	ID     string
	Names  []string
	Image  string
	State  string
	Status string
	Ports  []PortInfo
}

// PortInfo represents port information
type PortInfo struct {
	IP          string
	PrivatePort int
	PublicPort  int
	Type        string
}

// EnsureNetwork ensures the bot-portal network exists
func (m *Manager) EnsureNetwork(ctx context.Context) error {
	networks, err := m.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	for _, net := range networks {
		if net.Name == "bot-portal" {
			return nil
		}
	}

	_, err = m.cli.NetworkCreate(ctx, "bot-portal", network.CreateOptions{
		Driver: "bridge",
	})
	if err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	return nil
}

// Close closes the Docker manager
func (m *Manager) Close() error {
	if m.cli != nil {
		return m.cli.Close()
	}
	return nil
}
