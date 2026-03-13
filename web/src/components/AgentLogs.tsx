import { useState, useCallback, useRef, useEffect } from 'react'
import { api } from '../api/client'

interface AgentLogsProps {
  agentId: string
}

export default function AgentLogs({ agentId }: AgentLogsProps) {
  const [logs, setLogs] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [expanded, setExpanded] = useState(false)
  const logsEndRef = useRef<HTMLDivElement>(null)

  const fetchLogs = useCallback(async () => {
    try {
      setLoading(true)
      setError('')
      const data = await api.getAgentLogs(agentId)
      setLogs(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch logs')
    } finally {
      setLoading(false)
    }
  }, [agentId])

  useEffect(() => {
    if (expanded && logs === null) {
      fetchLogs()
    }
  }, [expanded, logs, fetchLogs])

  useEffect(() => {
    if (logs && logsEndRef.current) {
      logsEndRef.current.scrollTop = logsEndRef.current.scrollHeight
    }
  }, [logs])

  return (
    <div className="card" style={{ marginBottom: '1.5rem' }}>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          cursor: 'pointer',
        }}
        onClick={() => setExpanded(!expanded)}
      >
        <h3 style={{
          fontSize: 'var(--font-size-xl)',
          fontWeight: 'var(--font-weight-semibold)',
          margin: 0,
        }}>
          {expanded ? '▾' : '▸'} Container Logs
        </h3>
        {expanded && (
          <button
            className="btn btn-secondary"
            style={{ fontSize: 'var(--font-size-sm)' }}
            onClick={(e) => {
              e.stopPropagation()
              fetchLogs()
            }}
            disabled={loading}
          >
            {loading ? '⏳ Loading...' : '↻ Refresh'}
          </button>
        )}
      </div>

      {expanded && (
        <div style={{ marginTop: '1rem' }}>
          {error && (
            <div style={{
              color: 'var(--color-error, #ef4444)',
              marginBottom: '0.5rem',
              fontSize: 'var(--font-size-sm)',
            }}>
              {error}
            </div>
          )}
          <div
            ref={logsEndRef}
            style={{
              background: '#1a1a2e',
              color: '#e0e0e0',
              fontFamily: '"JetBrains Mono", "Fira Code", "Cascadia Code", monospace',
              fontSize: '12px',
              lineHeight: '1.5',
              padding: '1rem',
              borderRadius: 'var(--radius-md)',
              maxHeight: '500px',
              overflowY: 'auto',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-all',
            }}
          >
            {loading && logs === null
              ? 'Loading logs...'
              : logs
                ? logs
                : 'No logs available.'}
          </div>
        </div>
      )}
    </div>
  )
}
