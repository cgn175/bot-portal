import { useState, useEffect, useCallback, useRef } from 'react'
import { api, Model, AuthConfig } from '../api/client'
import Alert from '../components/Alert'

interface Message {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  timestamp: Date
}

export default function TestChat() {
  const [models, setModels] = useState<Model[]>([])
  const [authConfigs, setAuthConfigs] = useState<AuthConfig[]>([])
  const [selectedModel, setSelectedModel] = useState('')
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [isLoadingData, setIsLoadingData] = useState(true)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  useEffect(() => {
    scrollToBottom()
  }, [messages])

  const loadData = useCallback(async () => {
    try {
      setIsLoadingData(true)
      const [modelsData, authData] = await Promise.all([
        api.listModels(),
        api.listAuthConfigs()
      ])
      setModels(modelsData || [])
      setAuthConfigs(authData || [])
      if (modelsData.length > 0 && !selectedModel) {
        setSelectedModel(modelsData[0].id)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load data')
    } finally {
      setIsLoadingData(false)
    }
  }, [selectedModel])

  useEffect(() => {
    loadData()
  }, [loadData])

  const handleSend = async () => {
    if (!input.trim() || !selectedModel || loading) return

    const userMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content: input.trim(),
      timestamp: new Date()
    }

    setMessages(prev => [...prev, userMessage])
    setInput('')
    setLoading(true)
    setError('')

    try {
      const response = await api.sendChatMessage(selectedModel, [
        ...messages.map(m => ({ role: m.role, content: m.content })),
        { role: 'user', content: userMessage.content }
      ])

      if (response.error) {
        throw new Error(response.error.message)
      }

      const assistantMessage = response.choices[0]?.message
      if (assistantMessage) {
        setMessages(prev => [...prev, {
          id: (Date.now() + 1).toString(),
          role: 'assistant',
          content: assistantMessage.content,
          timestamp: new Date()
        }])
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to get response')
    } finally {
      setLoading(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const clearChat = () => {
    setMessages([])
    setError('')
  }

  const selectedModelName = models?.find(m => m.id === selectedModel)?.name || selectedModel

  if (isLoadingData) {
    return (
      <div className="test-chat-page">
        <div className="page-header">
          <h2>Test Chat</h2>
        </div>
        <div className="loading-state">Loading models...</div>
      </div>
    )
  }

  return (
    <div className="test-chat-page" style={{ height: 'calc(100vh - 120px)', display: 'flex', flexDirection: 'column' }}>
      <div className="page-header" style={{ flexShrink: 0 }}>
        <div>
          <h2>Test Chat</h2>
          <p style={{ color: 'var(--color-text-muted)', marginTop: '0.5rem' }}>
            Test your models with a simple chat interface
          </p>
        </div>
        <div style={{ display: 'flex', gap: '1rem', alignItems: 'center' }}>
          <select
            value={selectedModel}
            onChange={(e) => setSelectedModel(e.target.value)}
            className="model-select"
            style={{
              padding: '0.5rem 1rem',
              borderRadius: '6px',
              border: '1px solid var(--color-border)',
              background: 'var(--color-bg-secondary)',
              color: 'var(--color-text)',
              fontSize: '0.9rem'
            }}
          >
            <option value="">Select a model...</option>
            {models.map(model => (
              <option key={model.id} value={model.id}>
                {model.name} ({model.provider})
              </option>
            ))}
          </select>
          <button
            onClick={clearChat}
            className="btn btn-secondary"
            disabled={messages.length === 0}
          >
            Clear Chat
          </button>
        </div>
      </div>

      {authConfigs.length === 0 && (
        <Alert type="warning">
          No auth configs found. Please <a href="#/auth-configs">add an auth config</a> first.
        </Alert>
      )}

      {models.length === 0 && (
        <Alert type="warning">
          No models found. Please <a href="#/models">add a model</a> first.
        </Alert>
      )}

      {error && (
        <Alert type="error" onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      <div
        className="chat-container"
        style={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          background: 'var(--color-bg-secondary)',
          borderRadius: '8px',
          border: '1px solid var(--color-border)',
          overflow: 'hidden',
          marginTop: '1rem'
        }}
      >
        {/* Messages Area */}
        <div
          className="messages-area"
          style={{
            flex: 1,
            overflow: 'auto',
            padding: '1rem',
            display: 'flex',
            flexDirection: 'column',
            gap: '1rem'
          }}
        >
          {messages.length === 0 ? (
            <div
              style={{
                textAlign: 'center',
                color: 'var(--color-text-muted)',
                padding: '3rem',
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                justifyContent: 'center',
                height: '100%'
              }}
            >
              <div style={{ fontSize: '3rem', marginBottom: '1rem' }}>💬</div>
              <h3 style={{ marginBottom: '0.5rem' }}>Start a conversation</h3>
              <p>
                {selectedModel
                  ? `Testing with ${selectedModelName}`
                  : 'Select a model and start chatting'}
              </p>
            </div>
          ) : (
            messages.map((message) => (
              <div
                key={message.id}
                className={`message ${message.role}`}
                style={{
                  alignSelf: message.role === 'user' ? 'flex-end' : 'flex-start',
                  maxWidth: '80%',
                  padding: '0.75rem 1rem',
                  borderRadius: '12px',
                  background: message.role === 'user'
                    ? 'var(--color-primary)'
                    : 'var(--color-bg-tertiary)',
                  color: message.role === 'user'
                    ? 'white'
                    : 'var(--color-text)',
                  wordBreak: 'break-word'
                }}
              >
                <div style={{ fontSize: '0.75rem', opacity: 0.7, marginBottom: '0.25rem' }}>
                  {message.role === 'user' ? 'You' : selectedModelName}
                </div>
                <div style={{ whiteSpace: 'pre-wrap' }}>{message.content}</div>
              </div>
            ))
          )}
          <div ref={messagesEndRef} />
        </div>

        {/* Input Area */}
        <div
          className="input-area"
          style={{
            padding: '1rem',
            borderTop: '1px solid var(--color-border)',
            background: 'var(--color-bg)',
            display: 'flex',
            gap: '0.5rem'
          }}
        >
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={selectedModel ? "Type your message..." : "Select a model first"}
            disabled={!selectedModel || loading}
            style={{
              flex: 1,
              padding: '0.75rem',
              borderRadius: '8px',
              border: '1px solid var(--color-border)',
              background: 'var(--color-bg-secondary)',
              color: 'var(--color-text)',
              resize: 'none',
              minHeight: '60px',
              maxHeight: '150px',
              fontFamily: 'inherit',
              fontSize: '0.95rem'
            }}
          />
          <button
            onClick={handleSend}
            disabled={!input.trim() || !selectedModel || loading}
            className="btn btn-primary"
            style={{
              alignSelf: 'flex-end',
              padding: '0.75rem 1.5rem',
              minWidth: '80px'
            }}
          >
            {loading ? '...' : 'Send'}
          </button>
        </div>
      </div>

      <div
        className="chat-footer"
        style={{
          marginTop: '0.5rem',
          padding: '0.5rem',
          textAlign: 'center',
          color: 'var(--color-text-muted)',
          fontSize: '0.8rem'
        }}
      >
        Press Enter to send, Shift+Enter for new line
      </div>
    </div>
  )
}
