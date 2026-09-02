import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import { formatDateTime } from '../lib/format'
import { SearchBox, TableWrap, Loading, Empty } from '../components/ui'

interface Row {
  audit_id: string; actor_name: string; action: string
  subject_type: string; subject_id: string | null
  reason: string; ip_address: string; occurred_at: string
}

export default function Audit() {
  const [rows, setRows] = useState<Row[]>([])
  const [total, setTotal] = useState(0)
  const [q, setQ] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    api<{ data: Row[]; total: number }>(`/audit${q ? '?q=' + encodeURIComponent(q) : ''}`)
      .then((r) => { if (!cancelled) { setRows(r.data ?? []); setTotal(r.total) } })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [q])

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="kicker">Tata kelola</div>
          <h3>Jejak audit</h3>
        </div>
        <div className="w-full sm:w-80">
          <SearchBox value={q} onChange={setQ} placeholder="Cari tindakan, pelaku atau objek…" />
        </div>
      </div>

      <div className="card text-sm">
        <p className="font-semibold">Append-only, tanpa batas waktu simpan.</p>
        <p className="text-muted mt-0.5">
          Baris di sini tidak dapat diubah, dihapus, maupun dikosongkan — basis
          data menolaknya, termasuk lewat TRUNCATE. Tidak ada pekerjaan
          pembersihan, dan membuatnya berarti membalik keputusan, bukan merapikan.
        </p>
      </div>

      {loading ? <Loading /> : rows.length === 0 ? (
        <Empty title="Tidak ada catatan yang cocok" hint="Ubah kata kunci pencarian." />
      ) : (
        <>
          <TableWrap>
            <table className="table">
              <thead>
                <tr><th>Waktu</th><th>Pelaku</th><th>Tindakan</th><th>Objek</th><th>Alasan</th><th>IP</th></tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr key={r.audit_id}>
                    <td className="text-xs whitespace-nowrap">{formatDateTime(r.occurred_at)}</td>
                    <td className="text-xs font-semibold">{r.actor_name}</td>
                    <td className="font-mono text-xs">{r.action}</td>
                    <td className="text-xs text-muted">{r.subject_type}</td>
                    <td className="text-xs max-w-md">{r.reason || '—'}</td>
                    <td className="text-xs font-mono text-muted">{r.ip_address || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </TableWrap>
          <p className="text-xs text-muted">{total} catatan</p>
        </>
      )}
    </div>
  )
}
