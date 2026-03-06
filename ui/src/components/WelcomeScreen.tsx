import { useAgentStore } from '../stores/agentStore'
import { useProjectStore } from '../stores/projectStore'
import { useUIStore } from '../stores/uiStore'
import './WelcomeScreen.css'

function WelcomeScreen() {
  const createAgent = useAgentStore((s) => s.createAgent)
  const selectedProjectId = useProjectStore((s) => s.selectedProjectId)
  const { sidebarOpen, toggleSidebar, setSidebarOpen } = useUIStore()

  const handleNewAgent = () => {
    if (selectedProjectId) {
      createAgent(selectedProjectId)
    }
  }

  return (
    <div className="welcome-screen">
      {!sidebarOpen && (
        <button className="welcome-menu-btn icon-button" onClick={toggleSidebar} title="Open sidebar">
          <span className="material-symbols-outlined">menu</span>
        </button>
      )}
      <div className="welcome-content">
        <div className="welcome-hero">
          <span className="welcome-logo material-symbols-outlined">hub</span>
          <h1 className="welcome-title">ZeroLoop</h1>
          <p className="welcome-subtitle">Start an agent to begin working on your project</p>
        </div>
        <div className="action-grid">
          <button className="action-card" onClick={handleNewAgent}>
            <span className="material-symbols-outlined action-icon">add_comment</span>
            <span className="action-title">New Agent</span>
            <span className="action-desc">Start a new agent session</span>
          </button>
          <button className="action-card" onClick={() => setSidebarOpen(true)}>
            <span className="material-symbols-outlined action-icon">settings</span>
            <span className="action-title">Preferences</span>
            <span className="action-desc">Configure display settings</span>
          </button>
        </div>
      </div>
    </div>
  )
}

export default WelcomeScreen
