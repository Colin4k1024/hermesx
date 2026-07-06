import { useState, useEffect, useCallback } from 'react'
import { Splitter } from 'antd'
import { useWorkspaceStore } from '@shared/stores/workspaceStore'
import {
  WORKSPACE_SIDEBAR_WIDTH,
  WORKSPACE_SIDEBAR_MIN,
  WORKSPACE_SIDEBAR_MAX,
  WORKSPACE_RESULTS_WIDTH,
  WORKSPACE_RESULTS_MIN,
  WORKSPACE_RESULTS_MAX,
  WORKSPACE_DIALOG_MIN,
} from '@shared/theme/constants'
import { TaskSidebar } from './TaskSidebar'
import { DialogArea } from './DialogArea'
import { ResultsPanel } from './ResultsPanel'

export function WorkspaceLayout() {
  const sidebarCollapsed = useWorkspaceStore((s) => s.sidebarCollapsed)
  const resultsPanelCollapsed = useWorkspaceStore((s) => s.resultsPanelCollapsed)
  const toggleSidebar = useWorkspaceStore((s) => s.toggleSidebar)
  const toggleResultsPanel = useWorkspaceStore((s) => s.toggleResultsPanel)

  const [isNarrow, setIsNarrow] = useState(window.innerWidth < 1024)

  useEffect(() => {
    const handler = () => setIsNarrow(window.innerWidth < 1024)
    window.addEventListener('resize', handler)
    return () => window.removeEventListener('resize', handler)
  }, [])

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (!(e.metaKey || e.ctrlKey)) return
      if (e.key === 'b') { e.preventDefault(); toggleSidebar() }
      else if (e.key === 'e') { e.preventDefault(); toggleResultsPanel() }
    },
    [toggleSidebar, toggleResultsPanel],
  )

  useEffect(() => {
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [handleKeyDown])

  const effectiveSidebarCollapsed = isNarrow || sidebarCollapsed

  return (
    <Splitter style={{ height: '100vh' }}>
      <Splitter.Panel
        size={effectiveSidebarCollapsed ? 0 : WORKSPACE_SIDEBAR_WIDTH}
        min={WORKSPACE_SIDEBAR_MIN}
        max={WORKSPACE_SIDEBAR_MAX}
        collapsible
      >
        <TaskSidebar />
      </Splitter.Panel>
      <Splitter.Panel min={WORKSPACE_DIALOG_MIN}>
        <DialogArea />
      </Splitter.Panel>
      <Splitter.Panel
        size={resultsPanelCollapsed ? 0 : WORKSPACE_RESULTS_WIDTH}
        min={WORKSPACE_RESULTS_MIN}
        max={WORKSPACE_RESULTS_MAX}
        collapsible
      >
        <ResultsPanel />
      </Splitter.Panel>
    </Splitter>
  )
}
