import { useState, useRef, useCallback } from 'react'
import { useChatStore } from '../stores/chatStore'
import './ChatInput.css'

function ChatInput() {
  const [input, setInput] = useState('')
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const sendMessage = useChatStore((s) => s.sendMessage)
  const paused = useChatStore((s) => s.paused)
  const queueSize = useChatStore((s) => s.queueSize)
  const togglePause = useChatStore((s) => s.togglePause)
  const cancelChat = useChatStore((s) => s.cancelChat)
  const clearChat = useChatStore((s) => s.clearChat)
  const exportChat = useChatStore((s) => s.exportChat)
  const isRunning = useChatStore((s) => {
    const chat = s.chats.find((c) => c.id === s.selectedChatId)
    return chat?.running ?? false
  })

  const handleSend = useCallback(() => {
    if (!input.trim()) return
    sendMessage(input)
    setInput('')
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
    }
  }, [input, sendMessage])

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const handleInput = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setInput(e.target.value)
    const el = e.target
    el.style.height = 'auto'
    el.style.height = Math.min(el.scrollHeight, 200) + 'px'
  }

  return (
    <div className="chat-input-section">
      <div className="chat-input-container">
        <textarea
          ref={textareaRef}
          className="chat-textarea"
          placeholder="Type a message..."
          value={input}
          onChange={handleInput}
          onKeyDown={handleKeyDown}
          rows={1}
        />
        <button
          className={`send-btn ${input.trim() ? 'active' : ''}`}
          onClick={handleSend}
          disabled={!input.trim()}
          title="Send message"
        >
          <span className="material-symbols-outlined">send</span>
        </button>
      </div>
      <div className="input-actions">
        <button
          className="action-btn"
          onClick={togglePause}
          disabled={!paused && !isRunning}
        >
          <span className="material-symbols-outlined">
            {paused ? 'play_arrow' : 'pause'}
          </span>
          <span className="action-label">{paused ? 'Resume' : 'Pause'}</span>
        </button>
        <button
          className="action-btn action-btn-cancel"
          onClick={cancelChat}
          disabled={!isRunning && !paused}
        >
          <span className="material-symbols-outlined">cancel</span>
          <span className="action-label">Cancel</span>
        </button>
        {queueSize > 0 && (
          <span className="queue-indicator">
            <span className="material-symbols-outlined">queue</span>
            <span className="queue-count">{queueSize}</span>
          </span>
        )}
        <button className="action-btn" onClick={clearChat}>
          <span className="material-symbols-outlined">delete_sweep</span>
          <span className="action-label">Clear</span>
        </button>
        <button className="action-btn" onClick={exportChat}>
          <span className="material-symbols-outlined">download</span>
          <span className="action-label">Export</span>
        </button>
      </div>
    </div>
  )
}

export default ChatInput
