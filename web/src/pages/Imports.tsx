import { useEffect, useState } from 'react'
import { api, ApiFailure, download, getToken } from '../lib/api'
import { useAuth } from '../lib/auth'
import { formatDateTime, rp, count } from '../lib/format'
import { SearchBox, TableWrap, Loading, Empty, ErrorBox, Modal } from '../components/ui'
import { DropZone } from '../components/DropZone'

interface Run {
  import_run_id: string; file_name: string; file_checksum: string; kind: string
  rows_read: number; rows_inserted: number; rows_skipped: number; rows_rejected: number
  trailer_rows: number | null; trailer_total_idr: number | null
  outcome: string; message: string; started_at: string; finished_at: string | null
}
interface Rejection { line_no: number; reason: string; original_line: string }

const KIND_LABEL: Record<string, string> = {
  transactions: 'Transaksi',
  target_year: 'Target tahunan',
  target_month: 'Target bulanan',
}

// The three contracts, for the template row and the on-screen reference. The
// header text here is the same string the parser matches on — if it drifts,
// the template is wrong and the file it produces will be rejected.
const CONTRACTS = [
  {
    kind: 'transactions',
    label: 'Transaksi',
    note: 'Satu baris per struk. Nomor struk, tanggal dan toko bersama-sama menjadi kunci idempotensi.',
    header: 'site_code|business_date|pos_receipt_no|sales_type|promo_code|order_mode|gross_amount_idr',
    example: 'MXX-001|2026-09-01|R-000198231|promo|PRM-7QK2|dine_in|185000',
  },
  {
    kind: 'target_year',
    label: 'Target tahunan',
    note: 'Satu baris per toko, tahun dan jenis penjualan.',
    header: 'site_code|year|sales_type|target_amount_idr',
    example: 'MXX-001|2026|normal|9690000000',
  },
  {
    kind: 'target_month',
    label: 'Target bulanan',
    note: 'Satu baris per toko, bulan dan jenis penjualan. Jumlahnya tidak harus sama dengan target tahunan.',
    header: 'site_code|year|month|sales_type|target_amount_idr',
    example: 'MXX-001|2026|1|normal|850000000',
  },
] as const

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
  BAD_YEAR: 'Tahun bukan angka yang wajar (2000–2999).',
  BAD_MONTH: 'Bulan bukan angka 1–12.',
  DUPLICATE_IN_FILE: 'Target yang sama muncul dua kali dalam satu berkas. Baris terakhir akan menimpa yang pertama, jadi berkas ditolak sebelum itu terjadi.',
}

export default function Imports() {
  const { can } = useAuth()
  const [runs, setRuns] = useState<Run[]>([])
  const [loading, setLoading] = useState(true)
  const [q, setQ] = useState('')
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [detail, setDetail] = useState<{ run: Run; rejections: Rejection[] } | null>(null)
  const [kindFilter, setKindFilter] = useState('')
  const [progress, setProgress] = useState<
    { name: string; state: 'uploading' | 'done' | 'failed' | 'skipped'; detail?: string }[]
  >([])

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

  // Files are uploaded ONE AT A TIME, in order. Each is its own transaction on
  // the server, and sending them in parallel would let a corrected file land
  // before the file it corrects.
  async function upload(files: File[]) {
    setBusy(true); setErr(null); setProgress([])
    const token = getToken()
    for (const file of files) {
      setProgress((p) => [...p, { name: file.name, state: 'uploading' }])
      const fd = new FormData()
      fd.append('file', file)
      try {
        const res = await fetch('/api/v1/imports/upload', {
          method: 'POST',
          // The token lives in the api module, never in localStorage: a token
          // in localStorage is readable by any script on the page.
          headers: token ? { Authorization: `Bearer ${token}` } : {},
          body: fd, credentials: 'same-origin',
        })
        const payload = await res.json().catch(() => ({}))
        if (!res.ok) throw new Error(payload.message ?? `Ditolak (HTTP ${res.status})`)
        const run = payload.data?.run
        const skipped = payload.data?.skipped
        setProgress((p) => p.map((x) => x.name === file.name ? {
          name: file.name,
          state: skipped ? 'skipped' : run?.outcome === 'FAILED' ? 'failed' : 'done',
          detail: skipped
            ? 'sudah pernah dimuat'
            : `${KIND_LABEL[run?.kind] ?? run?.kind ?? ''} · ${run?.rows_inserted ?? 0} dimuat, ${run?.rows_rejected ?? 0} ditolak`,
        } : x))
      } catch (e) {
        setProgress((p) => p.map((x) => x.name === file.name ? {
          name: file.name, state: 'failed',
          detail: e instanceof Error ? e.message : 'gagal',
        } : x))
      }
    }
    await load()
    setBusy(false)
  }

  async function openDetail(run: Run) {
    const r = await api<{ data: Rejection[] }>(`/imports/${run.import_run_id}/rejections`)
    setDetail({ run, rejections: r.data ?? [] })
  }

  const shown = runs
    .filter((r) => !q || r.file_name.toLowerCase().includes(q.toLowerCase()))
    .filter((r) => !kindFilter || r.kind === kindFilter)

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="kicker">Rekonsiliasi</div>
          <h3>Impor data</h3>
        </div>
        <div className="flex flex-wrap gap-2 items-end">
          <div className="w-full sm:w-64">
            <SearchBox value={q} onChange={setQ} placeholder="Cari nama berkas…" />
          </div>
          <div>
            <label className="label" htmlFor="f-kind">Jenis</label>
            <select id="f-kind" className="input w-44" value={kindFilter}
                    onChange={(e) => setKindFilter(e.target.value)}>
              <option value="">Semua jenis</option>
              {CONTRACTS.map((k) => <option key={k.kind} value={k.kind}>{k.label}</option>)}
            </select>
          </div>
          {can('import.run') && (
            <button className="btn btn-primary" disabled={busy} onClick={runImport}>
              {busy ? 'Memproses…' : 'Jalankan impor dari direktori'}
            </button>
          )}
        </div>
      </div>

      {err && <ErrorBox message={err} onDismiss={() => setErr(null)} />}

      {can('import.run') && (
        <DropZone
          onFiles={upload}
          disabled={busy}
          hint="Boleh beberapa berkas sekaligus. Jenisnya dikenali dari baris header, bukan dari nama berkas."
        />
      )}

      {progress.length > 0 && (
        <ul className="flex flex-col gap-1 text-sm">
          {progress.map((f) => (
            <li key={f.name} className="card-paper flex flex-wrap items-center gap-2 py-2">
              <span aria-hidden="true" className={
                f.state === 'done' ? 'text-success' :
                f.state === 'failed' ? 'text-danger' :
                f.state === 'skipped' ? 'text-muted' : 'text-warn'}>
                {f.state === 'done' ? '✓' : f.state === 'failed' ? '✕' : f.state === 'skipped' ? '⊘' : '◷'}
              </span>
              <span className="font-mono text-xs">{f.name}</span>
              <span className="text-xs text-muted">{f.detail ?? 'mengunggah…'}</span>
            </li>
          ))}
        </ul>
      )}

      <div className="card text-sm">
        <div className="kicker mb-2">Kontrak berkas dan template</div>
        <p className="text-muted text-xs mb-3">
          Tiga jenis berkas, semuanya pipa sebagai pemisah. <b>Jenis dikenali
          dari baris header</b>, bukan dari nama berkas — berkas yang diganti
          namanya tetap diperlakukan sesuai isinya. Header dicocokkan
          <b> berdasarkan nama</b>, bukan urutan. Baris <code>#TOTAL</code> wajib:
          jumlah barisnya adalah satu-satunya hal yang menangkap unggahan
          terpotong, yang jika tidak akan terlihat persis seperti hari sepi.
          Identitas berkas adalah checksum-nya, bukan namanya.
        </p>
        <div className="flex flex-col gap-3">
          {CONTRACTS.map((k) => (
            <div key={k.kind}>
              <div className="flex flex-wrap items-center gap-2 mb-1">
                <span className="font-semibold">{k.label}</span>
                <button
                  className="btn btn-ghost text-xs"
                  onClick={() => download(`/imports/templates/${k.kind}`, `template_${k.kind}.csv`)}
                >
                  Unduh template
                </button>
              </div>
              <p className="text-xs text-muted mb-1">{k.note}</p>
              <code className="block bg-paper p-2 border border-hairline text-xs overflow-x-auto whitespace-pre">
{k.header}
{'\n'}{k.example}
              </code>
            </div>
          ))}
        </div>
      </div>

      {loading ? <Loading /> : shown.length === 0 ? (
        <Empty title="Belum ada riwayat impor"
               hint={can('import.run') ? 'Tarik berkas ke kotak di atas, atau letakkan di direktori drop lalu jalankan impor.' : 'Impor dijalankan oleh IT.'} />
      ) : (
        <TableWrap>
          <table className="table">
            <thead>
              <tr>
                <th>Berkas</th><th>Jenis</th><th>Waktu</th><th>Hasil</th>
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
                  <td className="text-xs">{KIND_LABEL[r.kind] ?? r.kind}</td>
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
