import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, download, type Company, type Plan } from '../lib/api'
import { useAuth } from '../lib/auth'
import { formatDate, rp, count, orderModeLabel, brandColor } from '../lib/format'
import { SearchBox, ExportButton, StatusPill, TableWrap, Empty, Loading, BrandTag } from '../components/ui'
import { BrandTabs } from '../components/BrandTabs'

const STATUSES = ['', 'DRAFT', 'PENDING', 'RELEASED', 'REJECTED', 'CANCELLED']
const STATUS_LABEL: Record<string, string> = {
  '': 'Semua status', DRAFT: 'Draf', PENDING: 'Menunggu',
  RELEASED: 'Dirilis', REJECTED: 'Ditolak', CANCELLED: 'Dibatalkan',
}

export default function Plans() {
  const { can } = useAuth()
  const [rows, setRows] = useState<Plan[]>([])
  const [total, setTotal] = useState(0)
  const [companies, setCompanies] = useState<Company[]>([])
  const [brand, setBrand] = useState('')          // '' = every brand
  const [counts, setCounts] = useState<Record<string, number>>({})
  const [countsLoading, setCountsLoading] = useState(false)
  const [q, setQ] = useState('')
  const [status, setStatus] = useState('')
  const [mode, setMode] = useState('')
  const [loading, setLoading] = useState(true)

  // The query string is built once and used for the list, the tab counts AND
  // the export, so the file is always what the screen is showing (BR-7.4) and
  // a tab's number always means the same thing as its table.
  const filters = new URLSearchParams()
  if (q) filters.set('q', q)
  if (status) filters.set('status', status)
  if (mode) filters.set('order_mode', mode)
  const filterQS = filters.toString()

  const params = new URLSearchParams(filters)
  if (brand) params.set('company_id', brand)
  const qs = params.toString()

  useEffect(() => {
    api<{ data: Company[] }>('/companies').then((r) => setCompanies(r.data ?? []))
  }, [])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    api<{ data: Plan[]; total: number }>(`/promotions${qs ? '?' + qs : ''}`)
      .then((res) => { if (!cancelled) { setRows(res.data ?? []); setTotal(res.total) } })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [qs])

  // Per-tab counts, recomputed whenever the filters move. `limit=1` because
  // only `total` is wanted — asking for the rows as well would download the
  // whole list three times to render three numbers.
  useEffect(() => {
    if (companies.length <= 1) return
    let cancelled = false
    setCountsLoading(true)
    const ask = (companyID: string) => {
      const p = new URLSearchParams(filters)
      if (companyID) p.set('company_id', companyID)
      p.set('limit', '1')
      return api<{ total: number }>(`/promotions?${p}`).then((r) => [companyID, r.total] as const)
    }
    Promise.all([ask(''), ...companies.map((c) => ask(c.company_id))])
      .then((pairs) => {
        if (cancelled) return
        setCounts(Object.fromEntries(pairs))
      })
      .catch(() => { /* the tabs simply show no number; the table is the truth */ })
      .finally(() => { if (!cancelled) setCountsLoading(false) })
    return () => { cancelled = true }
    // filters is rebuilt every render; filterQS is its stable identity.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filterQS, companies])

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="kicker">Marketing</div>
          <h3>Rencana promo</h3>
        </div>
        {can('promo.create') && (
          <Link to="/promo/baru" className="btn btn-primary">Buat rencana</Link>
        )}
      </div>

      <BrandTabs
        companies={companies}
        value={brand}
        onChange={setBrand}
        counts={counts}
        loading={countsLoading}
      />

      <div className="flex flex-wrap gap-2 items-end">
        <div className="w-full sm:w-72">
          <SearchBox value={q} onChange={setQ} placeholder="Cari nama, kode atau kelompok toko…" />
        </div>
        <div>
          <label className="label" htmlFor="f-status">Status</label>
          <select id="f-status" className="input w-44" value={status} onChange={(e) => setStatus(e.target.value)}>
            {STATUSES.map((s) => <option key={s} value={s}>{STATUS_LABEL[s]}</option>)}
          </select>
        </div>
        <div>
          <label className="label" htmlFor="f-mode">Mode pesanan</label>
          <select id="f-mode" className="input w-44" value={mode} onChange={(e) => setMode(e.target.value)}>
            <option value="">Semua mode</option>
            <option value="dine_in">Makan di tempat</option>
            <option value="take_away">Bawa pulang</option>
          </select>
        </div>
        <div className="ml-auto">
          {can('report.export') && (
            <ExportButton onExport={() => download(`/promotions/export${qs ? '?' + qs : ''}`, 'rencana-promo.csv')} />
          )}
        </div>
      </div>

      {loading ? <Loading /> : rows.length === 0 ? (
        <Empty
          title={brand
            ? `Tidak ada rencana untuk ${companies.find((c) => c.company_id === brand)?.company_name ?? 'merek ini'}`
            : 'Tidak ada rencana yang cocok'}
          hint={q || status || mode
            ? 'Ubah kata kunci atau filter di atas, atau pilih merek lain.'
            : can('promo.create') ? 'Mulai dengan tombol “Buat rencana”.' : 'Belum ada rencana promo yang dibuat.'}
        />
      ) : (
        <>
          <TableWrap>
            <table className="table">
              <thead>
                <tr>
                  <th>Kode</th><th>Nama promo</th>
                  {!brand && <th>Merek</th>}
                  <th>Kelompok toko</th>
                  <th>Periode</th><th>Mode</th>
                  <th className="text-right">Target penjualan</th>
                  <th className="text-right">Target struk</th>
                  <th>Status</th><th>Langkah</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((p) => (
                  <tr key={p.plan_id} style={{ borderLeft: `3px solid ${brandColor(p.company_code)}` }}>
                    <td className="font-mono text-xs">
                      <Link className="text-accent-ink underline" to={`/promo/${p.plan_id}`}>{p.plan_code}</Link>
                    </td>
                    <td className="font-semibold">{p.version.promo_name}</td>
                    {!brand && <td><BrandTag code={p.company_code} name={p.company_name} /></td>}
                    <td>{p.site_group_name}</td>
                    <td className="whitespace-nowrap">
                      {formatDate(p.version.start_date)} – {formatDate(p.version.end_date)}
                    </td>
                    <td>{orderModeLabel(p.version.order_mode)}</td>
                    <td className="text-right tnum">{rp(p.version.target_sales_idr)}</td>
                    <td className="text-right tnum">{count(p.version.target_receipt_count)}</td>
                    <td><StatusPill status={p.status} forceReleased={p.force_released} /></td>
                    <td className="text-xs text-muted">
                      {p.status === 'PENDING' ? `${p.current_step_no}. ${p.current_step_name}` : '—'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </TableWrap>
          <p className="text-xs text-muted">
            {total} rencana
            {brand && ` · ${companies.find((c) => c.company_id === brand)?.company_name ?? ''}`}
          </p>
        </>
      )}
    </div>
  )
}
