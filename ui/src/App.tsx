import { useUIStore } from './stores/uiStore'
import { useChatStore } from './stores/chatStore'
import Sidebar from './components/Sidebar'
import ChatArea from './components/ChatArea'
import WelcomeScreen from './components/WelcomeScreen'
import './App.css'

function App() {
  const theme = useUIStore((s) => s.theme)
  const sidebarOpen = useUIStore((s) => s.sidebarOpen)
  const setSidebarOpen = useUIStore((s) => s.setSidebarOpen)
  const selectedChatId = useChatStore((s) => s.selectedChatId)

  return (
    <div className={`app ${theme}-mode`}>
      {sidebarOpen && (
        <div className="sidebar-overlay" onClick={() => setSidebarOpen(false)} />
      )}
      <Sidebar />
      <main className="main-panel">
        {selectedChatId ? <ChatArea /> : <WelcomeScreen />}
      </main>
    </div>
  )
}

export default App
