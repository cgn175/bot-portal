package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

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

// NewManager creates a new Docker manager.
// It resolves the active Docker context (e.g., Docker Desktop vs Podman) to
// connect to the correct daemon, matching the behavior of the Docker CLI.
func NewManager() (*Manager, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}

	// If DOCKER_HOST is not explicitly set, resolve it from the active Docker context.
	// This prevents connecting to the wrong daemon when multiple runtimes are installed
	// (e.g., Docker Desktop and Podman both providing /var/run/docker.sock).
	if os.Getenv("DOCKER_HOST") == "" {
		if host := resolveDockerContextHost(); host != "" {
			opts = append(opts, client.WithHost(host))
		}
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &Manager{cli: cli}, nil
}

// resolveDockerContextHost reads ~/.docker/config.json to find the active
// Docker context and returns its endpoint host. Returns "" if the default
// context is active or if detection fails.
func resolveDockerContextHost() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	configPath := filepath.Join(home, ".docker", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	var config struct {
		CurrentContext string `json:"currentContext"`
	}
	if err := json.Unmarshal(data, &config); err != nil || config.CurrentContext == "" || config.CurrentContext == "default" {
		return ""
	}

	// Docker Desktop on macOS uses a well-known socket path
	socketPath := filepath.Join(home, ".docker", "run", "docker.sock")
	if _, err := os.Stat(socketPath); err == nil {
		return "unix://" + socketPath
	}

	return ""
}

// ModelConfig holds model configuration for environment variable injection
type ModelConfig struct {
	Provider    string
	Name        string
	Endpoint    string
	Temperature *float64
	MaxTokens   *int
}

// AuthConfig holds auth configuration for environment variable injection
type AuthConfig struct {
	Type     string
	ApiKey   string
	Endpoint string
}

// ContainerConfig holds configuration for creating a container
type ContainerConfig struct {
	AgentID      string
	AgentImage   string
	PortalURL    string
	PortalToken  string
	ListenPort   int
	A2APeersJSON string
	AgentName    string
	AgentDesc    string
	ModelConfig  *ModelConfig
	AuthConfig   *AuthConfig
}

// A2APeer holds peer info for config generation
type A2APeer struct {
	ID          string `json:"id"`
	Endpoint    string `json:"endpoint"`
	BearerToken string `json:"bearer_token"`
}

// buildEnvironmentVars builds the environment variables for a container
//
// SECURITY NOTE: This function injects API keys directly into environment variables.
// This is a known security limitation as environment variables are visible in:
//   - `docker inspect` output
//   - Process listings (`ps e`)
//   - Container logs
//   - /proc filesystem on the host
//
// For production deployments, consider using Docker secrets or mounted files instead:
//   https://docs.docker.com/engine/swarm/secrets/
//   https://docs.docker.com/compose/use-secrets/
//
// The proper fix would require architectural changes to support secret injection
// via files (e.g., /run/secrets/API_KEY) instead of environment variables.
func buildEnvironmentVars(config ContainerConfig) []string {
	var envVars []string

	// Always include base environment variables
	envVars = append(envVars, fmt.Sprintf("AGENT_ID=%s", config.AgentID))
	envVars = append(envVars, fmt.Sprintf("PORTAL_URL=%s", config.PortalURL))
	envVars = append(envVars, fmt.Sprintf("PORTAL_BEARER_TOKEN=%s", config.PortalToken))

	// Add model configuration if present
	if config.ModelConfig != nil {
		envVars = append(envVars, fmt.Sprintf("MODEL_PROVIDER=%s", config.ModelConfig.Provider))
		envVars = append(envVars, fmt.Sprintf("MODEL_NAME=%s", config.ModelConfig.Name))
		if config.ModelConfig.Endpoint != "" {
			envVars = append(envVars, fmt.Sprintf("MODEL_ENDPOINT=%s", config.ModelConfig.Endpoint))
		}
		if config.ModelConfig.Temperature != nil {
			envVars = append(envVars, fmt.Sprintf("MODEL_TEMPERATURE=%.2f", *config.ModelConfig.Temperature))
		}
		if config.ModelConfig.MaxTokens != nil {
			envVars = append(envVars, fmt.Sprintf("MODEL_MAX_TOKENS=%d", *config.ModelConfig.MaxTokens))
		}
	}

	// Add auth configuration if present
	if config.AuthConfig != nil {
		envVars = append(envVars, fmt.Sprintf("AUTH_TYPE=%s", config.AuthConfig.Type))
		if config.AuthConfig.Endpoint != "" {
			envVars = append(envVars, fmt.Sprintf("AUTH_ENDPOINT=%s", config.AuthConfig.Endpoint))
		}
		if config.AuthConfig.ApiKey != "" {
			envVars = append(envVars, fmt.Sprintf("API_KEY=%s", config.AuthConfig.ApiKey))
		}

		// Inject provider-specific API key environment variables
		switch config.AuthConfig.Type {
		case "github_copilot", "github_copilot_oauth":
			if config.AuthConfig.ApiKey != "" {
				envVars = append(envVars, fmt.Sprintf("COPILOT_API_KEY=%s", config.AuthConfig.ApiKey))
			}
		case "anthropic":
			if config.AuthConfig.ApiKey != "" {
				envVars = append(envVars, fmt.Sprintf("ANTHROPIC_API_KEY=%s", config.AuthConfig.ApiKey))
			}
		case "openai":
			if config.AuthConfig.ApiKey != "" {
				envVars = append(envVars, fmt.Sprintf("OPENAI_API_KEY=%s", config.AuthConfig.ApiKey))
			}
		}
	}

	return envVars
}

var agentConfigTmpl = template.Must(template.New("config").Parse(`workspace_dir = "/zeroclaw-data/workspace"
config_path = "/zeroclaw-data/.zeroclaw/config.toml"

[gateway]
port = {{ .GatewayPort }}
host = "[::]"
allow_public_bind = true

[channels_config]
cli = false

[channels_config.a2a]
enabled = true
listen_port = {{ .GatewayPort }}
discovery_mode = "static"
allowed_peer_ids = ["*"]
{{ range .Peers }}
[[channels_config.a2a.peers]]
id = "{{ .ID }}"
endpoint = "{{ .Endpoint }}"
bearer_token = "{{ .BearerToken }}"
enabled = true
{{ end }}
`))

// generateAgentConfig creates a zeroclaw config.toml with A2A enabled
// and writes it to a temp file, returning the path.
func generateAgentConfig(config ContainerConfig, gatewayPort string) (string, error) {
	var peers []A2APeer
	if config.A2APeersJSON != "" {
		json.Unmarshal([]byte(config.A2APeersJSON), &peers)
	}

	data := struct {
		GatewayPort string
		Peers       []A2APeer
	}{
		GatewayPort: gatewayPort,
		Peers:       peers,
	}

	tmpDir := filepath.Join(os.TempDir(), "bot-portal-configs")
	os.MkdirAll(tmpDir, 0755)

	configPath := filepath.Join(tmpDir, fmt.Sprintf("%s-config.toml", config.AgentID))
	f, err := os.Create(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()

	if err := agentConfigTmpl.Execute(f, data); err != nil {
		return "", fmt.Errorf("failed to write config: %w", err)
	}

	return configPath, nil
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

	// Inspect image to find the container's exposed port
	var containerPort nat.Port
	inspectResult, _, err := m.cli.ImageInspectWithRaw(ctx, imageID)
	if err == nil && inspectResult.Config != nil {
		for p := range inspectResult.Config.ExposedPorts {
			containerPort = p
			break
		}
	}
	if containerPort == "" {
		containerPort = nat.Port(fmt.Sprintf("%d/tcp", config.ListenPort))
	}

	// Generate agent config with A2A enabled and mount it
	gatewayPort := containerPort.Port()
	configPath, err := generateAgentConfig(config, gatewayPort)
	if err != nil {
		return "", fmt.Errorf("failed to generate agent config: %w", err)
	}

	// Map host port (from endpoint URL) to the image's exposed port
	portBindings := nat.PortMap{
		containerPort: []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", config.ListenPort)}},
	}

	// Build environment variables
	envVars := buildEnvironmentVars(config)
	envVars = append(envVars, fmt.Sprintf("ZEROCLAW_GATEWAY_PORT=%s", gatewayPort))

	// Container config - use image ID to avoid registry lookup
	containerConfig := &container.Config{
		Image:        imageID,
		Env:          envVars,
		ExposedPorts: nat.PortSet{containerPort: struct{}{}},
	}

	// Host config with config file bind mount
	hostConfig := &container.HostConfig{
		PortBindings:    portBindings,
		NetworkMode:     "bot-portal",
		AutoRemove:      false,
		PublishAllPorts: false,
		Binds: []string{
			fmt.Sprintf("%s:/zeroclaw-data/.zeroclaw/config.toml:ro", configPath),
		},
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
