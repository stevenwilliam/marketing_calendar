import { useEffect, useRef, useState } from 'react'

/**
 * A small rich-text editor for the promotion rule.
 *
 * Written rather than installed. The whole requirement is bold, italic,
 * underline, two list kinds and a sub-heading — a few hundred lines against a
 * dependency measured in hundreds of kilobytes, on a page that is already the
 * heaviest in the product.
 *
 * It is NOT a security boundary. The server sanitises what it stores against a
 * tiny allow-list, and it would do so even if this file allowed anything at
 * all: what a browser sends is whatever the sender chose to send. What this
 * does is stop the honest case producing markup the server would only throw
 * away — most usefully, pasting from Word.
 */

const TOOLBAR = [
  { cmd: 'bold', label: 'B', title: 'Tebal (Ctrl+B)', style: 'font-extrabold' },
  { cmd: 'italic', label: 'I', title: 'Miring (Ctrl+I)', style: 'italic' },
  { cmd: 'underline', label: 'U', title: 'Garis bawah (Ctrl+U)', style: 'underline' },
  { cmd: 'insertUnorderedList', label: '•', title: 'Daftar berpoin', style: '' },
  { cmd: 'insertOrderedList', label: '1.', title: 'Daftar bernomor', style: '' },
] as const

export function RichText({
  value, onChange, id, maxTextLength = 5000, placeholder,
}: {
  value: string
  onChange: (html: string) => void
  id?: string
  maxTextLength?: number
  placeholder?: string
}) {
  const ref = useRef<HTMLDivElement>(null)
  const [textLength, setTextLength] = useState(0)

  // The editor is uncontrolled once mounted. Writing `value` back into
  // innerHTML on every keystroke would move the caret to the start on every
  // character, which is the classic contenteditable bug.
  useEffect(() => {
    const el = ref.current
    if (el && el.innerHTML !== value) el.innerHTML = value || ''
    setTextLength((el?.innerText ?? '').trim().length)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const emit = () => {
    const el = ref.current
    if (!el) return
    setTextLength(el.innerText.trim().length)
    onChange(el.innerHTML)
  }

  const exec = (cmd: string) => {
    ref.current?.focus()
    // document.execCommand is deprecated and is still the only thing every
    // browser implements for this. The replacement (a Selection-based
    // implementation) would be several hundred more lines for a field that
    // formats a promotion rule.
    document.execCommand(cmd)
    emit()
  }

  const over = textLength > maxTextLength

  return (
    <div>
      <div className="flex flex-wrap items-center gap-1 border border-divider border-b-0 p-1 bg-surface"
           role="toolbar" aria-label="Format teks">
        {TOOLBAR.map((b) => (
          <button
            key={b.cmd}
            type="button"
            title={b.title}
            aria-label={b.title}
            // onMouseDown, not onClick: the button must not take focus, or the
            // selection in the editor collapses before the command runs.
            onMouseDown={(e) => { e.preventDefault(); exec(b.cmd) }}
            className={`min-w-[32px] h-8 px-2 text-sm border border-transparent hover:bg-neutral-200 ${b.style}`}
          >
            {b.label}
          </button>
        ))}
        <span className="w-px h-5 bg-divider mx-1" aria-hidden="true" />
        <button
          type="button"
          title="Hapus format"
          aria-label="Hapus format"
          onMouseDown={(e) => { e.preventDefault(); exec('removeFormat') }}
          className="min-w-[32px] h-8 px-2 text-xs border border-transparent hover:bg-neutral-200"
        >
          Tx
        </button>
      </div>

      <div
        id={id}
        ref={ref}
        contentEditable
        suppressContentEditableWarning
        role="textbox"
        aria-multiline="true"
        aria-label="Aturan promo"
        data-placeholder={placeholder}
        onInput={emit}
        onBlur={emit}
        onPaste={(e) => {
          // Paste as PLAIN TEXT. Word and Google Docs paste a wall of styled
          // markup that the server strips anyway; taking the text means what
          // the user sees after pasting is what will be saved, rather than
          // formatting that silently disappears on submit.
          e.preventDefault()
          const text = e.clipboardData.getData('text/plain')
          document.execCommand('insertText', false, text)
          emit()
        }}
        className="input min-h-[160px] max-h-[420px] overflow-y-auto prose-rule"
        style={{ borderColor: over ? '#9e1c28' : undefined }}
      />

      <div className="flex justify-between items-baseline mt-1">
        <p className="text-xs text-muted">
          Tebal, miring, garis bawah dan daftar. Tautan dan gambar tidak
          disimpan — aturan promo adalah teks, bukan halaman.
        </p>
        <p className={`text-xs tnum ${over ? 'text-danger font-semibold' : 'text-muted'}`}>
          {textLength} / {maxTextLength}
        </p>
      </div>
    </div>
  )
}

/**
 * Render stored rule markup.
 *
 * The server sanitised this against an allow-list before storing it, and the
 * page's CSP has no `unsafe-inline` for scripts — so a `<script>` that somehow
 * survived still would not run. Both, not either.
 */
export function RichTextView({ html, className }: { html: string; className?: string }) {
  return (
    <div
      className={`prose-rule ${className ?? ''}`}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}
