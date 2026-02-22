import { Routes, Route, Link, useLocation } from 'react-router-dom'
import { AgentProvider } from './contexts/AgentContext'
import { ErrorBoundary } from './components/ErrorBoundary'
import Dashboard from './pages/Dashboard'
import AgentDetail from './pages/AgentDetail'
import MessageViewer from './pages/MessageViewer'
import ChannelView from './pages/ChannelView'

function App() {
  const location = useLocation()

  const isActive = (path: string) => {
    if (path === '/') {
      return location.pathname === '/'
    }
    return location.pathname.startsWith(path)
  }

  return (
    <AgentProvider>
      <div className="app">
        <a href="#content" className="skip-link">
          Skip to main content
        </a>

        <nav className="navbar" role="navigation" aria-label="Main navigation">
          <h1>
            <span aria-hidden="true">◆</span>
            Bot Portal
          </h1>
          <div className="nav-links">
            <Link
              to="/"
              className={`nav-link ${isActive('/') ? 'active' : ''}`}
              aria-current={isActive('/') ? 'page' : undefined}
            >
              <span>Agents</span>
            </Link>
            <Link
              to="/messages"
              className={`nav-link ${isActive('/messages') ? 'active' : ''}`}
              aria-current={isActive('/messages') ? 'page' : undefined}
            >
              <span>Messages</span>
            </Link>
          </div>
        </nav>

        <main id="content" className="content" role="main">
          <ErrorBoundary>
            <Routes>
              <Route path="/" element={<Dashboard />} />
              <Route path="/agents/:id" element={<AgentDetail />} />
              <Route path="/messages" element={<MessageViewer />} />
              <Route path="/channels/:id" element={<ChannelView />} />
            </Routes>
          </ErrorBoundary>
        </main>
      </div>
    </AgentProvider>
  )
}

export default App
