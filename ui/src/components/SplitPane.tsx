import { useCallback, useRef, useState } from 'react'
import { useEditorStore } from '../stores/editorStore'
import EditorTabs from './EditorTabs'
import MonacoEditor from './MonacoEditor'

interface SplitPaneProps {
  projectId: string
}

export default function SplitPaneLayout({ projectId }: SplitPaneProps) {
  const layout = useEditorStore((s) => s.layout)
  const tabs = useEditorStore((s) => s.tabs)
  const splitPane = useEditorStore((s) => s.splitPane)
  const toggleQuickOpen = useEditorStore((s) => s.toggleQuickOpen)
  const [dragging, setDragging] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const sizesRef = useRef<number[]>(layout.sizes)

  sizesRef.current = layout.sizes

  const handleDividerMouseDown = useCallback((index: number) => {
    setDragging(true)
    const isHorizontal = layout.direction === 'horizontal'

    const onMouseMove = (e: MouseEvent) => {
      if (!containerRef.current) return
      const rect = containerRef.current.getBoundingClientRect()
      const total = isHorizontal ? rect.width : rect.height
      const pos = isHorizontal ? e.clientX - rect.left : e.clientY - rect.top
      const pct = (pos / total) * 100

      const newSizes = [...sizesRef.current]
      // Compute cumulative sizes up to divider
      let cumBefore = 0
      for (let i = 0; i < index; i++) cumBefore += newSizes[i]

      const sizeA = Math.max(10, pct - cumBefore)
      const sizeB = Math.max(10, newSizes[index] + newSizes[index + 1] - sizeA)
      newSizes[index] = sizeA
      newSizes[index + 1] = sizeB

      useEditorStore.setState((s) => ({
        layout: { ...s.layout, sizes: newSizes },
      }))
    }

    const onMouseUp = () => {
      setDragging(false)
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
    }

    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
  }, [layout.direction])

  const handleSplit = useCallback((paneId: string, direction: 'horizontal' | 'vertical') => {
    splitPane(paneId, direction)
  }, [splitPane])

  return (
    <div
      ref={containerRef}
      className={`split-pane-container ${layout.direction} ${dragging ? 'dragging' : ''}`}
    >
      {layout.panes.map((pane, index) => {
        const paneTabs = pane.tabs
          .map((tabId) => tabs.get(tabId))
          .filter((t): t is NonNullable<typeof t> => t !== undefined)
        const activeTab = pane.activeTabId ? tabs.get(pane.activeTabId) : undefined

        return (
          <div key={pane.id} className="split-pane" style={{ flexBasis: `${layout.sizes[index]}%` }}>
            <EditorTabs
              paneId={pane.id}
              tabs={paneTabs}
              activeTabId={pane.activeTabId}
              onQuickOpen={toggleQuickOpen}
            />
            <div className="pane-editor-area">
              {activeTab ? (
                <MonacoEditor
                  tab={activeTab}
                  paneId={pane.id}
                  projectId={projectId}
                  onSplit={(dir) => handleSplit(pane.id, dir)}
                />
              ) : (
                <div className="pane-empty">
                  <span className="material-symbols-outlined" style={{ fontSize: '48px', opacity: 0.4 }}>code</span>
                  <p>Open a file to start editing</p>
                  <div className="pane-empty-shortcuts">
                    <span><kbd>Ctrl+P</kbd> Quick Open</span>
                    <span><kbd>Ctrl+S</kbd> Save File</span>
                    <span><kbd>Ctrl+Shift+F</kbd> Search Files</span>
                    <span><kbd>F1</kbd> Command Palette</span>
                    <span><kbd>Ctrl+G</kbd> Go to Line</span>
                  </div>
                </div>
              )}
            </div>
            {index < layout.panes.length - 1 && (
              <div
                className={`split-divider ${layout.direction}`}
                onMouseDown={() => handleDividerMouseDown(index)}
              />
            )}
          </div>
        )
      })}
    </div>
  )
}
