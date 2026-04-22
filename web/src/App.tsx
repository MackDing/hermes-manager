import { Routes, Route } from 'react-router-dom'
import { ErrorBoundary } from './components/ErrorBoundary'
import { Layout } from './components/Layout'
import { DashboardPage } from './pages/DashboardPage'
import { SkillsPage } from './pages/SkillsPage'
import { EventsPage } from './pages/EventsPage'

function App() {
  return (
    <ErrorBoundary>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<DashboardPage />} />
          <Route path="skills" element={<SkillsPage />} />
          <Route path="events" element={<EventsPage />} />
        </Route>
      </Routes>
    </ErrorBoundary>
  )
}

export default App
