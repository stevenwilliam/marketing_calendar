import { useEffect, useState } from 'react'
import { api, ApiFailure, type Company, type Site, type SiteGroup } from '../lib/api'
import { SearchBox, TableWrap, Loading, Empty, ErrorBox, Modal, BrandTag, Reason } from '../components/ui'

const SITE_TYPES = [
  { value: 'coffee_shop', label: 'Kedai kopi' },
  { value: 'restaurant', label: 'Restoran' },
  { value: 'catering', label: 'Katering' },
]

export default function MasterData() {
  const [tab, setTab] = useState<'sites' | 'groups' | 'holidays'>('sites')
  return (
    <div className="flex flex-col gap-4">
      <div>
        <div className="kicker">Administrasi</div>
        <h3>Master data</h3>
      </div>
      <div className="inline-flex border border-divider self-start">
        {([['sites', 'Toko'], ['groups', 'Kelompok toko'], ['holidays', 'Hari libur']] as const).map(([t, label], i) => (
          <button key={t} className={`px-4 py-2 text-sm font-semibold ${i > 0 ? 'border-l border-divider' : ''}`}
                  style={tab === t ? { background: '#ae1800', color: '#f3f2f2' } : undefined}
                  onClick={() => setTab(t)}>{label}</button>
        ))}
      </div>
      {tab === 'sites' ? <Sites /> : tab === 'groups' ? <Groups /> : <Holidays />}
    </div>
  )
}

function Sites() {
  const [rows, setRows] = useState<Site[]>([])
  const [companies, setCompanies] = useState<Company[]>([])
  const [q, setQ] = useState('')
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)
  const [form, setForm] = useState<null | Partial<Site>>(null)

  async function load() {
    setLoading(true)
    try {
      const [s, c] = await Promise.all([
        api<{ data: Site[] }>(`/sites${q ? '?q=' + encodeURIComponent(q) : ''}`),
        api<{ data: Company[] }>('/companies'),
      ])
      setRows(s.data ?? []); setCompanies(c.data ?? [])
    } finally { setLoading(false) }
  }
  useEffect(() => { load() /* eslint-disable-next-line */ }, [q])

  async function save() {
    if (!form) return
    setErr(null)
    try {
      if (form.site_id) {
        await api(`/sites/${form.site_id}`, { method: 'PUT', body: form })
      } else {
        await api('/sites', { method: 'POST', body: form })
      }
      setForm(null); await load()
    } catch (e) { setErr(e instanceof ApiFailure ? e.body.message : 'Tidak dapat menyimpan') }
  }

  const companyName = (id: string) => companies.find((c) => c.company_id === id)?.company_name ?? ''
  const companyCode = (id: string) => companies.find((c) => c.company_id === id)?.company_code ?? ''

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-2 items-end">
        <div className="w-full sm:w-72">
          <SearchBox value={q} onChange={setQ} placeholder="Cari kode atau nama toko…" />
        </div>
        <button className="btn btn-primary ml-auto" onClick={() => setForm({ is_active: true, site_type: 'coffee_shop' })}>
          Tambah toko
        </button>
      </div>
      {err && <ErrorBox message={err} onDismiss={() => setErr(null)} />}
      {loading ? <Loading /> : rows.length === 0 ? (
        <Empty title="Tidak ada toko yang cocok" hint="Ubah pencarian, atau tambahkan toko baru." />
      ) : (
        <TableWrap>
          <table className="table">
            <thead><tr><th>Kode</th><th>Nama</th><th>Merek</th><th>Jenis</th><th>Aktif</th><th></th></tr></thead>
            <tbody>
              {rows.map((s) => (
                <tr key={s.site_id}>
                  <td className="font-mono text-xs">{s.site_code}</td>
                  <td className="font-semibold">{s.site_name}</td>
                  <td><BrandTag code={companyCode(s.company_id)} name={companyName(s.company_id)} /></td>
                  <td className="text-xs">{SITE_TYPES.find((t) => t.value === s.site_type)?.label ?? s.site_type}</td>
                  <td>{s.is_active ? 'Ya' : 'Tidak'}</td>
                  <td className="text-right">
                    <button className="btn btn-ghost text-xs" onClick={() => setForm(s)}>Ubah</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableWrap>
      )}

      {form && (
        <Modal title={form.site_id ? 'Ubah toko' : 'Tambah toko'} onClose={() => setForm(null)}
               footer={<>
                 <button className="btn btn-secondary" onClick={() => setForm(null)}>Batal</button>
                 <button className="btn btn-primary" onClick={save}>Simpan</button>
               </>}>
          <div className="flex flex-col gap-3">
            <div>
              <label className="label" htmlFor="m-company">Merek</label>
              <select id="m-company" className="input" value={form.company_id ?? ''}
                      disabled={Boolean(form.site_id)}
                      onChange={(e) => setForm({ ...form, company_id: e.target.value })}>
                <option value="">Pilih merek…</option>
                {companies.map((c) => <option key={c.company_id} value={c.company_id}>{c.company_name}</option>)}
              </select>
              {form.site_id && <Reason>Toko tidak dapat berpindah merek.</Reason>}
            </div>
            <div>
              <label className="label" htmlFor="m-code">Kode toko</label>
              <input id="m-code" className="input" value={form.site_code ?? ''}
                     onChange={(e) => setForm({ ...form, site_code: e.target.value })} />
              <Reason>Unik per merek, bukan unik secara global.</Reason>
            </div>
            <div>
              <label className="label" htmlFor="m-name">Nama toko</label>
              <input id="m-name" className="input" value={form.site_name ?? ''}
                     onChange={(e) => setForm({ ...form, site_name: e.target.value })} />
            </div>
            <div>
              <label className="label" htmlFor="m-type">Jenis</label>
              <select id="m-type" className="input" value={form.site_type ?? 'coffee_shop'}
                      onChange={(e) => setForm({ ...form, site_type: e.target.value })}>
                {SITE_TYPES.map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
              </select>
            </div>
            <label className="inline-flex items-center gap-2 text-sm">
              <input type="checkbox" checked={form.is_active ?? true}
                     onChange={(e) => setForm({ ...form, is_active: e.target.checked })} />
              Aktif
            </label>
            {!form.site_id && (
              <Reason>
                Membuat toko juga membuat kelompok toko otomatis berisi toko itu
                saja, dalam satu transaksi — sehingga promo satu toko tidak
                memerlukan penyiapan apa pun.
              </Reason>
            )}
          </div>
        </Modal>
      )}
    </div>
  )
}

function Groups() {
  const [rows, setRows] = useState<SiteGroup[]>([])
  const [sites, setSites] = useState<Site[]>([])
  const [companies, setCompanies] = useState<Company[]>([])
  const [q, setQ] = useState('')
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)
  const [form, setForm] = useState<null | { site_group_id?: string; company_id: string; site_group_name: string; site_ids: string[] }>(null)

  async function load() {
    setLoading(true)
    try {
      const [g, s, c] = await Promise.all([
        api<{ data: SiteGroup[] }>(`/site-groups${q ? '?q=' + encodeURIComponent(q) : ''}`),
        api<{ data: Site[] }>('/sites'),
        api<{ data: Company[] }>('/companies'),
      ])
      setRows(g.data ?? []); setSites(s.data ?? []); setCompanies(c.data ?? [])
    } finally { setLoading(false) }
  }
  useEffect(() => { load() /* eslint-disable-next-line */ }, [q])

  async function save() {
    if (!form) return
    setErr(null)
    try {
      if (form.site_group_id) {
        await api(`/site-groups/${form.site_group_id}/members`, { method: 'PUT', body: { site_ids: form.site_ids } })
      } else {
        await api('/site-groups', { method: 'POST', body: form })
      }
      setForm(null); await load()
    } catch (e) { setErr(e instanceof ApiFailure ? e.body.message : 'Tidak dapat menyimpan') }
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-2 items-end">
        <div className="w-full sm:w-72">
          <SearchBox value={q} onChange={setQ} placeholder="Cari kelompok toko…" />
        </div>
        <button className="btn btn-primary ml-auto"
                onClick={() => setForm({ company_id: companies[0]?.company_id ?? '', site_group_name: '', site_ids: [] })}>
          Tambah kelompok
        </button>
      </div>
      {err && <ErrorBox message={err} onDismiss={() => setErr(null)} />}
      <div className="card text-sm">
        <p className="font-semibold">Kelompok toko selalu satu merek.</p>
        <p className="text-muted mt-0.5">
          Toko dari merek lain ditolak oleh basis data, bukan sekadar diperingatkan.
          Kampanye lintas merek dimodelkan sebagai satu rencana per merek.
        </p>
      </div>
      {loading ? <Loading /> : rows.length === 0 ? (
        <Empty title="Tidak ada kelompok yang cocok" />
      ) : (
        <TableWrap>
          <table className="table">
            <thead><tr><th>Nama</th><th>Merek</th><th className="text-right">Jumlah toko</th><th>Jenis</th><th></th></tr></thead>
            <tbody>
              {rows.map((g) => {
                const c = companies.find((x) => x.company_id === g.company_id)
                return (
                  <tr key={g.site_group_id}>
                    <td className="font-semibold">{g.site_group_name}</td>
                    <td><BrandTag code={c?.company_code ?? ''} name={c?.company_name ?? ''} /></td>
                    <td className="text-right tnum">{g.member_count}</td>
                    <td className="text-xs">{g.is_system ? 'Otomatis (satu toko)' : 'Manual'}</td>
                    <td className="text-right">
                      {g.is_system ? (
                        <span className="text-xs text-muted" title="Kelompok otomatis per toko tidak dapat diubah keanggotaannya">
                          Terkunci
                        </span>
                      ) : (
                        <button className="btn btn-ghost text-xs"
                                onClick={async () => {
                                  const full = await api<SiteGroup>(`/site-groups`).then(() => g)
                                  setForm({
                                    site_group_id: g.site_group_id, company_id: g.company_id,
                                    site_group_name: g.site_group_name, site_ids: full.site_ids ?? [],
                                  })
                                }}>
                          Ubah anggota
                        </button>
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </TableWrap>
      )}

      {form && (
        <Modal title={form.site_group_id ? 'Ubah anggota kelompok' : 'Tambah kelompok toko'}
               onClose={() => setForm(null)}
               footer={<>
                 <button className="btn btn-secondary" onClick={() => setForm(null)}>Batal</button>
                 <button className="btn btn-primary" onClick={save}>Simpan</button>
               </>}>
          <div className="flex flex-col gap-3">
            {!form.site_group_id && (
              <>
                <div>
                  <label className="label" htmlFor="g-company">Merek</label>
                  <select id="g-company" className="input" value={form.company_id}
                          onChange={(e) => setForm({ ...form, company_id: e.target.value, site_ids: [] })}>
                    {companies.map((c) => <option key={c.company_id} value={c.company_id}>{c.company_name}</option>)}
                  </select>
                </div>
                <div>
                  <label className="label" htmlFor="g-name">Nama kelompok</label>
                  <input id="g-name" className="input" value={form.site_group_name}
                         onChange={(e) => setForm({ ...form, site_group_name: e.target.value })} />
                </div>
              </>
            )}
            <fieldset>
              <legend className="label">Toko anggota</legend>
              <div className="max-h-64 overflow-y-auto border border-divider p-2 flex flex-col gap-1">
                {sites.filter((s) => s.company_id === form.company_id).map((s) => (
                  <label key={s.site_id} className="inline-flex items-center gap-2 text-sm">
                    <input type="checkbox" checked={form.site_ids.includes(s.site_id)}
                           onChange={(e) => setForm({
                             ...form,
                             site_ids: e.target.checked
                               ? [...form.site_ids, s.site_id]
                               : form.site_ids.filter((x) => x !== s.site_id),
                           })} />
                    <span className="font-mono text-xs">{s.site_code}</span> {s.site_name}
                  </label>
                ))}
              </div>
              <Reason>Hanya toko dari merek yang dipilih yang ditampilkan.</Reason>
            </fieldset>
          </div>
        </Modal>
      )}
    </div>
  )
}

interface Holiday {
  holiday_id: string; holiday_date: string; holiday_name: string
  is_active: boolean; is_provisional: boolean
}

function Holidays() {
  const [rows, setRows] = useState<Holiday[]>([])
  const [year, setYear] = useState(new Date().getFullYear())
  const [q, setQ] = useState('')
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)
  const [form, setForm] = useState<null | { holiday_date: string; holiday_name: string; is_active: boolean }>(null)

  async function load() {
    setLoading(true)
    try { setRows((await api<{ data: Holiday[] }>(`/holidays?year=${year}`)).data ?? []) }
    finally { setLoading(false) }
  }
  useEffect(() => { load() /* eslint-disable-next-line */ }, [year])

  async function save() {
    if (!form) return
    setErr(null)
    try { await api('/holidays', { method: 'POST', body: form }); setForm(null); await load() }
    catch (e) { setErr(e instanceof ApiFailure ? e.body.message : 'Tidak dapat menyimpan') }
  }

  const shown = q ? rows.filter((h) => h.holiday_name.toLowerCase().includes(q.toLowerCase())) : rows

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-2 items-end">
        <div className="w-full sm:w-64">
          <SearchBox value={q} onChange={setQ} placeholder="Cari hari libur…" />
        </div>
        <div>
          <label className="label" htmlFor="h-year">Tahun</label>
          <input id="h-year" className="input w-28 tnum" type="number" value={year}
                 onChange={(e) => setYear(Number(e.target.value) || year)} />
        </div>
        <button className="btn btn-primary ml-auto"
                onClick={() => setForm({ holiday_date: `${year}-01-01`, holiday_name: '', is_active: true })}>
          Tambah hari libur
        </button>
      </div>
      {err && <ErrorBox message={err} onDismiss={() => setErr(null)} />}
      <div className="card text-sm">
        <p className="font-semibold">Kalender ini menggerakkan masa tenggang.</p>
        <p className="text-muted mt-0.5">
          Tanpa hari libur, "7 hari kerja" dihitung dari hari kerja saja dan
          memberi tanggal yang tidak pernah sah di sekitar Idul Fitri, Natal dan
          Nyepi — dan kesalahannya tidak terlihat, karena angkanya tetap tujuh.
          Lima tahun sudah dimuat. Tanggal <b>tetap</b> (1 Jan, 1 Mei, 1 Jun,
          17 Agu, 25 Des) dan yang <b>diturunkan dari Paskah</b> (Wafat dan
          Kenaikan Isa Almasih) dihitung persis. Tanggal <b>kalender lunar</b> —
          Idul Fitri, Idul Adha, Nyepi, Waisak, Imlek, Maulid, Isra Mikraj dan
          seluruh cuti bersama — ditandai <b>Perkiraan</b>: ditetapkan surat
          keputusan bersama, biasanya setahun sebelumnya, dan tidak dapat
          dihitung. Ganti dengan tanggal resminya begitu terbit; mengimpor ulang
          menimpa perkiraan, dan tidak pernah menimpa yang sudah dikonfirmasi.
        </p>
      </div>
      {loading ? <Loading /> : shown.length === 0 ? (
        <Empty title="Belum ada hari libur untuk tahun ini"
               hint="Tambahkan hari libur nasional agar masa tenggang dihitung benar." />
      ) : (
        <TableWrap>
          <table className="table">
            <thead><tr><th>Tanggal</th><th>Nama</th><th>Sumber</th><th>Aktif</th></tr></thead>
            <tbody>
              {shown.map((h) => (
                <tr key={h.holiday_id}>
                  <td className="tnum whitespace-nowrap">{h.holiday_date.slice(0, 10)}</td>
                  <td>{h.holiday_name}</td>
                  <td>
                    {h.is_provisional ? (
                      <span className="pill bg-[#fff2ef] text-warn"
                            title="Perkiraan yang dihitung sistem. Belum dikonfirmasi terhadap surat keputusan bersama.">
                        <span aria-hidden="true">~</span> Perkiraan
                      </span>
                    ) : (
                      <span className="pill bg-[#e8f2ec] text-success">
                        <span aria-hidden="true">✓</span> Dikonfirmasi
                      </span>
                    )}
                  </td>
                  <td>{h.is_active ? 'Ya' : 'Tidak'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableWrap>
      )}
      {form && (
        <Modal title="Tambah hari libur" onClose={() => setForm(null)}
               footer={<>
                 <button className="btn btn-secondary" onClick={() => setForm(null)}>Batal</button>
                 <button className="btn btn-primary" onClick={save}>Simpan</button>
               </>}>
          <div className="flex flex-col gap-3">
            <div>
              <label className="label" htmlFor="h-date">Tanggal</label>
              <input id="h-date" className="input" type="date" value={form.holiday_date}
                     onChange={(e) => setForm({ ...form, holiday_date: e.target.value })} />
            </div>
            <div>
              <label className="label" htmlFor="h-name">Nama</label>
              <input id="h-name" className="input" value={form.holiday_name}
                     onChange={(e) => setForm({ ...form, holiday_name: e.target.value })} />
            </div>
          </div>
        </Modal>
      )}
    </div>
  )
}
