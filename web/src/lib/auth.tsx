import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { api, setToken, type Principal } from './api'

interface AuthState {
  user: Principal | null
  ready: boolean
  can: (permission: string) => boolean
  signIn: (user: Principal, token: string) => void
  signOut: () => Promise<void>
}

const Ctx = createContext<AuthState>(null as unknown as AuthState)
export const useAuth = () => useContext(Ctx)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<Principal | null>(null)
  const [ready, setReady] = useState(false)

  // On load, try the refresh cookie. A staff member who reloads the page
  // should not have to type a TOTP code again.
  useEffect(() => {
    (async () => {
      try {
        const res = await fetch('/api/v1/auth/refresh', { method: 'POST', credentials: 'same-origin' })
        if (res.ok) {
          const body = await res.json()
          setToken(body.access_token)
          setUser(body.user)
        }
      } catch { /* no session; the login screen handles it */ }
      setReady(true)
    })()
  }, [])

  const value: AuthState = {
    user,
    ready,
    // Permissions come from the server on every login and refresh. The UI
    // hides what a user cannot do; the SERVER is what refuses it. Hiding a
    // button is a courtesy, never a control.
    can: (p) => !!user && user.permissions.includes(p),
    signIn: (u, t) => { setToken(t); setUser(u) },
    signOut: async () => {
      try { await api('/auth/logout', { method: 'POST' }) } catch { /* already gone */ }
      setToken(null)
      setUser(null)
    },
  }
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}
