import { useEffect, useState } from 'react'
import { api, ApiFailure, type Company } from '../lib/api'
import { SearchBox, TableWrap, Loading, Empty, ErrorBox, Modal, Reason } from '../components/ui'

interface UserRow {
  user_id: string; email: string; full_name: string
  is_active: boolean; totp_ready: boolean
  roles: string[]; companies: string[]
}
interface Role { role_id: string; role_code: string; label_id: string }

export default function Users() {
  const [rows, setRows] = useState<UserRow[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [companies, setCompanies] = useState<Company[]>([])
  const [q, setQ] = useState('')
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<{ message: string; fields?: Record<string, string> } | null>(null)
  const [form, setForm] = useState<null | {
    email: string; full_name: string; password: string
    role_id: string; company_ids: string[]
  }>(null)

  async function load() {
    setLoading(true)
    try {
      const [u, r, c] = await Promise.all([
        api<{ data: UserRow[] }>(`/users${q ? '?q=' + encodeURIComponent(q) : ''}`),
        api<{ data: Role[] }>('/roles'),
        api<{ data: Company[] }>('/companies'),
      ])
      setRows(u.data ?? []); setRoles(r.data ?? []); setCompanies(c.data ?? [])
    } finally { setLoading(false) }
  }
  useEffect(() => { load() /* eslint-disable-next-line */ }, [q])

  async function save() {
    if (!form) return
    setErr(null)
    try {
      await api('/users', {
        method: 'POST',
        body: {
          email: form.email, full_name: form.full_name, password: form.password,
          grants: form.company_ids.map((cid) => ({ role_id: form.role_id, company_id: cid })),
        },
      })
      setForm(null); await load()
    } catch (e) {
      if (e instanceof ApiFailure) setErr({ message: e.body.message, fields: e.body.fields })
      else setErr({ message: 'Tidak dapat membuat pengguna' })
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="kicker">Administrasi</div>
          <h3>Pengguna</h3>
        </div>
        <div className="flex flex-wrap gap-2 items-end">
          <div className="w-full sm:w-72">
            <SearchBox value={q} onChange={setQ} placeholder="Cari nama atau surel…" />
          </div>
          <button className="btn btn-primary"
                  onClick={() => setForm({ email: '', full_name: '', password: '', role_id: roles[0]?.role_id ?? '', company_ids: [] })}>
            Tambah pengguna
          </button>
        </div>
      </div>

      {err && <ErrorBox message={err.message} fields={err.fields} onDismiss={() => setErr(null)} />}

      <div className="card text-sm">
        <p className="font-semibold">Akun dibuat oleh administrator.</p>
        <p className="text-muted mt-0.5">
          Tidak ada pendaftaran mandiri. Setiap pengguna diberi <b>satu peran di
          satu atau lebih perusahaan, secara eksplisit</b> — tidak ada nilai
          "semua perusahaan". Menambah merek keempat kelak tidak memberi akses
          kepada siapa pun sampai seseorang memutuskannya.
        </p>
      </div>

      {loading ? <Loading /> : rows.length === 0 ? (
        <Empty title="Tidak ada pengguna yang cocok" />
      ) : (
        <TableWrap>
          <table className="table">
            <thead>
              <tr><th>Nama</th><th>Surel</th><th>Peran</th><th>Perusahaan</th><th>TOTP</th><th>Aktif</th></tr>
            </thead>
            <tbody>
              {rows.map((u) => (
                <tr key={u.user_id}>
                  <td className="font-semibold">{u.full_name}</td>
                  <td className="text-xs">{u.email}</td>
                  <td className="text-xs">{(u.roles ?? []).join(', ')}</td>
                  <td className="text-xs">{(u.companies ?? []).join(', ')}</td>
                  <td>
                    {u.totp_ready ? (
                      <span className="pill bg-[#e8f2ec] text-success"><span aria-hidden="true">✓</span> Terdaftar</span>
                    ) : (
                      <span className="pill bg-[#fff2ef] text-warn" title="Pengguna hanya dapat mencapai alur pendaftaran TOTP">
                        <span aria-hidden="true">◷</span> Belum
                      </span>
                    )}
                  </td>
                  <td>{u.is_active ? 'Ya' : 'Tidak'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableWrap>
      )}

      {form && (
        <Modal title="Tambah pengguna" onClose={() => setForm(null)}
               footer={<>
                 <button className="btn btn-secondary" onClick={() => setForm(null)}>Batal</button>
                 <button className="btn btn-primary" disabled={form.company_ids.length === 0} onClick={save}>
                   Simpan
                 </button>
               </>}>
          <div className="flex flex-col gap-3">
            <div>
              <label className="label" htmlFor="u-name">Nama lengkap</label>
              <input id="u-name" className="input" value={form.full_name}
                     onChange={(e) => setForm({ ...form, full_name: e.target.value })} />
            </div>
            <div>
              <label className="label" htmlFor="u-email">Surel</label>
              <input id="u-email" className="input" type="email" value={form.email}
                     onChange={(e) => setForm({ ...form, email: e.target.value })} />
            </div>
            <div>
              <label className="label" htmlFor="u-pass">Kata sandi awal</label>
              <input id="u-pass" className="input" type="password" value={form.password}
                     onChange={(e) => setForm({ ...form, password: e.target.value })} />
              <Reason>
                Minimal 12 karakter. Panjang yang menentukan, bukan campuran
                simbol. TOTP didaftarkan pengguna sendiri saat login pertama.
              </Reason>
            </div>
            <div>
              <label className="label" htmlFor="u-role">Peran</label>
              <select id="u-role" className="input" value={form.role_id}
                      onChange={(e) => setForm({ ...form, role_id: e.target.value })}>
                {roles.map((r) => <option key={r.role_id} value={r.role_id}>{r.label_id}</option>)}
              </select>
            </div>
            <fieldset>
              <legend className="label">Perusahaan (pilih satu atau lebih)</legend>
              <div className="flex flex-col gap-1 border border-divider p-2">
                {companies.map((c) => (
                  <label key={c.company_id} className="inline-flex items-center gap-2 text-sm">
                    <input type="checkbox" checked={form.company_ids.includes(c.company_id)}
                           onChange={(e) => setForm({
                             ...form,
                             company_ids: e.target.checked
                               ? [...form.company_ids, c.company_id]
                               : form.company_ids.filter((x) => x !== c.company_id),
                           })} />
                    {c.company_name}
                  </label>
                ))}
              </div>
              {form.company_ids.length === 0 && (
                <Reason>Wajib minimal satu. Tombol simpan aktif setelah dipilih.</Reason>
              )}
            </fieldset>
          </div>
        </Modal>
      )}
    </div>
  )
}
