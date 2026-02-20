import { Routes, Route, Link } from 'react-router-dom'
import Dashboard from './pages/Dashboard'
import AgentDetail from './pages/AgentDetail'
import MessageViewer from './pages/MessageViewer'
import ChannelView from './pages/ChannelView'

function App() {
  return (
    <div className="app">
      <nav className="navbar">
        <h1>Bot Portal</h1>
        <div className="nav-links">
          <Link to="/">Dashboard</Link>
          <Link to="/messages">Messages</Link>
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
  )
}

export default App
