import { useEffect, useCallback } from 'react'
import '../lib/monacoSetup' // Must be first — configures local Monaco loader
import { useProjectStore } from '../stores/projectStore'
import { useEditorStore } from '../stores/editorStore'
import SplitPaneLayout from './SplitPane'
import Breadcrumbs from './Breadcrumbs'
import QuickOpen from './QuickOpen'
import FileSearch from './FileSearch'
import StatusBar from './StatusBar'
import './MonacoIDE.css'

export default function MonacoIDE() {
  const selectedProjectId = useProjectStore((s) => s.selectedProjectId)
  const setMainView = useProjectStore((s) => s.setMainView)
  const activeFile = useEditorStore((s) => s.getActiveFile())
  const toggleQuickOpen = useEditorStore((s) => s.toggleQuickOpen)
  const toggleSearchPanel = useEditorStore((s) => s.toggleSearchPanel)
  const searchPanelVisible = useEditorStore((s) => s.searchPanelVisible)
  const initForProject = useEditorStore((s) => s.initForProject)
  const closeTab = useEditorStore((s) => s.closeTab)
  const layout = useEditorStore((s) => s.layout)

  // Initialize editor store for current project
  useEffect(() => {
    if (selectedProjectId) {
      initForProject(selectedProjectId)
    }
  }, [selectedProjectId, initForProject])

  // Global keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ctrl+P — Quick Open (override browser print)
      if ((e.ctrlKey || e.metaKey) && e.key === 'p' && !e.shiftKey) {
        e.preventDefault()
        toggleQuickOpen()
        return
      }
      // Ctrl+Shift+F — File Search panel toggle
      if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'F') {
        e.preventDefault()
        toggleSearchPanel()
        return
      }
      // Ctrl+W — Close active tab
      if ((e.ctrlKey || e.metaKey) && e.key === 'w') {
        e.preventDefault()
        const firstPane = layout.panes[0]
        if (firstPane?.activeTabId) {
          closeTab(firstPane.activeTabId, firstPane.id)
        }
        return
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [toggleQuickOpen, toggleSearchPanel, closeTab, layout])

  const handleBackToAgent = useCallback(() => {
    setMainView({ type: 'agent' })
  }, [setMainView])

  if (!selectedProjectId) return null

  return (
    <div className="monaco-ide">
      <div className="monaco-ide-toolbar">
        <button className="icon-button" onClick={handleBackToAgent} title="Back to agent">
          <span className="material-symbols-outlined">arrow_back</span>
        </button>
        <Breadcrumbs filePath={activeFile} />
        <div className="monaco-ide-toolbar-actions">
          <button
            className={`icon-button ${searchPanelVisible ? 'active' : ''}`}
            onClick={toggleSearchPanel}
            title="Search in Files (Ctrl+Shift+F)"
          >
            <span className="material-symbols-outlined">search</span>
          </button>
        </div>
      </div>
      <div className="monaco-ide-body">
        <div className="monaco-ide-editors">
          <SplitPaneLayout projectId={selectedProjectId} />
        </div>
        {searchPanelVisible && <FileSearch />}
      </div>
      <StatusBar />
      <QuickOpen />
    </div>
  )
}
