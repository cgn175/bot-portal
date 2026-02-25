// Package provider defines supported AI providers and their configurations.
// This registry centralizes provider metadata to ensure consistent handling
// across the codebase.
//
// Based on zeroclaw's provider implementation - most providers are OpenAI-compatible
// with different base URLs and some special header configurations.
package provider

import "strings"

// AuthStyle defines how the API key is sent in requests
type AuthStyle string

const (
	AuthStyleBearer AuthStyle = "bearer" // Authorization: Bearer <key>
	AuthStyleXApiKey AuthStyle = "x-api-key" // x-api-key: <key>
	AuthStyleCustom AuthStyle = "custom" // Custom header name
)

// Provider represents a supported AI provider with its configuration
type Provider struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	AuthType     string            `json:"authType"`     // bearer_token, github_copilot_oauth, etc.
	AuthStyle    AuthStyle         `json:"authStyle"`    // How to send the API key
	AuthHeader   string            `json:"authHeader"`   // Custom header name if AuthStyle is "custom"
	DefaultURL   string            `json:"defaultUrl"`
	APIKeyEnvVar string            `json:"apiKeyEnvVar"`
	Headers      map[string]string `json:"headers"`
	Description  string            `json:"description"`
	Aliases      []string          `json:"aliases"`      // Alternative IDs for this provider
	IsRegional   bool              `json:"isRegional"`   // Whether this provider has regional variants
	Region       string            `json:"region"`       // "global", "cn", "us", "intl"
}

// Built-in provider definitions
var (
	// OpenAI provider configuration
	OpenAI = Provider{
		ID:           "openai",
		Name:         "OpenAI",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://api.openai.com/v1",
		APIKeyEnvVar: "OPENAI_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "OpenAI GPT models (GPT-4, GPT-3.5, etc.)",
	}

	// Anthropic provider configuration
	Anthropic = Provider{
		ID:           "anthropic",
		Name:         "Anthropic",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleXApiKey,
		DefaultURL:   "https://api.anthropic.com/v1",
		APIKeyEnvVar: "ANTHROPIC_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "Anthropic Claude models",
	}

	// GitHubCopilot provider configuration
	GitHubCopilot = Provider{
		ID:           "github_copilot",
		Name:         "GitHub Copilot",
		AuthType:     "github_copilot_oauth",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://api.githubcopilot.com",
		APIKeyEnvVar: "COPILOT_API_KEY",
		Headers: map[string]string{
			"Editor-Version":        "vscode/1.85.1",
			"Editor-Plugin-Version": "copilot/1.155.0",
			"User-Agent":            "GithubCopilot/1.155.0",
		},
		Description: "GitHub Copilot models via OAuth",
		Aliases:     []string{"copilot"},
	}

	// Kimi / Moonshot provider configuration (International)
	Kimi = Provider{
		ID:           "kimi",
		Name:         "Kimi (Moonshot)",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://api.moonshot.cn/v1",
		APIKeyEnvVar: "MOONSHOT_API_KEY",
		Headers: map[string]string{
			"User-Agent": "KimiCLI/0.77",
		},
		Description: "Kimi (Moonshot AI) models",
		Aliases:     []string{"moonshot", "kimi-global", "moonshot-global"},
		IsRegional:  true,
		Region:      "cn",
	}

	// Kimi Code provider (requires specific User-Agent)
	KimiCode = Provider{
		ID:           "kimi-code",
		Name:         "Kimi Code",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://api.kimi.com/coding/v1",
		APIKeyEnvVar: "KIMI_API_KEY",
		Headers: map[string]string{
			"User-Agent": "KimiCLI/0.77",
		},
		Description: "Kimi Code specialized coding assistant",
	}

	// DeepSeek provider
	DeepSeek = Provider{
		ID:           "deepseek",
		Name:         "DeepSeek",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://api.deepseek.com",
		APIKeyEnvVar: "DEEPSEEK_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "DeepSeek AI models",
	}

	// GLM / Zhipu provider (Global)
	GLM = Provider{
		ID:           "glm",
		Name:         "GLM (Zhipu)",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://open.bigmodel.cn/api/paas/v4",
		APIKeyEnvVar: "GLM_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "GLM models by Zhipu AI (ChatGLM, etc.)",
		Aliases:     []string{"zhipu", "glm-global", "zhipu-global"},
		IsRegional:  true,
		Region:      "global",
	}

	// GLM / Zhipu provider (China)
	GLMCN = Provider{
		ID:           "glm-cn",
		Name:         "GLM China",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://open.bigmodel.cn/api/paas/v4",
		APIKeyEnvVar: "GLM_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "GLM models by Zhipu AI (China region)",
		Aliases:     []string{"zhipu-cn", "bigmodel"},
		IsRegional:  true,
		Region:      "cn",
	}

	// MiniMax provider (International)
	MiniMax = Provider{
		ID:           "minimax",
		Name:         "MiniMax",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://api.minimaxi.com/v1",
		APIKeyEnvVar: "MINIMAX_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "MiniMax AI models",
		Aliases:     []string{"minimax-intl", "minimax-io", "minimax-global", "minimaxi"},
		IsRegional:  true,
		Region:      "global",
	}

	// MiniMax provider (China)
	MiniMaxCN = Provider{
		ID:           "minimax-cn",
		Name:         "MiniMax China",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://api.minimaxi.cn/v1",
		APIKeyEnvVar: "MINIMAX_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "MiniMax AI models (China region)",
		Aliases:     []string{"minimaxi-cn"},
		IsRegional:  true,
		Region:      "cn",
	}

	// Qwen / Dashscope provider (China)
	Qwen = Provider{
		ID:           "qwen",
		Name:         "Qwen (Dashscope)",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://dashscope.aliyuncs.com/compatible-mode/v1",
		APIKeyEnvVar: "DASHSCOPE_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "Alibaba Qwen models via Dashscope",
		Aliases:     []string{"dashscope", "qwen-cn", "dashscope-cn"},
		IsRegional:  true,
		Region:      "cn",
	}

	// Qwen / Dashscope provider (International)
	QwenIntl = Provider{
		ID:           "qwen-intl",
		Name:         "Qwen International",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://dashscope-intl.aliyuncs.com/compatible-mode/v1",
		APIKeyEnvVar: "DASHSCOPE_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "Alibaba Qwen models (International)",
		Aliases:     []string{"dashscope-intl", "qwen-international", "dashscope-international"},
		IsRegional:  true,
		Region:      "intl",
	}

	// Qwen Code / OAuth provider
	QwenCode = Provider{
		ID:           "qwen-code",
		Name:         "Qwen Code",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://chat.qwen.ai/api",
		APIKeyEnvVar: "QWEN_API_KEY",
		Headers: map[string]string{
			"User-Agent": "QwenCode/1.0",
		},
		Description: "Qwen Code specialized coding assistant",
		Aliases:     []string{"qwen-oauth"},
	}

	// Mistral provider
	Mistral = Provider{
		ID:           "mistral",
		Name:         "Mistral AI",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://api.mistral.ai/v1",
		APIKeyEnvVar: "MISTRAL_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "Mistral AI models (Mistral, Mixtral, etc.)",
	}

	// Groq provider
	Groq = Provider{
		ID:           "groq",
		Name:         "Groq",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://api.groq.com/openai/v1",
		APIKeyEnvVar: "GROQ_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "Groq AI with fast inference",
	}

	// xAI / Grok provider
	XAI = Provider{
		ID:           "xai",
		Name:         "xAI (Grok)",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://api.x.ai/v1",
		APIKeyEnvVar: "XAI_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "xAI Grok models",
		Aliases:     []string{"grok"},
	}

	// Together AI provider
	Together = Provider{
		ID:           "together",
		Name:         "Together AI",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://api.together.xyz/v1",
		APIKeyEnvVar: "TOGETHER_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "Together AI inference platform",
	}

	// Fireworks AI provider
	Fireworks = Provider{
		ID:           "fireworks",
		Name:         "Fireworks AI",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://api.fireworks.ai/inference/v1",
		APIKeyEnvVar: "FIREWORKS_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "Fireworks AI inference platform",
	}

	// Perplexity provider
	Perplexity = Provider{
		ID:           "perplexity",
		Name:         "Perplexity",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://api.perplexity.ai",
		APIKeyEnvVar: "PERPLEXITY_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "Perplexity AI search and models",
	}

	// Cohere provider
	Cohere = Provider{
		ID:           "cohere",
		Name:         "Cohere",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://api.cohere.com/compatibility/v1",
		APIKeyEnvVar: "COHERE_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "Cohere AI models (Command, etc.)",
	}

	// OpenRouter provider
	OpenRouter = Provider{
		ID:           "openrouter",
		Name:         "OpenRouter",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://openrouter.ai/api/v1",
		APIKeyEnvVar: "OPENROUTER_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
			"HTTP-Referer": "https://bot-portal.local",
			"X-Title": "Bot Portal",
		},
		Description: "OpenRouter - unified API for many models",
	}

	// Ollama provider (local)
	Ollama = Provider{
		ID:           "ollama",
		Name:         "Ollama",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "http://localhost:11434/v1",
		APIKeyEnvVar: "OLLAMA_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "Ollama local LLM runner",
	}

	// LMStudio provider (local)
	LMStudio = Provider{
		ID:           "lmstudio",
		Name:         "LM Studio",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "http://localhost:1234/v1",
		APIKeyEnvVar: "LMSTUDIO_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "LM Studio local inference server",
	}

	// Gemini / Google provider
	Gemini = Provider{
		ID:           "gemini",
		Name:         "Google Gemini",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://generativelanguage.googleapis.com/v1beta",
		APIKeyEnvVar: "GOOGLE_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "Google Gemini models",
		Aliases:     []string{"google"},
	}

	// Cloudflare AI Gateway
	Cloudflare = Provider{
		ID:           "cloudflare",
		Name:         "Cloudflare AI Gateway",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://gateway.ai.cloudflare.com/v1",
		APIKeyEnvVar: "CLOUDFLARE_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "Cloudflare AI Gateway",
	}

	// Baidu Qianfan provider
	Qianfan = Provider{
		ID:           "qianfan",
		Name:         "Baidu Qianfan",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://aip.baidubce.com",
		APIKeyEnvVar: "QIANFAN_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "Baidu Qianfan AI platform",
	}

	// Z.AI provider
	ZAI = Provider{
		ID:           "zai",
		Name:         "Z.AI",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "https://api.z.ai/api/coding/paas/v4",
		APIKeyEnvVar: "ZAI_API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "Z.AI models",
		Aliases:     []string{"z.ai"},
	}

	// Custom provider for generic OpenAI-compatible endpoints
	Custom = Provider{
		ID:           "custom",
		Name:         "Custom Provider",
		AuthType:     "bearer_token",
		AuthStyle:    AuthStyleBearer,
		DefaultURL:   "",
		APIKeyEnvVar: "API_KEY",
		Headers: map[string]string{
			"User-Agent": "BotPortal/1.0",
		},
		Description: "Any OpenAI-compatible API endpoint",
	}
)

// Registry holds all supported providers
var Registry = map[string]Provider{
	OpenAI.ID:        OpenAI,
	Anthropic.ID:     Anthropic,
	GitHubCopilot.ID: GitHubCopilot,
	Kimi.ID:          Kimi,
	KimiCode.ID:      KimiCode,
	DeepSeek.ID:      DeepSeek,
	GLM.ID:           GLM,
	GLMCN.ID:         GLMCN,
	MiniMax.ID:       MiniMax,
	MiniMaxCN.ID:     MiniMaxCN,
	Qwen.ID:          Qwen,
	QwenIntl.ID:      QwenIntl,
	QwenCode.ID:      QwenCode,
	Mistral.ID:       Mistral,
	Groq.ID:          Groq,
	XAI.ID:           XAI,
	Together.ID:      Together,
	Fireworks.ID:     Fireworks,
	Perplexity.ID:    Perplexity,
	Cohere.ID:        Cohere,
	OpenRouter.ID:    OpenRouter,
	Ollama.ID:        Ollama,
	LMStudio.ID:      LMStudio,
	Gemini.ID:        Gemini,
	Cloudflare.ID:    Cloudflare,
	Qianfan.ID:       Qianfan,
	ZAI.ID:           ZAI,
	Custom.ID:        Custom,
}

// init registers all provider aliases
func init() {
	// Register all aliases from provider definitions
	for id, provider := range Registry {
		for _, alias := range provider.Aliases {
			// Only register if not already in registry
			if _, exists := Registry[alias]; !exists {
				Registry[alias] = Registry[id]
			}
		}
	}
}

// Get retrieves a provider by ID (case-insensitive)
func Get(id string) (Provider, bool) {
	p, ok := Registry[strings.ToLower(id)]
	return p, ok
}

// GetByAuthType retrieves a provider by its auth type
func GetByAuthType(authType string) (Provider, bool) {
	for _, p := range Registry {
		if p.AuthType == authType {
			return p, true
		}
	}
	return Custom, false
}

// List returns all registered providers as a slice (deduplicated by ID)
func List() []Provider {
	seen := make(map[string]bool)
	providers := make([]Provider, 0, len(Registry))
	for _, p := range Registry {
		if !seen[p.ID] {
			seen[p.ID] = true
			providers = append(providers, p)
		}
	}
	return providers
}

// IsValid checks if a provider ID is valid
func IsValid(id string) bool {
	_, ok := Registry[strings.ToLower(id)]
	return ok
}

// GetAPIKeyEnvVar returns the environment variable name for a provider's API key
func GetAPIKeyEnvVar(providerID string) string {
	if p, ok := Registry[strings.ToLower(providerID)]; ok {
		return p.APIKeyEnvVar
	}
	return "API_KEY"
}

// GetDefaultURL returns the default endpoint URL for a provider
func GetDefaultURL(providerID string) string {
	if p, ok := Registry[strings.ToLower(providerID)]; ok {
		return p.DefaultURL
	}
	return ""
}

// GetHeaders returns the headers for a provider
func GetHeaders(providerID string) map[string]string {
	if p, ok := Registry[strings.ToLower(providerID)]; ok {
		return p.Headers
	}
	return map[string]string{"User-Agent": "BotPortal/1.0"}
}

// ResolveProvider attempts to resolve a provider ID, checking aliases
func ResolveProvider(id string) (Provider, bool) {
	// Direct lookup
	if p, ok := Get(id); ok {
		return p, true
	}

	// Try case-insensitive
	lowerID := strings.ToLower(id)

	// Check for custom provider pattern: custom:https://api.example.com
	if strings.HasPrefix(lowerID, "custom:") {
		customURL := id[7:] // Remove "custom:" prefix
		return Provider{
			ID:           "custom",
			Name:         "Custom Provider",
			AuthType:     "bearer_token",
			AuthStyle:    AuthStyleBearer,
			DefaultURL:   customURL,
			APIKeyEnvVar: "API_KEY",
			Headers: map[string]string{
				"User-Agent": "BotPortal/1.0",
			},
			Description: "Custom OpenAI-compatible endpoint: " + customURL,
		}, true
	}

	return Custom, false
}

// GetAuthHeader returns the Authorization header value for a provider and API key
func GetAuthHeader(providerID, apiKey string) (headerName, headerValue string) {
	p, ok := Get(providerID)
	if !ok {
		// Default to Bearer
		return "Authorization", "Bearer " + apiKey
	}

	switch p.AuthStyle {
	case AuthStyleXApiKey:
		return "x-api-key", apiKey
	case AuthStyleCustom:
		if p.AuthHeader != "" {
			return p.AuthHeader, apiKey
		}
		return "Authorization", "Bearer " + apiKey
	default: // AuthStyleBearer
		return "Authorization", "Bearer " + apiKey
	}
}
