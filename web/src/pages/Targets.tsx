import { useEffect, useMemo, useState } from 'react'
import { api, download, type Company, type Site } from '../lib/api'
import { useAuth } from '../lib/auth'
import { rp, rupiah, percentFromBPS, monthName } from '../lib/format'
import { SearchBox, ExportButton, TableWrap, Loading, Empty, ErrorBox } from '../components/ui'

interface TargetRow {
  target_id: string; site_id: string; site_code: string; site_name: string
  period_kind: 'YEAR' | 'MONTH'; year: number; month: number
  sales_type: 'normal' | 'promo'; target_amount_idr: number
}

export default function Targets() {
  const { can } = useAuth()
  const [rows, setRows] = useState<TargetRow[]>([])
  const [sites, setSites] = useState<Site[]>([])
  const [companies, setCompanies] = useState<Company[]>([])
  const [year, setYear] = useState(new Date().getFullYear())
  const [salesType, setSalesType] = useState<'normal' | 'promo'>('normal')
  const [q, setQ] = useState('')
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)
  const [editing, setEditing] = useState<{ siteId: string; month: number } | null>(null)
  const [draft, setDraft] = useState('')

  const params = new URLSearchParams({ year: String(year), sales_type: salesType })
  if (q) params.set('q', q)
  const qs = params.toString()

  async function load() {
    setLoading(true)
    try {
      const [t, s, c] = await Promise.all([
        api<{ data: TargetRow[] }>(`/targets?${qs}`),
        api<{ data: Site[] }>('/sites'),
        api<{ data: Company[] }>('/companies'),
      ])
      setRows(t.data ?? []); setSites(s.data ?? []); setCompanies(c.data ?? [])
    } finally { setLoading(false) }
  }
  useEffect(() => { load() /* eslint-disable-next-line */ }, [qs])

  // Grouped per site: the twelve months plus the year cell, and the variance
  // between them — which is a DISPLAY value and never an error (BR-2.3).
  const bySite = useMemo(() => {
    const map = new Map<string, { code: string; name: string; months: (number | null)[]; year: number | null }>()
    for (const r of rows) {
      const cur = map.get(r.site_id) ?? {
        code: r.site_code, name: r.site_name,
        months: Array<number | null>(12).fill(null), year: null,
      }
      if (r.period_kind === 'YEAR') cur.year = r.target_amount_idr
      else cur.months[r.month - 1] = r.target_amount_idr
      map.set(r.site_id, cur)
    }
    return [...map.entries()].sort((a, b) => a[1].code.localeCompare(b[1].code))
  }, [rows])

  async function saveCell(siteId: string, month: number, value: string) {
    const site = sites.find((s) => s.site_id === siteId)
    if (!site) return
    const amount = Number(value.replace(/\D/g, '') || 0)
    setErr(null)
    try {
      await api('/targets', {
        method: 'PUT',
        body: {
          company_id: site.company_id, site_id: siteId,
          period_kind: month === 0 ? 'YEAR' : 'MONTH',
          year, month: month === 0 ? 0 : month,
          sales_type: salesType, target_amount_idr: amount,
        },
      })
      setEditing(null)
      await load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Tidak dapat menyimpan target')
    }
  }

  const canEdit = can('target.manage')

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="kicker">{companies.map((c) => c.company_name).join(' · ')}</div>
          <h3>Target {year}</h3>
        </div>
        <div className="flex flex-wrap gap-2 items-end">
          <div className="w-full sm:w-64">
            <SearchBox value={q} onChange={setQ} placeholder="Cari toko…" />
          </div>
          <div>
            <label className="label" htmlFor="year">Tahun</label>
            <input id="year" className="input w-28 tnum" type="number" value={year}
                   onChange={(e) => setYear(Number(e.target.value) || year)} />
          </div>
          <div>
            <label className="label" htmlFor="stype">Jenis</label>
            <select id="stype" className="input w-36" value={salesType}
                    onChange={(e) => setSalesType(e.target.value as 'normal' | 'promo')}>
              <option value="normal">Normal</option>
              <option value="promo">Promo</option>
            </select>
          </div>
          {can('report.export') && (
            <ExportButton onExport={() => download(`/targets/export?${qs}`, 'target.csv')} />
          )}
        </div>
      </div>

      {err && <ErrorBox message={err} onDismiss={() => setErr(null)} />}

      {loading ? <Loading /> : bySite.length === 0 ? (
        <Empty title="Belum ada target untuk tahun ini"
               hint={canEdit ? 'Klik sel mana pun untuk mengisi target bulanan.' : 'Hubungi Kepala Marketing atau Keuangan.'} />
      ) : (
        <>
          <TableWrap>
            <table className="table">
              <thead>
                <tr>
                  <th className="sticky left-0 bg-paper">Toko</th>
                  {Array.from({ length: 12 }, (_, i) => (
                    <th key={i} className="text-right">{monthName(i + 1).slice(0, 3)}</th>
                  ))}
                  <th className="text-right">Jumlah bulan</th>
                  <th className="text-right">Target tahun</th>
                  <th className="text-right">Selisih</th>
                </tr>
              </thead>
              <tbody>
                {bySite.map(([siteId, s]) => {
                  const sum = s.months.reduce<number>((a, b) => a + (b ?? 0), 0)
                  const delta = s.year === null ? null : sum - s.year
                  const bps = s.year ? Math.round((sum * 10000) / s.year) - 10000 : null
                  return (
                    <tr key={siteId}>
                      <td className="sticky left-0 bg-paper whitespace-nowrap">
                        <div className="font-semibold text-xs">{s.code}</div>
                        <div className="text-xs text-muted">{s.name}</div>
                      </td>
                      {s.months.map((m, i) => (
                        <td key={i} className="text-right tnum p-0">
                          {editing?.siteId === siteId && editing.month === i + 1 ? (
                            <input autoFocus className="input tnum text-right w-28 h-9"
                                   value={draft}
                                   onChange={(e) => setDraft(e.target.value.replace(/\D/g, ''))}
                                   onBlur={() => saveCell(siteId, i + 1, draft)}
                                   onKeyDown={(e) => {
                                     if (e.key === 'Enter') saveCell(siteId, i + 1, draft)
                                     if (e.key === 'Escape') setEditing(null)
                                   }} />
                          ) : (
                            <button
                              className="w-full h-9 px-2 text-right tnum disabled:cursor-default"
                              disabled={!canEdit}
                              title={canEdit ? 'Klik untuk mengubah' : 'Anda tidak memiliki izin target.manage'}
                              onClick={() => { setEditing({ siteId, month: i + 1 }); setDraft(String(m ?? '')) }}
                            >
                              {m === null ? <span className="text-muted">—</span> : rupiah(m)}
                            </button>
                          )}
                        </td>
                      ))}
                      <td className="text-right tnum font-semibold">{rupiah(sum)}</td>
                      <td className="text-right tnum">
                        {s.year === null ? <span className="text-muted">—</span> : rupiah(s.year)}
                      </td>
                      {/* Deliberately NOT an achievement badge.
                          BR-2.3: the months need not sum to the year, and this
                          difference is a display value that is never an error.
                          A strong red block here would assert a failure the
                          rule explicitly says does not exist. Bolder, yes;
                          judgemental, no. */}
                      <td className="text-right tnum">
                        {delta === null ? <span className="text-muted">—</span> : (
                          <span className={`font-extrabold ${delta === 0 ? '' : delta > 0 ? 'text-success' : 'text-warn'}`}>
                            {delta > 0 ? '+' : ''}{rupiah(delta)}
                            {bps !== null && <span className="block text-[11px] font-normal text-muted">
                              {percentFromBPS(bps)}
                            </span>}
                          </span>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </TableWrap>

          <div className="card text-sm">
            <p className="font-semibold">Selisih bukan kesalahan.</p>
            <p className="mt-0.5 text-muted">
              Jumlah dua belas bulan tidak harus sama dengan target tahun. Sistem
              menampilkan selisihnya dan tidak pernah menolaknya — sengaja tidak
              ada validasi yang memaksa keduanya cocok.
            </p>
            <p className="mt-1 text-muted">
              Total target tahun yang tampil: <b className="tnum">
                {rp(bySite.reduce((a, [, s]) => a + (s.year ?? 0), 0))}
              </b>
            </p>
          </div>
        </>
      )}
    </div>
  )
}
