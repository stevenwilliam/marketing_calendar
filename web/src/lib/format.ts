// Formatting helpers. Money is integer rupiah end to end — no float ever
// touches an amount, in the browser any more than in Go.

export function rupiah(n: number | null | undefined): string {
  if (n === null || n === undefined) return '—'
  const neg = n < 0
  const digits = Math.abs(Math.trunc(n)).toString()
  let out = ''
  for (let i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 === 0) out += '.'
    out += digits[i]
  }
  return (neg ? '-' : '') + out
}

export const rp = (n: number | null | undefined) =>
  n === null || n === undefined ? '—' : `Rp ${rupiah(n)}`

export function count(n: number | null | undefined): string {
  return n === null || n === undefined ? '—' : rupiah(n)
}

// Basis points to a percentage, in integer arithmetic. 12345 -> "123,45%".
// Indonesian uses a comma for the decimal mark.
export function percentFromBPS(bps: number | null | undefined): string {
  if (bps === null || bps === undefined) return '—'
  const neg = bps < 0
  const abs = Math.abs(Math.trunc(bps))
  const whole = Math.trunc(abs / 100)
  const frac = abs % 100
  return `${neg ? '-' : ''}${rupiah(whole)},${String(frac).padStart(2, '0')}%`
}

const MONTHS = ['Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
  'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember']
const MONTHS_SHORT = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun',
  'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']

export const monthName = (m: number) => MONTHS[m - 1] ?? ''

// Dates arrive as yyyy-mm-dd business dates already evaluated in
// Asia/Jakarta. They are parsed as plain strings, never through Date(), which
// would reinterpret them in the browser's zone and shift them by a day.
export function formatDate(iso: string | null | undefined): string {
  if (!iso) return '—'
  const [y, m, d] = iso.split('T')[0].split('-').map(Number)
  if (!y || !m || !d) return iso
  return `${d} ${MONTHS_SHORT[m - 1]} ${y}`
}

export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return '—'
  const dt = new Date(iso)
  if (Number.isNaN(dt.getTime())) return iso
  return new Intl.DateTimeFormat('id-ID', {
    day: 'numeric', month: 'short', year: 'numeric',
    hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Jakarta',
  }).format(dt)
}

export const orderModeLabel = (m: string) =>
  m === 'dine_in' ? 'Makan di tempat' : m === 'take_away' ? 'Bawa pulang' : m

export const salesTypeLabel = (s: string) =>
  s === 'promo' ? 'Promo' : s === 'normal' ? 'Normal' : s

// A brand accent is a 3px left border plus the brand NAME. The colour is a
// scan aid, never the only identification.
export function brandColor(code: string): string {
  switch (code) {
    case 'MAXX': return '#6b3b2a'
    case 'RUUMA': return '#7a2e63'
    case 'SUNSHINE': return '#7c4a00'
    default: return '#201e1d'
  }
}

export function todayISO(): string {
  return new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Jakarta' }).format(new Date())
}

export function addDaysISO(iso: string, days: number): string {
  const [y, m, d] = iso.split('-').map(Number)
  const dt = new Date(Date.UTC(y, m - 1, d))
  dt.setUTCDate(dt.getUTCDate() + days)
  return dt.toISOString().slice(0, 10)
}
