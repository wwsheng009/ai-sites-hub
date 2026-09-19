import { Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import Sites from './pages/Sites'
import SiteDetail from './pages/SiteDetail'
import Affiliates from './pages/Affiliates'
import Events from './pages/Events'
import Announcements from './pages/Announcements'
import Models from './pages/Models'
import UsageLogs from './pages/UsageLogs'

export default function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/sites" element={<Sites />} />
        <Route path="/sites/:id" element={<SiteDetail />} />
        <Route path="/sites/:id/usage/logs" element={<UsageLogs />} />
        <Route path="/events" element={<Events />} />
        <Route path="/affiliates" element={<Affiliates />} />
        <Route path="/announcements" element={<Announcements />} />
        <Route path="/models" element={<Models />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Layout>
  )
}
