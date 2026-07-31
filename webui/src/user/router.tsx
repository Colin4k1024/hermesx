import { createHashRouter, Navigate } from 'react-router'
import { Lazy, AuthGuard, lazy } from '@shared/components/RouteHelpers'
import App from './App'
import UserShell from './components/UserShell'

const Login = lazy(() => import('./pages/Login'))
const Register = lazy(() => import('./pages/Register'))
const Chat = lazy(() => import('./pages/Chat'))
const Workspace = lazy(() => import('./pages/Workspace'))
const Memories = lazy(() => import('./pages/Memories'))
const Skills = lazy(() => import('./pages/Skills'))
const Usage = lazy(() => import('./pages/Usage'))
const Settings = lazy(() => import('./pages/Settings'))
const Notifications = lazy(() => import('./pages/Notifications'))
const Agents = lazy(() => import('./pages/Agents'))

export const router = createHashRouter([
  {
    element: <App />,
    children: [
      { path: '/login', element: <Lazy><Login /></Lazy> },
      { path: '/register', element: <Lazy><Register /></Lazy> },
      {
        element: <AuthGuard shell={<UserShell />} />,
        children: [
          { path: '/chat', element: <Lazy><Chat /></Lazy> },
          { path: '/workspace', element: <Lazy><Workspace /></Lazy> },
          { path: '/memories', element: <Lazy><Memories /></Lazy> },
          { path: '/skills', element: <Lazy><Skills /></Lazy> },
          { path: '/agents', element: <Lazy><Agents /></Lazy> },
          { path: '/usage', element: <Lazy><Usage /></Lazy> },
          { path: '/settings', element: <Lazy><Settings /></Lazy> },
          { path: '/notifications', element: <Lazy><Notifications /></Lazy> },
        ],
      },
      { path: '*', element: <Navigate to="/login" replace /> },
    ],
  },
])
