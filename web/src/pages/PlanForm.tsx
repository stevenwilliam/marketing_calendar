import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, ApiFailure, type Company, type Plan, type SiteGroup, type Overlap } from '../lib/api'
import { formatDate, rupiah } from '../lib/format'
import { ErrorBox, Loading, Modal, Reason } from '../components/ui'
import { RichText } from '../components/RichText'
import { DateRangePicker } from '../components/DateRangePicker'
import { MediaLines, type MediaLine } from '../components/MediaLines'

export default function PlanForm() {
  const { id } = useParams()
  const nav = useNavigate()
  const editing = Boolean(id)

  const [companies, setCompanies] = useState<Company[]>([])
  const [groups, setGroups] = useState<SiteGroup[]>([])
  const [existing, setExisting] = useState<Plan | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<{ message: string; fields?: Record<string, string> } | null>(null)
  const [overlaps, setOverlaps] = useState<Overlap[] | null>(null)

  const [companyId, setCompanyId] = useState('')
  const [groupId, setGroupId] = useState('')
  const [name, setName] = useState('')
  // Deliberately empty for a new plan. A pre-filled range would be a date the
  // user did not choose, and one that could be INSIDE the lead time the moment
  // somebody raises `promo.lead_time_working_days`. The picker disables the
  // impossible dates, so letting it be the only way in makes a wrong range
  // unreachable rather than merely refused later.
  const [start, setStart] = useState('')
  const [end, setEnd] = useState('')
  const [sales, setSales] = useState('')
  const [receipts, setReceipts] = useState('')
  const [mode, setMode] = useState<'dine_in' | 'take_away'>('dine_in')
  const [rule, setRule] = useState('')
  const [media, setMedia] = useState<MediaLine[]>([])

  useEffect(() => {
    (async () => {
      const [c, g] = await Promise.all([
        api<{ data: Company[] }>('/companies'),
        api<{ data: SiteGroup[] }>('/site-groups'),
      ])
      setCompanies(c.data ?? [])
      setGroups(g.data ?? [])
      if (id) {
        const p = await api<Plan>(`/promotions/${id}`)
        setExisting(p)
        setCompanyId(p.company_id); setGroupId(p.site_group_id)
        setName(p.version.promo_name)
        setStart(p.version.start_date); setEnd(p.version.end_date)
        setSales(String(p.version.target_sales_idr))
        setReceipts(String(p.version.target_receipt_count))
        setMode(p.version.order_mode); setRule(p.version.promo_rule)
        setMedia((p.version.media ?? []).map((m) => ({
          media_name: m.media_name, price_idr: m.price_idr,
        })))
      } else if ((c.data ?? []).length === 1) {
        setCompanyId(c.data[0].company_id)
      }
      setLoading(false)
    })()
  }, [id])

  // A group belongs to exactly one brand (D31), so the picker only ever
  // offers groups from the chosen company. The database refuses the rest.
  const visibleGroups = groups.filter((g) => !companyId || g.company_id === companyId)

  function body() {
    return {
      company_id: companyId, site_group_id: groupId, promo_name: name,
      start_date: start, end_date: end,
      // Whole rupiah as an integer. The field strips anything that is not a
      // digit, so a pasted "185.000" cannot become 185.
      target_sales_idr: Number(sales.replace(/\D/g, '') || 0),
      target_receipt_count: Number(receipts.replace(/\D/g, '') || 0),
      order_mode: mode, promo_rule: rule,
      media: media.map((m) => ({ media_name: m.media_name, price_idr: m.price_idr })),
    }
  }

  async function save(thenSubmit: boolean) {
    setBusy(true); setErr(null); setOverlaps(null)
    try {
      let planId = id
      if (editing) {
        await api(`/promotions/${id}`, { method: 'PUT', body: body() })
      } else {
        const res = await api<{ plan_id: string }>('/promotions', { method: 'POST', body: body() })
        planId = res.plan_id
      }
      if (!thenSubmit) { nav(`/promo/${planId}`); return }
      await api(`/promotions/${planId}/submit`, { method: 'POST', body: {} })
      nav(`/promo/${planId}`)
    } catch (e) {
      if (e instanceof ApiFailure) {
        if (e.body.code === 'OVERLAP_UNACKNOWLEDGED' && e.body.overlaps) {
          setOverlaps(e.body.overlaps)
        } else {
          setErr({ message: e.body.message, fields: e.body.fields })
        }
      } else setErr({ message: 'Tidak dapat menyimpan' })
    } finally { setBusy(false) }
  }

  async function submitAcknowledged() {
    setBusy(true)
    try {
      await api(`/promotions/${id ?? existing?.plan_id}/submit`,
                { method: 'POST', body: { acknowledge_overlap: true } })
      nav(`/promo/${id ?? existing?.plan_id}`)
    } catch (e) {
      if (e instanceof ApiFailure) setErr({ message: e.body.message, fields: e.body.fields })
    } finally { setBusy(false); setOverlaps(null) }
  }

  if (loading) return <Loading />

  const locked = existing && existing.status !== 'DRAFT' && existing.status !== 'REJECTED'

  return (
    <div className="flex flex-col gap-4 max-w-3xl">
      <div>
        <div className="kicker">
          {editing ? `${existing?.plan_code} · versi ${existing?.version.version_no}` : 'Rencana promo · draf baru'}
        </div>
        <h3>{editing ? 'Ubah rencana' : 'Buat rencana promo'}</h3>
      </div>

      {locked && (
        <div className="border-l-[3px] border-info bg-[#e6f0f3] p-3 text-sm">
          <p className="font-semibold">Rencana ini terkunci</p>
          <p className="mt-0.5">
            Rencana yang sudah masuk atau menyelesaikan rantai persetujuan tidak
            dapat diubah. Menyimpan akan membuat <b>versi baru</b> yang masuk
            rantai dari langkah 1; versi sebelumnya tetap tersimpan dan terbaca.
          </p>
        </div>
      )}

      {err && <ErrorBox message={err.message} fields={err.fields} onDismiss={() => setErr(null)} />}

      <form className="flex flex-col gap-3" onSubmit={(e) => { e.preventDefault(); save(true) }}>
        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <label className="label" htmlFor="company">Merek</label>
            <select id="company" className="input" required value={companyId}
                    disabled={editing}
                    onChange={(e) => { setCompanyId(e.target.value); setGroupId('') }}>
              <option value="">Pilih merek…</option>
              {companies.map((c) => (
                <option key={c.company_id} value={c.company_id}>{c.company_name}</option>
              ))}
            </select>
            {editing && <Reason>Merek tidak dapat dipindah setelah rencana dibuat.</Reason>}
          </div>
          <div>
            <label className="label" htmlFor="group">Kelompok toko</label>
            <select id="group" className="input" required value={groupId}
                    disabled={!companyId}
                    onChange={(e) => setGroupId(e.target.value)}>
              <option value="">Pilih kelompok…</option>
              {visibleGroups.map((g) => (
                <option key={g.site_group_id} value={g.site_group_id}>
                  {g.site_group_name} ({g.member_count} toko)
                </option>
              ))}
            </select>
            {!companyId && <Reason>Pilih merek dahulu — kelompok toko selalu milik satu merek.</Reason>}
          </div>
        </div>

        <div>
          <label className="label" htmlFor="name">Nama promo</label>
          <input id="name" className="input" required maxLength={200}
                 value={name} onChange={(e) => setName(e.target.value)} />
        </div>

        <div>
          <span className="label">Periode promo</span>
          <DateRangePicker
            start={start}
            end={end}
            onChange={(s, e) => { setStart(s); setEnd(e) }}
          />
          <Reason>
            Masa tenggang dicek saat <b>pengajuan</b>, bukan saat menyimpan draf —
            draf yang ditinggal semalam tidak diam-diam menjadi tidak sah.
            Periode boleh melewati batas bulan dan tahun.
          </Reason>
        </div>

        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <label className="label" htmlFor="sales">Target penjualan (Rp)</label>
            <input id="sales" className="input tnum text-right" inputMode="numeric" required
                   value={sales ? rupiah(Number(sales.replace(/\D/g, ''))) : ''}
                   onChange={(e) => setSales(e.target.value.replace(/\D/g, ''))} />
            <Reason>Rupiah penuh, tanpa sen.</Reason>
          </div>
          <div>
            <label className="label" htmlFor="receipts">Target jumlah struk</label>
            <input id="receipts" className="input tnum text-right" inputMode="numeric" required
                   value={receipts ? rupiah(Number(receipts.replace(/\D/g, ''))) : ''}
                   onChange={(e) => setReceipts(e.target.value.replace(/\D/g, ''))} />
          </div>
        </div>

        <fieldset>
          <legend className="label">Mode pesanan</legend>
          <div className="inline-flex border border-divider">
            {(['dine_in', 'take_away'] as const).map((m, i) => (
              <label key={m}
                     className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-sm cursor-pointer ${i > 0 ? 'border-l border-divider' : ''}`}
                     style={mode === m ? { background: '#ae1800', color: '#f3f2f2' } : undefined}>
                <input type="radio" name="mode" className="sr-only" checked={mode === m}
                       onChange={() => setMode(m)} />
                {m === 'dine_in' ? 'Makan di tempat' : 'Bawa pulang'}
              </label>
            ))}
          </div>
          <Reason>Satu promo berlaku untuk satu mode. Untuk keduanya, buat dua rencana.</Reason>
        </fieldset>

        <div>
          <MediaLines value={media} onChange={setMedia} />
        </div>

        <div>
          <label className="label" htmlFor="rule">Aturan promo</label>
          <RichText
            id="rule"
            value={rule}
            onChange={setRule}
            maxTextLength={5000}
            placeholder="Contoh: Diskon 20% untuk gelas kedua, setiap hari 15.00–18.00."
          />
        </div>

        <div className="flex flex-wrap gap-2 pt-1">
          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? 'Menyimpan…' : 'Simpan dan ajukan'}
          </button>
          <button type="button" className="btn btn-secondary" disabled={busy}
                  onClick={() => save(false)}>
            Simpan draf
          </button>
          <button type="button" className="btn btn-ghost" onClick={() => nav(-1)}>Batal</button>
        </div>
      </form>

      {overlaps && (
        <Modal
          title="Ada promo yang tumpang tindih"
          onClose={() => setOverlaps(null)}
          footer={
            <>
              <button className="btn btn-secondary" onClick={() => setOverlaps(null)}>Periksa lagi</button>
              <button className="btn btn-primary" disabled={busy} onClick={submitAcknowledged}>
                Saya paham, tetap ajukan
              </button>
            </>
          }
        >
          <p>
            Promo berikut berjalan pada <b>toko yang sama</b> di rentang tanggal yang
            beririsan. Promo bertumpuk kadang memang disengaja, jadi ini bukan
            penolakan — tetapi harus diakui secara sadar.
          </p>
          <ul className="mt-2 flex flex-col gap-1.5">
            {overlaps.map((o) => (
              <li key={o.plan_id} className="card-paper text-sm">
                <div className="font-semibold">{o.promo_name}</div>
                <div className="text-muted text-xs">
                  {o.plan_code} · {formatDate(o.start_date)} – {formatDate(o.end_date)} ·
                  {' '}{o.shared_site_count} toko beririsan
                </div>
              </li>
            ))}
          </ul>
        </Modal>
      )}
    </div>
  )
}
