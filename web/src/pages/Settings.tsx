import { useEffect, useState } from 'react'
import { api, ApiFailure, type Company } from '../lib/api'
import { SearchBox, TableWrap, Loading, Empty, ErrorBox, Modal, Reason } from '../components/ui'

interface Param {
  param_key: string; param_value: string; value_type: string
  description: string; is_secret: boolean; updated_at: string
}
interface ChainStep {
  step_no: number; step_name: string; satisfaction: string
  role_ids: string[]; role_labels: string[]
}
interface Role { role_id: string; role_code: string; label_id: string }

export default function Settings() {
  const [tab, setTab] = useState<'params' | 'chain'>('params')
  return (
    <div className="flex flex-col gap-4">
      <div>
        <div className="kicker">Administrasi</div>
        <h3>Pengaturan</h3>
      </div>
      <div className="inline-flex border border-divider self-start">
        {([['params', 'Parameter sistem'], ['chain', 'Rantai persetujuan']] as const).map(([t, label], i) => (
          <button key={t} className={`px-4 py-2 text-sm font-semibold ${i > 0 ? 'border-l border-divider' : ''}`}
                  style={tab === t ? { background: '#ae1800', color: '#f3f2f2' } : undefined}
                  onClick={() => setTab(t)}>{label}</button>
        ))}
      </div>
      {tab === 'params' ? <Params /> : <Chain />}
    </div>
  )
}

function Params() {
  const [rows, setRows] = useState<Param[]>([])
  const [q, setQ] = useState('')
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)
  const [edit, setEdit] = useState<Param | null>(null)
  const [draft, setDraft] = useState('')

  async function load() {
    setLoading(true)
    try { setRows((await api<{ data: Param[] }>('/parameters')).data ?? []) }
    finally { setLoading(false) }
  }
  useEffect(() => { load() }, [])

  async function save() {
    if (!edit) return
    setErr(null)
    try {
      await api(`/parameters/${encodeURIComponent(edit.param_key)}`,
                { method: 'PUT', body: { param_value: draft } })
      setEdit(null); await load()
    } catch (e) { setErr(e instanceof ApiFailure ? e.body.message : 'Tidak dapat menyimpan') }
  }

  const shown = q
    ? rows.filter((p) => (p.param_key + p.description).toLowerCase().includes(q.toLowerCase()))
    : rows

  return (
    <div className="flex flex-col gap-3">
      <div className="w-full sm:w-80">
        <SearchBox value={q} onChange={setQ} placeholder="Cari parameter…" />
      </div>
      {err && <ErrorBox message={err} onDismiss={() => setErr(null)} />}
      <div className="card text-sm">
        <p className="font-semibold">Waktu operasional adalah parameter, bukan konstanta.</p>
        <p className="text-muted mt-0.5">
          Masa tenggang 7 hari kerja dan pembatalan otomatis 5 hari sebelum mulai
          adalah dua angka yang saling menentukan: dengan rantai lima langkah,
          rencana yang diajukan pada hari paling awal punya sekitar 4–6 hari
          kalender untuk lima persetujuan. Jika pembatalan otomatis mulai sering
          terjadi, naikkan masa tenggang atau turunkan hari pembatalan — tanpa
          deploy. Ukur dulu dari jejak persetujuan sebelum memilih angka baru.
        </p>
      </div>
      {loading ? <Loading /> : shown.length === 0 ? <Empty title="Tidak ada parameter yang cocok" /> : (
        <TableWrap>
          <table className="table">
            <thead><tr><th>Kunci</th><th>Nilai</th><th>Keterangan</th><th></th></tr></thead>
            <tbody>
              {shown.map((p) => (
                <tr key={p.param_key}>
                  <td className="font-mono text-xs">{p.param_key}</td>
                  <td className="font-semibold tnum">{p.param_value}</td>
                  <td className="text-xs text-muted max-w-md">{p.description}</td>
                  <td className="text-right">
                    <button className="btn btn-ghost text-xs"
                            onClick={() => { setEdit(p); setDraft(p.is_secret ? '' : p.param_value) }}>
                      Ubah
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableWrap>
      )}
      {edit && (
        <Modal title={edit.param_key} onClose={() => setEdit(null)}
               footer={<>
                 <button className="btn btn-secondary" onClick={() => setEdit(null)}>Batal</button>
                 <button className="btn btn-primary" onClick={save}>Simpan</button>
               </>}>
          <p className="text-muted mb-2">{edit.description}</p>
          <label className="label" htmlFor="p-value">Nilai</label>
          {edit.value_type === 'list' ? (
            <textarea id="p-value" className="input min-h-[80px]" value={draft}
                      onChange={(e) => setDraft(e.target.value)} />
          ) : (
            <input id="p-value" className="input" value={draft} onChange={(e) => setDraft(e.target.value)} />
          )}
          {edit.value_type === 'list' && <Reason>Pisahkan dengan koma.</Reason>}
          <Reason>Perubahan berlaku pada evaluasi berikutnya. Setiap perubahan tercatat di audit.</Reason>
        </Modal>
      )}
    </div>
  )
}

function Chain() {
  const [companies, setCompanies] = useState<Company[]>([])
  const [companyId, setCompanyId] = useState('')
  const [steps, setSteps] = useState<ChainStep[]>([])
  const [version, setVersion] = useState(0)
  const [roles, setRoles] = useState<Role[]>([])
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)
  const [note, setNote] = useState<string | null>(null)

  useEffect(() => {
    (async () => {
      const [c, r] = await Promise.all([
        api<{ data: Company[] }>('/companies'),
        api<{ data: Role[] }>('/roles'),
      ])
      setCompanies(c.data ?? []); setRoles(r.data ?? [])
      if (c.data?.length) setCompanyId(c.data[0].company_id)
      setLoading(false)
    })()
  }, [])

  useEffect(() => {
    if (!companyId) return
    api<{ data: ChainStep[]; version_no: number }>(`/approvals/chain?company_id=${companyId}`)
      .then((r) => { setSteps(r.data ?? []); setVersion(r.version_no) })
  }, [companyId])

  async function save() {
    setErr(null); setNote(null)
    try {
      const res = await api<{ version_no: number; note: string }>('/approvals/chain', {
        method: 'PUT',
        body: {
          company_id: companyId,
          steps: steps.map((s) => ({ step_name: s.step_name, satisfaction: s.satisfaction, role_ids: s.role_ids })),
        },
      })
      setVersion(res.version_no); setNote(res.note)
    } catch (e) { setErr(e instanceof ApiFailure ? e.body.message : 'Tidak dapat menyimpan') }
  }

  if (loading) return <Loading />

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-2 items-end">
        <div>
          <label className="label" htmlFor="c-company">Merek</label>
          <select id="c-company" className="input w-56" value={companyId}
                  onChange={(e) => setCompanyId(e.target.value)}>
            {companies.map((c) => <option key={c.company_id} value={c.company_id}>{c.company_name}</option>)}
          </select>
        </div>
        <div className="text-sm text-muted pb-2">Versi aktif: <b className="tnum">{version}</b></div>
        <button className="btn btn-primary ml-auto" onClick={save}>Simpan sebagai versi baru</button>
      </div>

      {err && <ErrorBox message={err} onDismiss={() => setErr(null)} />}
      {note && (
        <div className="border-l-[3px] border-info bg-[#e6f0f3] p-3 text-sm">
          <p className="font-semibold">Versi {version} disimpan</p>
          <p className="mt-0.5">{note}</p>
        </div>
      )}

      <div className="card text-sm">
        <p className="font-semibold">Menyimpan membuat versi baru, tidak mengubah yang lama.</p>
        <p className="text-muted mt-0.5">
          Rencana yang sedang berjalan tetap terikat pada versi tempat mereka
          mulai. Seorang administrator tidak dapat menghapus penyetuju yang tidak
          nyaman dari rencana yang sudah dalam proses.
        </p>
      </div>

      <div className="flex flex-col gap-2">
        {steps.map((s, i) => (
          <div key={i} className="card-paper flex flex-wrap items-center gap-3">
            <span className="font-extrabold text-lg tnum w-6">{i + 1}</span>
            <input className="input w-52" value={s.step_name}
                   aria-label={`Nama langkah ${i + 1}`}
                   onChange={(e) => setSteps(steps.map((x, j) => j === i ? { ...x, step_name: e.target.value } : x))} />
            <select className="input w-40" value={s.satisfaction}
                    aria-label={`Aturan langkah ${i + 1}`}
                    onChange={(e) => setSteps(steps.map((x, j) => j === i ? { ...x, satisfaction: e.target.value } : x))}>
              <option value="ANY_OF">Salah satu peran</option>
              <option value="ALL_OF">Semua peran</option>
            </select>
            <select className="input w-52" value={s.role_ids[0] ?? ''}
                    aria-label={`Peran langkah ${i + 1}`}
                    onChange={(e) => setSteps(steps.map((x, j) => j === i ? { ...x, role_ids: [e.target.value] } : x))}>
              {roles.map((r) => <option key={r.role_id} value={r.role_id}>{r.label_id}</option>)}
            </select>
            <button className="btn btn-ghost text-xs ml-auto"
                    onClick={() => setSteps(steps.filter((_, j) => j !== i))}>
              Hapus langkah
            </button>
          </div>
        ))}
        <button className="btn btn-secondary self-start"
                onClick={() => setSteps([...steps, {
                  step_no: steps.length + 1, step_name: '', satisfaction: 'ANY_OF',
                  role_ids: [roles[0]?.role_id ?? ''], role_labels: [],
                }])}>
          Tambah langkah
        </button>
      </div>
    </div>
  )
}
