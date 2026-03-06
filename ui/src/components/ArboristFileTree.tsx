import { useState, useRef, useCallback, useMemo } from 'react'
import { Tree, NodeRendererProps, NodeApi, TreeApi } from 'react-arborist'
import { useProjectStore } from '../stores/projectStore'
import { api } from '../services/api'
import ContextMenu, { ContextMenuItem } from './ContextMenu'
import type { ArboristNode } from '../types'
import './ArboristFileTree.css'

interface ArboristFileTreeProps {
  projectId: string
}

interface ClipboardState {
  paths: string[]
  mode: 'cut' | 'copy'
}

function Node({ node, style, dragHandle }: NodeRendererProps<ArboristNode>) {
  const isEditing = node.isEditing

  return (
    <div
      ref={dragHandle}
      style={style}
      className={`arb-node ${node.isSelected ? 'selected' : ''} ${node.willReceiveDrop ? 'drop-target' : ''} ${node.isEditing ? 'editing' : ''}`}
      onClick={(e) => {
        e.stopPropagation()
        if (node.data.isDir) {
          node.toggle()
        } else {
          node.activate()
        }
      }}
    >
      {node.data.isDir && (
        <span className={`material-symbols-outlined arb-arrow ${node.isOpen ? 'expanded' : ''}`}>
          chevron_right
        </span>
      )}
      <span className="material-symbols-outlined arb-icon">
        {node.data.isDir ? (node.isOpen ? 'folder_open' : 'folder') : 'description'}
      </span>
      {isEditing ? (
        <input
          className="arb-rename-input"
          autoFocus
          defaultValue={node.data.name}
          onBlur={() => node.reset()}
          onKeyDown={(e) => {
            if (e.key === 'Enter') node.submit(e.currentTarget.value)
            if (e.key === 'Escape') node.reset()
          }}
          onClick={(e) => e.stopPropagation()}
        />
      ) : (
        <span className={`arb-name ${node.data.isDir ? 'dir-name' : ''}`}>
          {node.data.name}
        </span>
      )}
    </div>
  )
}

function ArboristFileTree({ projectId }: ArboristFileTreeProps) {
  const files = useProjectStore((s) => s.files)
  const openFile = useProjectStore((s) => s.openFile)
  const loadFiles = useProjectStore((s) => s.loadFiles)
  const getArboristTree = useProjectStore((s) => s.getArboristTree)
  const treeData = useMemo(() => getArboristTree(), [files]) // eslint-disable-line react-hooks/exhaustive-deps

  const [clipboard, setClipboard] = useState<ClipboardState | null>(null)
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; node: NodeApi<ArboristNode> | null } | null>(null)
  const uploadRef = useRef<HTMLInputElement>(null)
  const uploadTargetDir = useRef<string>('')
  const treeRef = useRef<TreeApi<ArboristNode> | null>(null)

  const reload = useCallback(() => loadFiles(projectId), [loadFiles, projectId])

  // --- Handlers for react-arborist ---

  const handleActivate = (node: NodeApi<ArboristNode>) => {
    if (!node.data.isDir) {
      openFile(node.id)
    }
  }

  const handleMove = async ({ dragIds, parentId }: { dragIds: string[]; parentId: string | null }) => {
    for (const dragId of dragIds) {
      const name = dragId.split('/').pop() || dragId
      const newPath = parentId ? `${parentId}/${name}` : name
      if (newPath !== dragId) {
        try {
          await api.moveProjectFile(projectId, dragId, newPath)
        } catch (err) {
          console.error('Move failed:', err)
        }
      }
    }
    reload()
  }

  const handleRename = async ({ id, name }: { id: string; name: string; node: NodeApi<ArboristNode> }) => {
    const parts = id.split('/')
    parts[parts.length - 1] = name
    const newPath = parts.join('/')
    if (newPath !== id) {
      try {
        await api.moveProjectFile(projectId, id, newPath)
      } catch (err) {
        console.error('Rename failed:', err)
      }
      reload()
    }
  }

  const handleCreate = async ({ parentId, type }: { parentId: string | null; index: number; type: 'internal' | 'leaf' }) => {
    const isDir = type === 'internal'
    const defaultName = isDir ? 'new-folder' : 'new-file'
    const path = parentId ? `${parentId}/${defaultName}` : defaultName
    try {
      if (isDir) {
        await api.createProjectDir(projectId, path)
      } else {
        await api.createProjectFile(projectId, path, '')
      }
      reload()
    } catch (err) {
      console.error('Create failed:', err)
    }
    return { id: path }
  }

  const handleDelete = async ({ ids }: { ids: string[] }) => {
    const names = ids.map((id) => id.split('/').pop()).join(', ')
    if (!window.confirm(`Delete ${ids.length > 1 ? `${ids.length} items` : `"${names}"`}?`)) return
    for (const id of ids) {
      try {
        await api.deleteProjectFile(projectId, id)
      } catch (err) {
        console.error('Delete failed:', err)
      }
    }
    reload()
  }

  // --- Context menu ---

  const handleContextMenu = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()

    // Find which node was right-clicked
    const target = e.target as HTMLElement
    const nodeEl = target.closest('[data-testid]') || target.closest('.arb-node')
    let clickedNode: NodeApi<ArboristNode> | null = null

    if (treeRef.current) {
      // Try to find node from the focused/most-recent node
      const tree = treeRef.current
      if (tree.focusedNode && nodeEl) {
        clickedNode = tree.focusedNode
      }
      // Use selectedNodes if available
      const selected = tree.selectedNodes
      if (selected && selected.length === 1) {
        clickedNode = selected[0]
      }
    }

    setContextMenu({ x: e.clientX, y: e.clientY, node: clickedNode })
  }

  const getContextMenuItems = (): ContextMenuItem[] => {
    const node = contextMenu?.node
    const targetDir = node?.data.isDir ? node.id : node ? node.id.split('/').slice(0, -1).join('/') || '' : ''

    const items: ContextMenuItem[] = [
      {
        label: 'New File',
        icon: 'note_add',
        action: async () => {
          const name = prompt('File name:')
          if (!name) return
          const path = targetDir ? `${targetDir}/${name}` : name
          try {
            await api.createProjectFile(projectId, path, '')
            reload()
          } catch (err) {
            console.error('Create file failed:', err)
          }
        },
      },
      {
        label: 'New Folder',
        icon: 'create_new_folder',
        action: async () => {
          const name = prompt('Folder name:')
          if (!name) return
          const path = targetDir ? `${targetDir}/${name}` : name
          try {
            await api.createProjectDir(projectId, path)
            reload()
          } catch (err) {
            console.error('Create folder failed:', err)
          }
        },
      },
    ]

    if (node) {
      items.push(
        {
          label: 'Rename',
          icon: 'edit',
          divider: true,
          action: () => {
            node.edit()
          },
        },
        {
          label: 'Delete',
          icon: 'delete',
          action: async () => {
            const msg = node.data.isDir
              ? `Delete folder "${node.data.name}" and all its contents?`
              : `Delete "${node.data.name}"?`
            if (!window.confirm(msg)) return
            try {
              await api.deleteProjectFile(projectId, node.id)
              reload()
            } catch (err) {
              console.error('Delete failed:', err)
            }
          },
        },
        {
          label: 'Cut',
          icon: 'content_cut',
          divider: true,
          action: () => {
            const ids = treeRef.current?.selectedIds ?? new Set()
            const paths = ids.size > 0 ? [...ids] as string[] : [node.id]
            setClipboard({ paths, mode: 'cut' })
          },
        },
        {
          label: 'Copy',
          icon: 'content_copy',
          action: () => {
            const ids = treeRef.current?.selectedIds ?? new Set()
            const paths = ids.size > 0 ? [...ids] as string[] : [node.id]
            setClipboard({ paths, mode: 'copy' })
          },
        },
      )
    }

    items.push({
      label: 'Paste',
      icon: 'content_paste',
      disabled: !clipboard,
      divider: !node,
      action: async () => {
        if (!clipboard) return
        for (const path of clipboard.paths) {
          const name = path.split('/').pop() || path
          const dest = targetDir ? `${targetDir}/${name}` : name
          try {
            if (clipboard.mode === 'cut') {
              await api.moveProjectFile(projectId, path, dest)
            } else {
              // Copy: read then create
              const res = await api.readProjectFile(projectId, path)
              await api.createProjectFile(projectId, dest, res.content)
            }
          } catch (err) {
            console.error('Paste failed:', err)
          }
        }
        if (clipboard.mode === 'cut') setClipboard(null)
        reload()
      },
    })

    items.push({
      label: 'Upload Here',
      icon: 'upload',
      divider: true,
      action: () => {
        uploadTargetDir.current = targetDir
        uploadRef.current?.click()
      },
    })

    return items
  }

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files || e.target.files.length === 0) return
    const dir = uploadTargetDir.current

    // Create a new FormData with adjusted filenames
    const formData = new FormData()
    for (let i = 0; i < e.target.files.length; i++) {
      const file = e.target.files[i]
      const path = dir ? `${dir}/${file.name}` : file.name
      formData.append('files', file, path)
    }

    try {
      const res = await fetch(`/api/projects/${projectId}/upload`, {
        method: 'POST',
        body: formData,
      })
      if (!res.ok) throw new Error('Upload failed')
      reload()
    } catch (err) {
      console.error('Upload failed:', err)
    }
    e.target.value = ''
  }

  const disableDrop = ({ parentNode, dragNodes }: { parentNode: NodeApi<ArboristNode>; dragNodes: NodeApi<ArboristNode>[]; index: number }) => {
    // Only allow dropping into directories
    if (parentNode && !parentNode.data.isDir && parentNode.id !== null) return true
    // Prevent dropping into self
    for (const drag of dragNodes) {
      if (parentNode && parentNode.id.startsWith(drag.id + '/')) return true
    }
    return false
  }

  return (
    <div className="arb-tree-container" onContextMenu={handleContextMenu}>
      {treeData.length === 0 ? (
        <div className="arb-empty" onContextMenu={handleContextMenu}>
          No files yet. Right-click to create.
        </div>
      ) : (
        <Tree
          ref={treeRef}
          data={treeData}
          idAccessor="id"
          childrenAccessor="children"
          openByDefault={false}
          width="100%"
          height={files.length * 28}
          rowHeight={28}
          indent={16}
          onActivate={handleActivate}
          onMove={handleMove}
          onRename={handleRename}
          onCreate={handleCreate}
          onDelete={handleDelete}
          disableDrop={disableDrop}
          className="arb-tree"
        >
          {(props) => <Node {...props} />}
        </Tree>
      )}

      <input
        ref={uploadRef}
        type="file"
        multiple
        style={{ display: 'none' }}
        onChange={handleUpload}
      />

      {contextMenu && (
        <ContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          items={getContextMenuItems()}
          onClose={() => setContextMenu(null)}
        />
      )}
    </div>
  )
}

export default ArboristFileTree
