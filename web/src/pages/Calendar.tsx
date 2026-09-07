import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type Holiday, type Plan } from '../lib/api'
import { formatDate, monthName, rp, percentFromBPS } from '../lib/format'
import { SearchBox, StatusPill, Loading, Empty, ACHIEVEMENT } from '../components/ui'

const DAY_LABELS = ['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min']

interface DayAchievement {
  actual_idr: number
  receipt_count: number
  band: 'under' | 'near' | 'over' | 'none'
  achieved_bps: number | null
}

interface CalendarPlan extends Plan {
  days?: number
  daily_target_idr?: number
  has_daily_target?: boolean
  daily?: Record<string, DayAchievement>
}

/**
 * The three achievement bands, now SOLID fills with light ink (D56).
 *
 * They still separate from each other by only 1.03–1.09 — hue alone, because
 * three colours chosen to contrast equally with the page necessarily sit at
 * the same lightness. Boldness made them scannable, not more distinguishable.
 * The PERCENTAGE on the chip remains the signal.
 */
const BAND = ACHIEVEMENT

// The operating day in Jakarta. A browser in another timezone must not decide
// which days have already happened — that is a business date (BR-1.4).
const todayISO = new Intl.DateTimeFormat('en-CA', {
  timeZone: 'Asia/Jakarta', year: 'numeric', month: '2-digit', day: '2-digit',
}).format(new Date())

export default function Calendar() {
  const now = new Date()
  const [year, setYear] = useState(now.getFullYear())
  const [month, setMonth] = useState(now.getMonth() + 1)
  const [rows, setRows] = useState<CalendarPlan[]>([])
  const [holidays, setHolidays] = useState<Holiday[]>([])
  const [q, setQ] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    const p = new URLSearchParams({ year: String(year), month: String(month) })
    if (q) p.set('q', q)
    api<{ data: CalendarPlan[] }>(`/promotions/calendar?${p}`)
      .then((res) => { if (!cancelled) setRows(res.data ?? []) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [year, month, q])

  // Holidays are fetched per YEAR, not per month, because the calendar only
  // moves a month at a time and refetching the same year on every step would
  // be a request per click for data that did not change.
  useEffect(() => {
    let cancelled = false
    api<{ data: Holiday[] }>(`/holidays?year=${year}`)
      .then((r) => { if (!cancelled) setHolidays(r.data ?? []) })
      .catch(() => { if (!cancelled) setHolidays([]) })
    return () => { cancelled = true }
  }, [year])

  // Keyed by yyyy-mm-dd. The API returns a timestamp, so it is sliced rather
  // than passed through Date(), which would reinterpret it in the browser's
  // zone and move a holiday across midnight.
  const holidayOn = useMemo(() => {
    const m = new Map<string, Holiday>()
    for (const h of holidays) {
      if (h.is_active !== false) m.set(String(h.holiday_date).slice(0, 10), h)
    }
    return m
  }, [holidays])

  // The grid starts on Monday, which is how an Indonesian working week reads.
  // The grid is laid out Monday-first, so columns 5 and 6 are Saturday and
  // Sunday. Deriving it from the POSITION rather than from a Date avoids
  // parsing the cell's iso string in the browser's zone, which would put a
  // Jakarta Saturday in Friday for anyone west of here.
  const isWeekendColumn = (i: number) => i % 7 >= 5

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
                {DAY_LABELS.map((d, i) => (
                  <div key={d}
                       className="kicker p-2 border-r border-b border-divider"
                       style={{ background: isWeekendColumn(i) ? '#fbe4e8' : '#eae9e9' }}>
                    {d}
                  </div>
                ))}
              </div>
              <div className="grid grid-cols-7">
                {cells.map((c, i) => {
                  const running = c.iso ? runningOn(c.iso) : []
                  const holiday = c.iso ? holidayOn.get(c.iso) : undefined
                  // One tint, one meaning: this day does not count toward the
                  // promotion lead time. A weekend and a public holiday are
                  // the same thing to BR-3.3, and colouring them differently
                  // would invent a distinction the rule does not make.
                  const nonWorking = c.day !== null && (isWeekendColumn(i) || Boolean(holiday))
                  return (
                    <div key={i}
                         className="border-r border-b border-divider min-h-[118px] p-1.5 bg-paper"
                         style={
                           c.day === null ? { background: '#f3f2f2' }
                           // Measured against everything the cell draws on top
                           // of it: ink 13.73, muted 4.80, danger 6.55.
                           : nonWorking ? { background: '#fbe4e8' }
                           : undefined
                         }>
                      {c.day !== null && (
                        <>
                          <div className="flex items-baseline gap-1 mb-1">
                            <span className="text-xs text-muted tnum">{c.day}</span>
                            {holiday && (
                              // The tint is only 1.21 against the white cell
                              // beside it, so it cannot be the only signal:
                              // the name is the signal, the colour is the aid.
                              <span className="text-[10px] leading-tight text-danger font-semibold truncate"
                                    title={holiday.holiday_name +
                                      (holiday.is_provisional ? ' (perkiraan, belum dikonfirmasi SKB)' : '')}>
                                {holiday.holiday_name}
                                {holiday.is_provisional && (
                                  <span aria-label="perkiraan" title="perkiraan"> ~</span>
                                )}
                              </span>
                            )}
                          </div>
                          {running.slice(0, 3).map((p) => {
                            const a = p.daily?.[c.iso]
                            const band = BAND[a?.band ?? 'none'] ?? BAND.none
                            const pct = a?.achieved_bps == null ? null : percentFromBPS(a.achieved_bps)
                            // Why there is no percentage, in the order the
                            // reasons apply. A day nobody has lived through
                            // yet, and a plan that was never released, have
                            // sold nothing for reasons that are not failures
                            // — they must not read as a red 0% (D58), and
                            // they must not borrow the em dash either, which
                            // means one thing only: no daily target.
                            const why = a ? null
                              : c.iso > todayISO ? 'Hari ini belum berjalan'
                              : p.status !== 'RELEASED' ? 'Promo belum dirilis — tidak ada realisasi'
                              : p.has_daily_target === false ? 'Belum ada target harian'
                              : 'Belum ada realisasi'
                            return (
                              <Link key={p.plan_id} to={`/promo/${p.plan_id}`}
                                    title={`${p.plan_code} · ${p.company_name}\n` +
                                      `Target harian ${rp(p.daily_target_idr ?? 0)}\n` +
                                      (why ? why
                                        : `Aktual ${rp(a?.actual_idr ?? 0)}` +
                                          (pct ? ` · ${pct} — ${band.label}` : ' · belum ada target harian'))}
                                    className="flex items-baseline gap-1 text-[10.5px] font-extrabold leading-[1.35] px-1.5 py-0.5 mb-0.5"
                                    style={{
                                      background: band.bg,
                                      borderLeft: `3px solid ${brand(p.company_code)}`,
                                      color: band.ink,
                                    }}>
                                <span aria-hidden="true">{why ? '·' : band.glyph}</span>
                                <span className="truncate flex-1">{p.version.promo_name}</span>
                                {/* The number IS the signal. Without it the three
                                    bands are the same shade to anyone who cannot
                                    see hue. */}
                                {why
                                  ? <span className="sr-only">{why}</span>
                                  : <span className="tnum shrink-0">{pct ?? '—'}</span>}
                              </Link>
                            )
                          })}
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

          <div className="flex flex-wrap items-center gap-4 text-xs text-muted">
            <span className="inline-flex items-center gap-1.5">
              <span aria-hidden="true" className="inline-block w-3.5 h-3.5 border border-divider"
                    style={{ background: '#fbe4e8' }} />
              <b>Bukan hari kerja</b> — akhir pekan dan hari libur nasional
            </span>
            <span className="inline-flex items-center gap-1.5">
              <span aria-hidden="true">~</span>
              Hari libur perkiraan, belum dikonfirmasi surat keputusan bersama
            </span>
            <span>
              Hari bertanda ini <b>tidak dihitung</b> dalam masa tenggang promo.
            </span>
          </div>

          <div className="flex flex-wrap items-center gap-4 text-xs text-muted">
            <span className="font-semibold text-ink">Capaian harian:</span>
            {(['under', 'near', 'over'] as const).map((k) => (
              <span key={k} className="inline-flex items-center gap-1.5">
                <span className="inline-flex items-center gap-1 px-1.5 py-0.5 text-[10.5px] font-extrabold"
                      style={{ background: BAND[k].bg, color: BAND[k].ink }}>
                  <span aria-hidden="true">{BAND[k].glyph}</span>
                  {k === 'under' ? '<70%' : k === 'near' ? '70–100%' : '≥100%'}
                </span>
                {BAND[k].label}
              </span>
            ))}
            {/* The fourth state is not a band and must not look like one: it
                is the absence of a verdict, not a bad one (D58). */}
            <span className="inline-flex items-center gap-1.5">
              <span className="inline-flex items-center gap-1 px-1.5 py-0.5 text-[10.5px] font-extrabold"
                    style={{ background: BAND.none.bg, color: BAND.none.ink }}>
                <span aria-hidden="true">·</span>
                belum
              </span>
              belum berjalan, belum dirilis, atau belum ada target harian
            </span>
            <span>
              Target harian = target penjualan promo dibagi jumlah harinya.
              Aktual dihitung dari transaksi bertanda id promo, bukan dari rentang tanggal.
            </span>
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
                  {p.has_daily_target && (
                    <span className="text-xs text-muted tnum">
                      Target harian {rp(p.daily_target_idr ?? 0)} × {p.days} hari
                    </span>
                  )}
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
