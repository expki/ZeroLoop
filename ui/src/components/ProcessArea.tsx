import { useRef, useEffect, useCallback, useState } from 'react'
import { useProcessStore } from '../stores/processStore'
import { useUIStore } from '../stores/uiStore'
import './ProcessArea.css'

function ProcessArea() {
  const { sidebarOpen, toggleSidebar } = useUIStore()
  const selectedProcessId = useProcessStore((s) => s.selectedProcessId)
  const processes = useProcessStore((s) => s.processes)
  const logs = useProcessStore((s) => s.logs)
  const stopProcess = useProcessStore((s) => s.stopProcess)

  const logContainerRef = useRef<HTMLDivElement>(null)
  const [autoScroll, setAutoScroll] = useState(true)

  const currentProcess = processes.find((p) => p.id === selectedProcessId)
  const currentLog = selectedProcessId ? logs.get(selectedProcessId) || [] : []

  // Auto-scroll to bottom when new output arrives
  useEffect(() => {
    if (autoScroll && logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight
    }
  }, [currentLog.length, autoScroll])

  // Detect manual scroll to pause auto-scroll
  const handleScroll = useCallback(() => {
    if (!logContainerRef.current) return
    const { scrollTop, scrollHeight, clientHeight } = logContainerRef.current
    const atBottom = scrollHeight - scrollTop - clientHeight < 30
    setAutoScroll(atBottom)
  }, [])

  const handleStop = useCallback(() => {
    if (selectedProcessId) {
      stopProcess(selectedProcessId)
    }
  }, [selectedProcessId, stopProcess])

  const statusClass = currentProcess?.status === 'running' ? 'running' :
    currentProcess?.status === 'stopped' ? 'stopped' : 'exited'

  return (
    <div className="process-area">
      <div className="topbar">
        <div className="topbar-left">
          {!sidebarOpen && (
            <button className="icon-button" onClick={toggleSidebar} title="Open sidebar">
              <span className="material-symbols-outlined">menu</span>
            </button>
          )}
          <span className="material-symbols-outlined" style={{ fontSize: '18px', opacity: 0.6 }}>
            memory
          </span>
          {currentProcess && (
            <>
              <span className="topbar-agent-name">{currentProcess.command}</span>
              <span className={`process-status-badge ${statusClass}`}>
                {currentProcess.status}
                {currentProcess.exit_code !== undefined && currentProcess.status !== 'running'
                  ? ` (${currentProcess.exit_code})`
                  : ''}
              </span>
            </>
          )}
        </div>
        <div className="topbar-right">
          {currentProcess?.status === 'running' && (
            <button className="process-stop-btn" onClick={handleStop} title="Stop process">
              <span className="material-symbols-outlined" style={{ fontSize: '18px' }}>stop</span>
              <span>Stop</span>
            </button>
          )}
        </div>
      </div>
      <div className="process-log-wrapper">
        <div
          className="process-log-container"
          ref={logContainerRef}
          onScroll={handleScroll}
        >
          {currentLog.length === 0 ? (
            <div className="process-log-empty">Waiting for output...</div>
          ) : (
            currentLog.map((line, i) => (
              <span
                key={`${line.timestamp}-${i}`}
                className={`process-log-line ${line.stream === 'stderr' ? 'stderr' : 'stdout'}`}
              >{line.text}</span>
            ))
          )}
        </div>
        {!autoScroll && currentProcess?.status === 'running' && (
          <button
            className="process-scroll-btn"
            onClick={() => {
              setAutoScroll(true)
              if (logContainerRef.current) {
                logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight
              }
            }}
          >
            <span className="material-symbols-outlined" style={{ fontSize: '16px' }}>arrow_downward</span>
            Follow output
          </button>
        )}
      </div>
    </div>
  )
}

export default ProcessArea
