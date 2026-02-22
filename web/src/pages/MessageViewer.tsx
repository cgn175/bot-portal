import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { api, Channel, TaskLog } from '../api/client'
import Alert from '../components/Alert'
import StatusBadge from '../components/StatusBadge'
import { LoadingState } from '../components/LoadingState'
import EmptyState from '../components/EmptyState'

export default function MessageViewer() {
  const [channels, setChannels] = useState<Channel[]>([])
  const [selectedChannel, setSelectedChannel] = useState<string>('')
  const [messages, setMessages] = useState<TaskLog[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    const loadChannels = async () => {
      try {
        const data = await api.listChannels()
        setChannels(data || [])
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load channels')
      }
    }
    loadChannels()
  }, [])

  useEffect(() => {
    if (!selectedChannel) {
      setMessages([])
      setLoading(false)
      return
    }

    const loadMessages = async () => {
      try {
        setLoading(true)
        setError('')
        const data = await api.getChannelMessages(selectedChannel)
        setMessages(data || [])
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load messages')
      } finally {
        setLoading(false)
      }
    }

    loadMessages()

    // Use SSE for real-time message updates
    const eventSource = api.streamMessages(selectedChannel)

    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        setMessages(data || [])
        setLoading(false)
      } catch (err) {
        console.error('Failed to parse SSE data:', err)
      }
    }

    eventSource.onerror = () => {
      eventSource.close()
      // Fallback to polling if SSE fails
      const interval = setInterval(loadMessages, 10000)
      return () => clearInterval(interval)
    }

    return () => eventSource.close()
  }, [selectedChannel])

  return (
    <div>
      <div className="page-header">
        <div>
          <h2>Messages</h2>
          <p style={{ color: 'var(--color-text-muted)', marginTop: '0.5rem' }}>
            View and monitor message flows between agents
          </p>
        </div>
      </div>

      {error && (
        <Alert type="error" onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      <div className="card" style={{ marginBottom: '1.5rem' }}>
        <div className="form-group" style={{ marginBottom: 0 }}>
          <label htmlFor="channel-select">Filter by Channel</label>
          <select
            id="channel-select"
            value={selectedChannel}
            onChange={e => setSelectedChannel(e.target.value)}
          >
            <option value="">Select a channel...</option>
            {channels.map(channel => (
              <option key={channel.id} value={channel.id}>
                {channel.id} ({channel.members.join(', ')})
              </option>
            ))}
          </select>
        </div>
      </div>

      {loading && selectedChannel && <LoadingState message="Loading messages..." />}

      {!loading && !selectedChannel && (
        <EmptyState
          icon="📨"
          title="Select a channel"
          description="Choose a channel from the dropdown above to view messages."
        />
      )}

      {!loading && selectedChannel && messages.length === 0 && (
        <EmptyState
          icon="📭"
          title="No messages yet"
          description="This channel doesn't have any messages yet. Messages will appear here when agents communicate."
        />
      )}

      {!loading && messages.length > 0 && (
        <div className="grid" style={{ gap: '1rem' }}>
          {messages.map((msg, index) => (
            <MessageCard key={msg.id} message={msg} index={index} />
          ))}
        </div>
      )}
    </div>
  )
}

interface MessageCardProps {
  message: TaskLog
  index: number
}

function MessageCard({ message, index }: MessageCardProps) {
  return (
    <div
      className="card animate-fade-in"
      style={{ animationDelay: `${index * 0.05}s` }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '0.75rem' }}>
        <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
          <StatusBadge status={message.status} />
          <StatusBadge status={message.direction} />
        </div>
        <div style={{ fontSize: 'var(--font-size-xs)', color: 'var(--color-text-muted)' }}>
          {new Date(message.created_at).toLocaleString()}
        </div>
      </div>

      <div
        style={{
          fontSize: 'var(--font-size-sm)',
          marginBottom: '0.75rem',
          display: 'flex',
          alignItems: 'center',
          gap: '0.5rem',
          flexWrap: 'wrap'
        }}
      >
        <code>{message.sender_id}</code>
        <span style={{ color: 'var(--color-text-muted)' }}>→</span>
        <code>{message.recipient_id || 'broadcast'}</code>
      </div>

      {message.messages && message.messages.length > 0 && (
        <div
          style={{
            background: 'var(--color-bg)',
            padding: '1rem',
            borderRadius: 'var(--radius-md)',
            fontSize: 'var(--font-size-sm)',
            border: '1px solid var(--color-border)'
          }}
        >
          {message.messages.map((m, i) => (
            <div
              key={i}
              style={{
                marginBottom: i < message.messages!.length - 1 ? '0.75rem' : 0,
                paddingBottom: i < message.messages!.length - 1 ? '0.75rem' : 0,
                borderBottom: i < message.messages!.length - 1 ? '1px solid var(--color-border)' : 'none'
              }}
            >
              <div
                style={{
                  fontWeight: 'var(--font-weight-semibold)',
                  color: 'var(--color-text-secondary)',
                  marginBottom: '0.25rem',
                  fontSize: 'var(--font-size-xs)',
                  textTransform: 'uppercase'
                }}
              >
                {m.role}
              </div>
              <div style={{ color: 'var(--color-text)' }}>{m.content}</div>
            </div>
          ))}
        </div>
      )}

      <div style={{ marginTop: '1rem', display: 'flex', gap: '0.5rem' }}>
        <Link
          to={`/channels/${message.channel_id}`}
          className="btn btn-secondary btn-sm"
          style={{ textDecoration: 'none' }}
        >
          View Channel
        </Link>
      </div>
    </div>
  )
}
