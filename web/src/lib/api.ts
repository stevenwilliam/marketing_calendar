// The API client. One place that knows how to talk to the backend, so the
// access token, the refresh dance and the error shape are handled once.

export interface ApiError {
  code: string
  message: string
  fields?: Record<string, string>
  trace_id?: string
  overlaps?: Overlap[]
}

export interface Overlap {
  plan_id: string
  plan_code: string
  promo_name: string
  start_date: string
  end_date: string
  shared_site_count: number
}

export class ApiFailure extends Error {
  constructor(public status: number, public body: ApiError) {
    super(body.message || 'Terjadi kesalahan')
  }
}

let accessToken: string | null = null
export const setToken = (t: string | null) => { accessToken = t }
export const getToken = () => accessToken

// refreshing holds the in-flight refresh so ten parallel 401s trigger ONE
// refresh rather than ten, which would rotate the token ten times and revoke
// the family as a suspected theft.
let refreshing: Promise<boolean> | null = null

async function tryRefresh(): Promise<boolean> {
  if (!refreshing) {
    refreshing = (async () => {
      try {
        const res = await fetch('/api/v1/auth/refresh', {
          method: 'POST',
          credentials: 'same-origin',
        })
        if (!res.ok) return false
        const body = await res.json()
        accessToken = body.access_token
        return true
      } catch {
        return false
      } finally {
        // Cleared on the next tick so concurrent callers all see this result.
        setTimeout(() => { refreshing = null }, 0)
      }
    })()
  }
  return refreshing
}

export async function api<T = unknown>(
  path: string,
  opts: { method?: string; body?: unknown; retry?: boolean } = {},
): Promise<T> {
  const { method = 'GET', body, retry = true } = opts
  const headers: Record<string, string> = {}
  if (accessToken) headers.Authorization = `Bearer ${accessToken}`
  if (body !== undefined) headers['Content-Type'] = 'application/json'

  const res = await fetch(`/api/v1${path}`, {
    method,
    headers,
    credentials: 'same-origin',
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  if (res.status === 401 && retry) {
    if (await tryRefresh()) return api<T>(path, { ...opts, retry: false })
  }
  if (!res.ok) {
    let payload: ApiError = { code: 'INTERNAL', message: `HTTP ${res.status}` }
    try { payload = await res.json() } catch { /* a non-JSON error body */ }
    throw new ApiFailure(res.status, payload)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

// download fetches a CSV with the access token and saves it. A plain <a href>
// cannot carry an Authorization header, and putting the token in a query
// string would land it in the nginx access log.
export async function download(path: string, fallbackName: string) {
  const res = await fetch(`/api/v1${path}`, {
    headers: accessToken ? { Authorization: `Bearer ${accessToken}` } : {},
    credentials: 'same-origin',
  })
  if (!res.ok) {
    if (res.status === 401 && await tryRefresh()) return download(path, fallbackName)
    throw new ApiFailure(res.status, { code: 'INTERNAL', message: 'Unduhan gagal' })
  }
  const disposition = res.headers.get('Content-Disposition') || ''
  const match = /filename="([^"]+)"/.exec(disposition)
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = match ? match[1] : fallbackName
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

export interface Principal {
  user_id: string
  email: string
  full_name: string
  permissions: string[]
  grants: { role_id: string; role_code: string; company_id: string }[]
  company_ids: string[]
  is_superadmin: boolean
}

export interface MediaLineDTO {
  media_id?: string
  line_no?: number
  media_name: string
  price_idr: number
}

export interface PlanVersion {
  version_id: string
  version_no: number
  promo_name: string
  start_date: string
  end_date: string
  target_sales_idr: number
  target_receipt_count: number
  order_mode: 'dine_in' | 'take_away'
  promo_rule: string
  lead_time_overridden?: boolean
  created_at?: string
  media?: MediaLineDTO[]
  media_total_idr?: number
}

export interface Plan {
  plan_id: string
  plan_code: string
  status: 'DRAFT' | 'PENDING' | 'RELEASED' | 'REJECTED' | 'CANCELLED'
  company_id: string
  company_name: string
  company_code: string
  site_group_id: string
  site_group_name: string
  force_released: boolean
  created_by_name: string
  current_step_no: number
  current_step_name: string
  version: PlanVersion
  versions?: PlanVersion[]
  approval?: {
    instance_id: string
    status: string
    current_step_no: number
    steps: { step_no: number; step_name: string; satisfaction: string }[]
    events: {
      step_no: number
      step_name: string
      action: string
      actor_name: string
      reason: string
      occurred_at: string
    }[]
  }
}

export interface Holiday {
  holiday_id: string
  holiday_date: string
  holiday_name: string
  country: string
  is_active: boolean
  is_provisional: boolean
}

export interface Company { company_id: string; company_code: string; company_name: string; is_active: boolean }
export interface Site { site_id: string; company_id: string; site_code: string; site_name: string; site_type: string; is_active: boolean }
export interface SiteGroup { site_group_id: string; company_id: string; site_group_name: string; is_system: boolean; member_count: number; site_ids?: string[] }
