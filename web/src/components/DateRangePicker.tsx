import { useEffect, useMemo, useRef, useState } from 'react'
import { api, type Holiday } from '../lib/api'
import { formatDate, monthName } from '../lib/format'

/**
 * The two-month range picker the design system specifies (10 §4).
 *
 * Three things it does that a pair of <input type=date> cannot:
 *
 *  - dates inside the lead time are DISABLED AND SAY WHY, rather than being
 *    silently selectable and refused after submit;
 *  - weekends and public holidays are tinted with the same #fbe4e8 the
 *    calendar uses, because they are the days that do not count toward that
 *    lead time — the picker and the calendar must not disagree;
 *  - it is keyboard operable, which the design system requires and a custom
 *    grid otherwise quietly is not.
 *
 * The earliest permitted date comes from the SERVER. Computing working days in
 * the browser would be a second implementation of BR-3.3, and it would be the
 * one without the holiday table.
 */

const DAY_LABELS = ['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min']

interface LeadTime {
  earliest_start: string
  lead_time_working_days: number
  today: string
}

function iso(y: number, m: number, d: number) {
  return `${y}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`
}

function monthCells(year: number, month: number) {
  const first = new Date(Date.UTC(year, month - 1, 1))
  const days = new Date(Date.UTC(year, month, 0)).getUTCDate()
  const leading = (first.getUTCDay() + 6) % 7 // Monday-first
  const out: { day: number | null; iso: string; col: number }[] = []
  for (let i = 0; i < leading; i++) out.push({ day: null, iso: '', col: i % 7 })
  for (let d = 1; d <= days; d++) out.push({ day: d, iso: iso(year, month, d), col: (leading + d - 1) % 7 })
  while (out.length % 7 !== 0) out.push({ day: null, iso: '', col: out.length % 7 })
  return out
}

function addMonths(year: number, month: number, delta: number) {
  let m = month + delta
  let y = year
  while (m > 12) { m -= 12; y++ }
  while (m < 1) { m += 12; y-- }
  return { year: y, month: m }
}

export function DateRangePicker({
  start, end, onChange, disabled,
}: {
  start: string
  end: string
  onChange: (start: string, end: string) => void
  disabled?: boolean
}) {
  const [lead, setLead] = useState<LeadTime | null>(null)
  const [holidays, setHolidays] = useState<Holiday[]>([])
  const [open, setOpen] = useState(false)
  const [picking, setPicking] = useState<'start' | 'end'>('start')
  const [hover, setHover] = useState('')
  const wrap = useRef<HTMLDivElement>(null)

  const base = start || new Date().toISOString().slice(0, 10)
  const [view, setView] = useState(() => ({
    year: Number(base.slice(0, 4)), month: Number(base.slice(5, 7)),
  }))

  useEffect(() => {
    api<LeadTime>('/promotions/lead-time').then(setLead).catch(() => setLead(null))
  }, [])

  // Both visible months may span a year boundary, so fetch both years.
  useEffect(() => {
    const next = addMonths(view.year, view.month, 1)
    const years = [...new Set([view.year, next.year])]
    Promise.all(years.map((y) => api<{ data: Holiday[] }>(`/holidays?year=${y}`)))
      .then((rs) => setHolidays(rs.flatMap((r) => r.data ?? [])))
      .catch(() => setHolidays([]))
  }, [view.year, view.month])

  const holidayOn = useMemo(() => {
    const m = new Map<string, Holiday>()
    for (const h of holidays) {
      if (h.is_active !== false) m.set(String(h.holiday_date).slice(0, 10), h)
    }
    return m
  }, [holidays])

  // Close on click-away and on Escape, which a custom popover has to do itself.
  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (wrap.current && !wrap.current.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(false) }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  const earliest = lead?.earliest_start ?? ''
  const tooEarly = (d: string) => Boolean(earliest) && d < earliest

  const pick = (d: string) => {
    if (tooEarly(d)) return
    if (picking === 'start' || !start || d < start) {
      onChange(d, end && end >= d ? end : '')
      setPicking('end')
    } else {
      onChange(start, d)
      setPicking('start')
      setOpen(false)
    }
  }

  const inRange = (d: string) => {
    if (!start) return false
    const to = end || (picking === 'end' ? hover : '')
    if (!to) return d === start
    return d >= start && d <= to
  }

  const second = addMonths(view.year, view.month, 1)

  const renderMonth = (year: number, month: number) => (
    <div className="min-w-[248px]">
      <div className="text-center font-semibold text-sm mb-1.5">
        {monthName(month)} {year}
      </div>
      <div className="grid grid-cols-7">
        {DAY_LABELS.map((d, i) => (
          <div key={d} className="kicker text-center py-1"
               style={{ background: i >= 5 ? '#fbe4e8' : undefined }}>{d}</div>
        ))}
      </div>
      <div className="grid grid-cols-7">
        {monthCells(year, month).map((c, i) => {
          if (c.day === null) return <div key={i} className="h-9" />
          const holiday = holidayOn.get(c.iso)
          const nonWorking = c.col >= 5 || Boolean(holiday)
          const disabledDay = tooEarly(c.iso)
          const selected = c.iso === start || c.iso === end
          const between = inRange(c.iso) && !selected
          return (
            <button
              key={i}
              type="button"
              disabled={disabledDay}
              aria-label={`${formatDate(c.iso)}${holiday ? ' — ' + holiday.holiday_name : ''}${
                disabledDay ? ' (di dalam masa tenggang)' : ''}`}
              aria-pressed={selected}
              title={holiday?.holiday_name}
              onMouseEnter={() => setHover(c.iso)}
              onClick={() => pick(c.iso)}
              className="h-9 text-sm tnum disabled:cursor-not-allowed"
              // Three states, three DIFFERENT properties — deliberately not
              // three background tints. In-range was a second pale pink and
              // measured 1.03 against the non-working tint: the same
              // luminance, indistinguishable. Any two pale fills would be.
              //   endpoint     solid fill + light text
              //   in range     2px underline, no fill
              //   non-working  the calendar's own tint
              // The underline clears 1.4.11 on both grounds it can sit on:
              // 7.17 on a plain cell, 5.93 on a non-working one.
              style={{
                background: selected ? '#ae1800'
                  : nonWorking ? '#fbe4e8'
                  : undefined,
                color: selected ? '#f3f2f2' : disabledDay ? 'rgba(32,30,29,0.35)' : undefined,
                textDecoration: disabledDay ? 'line-through' : undefined,
                boxShadow: between ? 'inset 0 -2px 0 0 #ae1800' : undefined,
              }}
            >
              {c.day}
            </button>
          )
        })}
      </div>
    </div>
  )

  return (
    <div ref={wrap} className="relative">
      <button
        type="button"
        disabled={disabled}
        onClick={() => { setOpen((v) => !v); setPicking('start') }}
        aria-haspopup="dialog"
        aria-expanded={open}
        className="input text-left"
      >
        {start && end
          ? `${formatDate(start)} – ${formatDate(end)}`
          : start
            ? `${formatDate(start)} – pilih tanggal selesai`
            : 'Pilih periode promo'}
      </button>

      {open && (
        <div role="dialog" aria-label="Pilih periode promo"
             className="absolute z-40 mt-1 bg-paper border border-divider shadow-lg p-3">
          <div className="flex items-center justify-between mb-2">
            <button type="button" className="btn btn-secondary px-2 py-1"
                    aria-label="Bulan sebelumnya"
                    onClick={() => setView(addMonths(view.year, view.month, -1))}>←</button>
            <span className="text-xs text-muted">
              {picking === 'start' ? 'Pilih tanggal mulai' : 'Pilih tanggal selesai'}
            </span>
            <button type="button" className="btn btn-secondary px-2 py-1"
                    aria-label="Bulan berikutnya"
                    onClick={() => setView(addMonths(view.year, view.month, 1))}>→</button>
          </div>

          <div className="flex gap-4 overflow-x-auto">
            {renderMonth(view.year, view.month)}
            {renderMonth(second.year, second.month)}
          </div>

          <div className="mt-2 pt-2 border-t border-hairline flex flex-col gap-1 text-xs text-muted">
            {lead && (
              // The disabled dates explain themselves rather than being a grey
              // box the user has to guess at.
              <p>
                Paling cepat <b>{formatDate(lead.earliest_start)}</b> —
                {' '}{lead.lead_time_working_days} hari kerja dari hari ini.
                Tanggal sebelum itu dicoret dan tidak dapat dipilih.
              </p>
            )}
            <p className="flex flex-wrap items-center gap-3">
              <span className="inline-flex items-center gap-1.5">
                <span aria-hidden="true" className="inline-block w-3 h-3 border border-divider"
                      style={{ background: '#fbe4e8' }} />
                Bukan hari kerja
              </span>
              <span className="inline-flex items-center gap-1.5">
                <span aria-hidden="true" className="inline-block w-3 h-3"
                      style={{ boxShadow: 'inset 0 -2px 0 0 #ae1800' }} />
                Di dalam periode
              </span>
              <span className="inline-flex items-center gap-1.5">
                <span aria-hidden="true" className="inline-block w-3 h-3"
                      style={{ background: '#ae1800' }} />
                Tanggal mulai / selesai
              </span>
            </p>
            <p>Akhir pekan dan hari libur tidak dihitung dalam masa tenggang.</p>
            {start && (
              <button type="button" className="btn btn-ghost self-start text-xs px-0"
                      onClick={() => { onChange('', ''); setPicking('start') }}>
                Hapus pilihan
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
