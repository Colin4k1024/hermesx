import { lazy, Suspense } from 'react'
import { createHashRouter, Navigate } from 'react-router-dom'
import { useAuthStore } from '@shared/stores/authStore'
import { PageSkeleton } from '@shared/components/PageSkeleton'
import { ErrorBoundary } from '@shared/components/ErrorBoundary'
import App from './App'
import AdminShell from './components/AdminShell'

const Login = lazy(() => import('./pages/Login'))
const Bootstrap = lazy(() => import('./pages/Bootstrap'))
const Dashboard = lazy(() => import('./pages/Dashboard'))
const Tenants = lazy(() => import('./pages/Tenants'))
const Users = lazy(() => import('./pages/Users'))
const ApiKeys = lazy(() => import('./pages/ApiKeys'))
const AuditLogs = lazy(() => import('./pages/AuditLogs'))
const Pricing = lazy(() => import('./pages/Pricing'))
const Sandbox = lazy(() => import('./pages/Sandbox'))
const SystemSettings = lazy(() => import('./pages/SystemSettings'))

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
  return <AdminShell />
}

export const router = createHashRouter([
  {
    element: <App />,
    children: [
      { path: '/login', element: <Lazy><Login /></Lazy> },
      { path: '/bootstrap', element: <Lazy><Bootstrap /></Lazy> },
      {
        element: <AuthGuard />,
        children: [
          { path: '/dashboard', element: <Lazy><Dashboard /></Lazy> },
          { path: '/tenants', element: <Lazy><Tenants /></Lazy> },
          { path: '/users', element: <Lazy><Users /></Lazy> },
          { path: '/keys', element: <Lazy><ApiKeys /></Lazy> },
          { path: '/audit', element: <Lazy><AuditLogs /></Lazy> },
          { path: '/pricing', element: <Lazy><Pricing /></Lazy> },
          { path: '/sandbox', element: <Lazy><Sandbox /></Lazy> },
          { path: '/settings', element: <Lazy><SystemSettings /></Lazy> },
        ],
      },
      { path: '*', element: <Navigate to="/dashboard" replace /> },
    ],
  },
])
