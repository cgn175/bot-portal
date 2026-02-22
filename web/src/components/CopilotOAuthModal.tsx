import { useState, useEffect, useCallback, useRef } from 'react'
import Modal from './Modal'
import Alert from './Alert'
import { api } from '../api/client'

interface CopilotOAuthModalProps {
  onSuccess: (configId?: string) => void
  onCancel: () => void}

export default function CopilotOAuthModal({ onSuccess, onCancel }: CopilotOAuthModalProps) {
  const [deviceCode, setDeviceCode] = useState('')
  const [userCode, setUserCode] = useState('')
  const [verificationUri, setVerificationUri] = useState('')
  const [pollInterval, setPollInterval] = useState(5)
  const [error, setError] = useState('')
  const [status, setStatus] = useState<'init' | 'polling' | 'success' | 'error'>('init')
  const [timeLeft, setTimeLeft] = useState(0)
  const pollingRef = useRef<number | null>(null)
  const countdownRef = useRef<number | null>(null)

  // Initialize device flow
  useEffect(() => {
    const initDeviceFlow = async () => {
      try {
        setStatus('init')
        const response = await api.initiateCopilotDeviceFlow()
        setDeviceCode(response.device_code)
        setUserCode(response.user_code)
        setVerificationUri(response.verification_uri)
        setPollInterval(response.interval)
        setTimeLeft(response.expires_in)
        setStatus('polling')
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to initiate device flow')
        setStatus('error')
      }
    }

    initDeviceFlow()

    // Cleanup on unmount
    return () => {
      if (pollingRef.current) {
        window.clearTimeout(pollingRef.current)
      }
      if (countdownRef.current) {
        window.clearInterval(countdownRef.current)
      }
    }
  }, [])

  // Countdown timer
  useEffect(() => {
    if (status === 'polling' && timeLeft > 0) {
      countdownRef.current = window.setInterval(() => {
        setTimeLeft(currentTime => {
          if (currentTime <= 1) {
            setError('Authentication timed out. Please try again.')
            setStatus('error')
            return 0
          }
          return currentTime - 1
        })
      }, 1000)
    }

    return () => {
      if (countdownRef.current) {
        window.clearInterval(countdownRef.current)
      }
    }
  }, [status, timeLeft])

  // Poll for token
  const pollForToken = useCallback(async () => {
    if (!deviceCode || status !== 'polling') return

    try {
      const response = await api.pollCopilotToken(deviceCode)

      if (response) {
        // Token received and saved server-side!
        setStatus('success')
        onSuccess(response.configId)
        return
      }

      // Still waiting, poll again after interval
      pollingRef.current = window.setTimeout(pollForToken, pollInterval * 1000)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to get token')
      setStatus('error')
    }
  }, [deviceCode, status, pollInterval, onSuccess])

  // Start polling when we have device code
  useEffect(() => {
    if (deviceCode && status === 'polling') {
      pollingRef.current = window.setTimeout(pollForToken, pollInterval * 1000)
    }

    return () => {
      if (pollingRef.current) {
        window.clearTimeout(pollingRef.current)
      }
    }
  }, [deviceCode, status, pollInterval, pollForToken])

  const handleCopyCode = () => {
    navigator.clipboard.writeText(userCode)
  }

  const handleOpenGitHub = () => {
    window.open(verificationUri, '_blank')
  }

  const formatTime = (seconds: number) => {
    const mins = Math.floor(seconds / 60)
    const secs = seconds % 60
    return `${mins}:${secs.toString().padStart(2, '0')}`
  }

  const footer = (
    <>
      <button
        type="button"
        className="btn btn-secondary"
        onClick={onCancel}
      >
        Cancel
      </button>
    </>
  )

  return (
    <Modal
      isOpen={true}
      onClose={onCancel}
      title="Authenticate with GitHub"
      footer={footer}
      size="md"
    >
      {error && (
        <Alert type="error" onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      {status === 'init' && (
        <div className="loading" style={{ padding: '2rem' }}>
          <div className="loading-spinner" />
          <p>Initializing authentication...</p>
        </div>
      )}

      {status === 'polling' && (
        <div style={{ textAlign: 'center' }}>
          <div style={{ marginBottom: '1.5rem' }}>
            <svg
              width="48"
              height="48"
              viewBox="0 0 24 24"
              fill="currentColor"
              style={{ color: 'var(--color-text-muted)' }}
            >
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
            </svg>
          </div>

          <p style={{ marginBottom: '1rem', color: 'var(--color-text-secondary)' }}>
            Go to <strong>github.com/login/device</strong> and enter this code:
          </p>

          <div
            style={{
              background: 'var(--color-bg)',
              border: '2px solid var(--color-border)',
              borderRadius: 'var(--radius-lg)',
              padding: '1.5rem',
              marginBottom: '1.5rem',
              display: 'flex',
              flexDirection: 'column',
              gap: '1rem'
            }}
          >
            <code
              style={{
                fontSize: '2rem',
                fontWeight: 'bold',
                letterSpacing: '0.1em',
                color: 'var(--color-text)',
                userSelect: 'all'
              }}
            >
              {userCode}
            </code>

            <button
              type="button"
              className="btn btn-secondary"
              onClick={handleCopyCode}
              style={{ alignSelf: 'center' }}
            >
              Copy Code
            </button>
          </div>

          <button
            type="button"
            className="btn btn-primary"
            onClick={handleOpenGitHub}
            style={{ width: '100%', marginBottom: '1rem' }}
          >
            Open github.com/login/device
          </button>

          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '0.5rem',
              color: 'var(--color-text-muted)',
              fontSize: 'var(--font-size-sm)'
            }}
          >
            <div className="loading-pulse">
              <span />
              <span />
              <span />
            </div>
            <span>Waiting for authentication...</span>
            <span style={{ fontFamily: 'monospace' }}>({formatTime(timeLeft)})</span>
          </div>
        </div>
      )}

      {status === 'success' && (
        <div style={{ textAlign: 'center', padding: '1rem' }}>
          <div
            style={{
              width: '64px',
              height: '64px',
              borderRadius: '50%',
              background: 'var(--color-success-subtle)',
              color: 'var(--color-success)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              margin: '0 auto 1rem',
              fontSize: '2rem'
            }}
          >
            ✓
          </div>
          <h3 style={{ color: 'var(--color-success)', marginBottom: '0.5rem' }}>
            Authentication Successful!
          </h3>
          <p style={{ color: 'var(--color-text-muted)' }}>
            Your GitHub Copilot token has been saved.
          </p>
        </div>
      )}
    </Modal>
  )
}
