import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api, Channel, TaskLog } from '../api/client'

export default function ChannelView() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
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
    const interval = setInterval(loadData, 3000)
    return () => clearInterval(interval)
  }, [id])

  if (loading && !channel) {
    return <div className="loading">Loading channel...</div>
  }

  if (!channel) {
    return (
      <div className="card">
        <h2>Channel not found</h2>
        <button className="btn btn-primary" onClick={() => navigate('/messages')}>
          Back to Messages
        </button>
      </div>
    )
  }

  return (
    <div>
      <button className="btn btn-secondary" onClick={() => navigate('/messages')} style={{ marginBottom: '1.5rem' }}>
        ← Back to Messages
      </button>

      {error && <div className="error-message">{error}</div>}

      <div className="card" style={{ marginBottom: '1.5rem' }}>
        <h2 style={{ fontSize: '2rem', fontWeight: '700', marginBottom: '1rem' }}>
          {channel.id}
        </h2>
        <div>
          <label style={{ display: 'block', fontWeight: '600', marginBottom: '0.5rem', color: 'var(--color-text-secondary)' }}>
            Members
          </label>
          <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
            {channel.members.map(member => (
              <code key={member} style={{ padding: '0.5rem 0.75rem', background: 'var(--color-bg-tertiary)', borderRadius: 'var(--radius)' }}>
                {member}
              </code>
            ))}
          </div>
        </div>
      </div>

      <h3 style={{ fontSize: '1.5rem', fontWeight: '600', marginBottom: '1rem' }}>
        Messages ({messages.length})
      </h3>

      {messages.length === 0 ? (
        <div className="card" style={{ textAlign: 'center', padding: '3rem' }}>
          <p style={{ color: 'var(--color-text-muted)' }}>
            No messages in this channel yet
          </p>
        </div>
      ) : (
        <div style={{ display: 'grid', gap: '1rem' }}>
          {messages.map(msg => (
            <div key={msg.id} className="card">
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'start', marginBottom: '0.75rem' }}>
                <div>
                  <span className={`badge ${msg.status}`}>{msg.status}</span>
                  <span className={`badge ${msg.direction}`} style={{ marginLeft: '0.5rem' }}>
                    {msg.direction}
                  </span>
                </div>
                <div style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>
                  {new Date(msg.created_at).toLocaleString()}
                </div>
              </div>

              <div style={{ fontSize: '0.875rem', marginBottom: '0.75rem' }}>
                <code>{msg.sender_id}</code>
                <span style={{ margin: '0 0.5rem', color: 'var(--color-text-muted)' }}>→</span>
                <code>{msg.recipient_id || 'broadcast'}</code>
              </div>

              {msg.messages && msg.messages.length > 0 && (
                <div style={{ background: 'var(--color-bg-tertiary)', padding: '1rem', borderRadius: 'var(--radius)', fontSize: '0.875rem' }}>
                  {msg.messages.map((m, i) => (
                    <div key={i} style={{ marginBottom: i < msg.messages!.length - 1 ? '0.75rem' : 0 }}>
                      <div style={{ fontWeight: '600', color: 'var(--color-text-secondary)', marginBottom: '0.25rem' }}>
                        {m.role}
                      </div>
                      <div style={{ color: 'var(--color-text)' }}>
                        {m.content}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
