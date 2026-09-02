import { useEffect, useState } from 'react'
import { api, download } from '../lib/api'
import { useAuth } from '../lib/auth'
import { formatDate, rp, count, percentFromBPS, monthName } from '../lib/format'
import { SearchBox, ExportButton, TableWrap, Loading, Empty, StatusPill } from '../components/ui'

interface PromoReportRow {
  plan_id: string; plan_code: string; promo_name: string
  company_name: string; site_group_name: string
  start_date: string; end_date: string; order_mode: string
  status: string; force_released: boolean
  target_sales_idr: number; actual_sales_idr: number; sales_delta_idr: number
  achieved_bps: number | null
  target_receipts: number; actual_receipts: number; receipt_delta: number
}

interface TargetReportRow {
  company_name: string; site_code: string; site_name: string
  year: number; month: number; sales_type: string
  target_idr: number; actual_idr: number; delta_idr: number
  achieved_bps: number | null; receipt_count: number
}

export default function Reports() {
  const { can } = useAuth()
  const [tab, setTab] = useState<'promo' | 'target'>('promo')
  return (
    <div className="flex flex-col gap-4">
      <div>
        <div className="kicker">Laporan</div>
        <h3>{tab === 'promo' ? 'Rencana versus aktual promo' : 'Target versus aktual'}</h3>
      </div>
      <div className="inline-flex border border-divider self-start">
        {(['promo', 'target'] as const).map((t, i) => (
          <button key={t}
                  className={`px-4 py-2 text-sm font-semibold ${i > 0 ? 'border-l border-divider' : ''}`}
                  style={tab === t ? { background: '#ae1800', color: '#f3f2f2' } : undefined}
                  onClick={() => setTab(t)}>
            {t === 'promo' ? 'Laporan promo' : 'Target vs aktual'}
          </button>
        ))}
      </div>
      {tab === 'promo' ? <PromoReport canExport={can('report.export')} />
                       : <TargetReport canExport={can('report.export')} />}
    </div>
  )
}

function PromoReport({ canExport }: { canExport: boolean }) {
  const [rows, setRows] = useState<PromoReportRow[]>([])
  const [q, setQ] = useState('')
  const [status, setStatus] = useState('')
  const [loading, setLoading] = useState(true)

  const params = new URLSearchParams()
  if (q) params.set('q', q)
  if (status) params.set('status', status)
  const qs = params.toString()

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    api<{ data: PromoReportRow[] }>(`/reports/promotions${qs ? '?' + qs : ''}`)
      .then((r) => { if (!cancelled) setRows(r.data ?? []) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [qs])

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-2 items-end">
        <div className="w-full sm:w-72">
          <SearchBox value={q} onChange={setQ} placeholder="Cari promo…" />
        </div>
        <div>
          <label className="label" htmlFor="r-status">Status</label>
          <select id="r-status" className="input w-44" value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="">Semua status</option>
            <option value="RELEASED">Dirilis</option>
            <option value="PENDING">Menunggu</option>
            <option value="CANCELLED">Dibatalkan</option>
          </select>
        </div>
        <div className="ml-auto">
          {canExport && <ExportButton onExport={() => download(`/reports/promotions/export${qs ? '?' + qs : ''}`, 'laporan-promo.csv')} />}
        </div>
      </div>

      {loading ? <Loading /> : rows.length === 0 ? (
        <Empty title="Belum ada promo yang cocok" hint="Ubah filter, atau tunggu promo pertama dirilis." />
      ) : (
        <TableWrap>
          <table className="table">
            <thead>
              <tr>
                <th>Kode</th><th>Promo</th><th>Merek</th><th>Periode</th>
                <th className="text-right">Target</th>
                <th className="text-right">Aktual</th>
                <th className="text-right">Selisih</th>
                <th className="text-right">Capaian</th>
                <th className="text-right">Struk target</th>
                <th className="text-right">Struk aktual</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={r.plan_id}>
                  <td className="font-mono text-xs">{r.plan_code}</td>
                  <td className="font-semibold">{r.promo_name}</td>
                  <td className="text-xs">{r.company_name}</td>
                  <td className="whitespace-nowrap text-xs">
                    {formatDate(r.start_date)} – {formatDate(r.end_date)}
                  </td>
                  <td className="text-right tnum">{rp(r.target_sales_idr)}</td>
                  <td className="text-right tnum">{rp(r.actual_sales_idr)}</td>
                  <td className={`text-right tnum ${r.sales_delta_idr >= 0 ? 'text-success' : 'text-danger'}`}>
                    {r.sales_delta_idr > 0 ? '+' : ''}{rp(r.sales_delta_idr)}
                  </td>
                  <td className="text-right tnum">
                    {/* A percentage of a zero target is undefined, not 0% — a
                        site with no target must not read as a total miss. */}
                    {r.achieved_bps === null
                      ? <span className="text-muted" title="Target nol: capaian tidak terdefinisi">—</span>
                      : percentFromBPS(r.achieved_bps)}
                  </td>
                  <td className="text-right tnum">{count(r.target_receipts)}</td>
                  <td className="text-right tnum">{count(r.actual_receipts)}</td>
                  <td><StatusPill status={r.status} forceReleased={r.force_released} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableWrap>
      )}
      <p className="text-xs text-muted">
        Aktual dihitung dari transaksi yang <b>ditandai dengan id promo</b>, bukan
        dari rentang tanggal. Transaksi di dalam periode promo yang tidak ditandai
        adalah penjualan normal.
      </p>
    </div>
  )
}

function TargetReport({ canExport }: { canExport: boolean }) {
  const [rows, setRows] = useState<TargetReportRow[]>([])
  const [year, setYear] = useState(new Date().getFullYear())
  const [month, setMonth] = useState(0)
  const [q, setQ] = useState('')
  const [loading, setLoading] = useState(true)

  const params = new URLSearchParams({ year: String(year) })
  if (month) params.set('month', String(month))
  if (q) params.set('q', q)
  const qs = params.toString()

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    api<{ data: TargetReportRow[] }>(`/reports/targets?${qs}`)
      .then((r) => { if (!cancelled) setRows(r.data ?? []) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [qs])

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-2 items-end">
        <div className="w-full sm:w-64">
          <SearchBox value={q} onChange={setQ} placeholder="Cari toko…" />
        </div>
        <div>
          <label className="label" htmlFor="tr-year">Tahun</label>
          <input id="tr-year" className="input w-28 tnum" type="number" value={year}
                 onChange={(e) => setYear(Number(e.target.value) || year)} />
        </div>
        <div>
          <label className="label" htmlFor="tr-month">Bulan</label>
          <select id="tr-month" className="input w-40" value={month}
                  onChange={(e) => setMonth(Number(e.target.value))}>
            <option value={0}>Setahun penuh</option>
            {Array.from({ length: 12 }, (_, i) => (
              <option key={i + 1} value={i + 1}>{monthName(i + 1)}</option>
            ))}
          </select>
        </div>
        <div className="ml-auto">
          {canExport && <ExportButton onExport={() => download(`/reports/targets/export?${qs}`, 'target-vs-aktual.csv')} />}
        </div>
      </div>

      {loading ? <Loading /> : rows.length === 0 ? (
        <Empty title="Tidak ada data untuk periode ini"
               hint="Pilih tahun atau bulan lain, atau periksa apakah impor transaksi sudah berjalan." />
      ) : (
        <TableWrap>
          <table className="table">
            <thead>
              <tr>
                <th>Merek</th><th>Toko</th><th>Periode</th><th>Jenis</th>
                <th className="text-right">Target</th>
                <th className="text-right">Aktual</th>
                <th className="text-right">Selisih</th>
                <th className="text-right">Capaian</th>
                <th className="text-right">Struk</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r, i) => (
                <tr key={i}>
                  <td className="text-xs">{r.company_name}</td>
                  <td>
                    <div className="font-semibold text-xs">{r.site_code}</div>
                    <div className="text-xs text-muted">{r.site_name}</div>
                  </td>
                  <td className="text-xs">{r.month ? monthName(r.month) : ''} {r.year}</td>
                  <td className="text-xs">{r.sales_type === 'promo' ? 'Promo' : 'Normal'}</td>
                  <td className="text-right tnum">{rp(r.target_idr)}</td>
                  <td className="text-right tnum">{rp(r.actual_idr)}</td>
                  <td className={`text-right tnum ${r.delta_idr >= 0 ? 'text-success' : 'text-danger'}`}>
                    {r.delta_idr > 0 ? '+' : ''}{rp(r.delta_idr)}
                  </td>
                  <td className="text-right tnum">
                    {r.achieved_bps === null
                      ? <span className="text-muted" title="Target nol: capaian tidak terdefinisi">—</span>
                      : percentFromBPS(r.achieved_bps)}
                  </td>
                  <td className="text-right tnum">{count(r.receipt_count)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableWrap>
      )}
    </div>
  )
}
