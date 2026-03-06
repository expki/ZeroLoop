import { useEditorStore } from '../stores/editorStore'

const LANG_DISPLAY: Record<string, string> = {
  javascript: 'JavaScript',
  typescript: 'TypeScript',
  typescriptreact: 'TypeScript React',
  html: 'HTML',
  css: 'CSS',
  json: 'JSON',
  markdown: 'Markdown',
  python: 'Python',
  xml: 'XML',
  yaml: 'YAML',
  cpp: 'C/C++',
  java: 'Java',
  rust: 'Rust',
  sql: 'SQL',
  php: 'PHP',
  go: 'Go',
  shell: 'Shell',
  dockerfile: 'Dockerfile',
  plaintext: 'Plain Text',
}

function getLanguageFromPath(filePath: string | null): string {
  if (!filePath) return 'Plain Text'
  const ext = filePath.split('.').pop()?.toLowerCase()
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

export default function StatusBar() {
  const cursorInfo = useEditorStore((s) => s.cursorInfo)
  const activeFile = useEditorStore((s) => s.getActiveFile())

  const langId = getLanguageFromPath(activeFile)
  const langName = LANG_DISPLAY[langId] || langId

  return (
    <div className="status-bar">
      <div className="status-bar-left">
        {activeFile && (
          <>
            <span className="status-item">
              Ln {cursorInfo.line}, Col {cursorInfo.column}
            </span>
            {cursorInfo.selectedChars > 0 && (
              <span className="status-item">
                ({cursorInfo.selectedChars} selected{cursorInfo.selectedLines > 1 ? `, ${cursorInfo.selectedLines} lines` : ''})
              </span>
            )}
          </>
        )}
      </div>
      <div className="status-bar-right">
        {activeFile && (
          <>
            <span className="status-item">Spaces: 2</span>
            <span className="status-item">UTF-8</span>
            <span className="status-item">{langName}</span>
          </>
        )}
      </div>
    </div>
  )
}
