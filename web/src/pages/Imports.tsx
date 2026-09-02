import { useEffect, useState } from 'react'
import { api, ApiFailure, getToken } from '../lib/api'
import { useAuth } from '../lib/auth'
import { formatDateTime, rp, count } from '../lib/format'
import { SearchBox, TableWrap, Loading, Empty, ErrorBox, Modal } from '../components/ui'

interface Run {
  import_run_id: string; file_name: string; file_checksum: string
  rows_read: number; rows_inserted: number; rows_skipped: number; rows_rejected: number
  trailer_rows: number | null; trailer_total_idr: number | null
  outcome: string; message: string; started_at: string; finished_at: string | null
}
interface Rejection { line_no: number; reason: string; original_line: string }

const REASON_HELP: Record<string, string> = {
  UNKNOWN_SITE: 'Kode toko tidak ada di master data. Buat tokonya, lalu impor ulang.',
  UNKNOWN_PROMO: 'Kode promo bukan rencana yang sudah dirilis. Periksa pemetaan di POS.',
  PROMO_INCONSISTENT: 'Baris promo tanpa kode promo, atau baris normal yang membawa kode promo. Cacat ekspor POS.',
  DUPLICATE_RECEIPT: 'Struk sudah pernah dimuat. Informasional saja.',
  NEGATIVE_AMOUNT: 'Nilai di bawah nol. Tidak pernah dikoreksi diam-diam.',
  BAD_AMOUNT: 'Nilai bukan rupiah bulat. "185.000" ditolak karena ambigu antara pemisah ribuan dan desimal.',
  BAD_DATE: 'Tanggal bukan format YYYY-MM-DD.',
  BAD_ENUM: 'sales_type atau order_mode di luar daftar yang diizinkan.',
  SHORT_ROW: 'Baris memiliki kolom lebih sedikit daripada kontrak.',
  BLANK_RECEIPT: 'Nomor struk kosong.',
}

export default function Imports() {
  const { can } = useAuth()
  const [runs, setRuns] = useState<Run[]>([])
  const [loading, setLoading] = useState(true)
  const [q, setQ] = useState('')
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [detail, setDetail] = useState<{ run: Run; rejections: Rejection[] } | null>(null)

  async function load() {
    setLoading(true)
    try { setRuns((await api<{ data: Run[] }>('/imports')).data ?? []) }
    finally { setLoading(false) }
  }
  useEffect(() => { load() }, [])

  async function runImport() {
    setBusy(true); setErr(null)
    try { await api('/imports/run', { method: 'POST', body: {} }); await load() }
    catch (e) { setErr(e instanceof ApiFailure ? e.body.message : 'Impor gagal') }
    finally { setBusy(false) }
  }

  async function upload(file: File) {
    setBusy(true); setErr(null)
    const fd = new FormData()
    fd.append('file', file)
    try {
      // The token lives in the api module, never in localStorage: a token in
      // localStorage is readable by any script that ends up on the page.
      const token = getToken()
      const res = await fetch('/api/v1/imports/upload', {
        method: 'POST',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
        body: fd, credentials: 'same-origin',
      })
      if (!res.ok) throw new Error((await res.json()).message ?? 'Unggahan ditolak')
      await load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Unggahan gagal')
    } finally { setBusy(false) }
  }

  async function openDetail(run: Run) {
    const r = await api<{ data: Rejection[] }>(`/imports/${run.import_run_id}/rejections`)
    setDetail({ run, rejections: r.data ?? [] })
  }

  const shown = q ? runs.filter((r) => r.file_name.toLowerCase().includes(q.toLowerCase())) : runs

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="kicker">Rekonsiliasi</div>
          <h3>Impor transaksi</h3>
        </div>
        <div className="flex flex-wrap gap-2 items-end">
          <div className="w-full sm:w-64">
            <SearchBox value={q} onChange={setQ} placeholder="Cari nama berkas…" />
          </div>
          {can('import.run') && (
            <>
              <label className="btn btn-secondary cursor-pointer">
                Unggah berkas
                <input type="file" accept=".csv" className="sr-only"
                       onChange={(e) => { const f = e.target.files?.[0]; if (f) upload(f) }} />
              </label>
              <button className="btn btn-primary" disabled={busy} onClick={runImport}>
                {busy ? 'Memproses…' : 'Jalankan impor'}
              </button>
            </>
          )}
        </div>
      </div>

      {err && <ErrorBox message={err} onDismiss={() => setErr(null)} />}

      <div className="card text-sm">
        <div className="kicker mb-1">Kontrak berkas</div>
        <code className="block bg-paper p-2 border border-hairline text-xs overflow-x-auto whitespace-pre">
{`site_code|business_date|pos_receipt_no|sales_type|promo_code|order_mode|gross_amount_idr
MXX-001|2026-09-01|R-000198231|promo|PRM-7QK2|dine_in|185000
MXX-001|2026-09-01|R-000198232|normal||take_away|42000
#TOTAL|2|227000`}
        </code>
        <p className="text-muted mt-2 text-xs">
          Pemisah adalah pipa. Header dicocokkan <b>berdasarkan nama</b>, bukan
          urutan. Baris <code>#TOTAL</code> wajib — itulah satu-satunya hal yang
          menangkap unggahan terpotong, yang jika tidak akan terlihat persis
          seperti hari sepi. Identitas berkas adalah checksum-nya, bukan namanya.
        </p>
      </div>

      {loading ? <Loading /> : shown.length === 0 ? (
        <Empty title="Belum ada riwayat impor"
               hint={can('import.run') ? 'Letakkan berkas di direktori drop lalu jalankan impor.' : 'Impor dijalankan oleh IT.'} />
      ) : (
        <TableWrap>
          <table className="table">
            <thead>
              <tr>
                <th>Berkas</th><th>Waktu</th><th>Hasil</th>
                <th className="text-right">Dibaca</th>
                <th className="text-right">Dimasukkan</th>
                <th className="text-right">Dilewati</th>
                <th className="text-right">Ditolak</th>
                <th>Catatan</th>
              </tr>
            </thead>
            <tbody>
              {shown.map((r) => (
                <tr key={r.import_run_id}>
                  <td className="font-mono text-xs">{r.file_name}</td>
                  <td className="text-xs whitespace-nowrap">{formatDateTime(r.started_at)}</td>
                  <td>
                    <span className={`pill ${
                      r.outcome === 'OK' ? 'bg-[#e8f2ec] text-success'
                      : r.outcome === 'PARTIAL' ? 'bg-[#fff2ef] text-warn'
                      : r.outcome === 'SKIPPED' ? 'bg-neutral-200 text-neutral-800'
                      : 'bg-[#fdeaec] text-danger'}`}>
                      <span aria-hidden="true">
                        {r.outcome === 'OK' ? '✓' : r.outcome === 'PARTIAL' ? '◷' : r.outcome === 'SKIPPED' ? '⊘' : '✕'}
                      </span>{' '}
                      {r.outcome === 'OK' ? 'Berhasil' : r.outcome === 'PARTIAL' ? 'Sebagian'
                        : r.outcome === 'SKIPPED' ? 'Dilewati' : 'Gagal'}
                    </span>
                  </td>
                  <td className="text-right tnum">{count(r.rows_read)}</td>
                  <td className="text-right tnum">{count(r.rows_inserted)}</td>
                  <td className="text-right tnum">{count(r.rows_skipped)}</td>
                  <td className="text-right tnum">
                    {r.rows_rejected > 0 ? (
                      <button className="text-accent-ink underline" onClick={() => openDetail(r)}>
                        {count(r.rows_rejected)}
                      </button>
                    ) : count(r.rows_rejected)}
                  </td>
                  <td className="text-xs text-muted">
                    {r.message}
                    {r.trailer_total_idr !== null && (
                      <div className="tnum">Trailer: {rp(r.trailer_total_idr)}</div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableWrap>
      )}

      {detail && (
        <Modal title={`Baris ditolak — ${detail.run.file_name}`} onClose={() => setDetail(null)}>
          <p className="text-muted mb-2">
            Baris yang ditolak tidak pernah hilang diam-diam: alasan dan baris
            aslinya disimpan.
          </p>
          <div className="max-h-80 overflow-y-auto flex flex-col gap-2">
            {detail.rejections.map((r, i) => (
              <div key={i} className="card-paper text-xs">
                <div className="font-semibold">Baris {r.line_no} · {r.reason}</div>
                <div className="text-muted mt-0.5">{REASON_HELP[r.reason] ?? ''}</div>
                <code className="block mt-1 break-all">{r.original_line}</code>
              </div>
            ))}
          </div>
        </Modal>
      )}
    </div>
  )
}
