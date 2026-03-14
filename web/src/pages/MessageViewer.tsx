import { useState, useEffect, useRef, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { api, Channel, TaskLog } from '../api/client'
import StatusBadge from '../components/StatusBadge'

// ============================================================================
// Main Component
// ============================================================================

export default function MessageViewer() {
  const [channels, setChannels] = useState<Channel[]>([])
  const [selectedChannelId, setSelectedChannelId] = useState<string>('')
  const [messages, setMessages] = useState<TaskLog[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [mobilePanel, setMobilePanel] = useState<'channels' | 'chat' | 'details'>('channels')
  const [unread, setUnread] = useState<Record<string, number>>({})

  // Load channels
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

  // Global SSE — subscribe to ALL channels for unread badges
  const selectedChannelIdRef = useRef(selectedChannelId)
  useEffect(() => { selectedChannelIdRef.current = selectedChannelId }, [selectedChannelId])

  useEffect(() => {
    const es = api.streamMessages()
    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        const channelId: string = data?.channel_id
        if (!channelId) return
        // Refresh channel list so new channels appear
        api.listChannels().then(d => setChannels(d || []))
        // Increment unread if not the currently viewed channel
        if (channelId !== selectedChannelIdRef.current) {
          setUnread(prev => ({ ...prev, [channelId]: (prev[channelId] || 0) + 1 }))
        }
      } catch { /* ignore parse errors */ }
    }
    return () => es.close()
  }, [])

  // Load messages when channel selected + SSE
  useEffect(() => {
    if (!selectedChannelId) {
      setMessages([])
      setLoading(false)
      return
    }

    const loadMessages = async () => {
      try {
        setLoading(true)
        setError('')
        const data = await api.getChannelMessages(selectedChannelId)
        setMessages(data || [])
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load messages')
      } finally {
        setLoading(false)
      }
    }

    loadMessages()

    const eventSource = api.streamMessages(selectedChannelId)
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
      pollInterval = setInterval(loadMessages, 10000)
    }

    return () => {
      eventSource.close()
      if (pollInterval) clearInterval(pollInterval)
    }
  }, [selectedChannelId])

  const selectedChannel = channels.find(c => c.id === selectedChannelId)

  const handleSelectChannel = useCallback((id: string) => {
    setSelectedChannelId(id)
    setUnread(prev => { const n = { ...prev }; delete n[id]; return n })
    setMobilePanel('chat')
  }, [])

  return (
    <div className="msger-layout">
      {/* Left Panel — Channel List */}
      <div className={`msger-channels ${mobilePanel === 'channels' ? 'msger-panel-active' : ''}`}>
        <div className="msger-channels-header">
          <h2>Agent Channels</h2>
        </div>
        <div className="msger-channels-list">
          {channels.length === 0 ? (
            <div className="msger-channels-empty">No channels yet</div>
          ) : (
            channels.map(channel => (
              <ChannelItem
                key={channel.id}
                channel={channel}
                isActive={channel.id === selectedChannelId}
                unread={unread[channel.id] || 0}
                onClick={handleSelectChannel}
              />
            ))
          )}
        </div>
      </div>

      {/* Center Panel — Chat Area */}
      <div className={`msger-chat ${mobilePanel === 'chat' ? 'msger-panel-active' : ''}`}>
        {selectedChannel ? (
          <>
            <ChatHeader
              channel={selectedChannel}
              onBack={() => setMobilePanel('channels')}
              onInfo={() => setMobilePanel('details')}
            />
            {error && (
              <div className="msger-chat-error">{error}</div>
            )}
            <ChatMessages messages={messages} loading={loading} />
          </>
        ) : (
          <div className="msger-chat-empty">
            <div className="msger-chat-empty-icon">💬</div>
            <h3>Select a channel</h3>
            <p>Choose an agent channel from the left to view messages.</p>
          </div>
        )}
      </div>

      {/* Right Panel — Channel Details */}
      <div className={`msger-details ${mobilePanel === 'details' ? 'msger-panel-active' : ''}`}>
        {selectedChannel ? (
          <ChannelDetailsPanel
            channel={selectedChannel}
            messages={messages}
            onBack={() => setMobilePanel('chat')}
          />
        ) : (
          <div className="msger-details-empty">
            <p>Select a channel to view details</p>
          </div>
        )}
      </div>
    </div>
  )
}

// ============================================================================
// Channel List Item
// ============================================================================

function ChannelItem({ channel, isActive, unread, onClick }: {
  channel: Channel
  isActive: boolean
  unread: number
  onClick: (id: string) => void
}) {
  return (
    <button
      className={`msger-channel-item ${isActive ? 'active' : ''}`}
      onClick={() => onClick(channel.id)}
    >
      <div className="msger-channel-avatar">
        <span>{channel.id.charAt(0).toUpperCase()}</span>
      </div>
      <div className="msger-channel-info">
        <div className="msger-channel-name">{channel.id}</div>
        <div className="msger-channel-preview">
          {channel.members.join(', ')}
        </div>
      </div>
      <div className="msger-channel-meta">
        <div className="msger-channel-time">
          {channel.created_at ? formatTime(channel.created_at) : ''}
        </div>
        {unread > 0 && (
          <div className="msger-unread-badge">{unread > 99 ? '99+' : unread}</div>
        )}
      </div>
    </button>
  )
}

// ============================================================================
// Chat Header
// ============================================================================

function ChatHeader({ channel, onBack, onInfo }: {
  channel: Channel
  onBack: () => void
  onInfo: () => void
}) {
  return (
    <div className="msger-chat-header">
      <button className="msger-back-btn" onClick={onBack} aria-label="Back to channels">
        ←
      </button>
      <div className="msger-chat-header-avatar">
        <span>{channel.id.charAt(0).toUpperCase()}</span>
      </div>
      <div className="msger-chat-header-info">
        <div className="msger-chat-header-name">{channel.id}</div>
        <div className="msger-chat-header-members">
          {channel.members.length} member{channel.members.length !== 1 ? 's' : ''}
        </div>
      </div>
      <button className="msger-info-btn" onClick={onInfo} aria-label="Channel details">
        ⓘ
      </button>
    </div>
  )
}

// ============================================================================
// Chat Messages (center panel body)
// ============================================================================

function ChatMessages({ messages, loading }: { messages: TaskLog[]; loading: boolean }) {
  const messagesEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  if (loading) {
    return (
      <div className="msger-chat-body">
        <div className="msger-chat-loading">
          <div className="loading-pulse"><span /><span /><span /></div>
          <p>Loading messages...</p>
        </div>
      </div>
    )
  }

  // Flatten TaskLogs into individual chat bubbles
  const bubbles = flattenMessages(messages)

  return (
    <div className="msger-chat-body">
      {bubbles.length === 0 ? (
        <div className="msger-chat-loading">
          <p>No messages in this channel yet.</p>
        </div>
      ) : (
        bubbles.map((bubble, idx) => (
          <ChatBubble key={idx} bubble={bubble} />
        ))
      )}
      <div ref={messagesEndRef} />
    </div>
  )
}

// ============================================================================
// Chat Bubble
// ============================================================================

interface BubbleData {
  role: 'user' | 'agent'
  senderId: string
  content: string
  timestamp: string
  status?: string
  direction?: string
}

function ChatBubble({ bubble }: { bubble: BubbleData }) {
  const isUser = bubble.role === 'user'

  return (
    <div className={`msger-bubble-row ${isUser ? 'msger-bubble-row-user' : 'msger-bubble-row-agent'}`}>
      {!isUser && (
        <div className="msger-bubble-avatar">
          <span>{bubble.senderId.charAt(0).toUpperCase()}</span>
        </div>
      )}
      <div className="msger-bubble-group">
        <div className="msger-bubble-sender">
          {isUser ? 'USER' : bubble.senderId.toUpperCase()}
        </div>
        <div className={`msger-bubble ${isUser ? 'msger-bubble-user' : 'msger-bubble-agent'}`}>
          <div className="msger-bubble-content">{bubble.content}</div>
        </div>
        <div className="msger-bubble-meta">
          <span className="msger-bubble-time">{formatTime(bubble.timestamp)}</span>
          {isUser && bubble.status === 'completed' && (
            <span className="msger-bubble-read">Read {formatTime(bubble.timestamp)}</span>
          )}
        </div>
      </div>
    </div>
  )
}

// ============================================================================
// Channel Details Panel (right)
// ============================================================================

function ChannelDetailsPanel({ channel, messages, onBack }: {
  channel: Channel
  messages: TaskLog[]
  onBack: () => void
}) {
  // Derive some stats from messages
  const latestStatus = messages.length > 0 ? messages[messages.length - 1].status : 'pending'
  const latestDirection = messages.length > 0 ? messages[messages.length - 1].direction : 'inbound'

  // Calculate average response time (mock-ish — diff between consecutive messages)
  const responseTime = calculateResponseTime(messages)

  return (
    <div className="msger-details-inner">
      <button className="msger-details-back-btn" onClick={onBack} aria-label="Back to chat">
        ← Back
      </button>

      <div className="msger-details-header">
        <h3>Channel Details</h3>
      </div>

      <div className="msger-details-avatar-section">
        <div className="msger-details-avatar">
          <span>{channel.id.charAt(0).toUpperCase()}</span>
        </div>
        <div className="msger-details-name">
          <span className="msger-details-label">Channel:</span>
          <strong>{channel.id}</strong>
        </div>
      </div>

      {/* Status */}
      <div className="msger-details-section">
        <h4>Status</h4>
        <div className="msger-details-badges">
          <StatusBadge status={latestStatus} />
          <StatusBadge status={latestDirection} />
        </div>
        {responseTime && (
          <div className="msger-details-response-time">
            Response Time: <strong>{responseTime}</strong>
          </div>
        )}
      </div>

      {/* Members */}
      <div className="msger-details-section">
        <h4>Members</h4>
        <div className="msger-details-members">
          {channel.members.map(member => (
            <div key={member} className="msger-details-member">
              <div className="msger-details-member-avatar">
                {member.charAt(0).toUpperCase()}
              </div>
              <span>{member}</span>
            </div>
          ))}
        </div>
      </div>

      {/* Stats */}
      <div className="msger-details-section">
        <h4>Activity</h4>
        <div className="msger-details-stat-row">
          <span>Total Messages</span>
          <strong>{messages.length}</strong>
        </div>
        <div className="msger-details-stat-row">
          <span>Inbound</span>
          <strong>{messages.filter(m => m.direction === 'inbound').length}</strong>
        </div>
        <div className="msger-details-stat-row">
          <span>Outbound</span>
          <strong>{messages.filter(m => m.direction === 'outbound').length}</strong>
        </div>
      </div>

      {/* Actions */}
      <div className="msger-details-section">
        <Link
          to={`/channels/${channel.id}`}
          className="btn btn-secondary msger-details-btn"
          style={{ textDecoration: 'none' }}
        >
          View Full Channel
        </Link>
      </div>
    </div>
  )
}

// ============================================================================
// Helpers
// ============================================================================

function flattenMessages(taskLogs: TaskLog[]): BubbleData[] {
  const bubbles: BubbleData[] = []

  const sortedLogs = [...taskLogs].reverse()
  for (const log of sortedLogs) {
    if (log.messages && log.messages.length > 0) {
      for (const m of log.messages) {
        bubbles.push({
          role: m.role === 'user' ? 'user' : 'agent',
          senderId: m.role === 'user' ? (log.sender_id || 'user') : (log.recipient_id || log.sender_id || 'agent'),
          content: m.content,
          timestamp: log.createdAt,
          status: log.status,
          direction: log.direction,
        })
      }
    } else {
      // TaskLog without nested messages — show as system event
      bubbles.push({
        role: log.direction === 'outbound' ? 'user' : 'agent',
        senderId: log.sender_id,
        content: `[${log.status}] Task from ${log.sender_id}`,
        timestamp: log.createdAt,
        status: log.status,
        direction: log.direction,
      })
    }
  }

  return bubbles
}

function formatTime(timestamp: string | undefined): string {
  if (!timestamp) return 'Unknown time'
  const date = new Date(timestamp)
  if (isNaN(date.getTime())) return 'Unknown time'

  const now = new Date()
  const isToday = date.toDateString() === now.toDateString()

  if (isToday) {
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }

  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)
  if (date.toDateString() === yesterday.toDateString()) {
    return 'Yesterday, ' + date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }

  return date.toLocaleDateString([], { month: 'short', day: 'numeric' }) +
    ', ' + date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

function calculateResponseTime(messages: TaskLog[]): string | null {
  if (messages.length < 2) return null

  let totalDiff = 0
  let count = 0

  for (let i = 1; i < messages.length; i++) {
    const prev = new Date(messages[i - 1].createdAt).getTime()
    const curr = new Date(messages[i].createdAt).getTime()
    const diff = curr - prev
    if (diff > 0 && diff < 3600000) { // ignore gaps > 1 hour
      totalDiff += diff
      count++
    }
  }

  if (count === 0) return null

  const avgMs = totalDiff / count
  if (avgMs < 1000) return `${Math.round(avgMs)}ms`
  if (avgMs < 60000) return `${(avgMs / 1000).toFixed(1)}s`
  return `${(avgMs / 60000).toFixed(1)}min`
}
