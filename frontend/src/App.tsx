import { lazy, Suspense } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { AppLayout } from './layouts/AppLayout'

const ChatPage = lazy(() => import('./pages/Chat/ChatPage').then((module) => ({ default: module.ChatPage })))
const DocumentsPage = lazy(() => import('./pages/Documents/DocumentsPage').then((module) => ({ default: module.DocumentsPage })))
const LoginPage = lazy(() => import('./pages/Login/LoginPage').then((module) => ({ default: module.LoginPage })))
const RuntimeLogsPage = lazy(() => import('./pages/RuntimeLogs/RuntimeLogsPage').then((module) => ({ default: module.RuntimeLogsPage })))
const SettingsPasswordPage = lazy(() => import('./pages/Settings/SettingsPasswordPage').then((module) => ({ default: module.SettingsPasswordPage })))
const SettingsPage = lazy(() => import('./pages/Settings/SettingsPage').then((module) => ({ default: module.SettingsPage })))
const SubjectsPage = lazy(() => import('./pages/Subjects/SubjectsPage').then((module) => ({ default: module.SubjectsPage })))

type AppProps = {
  authToken: string
  onAuthChange: (token: string) => void
  isDarkMode: boolean
  onToggleTheme: () => void
}

export default function App({ authToken, onAuthChange, isDarkMode, onToggleTheme }: AppProps) {
  const isAuthenticated = Boolean(authToken)

  return (
    <Suspense fallback={null}>
      <Routes>
        <Route
          path="/login"
          element={
            isAuthenticated ? (
              <Navigate to="/chat" replace />
            ) : (
              <LoginPage
                isDarkMode={isDarkMode}
                onToggleTheme={onToggleTheme}
                onAuthSuccess={onAuthChange}
              />
            )
          }
        />
        <Route
          element={
            isAuthenticated ? (
              <AppLayout
                isDarkMode={isDarkMode}
                onToggleTheme={onToggleTheme}
                onLogout={() => onAuthChange('')}
              />
            ) : (
              <Navigate to="/login" replace />
            )
          }
        >
          <Route path="/subjects" element={<SubjectsPage />} />
          <Route path="/documents" element={<DocumentsPage />} />
          <Route path="/chat" element={<ChatPage />} />
          <Route path="/logs" element={<RuntimeLogsPage />} />
          <Route path="/settings/profile" element={<SettingsPage />} />
          <Route path="/settings/security" element={<SettingsPasswordPage />} />
          <Route path="/settings" element={<Navigate to="/settings/profile" replace />} />
        </Route>
        <Route path="*" element={<Navigate to={isAuthenticated ? '/chat' : '/login'} replace />} />
      </Routes>
    </Suspense>
  )
}
