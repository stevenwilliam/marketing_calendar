import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type Plan } from '../lib/api'
import { formatDate, monthName, rp } from '../lib/format'
import { SearchBox, StatusPill, Loading, Empty } from '../components/ui'

const DAY_LABELS = ['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min']

export default function Calendar() {
  const now = new Date()
  const [year, setYear] = useState(now.getFullYear())
  const [month, setMonth] = useState(now.getMonth() + 1)
  const [rows, setRows] = useState<Plan[]>([])
  const [q, setQ] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    const p = new URLSearchParams({ year: String(year), month: String(month) })
    if (q) p.set('q', q)
    api<{ data: Plan[] }>(`/promotions/calendar?${p}`)
      .then((res) => { if (!cancelled) setRows(res.data ?? []) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [year, month, q])

  // The grid starts on Monday, which is how an Indonesian working week reads.
  const cells = useMemo(() => {
    const first = new Date(Date.UTC(year, month - 1, 1))
    const daysInMonth = new Date(Date.UTC(year, month, 0)).getUTCDate()
    const leading = (first.getUTCDay() + 6) % 7
    const out: { day: number | null; iso: string }[] = []
    for (let i = 0; i < leading; i++) out.push({ day: null, iso: '' })
    for (let d = 1; d <= daysInMonth; d++) {
      out.push({ day: d, iso: `${year}-${String(month).padStart(2, '0')}-${String(d).padStart(2, '0')}` })
    }
    while (out.length % 7 !== 0) out.push({ day: null, iso: '' })
    return out
  }, [year, month])

  const step = (delta: number) => {
    let m = month + delta, y = year
    if (m < 1) { m = 12; y-- }
    if (m > 12) { m = 1; y++ }
    setMonth(m); setYear(y)
  }

  const runningOn = (iso: string) =>
    rows.filter((p) => p.version.start_date <= iso && p.version.end_date >= iso)

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="kicker">Lintas merek</div>
          <h3>{monthName(month)} {year}</h3>
        </div>
        <div className="flex gap-2 items-end">
          <div className="w-full sm:w-64">
            <SearchBox value={q} onChange={setQ} placeholder="Cari promo di bulan ini…" />
          </div>
          <button className="btn btn-secondary" onClick={() => step(-1)} aria-label="Bulan sebelumnya">←</button>
          <button className="btn btn-secondary" onClick={() => { setYear(now.getFullYear()); setMonth(now.getMonth() + 1) }}>
            Hari ini
          </button>
          <button className="btn btn-secondary" onClick={() => step(1)} aria-label="Bulan berikutnya">→</button>
        </div>
      </div>

      {loading ? <Loading /> : (
        <>
          <div className="overflow-x-auto">
            <div className="min-w-[720px] border-t border-l border-divider">
              <div className="grid grid-cols-7">
                {DAY_LABELS.map((d) => (
                  <div key={d} className="kicker p-2 border-r border-b border-divider bg-surface">{d}</div>
                ))}
              </div>
              <div className="grid grid-cols-7">
                {cells.map((c, i) => {
                  const running = c.iso ? runningOn(c.iso) : []
                  return (
                    <div key={i}
                         className="border-r border-b border-divider min-h-[118px] p-1.5 bg-paper"
                         style={c.day === null ? { background: '#f3f2f2' } : undefined}>
                      {c.day !== null && (
                        <>
                          <div className="text-xs text-muted tnum mb-1">{c.day}</div>
                          {running.slice(0, 3).map((p) => (
                            <Link key={p.plan_id} to={`/promo/${p.plan_id}`}
                                  title={`${p.plan_code} · ${p.company_name} · ${rp(p.version.target_sales_idr)}`}
                                  className="block text-[10.5px] font-semibold leading-[1.35] px-1.5 py-0.5 mb-0.5 truncate"
                                  style={{
                                    background: '#eae7e7',
                                    borderLeft: `3px solid ${brand(p.company_code)}`,
                                    color: '#201e1d',
                                  }}>
                              {p.version.promo_name}
                            </Link>
                          ))}
                          {running.length > 3 && (
                            <div className="text-[10px] text-muted">+{running.length - 3} lagi</div>
                          )}
                        </>
                      )}
                    </div>
                  )
                })}
              </div>
            </div>
          </div>

          {rows.length === 0 ? (
            <Empty title="Tidak ada promo pada bulan ini"
                   hint="Gunakan panah untuk berpindah bulan, atau buat rencana baru." />
          ) : (
            <div className="flex flex-col gap-1.5">
              <div className="kicker">Berjalan bulan ini</div>
              {rows.map((p) => (
                <Link key={p.plan_id} to={`/promo/${p.plan_id}`}
                      className="card-paper flex flex-wrap items-center gap-3 hover:bg-surface"
                      style={{ borderLeft: `3px solid ${brand(p.company_code)}` }}>
                  <span className="font-mono text-xs">{p.plan_code}</span>
                  <span className="font-semibold">{p.version.promo_name}</span>
                  <span className="text-xs text-muted">{p.company_name} · {p.site_group_name}</span>
                  <span className="text-xs text-muted">
                    {formatDate(p.version.start_date)} – {formatDate(p.version.end_date)}
                  </span>
                  <span className="ml-auto"><StatusPill status={p.status} forceReleased={p.force_released} /></span>
                </Link>
              ))}
            </div>
          )}
        </>
      )}
    </div>
  )
}

function brand(code: string) {
  return code === 'MAXX' ? '#6b3b2a' : code === 'RUUMA' ? '#7a2e63'
    : code === 'SUNSHINE' ? '#7c4a00' : '#201e1d'
}
