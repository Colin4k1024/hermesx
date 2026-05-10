import { Navigate, Outlet } from 'react-router-dom'
import { useAuthStore } from '@shared/stores/authStore'

interface Props {
  redirectTo?: string
}

export function ProtectedRoute({ redirectTo = '/login' }: Props) {
  const connected = useAuthStore((s) => s.connected)
  if (!connected) return <Navigate to={redirectTo} replace />
  return <Outlet />
}
