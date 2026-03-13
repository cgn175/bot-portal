import { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import AnsiToHtml from 'ansi-to-html'

interface LogEntry {
  timestamp: string
  stream: 'stdout' | 'stderr'
  line: string
}

interface LiveLogViewerProps {
  agentId: string
  containerId?: string
}

type ConnectionStatus = 'connecting' | 'live' | 'paused' | 'reconnecting' | 'error' | 'closed'

const MAX_RECONNECT_ATTEMPTS = 5
const RECONNECT_DELAYS = [1000, 2000, 4000, 8000, 16000]
const TAIL_OPTIONS = [50, 100, 500, 1000, 5000]

// Fetch-based EventSource polyfill that supports Authorization header
class FetchEventSource {
  private url: string
  private abortController: AbortController
  private onMessage: (data: string) => void
  private onError: (error: Error) => void
  private onOpen: () => void
  private isClosed = false

  constructor(
    url: string,
    options: {
      onMessage: (data: string) => void
      onError: (error: Error) => void
      onOpen: () => void
    }
  ) {
    this.url = url
    this.abortController = new AbortController()
    this.onMessage = options.onMessage
    this.onError = options.onError
    this.onOpen = options.onOpen
  }

  async connect() {
    try {
      const response = await fetch(this.url, {
        method: 'GET',
        headers: {
          Accept: 'text/event-stream',
        },
        signal: this.abortController.signal,
      })

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`)
      }

      if (!response.body) {
        throw new Error('No response body')
      }

      this.onOpen()

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (!this.isClosed) {
        const { done, value } = await reader.read()

        if (done) {
          break
        }

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const data = line.slice(6)
            if (data !== '[DONE]') {
              this.onMessage(data)
            }
          }
        }
      }
    } catch (error) {
      if (error instanceof Error && error.name !== 'AbortError') {
        this.onError(error)
      }
    }
  }

  close() {
    this.isClosed = true
    this.abortController.abort()
  }
}

export default function LiveLogViewer({ agentId, containerId }: LiveLogViewerProps) {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [isPaused, setIsPaused] = useState(false)
  const [autoScroll, setAutoScroll] = useState(true)
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('connecting')
  const [reconnectAttempt, setReconnectAttempt] = useState(0)
  const [tail, setTail] = useState(100)
  const [error, setError] = useState<string | null>(null)

  const ansi = useMemo(() => new AnsiToHtml({ fg: '#e6edf3', bg: '#0d1117', escapeXML: true }), [])

  const logsContainerRef = useRef<HTMLDivElement>(null)
  const fetchESRef = useRef<FetchEventSource | null>(null)
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const pendingLogsRef = useRef<LogEntry[]>([])

  const targetContainerId = containerId || agentId

  // Format timestamp for display
  const formatTimestamp = useCallback((timestamp: string): string => {
    try {
      const date = new Date(timestamp)
      return date.toLocaleTimeString('en-US', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false,
      })
    } catch {
      return timestamp
    }
  }, [])

  // Scroll to bottom if autoScroll is enabled
  const scrollToBottom = useCallback(() => {
    if (autoScroll && logsContainerRef.current) {
      logsContainerRef.current.scrollTop = logsContainerRef.current.scrollHeight
    }
  }, [autoScroll])

  // Process pending logs when unpaused
  useEffect(() => {
    if (!isPaused && pendingLogsRef.current.length > 0) {
      setLogs((prev) => {
        const newLogs = [...prev, ...pendingLogsRef.current]
        pendingLogsRef.current = []
        // Keep only the last 'tail' number of logs
        if (newLogs.length > tail) {
          return newLogs.slice(newLogs.length - tail)
        }
        return newLogs
      })
      scrollToBottom()
    }
  }, [isPaused, tail, scrollToBottom])

  // Auto-scroll when logs change
  useEffect(() => {
    scrollToBottom()
  }, [logs, scrollToBottom])

  // Connect to SSE endpoint
  const connect = useCallback(() => {
    if (fetchESRef.current) {
      fetchESRef.current.close()
    }

    setConnectionStatus('connecting')
    setError(null)

    const url = `/api/agents/container-logs-stream?agentID=${encodeURIComponent(agentId)}&tail=${tail}`

    fetchESRef.current = new FetchEventSource(url, {
      onMessage: (data) => {
        try {
          const entry: LogEntry = JSON.parse(data)

          if (isPaused) {
            pendingLogsRef.current.push(entry)
            // Prevent unbounded growth of pending logs
            if (pendingLogsRef.current.length > tail) {
              pendingLogsRef.current = pendingLogsRef.current.slice(pendingLogsRef.current.length - tail)
            }
          } else {
            setLogs((prev) => {
              const newLogs = [...prev, entry]
              if (newLogs.length > tail) {
                return newLogs.slice(newLogs.length - tail)
              }
              return newLogs
            })
          }
        } catch (e) {
          console.error('Failed to parse log entry:', e)
        }
      },
      onError: (err) => {
        console.error('SSE error:', err)
        setError(err.message)
        setConnectionStatus('error')

        // Start reconnection logic
        if (reconnectAttempt < MAX_RECONNECT_ATTEMPTS) {
          const delay = RECONNECT_DELAYS[reconnectAttempt] || RECONNECT_DELAYS[RECONNECT_DELAYS.length - 1]
          setConnectionStatus('reconnecting')
          setReconnectAttempt((prev) => prev + 1)

          reconnectTimeoutRef.current = setTimeout(() => {
            connect()
          }, delay)
        } else {
          setConnectionStatus('error')
        }
      },
      onOpen: () => {
        setConnectionStatus('live')
        setReconnectAttempt(0)
        setError(null)
      },
    })

    fetchESRef.current.connect()
  }, [agentId, targetContainerId, tail, isPaused, reconnectAttempt])

  // Initial connection and cleanup
  useEffect(() => {
    connect()

    return () => {
      if (fetchESRef.current) {
        fetchESRef.current.close()
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
    }
  }, [connect])

  // Reconnect when tail changes
  useEffect(() => {
    setLogs([])
    pendingLogsRef.current = []
    connect()
  }, [tail, connect])

  // Handle pause/resume
  const handlePauseToggle = () => {
    setIsPaused((prev) => !prev)
    if (isPaused) {
      setConnectionStatus('live')
    } else {
      setConnectionStatus('paused')
    }
  }

  // Handle auto-scroll toggle
  const handleAutoScrollToggle = () => {
    setAutoScroll((prev) => !prev)
  }

  // Handle tail size change
  const handleTailChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    setTail(Number(e.target.value))
  }

  // Copy all logs to clipboard
  const handleCopyAll = async () => {
    const text = logs
      .map((log) => `[${formatTimestamp(log.timestamp)}] [${log.stream}] ${log.line}`)
      .join('\n')

    try {
      await navigator.clipboard.writeText(text)
    } catch (err) {
      console.error('Failed to copy logs:', err)
    }
  }

  // Get status indicator color
  const getStatusColor = () => {
    switch (connectionStatus) {
      case 'live':
        return '#10b981' // success
      case 'paused':
        return '#f59e0b' // warning
      case 'connecting':
      case 'reconnecting':
        return '#3b82f6' // info
      case 'error':
        return '#ef4444' // error
      case 'closed':
        return '#6b7280' // muted
      default:
        return '#6b7280'
    }
  }

  // Get status text
  const getStatusText = () => {
    switch (connectionStatus) {
      case 'live':
        return 'Live'
      case 'paused':
        return 'Paused'
      case 'connecting':
        return 'Connecting...'
      case 'reconnecting':
        return `Reconnecting (${reconnectAttempt}/${MAX_RECONNECT_ATTEMPTS})...`
      case 'error':
        return 'Error'
      case 'closed':
        return 'Closed'
      default:
        return connectionStatus
    }
  }

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        border: '1px solid var(--color-border)',
        borderRadius: 'var(--radius-md)',
        overflow: 'hidden',
      }}
    >
      {/* Header */}
      <div
        style={{
          padding: '0.75rem 1rem',
          borderBottom: '1px solid var(--color-border)',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          background: 'var(--color-bg-secondary)',
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '0.5rem',
          }}
        >
          <span
            style={{
              width: 8,
              height: 8,
              borderRadius: '50%',
              backgroundColor: getStatusColor(),
              boxShadow: connectionStatus === 'live' ? `0 0 8px ${getStatusColor()}` : 'none',
              transition: 'all 0.3s ease',
            }}
          />
          <span
            style={{
              fontSize: 'var(--font-size-sm)',
              color: 'var(--color-text-muted)',
            }}
          >
            {getStatusText()}
          </span>
          {logs.length > 0 && (
            <span
              style={{
                fontSize: 'var(--font-size-xs)',
                color: 'var(--color-text-subtle)',
                marginLeft: '0.5rem',
              }}
            >
              ({logs.length.toLocaleString()} lines)
            </span>
          )}
        </div>
      </div>

      {/* Warning Banner */}
      <div
        style={{
          background: 'var(--color-warning-subtle)',
          borderBottom: '1px solid rgba(245, 158, 11, 0.2)',
          padding: '0.75rem 1.5rem',
          display: 'flex',
          alignItems: 'center',
          gap: '0.5rem',
          fontSize: 'var(--font-size-sm)',
          color: 'var(--color-warning)',
        }}
      >
        <span>⚠️</span>
        <span>
          Warning: Logs may contain sensitive information such as API keys, tokens, or passwords.
          Use caution when sharing.
        </span>
      </div>

      {/* Controls Toolbar */}
      <div
        style={{
          padding: '0.75rem 1.5rem',
          borderBottom: '1px solid var(--color-border)',
          display: 'flex',
          flexWrap: 'wrap',
          gap: '0.5rem',
          alignItems: 'center',
          background: 'var(--color-bg-tertiary)',
        }}
      >
        <button
          onClick={handlePauseToggle}
          className={`btn ${isPaused ? 'btn-primary' : 'btn-secondary'}`}
          style={{ fontSize: 'var(--font-size-sm)', padding: '0.5rem 0.75rem' }}
        >
          {isPaused ? '▶ Resume' : '⏸ Pause'}
        </button>

        <button
          onClick={handleAutoScrollToggle}
          className={`btn ${autoScroll ? 'btn-primary' : 'btn-secondary'}`}
          style={{ fontSize: 'var(--font-size-sm)', padding: '0.5rem 0.75rem' }}
        >
          {autoScroll ? '⬇ Auto-scroll ON' : '⬇ Auto-scroll OFF'}
        </button>

        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <label
            htmlFor="tail-select"
            style={{
              fontSize: 'var(--font-size-sm)',
              color: 'var(--color-text-muted)',
            }}
          >
            Tail:
          </label>
          <select
            id="tail-select"
            value={tail}
            onChange={handleTailChange}
            style={{
              padding: '0.5rem',
              borderRadius: 'var(--radius-md)',
              border: '1px solid var(--color-border)',
              background: 'var(--color-bg)',
              color: 'var(--color-text)',
              fontSize: 'var(--font-size-sm)',
            }}
          >
            {TAIL_OPTIONS.map((option) => (
              <option key={option} value={option}>
                {option} lines
              </option>
            ))}
          </select>
        </div>

        <div style={{ flex: 1 }} />

        <button
          onClick={handleCopyAll}
          className="btn btn-secondary"
          style={{ fontSize: 'var(--font-size-sm)', padding: '0.5rem 0.75rem' }}
          disabled={logs.length === 0}
        >
          📋 Copy All
        </button>
      </div>

      {/* Log Container */}
      <div
        ref={logsContainerRef}
        style={{
          height: '500px',
          overflowY: 'auto',
          padding: '1rem',
          background: '#0d1117',
          fontFamily: '"JetBrains Mono", "Fira Code", "Cascadia Code", monospace',
          fontSize: '12px',
          lineHeight: '1.5',
        }}
      >
        {logs.length === 0 && connectionStatus === 'connecting' && (
          <div
            style={{
              textAlign: 'center',
              color: 'var(--color-text-muted)',
              padding: '3rem 0',
            }}
          >
            <div className="loading-pulse" style={{ justifyContent: 'center' }}>
              <span />
              <span />
              <span />
            </div>
            <p style={{ marginTop: '1rem' }}>Connecting to log stream...</p>
          </div>
        )}

        {logs.length === 0 && connectionStatus === 'error' && (
          <div
            style={{
              textAlign: 'center',
              color: 'var(--color-error)',
              padding: '3rem 0',
            }}
          >
            <p>❌ Failed to connect to log stream</p>
            {error && (
              <p style={{ fontSize: 'var(--font-size-xs)', marginTop: '0.5rem', opacity: 0.8 }}>
                {error}
              </p>
            )}
          </div>
        )}

        {logs.map((log, index) => (
          <div
            key={index}
            style={{
              display: 'flex',
              gap: '0.75rem',
              padding: '0.125rem 0',
              color: log.stream === 'stderr' ? '#f85149' : '#e6edf3',
            }}
          >
            <span
              style={{
                color: 'var(--color-text-subtle)',
                userSelect: 'none',
                minWidth: '60px',
              }}
            >
              {formatTimestamp(log.timestamp)}
            </span>
            <span
              style={{
                color: log.stream === 'stderr' ? '#f85149' : '#3fb950',
                userSelect: 'none',
                minWidth: '30px',
                textTransform: 'uppercase',
                fontSize: '10px',
                display: 'flex',
                alignItems: 'center',
              }}
            >
              {log.stream}
            </span>
            <span
              style={{
                flex: 1,
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-all',
              }}
              dangerouslySetInnerHTML={{ __html: ansi.toHtml(log.line) }}
            />
          </div>
        ))}

        {isPaused && pendingLogsRef.current.length > 0 && (
          <div
            style={{
              textAlign: 'center',
              padding: '0.5rem',
              marginTop: '0.5rem',
              background: 'var(--color-warning-subtle)',
              borderRadius: 'var(--radius-md)',
              color: 'var(--color-warning)',
              fontSize: 'var(--font-size-xs)',
            }}
          >
            ⏸ {pendingLogsRef.current.length.toLocaleString()} new lines buffered (resume to view)
          </div>
        )}
      </div>

      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.5; }
        }
      `}</style>
    </div>
  )
}
