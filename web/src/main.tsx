import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import './index.css'
import { AuthProvider, useAuth } from './lib/auth'
import Shell from './components/Shell'
import Login from './pages/Login'
import Calendar from './pages/Calendar'
import Plans from './pages/Plans'
import PlanDetail from './pages/PlanDetail'
import PlanForm from './pages/PlanForm'
import Inbox from './pages/Inbox'
import Targets from './pages/Targets'
import Reports from './pages/Reports'
import Imports from './pages/Imports'
import MasterData from './pages/MasterData'
import Users from './pages/Users'
import Settings from './pages/Settings'
import Audit from './pages/Audit'
import { Loading } from './components/ui'

function App() {
  const { user, ready } = useAuth()
  if (!ready) return <Loading what="Memeriksa sesi…" />
  if (!user) return <Login />
  return (
    <Routes>
      <Route element={<Shell />}>
        <Route path="/" element={<Navigate to="/kalender" replace />} />
        <Route path="/kalender" element={<Calendar />} />
        <Route path="/promo" element={<Plans />} />
        <Route path="/promo/baru" element={<PlanForm />} />
        <Route path="/promo/:id" element={<PlanDetail />} />
        <Route path="/promo/:id/ubah" element={<PlanForm />} />
        <Route path="/persetujuan" element={<Inbox />} />
        <Route path="/target" element={<Targets />} />
        <Route path="/laporan" element={<Reports />} />
        <Route path="/impor" element={<Imports />} />
        <Route path="/master" element={<MasterData />} />
        <Route path="/pengguna" element={<Users />} />
        <Route path="/pengaturan" element={<Settings />} />
        <Route path="/audit" element={<Audit />} />
        <Route path="*" element={<Navigate to="/kalender" replace />} />
      </Route>
    </Routes>
  )
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <App />
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>,
)
