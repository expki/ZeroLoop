import { useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'

interface TerminalViewProps {
  onData: (data: string) => void
  onResize: (cols: number, rows: number) => void
  terminalRef: React.MutableRefObject<Terminal | null>
}

function TerminalView({ onData, onResize, terminalRef }: TerminalViewProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)

  useEffect(() => {
    if (!containerRef.current) return

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 14,
      lineHeight: 1.35,
      fontFamily: "'JetBrains Mono', 'Cascadia Code', 'Fira Code', 'Source Code Pro', 'Consolas', 'Monaco', monospace",
      letterSpacing: 0.5,
      theme: {
        background: '#1a1a2e',
        foreground: '#e0e0e0',
        cursor: '#e0e0e0',
        selectionBackground: '#3a3a5e',
        black: '#1a1a2e',
        red: '#ff6b6b',
        green: '#51cf66',
        yellow: '#ffd43b',
        blue: '#748ffc',
        magenta: '#da77f2',
        cyan: '#66d9e8',
        white: '#e0e0e0',
        brightBlack: '#555577',
        brightRed: '#ff8787',
        brightGreen: '#69db7c',
        brightYellow: '#ffe066',
        brightBlue: '#91a7ff',
        brightMagenta: '#e599f7',
        brightCyan: '#99e9f2',
        brightWhite: '#ffffff',
      },
    })

    const fitAddon = new FitAddon()
    term.loadAddon(fitAddon)

    term.open(containerRef.current)
    fitAddon.fit()

    terminalRef.current = term
    fitAddonRef.current = fitAddon

    // Report initial size
    onResize(term.cols, term.rows)

    // Forward user input
    term.onData((data) => {
      onData(data)
    })

    // Handle window resize
    const handleResize = () => {
      fitAddon.fit()
      onResize(term.cols, term.rows)
    }

    const observer = new ResizeObserver(handleResize)
    observer.observe(containerRef.current)

    return () => {
      observer.disconnect()
      term.dispose()
      terminalRef.current = null
      fitAddonRef.current = null
    }
  }, [onData, onResize, terminalRef])

  return (
    <div
      ref={containerRef}
      style={{ width: '100%', height: '100%', padding: '4px' }}
    />
  )
}

export default TerminalView
