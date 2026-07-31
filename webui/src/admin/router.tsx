import { createHashRouter, Navigate } from 'react-router'
import { Lazy, AuthGuard, lazy } from '@shared/components/RouteHelpers'
import App from './App'
import AdminShell from './components/AdminShell'

const Login = lazy(() => import('./pages/Login'))
const Bootstrap = lazy(() => import('./pages/Bootstrap'))
const Dashboard = lazy(() => import('./pages/Dashboard'))
const Tenants = lazy(() => import('./pages/Tenants'))
const Users = lazy(() => import('./pages/Users'))
const ApiKeys = lazy(() => import('./pages/ApiKeys'))
const AuditLogs = lazy(() => import('./pages/AuditLogs'))
const Receipts = lazy(() => import('./pages/Receipts'))
const WorkflowRuns = lazy(() => import('./pages/WorkflowRuns'))
const Pricing = lazy(() => import('./pages/Pricing'))
const Sandbox = lazy(() => import('./pages/Sandbox'))
const Security = lazy(() => import('./pages/Security'))
const Governance = lazy(() => import('./pages/Governance'))
const ChannelApps = lazy(() => import('./pages/ChannelApps'))
const SystemSettings = lazy(() => import('./pages/SystemSettings'))

export const router = createHashRouter([
  {
    element: <App />,
    children: [
      { path: '/login', element: <Lazy><Login /></Lazy> },
      { path: '/bootstrap', element: <Lazy><Bootstrap /></Lazy> },
      {
        element: <AuthGuard shell={<AdminShell />} />,
        children: [
          { path: '/dashboard', element: <Lazy><Dashboard /></Lazy> },
          { path: '/tenants', element: <Lazy><Tenants /></Lazy> },
          { path: '/users', element: <Lazy><Users /></Lazy> },
          { path: '/keys', element: <Lazy><ApiKeys /></Lazy> },
          { path: '/audit', element: <Lazy><AuditLogs /></Lazy> },
          { path: '/receipts', element: <Lazy><Receipts /></Lazy> },
          { path: '/workflows', element: <Lazy><WorkflowRuns /></Lazy> },
          { path: '/pricing', element: <Lazy><Pricing /></Lazy> },
          { path: '/sandbox', element: <Lazy><Sandbox /></Lazy> },
          { path: '/security', element: <Lazy><Security /></Lazy> },
          { path: '/governance', element: <Lazy><Governance /></Lazy> },
          { path: '/channels', element: <Lazy><ChannelApps /></Lazy> },
          { path: '/settings', element: <Lazy><SystemSettings /></Lazy> },
        ],
      },
      { path: '*', element: <Navigate to="/dashboard" replace /> },
    ],
  },
])
