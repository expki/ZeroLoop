import { useState, useRef, useCallback } from 'react'
import { useAgentStore } from '../stores/agentStore'
import './AgentInput.css'

function AgentInput() {
  const [input, setInput] = useState('')
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const sendMessage = useAgentStore((s) => s.sendMessage)
  const intervene = useAgentStore((s) => s.intervene)
  const paused = useAgentStore((s) => s.paused)
  const queueSize = useAgentStore((s) => s.queueSize)
  const togglePause = useAgentStore((s) => s.togglePause)
  const cancelAgent = useAgentStore((s) => s.cancelAgent)
  const clearAgent = useAgentStore((s) => s.clearAgent)
  const exportAgent = useAgentStore((s) => s.exportAgent)
  const selectedAgent = useAgentStore((s) => s.agents.find((a) => a.id === s.selectedAgentId))
  const isRunning = selectedAgent?.running ?? false
  const isAutomated = selectedAgent?.type === 'automated'
  const isInfinite = selectedAgent?.mode === 'infinite'

  const handleSend = useCallback(() => {
    if (!input.trim()) return
    // Automated agents use intervene when running, sendMessage otherwise
    if (isAutomated && isRunning) {
      intervene(input)
    } else {
      sendMessage(input)
    }
    setInput('')
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
    }
  }, [input, sendMessage, intervene, isAutomated, isRunning])

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
    <div className="agent-input-section">
      <div className="agent-input-container">
        <textarea
          ref={textareaRef}
          className="agent-textarea"
          placeholder={isAutomated && isRunning ? 'Inject message...' : 'Type a message...'}
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
        {!isAutomated && (
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
        )}
        <button
          className="action-btn action-btn-cancel"
          onClick={cancelAgent}
          disabled={!isRunning && !paused}
        >
          <span className="material-symbols-outlined">{isInfinite ? 'stop' : 'cancel'}</span>
          <span className="action-label">{isInfinite ? 'Stop' : 'Cancel'}</span>
        </button>
        {queueSize > 0 && (
          <span className="queue-indicator">
            <span className="material-symbols-outlined">queue</span>
            <span className="queue-count">{queueSize}</span>
          </span>
        )}
        <button className="action-btn" onClick={clearAgent}>
          <span className="material-symbols-outlined">delete_sweep</span>
          <span className="action-label">Clear</span>
        </button>
        <button className="action-btn" onClick={exportAgent}>
          <span className="material-symbols-outlined">download</span>
          <span className="action-label">Export</span>
        </button>
      </div>
    </div>
  )
}

export default AgentInput
