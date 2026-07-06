import { Splitter } from 'antd'
import { TaskSidebar } from './TaskSidebar'
import { DialogArea } from './DialogArea'
import { ResultsPanel } from './ResultsPanel'

export function WorkspaceLayout() {
  return (
    <Splitter style={{ height: '100vh' }}>
      <Splitter.Panel defaultSize={260} min={200} collapsible>
        <TaskSidebar />
      </Splitter.Panel>
      <Splitter.Panel min={400}>
        <DialogArea />
      </Splitter.Panel>
      <Splitter.Panel defaultSize={360} min={280} collapsible>
        <ResultsPanel />
      </Splitter.Panel>
    </Splitter>
  )
}
