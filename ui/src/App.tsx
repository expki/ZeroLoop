import { useEffect } from 'react'
import { useUIStore } from './stores/uiStore'
import { useAgentStore } from './stores/agentStore'
import { useTerminalStore } from './stores/terminalStore'
import { useProjectStore } from './stores/projectStore'
import { ws } from './services/websocket'
import ProjectList from './components/ProjectList'
import ProjectSidebar from './components/ProjectSidebar'
import AgentArea from './components/AgentArea'
import TerminalArea from './components/TerminalArea'
import MonacoIDE from './components/MonacoIDE'
import WelcomeScreen from './components/WelcomeScreen'
import './App.css'

function App() {
  const theme = useUIStore((s) => s.theme)
  const sidebarOpen = useUIStore((s) => s.sidebarOpen)
  const setSidebarOpen = useUIStore((s) => s.setSidebarOpen)
  const selectedProjectId = useProjectStore((s) => s.selectedProjectId)
  const mainView = useProjectStore((s) => s.mainView)
  const selectedAgentId = useAgentStore((s) => s.selectedAgentId)
  const selectedTerminalId = useTerminalStore((s) => s.selectedTerminalId)
  const initAgent = useAgentStore((s) => s.init)
  const initTerminal = useTerminalStore((s) => s.init)
  const initProject = useProjectStore((s) => s.init)

  useEffect(() => {
    initAgent()
    initTerminal()
    initProject()

    const unsub = ws.on('_status', (payload: { status: string }) => {
      useUIStore.setState({
        connectionStatus: payload.status as 'connected' | 'degraded' | 'disconnected',
      })
    })

    return unsub
  }, [initAgent, initTerminal, initProject])

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
  } else if (mainView.type === 'terminal' && selectedTerminalId) {
    mainContent = <TerminalArea />
  } else if (selectedAgentId) {
    mainContent = <AgentArea />
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
