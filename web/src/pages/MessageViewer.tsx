import { useState, useEffect } from 'react'

interface TaskLog {
  id: string
  channelId: string
  senderId: string
  recipientId: string
  status: string
  direction: string
  createdAt: string
}

export default function MessageViewer() {
  const [messages, setMessages] = useState<TaskLog[]>([])
  const [channels, setChannels] = useState<string[]>([])
  const [selectedChannel, setSelectedChannel] = useState<string>('')

  useEffect(() => {
    fetch('/api/channels')
      .then(res => res.json())
      .then(data => setChannels(data.map((c: { id: string }) => c.id)))
  }, [])

  useEffect(() => {
    const url = selectedChannel 
      ? `/api/channels/${selectedChannel}/messages?messages=true`
      : '/api/messages/stream'
    // Note: For SSE, we'd use EventSource instead
    fetch(url)
      .then(res => res.json())
      .then(data => setMessages(data))
      .catch(console.error)
  }, [selectedChannel])

  return (
    <div className="message-viewer">
      <h2>Messages</h2>
      <div className="filters">
        <select 
          value={selectedChannel} 
          onChange={e => setSelectedChannel(e.target.value)}
        >
          <option value="">All Channels</option>
          {channels.map(channel => (
            <option key={channel} value={channel}>{channel}</option>
          ))}
        </select>
      </div>
      <div className="message-list">
        {messages.map(msg => (
          <div key={msg.id} className={`message ${msg.direction}`}>
            <div className="message-header">
              <span className="channel">{msg.channelId}</span>
              <span className={`status ${msg.status}`}>{msg.status}</span>
            </div>
            <div className="message-body">
              <span className="sender">{msg.senderId}</span>
              <span className="arrow">→</span>
              <span className="recipient">{msg.recipientId || 'all'}</span>
            </div>
            <div className="message-time">
              {new Date(msg.createdAt).toLocaleString()}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
