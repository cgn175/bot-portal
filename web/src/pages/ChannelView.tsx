import { useState, useEffect } from 'react'
import { useParams } from 'react-router-dom'

interface TaskLog {
  id: string
  channelId: string
  senderId: string
  recipientId: string
  status: string
  direction: string
  createdAt: string
}

export default function ChannelView() {
  const { id } = useParams<{ id: string }>()
  const [messages, setMessages] = useState<TaskLog[]>([])

  useEffect(() => {
    if (id) {
      fetch(`/api/channels/${id}/messages?messages=true`)
        .then(res => res.json())
        .then(data => setMessages(data))
        .catch(console.error)
    }
  }, [id])

  return (
    <div className="channel-view">
      <h2>Channel: {id}</h2>
      <div className="message-list">
        {messages.map(msg => (
          <div key={msg.id} className={`message ${msg.direction}`}>
            <div className="message-header">
              <span className="sender">{msg.senderId}</span>
              <span className="arrow">→</span>
              <span className="recipient">{msg.recipientId || 'all'}</span>
              <span className={`status ${msg.status}`}>{msg.status}</span>
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
