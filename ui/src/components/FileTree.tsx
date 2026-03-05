import { useState } from 'react'
import type { FileTreeNode } from '../types'
import './FileTree.css'

interface FileTreeProps {
  nodes: FileTreeNode[]
  onSelect: (path: string) => void
  selectedPath?: string
}

function FileTreeItem({ node, depth, onSelect, selectedPath }: {
  node: FileTreeNode
  depth: number
  onSelect: (path: string) => void
  selectedPath?: string
}) {
  const [expanded, setExpanded] = useState(depth < 2)

  if (node.isDir) {
    return (
      <div className="file-tree-dir">
        <div
          className="file-tree-item"
          style={{ paddingLeft: `${depth * 12 + 8}px` }}
          onClick={() => setExpanded(!expanded)}
        >
          <span className={`material-symbols-outlined file-tree-arrow ${expanded ? 'expanded' : ''}`}>
            chevron_right
          </span>
          <span className="material-symbols-outlined file-tree-icon">folder</span>
          <span className="file-tree-name">{node.name}</span>
        </div>
        {expanded && node.children && (
          <div className="file-tree-children">
            {node.children.map((child) => (
              <FileTreeItem
                key={child.path}
                node={child}
                depth={depth + 1}
                onSelect={onSelect}
                selectedPath={selectedPath}
              />
            ))}
          </div>
        )}
      </div>
    )
  }

  return (
    <div
      className={`file-tree-item file-tree-file ${selectedPath === node.path ? 'active' : ''}`}
      style={{ paddingLeft: `${depth * 12 + 28}px` }}
      onClick={() => onSelect(node.path)}
    >
      <span className="material-symbols-outlined file-tree-icon">description</span>
      <span className="file-tree-name">{node.name}</span>
    </div>
  )
}

function FileTree({ nodes, onSelect, selectedPath }: FileTreeProps) {
  if (nodes.length === 0) {
    return <div className="file-tree-empty">No files yet</div>
  }

  return (
    <div className="file-tree">
      {nodes.map((node) => (
        <FileTreeItem
          key={node.path}
          node={node}
          depth={0}
          onSelect={onSelect}
          selectedPath={selectedPath}
        />
      ))}
    </div>
  )
}

export default FileTree
