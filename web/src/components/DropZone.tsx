import { useCallback, useRef, useState, type DragEvent } from 'react'

/**
 * A drag-and-drop file zone that is also a real button and a real file input.
 *
 * Drag-and-drop alone is not accessible: it cannot be reached by keyboard and
 * it does not exist on a phone. The visible control here is a <button> that
 * opens the file picker; the drop target is an enhancement layered on top of
 * it, not the only way in.
 *
 * dragCounter, not a boolean: dragenter and dragleave both fire when the
 * pointer crosses a CHILD element, so a naive boolean flickers off the moment
 * the cursor passes over the label inside the zone.
 */
export function DropZone({
  onFiles, accept = '.csv', disabled, hint, multiple = true,
}: {
  onFiles: (files: File[]) => void
  accept?: string
  disabled?: boolean
  hint?: string
  multiple?: boolean
}) {
  const [over, setOver] = useState(false)
  const [rejected, setRejected] = useState<string[]>([])
  const dragCounter = useRef(0)
  const input = useRef<HTMLInputElement>(null)

  const take = useCallback((list: FileList | null) => {
    setRejected([])
    if (!list) return
    const all = Array.from(list)
    // Filter by EXTENSION, not by the browser's reported MIME type: a CSV
    // arrives as text/csv, application/vnd.ms-excel, text/plain or "" depending
    // on the machine, and rejecting on that would refuse valid files.
    const ok = all.filter((f) => f.name.toLowerCase().endsWith('.csv'))
    const bad = all.filter((f) => !f.name.toLowerCase().endsWith('.csv'))
    if (bad.length) setRejected(bad.map((f) => f.name))
    if (ok.length) onFiles(multiple ? ok : ok.slice(0, 1))
  }, [onFiles, multiple])

  const stop = (e: DragEvent) => { e.preventDefault(); e.stopPropagation() }

  return (
    <div>
      <div
        onDragEnter={(e) => { stop(e); if (!disabled) { dragCounter.current++; setOver(true) } }}
        onDragOver={stop}
        onDragLeave={(e) => {
          stop(e)
          dragCounter.current--
          if (dragCounter.current <= 0) { dragCounter.current = 0; setOver(false) }
        }}
        onDrop={(e) => {
          stop(e)
          dragCounter.current = 0
          setOver(false)
          if (!disabled) take(e.dataTransfer.files)
        }}
        className="border-2 border-dashed p-6 text-center transition-colors"
        style={{
          borderColor: over ? '#ec3013' : 'rgba(32,30,29,0.55)',
          background: over ? '#fff2ef' : '#ffffff',
          opacity: disabled ? 0.5 : 1,
        }}
      >
        <p className="font-semibold text-sm">
          {over ? 'Lepaskan berkas di sini' : 'Tarik berkas CSV ke sini'}
        </p>
        <p className="text-xs text-muted mt-1">atau</p>
        <button
          type="button"
          className="btn btn-secondary mt-2"
          disabled={disabled}
          onClick={() => input.current?.click()}
        >
          Pilih berkas
        </button>
        <input
          ref={input}
          type="file"
          accept={accept}
          multiple={multiple}
          className="sr-only"
          onChange={(e) => { take(e.target.files); e.target.value = '' }}
        />
        {hint && <p className="text-xs text-muted mt-2">{hint}</p>}
      </div>

      {rejected.length > 0 && (
        <p className="text-xs text-danger mt-1.5" role="alert">
          Bukan berkas CSV, dilewati: {rejected.join(', ')}
        </p>
      )}
    </div>
  )
}
