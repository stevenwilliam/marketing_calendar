import { useState } from 'react'
import { api, ApiFailure, type Principal } from '../lib/api'
import { useAuth } from '../lib/auth'
import { ErrorBox } from '../components/ui'

export default function Login() {
  const { signIn } = useAuth()
  const [step, setStep] = useState<'password' | 'totp'>('password')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [challenge, setChallenge] = useState('')
  const [code, setCode] = useState('')
  const [enrol, setEnrol] = useState<{ uri: string; secret: string } | null>(null)
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submitPassword(e: React.FormEvent) {
    e.preventDefault()
    setErr(null); setBusy(true)
    try {
      const res = await api<{
        challenge?: string; needs_enrolment?: boolean
        provisioning_uri?: string; secret?: string
        access_token?: string; user?: Principal
      }>('/auth/login', { method: 'POST', body: { email, password } })

      // With the second factor off the server returns a session here and
      // there is no step 2. Branching on access_token rather than on a
      // client-side flag means the SERVER decides how many steps there are —
      // the UI cannot get out of step with `auth.totp_required`.
      if (res.access_token && res.user) {
        signIn(res.user, res.access_token)
        return
      }

      setChallenge(res.challenge ?? '')
      if (res.needs_enrolment && res.secret) {
        setEnrol({ uri: res.provisioning_uri ?? '', secret: res.secret })
      }
      setStep('totp')
    } catch (e) {
      setErr(e instanceof ApiFailure ? e.body.message : 'Tidak dapat masuk')
    } finally { setBusy(false) }
  }

  async function submitTOTP(e: React.FormEvent) {
    e.preventDefault()
    setErr(null); setBusy(true)
    try {
      const res = await api<{ access_token: string; user: Principal }>(
        '/auth/totp', { method: 'POST', body: { challenge, code } })
      signIn(res.user, res.access_token)
    } catch (e) {
      setErr(e instanceof ApiFailure ? e.body.message : 'Kode tidak diterima')
      setCode('')
    } finally { setBusy(false) }
  }

  return (
    <div className="min-h-screen grid md:grid-cols-2">
      {/* The dark half is the identity: near-black ground, one accent rule. */}
      <div className="bg-ink text-bg p-8 md:p-14 flex flex-col justify-between min-h-[220px]">
        <div className="font-extrabold text-lg leading-none tracking-tight">MARKETING<br />CALENDAR</div>
        <div className="hidden md:block">
          <div className="h-0.5 bg-accent w-[72px] mb-6" />
          <div className="font-extrabold text-[40px] leading-[1.05] tracking-tight max-w-[12em]">
            Satu tempat untuk merencanakan, menyetujui dan mengukur promo.
          </div>
          <div className="flex gap-7 mt-8 text-xs" style={{ color: 'rgba(243,242,242,0.72)' }}>
            <span>Maxx Coffee</span><span>Ruuma</span><span>Sunshine</span>
          </div>
        </div>
        <div className="text-xs" style={{ color: 'rgba(243,242,242,0.62)' }}>
          Jaringan internal · TLS · daftar izin IP
        </div>
      </div>

      <div className="p-8 md:p-14 flex flex-col justify-center max-w-[520px] w-full">
        {step === 'password' ? (
          <form onSubmit={submitPassword} className="flex flex-col gap-3">
            <h3>Masuk</h3>
            <p className="text-sm text-muted -mt-1">
              Akun dibuat oleh administrator. Tidak ada pendaftaran mandiri.
            </p>
            {err && <ErrorBox message={err} onDismiss={() => setErr(null)} />}
            <div>
              <label className="label" htmlFor="email">Surel</label>
              <input id="email" className="input" type="email" autoComplete="username"
                     required value={email} onChange={(e) => setEmail(e.target.value)} />
            </div>
            <div>
              <label className="label" htmlFor="password">Kata sandi</label>
              <input id="password" className="input" type="password" autoComplete="current-password"
                     required value={password} onChange={(e) => setPassword(e.target.value)} />
            </div>
            <button type="submit" className="btn btn-primary mt-1 self-start px-5 py-2.5" disabled={busy}>
              {busy ? 'Memeriksa…' : 'Masuk'}
            </button>
          </form>
        ) : (
          <form onSubmit={submitTOTP} className="flex flex-col gap-3">
            <div className="kicker">Langkah kedua</div>
            <h3>Kode autentikasi</h3>
            <p className="text-sm text-muted -mt-1">
              Masukkan enam angka dari aplikasi autentikator Anda.
              TOTP wajib untuk semua akun staf.
            </p>

            {enrol && (
              <div className="card text-sm">
                <p className="font-semibold">Pendaftaran pertama</p>
                <p className="mt-1">
                  Pindai kode ini di aplikasi autentikator, lalu masukkan enam angkanya.
                </p>
                <p className="mt-2 break-all font-mono text-xs bg-paper p-2 border border-hairline">
                  {enrol.secret}
                </p>
                <p className="text-xs text-muted mt-1">
                  Simpan kunci ini di tempat aman. Kunci hanya ditampilkan sekali.
                </p>
              </div>
            )}

            {err && <ErrorBox message={err} onDismiss={() => setErr(null)} />}
            <div>
              <label className="label" htmlFor="code">Kode enam angka</label>
              <input id="code" className="input tnum text-center text-2xl font-extrabold tracking-[0.3em]"
                     inputMode="numeric" pattern="[0-9]*" maxLength={6} autoComplete="one-time-code"
                     required autoFocus value={code}
                     onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))} />
            </div>
            <div className="flex gap-2">
              <button type="submit" className="btn btn-primary px-5 py-2.5" disabled={busy || code.length !== 6}>
                {busy ? 'Memverifikasi…' : 'Verifikasi dan masuk'}
              </button>
              <button type="button" className="btn btn-secondary px-5 py-2.5"
                      onClick={() => { setStep('password'); setCode(''); setErr(null) }}>
                Kembali
              </button>
            </div>
            <hr className="border-0 h-0.5 bg-divider my-2" />
            <p className="text-xs text-muted">
              Kode berlaku 30 detik. Percobaan berulang mengunci akun sementara
              dan menulis baris audit.
            </p>
          </form>
        )}
      </div>
    </div>
  )
}
