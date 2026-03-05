import { useChatStore } from '../stores/chatStore'
import { useUIStore } from '../stores/uiStore'
import './WelcomeScreen.css'

const actions = [
  { icon: 'add_comment', title: 'New Chat', description: 'Start a new conversation' },
  { icon: 'folder', title: 'Projects', description: 'Manage your projects' },
  { icon: 'psychology', title: 'Memory', description: 'View agent memory' },
  { icon: 'schedule', title: 'Scheduler', description: 'Schedule automated tasks' },
  { icon: 'settings', title: 'Settings', description: 'Configure agent behavior' },
  { icon: 'description', title: 'Files', description: 'Browse workspace files' },
]

function WelcomeScreen() {
  const createChat = useChatStore((s) => s.createChat)
  const { sidebarOpen, toggleSidebar } = useUIStore()

  const handleAction = (title: string) => {
    if (title === 'New Chat') {
      createChat()
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
          <p className="welcome-subtitle">AI Agent Framework</p>
        </div>
        <div className="action-grid">
          {actions.map((action) => (
            <button
              key={action.title}
              className="action-card"
              onClick={() => handleAction(action.title)}
            >
              <span className="material-symbols-outlined action-icon">{action.icon}</span>
              <span className="action-title">{action.title}</span>
              <span className="action-desc">{action.description}</span>
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}

export default WelcomeScreen
