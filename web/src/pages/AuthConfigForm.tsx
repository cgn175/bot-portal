import { useState, useCallback, useEffect } from 'react'
import { api, CreateAuthConfigRequest, Provider } from '../api/client'
import Modal from '../components/Modal'
import Alert from '../components/Alert'
import CopilotOAuthModal from '../components/CopilotOAuthModal'
import { AuthConfig } from '../api/client'

interface AuthConfigFormProps {
  config?: AuthConfig
  onSuccess: () => void
  onCancel: () => void
}

type AuthMode = 'choose' | 'copilot' | 'custom' | 'provider'

// Provider categories for the UI
const PROVIDER_CATEGORIES = [
  {
    name: 'Popular',
    providers: ['openai', 'anthropic', 'github_copilot', 'gemini']
  },
  {
    name: 'Chinese Providers',
    providers: ['kimi', 'kimi-code', 'deepseek', 'glm', 'glm-cn', 'minimax', 'minimax-cn', 'qwen', 'qwen-intl', 'qwen-code', 'qianfan', 'zai']
  },
  {
    name: 'Open Source / Inference',
    providers: ['groq', 'together', 'fireworks', 'mistral', 'ollama', 'lmstudio', 'openrouter']
  },
  {
    name: 'Enterprise',
    providers: ['xai', 'perplexity', 'cohere', 'cloudflare']
  }
]

export default function AuthConfigForm({ config, onSuccess, onCancel }: AuthConfigFormProps) {
  const [mode, setMode] = useState<AuthMode>(config ? (config.authType === 'github_copilot_oauth' ? 'copilot' : 'custom') : 'choose')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [providers, setProviders] = useState<Provider[]>([])
  const [loadingProviders, setLoadingProviders] = useState(false)

  // Custom form state
  const [name, setName] = useState(config?.name || '')
  const [provider, setProvider] = useState(config?.provider || 'custom')
  const [apiUrl, setApiUrl] = useState(config?.endpointUrl || '')
  const [apiKey, setApiKey] = useState('')

  // Load providers when entering provider mode
  useEffect(() => {
    if (mode === 'provider' && providers.length === 0) {
      setLoadingProviders(true)
      api.listProviders()
        .then(data => {
          setProviders(data)
        })
        .catch(err => {
          console.error('Failed to load providers:', err)
        })
        .finally(() => {
          setLoadingProviders(false)
        })
    }
  }, [mode, providers.length])

  // Update API URL when provider changes
  useEffect(() => {
    if (mode === 'provider' && provider && !config) {
      const selectedProvider = providers.find(p => p.id === provider)
      if (selectedProvider?.defaultUrl) {
        setApiUrl(selectedProvider.defaultUrl)
      }
    }
  }, [provider, providers, mode, config])

  const handleCopilotSuccess = useCallback(() => {
    onSuccess()
  }, [onSuccess])

  const handleCustomSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      const id = config?.id || name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '')

      const submitData: CreateAuthConfigRequest = {
        id,
        name,
        provider: mode === 'provider' ? provider : 'custom',
        authType: 'bearer_token',
        credentials: { api_key: apiKey },
        endpointUrl: apiUrl
      }

      if (config) {
        if (!apiKey) {
          submitData.credentials = {}
        }
        await api.updateAuthConfig(config.id, submitData)
      } else {
        await api.createAuthConfig(submitData)
      }
      onSuccess()
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to ${config ? 'update' : 'create'} auth config`)
    } finally {
      setLoading(false)
    }
  }

  // Mode selection screen
  if (mode === 'choose') {
    return (
      <Modal
        isOpen={true}
        onClose={onCancel}
        title="Add Auth Config"
        size="md"
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          <button
            type="button"
            className="btn btn-secondary"
            onClick={() => setMode('copilot')}
            style={{
              padding: '1.25rem',
              display: 'flex',
              alignItems: 'center',
              gap: '0.75rem',
              textAlign: 'left',
              width: '100%',
              fontSize: '1rem'
            }}
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
            </svg>
            <div>
              <strong>GitHub Copilot</strong>
              <div style={{ fontSize: '0.85rem', color: 'var(--color-text-muted)', marginTop: '0.25rem' }}>
                Authenticate via GitHub OAuth and auto-discover available models
              </div>
            </div>
          </button>

          <button
            type="button"
            className="btn btn-secondary"
            onClick={() => setMode('provider')}
            style={{
              padding: '1.25rem',
              display: 'flex',
              alignItems: 'center',
              gap: '0.75rem',
              textAlign: 'left',
              width: '100%',
              fontSize: '1rem'
            }}
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <polygon points="12 2 2 7 12 12 22 7 12 2" />
              <polyline points="2 17 12 22 22 17" />
              <polyline points="2 12 12 17 22 12" />
            </svg>
            <div>
              <strong>Choose Provider</strong>
              <div style={{ fontSize: '0.85rem', color: 'var(--color-text-muted)', marginTop: '0.25rem' }}>
                Select from 25+ pre-configured AI providers (OpenAI, Anthropic, Kimi, etc.)
              </div>
            </div>
          </button>

          <button
            type="button"
            className="btn btn-secondary"
            onClick={() => setMode('custom')}
            style={{
              padding: '1.25rem',
              display: 'flex',
              alignItems: 'center',
              gap: '0.75rem',
              textAlign: 'left',
              width: '100%',
              fontSize: '1rem'
            }}
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
              <path d="M7 11V7a5 5 0 0 1 10 0v4" />
            </svg>
            <div>
              <strong>Custom Provider</strong>
              <div style={{ fontSize: '0.85rem', color: 'var(--color-text-muted)', marginTop: '0.25rem' }}>
                Enter your own API URL and API key for any OpenAI-compatible provider
              </div>
            </div>
          </button>
        </div>
      </Modal>
    )
  }

  // GitHub Copilot OAuth flow
  if (mode === 'copilot' && !config) {
    return (
      <CopilotOAuthModal
        onSuccess={handleCopilotSuccess}
        onCancel={onCancel}
      />
    )
  }

  // Provider selection form
  if (mode === 'provider' && !config) {
    const footer = (
      <>
        <button
          type="button"
          className="btn btn-secondary"
          onClick={() => setMode('choose')}
          disabled={loading}
        >
          Back
        </button>
        <button
          type="submit"
          className="btn btn-primary"
          disabled={loading || !provider || !name || !apiKey}
          form="auth-config-form"
        >
          {loading ? 'Creating...' : 'Create'}
        </button>
      </>
    )

    return (
      <Modal
        isOpen={true}
        onClose={onCancel}
        title="Choose Provider"
        footer={footer}
        size="lg"
      >
        {error && (
          <Alert type="error" onClose={() => setError('')}>
            {error}
          </Alert>
        )}

        <form id="auth-config-form" onSubmit={handleCustomSubmit}>
          <div className="form-group">
            <label htmlFor="name">Name *</label>
            <input
              id="name"
              type="text"
              required
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="My API Provider"
            />
            <small>Human-readable display name</small>
          </div>

          {loadingProviders ? (
            <div style={{ textAlign: 'center', padding: '2rem', color: 'var(--color-text-muted)' }}>
              Loading providers...
            </div>
          ) : (
            <div className="form-group">
              <label>Provider *</label>
              <div style={{
                display: 'flex',
                flexDirection: 'column',
                gap: '1rem',
                maxHeight: '300px',
                overflow: 'auto',
                padding: '0.5rem',
                border: '1px solid var(--color-border)',
                borderRadius: '6px',
                background: 'var(--color-bg-secondary)'
              }}>
                {PROVIDER_CATEGORIES.map(category => {
                  const categoryProviders = providers.filter(p =>
                    category.providers.includes(p.id)
                  )
                  if (categoryProviders.length === 0) return null

                  return (
                    <div key={category.name}>
                      <div style={{
                        fontSize: '0.75rem',
                        fontWeight: 600,
                        textTransform: 'uppercase',
                        color: 'var(--color-text-muted)',
                        marginBottom: '0.5rem',
                        paddingLeft: '0.5rem'
                      }}>
                        {category.name}
                      </div>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                        {categoryProviders.map(p => (
                          <button
                            key={p.id}
                            type="button"
                            onClick={() => setProvider(p.id)}
                            style={{
                              padding: '0.5rem 0.75rem',
                              textAlign: 'left',
                              border: 'none',
                              borderRadius: '4px',
                              background: provider === p.id ? 'var(--color-primary)' : 'transparent',
                              color: provider === p.id ? 'white' : 'var(--color-text)',
                              cursor: 'pointer',
                              display: 'flex',
                              justifyContent: 'space-between',
                              alignItems: 'center'
                            }}
                          >
                            <span>{p.name}</span>
                            {provider === p.id && (
                              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                                <polyline points="20 6 9 17 4 12" />
                              </svg>
                            )}
                          </button>
                        ))}
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          )}

          <div className="form-group">
            <label htmlFor="apiUrl">API URL *</label>
            <input
              id="apiUrl"
              type="url"
              required
              value={apiUrl}
              onChange={e => setApiUrl(e.target.value)}
              placeholder="https://api.example.com/v1"
            />
            <small>Base URL for the API endpoint (auto-filled from provider selection)</small>
          </div>

          <div className="form-group">
            <label htmlFor="apiKey">API Key *</label>
            <input
              id="apiKey"
              type="password"
              required
              value={apiKey}
              onChange={e => setApiKey(e.target.value)}
              placeholder="sk-..."
            />
            <small>Your API key will be encrypted and stored securely</small>
          </div>
        </form>
      </Modal>
    )
  }

  // Custom form (or editing existing)
  const footer = (
    <>
      <button
        type="button"
        className="btn btn-secondary"
        onClick={config ? onCancel : () => setMode('choose')}
        disabled={loading}
      >
        {config ? 'Cancel' : 'Back'}
      </button>
      <button
        type="submit"
        className="btn btn-primary"
        disabled={loading}
        form="auth-config-form"
      >
        {loading ? (config ? 'Updating...' : 'Creating...') : (config ? 'Update' : 'Create')}
      </button>
    </>
  )

  return (
    <Modal
      isOpen={true}
      onClose={onCancel}
      title={config ? 'Edit Auth Config' : 'Custom Provider'}
      footer={footer}
      size="md"
    >
      {error && (
        <Alert type="error" onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      <form id="auth-config-form" onSubmit={handleCustomSubmit}>
        <div className="form-group">
          <label htmlFor="name">Name *</label>
          <input
            id="name"
            type="text"
            required
            value={name}
            onChange={e => setName(e.target.value)}
            placeholder="My API Provider"
          />
          <small>Human-readable display name</small>
        </div>

        <div className="form-group">
          <label htmlFor="apiUrl">API URL *</label>
          <input
            id="apiUrl"
            type="url"
            required
            value={apiUrl}
            onChange={e => setApiUrl(e.target.value)}
            placeholder="https://api.openai.com/v1"
          />
          <small>Base URL for the API endpoint</small>
        </div>

        <div className="form-group">
          <label htmlFor="apiKey">API Key {config ? '' : '*'}</label>
          <input
            id="apiKey"
            type="password"
            required={!config}
            value={apiKey}
            onChange={e => setApiKey(e.target.value)}
            placeholder={config ? '•••••••• (leave blank to keep current)' : 'sk-...'}
          />
          <small>Your API key will be encrypted and stored securely</small>
        </div>
      </form>
    </Modal>
  )
}
