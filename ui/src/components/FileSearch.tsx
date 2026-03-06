import { useState, useCallback, useRef, useEffect } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { useEditorStore } from '../stores/editorStore'
import { api } from '../services/api'

interface SearchResult {
  path: string
  line: number
  column: number
  content: string
}

interface GroupedResults {
  [path: string]: SearchResult[]
}

export default function FileSearch() {
  const visible = useEditorStore((s) => s.searchPanelVisible)
  const toggleSearchPanel = useEditorStore((s) => s.toggleSearchPanel)
  const revealLine = useEditorStore((s) => s.revealLine)
  const selectedProjectId = useProjectStore((s) => s.selectedProjectId)
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<GroupedResults>({})
  const [searching, setSearching] = useState(false)
  const [totalCount, setTotalCount] = useState(0)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (visible) {
      setTimeout(() => inputRef.current?.focus(), 0)
    }
  }, [visible])

  const doSearch = useCallback(async (q: string) => {
    if (!q || !selectedProjectId) {
      setResults({})
      setTotalCount(0)
      return
    }
    setSearching(true)
    try {
      const matches = await api.searchProjectFiles(selectedProjectId, q, 100)
      const grouped: GroupedResults = {}
      for (const match of matches) {
        if (!grouped[match.path]) grouped[match.path] = []
        grouped[match.path].push(match)
      }
      setResults(grouped)
      setTotalCount(matches.length)
    } catch {
      setResults({})
      setTotalCount(0)
    } finally {
      setSearching(false)
    }
  }, [selectedProjectId])

  const handleChange = useCallback((value: string) => {
    setQuery(value)
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => doSearch(value), 300)
  }, [doSearch])

  const handleResultClick = useCallback((path: string, line: number, column: number) => {
    revealLine(path, line, column)
  }, [revealLine])

  if (!visible) return null

  return (
    <div className="file-search-panel">
      <div className="file-search-header">
        <span className="file-search-title">Search in Files</span>
        <button className="icon-button" onClick={toggleSearchPanel} title="Close">
          <span className="material-symbols-outlined" style={{ fontSize: '16px' }}>close</span>
        </button>
      </div>
      <input
        ref={inputRef}
        className="file-search-input"
        type="text"
        placeholder="Search..."
        value={query}
        onChange={(e) => handleChange(e.target.value)}
      />
      {searching && <div className="file-search-status">Searching...</div>}
      {!searching && totalCount > 0 && (
        <div className="file-search-status">{totalCount} results in {Object.keys(results).length} files</div>
      )}
      <div className="file-search-results">
        {Object.entries(results).map(([path, matches]) => (
          <div key={path} className="file-search-group">
            <div className="file-search-file">{path}</div>
            {matches.map((match, i) => (
              <div
                key={i}
                className="file-search-match"
                onClick={() => handleResultClick(path, match.line, match.column)}
              >
                <span className="match-line-num">{match.line}</span>
                <span className="match-content">{match.content.trim()}</span>
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}
