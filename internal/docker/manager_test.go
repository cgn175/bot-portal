package docker

import (
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
	// Base: 3, Model: 5, Auth: 3 = 11 total
	if len(envVars) != 11 {
		t.Errorf("Expected 11 environment variables, got %d: %v", len(envVars), envVars)
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
	if envMap["MODEL_NAME"] != "gpt-4-turbo" {
		t.Errorf("Expected MODEL_NAME=gpt-4-turbo, got %s", envMap["MODEL_NAME"])
	}
	if envMap["MODEL_ENDPOINT"] != "https://api.openai.com/v1" {
		t.Errorf("Expected MODEL_ENDPOINT=https://api.openai.com/v1, got %s", envMap["MODEL_ENDPOINT"])
	}
	if envMap["MODEL_TEMPERATURE"] != "0.70" {
		t.Errorf("Expected MODEL_TEMPERATURE=0.70, got %s", envMap["MODEL_TEMPERATURE"])
	}
	if envMap["MODEL_MAX_TOKENS"] != "4096" {
		t.Errorf("Expected MODEL_MAX_TOKENS=4096, got %s", envMap["MODEL_MAX_TOKENS"])
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
	if envMap["MODEL_NAME"] != "claude-3-opus" {
		t.Errorf("Expected MODEL_NAME=claude-3-opus, got %s", envMap["MODEL_NAME"])
	}
	if envMap["MODEL_ENDPOINT"] != "https://api.anthropic.com/v1" {
		t.Errorf("Expected MODEL_ENDPOINT=https://api.anthropic.com/v1, got %s", envMap["MODEL_ENDPOINT"])
	}

	// Check that optional model vars are NOT present
	if _, ok := envMap["MODEL_TEMPERATURE"]; ok {
		t.Error("MODEL_TEMPERATURE should not be present when not set")
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
