import { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, Channel, TaskLog } from '../api/client'
import Alert from '../components/Alert'
import StatusBadge from '../components/StatusBadge'
import { LoadingState } from '../components/LoadingState'
import EmptyState from '../components/EmptyState'

export default function ChannelView() {
  const { id } = useParams<{ id: string }>()
  const [channel, setChannel] = useState<Channel | null>(null)
  const [messages, setMessages] = useState<TaskLog[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!id) return

    const loadData = async () => {
      try {
        setLoading(true)
        setError('')
        const [channelData, messagesData] = await Promise.all([
          api.getChannel(id),
          api.getChannelMessages(id)
        ])
        setChannel(channelData)
        setMessages(messagesData || [])
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load channel')
      } finally {
        setLoading(false)
      }
    }

    loadData()

    // Use SSE for real-time message updates
    const eventSource = api.streamMessages(id)
    let pollInterval: ReturnType<typeof setInterval> | null = null

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
      pollInterval = setInterval(loadData, 10000)
    }

    return () => {
      eventSource.close()
      if (pollInterval) clearInterval(pollInterval)
    }
  }, [id])

  if (loading && !channel) {
    return <LoadingState message="Loading channel..." />
  }

  if (!channel) {
    return (
      <EmptyState
        icon="🔍"
        title="Channel not found"
        description="The channel you're looking for doesn't exist or has been removed."
        action={
          <Link to="/messages" className="btn btn-primary" style={{ textDecoration: 'none' }}>
            Back to Messages
          </Link>
        }
      />
    )
  }

  return (
    <div>
      <div style={{ marginBottom: '1.5rem' }}>
        <Link
          to="/messages"
          className="btn btn-ghost"
          style={{ textDecoration: 'none', paddingLeft: 0 }}
        >
          ← Back to Messages
        </Link>
      </div>

      {error && (
        <Alert type="error" onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      <div className="card" style={{ marginBottom: '1.5rem' }}>
        <h2 style={{ fontSize: 'var(--font-size-3xl)', fontWeight: 'var(--font-weight-bold)', marginBottom: '1rem' }}>
          {channel.id}
        </h2>
        <div>
          <label style={labelStyle}>
            Members
          </label>
          <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
            {channel.members.map(member => (
              <code
                key={member}
                style={{
                  padding: '0.5rem 0.75rem',
                  background: 'var(--color-bg)',
                  borderRadius: 'var(--radius-md)',
                  border: '1px solid var(--color-border)'
                }}
              >
                {member}
              </code>
            ))}
          </div>
        </div>
      </div>

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <h3 style={{ fontSize: 'var(--font-size-xl)', fontWeight: 'var(--font-weight-semibold)' }}>
          Messages
        </h3>
        <span style={{ color: 'var(--color-text-muted)', fontSize: 'var(--font-size-sm)' }}>
          {messages.length} message{messages.length !== 1 ? 's' : ''}
        </span>
      </div>

      {messages.length === 0 ? (
        <EmptyState
          icon="📭"
          title="No messages yet"
          description="This channel doesn't have any messages yet. Messages will appear here when agents communicate."
        />
      ) : (
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
          {new Date(message.createdAt).toLocaleString()}
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
    </div>
  )
}

const labelStyle = {
  display: 'block',
  fontWeight: 'var(--font-weight-medium)',
  marginBottom: '0.5rem',
  color: 'var(--color-text-secondary)',
  fontSize: 'var(--font-size-sm)'
}
