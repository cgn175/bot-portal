import { Routes, Route, Link, useLocation } from 'react-router-dom'
import { AgentProvider } from './contexts/AgentContext'
import Dashboard from './pages/Dashboard'
import AgentDetail from './pages/AgentDetail'
import MessageViewer from './pages/MessageViewer'
import ChannelView from './pages/ChannelView'

function App() {
  const location = useLocation()

  return (
    <AgentProvider>
      <div className="app">
        <nav className="navbar">
          <h1>🤖 Bot Portal</h1>
          <div className="nav-links">
            <Link 
              to="/" 
              style={{ 
                color: location.pathname === '/' ? 'var(--color-primary)' : undefined 
              }}
            >
              Agents
            </Link>
            <Link 
              to="/messages"
              style={{ 
                color: location.pathname === '/messages' ? 'var(--color-primary)' : undefined 
              }}
            >
              Messages
            </Link>
          </div>
        </nav>
        
        <main className="content">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/agents/:id" element={<AgentDetail />} />
            <Route path="/messages" element={<MessageViewer />} />
            <Route path="/channels/:id" element={<ChannelView />} />
          </Routes>
        </main>
      </div>
    </AgentProvider>
  )
}

export default App
