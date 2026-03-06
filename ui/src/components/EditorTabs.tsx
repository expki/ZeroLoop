import { useCallback, useState } from 'react'
import { useEditorStore, type EditorTab } from '../stores/editorStore'

interface EditorTabsProps {
  paneId: string
  tabs: EditorTab[]
  activeTabId: string | null
  onQuickOpen: () => void
}

interface ContextMenu {
  x: number
  y: number
  filePath: string
}

function getFileIcon(filePath: string): string {
  const ext = filePath.split('.').pop()?.toLowerCase()
  switch (ext) {
    case 'ts': case 'mts': case 'cts': return 'brand_awareness'
    case 'tsx': return 'brand_awareness'
    case 'js': case 'mjs': case 'cjs': return 'javascript'
    case 'jsx': return 'javascript'
    case 'json': return 'data_object'
    case 'html': case 'htm': return 'html'
    case 'css': case 'scss': case 'less': return 'css'
    case 'md': case 'mdx': return 'article'
    case 'py': return 'code'
    case 'go': return 'code'
    case 'rs': return 'code'
    case 'java': return 'coffee'
    case 'sql': return 'database'
    case 'yaml': case 'yml': return 'settings'
    case 'xml': case 'svg': return 'code'
    case 'sh': case 'bash': return 'terminal'
    case 'dockerfile': return 'deployed_code'
    case 'png': case 'jpg': case 'jpeg': case 'gif': case 'webp': return 'image'
    default: return 'description'
  }
}

export default function EditorTabs({ paneId, tabs, activeTabId, onQuickOpen }: EditorTabsProps) {
  const setActiveTab = useEditorStore((s) => s.setActiveTab)
  const closeTab = useEditorStore((s) => s.closeTab)
  const closeOtherTabs = useEditorStore((s) => s.closeOtherTabs)
  const closeAllTabs = useEditorStore((s) => s.closeAllTabs)
  const closeSavedTabs = useEditorStore((s) => s.closeSavedTabs)
  const [contextMenu, setContextMenu] = useState<ContextMenu | null>(null)

  const handleClick = useCallback((filePath: string) => {
    setActiveTab(filePath, paneId)
  }, [paneId, setActiveTab])

  const handleClose = useCallback((e: React.MouseEvent, filePath: string) => {
    e.stopPropagation()
    closeTab(filePath, paneId)
  }, [paneId, closeTab])

  const handleMiddleClick = useCallback((e: React.MouseEvent, filePath: string) => {
    if (e.button === 1) {
      e.preventDefault()
      closeTab(filePath, paneId)
    }
  }, [paneId, closeTab])

  const handleContextMenu = useCallback((e: React.MouseEvent, filePath: string) => {
    e.preventDefault()
    setContextMenu({ x: e.clientX, y: e.clientY, filePath })
  }, [])

  const handleDragStart = useCallback((e: React.DragEvent, filePath: string) => {
    e.dataTransfer.setData('text/plain', JSON.stringify({ filePath, fromPaneId: paneId }))
    e.dataTransfer.effectAllowed = 'move'
  }, [paneId])

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    try {
      const data = JSON.parse(e.dataTransfer.getData('text/plain'))
      if (data.fromPaneId && data.filePath && data.fromPaneId !== paneId) {
        useEditorStore.getState().moveTab(data.filePath, data.fromPaneId, paneId)
      }
    } catch {
      // Ignore
    }
  }, [paneId])

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
  }, [])

  const closeContextMenu = useCallback(() => setContextMenu(null), [])

  return (
    <>
      <div className="editor-tabs" onDrop={handleDrop} onDragOver={handleDragOver}>
        {tabs.map((tab) => {
          const fileName = tab.filePath.split('/').pop() || tab.filePath
          const isActive = tab.filePath === activeTabId
          const icon = getFileIcon(tab.filePath)
          return (
            <div
              key={tab.filePath}
              className={`editor-tab ${isActive ? 'active' : ''} ${tab.dirty ? 'dirty' : ''}`}
              onClick={() => handleClick(tab.filePath)}
              onMouseDown={(e) => handleMiddleClick(e, tab.filePath)}
              onContextMenu={(e) => handleContextMenu(e, tab.filePath)}
              draggable
              onDragStart={(e) => handleDragStart(e, tab.filePath)}
              title={tab.filePath}
            >
              <span className="material-symbols-outlined tab-icon">{icon}</span>
              <span className="tab-name">{fileName}</span>
              {tab.dirty && <span className="tab-dirty-dot" title="Unsaved changes" />}
              {tab.conflictContent !== null && (
                <span className="material-symbols-outlined tab-conflict-icon" title="File modified externally">warning</span>
              )}
              <button
                className="tab-close"
                onClick={(e) => handleClose(e, tab.filePath)}
                title="Close"
              >
                <span className="material-symbols-outlined" style={{ fontSize: '14px' }}>close</span>
              </button>
            </div>
          )
        })}
        <button className="tab-add" onClick={onQuickOpen} title="Open File (Ctrl+P)">
          <span className="material-symbols-outlined" style={{ fontSize: '16px' }}>add</span>
        </button>
      </div>
      {contextMenu && (
        <>
          <div className="context-menu-backdrop" onClick={closeContextMenu} />
          <div
            className="context-menu"
            style={{ left: contextMenu.x, top: contextMenu.y }}
          >
            <button onClick={() => { closeTab(contextMenu.filePath, paneId); closeContextMenu() }}>
              Close
            </button>
            <button onClick={() => { closeOtherTabs(contextMenu.filePath, paneId); closeContextMenu() }}>
              Close Others
            </button>
            <button onClick={() => { closeSavedTabs(paneId); closeContextMenu() }}>
              Close Saved
            </button>
            <div className="context-menu-separator" />
            <button onClick={() => { closeAllTabs(paneId); closeContextMenu() }}>
              Close All
            </button>
          </div>
        </>
      )}
    </>
  )
}
