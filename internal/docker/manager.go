package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/zeroclaw/bot-portal/internal/provider"
)

// Manager handles Docker container lifecycle
type Manager struct {
	cli *client.Client
}

const ZEROCLAW_WORK_DIR = "/zeroclaw-data"

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
// Docker context and returns its endpoint host. It follows the Docker CLI
// logic to resolve the endpoint from context metadata.
func resolveDockerContextHost() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	configPath := filepath.Join(home, ".docker", "config.json")
	var currentContext string
	if data, err := os.ReadFile(configPath); err == nil {
		var config struct {
			CurrentContext string `json:"currentContext"`
		}
		if err := json.Unmarshal(data, &config); err == nil {
			currentContext = config.CurrentContext
		}
	}

	if currentContext != "" && currentContext != "default" {
		// Search for context metadata
		contextMetaDir := filepath.Join(home, ".docker", "contexts", "meta")
		if files, err := os.ReadDir(contextMetaDir); err == nil {
			for _, f := range files {
				if !f.IsDir() {
					continue
				}
				metaPath := filepath.Join(contextMetaDir, f.Name(), "meta.json")
				if metaData, err := os.ReadFile(metaPath); err == nil {
					var meta struct {
						Name      string `json:"Name"`
						Endpoints struct {
							Docker struct {
								Host string `json:"Host"`
							} `json:"docker"`
						} `json:"Endpoints"`
					}
					if err := json.Unmarshal(metaData, &meta); err == nil && meta.Name == currentContext {
						host := meta.Endpoints.Docker.Host
						if host != "" {
							// Check if the host is a local socket and if it exists
							if strings.HasPrefix(host, "unix://") {
								socketPath := strings.TrimPrefix(host, "unix://")
								if _, err := os.Stat(socketPath); err == nil {
									return host
								}
								// If the socket doesn't exist, we'll continue and try Podman fallback
							} else {
								// For other protocols (npipe://, tcp:// etc), return as is
								return host
							}
						}
					}
				}
			}
		}
	}

	// Fallback for Podman if no Docker context is set or if the context is broken.
	// Check both Mac/Linux and Windows common paths.
	podmanPaths := []string{
		// macOS/Linux (Podman Machine)
		filepath.Join(home, ".local/share/containers/podman/machine/qemu/podman.sock"),
		filepath.Join(home, ".local/share/containers/podman/machine/applehv/podman.sock"),

		// macOS temporary paths (sometimes used by Podman Desktop/Machine)
		// User's specific path: /var/folders/gz/vlc1419x1xdbs03k67m68qqc0000gn/T/podman/podman-machine-default-api.sock
		filepath.Join(os.TempDir(), "podman/podman-machine-default-api.sock"),

		// Linux Rootless
		filepath.Join("/run/user", fmt.Sprint(os.Getuid()), "podman/podman.sock"),

		// Linux Rootful / Standard
		"/run/podman/podman.sock",
		"/var/run/podman.sock",

		// Windows
		`\\.\pipe\podman-machine-default`,
		`\\.\pipe\podman-machine-default-api`,
	}

	// Check XDG_RUNTIME_DIR if it's set (Linux)
	if xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR"); xdgRuntimeDir != "" {
		podmanPaths = append([]string{filepath.Join(xdgRuntimeDir, "podman/podman.sock")}, podmanPaths...)
	}

	for _, p := range podmanPaths {
		if _, err := os.Stat(p); err == nil {
			if strings.HasPrefix(p, `\\.\pipe\`) {
				return "npipe://" + p
			}
			return "unix://" + p
		}
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
//
//	https://docs.docker.com/engine/swarm/secrets/
//	https://docs.docker.com/compose/use-secrets/
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
		var modelProvider = config.ModelConfig.Provider
		if modelProvider == "custom" {
			modelProvider = "custom:" + config.ModelConfig.Endpoint
		}
		envVars = append(envVars, fmt.Sprintf("MODEL_PROVIDER=%s", modelProvider))
		envVars = append(envVars, fmt.Sprintf("MODEL=%s", config.ModelConfig.Name))
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
		// Use the provider registry to determine the correct env var name
		if config.AuthConfig.ApiKey != "" {
			envVarName := provider.GetAPIKeyEnvVar(config.AuthConfig.Type)
			if envVarName != "API_KEY" {
				envVars = append(envVars, fmt.Sprintf("%s=%s", envVarName, config.AuthConfig.ApiKey))
			}
		}

		// Also inject based on model provider if available
		if config.ModelConfig != nil && config.ModelConfig.Provider != "" {
			envVarName := provider.GetAPIKeyEnvVar(config.ModelConfig.Provider)
			if envVarName != "API_KEY" && config.AuthConfig.ApiKey != "" {
				// Only add if not already added
				found := false
				for _, env := range envVars {
					if len(env) > len(envVarName) && env[:len(envVarName)] == envVarName {
						found = true
						break
					}
				}
				if !found {
					envVars = append(envVars, fmt.Sprintf("%s=%s", envVarName, config.AuthConfig.ApiKey))
				}
			}
		}
	}

	return envVars
}

var agentConfigTmpl = template.Must(template.New("config").Parse(`workspace_dir = "{{ .ZeroClawWorkDir }}/workspace"
config_path = "{{ .ZeroClawWorkDir }}/.zeroclaw/config.toml"
{{ if .DefaultProvider }}default_provider = "{{ .DefaultProvider }}"
{{ end }}{{ if .DefaultModel }}default_model = "{{ .DefaultModel }}"
{{ end }}{{ if .ApiURL }}api_url = "{{ .ApiURL }}"
{{ end }}default_temperature = {{ .DefaultTemperature }}

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

[autonomy]
auto_approve = ["file_read", "memory_recall", "a2a_send"]
level = "full"
workspace_only = true
allowed_commands = ["*"]
forbidden_paths = []
max_actions_per_hour = 50
max_cost_per_day_cents = 200
`))

// generateAgentConfig creates a zeroclaw config.toml with A2A enabled
// and writes it to a temp file, returning the path.
func generateAgentConfig(config ContainerConfig, gatewayPort string) (string, error) {
	var peers []A2APeer
	if config.A2APeersJSON != "" {
		json.Unmarshal([]byte(config.A2APeersJSON), &peers)
	}

	// For Docker agents, we always derive the endpoint from the AgentID
	for i := range peers {
		// Assume internal Docker hostname: http://<AgentID>:<port>
		peers[i].Endpoint = fmt.Sprintf("http://bot-portal-agent-%s:%s", peers[i].ID, gatewayPort)
	}

	// Always add portal as a peer so the agent recognizes portal's bearer token
	// Use host.docker.internal to allow agent to reach portal from inside container
	portalEndpoint := config.PortalURL
	if portalEndpoint == "" {
		// Default to host.docker.internal if not specified
		portalEndpoint = "http://host.docker.internal:8080"
	}
	peers = append(peers, A2APeer{
		ID:          "portal",
		Endpoint:    portalEndpoint,
		BearerToken: config.PortalToken,
	})

	// Extract model config for template
	var defaultProvider, defaultModel, apiURL string
	defaultTemperature := 0.7
	if config.ModelConfig != nil {
		defaultProvider = config.ModelConfig.Provider
		defaultModel = config.ModelConfig.Name
		apiURL = config.ModelConfig.Endpoint
		if config.ModelConfig.Temperature != nil {
			defaultTemperature = *config.ModelConfig.Temperature
		}
	}

	data := struct {
		GatewayPort        string
		Peers              []A2APeer
		DefaultProvider    string
		DefaultModel       string
		ApiURL             string
		DefaultTemperature float64
		ZeroClawWorkDir    string
	}{
		GatewayPort:        gatewayPort,
		Peers:              peers,
		DefaultProvider:    defaultProvider,
		DefaultModel:       defaultModel,
		ApiURL:             apiURL,
		DefaultTemperature: defaultTemperature,
		ZeroClawWorkDir:    ZEROCLAW_WORK_DIR,
	}

	tmpDir := os.Getenv("AGENT_CONFIG_DIR")
	if tmpDir == "" {
		tmpDir = filepath.Join(os.TempDir(), "bot-portal-configs")
	}
	os.MkdirAll(tmpDir, 0755)

	configPath := filepath.Join(tmpDir, fmt.Sprintf("%s-config.toml", config.AgentID))
	// Always remove whatever exists at that path to be safe, then ensure it's a file
	if err := os.RemoveAll(configPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to remove existing config path: %w", err)
	}

	f, err := os.OpenFile(configPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()

	if err := agentConfigTmpl.Execute(f, data); err != nil {
		return "", fmt.Errorf("failed to write config: %w", err)
	}
	f.Close()

	// Make config file readable by all users (agent runs as nobody:nobody)
	if err := os.Chmod(configPath, 0644); err != nil {
		fmt.Printf("Warning: failed to chmod config file: %v\n", err)
	}

	if err := os.Chown(configPath, 65534, 65534); err != nil {
		fmt.Printf("Warning: failed to chown config file: %v\n", err)
	}

	return configPath, nil
}

// CreateContainer creates a new Docker container for an agent
func (m *Manager) CreateContainer(ctx context.Context, config ContainerConfig) (string, error) {
	containerName := m.getContainerNameByAgentId(config.AgentID)

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

	// Build environment variables
	envVars := buildEnvironmentVars(config)
	envVars = append(envVars, fmt.Sprintf("ZEROCLAW_GATEWAY_PORT=%s", gatewayPort))

	// Port bindings for host access
	portBindings := nat.PortMap{}
	if config.ListenPort > 0 {
		portBindings[containerPort] = []nat.PortBinding{
			{
				HostIP:   "127.0.0.1",
				HostPort: fmt.Sprintf("%d", config.ListenPort),
			},
		}
	}

	// Container config - use image ID to avoid registry lookup
	containerConfig := &container.Config{
		Image:        imageID,
		Env:          envVars,
		ExposedPorts: nat.PortSet{containerPort: struct{}{}},
	}

	// Determine which network to use
	selectedNetwork := "bot-portal"
	networks, err := m.cli.NetworkList(ctx, network.ListOptions{})
	if err == nil {
		for _, net := range networks {
			if net.Name == "bot-portal-network" {
				selectedNetwork = "bot-portal-network"
				break
			}
		}
	}

	// Determine host configuration path for bind mount.
	// If AGENT_CONFIG_DIR_HOST is set, we use it as the source prefix for the bind mount
	// (this is needed when running the portal inside a Docker container).
	hostConfigPath := configPath
	if hostPrefix := os.Getenv("AGENT_CONFIG_DIR_HOST"); hostPrefix != "" {
		hostConfigPath = filepath.Join(hostPrefix, filepath.Base(configPath))
	}
	absConfigPath, _ := filepath.Abs(hostConfigPath)

	// On some systems (like macOS with Podman), if the file doesn't exist on the host,
	// Docker/Podman might create it as a directory. We ensure it's a file above.
	// We also use Mounts instead of Binds for more explicit control if needed,
	// but Binds is usually fine if the host path exists.
	hostConfig := &container.HostConfig{
		NetworkMode:     container.NetworkMode(selectedNetwork),
		PortBindings:    portBindings,
		AutoRemove:      false,
		PublishAllPorts: false,
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeBind,
				Source:   absConfigPath,
				Target:   ZEROCLAW_WORK_DIR + "/.zeroclaw/config.toml",
				ReadOnly: false,
			},
			{
				Type:   mount.TypeVolume,
				Source: fmt.Sprintf("bot-portal-agent-%s-workspace", config.AgentID),
				Target: ZEROCLAW_WORK_DIR + "/workspace",
			},
		},
	}

	// Network config
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			selectedNetwork: {
				Aliases: []string{config.AgentID},
			},
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

// ContainerExists checks if a container exists in Docker
func (m *Manager) ContainerExists(ctx context.Context, containerID string) bool {
	_, err := m.cli.ContainerInspect(ctx, containerID)
	return err == nil
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

	// Check both "bot-portal" and "bot-portal-network" for consistency with docker-compose
	var selectedNetwork string
	for _, netName := range []string{"bot-portal", "bot-portal-network"} {
		for _, net := range networks {
			if net.Name == netName {
				selectedNetwork = netName
				break
			}
		}
		if selectedNetwork != "" {
			break
		}
	}

	if selectedNetwork != "" {
		return nil
	}

	_, err = m.cli.NetworkCreate(ctx, "bot-portal", network.CreateOptions{
		Driver: "bridge",
	})
	if err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	return nil
}

// GetContainerIP returns the IP address of a container in the bot-portal network
func (m *Manager) GetContainerIP(ctx context.Context, containerID string) (string, error) {
	inspect, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}

	// Try "bot-portal" network first
	if network, ok := inspect.NetworkSettings.Networks["bot-portal"]; ok {
		return network.IPAddress, nil
	}

	// Try "bot-portal-network" as fallback
	if network, ok := inspect.NetworkSettings.Networks["bot-portal-network"]; ok {
		return network.IPAddress, nil
	}

	return "", fmt.Errorf("container not connected to bot-portal or bot-portal-network")
}

// RegenerateConfig regenerates the config.toml for an agent on the host.
// Since the file is bind-mounted, the running container sees the updated file.
func (m *Manager) RegenerateConfig(config ContainerConfig, gatewayPort string) error {
	_, err := generateAgentConfig(config, gatewayPort)
	return err
}

// Close closes the Docker manager
func (m *Manager) Close() error {
	if m.cli != nil {
		return m.cli.Close()
	}
	return nil
}
