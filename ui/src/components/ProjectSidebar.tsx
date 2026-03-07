import { useState, useEffect } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { useAgentStore } from '../stores/agentStore'
import { useTerminalStore } from '../stores/terminalStore'
import { useProcessStore } from '../stores/processStore'
import { useUIStore } from '../stores/uiStore'
import ArboristFileTree from './ArboristFileTree'
import type { AgentWidth, DetailMode } from '../types'
import './Sidebar.css'
import './ProjectSidebar.css'

function ProjectSidebar() {
  const { selectedProjectId, selectProject, projects, loadFiles } = useProjectStore()
  const { agents, selectedAgentId, selectAgent, createAgent, deleteAgent, loadAgentsForProject } = useAgentStore()
  const { terminals, selectedTerminalId, selectTerminal, createTerminal, deleteTerminal, loadTerminalsForProject } = useTerminalStore()
  const { processes, selectedProcessId, selectProcess, stopProcess, loadProcessesForProject } = useProcessStore()
  const {
    sidebarOpen, toggleSidebar, theme, toggleTheme,
    agentWidth, setAgentWidth, detailMode, setDetailMode, setSidebarOpen,
  } = useUIStore()
  const [prefsOpen, setPrefsOpen] = useState(false)

  const project = projects.find((p) => p.id === selectedProjectId)

  // Load agents, terminals, and files when project changes
  useEffect(() => {
    if (selectedProjectId) {
      loadAgentsForProject(selectedProjectId)
      loadTerminalsForProject(selectedProjectId)
      loadProcessesForProject(selectedProjectId)
      loadFiles(selectedProjectId)
    }
  }, [selectedProjectId, loadAgentsForProject, loadTerminalsForProject, loadProcessesForProject, loadFiles])

  const handleSelectAgent = (id: string) => {
    selectAgent(id)
    useProjectStore.getState().setMainView({ type: 'agent' })
    if (window.innerWidth < 768) {
      setSidebarOpen(false)
    }
  }

  const handleNewAgent = () => {
    if (selectedProjectId) {
      createAgent(selectedProjectId)
      useProjectStore.getState().setMainView({ type: 'agent' })
    }
  }

  const handleSelectTerminal = (id: string) => {
    selectTerminal(id)
    useProjectStore.getState().setMainView({ type: 'terminal' })
    if (window.innerWidth < 768) {
      setSidebarOpen(false)
    }
  }

  const handleNewTerminal = () => {
    if (selectedProjectId) {
      createTerminal(selectedProjectId)
      useProjectStore.getState().setMainView({ type: 'terminal' })
    }
  }

  const handleSelectProcess = (id: string) => {
    selectProcess(id)
    useProjectStore.getState().setMainView({ type: 'process' })
    if (window.innerWidth < 768) {
      setSidebarOpen(false)
    }
  }

  const widthOptions: { label: string; value: AgentWidth }[] = [
    { label: 'S', value: '40em' },
    { label: 'M', value: '55em' },
    { label: 'L', value: '80em' },
    { label: 'Full', value: 'full' },
  ]

  const detailOptions: { label: string; value: DetailMode }[] = [
    { label: 'Hide', value: 'collapsed' },
    { label: 'List', value: 'list' },
    { label: 'Current', value: 'current' },
    { label: 'All', value: 'expanded' },
  ]

  return (
    <aside className={`sidebar ${sidebarOpen ? 'open' : 'closed'}`}>
      <div className="sidebar-header">
        <button className="sidebar-brand" onClick={() => selectProject(null)}>
          <span className="brand-icon material-symbols-outlined">hub</span>
          <span className="brand-text">ZeroLoop</span>
        </button>
        <button className="icon-button" onClick={toggleSidebar} title="Toggle sidebar">
          <span className="material-symbols-outlined">menu</span>
        </button>
      </div>

      {project && (
        <div className="project-name-bar">
          <span className="material-symbols-outlined" style={{ fontSize: '18px' }}>folder</span>
          <span className="project-name-text">{project.name}</span>
        </div>
      )}

      {/* Files section */}
      <div className="sidebar-section">
        <div className="sidebar-section-header">
          <span className="sidebar-section-title">Files</span>
        </div>
        {selectedProjectId && <ArboristFileTree projectId={selectedProjectId} />}
      </div>

      {/* Terminals section */}
      <div className="sidebar-section">
        <div className="sidebar-section-header">
          <span className="sidebar-section-title">Terminals</span>
          <button className="icon-button" onClick={handleNewTerminal} title="New terminal">
            <span className="material-symbols-outlined" style={{ fontSize: '18px' }}>add</span>
          </button>
        </div>
        <div className="terminal-list">
          {terminals.length === 0 ? (
            <div className="terminal-list-empty">No terminals yet</div>
          ) : (
            terminals.map((terminal) => (
              <div
                key={terminal.id}
                className={`terminal-item ${selectedTerminalId === terminal.id ? 'active' : ''}`}
                onClick={() => handleSelectTerminal(terminal.id)}
              >
                <span className="material-symbols-outlined terminal-icon">terminal</span>
                <span className="terminal-name">{terminal.name}</span>
                <button
                  className="terminal-close icon-button"
                  onClick={(e) => { e.stopPropagation(); deleteTerminal(terminal.id) }}
                  title="Remove terminal"
                >
                  <span className="material-symbols-outlined">close</span>
                </button>
              </div>
            ))
          )}
        </div>
      </div>

      {/* Processes section */}
      <div className="sidebar-section">
        <div className="sidebar-section-header">
          <span className="sidebar-section-title">Processes</span>
        </div>
        <div className="process-list">
          {processes.length === 0 ? (
            <div className="process-list-empty">No processes yet</div>
          ) : (
            processes.map((process) => (
              <div
                key={process.id}
                className={`process-item ${selectedProcessId === process.id ? 'active' : ''}`}
                onClick={() => handleSelectProcess(process.id)}
              >
                <div className={`process-status-dot ${process.status === 'running' ? 'running' : process.status === 'stopped' ? 'stopped' : ''}`} />
                <span className="process-name">{process.command}</span>
                {process.status === 'running' && (
                  <button
                    className="process-close icon-button"
                    onClick={(e) => { e.stopPropagation(); stopProcess(process.id) }}
                    title="Stop process"
                  >
                    <span className="material-symbols-outlined">stop</span>
                  </button>
                )}
              </div>
            ))
          )}
        </div>
      </div>

      {/* Agents section */}
      <div className="sidebar-section sidebar-section-grow">
        <div className="sidebar-section-header">
          <span className="sidebar-section-title">Agents</span>
          <button className="icon-button" onClick={handleNewAgent} title="New agent">
            <span className="material-symbols-outlined" style={{ fontSize: '18px' }}>add</span>
          </button>
        </div>
        <div className="agent-list">
          {agents.length === 0 ? (
            <div className="agent-list-empty">No agents yet</div>
          ) : (
            agents.map((agent) => (
              <div
                key={agent.id}
                className={`agent-item ${selectedAgentId === agent.id ? 'active' : ''}`}
                onClick={() => handleSelectAgent(agent.id)}
              >
                <div className={`agent-status-dot ${agent.running ? 'running' : ''}`} />
                <span className="agent-name">{agent.name}</span>
                <button
                  className="agent-close icon-button"
                  onClick={(e) => { e.stopPropagation(); deleteAgent(agent.id) }}
                  title="Remove agent"
                >
                  <span className="material-symbols-outlined">close</span>
                </button>
              </div>
            ))
          )}
        </div>
      </div>

      <div className="sidebar-bottom">
        <div className="prefs-section">
          <button className="prefs-toggle" onClick={() => setPrefsOpen(!prefsOpen)}>
            <span className="material-symbols-outlined">settings</span>
            <span>Preferences</span>
            <span className={`material-symbols-outlined chevron ${prefsOpen ? 'open' : ''}`}>
              expand_more
            </span>
          </button>

          {prefsOpen && (
            <div className="prefs-content">
              <div className="pref-row">
                <span className="pref-label">Theme</span>
                <button className="toggle-switch" onClick={toggleTheme}>
                  <span className="material-symbols-outlined" style={{ fontSize: '16px' }}>
                    {theme === 'dark' ? 'dark_mode' : 'light_mode'}
                  </span>
                  <span>{theme === 'dark' ? 'Dark' : 'Light'}</span>
                </button>
              </div>

              <div className="pref-row">
                <span className="pref-label">Width</span>
                <div className="button-group">
                  {widthOptions.map((opt) => (
                    <button
                      key={opt.value}
                      className={`btn-group-item ${agentWidth === opt.value ? 'active' : ''}`}
                      onClick={() => setAgentWidth(opt.value)}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
              </div>

              <div className="pref-row">
                <span className="pref-label">Detail</span>
                <div className="button-group">
                  {detailOptions.map((opt) => (
                    <button
                      key={opt.value}
                      className={`btn-group-item ${detailMode === opt.value ? 'active' : ''}`}
                      onClick={() => setDetailMode(opt.value)}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>

        <div className="sidebar-footer">
          <span className="version-text">ZeroLoop v0.1.0</span>
        </div>
      </div>
    </aside>
  )
}

export default ProjectSidebar
