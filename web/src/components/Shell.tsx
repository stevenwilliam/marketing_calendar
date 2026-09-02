import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '../lib/auth'
import { useState } from 'react'

interface NavItem { to: string; label: string; permission?: string }

const NAV: NavItem[] = [
  { to: '/kalender',    label: 'Kalender',    permission: 'promo.view' },
  { to: '/promo',       label: 'Rencana promo', permission: 'promo.view' },
  { to: '/persetujuan', label: 'Kotak persetujuan', permission: 'promo.view' },
  { to: '/target',      label: 'Target',      permission: 'target.view' },
  { to: '/laporan',     label: 'Laporan',     permission: 'report.view' },
  { to: '/impor',       label: 'Impor',       permission: 'import.view' },
  { to: '/master',      label: 'Master data', permission: 'site.manage' },
  { to: '/pengguna',    label: 'Pengguna',    permission: 'user.manage' },
  { to: '/pengaturan',  label: 'Pengaturan',  permission: 'settings.manage' },
  { to: '/audit',       label: 'Audit',       permission: 'audit.view' },
]

export default function Shell() {
  const { user, can, signOut } = useAuth()
  const nav = useNavigate()
  const [open, setOpen] = useState(false)

  // The sidebar shows only what this user can reach. The server refuses the
  // rest regardless; hiding it keeps the nav honest rather than offering
  // doors that do not open.
  const items = NAV.filter((n) => !n.permission || can(n.permission))
  const roles = [...new Set(user?.grants.map((g) => g.role_code) ?? [])].join(', ')

  return (
    <div className="min-h-screen md:grid" style={{ gridTemplateColumns: '236px minmax(0,1fr)' }}>
      <button
        className="md:hidden w-full text-left bg-ink text-bg px-5 py-3 font-extrabold"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-controls="sidebar"
      >
        MARKETING CALENDAR <span aria-hidden="true" className="float-right">{open ? '✕' : '☰'}</span>
      </button>

      <nav
        id="sidebar"
        className={`bg-ink text-bg py-5 flex-col ${open ? 'flex' : 'hidden'} md:flex`}
        aria-label="Navigasi utama"
      >
        <div className="px-5 pb-4 font-extrabold text-[15px] leading-tight tracking-tight hidden md:block">
          MARKETING<br />CALENDAR
        </div>
        <div className="h-0.5 mb-3.5 hidden md:block" style={{ background: 'rgba(255,255,255,0.14)' }} />

        {items.map((n) => (
          <NavLink
            key={n.to}
            to={n.to}
            onClick={() => setOpen(false)}
            className={({ isActive }) =>
              `flex items-center gap-2.5 px-5 py-2.5 text-[13px] font-semibold border-l-[3px] ` +
              (isActive
                ? 'text-white border-l-accent'
                : 'border-l-transparent hover:text-white')
            }
            style={({ isActive }) => ({
              color: isActive ? '#ffffff' : 'rgba(243,242,242,0.72)',
              background: isActive ? 'rgba(236,48,19,0.14)' : undefined,
            })}
          >
            {n.label}
          </NavLink>
        ))}

        <div className="mt-auto px-5 pt-4" style={{ borderTop: '2px solid rgba(255,255,255,0.14)' }}>
          <div className="font-semibold text-xs">{user?.full_name}</div>
          {/* 62%, not 45%: the guideline's value measured 4.06 and failed AA. */}
          <div className="text-[11px]" style={{ color: 'rgba(243,242,242,0.62)' }}>
            {roles}{user?.is_superadmin ? ' · superadmin' : ''}
          </div>
          <button
            className="text-[13px] font-semibold mt-2 hover:text-white"
            style={{ color: 'rgba(243,242,242,0.72)' }}
            onClick={async () => { await signOut(); nav('/') }}
          >
            Keluar
          </button>
        </div>
      </nav>

      <main className="min-w-0 p-4 md:p-7">
        <Outlet />
      </main>
    </div>
  )
}
