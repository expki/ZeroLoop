import { useState, useCallback, useEffect, useRef, useMemo } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { useEditorStore } from '../stores/editorStore'

function getFileIcon(name: string): string {
  const ext = name.split('.').pop()?.toLowerCase()
  switch (ext) {
    case 'ts': case 'mts': case 'cts': case 'tsx': return 'brand_awareness'
    case 'js': case 'mjs': case 'cjs': case 'jsx': return 'javascript'
    case 'json': return 'data_object'
    case 'html': case 'htm': return 'html'
    case 'css': case 'scss': case 'less': return 'css'
    case 'md': case 'mdx': return 'article'
    case 'py': return 'code'
    case 'go': case 'rs': case 'java': return 'code'
    case 'sql': return 'database'
    case 'yaml': case 'yml': return 'settings'
    case 'sh': case 'bash': return 'terminal'
    case 'dockerfile': return 'deployed_code'
    case 'png': case 'jpg': case 'jpeg': case 'gif': case 'webp': return 'image'
    default: return 'description'
  }
}

export default function QuickOpen() {
  const visible = useEditorStore((s) => s.quickOpenVisible)
  const toggleQuickOpen = useEditorStore((s) => s.toggleQuickOpen)
  const openTab = useEditorStore((s) => s.openTab)
  const files = useProjectStore((s) => s.files)
  const [query, setQuery] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)

  // Filter files (exclude directories)
  const filtered = useMemo(() => {
    const nonDirs = files.filter((f) => !f.is_dir)
    if (!query) return nonDirs.slice(0, 50)

    const q = query.toLowerCase()
    return nonDirs
      .filter((f) => {
        const name = f.name.toLowerCase()
        const path = f.path.toLowerCase()
        return name.includes(q) || path.includes(q)
      })
      .sort((a, b) => {
        // Prefer name matches over path matches
        const aName = a.name.toLowerCase().includes(q)
        const bName = b.name.toLowerCase().includes(q)
        if (aName && !bName) return -1
        if (!aName && bName) return 1
        return a.path.localeCompare(b.path)
      })
      .slice(0, 50)
  }, [files, query])

  useEffect(() => {
    if (visible) {
      setQuery('')
      setSelectedIndex(0)
      setTimeout(() => inputRef.current?.focus(), 0)
    }
  }, [visible])

  useEffect(() => {
    setSelectedIndex(0)
  }, [query])

  const handleSelect = useCallback((filePath: string) => {
    openTab(filePath)
    toggleQuickOpen()
  }, [openTab, toggleQuickOpen])

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      toggleQuickOpen()
    } else if (e.key === 'ArrowDown') {
      e.preventDefault()
      setSelectedIndex((i) => Math.min(i + 1, filtered.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setSelectedIndex((i) => Math.max(i - 1, 0))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (filtered[selectedIndex]) {
        handleSelect(filtered[selectedIndex].path)
      }
    }
  }, [filtered, selectedIndex, handleSelect, toggleQuickOpen])

  if (!visible) return null

  return (
    <div className="quick-open-overlay" onClick={toggleQuickOpen}>
      <div className="quick-open" onClick={(e) => e.stopPropagation()}>
        <input
          ref={inputRef}
          className="quick-open-input"
          type="text"
          placeholder="Search files..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={handleKeyDown}
        />
        <div className="quick-open-results">
          {filtered.map((file, index) => (
            <div
              key={file.path}
              className={`quick-open-item ${index === selectedIndex ? 'selected' : ''}`}
              onClick={() => handleSelect(file.path)}
              onMouseEnter={() => setSelectedIndex(index)}
            >
              <span className="material-symbols-outlined quick-open-icon">{getFileIcon(file.name)}</span>
              <span className="quick-open-name">{file.name}</span>
              <span className="quick-open-path">{file.path}</span>
            </div>
          ))}
          {filtered.length === 0 && (
            <div className="quick-open-empty">No files found</div>
          )}
        </div>
      </div>
    </div>
  )
}
