import { lazy, Suspense } from 'react'
import { createHashRouter, Navigate } from 'react-router-dom'
import { useAuthStore } from '@shared/stores/authStore'
import { PageSkeleton } from '@shared/components/PageSkeleton'
import { ErrorBoundary } from '@shared/components/ErrorBoundary'
import App from './App'
import UserShell from './components/UserShell'

const Login = lazy(() => import('./pages/Login'))
const Chat = lazy(() => import('./pages/Chat'))
const Memories = lazy(() => import('./pages/Memories'))
const Skills = lazy(() => import('./pages/Skills'))
const Usage = lazy(() => import('./pages/Usage'))
const Settings = lazy(() => import('./pages/Settings'))
const Notifications = lazy(() => import('./pages/Notifications'))

function Lazy({ children }: { children: React.ReactNode }) {
  return (
    <ErrorBoundary>
      <Suspense fallback={<PageSkeleton />}>{children}</Suspense>
    </ErrorBoundary>
  )
}

function AuthGuard() {
  const connected = useAuthStore((s) => s.connected)
  if (!connected) return <Navigate to="/login" replace />
  return <UserShell />
}

export const router = createHashRouter([
  {
    element: <App />,
    children: [
      { path: '/login', element: <Lazy><Login /></Lazy> },
      {
        element: <AuthGuard />,
        children: [
          { path: '/chat', element: <Lazy><Chat /></Lazy> },
          { path: '/memories', element: <Lazy><Memories /></Lazy> },
          { path: '/skills', element: <Lazy><Skills /></Lazy> },
          { path: '/usage', element: <Lazy><Usage /></Lazy> },
          { path: '/settings', element: <Lazy><Settings /></Lazy> },
          { path: '/notifications', element: <Lazy><Notifications /></Lazy> },
        ],
      },
      { path: '*', element: <Navigate to="/login" replace /> },
    ],
  },
])
