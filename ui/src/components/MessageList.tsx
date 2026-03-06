import { useRef, useEffect, useMemo, useState } from 'react'
import { useAgentStore } from '../stores/agentStore'
import { useUIStore } from '../stores/uiStore'
import ProcessGroup from './ProcessGroup'
import type { Message } from '../types'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import './MessageList.css'

interface MessageGroup {
  type: 'user' | 'process' | 'standalone'
  messages: Message[]
  response?: Message
}

function groupMessages(messages: Message[]): MessageGroup[] {
  const groups: MessageGroup[] = []
  let currentProcess: Message[] | null = null

  for (const msg of messages) {
    if (msg.type === 'user') {
      if (currentProcess) {
        groups.push({ type: 'process', messages: currentProcess })
        currentProcess = null
      }
      groups.push({ type: 'user', messages: [msg] })
    } else if (msg.type === 'response') {
      if (currentProcess) {
        groups.push({ type: 'process', messages: currentProcess, response: msg })
        currentProcess = null
      } else {
        groups.push({ type: 'standalone', messages: [msg] })
      }
    } else if (['error', 'warning', 'hint'].includes(msg.type)) {
      groups.push({ type: 'standalone', messages: [msg] })
    } else {
      if (!currentProcess) currentProcess = []
      currentProcess.push(msg)
    }
  }

  if (currentProcess) {
    groups.push({ type: 'process', messages: currentProcess })
  }

  return groups
}

function UserMessage({ message }: { message: Message }) {
  return (
    <div className="message user-message">
      <div className="message-content">
        <pre className="user-text">{message.content}</pre>
      </div>
    </div>
  )
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Fallback for non-secure contexts
      const ta = document.createElement('textarea')
      ta.value = text
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  return (
    <button className="icon-button action-small" title={copied ? 'Copied!' : 'Copy'} onClick={handleCopy}>
      <span className="material-symbols-outlined">{copied ? 'check' : 'content_copy'}</span>
    </button>
  )
}

function AgentResponse({ message }: { message: Message }) {
  return (
    <div className="message agent-response">
      <div className="message-content markdown-body">
        <Markdown remarkPlugins={[remarkGfm]}>{message.content}</Markdown>
      </div>
      <div className="message-actions">
        <CopyButton text={message.content} />
      </div>
    </div>
  )
}

function StandaloneMessage({ message }: { message: Message }) {
  const styleMap: Record<string, string> = {
    error: 'standalone-error',
    warning: 'standalone-warning',
    hint: 'standalone-hint',
  }

  if (message.type === 'response') {
    return <AgentResponse message={message} />
  }

  const cls = styleMap[message.type] || ''
  return (
    <div className={`message standalone-message ${cls}`}>
      <div className="standalone-content">
        {message.heading && <span className="standalone-heading">{message.heading}</span>}
        <span>{message.content}</span>
      </div>
    </div>
  )
}

function MessageList() {
  const messages = useAgentStore((s) => s.messages)
  const loading = useAgentStore((s) => s.loading)
  const agentWidth = useUIStore((s) => s.agentWidth)
  const detailMode = useUIStore((s) => s.detailMode)
  const scrollRef = useRef<HTMLDivElement>(null)

  const messageGroups = useMemo(() => groupMessages(messages), [messages])

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [messages])

  const maxWidth = agentWidth === 'full' ? '100%' : agentWidth

  if (loading) {
    return (
      <div className="message-list">
        <div className="message-list-inner" style={{ maxWidth }}>
          <div className="message-list-empty">
            <span className="material-symbols-outlined spinning" style={{ fontSize: 32, opacity: 0.4 }}>progress_activity</span>
            <p>Loading messages...</p>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="message-list" ref={scrollRef}>
      <div className="message-list-inner" style={{ maxWidth }}>
        {messageGroups.length === 0 ? (
          <div className="message-list-empty">
            <span className="material-symbols-outlined" style={{ fontSize: 48, opacity: 0.2 }}>chat</span>
            <p>Send a message to get started</p>
          </div>
        ) : (
          messageGroups.map((group, i) => {
            if (group.type === 'user') {
              return <UserMessage key={i} message={group.messages[0]} />
            }
            if (group.type === 'process') {
              return (
                <ProcessGroup
                  key={i}
                  steps={group.messages}
                  response={group.response}
                  detailMode={detailMode}
                />
              )
            }
            return <StandaloneMessage key={i} message={group.messages[0]} />
          })
        )}
      </div>
    </div>
  )
}

export default MessageList
