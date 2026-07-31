import { lazy, Suspense } from 'react'
import { Navigate } from 'react-router'
import { useAuthStore } from '@shared/stores/authStore'
import { PageSkeleton } from '@shared/components/PageSkeleton'
import { ErrorBoundary } from '@shared/components/ErrorBoundary'

export function Lazy({ children }: { children: React.ReactNode }) {
  return (
    <ErrorBoundary>
      <Suspense fallback={<PageSkeleton />}>{children}</Suspense>
    </ErrorBoundary>
  )
}

export function AuthGuard({ shell }: { shell: React.ReactNode }) {
  const connected = useAuthStore((s) => s.connected)
  if (!connected) return <Navigate to="/login" replace />
  return <>{shell}</>
}

export { lazy }
