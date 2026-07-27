import { Outlet } from 'react-router'
import { ErrorBoundary } from '@shared/components'

export default function App() {
  return (
    <ErrorBoundary>
      <Outlet />
    </ErrorBoundary>
  )
}
