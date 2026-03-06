import { useRef, useEffect, useCallback } from 'react'
import Editor, { type OnMount } from '@monaco-editor/react'
import type * as monacoTypes from 'monaco-editor'
import { monaco } from '../lib/monacoSetup'
import { useUIStore } from '../stores/uiStore'
import { useEditorStore, type EditorTab } from '../stores/editorStore'
import { api } from '../services/api'

function getLanguage(filename: string): string {
  const ext = filename.split('.').pop()?.toLowerCase()
  switch (ext) {
    case 'js': case 'mjs': case 'cjs': return 'javascript'
    case 'ts': case 'mts': case 'cts': return 'typescript'
    case 'jsx': return 'javascript'
    case 'tsx': return 'typescriptreact'
    case 'html': case 'htm': return 'html'
    case 'css': case 'scss': case 'less': return 'css'
    case 'json': return 'json'
    case 'md': case 'mdx': return 'markdown'
    case 'py': return 'python'
    case 'xml': case 'svg': return 'xml'
    case 'yaml': case 'yml': return 'yaml'
    case 'c': case 'h': case 'cpp': case 'hpp': case 'cc': return 'cpp'
    case 'java': return 'java'
    case 'rs': return 'rust'
    case 'sql': return 'sql'
    case 'php': return 'php'
    case 'go': return 'go'
    case 'sh': case 'bash': return 'shell'
    case 'dockerfile': return 'dockerfile'
    default: return 'plaintext'
  }
}

interface MonacoEditorProps {
  tab: EditorTab
  paneId: string
  projectId: string
  onSplit?: (direction: 'horizontal' | 'vertical') => void
}

export default function MonacoEditor({ tab, paneId, projectId, onSplit }: MonacoEditorProps) {
  const theme = useUIStore((s) => s.theme)
  const editorRef = useRef<monacoTypes.editor.IStandaloneCodeEditor | null>(null)
  const markDirty = useEditorStore((s) => s.markDirty)
  const markClean = useEditorStore((s) => s.markClean)
  const resolveConflict = useEditorStore((s) => s.resolveConflict)
  const closeSplitPane = useEditorStore((s) => s.closeSplitPane)
  const setCursorInfo = useEditorStore((s) => s.setCursorInfo)
  const pendingReveal = useEditorStore((s) => s.pendingReveal)
  const clearPendingReveal = useEditorStore((s) => s.clearPendingReveal)

  const handleSave = useCallback(async () => {
    if (!editorRef.current) return
    const content = editorRef.current.getValue()
    try {
      await api.updateProjectFile(projectId, tab.filePath, content)
      // Sync store content before marking clean to prevent the useEffect
      // from reverting the editor to stale store content
      useEditorStore.getState().updateContent(tab.filePath, content)
      markClean(tab.filePath)
    } catch (err) {
      console.error('Save failed:', err)
    }
  }, [projectId, tab.filePath, markClean])

  const handleMount: OnMount = useCallback((editor) => {
    editorRef.current = editor

    // Register Ctrl+S
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
      handleSave()
    })

    // Register custom command palette actions
    editor.addAction({
      id: 'editor.action.splitRight',
      label: 'Split Editor Right',
      keybindings: [],
      run: () => onSplit?.('horizontal'),
    })
    editor.addAction({
      id: 'editor.action.splitDown',
      label: 'Split Editor Down',
      keybindings: [],
      run: () => onSplit?.('vertical'),
    })
    editor.addAction({
      id: 'editor.action.closePane',
      label: 'Close Pane',
      keybindings: [],
      run: () => closeSplitPane(paneId),
    })

    // Track cursor position for status bar
    editor.onDidChangeCursorPosition((e) => {
      const sel = editor.getSelection()
      let selectedLines = 0
      let selectedChars = 0
      if (sel && !sel.isEmpty()) {
        const selectedText = editor.getModel()?.getValueInRange(sel) || ''
        selectedChars = selectedText.length
        selectedLines = sel.endLineNumber - sel.startLineNumber + 1
      }
      setCursorInfo({
        line: e.position.lineNumber,
        column: e.position.column,
        selectedLines,
        selectedChars,
      })
    })

  }, [handleSave, paneId, onSplit, closeSplitPane, setCursorInfo])

  // Handle pending reveal (e.g. from search result clicks)
  useEffect(() => {
    if (pendingReveal && pendingReveal.filePath === tab.filePath && editorRef.current) {
      const editor = editorRef.current
      editor.revealLineInCenter(pendingReveal.line)
      editor.setPosition({ lineNumber: pendingReveal.line, column: pendingReveal.column })
      editor.focus()
      clearPendingReveal()
    }
  }, [pendingReveal, tab.filePath, clearPendingReveal])

  // Update editor content when tab content changes externally (conflict resolution)
  useEffect(() => {
    if (editorRef.current && tab.content !== null) {
      const currentValue = editorRef.current.getValue()
      if (currentValue !== tab.content && !tab.dirty) {
        editorRef.current.setValue(tab.content)
      }
    }
  }, [tab.content, tab.dirty])

  const handleChange = useCallback((value: string | undefined) => {
    if (value !== undefined) {
      markDirty(tab.filePath)
    }
  }, [tab.filePath, markDirty])

  if (tab.content === null) {
    return <div className="monaco-loading">Loading...</div>
  }

  return (
    <div className="monaco-editor-wrapper">
      {tab.conflictContent !== null && (
        <div className="conflict-bar">
          <span className="material-symbols-outlined" style={{ fontSize: '16px' }}>warning</span>
          <span>File modified externally.</span>
          <button onClick={() => resolveConflict(tab.filePath, 'load-theirs')}>Reload</button>
          <button onClick={() => resolveConflict(tab.filePath, 'keep-mine')}>Keep Mine</button>
        </div>
      )}
      <Editor
        path={tab.filePath}
        height="100%"
        language={getLanguage(tab.filePath)}
        theme={theme === 'dark' ? 'vs-dark' : 'vs'}
        value={tab.content}
        onChange={handleChange}
        onMount={handleMount}
        options={{
          minimap: { enabled: true },
          folding: true,
          lineNumbers: 'on',
          bracketPairColorization: { enabled: true },
          automaticLayout: true,
          scrollBeyondLastLine: false,
          fontSize: 14,
          wordWrap: 'off',
          renderWhitespace: 'selection',
          smoothScrolling: true,
          cursorBlinking: 'smooth',
          cursorSmoothCaretAnimation: 'on',
          suggestOnTriggerCharacters: false,
          quickSuggestions: false,
          parameterHints: { enabled: false },
          inlineSuggest: { enabled: true },
        }}
      />
    </div>
  )
}
