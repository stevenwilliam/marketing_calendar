// Shared components. Every rule the design system calls "not taste" lives
// here once, so a screen cannot quietly break it.

import { useEffect, useRef, useState, type ReactNode } from 'react'

/* --- status --------------------------------------------------------------
   BR + design rule 1: status is NEVER colour alone. Every pill carries a
   glyph and a word. A red pill and a green pill are the same pill to roughly
   one man in twelve. */

const STATUS: Record<string, { label: string; glyph: string; className: string }> = {
  DRAFT:     { label: 'Draf',          glyph: '○', className: 'bg-neutral-200 text-neutral-900' },
  PENDING:   { label: 'Menunggu',      glyph: '◷', className: 'bg-[#fff2ef] text-warn' },
  RELEASED:  { label: 'Dirilis',       glyph: '✓', className: 'bg-[#e8f2ec] text-success' },
  REJECTED:  { label: 'Ditolak',       glyph: '✕', className: 'bg-[#fdeaec] text-danger' },
  CANCELLED: { label: 'Dibatalkan',    glyph: '⊘', className: 'bg-neutral-200 text-neutral-800' },
}

export function StatusPill({ status, forceReleased }: { status: string; forceReleased?: boolean }) {
  // Force-released is deliberately distinct: a reviewer must see at a glance
  // which promotions bypassed the chain (BR-4.8, BR-7.5).
  if (forceReleased) {
    return (
      <span className="pill bg-[#e6f0f3] text-info" title="Dirilis paksa oleh superadmin dengan alasan tertulis">
        <span aria-hidden="true">!</span> Rilis paksa
      </span>
    )
  }
  const s = STATUS[status] ?? { label: status, glyph: '·', className: 'bg-neutral-200 text-ink' }
  return (
    <span className={`pill ${s.className}`}>
      <span aria-hidden="true">{s.glyph}</span> {s.label}
    </span>
  )
}

/* --- search --------------------------------------------------------------
   BR-7.1: every screen rendering a list has a debounced search box. No
   exceptions, which is why this is a component and not a per-page input. */

export function SearchBox({
  value, onChange, placeholder = 'Cari…', label = 'Cari',
}: { value: string; onChange: (v: string) => void; placeholder?: string; label?: string }) {
  const [local, setLocal] = useState(value)
  const first = useRef(true)

  useEffect(() => { setLocal(value) }, [value])
  useEffect(() => {
    if (first.current) { first.current = false; return }
    const t = setTimeout(() => onChange(local), 250)
    return () => clearTimeout(t)
    // onChange is intentionally excluded: callers pass an inline function, and
    // including it would restart the timer on every render and debounce
    // nothing.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [local])

  return (
    <div className="relative">
      <label className="sr-only" htmlFor="search-box">{label}</label>
      <input
        id="search-box"
        type="search"
        className="input pl-8"
        placeholder={placeholder}
        value={local}
        onChange={(e) => setLocal(e.target.value)}
      />
      <span className="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted pointer-events-none" aria-hidden="true">⌕</span>
    </div>
  )
}

/* --- export --------------------------------------------------------------
   BR-7.2/7.4: every grid ships an Export CSV button, and it exports what the
   screen is currently showing. */

export function ExportButton({ onExport, disabled, title }: { onExport: () => void; disabled?: boolean; title?: string }) {
  const [busy, setBusy] = useState(false)
  return (
    <button
      className="btn btn-secondary"
      disabled={disabled || busy}
      title={title ?? 'Unduh CSV dengan filter dan pencarian yang sedang aktif'}
      onClick={async () => {
        setBusy(true)
        try { await onExport() } finally { setBusy(false) }
      }}
    >
      {busy ? 'Menyiapkan…' : 'Ekspor CSV'}
    </button>
  )
}

/* --- disabled states explain themselves ---------------------------------- */

export function Reason({ children }: { children: ReactNode }) {
  return <p className="text-xs text-muted mt-1">{children}</p>
}

/* --- zero state: say what to do, never "No data" ------------------------- */

export function Empty({ title, hint }: { title: string; hint?: string }) {
  return (
    <div className="p-8 text-center bg-paper border border-hairline">
      <p className="font-semibold">{title}</p>
      {hint && <p className="text-sm text-muted mt-1">{hint}</p>}
    </div>
  )
}

export function Loading({ what = 'Memuat…' }: { what?: string }) {
  return <div className="p-8 text-center text-muted" role="status" aria-live="polite">{what}</div>
}

/* --- errors are announced, associated, and say what to do ---------------- */

export function ErrorBox({ title, message, fields, onDismiss }: {
  title?: string; message: string
  fields?: Record<string, string>
  onDismiss?: () => void
}) {
  return (
    <div role="alert" className="border-l-[3px] border-danger bg-[#fdeaec] p-3 text-sm">
      <div className="flex items-start gap-2">
        <span aria-hidden="true" className="text-danger font-extrabold">✕</span>
        <div className="flex-1">
          <p className="font-semibold text-danger">{title ?? 'Tidak dapat dilanjutkan'}</p>
          <p className="mt-0.5">{message}</p>
          {fields && Object.keys(fields).length > 0 && (
            <ul className="mt-1.5 list-disc list-inside">
              {Object.entries(fields).map(([k, v]) => <li key={k}><b>{k}</b>: {v}</li>)}
            </ul>
          )}
        </div>
        {onDismiss && (
          <button className="btn btn-ghost text-xs" onClick={onDismiss} aria-label="Tutup pesan">✕</button>
        )}
      </div>
    </div>
  )
}

/* --- a table that scrolls in its OWN wrapper, never the page ------------- */

export function TableWrap({ children }: { children: ReactNode }) {
  return <div className="overflow-x-auto bg-paper border border-hairline">{children}</div>
}

/* --- modal --------------------------------------------------------------- */

export function Modal({ title, children, onClose, footer }: {
  title: string; children: ReactNode; onClose: () => void; footer?: ReactNode
}) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', onKey)
    // Focus moves into the dialog so a keyboard user is not left behind it.
    ref.current?.querySelector<HTMLElement>('button, input, textarea, select')?.focus()
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose])

  return (
    <div
      className="fixed inset-0 z-50 grid place-items-center p-4"
      style={{ background: 'rgba(45,43,43,0.5)' }}
      onMouseDown={(e) => { if (e.target === e.currentTarget) onClose() }}
    >
      <div ref={ref} role="dialog" aria-modal="true" aria-label={title}
           className="w-full max-w-lg bg-surface p-4 flex flex-col gap-3 shadow-lg">
        <h4>{title}</h4>
        <div className="text-sm">{children}</div>
        {footer && <div className="flex justify-end gap-2 mt-1">{footer}</div>}
      </div>
    </div>
  )
}

/* --- brand accent: a 3px left border PLUS the name ----------------------- */

export function BrandTag({ code, name }: { code: string; name: string }) {
  const colors: Record<string, string> = {
    MAXX: '#6b3b2a', RUUMA: '#7a2e63', SUNSHINE: '#7c4a00',
  }
  return (
    <span className="inline-flex items-center gap-1.5 text-xs">
      <span aria-hidden="true" className="inline-block w-[3px] h-3.5"
            style={{ background: colors[code] ?? '#201e1d' }} />
      {name}
    </span>
  )
}
