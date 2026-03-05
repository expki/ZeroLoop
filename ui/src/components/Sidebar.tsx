import { useState, useRef, useEffect } from 'react'
import { useChatStore } from '../stores/chatStore'
import { useUIStore } from '../stores/uiStore'
import type { ChatWidth, DetailMode } from '../types'
import './Sidebar.css'

function EditableChatName({ chatId, name, isActive }: { chatId: string; name: string; isActive: boolean }) {
  const [editing, setEditing] = useState(false)
  const [editValue, setEditValue] = useState(name)
  const inputRef = useRef<HTMLInputElement>(null)
  const renameChat = useChatStore((s) => s.renameChat)

  useEffect(() => {
    if (editing && inputRef.current) {
      inputRef.current.focus()
      inputRef.current.select()
    }
  }, [editing])

  // Sync name prop changes
  useEffect(() => {
    if (!editing) setEditValue(name)
  }, [name, editing])

  const handleSave = () => {
    const trimmed = editValue.trim()
    if (trimmed && trimmed !== name) {
      renameChat(chatId, trimmed)
    } else {
      setEditValue(name)
    }
    setEditing(false)
  }

  if (editing) {
    return (
      <input
        ref={inputRef}
        className="chat-name-input"
        value={editValue}
        onChange={(e) => setEditValue(e.target.value)}
        onBlur={handleSave}
        onKeyDown={(e) => {
          if (e.key === 'Enter') handleSave()
          if (e.key === 'Escape') { setEditValue(name); setEditing(false) }
        }}
        onClick={(e) => e.stopPropagation()}
      />
    )
  }

  return (
    <span
      className="chat-name"
      onDoubleClick={(e) => {
        e.stopPropagation()
        if (isActive) setEditing(true)
      }}
      title="Double-click to rename"
    >
      {name}
    </span>
  )
}

function Sidebar() {
  const { chats, selectedChatId, selectChat, createChat, deleteChat } = useChatStore()
  const {
    sidebarOpen, toggleSidebar, theme, toggleTheme,
    chatWidth, setChatWidth, detailMode, setDetailMode, setSidebarOpen,
  } = useUIStore()
  const [prefsOpen, setPrefsOpen] = useState(false)

  const handleSelectChat = (id: string) => {
    selectChat(id)
    if (window.innerWidth < 768) {
      setSidebarOpen(false)
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
        <button className="sidebar-brand" onClick={() => selectChat(null)}>
          <span className="brand-icon material-symbols-outlined">hub</span>
          <span className="brand-text">ZeroLoop</span>
        </button>
        <button className="icon-button" onClick={toggleSidebar} title="Toggle sidebar">
          <span className="material-symbols-outlined">menu</span>
        </button>
      </div>

      <button className="new-chat-btn" onClick={() => createChat('')}>
        <span className="material-symbols-outlined">add</span>
        New Chat
      </button>

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
              <EditableChatName chatId={chat.id} name={chat.name} isActive={selectedChatId === chat.id} />
              <button
                className="chat-close icon-button"
                onClick={(e) => {
                  e.stopPropagation()
                  deleteChat(chat.id)
                }}
                title="Remove chat"
              >
                <span className="material-symbols-outlined">close</span>
              </button>
            </div>
          ))
        )}
      </div>

      <div className="sidebar-bottom">
        <div className="prefs-section">
          <button
            className="prefs-toggle"
            onClick={() => setPrefsOpen(!prefsOpen)}
          >
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

export default Sidebar
