import { useEffect } from 'react'
import { useUIStore } from './stores/uiStore'
import { useChatStore } from './stores/chatStore'
import { useProjectStore } from './stores/projectStore'
import { ws } from './services/websocket'
import ProjectList from './components/ProjectList'
import ProjectSidebar from './components/ProjectSidebar'
import ChatArea from './components/ChatArea'
import MonacoIDE from './components/MonacoIDE'
import WelcomeScreen from './components/WelcomeScreen'
import './App.css'

function App() {
  const theme = useUIStore((s) => s.theme)
  const sidebarOpen = useUIStore((s) => s.sidebarOpen)
  const setSidebarOpen = useUIStore((s) => s.setSidebarOpen)
  const selectedProjectId = useProjectStore((s) => s.selectedProjectId)
  const mainView = useProjectStore((s) => s.mainView)
  const selectedChatId = useChatStore((s) => s.selectedChatId)
  const initChat = useChatStore((s) => s.init)
  const initProject = useProjectStore((s) => s.init)

  useEffect(() => {
    initChat()
    initProject()

    const unsub = ws.on('_status', (payload: { status: string }) => {
      useUIStore.setState({
        connectionStatus: payload.status as 'connected' | 'degraded' | 'disconnected',
      })
    })

    return unsub
  }, [initChat, initProject])

  // No project selected: show project list
  if (!selectedProjectId) {
    return (
      <div className={`app ${theme}-mode`}>
        <ProjectList />
      </div>
    )
  }

  // Project selected: show sidebar + main content
  let mainContent
  if (mainView.type === 'editor') {
    mainContent = <MonacoIDE />
  } else if (selectedChatId) {
    mainContent = <ChatArea />
  } else {
    mainContent = <WelcomeScreen />
  }

  return (
    <div className={`app ${theme}-mode`}>
      {sidebarOpen && (
        <div className="sidebar-overlay" onClick={() => setSidebarOpen(false)} />
      )}
      <ProjectSidebar />
      <main className="main-panel">
        {mainContent}
      </main>
    </div>
  )
}

export default App
