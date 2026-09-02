import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, ApiFailure, type Plan } from '../lib/api'
import { useAuth } from '../lib/auth'
import { formatDate, formatDateTime, rp, count, orderModeLabel } from '../lib/format'
import { StatusPill, Loading, ErrorBox, Modal, BrandTag, Reason } from '../components/ui'

const ACTION_LABEL: Record<string, string> = {
  APPROVE: 'Disetujui', REJECT: 'Ditolak', FORCE_RELEASE: 'Dirilis paksa',
  AUTO_CANCEL: 'Dibatalkan otomatis', REVIVE: 'Dihidupkan kembali', REOPEN: 'Dibuka kembali',
}

export default function PlanDetail() {
  const { id } = useParams()
  const { user, can } = useAuth()
  const [plan, setPlan] = useState<Plan | null>(null)
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<{ message: string; fields?: Record<string, string> } | null>(null)
  const [dialog, setDialog] = useState<null | 'reject' | 'force' | 'revive'>(null)
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try { setPlan(await api<Plan>(`/promotions/${id}`)) }
    catch (e) { setErr({ message: e instanceof ApiFailure ? e.body.message : 'Tidak dapat memuat' }) }
    finally { setLoading(false) }
  }, [id])

  useEffect(() => { load() }, [load])

  async function act(path: string, body?: unknown) {
    setBusy(true); setErr(null)
    try {
      await api(`/promotions/${id}/${path}`, { method: 'POST', body })
      setDialog(null); setReason('')
      await load()
    } catch (e) {
      if (e instanceof ApiFailure) setErr({ message: e.body.message, fields: e.body.fields })
      else setErr({ message: 'Tindakan gagal' })
    } finally { setBusy(false) }
  }

  if (loading) return <Loading />
  if (!plan) return <ErrorBox message={err?.message ?? 'Rencana tidak ditemukan'} />

  const v = plan.version
  const canDecide = plan.status === 'PENDING'
  const isMine = user?.full_name === plan.created_by_name

  return (
    <div className="flex flex-col gap-4 max-w-5xl">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="kicker">
            {plan.plan_code} · {plan.company_name} · versi {v.version_no}
          </div>
          <h3>{v.promo_name}</h3>
          <div className="flex items-center gap-2 mt-1">
            <StatusPill status={plan.status} forceReleased={plan.force_released} />
            <BrandTag code={plan.company_code} name={plan.company_name} />
          </div>
        </div>
        <div className="flex gap-2">
          <Link to="/promo" className="btn btn-secondary">Kembali</Link>
          {(plan.status === 'DRAFT' || plan.status === 'REJECTED') && can('promo.create') && (
            <Link to={`/promo/${plan.plan_id}/ubah`} className="btn btn-secondary">Ubah</Link>
          )}
          {(plan.status === 'RELEASED' || plan.status === 'CANCELLED') && can('promo.create') && (
            <Link to={`/promo/${plan.plan_id}/ubah`} className="btn btn-secondary"
                  title="Rencana terkunci: perubahan membuat versi baru yang masuk rantai dari langkah 1">
              Buat versi baru
            </Link>
          )}
        </div>
      </div>

      {err && <ErrorBox message={err.message} fields={err.fields} onDismiss={() => setErr(null)} />}

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Fact label="Periode" value={`${formatDate(v.start_date)} – ${formatDate(v.end_date)}`} />
        <Fact label="Kelompok toko" value={plan.site_group_name} />
        <Fact label="Target penjualan" value={rp(v.target_sales_idr)} numeric />
        <Fact label="Target struk" value={count(v.target_receipt_count)} numeric />
        <Fact label="Mode pesanan" value={orderModeLabel(v.order_mode)} />
        <Fact label="Dibuat oleh" value={plan.created_by_name} />
        {v.lead_time_overridden && (
          <Fact label="Masa tenggang" value="Dikesampingkan oleh superadmin" />
        )}
      </div>

      <div className="card-paper">
        <div className="kicker mb-1">Aturan promo</div>
        <p className="whitespace-pre-wrap text-sm">{v.promo_rule}</p>
      </div>

      {plan.approval && (
        <div className="card-paper">
          <div className="kicker mb-2">Rantai persetujuan</div>
          <ol className="flex flex-col gap-0">
            {plan.approval.steps.map((s) => {
              const events = plan.approval!.events.filter((e) => e.step_no === s.step_no)
              const done = events.some((e) => e.action === 'APPROVE')
              const current = plan.approval!.current_step_no === s.step_no && plan.status === 'PENDING'
              return (
                <li key={s.step_no} className="flex gap-3 py-2 border-b border-hairline last:border-0">
                  <div className="w-6 shrink-0 text-center">
                    <span aria-hidden="true" className={done ? 'text-success' : current ? 'text-warn' : 'text-muted'}>
                      {done ? '✓' : current ? '◷' : '○'}
                    </span>
                  </div>
                  <div className="flex-1">
                    <div className="font-semibold text-sm">
                      {s.step_no}. {s.step_name}
                      {current && <span className="ml-2 pill bg-[#fff2ef] text-warn">
                        <span aria-hidden="true">◷</span> Menunggu di sini
                      </span>}
                    </div>
                    {events.map((e, i) => (
                      <div key={i} className="text-xs text-muted mt-0.5">
                        {ACTION_LABEL[e.action] ?? e.action} oleh {e.actor_name} · {formatDateTime(e.occurred_at)}
                        {e.reason && <div className="mt-0.5 text-ink">Alasan: {e.reason}</div>}
                      </div>
                    ))}
                    {!done && !current && (
                      <div className="text-xs text-muted mt-0.5">Belum dibuka</div>
                    )}
                  </div>
                </li>
              )
            })}
          </ol>
        </div>
      )}

      {canDecide && (
        <div className="card">
          <div className="kicker mb-2">Keputusan Anda</div>
          {isMine ? (
            <>
              <button className="btn btn-primary" disabled>Setujui</button>
              <Reason>
                Anda pembuat rencana ini. Pembuat tidak boleh menyetujui rencananya
                sendiri, di langkah mana pun.
              </Reason>
            </>
          ) : (
            <div className="flex flex-wrap gap-2">
              <button className="btn btn-primary" disabled={busy} onClick={() => act('approve')}>
                Setujui langkah {plan.current_step_no}
              </button>
              <button className="btn btn-secondary" disabled={busy} onClick={() => setDialog('reject')}>
                Tolak
              </button>
              {can('force_release') && (
                <button className="btn btn-danger" disabled={busy} onClick={() => setDialog('force')}
                        title="Melewati seluruh rantai. Wajib beralasan dan tercatat di audit.">
                  Rilis paksa
                </button>
              )}
            </div>
          )}
          <Reason>
            Keputusan bersifat final dan tercatat permanen. Koreksi dilakukan
            dengan versi baru, bukan dengan mengubah keputusan.
          </Reason>
        </div>
      )}

      {plan.status === 'CANCELLED' && can('force_release') && (
        <div className="card">
          <div className="kicker mb-2">Pemulihan</div>
          <button className="btn btn-secondary" onClick={() => setDialog('revive')}>
            Hidupkan kembali
          </button>
          <Reason>
            Rencana kembali ke langkah tempat ia menunggu. Pembatalan tetap
            tersimpan di riwayat — pemulihan adalah peristiwa kedua, bukan penghapusan.
          </Reason>
        </div>
      )}

      {plan.versions && plan.versions.length > 1 && (
        <div className="card-paper">
          <div className="kicker mb-2">Riwayat versi</div>
          <ul className="text-sm flex flex-col gap-1">
            {plan.versions.map((ver) => (
              <li key={ver.version_id} className="flex flex-wrap gap-3">
                <span className="font-semibold">v{ver.version_no}</span>
                <span>{ver.promo_name}</span>
                <span className="text-muted">{formatDate(ver.start_date)} – {formatDate(ver.end_date)}</span>
                <span className="tnum text-muted">{rp(ver.target_sales_idr)}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {dialog && (
        <Modal
          title={dialog === 'reject' ? 'Tolak rencana'
            : dialog === 'force' ? 'Rilis paksa' : 'Hidupkan kembali'}
          onClose={() => { setDialog(null); setReason('') }}
          footer={
            <>
              <button className="btn btn-secondary" onClick={() => { setDialog(null); setReason('') }}>
                Batal
              </button>
              <button
                className={dialog === 'force' ? 'btn btn-danger' : 'btn btn-primary'}
                disabled={busy || reason.trim().length === 0}
                onClick={() => act(dialog === 'reject' ? 'reject' : dialog === 'force' ? 'force-release' : 'revive',
                                   { reason })}
              >
                {busy ? 'Menyimpan…' : 'Konfirmasi'}
              </button>
            </>
          }
        >
          <label className="label" htmlFor="reason">Alasan (wajib)</label>
          <textarea id="reason" className="input min-h-[90px]" value={reason} required
                    onChange={(e) => setReason(e.target.value)}
                    placeholder="Jelaskan alasannya. Teks ini tersimpan permanen di jejak audit." />
          {reason.trim().length === 0 && (
            <Reason>Tombol konfirmasi aktif setelah alasan diisi.</Reason>
          )}
        </Modal>
      )}
    </div>
  )
}

function Fact({ label, value, numeric }: { label: string; value: string; numeric?: boolean }) {
  return (
    <div className="card-paper">
      <div className="kicker">{label}</div>
      <div className={`font-semibold mt-0.5 ${numeric ? 'tnum' : ''}`}>{value}</div>
    </div>
  )
}
