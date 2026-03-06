import { useState, useCallback } from 'react'
import type { Message, DetailMode } from '../types'
import { useAgentStore } from '../stores/agentStore'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import './ProcessGroup.css'

const BADGE_MAP: Record<string, { code: string; color: string }> = {
  agent: { code: 'GEN', color: 'var(--color-accent-blue)' },
  tool: { code: 'USE', color: 'var(--color-accent-amber)' },
  code_exe: { code: 'EXE', color: 'var(--color-accent-purple)' },
  info: { code: 'INF', color: 'var(--color-text-tertiary)' },
  progress: { code: 'HDL', color: 'var(--color-accent-slate)' },
  util: { code: 'UTL', color: 'var(--color-accent-slate)' },
  response: { code: 'RES', color: 'var(--color-accent-teal)' },
}

function getKvps(kvps?: Record<string, string> | string): Record<string, string> | undefined {
  if (!kvps || typeof kvps === 'string') return undefined
  return kvps
}

function getBadge(type: string, kvps?: Record<string, string> | string) {
  const parsed = getKvps(kvps)
  if (type === 'tool' && parsed?.tool_name) {
    const toolBadges: Record<string, { code: string; color: string }> = {
      code_execution: { code: 'EXE', color: 'var(--color-accent-purple)' },
      web_search: { code: 'WEB', color: 'var(--color-accent-indigo)' },
      memory: { code: 'MEM', color: 'var(--color-accent-teal)' },
      mcp: { code: 'MCP', color: 'var(--color-accent-amber)' },
      response: { code: 'RES', color: 'var(--color-accent-teal)' },
      call_subordinate: { code: 'SUB', color: 'var(--color-accent-rose)' },
      knowledge: { code: 'KNW', color: 'var(--color-accent-teal)' },
    }
    return toolBadges[parsed.tool_name] || BADGE_MAP.tool
  }
  return BADGE_MAP[type] || { code: '???', color: 'var(--color-text-tertiary)' }
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = useCallback(async (e: React.MouseEvent) => {
    e.stopPropagation()
    try {
      await navigator.clipboard.writeText(text)
    } catch {
      const ta = document.createElement('textarea')
      ta.value = text
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }, [text])

  return (
    <button className="icon-button action-small" title={copied ? 'Copied!' : 'Copy'} onClick={handleCopy}>
      <span className="material-symbols-outlined">{copied ? 'check' : 'content_copy'}</span>
    </button>
  )
}

function ProcessStep({ step, expanded }: { step: Message; expanded: boolean }) {
  const [isOpen, setIsOpen] = useState(expanded)
  const badge = getBadge(step.type, step.kvps)
  const kvps = getKvps(step.kvps)

  return (
    <div className="process-step">
      <div className="step-header" onClick={() => setIsOpen(!isOpen)}>
        <span className={`material-symbols-outlined step-chevron ${isOpen ? 'open' : ''}`}>
          chevron_right
        </span>
        <span
          className="step-badge"
          style={{ '--badge-color': badge.color } as React.CSSProperties}
        >
          {badge.code}
        </span>
        <span className="step-title">{step.heading || step.type}</span>
      </div>
      {isOpen && (
        <div className="step-detail">
          {kvps && Object.keys(kvps).length > 0 && (
            <table className="step-kvps">
              <tbody>
                {Object.entries(kvps).map(([key, value]) => (
                  <tr key={key}>
                    <td className="kvp-key">{key}</td>
                    <td className="kvp-value">{value}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          {step.content && (
            <div className={`step-content ${step.type === 'code_exe' ? 'terminal' : ''}`}>
              <pre>{step.content}</pre>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

interface ProcessGroupProps {
  steps: Message[]
  response?: Message
  detailMode: DetailMode
}

function ProcessGroup({ steps, response, detailMode }: ProcessGroupProps) {
  const [isOpen, setIsOpen] = useState(detailMode !== 'collapsed')
  const paused = useAgentStore((s) => s.paused)
  const isRunning = useAgentStore((s) => {
    const agent = s.agents.find((a) => a.id === s.selectedAgentId)
    return agent?.running ?? false
  })
  const lastStep = steps[steps.length - 1]
  const badge = getBadge(lastStep.type, lastStep.kvps)
  const title = steps[0]?.heading || 'Processing'
  const isComplete = !!response

  const getStepExpanded = (index: number) => {
    switch (detailMode) {
      case 'expanded': return true
      case 'collapsed': return false
      case 'list': return false
      case 'current': return index === steps.length - 1
      default: return false
    }
  }

  return (
    <div className={`process-group ${isComplete ? 'complete' : 'active'}`}>
      <div className="group-header" onClick={() => setIsOpen(!isOpen)}>
        <span className={`material-symbols-outlined group-chevron ${isOpen ? 'open' : ''}`}>
          chevron_right
        </span>
        <span
          className="group-badge"
          style={{ '--badge-color': badge.color } as React.CSSProperties}
        >
          {badge.code}
        </span>
        <span className="group-title">{title}</span>
        <span className="group-meta">
          <span className="step-count">
            {steps.length} step{steps.length !== 1 ? 's' : ''}
          </span>
          {!isComplete && paused && (
            <span className="group-spinner">
              <span className="material-symbols-outlined">pause</span>
            </span>
          )}
          {!isComplete && !paused && isRunning && (
            <span className="group-spinner">
              <span className="material-symbols-outlined spinning">progress_activity</span>
            </span>
          )}
        </span>
      </div>
      {isOpen && (
        <div className="group-content">
          <div className="group-steps">
            {steps.map((step, i) => (
              <ProcessStep key={step.id} step={step} expanded={getStepExpanded(i)} />
            ))}
          </div>
          {response && (
            <div className="group-response markdown-body">
              <Markdown remarkPlugins={[remarkGfm]}>{response.content}</Markdown>
              <div className="message-actions">
                <CopyButton text={response.content} />
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

export default ProcessGroup
