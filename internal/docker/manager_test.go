package docker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ptrFloat64(f float64) *float64 {
	return &f
}

func ptrInt(i int) *int {
	return &i
}

func TestBuildEnvironmentVars_CompleteConfig(t *testing.T) {
	config := ContainerConfig{
		AgentID:     "agent1",
		AgentImage:  "test-image",
		PortalURL:   "http://localhost:8080",
		PortalToken: "test-token-123",
		ListenPort:  8081,
		ModelConfig: &ModelConfig{
			Provider:    "openai",
			Name:        "gpt-4-turbo",
			Endpoint:    "https://api.openai.com/v1",
			Temperature: ptrFloat64(0.7),
			MaxTokens:   ptrInt(4096),
		},
		AuthConfig: &AuthConfig{
			Type:     "api_key",
			ApiKey:   "sk-test123",
			Endpoint: "https://api.openai.com/v1",
		},
	}

	envVars := buildEnvironmentVars(config)

	// Check that we have the expected number of env vars
	// Base: 3, Model: 2, Auth: 3, Provider-specific API key: 1 = 9 total
	if len(envVars) != 9 {
		t.Errorf("Expected 9 environment variables, got %d: %v", len(envVars), envVars)
	}

	// Build a map for easier checking
	envMap := make(map[string]string)
	for _, env := range envVars {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Check base variables
	if envMap["AGENT_ID"] != "agent1" {
		t.Errorf("Expected AGENT_ID=agent1, got %s", envMap["AGENT_ID"])
	}
	if envMap["PORTAL_URL"] != "http://localhost:8080" {
		t.Errorf("Expected PORTAL_URL=http://localhost:8080, got %s", envMap["PORTAL_URL"])
	}
	if envMap["PORTAL_BEARER_TOKEN"] != "test-token-123" {
		t.Errorf("Expected PORTAL_BEARER_TOKEN=test-token-123, got %s", envMap["PORTAL_BEARER_TOKEN"])
	}

	// Check model variables
	if envMap["MODEL_PROVIDER"] != "openai" {
		t.Errorf("Expected MODEL_PROVIDER=openai, got %s", envMap["MODEL_PROVIDER"])
	}
	if envMap["MODEL"] != "gpt-4-turbo" {
		t.Errorf("Expected MODEL=gpt-4-turbo, got %s", envMap["MODEL"])
	}

	// Check auth variables
	if envMap["AUTH_TYPE"] != "api_key" {
		t.Errorf("Expected AUTH_TYPE=api_key, got %s", envMap["AUTH_TYPE"])
	}
	if envMap["AUTH_ENDPOINT"] != "https://api.openai.com/v1" {
		t.Errorf("Expected AUTH_ENDPOINT=https://api.openai.com/v1, got %s", envMap["AUTH_ENDPOINT"])
	}
	if envMap["API_KEY"] != "sk-test123" {
		t.Errorf("Expected API_KEY=sk-test123, got %s", envMap["API_KEY"])
	}
	if envMap["OPENAI_API_KEY"] != "sk-test123" {
		t.Errorf("Expected OPENAI_API_KEY=sk-test123, got %s", envMap["OPENAI_API_KEY"])
	}
}

func TestBuildEnvironmentVars_NoModelOrAuth(t *testing.T) {
	config := ContainerConfig{
		AgentID:     "agent2",
		AgentImage:  "test-image",
		PortalURL:   "http://localhost:8080",
		PortalToken: "test-token-456",
		ListenPort:  8082,
	}

	envVars := buildEnvironmentVars(config)

	// Should only have base env vars
	if len(envVars) != 3 {
		t.Errorf("Expected 3 environment variables, got %d: %v", len(envVars), envVars)
	}

	// Build a map for easier checking
	envMap := make(map[string]string)
	for _, env := range envVars {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Check base variables
	if envMap["AGENT_ID"] != "agent2" {
		t.Errorf("Expected AGENT_ID=agent2, got %s", envMap["AGENT_ID"])
	}
	if envMap["PORTAL_URL"] != "http://localhost:8080" {
		t.Errorf("Expected PORTAL_URL=http://localhost:8080, got %s", envMap["PORTAL_URL"])
	}
	if envMap["PORTAL_BEARER_TOKEN"] != "test-token-456" {
		t.Errorf("Expected PORTAL_BEARER_TOKEN=test-token-456, got %s", envMap["PORTAL_BEARER_TOKEN"])
	}

	// Check that model and auth vars are NOT present
	if _, ok := envMap["MODEL_PROVIDER"]; ok {
		t.Error("MODEL_PROVIDER should not be present when no model config")
	}
	if _, ok := envMap["AUTH_TYPE"]; ok {
		t.Error("AUTH_TYPE should not be present when no auth config")
	}
}

func TestBuildEnvironmentVars_OnlyModel(t *testing.T) {
	config := ContainerConfig{
		AgentID:     "agent3",
		AgentImage:  "test-image",
		PortalURL:   "http://localhost:8080",
		PortalToken: "test-token-789",
		ListenPort:  8083,
		ModelConfig: &ModelConfig{
			Provider: "anthropic",
			Name:     "claude-3-opus",
			Endpoint: "https://api.anthropic.com/v1",
			// No temperature or max_tokens
		},
	}

	envVars := buildEnvironmentVars(config)

	// Build a map for easier checking
	envMap := make(map[string]string)
	for _, env := range envVars {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Check model variables present
	if envMap["MODEL_PROVIDER"] != "anthropic" {
		t.Errorf("Expected MODEL_PROVIDER=anthropic, got %s", envMap["MODEL_PROVIDER"])
	}
	if envMap["MODEL"] != "claude-3-opus" {
		t.Errorf("Expected MODEL=claude-3-opus, got %s", envMap["MODEL"])
	}
	if _, ok := envMap["MODEL_MAX_TOKENS"]; ok {
		t.Error("MODEL_MAX_TOKENS should not be present when not set")
	}

	// Check that auth vars are NOT present
	if _, ok := envMap["AUTH_TYPE"]; ok {
		t.Error("AUTH_TYPE should not be present when no auth config")
	}
}

func TestBuildEnvironmentVars_OnlyAuth(t *testing.T) {
	config := ContainerConfig{
		AgentID:     "agent4",
		AgentImage:  "test-image",
		PortalURL:   "http://localhost:8080",
		PortalToken: "test-token-abc",
		ListenPort:  8084,
		AuthConfig: &AuthConfig{
			Type: "bearer",
			// No api_key or endpoint
		},
	}

	envVars := buildEnvironmentVars(config)

	// Build a map for easier checking
	envMap := make(map[string]string)
	for _, env := range envVars {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Check auth variables present
	if envMap["AUTH_TYPE"] != "bearer" {
		t.Errorf("Expected AUTH_TYPE=bearer, got %s", envMap["AUTH_TYPE"])
	}

	// Check that optional auth vars are NOT present
	if _, ok := envMap["AUTH_ENDPOINT"]; ok {
		t.Error("AUTH_ENDPOINT should not be present when not set")
	}
	if _, ok := envMap["API_KEY"]; ok {
		t.Error("API_KEY should not be present when not set")
	}

	// Check that model vars are NOT present
	if _, ok := envMap["MODEL_PROVIDER"]; ok {
		t.Error("MODEL_PROVIDER should not be present when no model config")
	}
}

func TestBuildEnvironmentVars_EmptyEndpoint(t *testing.T) {
	config := ContainerConfig{
		AgentID:     "agent5",
		AgentImage:  "test-image",
		PortalURL:   "http://localhost:8080",
		PortalToken: "test-token-def",
		ListenPort:  8085,
		ModelConfig: &ModelConfig{
			Provider: "openai",
			Name:     "gpt-3.5-turbo",
			Endpoint: "", // Empty endpoint should not be added
		},
		AuthConfig: &AuthConfig{
			Type:     "api_key",
			Endpoint: "", // Empty endpoint should not be added
			ApiKey:   "key123",
		},
	}

	envVars := buildEnvironmentVars(config)

	// Build a map for easier checking
	envMap := make(map[string]string)
	for _, env := range envVars {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Check that empty endpoints are NOT added
	if _, ok := envMap["MODEL_ENDPOINT"]; ok {
		t.Error("MODEL_ENDPOINT should not be present when empty")
	}
	if _, ok := envMap["AUTH_ENDPOINT"]; ok {
		t.Error("AUTH_ENDPOINT should not be present when empty")
	}

	// Check that other vars are present
	if envMap["MODEL_PROVIDER"] != "openai" {
		t.Errorf("Expected MODEL_PROVIDER=openai, got %s", envMap["MODEL_PROVIDER"])
	}
	if envMap["API_KEY"] != "key123" {
		t.Errorf("Expected API_KEY=key123, got %s", envMap["API_KEY"])
	}
}

func TestResolveDockerContextHost(t *testing.T) {
	// Create a temporary home directory
	tempHome, err := os.MkdirTemp("", "docker-test-home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempHome)

	// Save original HOME and restore it later
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", originalHome)

	t.Run("Default - No config.json", func(t *testing.T) {
		// Ensure no Podman socks exist in the search paths during this test
		// Or just skip checking if we are on a system where one actually exists in TmpDir
		host := resolveDockerContextHost()
		if host != "" && !strings.Contains(host, "podman") {
			t.Errorf("Expected empty host or podman fallback, got %s", host)
		}
	})

	t.Run("Podman Fallback", func(t *testing.T) {
		podmanSockDir := filepath.Join(tempHome, ".local/share/containers/podman/machine/qemu")
		if err := os.MkdirAll(podmanSockDir, 0755); err != nil {
			t.Fatal(err)
		}
		podmanSock := filepath.Join(podmanSockDir, "podman.sock")
		if err := os.WriteFile(podmanSock, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		host := resolveDockerContextHost()
		expected := "unix://" + podmanSock
		if host != expected {
			t.Errorf("Expected %s, got %s", expected, host)
		}

		// Test XDG_RUNTIME_DIR
		os.Remove(podmanSock)
		xdgDir, _ := os.MkdirTemp("", "xdg-test")
		defer os.RemoveAll(xdgDir)
		os.Setenv("XDG_RUNTIME_DIR", xdgDir)
		defer os.Unsetenv("XDG_RUNTIME_DIR")

		xdgPodmanDir := filepath.Join(xdgDir, "podman")
		os.MkdirAll(xdgPodmanDir, 0755)
		xdgSock := filepath.Join(xdgPodmanDir, "podman.sock")
		os.WriteFile(xdgSock, []byte(""), 0644)

		host = resolveDockerContextHost()
		if host != "unix://"+xdgSock {
			t.Errorf("Expected unix://%s, got %s", xdgSock, host)
		}
	})

	t.Run("Docker Context", func(t *testing.T) {
		// Set up docker config
		dockerDir := filepath.Join(tempHome, ".docker")
		if err := os.MkdirAll(dockerDir, 0755); err != nil {
			t.Fatal(err)
		}
		config := map[string]string{"currentContext": "test-context"}
		configData, _ := json.Marshal(config)
		if err := os.WriteFile(filepath.Join(dockerDir, "config.json"), configData, 0644); err != nil {
			t.Fatal(err)
		}

		// Set up context meta
		metaDir := filepath.Join(dockerDir, "contexts", "meta", "somehash")
		if err := os.MkdirAll(metaDir, 0755); err != nil {
			t.Fatal(err)
		}
		meta := map[string]interface{}{
			"Name": "test-context",
			"Endpoints": map[string]interface{}{
				"docker": map[string]string{
					"Host": "unix:///tmp/test.sock",
				},
			},
		}
		metaData, _ := json.Marshal(meta)
		if err := os.WriteFile(filepath.Join(metaDir, "meta.json"), metaData, 0644); err != nil {
			t.Fatal(err)
		}

		host := "unix:///tmp/test.sock"
		socketPath := "/tmp/test.sock"
		if err := os.WriteFile(socketPath, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(socketPath)

		resolvedHost := resolveDockerContextHost()
		if resolvedHost != host {
			t.Errorf("Expected %s, got %s", host, resolvedHost)
		}
	})

	t.Run("Broken Docker Context Fallback to Podman", func(t *testing.T) {
		// Set up docker config with broken context
		dockerDir := filepath.Join(tempHome, ".docker")
		os.MkdirAll(dockerDir, 0755)
		config := map[string]string{"currentContext": "broken-context"}
		configData, _ := json.Marshal(config)
		os.WriteFile(filepath.Join(dockerDir, "config.json"), configData, 0644)

		// Set up context meta pointing to non-existent file
		metaDir := filepath.Join(dockerDir, "contexts", "meta", "brokenhash")
		os.MkdirAll(metaDir, 0755)
		meta := map[string]interface{}{
			"Name": "broken-context",
			"Endpoints": map[string]interface{}{
				"docker": map[string]string{
					"Host": "unix:///non/existent/sock",
				},
			},
		}
		metaData, _ := json.Marshal(meta)
		os.WriteFile(filepath.Join(metaDir, "meta.json"), metaData, 0644)

		// Set up working Podman fallback
		podmanSockDir := filepath.Join(tempHome, ".local/share/containers/podman/machine/qemu")
		os.MkdirAll(podmanSockDir, 0755)
		podmanSock := filepath.Join(podmanSockDir, "podman.sock")
		os.WriteFile(podmanSock, []byte(""), 0644)

		host := resolveDockerContextHost()
		expected := "unix://" + podmanSock
		if host != expected {
			t.Errorf("Expected fallback to Podman %s, got %s", expected, host)
		}
	})
}
