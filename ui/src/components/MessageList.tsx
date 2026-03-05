import { useRef, useEffect, useMemo } from 'react'
import { useChatStore } from '../stores/chatStore'
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

function AgentResponse({ message }: { message: Message }) {
  return (
    <div className="message agent-response">
      <div className="message-content markdown-body">
        <Markdown remarkPlugins={[remarkGfm]}>{message.content}</Markdown>
      </div>
      <div className="message-actions">
        <button className="icon-button action-small" title="Copy">
          <span className="material-symbols-outlined">content_copy</span>
        </button>
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
  const messages = useChatStore((s) => s.messages)
  const chatWidth = useUIStore((s) => s.chatWidth)
  const detailMode = useUIStore((s) => s.detailMode)
  const scrollRef = useRef<HTMLDivElement>(null)

  const messageGroups = useMemo(() => groupMessages(messages), [messages])

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [messages])

  const maxWidth = chatWidth === 'full' ? '100%' : chatWidth

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
