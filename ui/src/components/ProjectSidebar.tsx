import { useState, useEffect } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { useChatStore } from '../stores/chatStore'
import { useUIStore } from '../stores/uiStore'
import ArboristFileTree from './ArboristFileTree'
import type { ChatWidth, DetailMode } from '../types'
import './Sidebar.css'
import './ProjectSidebar.css'

function ProjectSidebar() {
  const { selectedProjectId, selectProject, projects, loadFiles } = useProjectStore()
  const { chats, selectedChatId, selectChat, createChat, deleteChat, loadChatsForProject } = useChatStore()
  const {
    sidebarOpen, toggleSidebar, theme, toggleTheme,
    chatWidth, setChatWidth, detailMode, setDetailMode, setSidebarOpen,
  } = useUIStore()
  const [prefsOpen, setPrefsOpen] = useState(false)

  const project = projects.find((p) => p.id === selectedProjectId)

  // Load chats and files when project changes
  useEffect(() => {
    if (selectedProjectId) {
      loadChatsForProject(selectedProjectId)
      loadFiles(selectedProjectId)
    }
  }, [selectedProjectId, loadChatsForProject, loadFiles])

  const handleSelectChat = (id: string) => {
    selectChat(id)
    useProjectStore.getState().setMainView({ type: 'chat' })
    if (window.innerWidth < 768) {
      setSidebarOpen(false)
    }
  }

  const handleNewChat = () => {
    if (selectedProjectId) {
      createChat(selectedProjectId)
      useProjectStore.getState().setMainView({ type: 'chat' })
    }
  }

  const widthOptions: { label: string; value: ChatWidth }[] = [
    { label: 'S', value: '40em' },
    { label: 'M', value: '55em' },
    { label: 'L', value: '80em' },
    { label: 'Full', value: 'full' },
  ]

  const detailOptions: { label: string; value: DetailMode }[] = [
    { label: 'Hide', value: 'collapsed' },
    { label: 'List', value: 'list' },
    { label: 'Current', value: 'current' },
    { label: 'All', value: 'expanded' },
  ]

  return (
    <aside className={`sidebar ${sidebarOpen ? 'open' : 'closed'}`}>
      <div className="sidebar-header">
        <button className="sidebar-brand" onClick={() => selectProject(null)}>
          <span className="brand-icon material-symbols-outlined">hub</span>
          <span className="brand-text">ZeroLoop</span>
        </button>
        <button className="icon-button" onClick={toggleSidebar} title="Toggle sidebar">
          <span className="material-symbols-outlined">menu</span>
        </button>
      </div>

      {project && (
        <div className="project-name-bar">
          <span className="material-symbols-outlined" style={{ fontSize: '18px' }}>folder</span>
          <span className="project-name-text">{project.name}</span>
        </div>
      )}

      {/* Files section */}
      <div className="sidebar-section">
        <div className="sidebar-section-header">
          <span className="sidebar-section-title">Files</span>
        </div>
        {selectedProjectId && <ArboristFileTree projectId={selectedProjectId} />}
      </div>

      {/* Chats section */}
      <div className="sidebar-section sidebar-section-grow">
        <div className="sidebar-section-header">
          <span className="sidebar-section-title">Chats</span>
          <button className="icon-button" onClick={handleNewChat} title="New chat">
            <span className="material-symbols-outlined" style={{ fontSize: '18px' }}>add</span>
          </button>
        </div>
        <div className="chat-list">
          {chats.length === 0 ? (
            <div className="chat-list-empty">No chats yet</div>
          ) : (
            chats.map((chat) => (
              <div
                key={chat.id}
                className={`chat-item ${selectedChatId === chat.id ? 'active' : ''}`}
                onClick={() => handleSelectChat(chat.id)}
              >
                <div className={`chat-status-dot ${chat.running ? 'running' : ''}`} />
                <span className="chat-name">{chat.name}</span>
                <button
                  className="chat-close icon-button"
                  onClick={(e) => { e.stopPropagation(); deleteChat(chat.id) }}
                  title="Remove chat"
                >
                  <span className="material-symbols-outlined">close</span>
                </button>
              </div>
            ))
          )}
        </div>
      </div>

      <div className="sidebar-bottom">
        <div className="prefs-section">
          <button className="prefs-toggle" onClick={() => setPrefsOpen(!prefsOpen)}>
            <span className="material-symbols-outlined">settings</span>
            <span>Preferences</span>
            <span className={`material-symbols-outlined chevron ${prefsOpen ? 'open' : ''}`}>
              expand_more
            </span>
          </button>

          {prefsOpen && (
            <div className="prefs-content">
              <div className="pref-row">
                <span className="pref-label">Theme</span>
                <button className="toggle-switch" onClick={toggleTheme}>
                  <span className="material-symbols-outlined" style={{ fontSize: '16px' }}>
                    {theme === 'dark' ? 'dark_mode' : 'light_mode'}
                  </span>
                  <span>{theme === 'dark' ? 'Dark' : 'Light'}</span>
                </button>
              </div>

              <div className="pref-row">
                <span className="pref-label">Width</span>
                <div className="button-group">
                  {widthOptions.map((opt) => (
                    <button
                      key={opt.value}
                      className={`btn-group-item ${chatWidth === opt.value ? 'active' : ''}`}
                      onClick={() => setChatWidth(opt.value)}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
              </div>

              <div className="pref-row">
                <span className="pref-label">Detail</span>
                <div className="button-group">
                  {detailOptions.map((opt) => (
                    <button
                      key={opt.value}
                      className={`btn-group-item ${detailMode === opt.value ? 'active' : ''}`}
                      onClick={() => setDetailMode(opt.value)}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>

        <div className="sidebar-footer">
          <span className="version-text">ZeroLoop v0.1.0</span>
        </div>
      </div>
    </aside>
  )
}

export default ProjectSidebar
