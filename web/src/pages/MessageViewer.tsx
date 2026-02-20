import { useState, useEffect } from 'react'
import { api, Channel, TaskLog } from '../api/client'

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
    const interval = setInterval(loadMessages, 3000)
    return () => clearInterval(interval)
  }, [selectedChannel])

  return (
    <div>
      <h2 style={{ fontSize: '2rem', fontWeight: '700', marginBottom: '1.5rem' }}>
        Messages
      </h2>

      {error && <div className="error-message">{error}</div>}

      <div className="card" style={{ marginBottom: '1.5rem' }}>
        <label style={{ display: 'block', fontWeight: '600', marginBottom: '0.5rem', color: 'var(--color-text-secondary)' }}>
          Filter by Channel
        </label>
        <select 
          value={selectedChannel} 
          onChange={e => setSelectedChannel(e.target.value)}
          style={{ width: '100%', padding: '0.625rem', background: 'var(--color-bg-tertiary)', border: '1px solid var(--color-border)', borderRadius: 'var(--radius)', color: 'var(--color-text)' }}
        >
          <option value="">Select a channel...</option>
          {channels.map(channel => (
            <option key={channel.id} value={channel.id}>
              {channel.id} ({channel.members.join(', ')})
            </option>
          ))}
        </select>
      </div>

      {loading && <div className="loading">Loading messages...</div>}

      {!loading && !selectedChannel && (
        <div className="card" style={{ textAlign: 'center', padding: '3rem' }}>
          <p style={{ color: 'var(--color-text-muted)' }}>
            Select a channel to view messages
          </p>
        </div>
      )}

      {!loading && selectedChannel && messages.length === 0 && (
        <div className="card" style={{ textAlign: 'center', padding: '3rem' }}>
          <p style={{ color: 'var(--color-text-muted)' }}>
            No messages in this channel yet
          </p>
        </div>
      )}

      {!loading && messages.length > 0 && (
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
