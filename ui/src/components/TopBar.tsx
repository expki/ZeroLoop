import { useState, useEffect } from 'react'
import { useUIStore } from '../stores/uiStore'
import { useChatStore } from '../stores/chatStore'
import './TopBar.css'

function TopBar() {
  const { sidebarOpen, toggleSidebar, connectionStatus } = useUIStore()
  const selectChat = useChatStore((s) => s.selectChat)
  const selectedChatId = useChatStore((s) => s.selectedChatId)
  const chats = useChatStore((s) => s.chats)
  const [time, setTime] = useState(new Date())

  useEffect(() => {
    const interval = setInterval(() => setTime(new Date()), 60000)
    return () => clearInterval(interval)
  }, [])

  const currentChat = chats.find((c) => c.id === selectedChatId)
  const timeStr = time.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  const dateStr = time.toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' })

  const statusLabel = {
    connected: 'Connected',
    degraded: 'Reconnecting',
    disconnected: 'Disconnected',
  }[connectionStatus]

  return (
    <div className="topbar">
      <div className="topbar-left">
        {!sidebarOpen && (
          <button className="icon-button" onClick={toggleSidebar} title="Open sidebar">
            <span className="material-symbols-outlined">menu</span>
          </button>
        )}
        <button className="icon-button" onClick={() => selectChat(null)} title="Home">
          <span className="material-symbols-outlined">home</span>
        </button>
        {currentChat && (
          <span className="topbar-chat-name">{currentChat.name}</span>
        )}
      </div>
      <div className="topbar-right">
        <span className="topbar-time">{timeStr}</span>
        <span className="topbar-date">{dateStr}</span>
        <div className={`connection-status ${connectionStatus}`}>
          <div className="status-dot" />
          <span className="status-label">{statusLabel}</span>
        </div>
      </div>
    </div>
  )
}

export default TopBar
