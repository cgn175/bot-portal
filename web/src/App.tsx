import { Routes, Route, Link, useLocation } from 'react-router-dom'
import { CopilotPopup } from '@copilotkit/react-ui'
import { AgentProvider } from './contexts/AgentContext'
import { CopilotProvider } from './copilot/CopilotProvider'
import { CopilotActions } from './copilot/CopilotActions'
import { ErrorBoundary } from './components/ErrorBoundary'
import Dashboard from './pages/Dashboard'
import AgentDetail from './pages/AgentDetail'
import IdentityFileEditor from './pages/IdentityFileEditor'
import MessageViewer from './pages/MessageViewer'
import ChannelView from './pages/ChannelView'
import Models from './pages/Models'
import AuthConfigs from './pages/AuthConfigs'
import TestChat from './pages/TestChat'

function App() {
  const location = useLocation()

  const isActive = (path: string) => {
    if (path === '/') {
      return location.pathname === '/'
    }
    return location.pathname.startsWith(path)
  }

  return (
    <CopilotProvider>
      <AgentProvider>
        <CopilotActions />
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
              to="/models"
              className={`nav-link ${isActive('/models') ? 'active' : ''}`}
              aria-current={isActive('/models') ? 'page' : undefined}
            >
              <span>Models</span>
            </Link>
            <Link
              to="/auth-configs"
              className={`nav-link ${isActive('/auth-configs') ? 'active' : ''}`}
              aria-current={isActive('/auth-configs') ? 'page' : undefined}
            >
              <span>Auth</span>
            </Link>
            <Link
              to="/test-chat"
              className={`nav-link ${isActive('/test-chat') ? 'active' : ''}`}
              aria-current={isActive('/test-chat') ? 'page' : undefined}
            >
              <span>Chat</span>
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
              <Route path="/agents/:agentId/identity" element={<IdentityFileEditor />} />
              <Route path="/models" element={<Models />} />
              <Route path="/auth-configs" element={<AuthConfigs />} />
              <Route path="/test-chat" element={<TestChat />} />
              <Route path="/messages" element={<MessageViewer />} />
              <Route path="/channels/:id" element={<ChannelView />} />
            </Routes>
          </ErrorBoundary>
        </main>
      </div>
      <CopilotPopup
        labels={{
          title: "Bot Portal Assistant",
          initial: "Hi! I can help you manage agents, models, and auth configs. What would you like to do?",
        }}
      />
    </AgentProvider>
    </CopilotProvider>
  )
}

export default App
