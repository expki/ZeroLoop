import { useState } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { useUIStore } from '../stores/uiStore'
import './WelcomeScreen.css'
import './ProjectList.css'

function ProjectList() {
  const { projects, selectProject, createProject, deleteProject } = useProjectStore()
  const { sidebarOpen, toggleSidebar } = useUIStore()
  const [showNew, setShowNew] = useState(false)
  const [newName, setNewName] = useState('')
  const [newDesc, setNewDesc] = useState('')

  const handleCreate = () => {
    const name = newName.trim()
    if (!name) return
    createProject(name, newDesc.trim())
    setNewName('')
    setNewDesc('')
    setShowNew(false)
  }

  return (
    <div className="project-list-screen">
      {!sidebarOpen && (
        <button className="welcome-menu-btn icon-button" onClick={toggleSidebar} title="Open sidebar">
          <span className="material-symbols-outlined">menu</span>
        </button>
      )}
      <div className="project-list-content">
        <div className="project-list-hero">
          <span className="welcome-logo material-symbols-outlined">hub</span>
          <h1 className="welcome-title">ZeroLoop</h1>
          <p className="welcome-subtitle">Select or create a project</p>
        </div>

        <button className="new-project-btn" onClick={() => setShowNew(true)}>
          <span className="material-symbols-outlined">add</span>
          New Project
        </button>

        {showNew && (
          <div className="new-project-form">
            <input
              className="project-input"
              placeholder="Project name"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
              autoFocus
            />
            <input
              className="project-input"
              placeholder="Description (optional)"
              value={newDesc}
              onChange={(e) => setNewDesc(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
            />
            <div className="new-project-actions">
              <button className="btn-primary" onClick={handleCreate}>Create</button>
              <button className="btn-secondary" onClick={() => setShowNew(false)}>Cancel</button>
            </div>
          </div>
        )}

        <div className="project-grid">
          {projects.map((project) => (
            <div key={project.id} className="project-card" onClick={() => selectProject(project.id)}>
              <div className="project-card-header">
                <span className="material-symbols-outlined project-card-icon">folder</span>
                <button
                  className="project-card-delete icon-button"
                  onClick={(e) => { e.stopPropagation(); deleteProject(project.id) }}
                  title="Delete project"
                >
                  <span className="material-symbols-outlined">close</span>
                </button>
              </div>
              <span className="project-card-name">{project.name}</span>
              {project.description && (
                <span className="project-card-desc">{project.description}</span>
              )}
              <span className="project-card-date">
                {new Date(project.created_at).toLocaleDateString()}
              </span>
            </div>
          ))}
          {projects.length === 0 && !showNew && (
            <div className="project-list-empty">No projects yet. Create one to get started.</div>
          )}
        </div>
      </div>
    </div>
  )
}

export default ProjectList
