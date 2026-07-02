import { Navigate, Route, Routes } from 'react-router-dom'
import { getAuthToken } from './api/auth'
import { AppLayout } from './layouts/AppLayout'
import { DashboardPage } from './pages/Dashboard/DashboardPage'
import { ChatPage } from './pages/Chat/ChatPage'
import { DocumentsPage } from './pages/Documents/DocumentsPage'
import { LoginPage } from './pages/Login/LoginPage'
import { SubjectsPage } from './pages/Subjects/SubjectsPage'

type AppProps = {
  isDarkMode: boolean
  onToggleTheme: () => void
}

export default function App({ isDarkMode, onToggleTheme }: AppProps) {
  const isAuthenticated = Boolean(getAuthToken())

  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<AppLayout isDarkMode={isDarkMode} onToggleTheme={onToggleTheme} />}>
        <Route
          path="/dashboard"
          element={isAuthenticated ? <DashboardPage /> : <Navigate to="/login" replace />}
        />
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
      </Route>
      <Route path="*" element={<Navigate to={isAuthenticated ? '/dashboard' : '/login'} replace />} />
    </Routes>
  )
}
