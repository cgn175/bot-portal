import { useState, useEffect, useCallback } from 'react'
import { api, CreateAuthConfigRequest, AuthConfig } from '../api/client'
import Modal from '../components/Modal'
import Alert from '../components/Alert'
import CopilotOAuthModal from '../components/CopilotOAuthModal'

interface AuthConfigFormProps {
  config?: AuthConfig
  onSuccess: () => void
  onCancel: () => void
}

const PROVIDERS = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'copilot', label: 'Copilot' },
  { value: 'ollama', label: 'Ollama' },
  { value: 'openrouter', label: 'OpenRouter' }
]

const AUTH_TYPES = [
  { value: 'bearer_token', label: 'Bearer Token' },
  { value: 'basic_auth', label: 'Basic Auth' },
  { value: 'github_copilot_oauth', label: 'GitHub Copilot OAuth' }
]

export default function AuthConfigForm({ config, onSuccess, onCancel }: AuthConfigFormProps) {
  const [formData, setFormData] = useState<CreateAuthConfigRequest>({
    id: '',
    name: '',
    provider: 'openai',
    authType: 'bearer_token',
    credentials: {},
    endpointUrl: ''
  })
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [showCopilotModal, setShowCopilotModal] = useState(false)

  useEffect(() => {
    if (config) {
      // Parse credentials from JSON string
      let credentials: Record<string, string> = {}
      try {
        if (config.credentials) {
          credentials = JSON.parse(config.credentials)
          // Replace masked values with empty strings for editing
          Object.keys(credentials).forEach(key => {
            if (credentials[key] === '***masked***') {
              credentials[key] = ''
            }
          })
        }
      } catch {
        credentials = {}
      }

      setFormData({
        id: config.id,
        name: config.name,
        provider: config.provider,
        authType: config.authType,
        credentials,
        endpointUrl: config.endpointUrl || ''
      })
    }
  }, [config])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      // Filter out empty credentials for update
      const filteredCredentials: Record<string, string> = {}
      Object.entries(formData.credentials).forEach(([key, value]) => {
        if (value.trim()) {
          filteredCredentials[key] = value
        }
      })

      const submitData: CreateAuthConfigRequest = {
        ...formData,
        credentials: filteredCredentials
      }

      if (config) {
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

  const handleCopilotSuccess = useCallback(() => {
    // The token has been saved server-side in AuthConfig
    // We don't receive the token itself anymore for security reasons
    // Close the modal and let the user save the form
    setShowCopilotModal(false)
    // Optionally show a success message that the OAuth flow completed
    alert('GitHub Copilot authentication successful! Click Save to store the configuration.')
  }, [])

  const updateCredential = (key: string, value: string) => {
    setFormData(prev => ({
      ...prev,
      credentials: { ...prev.credentials, [key]: value }
    }))
  }

  const footer = (
    <>
      <button
        type="button"
        className="btn btn-secondary"
        onClick={onCancel}
        disabled={loading}
      >
        Cancel
      </button>
      <button
        type="submit"
        className="btn btn-primary"
        disabled={loading}
        form="auth-config-form"
      >
        {loading ? (config ? 'Updating...' : 'Creating...') : (config ? 'Update Auth Config' : 'Create Auth Config')}
      </button>
    </>
  )

  return (
    <>
      <Modal
        isOpen={true}
        onClose={onCancel}
        title={config ? 'Edit Auth Config' : 'Add New Auth Config'}
        footer={footer}
        size="lg"
      >
        {error && (
          <Alert type="error" onClose={() => setError('')}>
            {error}
          </Alert>
        )}

        <form id="auth-config-form" onSubmit={handleSubmit}>
          <div className="form-group">
            <label htmlFor="id">Config ID *</label>
            <input
              id="id"
              type="text"
              required
              value={formData.id}
              onChange={e => setFormData({ ...formData, id: e.target.value })}
              placeholder="openai-api-key"
              disabled={!!config}
              className={error ? 'error' : ''}
            />
            <small>
              {config ? 'Config ID cannot be changed' : 'Unique identifier (lowercase, no spaces)'}
            </small>
          </div>

          <div className="form-group">
            <label htmlFor="name">Name *</label>
            <input
              id="name"
              type="text"
              required
              value={formData.name}
              onChange={e => setFormData({ ...formData, name: e.target.value })}
              placeholder="OpenAI API Key"
            />
            <small>Human-readable display name</small>
          </div>

          <div className="form-group">
            <label htmlFor="provider">Provider *</label>
            <select
              id="provider"
              required
              value={formData.provider}
              onChange={e => setFormData({ ...formData, provider: e.target.value })}
            >
              {PROVIDERS.map(p => (
                <option key={p.value} value={p.value}>{p.label}</option>
              ))}
            </select>
            <small>Select the AI provider this auth config is for</small>
          </div>

          <div className="form-group">
            <label htmlFor="authType">Auth Type *</label>
            <select
              id="authType"
              required
              value={formData.authType}
              onChange={e => {
                const newAuthType = e.target.value as CreateAuthConfigRequest['authType']
                setFormData({
                  ...formData,
                  authType: newAuthType,
                  credentials: {} // Reset credentials when auth type changes
                })
              }}
            >
              {AUTH_TYPES.map(t => (
                <option key={t.value} value={t.value}>{t.label}</option>
              ))}
            </select>
            <small>Select the authentication method</small>
          </div>

          <div className="form-group">
            <label htmlFor="endpointUrl">Endpoint URL (Optional)</label>
            <input
              id="endpointUrl"
              type="text"
              value={formData.endpointUrl}
              onChange={e => setFormData({ ...formData, endpointUrl: e.target.value })}
              placeholder="https://api.openai.com/v1"
            />
            <small>Override the default authentication endpoint</small>
          </div>

          {/* Dynamic credential fields based on auth type */}
          <div className="form-group">
            <label>Credentials</label>
            <CredentialFields
              authType={formData.authType}
              credentials={formData.credentials}
              onChange={updateCredential}
              onCopilotClick={() => setShowCopilotModal(true)}
              isEditing={!!config}
            />
          </div>
        </form>
      </Modal>

      {showCopilotModal && (
        <CopilotOAuthModal
          onSuccess={handleCopilotSuccess}
          onCancel={() => setShowCopilotModal(false)}
        />
      )}
    </>
  )
}

interface CredentialFieldsProps {
  authType: CreateAuthConfigRequest['authType']
  credentials: Record<string, string>
  onChange: (key: string, value: string) => void
  onCopilotClick: () => void
  isEditing: boolean
}

function CredentialFields({ authType, credentials, onChange, onCopilotClick, isEditing }: CredentialFieldsProps) {
  switch (authType) {
    case 'bearer_token':
      return (
        <div className="credential-field">
          <input
            type="password"
            placeholder={isEditing ? '•••••••• (leave blank to keep current)' : 'Enter API Key'}
            value={credentials.api_key || ''}
            onChange={e => onChange('api_key', e.target.value)}
            style={{ marginBottom: '0.5rem' }}
          />
          <small>Your API key will be encrypted and stored securely</small>
        </div>
      )

    case 'basic_auth':
      return (
        <div className="credential-fields-stack">
          <input
            type="text"
            placeholder="Username"
            value={credentials.username || ''}
            onChange={e => onChange('username', e.target.value)}
            style={{ marginBottom: '0.5rem' }}
          />
          <input
            type="password"
            placeholder={isEditing ? 'Password (leave blank to keep current)' : 'Password'}
            value={credentials.password || ''}
            onChange={e => onChange('password', e.target.value)}
          />
          <small>Username and password will be encrypted and stored securely</small>
        </div>
      )

    case 'github_copilot_oauth':
      return (
        <div className="credential-field">
          {credentials.access_token ? (
            <div style={{
              padding: '0.75rem 1rem',
              background: 'var(--color-success-subtle)',
              border: '1px solid var(--color-success)',
              borderRadius: 'var(--radius-md)',
              color: 'var(--color-success)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: '1rem'
            }}>
              <span>✓ Authenticated with GitHub Copilot</span>
              <button
                type="button"
                className="btn btn-sm btn-secondary"
                onClick={onCopilotClick}
              >
                Re-authenticate
              </button>
            </div>
          ) : (
            <button
              type="button"
              className="btn btn-primary"
              onClick={onCopilotClick}
              style={{ width: '100%' }}
            >
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="currentColor"
                style={{ marginRight: '0.5rem' }}
              >
                <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/>
              </svg>
              Authenticate with GitHub
            </button>
          )}
          <small>Click to start the GitHub device flow authentication</small>
        </div>
      )

    default:
      return null
  }
}
