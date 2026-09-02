import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, download, type Plan } from '../lib/api'
import { useAuth } from '../lib/auth'
import { formatDate, rp, count, orderModeLabel } from '../lib/format'
import { SearchBox, ExportButton, StatusPill, TableWrap, Empty, Loading, BrandTag } from '../components/ui'

const STATUSES = ['', 'DRAFT', 'PENDING', 'RELEASED', 'REJECTED', 'CANCELLED']
const STATUS_LABEL: Record<string, string> = {
  '': 'Semua status', DRAFT: 'Draf', PENDING: 'Menunggu',
  RELEASED: 'Dirilis', REJECTED: 'Ditolak', CANCELLED: 'Dibatalkan',
}

export default function Plans() {
  const { can } = useAuth()
  const [rows, setRows] = useState<Plan[]>([])
  const [total, setTotal] = useState(0)
  const [q, setQ] = useState('')
  const [status, setStatus] = useState('')
  const [mode, setMode] = useState('')
  const [loading, setLoading] = useState(true)

  // The query string is built once and used for BOTH the list and the export,
  // so the file is always what the screen is showing (BR-7.4).
  const params = new URLSearchParams()
  if (q) params.set('q', q)
  if (status) params.set('status', status)
  if (mode) params.set('order_mode', mode)
  const qs = params.toString()

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    api<{ data: Plan[]; total: number }>(`/promotions${qs ? '?' + qs : ''}`)
      .then((res) => { if (!cancelled) { setRows(res.data ?? []); setTotal(res.total) } })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [qs])

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
          title="Tidak ada rencana yang cocok"
          hint={q || status || mode
            ? 'Ubah kata kunci atau filter di atas.'
            : can('promo.create') ? 'Mulai dengan tombol “Buat rencana”.' : 'Belum ada rencana promo yang dibuat.'}
        />
      ) : (
        <>
          <TableWrap>
            <table className="table">
              <thead>
                <tr>
                  <th>Kode</th><th>Nama promo</th><th>Merek</th><th>Kelompok toko</th>
                  <th>Periode</th><th>Mode</th>
                  <th className="text-right">Target penjualan</th>
                  <th className="text-right">Target struk</th>
                  <th>Status</th><th>Langkah</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((p) => (
                  <tr key={p.plan_id} style={{ borderLeft: `3px solid ${brand(p.company_code)}` }}>
                    <td className="font-mono text-xs">
                      <Link className="text-accent-ink underline" to={`/promo/${p.plan_id}`}>{p.plan_code}</Link>
                    </td>
                    <td className="font-semibold">{p.version.promo_name}</td>
                    <td><BrandTag code={p.company_code} name={p.company_name} /></td>
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
          <p className="text-xs text-muted">{total} rencana</p>
        </>
      )}
    </div>
  )
}

function brand(code: string) {
  return code === 'MAXX' ? '#6b3b2a' : code === 'RUUMA' ? '#7a2e63'
    : code === 'SUNSHINE' ? '#7c4a00' : 'transparent'
}
