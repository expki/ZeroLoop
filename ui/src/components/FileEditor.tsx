import { useEffect, useRef, useState, useCallback } from 'react'
import { EditorState } from '@codemirror/state'
import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter } from '@codemirror/view'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { syntaxHighlighting, defaultHighlightStyle, bracketMatching } from '@codemirror/language'
import { oneDark } from '@codemirror/theme-one-dark'
import { javascript } from '@codemirror/lang-javascript'
import { html } from '@codemirror/lang-html'
import { css } from '@codemirror/lang-css'
import { json } from '@codemirror/lang-json'
import { markdown } from '@codemirror/lang-markdown'
import { python } from '@codemirror/lang-python'
import { xml } from '@codemirror/lang-xml'
import { yaml } from '@codemirror/lang-yaml'
import { cpp } from '@codemirror/lang-cpp'
import { java } from '@codemirror/lang-java'
import { rust } from '@codemirror/lang-rust'
import { sql } from '@codemirror/lang-sql'
import { php } from '@codemirror/lang-php'
import { go } from '@codemirror/lang-go'
import { useProjectStore } from '../stores/projectStore'
import { useUIStore } from '../stores/uiStore'
import { api } from '../services/api'
import './FileEditor.css'

function getLanguageExtension(filename: string) {
  const ext = filename.split('.').pop()?.toLowerCase()
  switch (ext) {
    case 'js': case 'mjs': case 'cjs': return javascript()
    case 'ts': case 'mts': case 'cts': return javascript({ typescript: true })
    case 'jsx': return javascript({ jsx: true })
    case 'tsx': return javascript({ jsx: true, typescript: true })
    case 'html': case 'htm': return html()
    case 'css': case 'scss': case 'less': return css()
    case 'json': return json()
    case 'md': case 'mdx': return markdown()
    case 'py': return python()
    case 'xml': case 'svg': return xml()
    case 'yaml': case 'yml': return yaml()
    case 'c': case 'h': case 'cpp': case 'hpp': case 'cc': return cpp()
    case 'java': return java()
    case 'rs': return rust()
    case 'sql': return sql()
    case 'php': return php()
    case 'go': return go()
    default: return []
  }
}

interface FileEditorProps {
  filePath: string
}

function FileEditor({ filePath }: FileEditorProps) {
  const editorRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<EditorView | null>(null)
  const selectedProjectId = useProjectStore((s) => s.selectedProjectId)
  const theme = useUIStore((s) => s.theme)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loaded, setLoaded] = useState(false)
  const contentRef = useRef('')

  const saveFile = useCallback(async () => {
    if (!selectedProjectId || !viewRef.current) return
    const content = viewRef.current.state.doc.toString()
    setSaving(true)
    setError(null)
    try {
      await api.updateProjectFile(selectedProjectId, filePath, content)
      contentRef.current = content
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }, [selectedProjectId, filePath])

  useEffect(() => {
    if (!editorRef.current || !selectedProjectId) return

    let destroyed = false
    setLoaded(false)
    setError(null)

    api.readProjectFile(selectedProjectId, filePath)
      .then(({ content }) => {
        if (destroyed || !editorRef.current) return
        contentRef.current = content

        const extensions = [
          lineNumbers(),
          highlightActiveLine(),
          highlightActiveLineGutter(),
          history(),
          bracketMatching(),
          syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
          keymap.of([
            ...defaultKeymap,
            ...historyKeymap,
            { key: 'Mod-s', run: () => { saveFile(); return true } },
          ]),
          getLanguageExtension(filePath),
          EditorView.lineWrapping,
          theme === 'dark' ? oneDark : [],
        ]

        const state = EditorState.create({ doc: content, extensions })
        const view = new EditorView({ state, parent: editorRef.current })
        viewRef.current = view
        setLoaded(true)
      })
      .catch((err) => {
        if (!destroyed) setError(err instanceof Error ? err.message : 'Failed to load file')
      })

    return () => {
      destroyed = true
      if (viewRef.current) {
        viewRef.current.destroy()
        viewRef.current = null
      }
    }
  }, [selectedProjectId, filePath, theme, saveFile])

  const setMainView = useProjectStore((s) => s.setMainView)

  return (
    <div className="file-editor">
      <div className="file-editor-toolbar">
        <button className="icon-button" onClick={() => setMainView({ type: 'chat' })} title="Back to chat">
          <span className="material-symbols-outlined">arrow_back</span>
        </button>
        <span className="file-editor-path">{filePath}</span>
        <div className="file-editor-actions">
          {saving && <span className="file-editor-status">Saving...</span>}
          {error && <span className="file-editor-error">{error}</span>}
          <button className="btn-primary" onClick={saveFile} disabled={saving}>
            <span className="material-symbols-outlined" style={{ fontSize: '16px' }}>save</span>
            Save
          </button>
        </div>
      </div>
      <div className="file-editor-container" ref={editorRef}>
        {!loaded && !error && <div className="file-editor-loading">Loading...</div>}
      </div>
    </div>
  )
}

export default FileEditor
