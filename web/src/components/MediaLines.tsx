import { rupiah, rp } from '../lib/format'

export interface MediaLine { media_name: string; price_idr: number }

/**
 * The marketing-media lines on a promotion: what is being bought, what each
 * costs, and what they come to.
 *
 * The total is computed from the rows on every render and is never held in
 * state. A total in state is a number that can disagree with the rows it
 * claims to summarise — the same reason the server computes it on read rather
 * than storing it.
 */
export function MediaLines({
  value, onChange, disabled,
}: {
  value: MediaLine[]
  onChange: (lines: MediaLine[]) => void
  disabled?: boolean
}) {
  const total = value.reduce((sum, m) => sum + (m.price_idr || 0), 0)

  const set = (i: number, patch: Partial<MediaLine>) =>
    onChange(value.map((m, j) => (j === i ? { ...m, ...patch } : m)))

  const add = () => onChange([...value, { media_name: '', price_idr: 0 }])
  const remove = (i: number) => onChange(value.filter((_, j) => j !== i))

  return (
    <div>
      <div className="flex items-baseline justify-between">
        <span className="label mb-0">Media pemasaran</span>
        <span className="text-xs text-muted">Minimal satu</span>
      </div>

      {value.length === 0 ? (
        // Said at the point of failure rather than only after a rejected
        // submit: the requirement is visible while the form is still being
        // filled in, which is when it can still be met cheaply.
        <p className="text-sm py-2 text-warn">
          Minimal satu media pemasaran diperlukan sebelum rencana dapat diajukan.
          Draf tetap dapat disimpan tanpa media.
        </p>
      ) : (
        <div className="flex flex-col gap-2 mt-1">
          {value.map((m, i) => (
            <div key={i} className="flex flex-wrap items-start gap-2">
              <div className="flex-1 min-w-[180px]">
                <label className="sr-only" htmlFor={`media-name-${i}`}>
                  Nama media baris {i + 1}
                </label>
                <input
                  id={`media-name-${i}`}
                  className="input"
                  disabled={disabled}
                  placeholder="Contoh: Billboard Sudirman"
                  value={m.media_name}
                  onChange={(e) => set(i, { media_name: e.target.value })}
                />
              </div>
              <div className="w-44">
                <label className="sr-only" htmlFor={`media-price-${i}`}>
                  Harga baris {i + 1}
                </label>
                <input
                  id={`media-price-${i}`}
                  className="input tnum text-right"
                  inputMode="numeric"
                  disabled={disabled}
                  placeholder="0"
                  // Digits only, so a pasted "45.000.000" becomes 45000000
                  // rather than 45 — the same rule the target fields use.
                  value={m.price_idr ? rupiah(m.price_idr) : ''}
                  onChange={(e) =>
                    set(i, { price_idr: Number(e.target.value.replace(/\D/g, '') || 0) })
                  }
                />
              </div>
              <button
                type="button"
                className="btn btn-ghost text-xs h-9"
                disabled={disabled}
                aria-label={`Hapus baris ${i + 1}${m.media_name ? ': ' + m.media_name : ''}`}
                onClick={() => remove(i)}
              >
                Hapus
              </button>
            </div>
          ))}
        </div>
      )}

      <div className="flex flex-wrap items-center justify-between gap-2 mt-2 pt-2 border-t border-hairline">
        <button type="button" className="btn btn-secondary" disabled={disabled} onClick={add}>
          Tambah media
        </button>
        <div className="text-right">
          <div className="kicker">Total biaya media</div>
          <div className="font-extrabold text-lg tnum">{rp(total)}</div>
          <div className="text-xs text-muted">
            {value.length} baris · dijumlahkan otomatis
          </div>
        </div>
      </div>
    </div>
  )
}

/** Read-only view, for the plan detail screen. */
export function MediaSummary({ lines, total }: { lines: MediaLine[]; total: number }) {
  if (!lines.length) {
    return <p className="text-sm text-muted">Tidak ada media pemasaran yang dicatat.</p>
  }
  return (
    <table className="table">
      <thead>
        <tr><th>Media</th><th className="text-right">Harga</th></tr>
      </thead>
      <tbody>
        {lines.map((m, i) => (
          <tr key={i}>
            <td>{m.media_name}</td>
            <td className="text-right tnum">{rp(m.price_idr)}</td>
          </tr>
        ))}
      </tbody>
      <tfoot>
        <tr>
          <th className="text-left">Total</th>
          <th className="text-right tnum text-[15px] normal-case tracking-normal">{rp(total)}</th>
        </tr>
      </tfoot>
    </table>
  )
}
