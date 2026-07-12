import { Navigate, Route, Routes } from 'react-router-dom'
import { AppLayout } from './layouts/AppLayout'
import { ChatPage } from './pages/Chat/ChatPage'
import { DocumentsPage } from './pages/Documents/DocumentsPage'
import { LoginPage } from './pages/Login/LoginPage'
import { RuntimeLogsPage } from './pages/RuntimeLogs/RuntimeLogsPage'
import { SettingsPasswordPage } from './pages/Settings/SettingsPasswordPage'
import { SettingsPage } from './pages/Settings/SettingsPage'
import { SubjectsPage } from './pages/Subjects/SubjectsPage'

type AppProps = {
  authToken: string
  onAuthChange: (token: string) => void
  isDarkMode: boolean
  onToggleTheme: () => void
}

export default function App({ authToken, onAuthChange, isDarkMode, onToggleTheme }: AppProps) {
  const isAuthenticated = Boolean(authToken)

  return (
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
          <AppLayout
            isDarkMode={isDarkMode}
            onToggleTheme={onToggleTheme}
            onLogout={() => onAuthChange('')}
          />
        }
      >
        <Route
          path="/subjects"
          element={isAuthenticated ? <SubjectsPage /> : <Navigate to="/login" replace />}
        />
        <Route
          path="/documents"
          element={isAuthenticated ? <DocumentsPage /> : <Navigate to="/login" replace />}
        />
        <Route
          path="/chat"
          element={isAuthenticated ? <ChatPage /> : <Navigate to="/login" replace />}
        />
        <Route
          path="/logs"
          element={isAuthenticated ? <RuntimeLogsPage /> : <Navigate to="/login" replace />}
        />
        <Route
          path="/settings/profile"
          element={isAuthenticated ? <SettingsPage /> : <Navigate to="/login" replace />}
        />
        <Route
          path="/settings/security"
          element={isAuthenticated ? <SettingsPasswordPage /> : <Navigate to="/login" replace />}
        />
        <Route
          path="/settings"
          element={<Navigate to={isAuthenticated ? '/settings/profile' : '/login'} replace />}
        />
      </Route>
      <Route path="*" element={<Navigate to={isAuthenticated ? '/chat' : '/login'} replace />} />
    </Routes>
  )
}
