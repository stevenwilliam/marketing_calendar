import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type Plan } from '../lib/api'
import { formatDate, rp, count } from '../lib/format'
import { Loading, Empty, BrandTag, SearchBox } from '../components/ui'

export default function Inbox() {
  const [rows, setRows] = useState<Plan[]>([])
  const [loading, setLoading] = useState(true)
  const [q, setQ] = useState('')

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    api<{ data: Plan[]; total: number }>('/approvals/inbox')
      .then((r) => { if (!cancelled) setRows(r.data ?? []) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [])

  const shown = q
    ? rows.filter((p) =>
        (p.version.promo_name + p.plan_code + p.site_group_name + p.company_name)
          .toLowerCase().includes(q.toLowerCase()))
    : rows

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="kicker">Persetujuan</div>
          <h3>Menunggu keputusan Anda</h3>
        </div>
        <div className="w-full sm:w-72">
          <SearchBox value={q} onChange={setQ} placeholder="Cari di kotak persetujuan…" />
        </div>
      </div>

      {loading ? <Loading /> : shown.length === 0 ? (
        <Empty
          title="Tidak ada yang menunggu Anda"
          hint={
            'Kotak ini hanya menampilkan rencana yang berada pada langkah dengan peran ' +
            'yang Anda pegang di merek rencana itu — dan bukan rencana yang Anda buat sendiri.'
          }
        />
      ) : (
        <div className="flex flex-col gap-2">
          {shown.map((p) => (
            <Link key={p.plan_id} to={`/promo/${p.plan_id}`}
                  className="card-paper hover:bg-surface flex flex-col gap-1.5"
                  style={{ borderLeft: `3px solid ${brand(p.company_code)}` }}>
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-mono text-xs">{p.plan_code}</span>
                <span className="font-semibold">{p.version.promo_name}</span>
                <BrandTag code={p.company_code} name={p.company_name} />
                <span className="ml-auto pill bg-[#fff2ef] text-warn">
                  <span aria-hidden="true">◷</span> Langkah {p.current_step_no}: {p.current_step_name}
                </span>
              </div>
              <div className="flex flex-wrap gap-4 text-xs text-muted">
                <span>{p.site_group_name}</span>
                <span>{formatDate(p.version.start_date)} – {formatDate(p.version.end_date)}</span>
                <span className="tnum">{rp(p.version.target_sales_idr)}</span>
                <span className="tnum">{count(p.version.target_receipt_count)} struk</span>
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}

function brand(code: string) {
  return code === 'MAXX' ? '#6b3b2a' : code === 'RUUMA' ? '#7a2e63'
    : code === 'SUNSHINE' ? '#7c4a00' : '#201e1d'
}
