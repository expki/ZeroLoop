import { useChatStore } from '../stores/chatStore'
import { useProjectStore } from '../stores/projectStore'
import { useUIStore } from '../stores/uiStore'
import './WelcomeScreen.css'

function WelcomeScreen() {
  const createChat = useChatStore((s) => s.createChat)
  const selectedProjectId = useProjectStore((s) => s.selectedProjectId)
  const { sidebarOpen, toggleSidebar, setSidebarOpen } = useUIStore()

  const handleNewChat = () => {
    if (selectedProjectId) {
      createChat(selectedProjectId)
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
          <p className="welcome-subtitle">Start a chat to begin working on your project</p>
        </div>
        <div className="action-grid">
          <button className="action-card" onClick={handleNewChat}>
            <span className="material-symbols-outlined action-icon">add_comment</span>
            <span className="action-title">New Chat</span>
            <span className="action-desc">Start a new conversation</span>
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
