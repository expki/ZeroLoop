import { useRef, useCallback, useEffect } from 'react'
import { Terminal } from '@xterm/xterm'
import { useTerminalStore } from '../stores/terminalStore'
import { useUIStore } from '../stores/uiStore'
import TerminalView from './TerminalView'
import './TerminalArea.css'

function TerminalArea() {
  const { sidebarOpen, toggleSidebar } = useUIStore()
  const selectedTerminalId = useTerminalStore((s) => s.selectedTerminalId)
  const terminals = useTerminalStore((s) => s.terminals)
  const sendInput = useTerminalStore((s) => s.sendInput)
  const resizeTerminal = useTerminalStore((s) => s.resizeTerminal)
  const terminalRef = useRef<Terminal | null>(null)

  const currentTerminal = terminals.find((t) => t.id === selectedTerminalId)

  // Register the xterm instance so the store can write output to it
  const registerXterm = useTerminalStore((s) => s.registerXterm)
  const unregisterXterm = useTerminalStore((s) => s.unregisterXterm)

  useEffect(() => {
    if (selectedTerminalId && terminalRef.current) {
      registerXterm(selectedTerminalId, terminalRef.current)
      return () => unregisterXterm(selectedTerminalId)
    }
  }, [selectedTerminalId, registerXterm, unregisterXterm])

  const handleData = useCallback(
    (data: string) => {
      if (selectedTerminalId) {
        sendInput(selectedTerminalId, data)
      }
    },
    [selectedTerminalId, sendInput]
  )

  const handleResize = useCallback(
    (cols: number, rows: number) => {
      if (selectedTerminalId) {
        resizeTerminal(selectedTerminalId, cols, rows)
      }
    },
    [selectedTerminalId, resizeTerminal]
  )

  return (
    <div className="terminal-area">
      <div className="topbar">
        <div className="topbar-left">
          {!sidebarOpen && (
            <button className="icon-button" onClick={toggleSidebar} title="Open sidebar">
              <span className="material-symbols-outlined">menu</span>
            </button>
          )}
          <span className="material-symbols-outlined" style={{ fontSize: '18px', opacity: 0.6 }}>
            terminal
          </span>
          {currentTerminal && (
            <span className="topbar-agent-name">{currentTerminal.name}</span>
          )}
        </div>
      </div>
      <div className="terminal-container">
        {selectedTerminalId && (
          <TerminalView
            key={selectedTerminalId}
            onData={handleData}
            onResize={handleResize}
            terminalRef={terminalRef}
          />
        )}
      </div>
    </div>
  )
}

export default TerminalArea
