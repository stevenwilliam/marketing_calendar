// Parse every mermaid block in docs/ with the real mermaid parser.
//
// A diagram that renders as an error box on GitHub is worse than no diagram:
// it is a document that looks maintained and is not. Checking the fence says
// `mermaid` proves nothing — this runs the parser.
//
//   make diagrams          (from the repo root)
//
// It lives beside its own node_modules because ESM resolves bare imports from
// the IMPORTING FILE's directory upward — not from the working directory, and
// not from NODE_PATH. Keeping it here also means a docs check never touches
// the application's dependency tree.
//
// If mermaid or jsdom are missing it SKIPS **loudly**, because a check that
// silently does nothing is exactly the failure it exists to prevent.
import fs from 'node:fs'
import path from 'node:path'

// Relative to this file, so the check works from any working directory.
const ROOT = path.resolve(path.dirname(new URL(import.meta.url).pathname), '..', '..')
const DOCS = path.join(ROOT, 'docs')

let JSDOM, mermaid
try {
  ;({ JSDOM } = await import('jsdom'))
  const dom = new JSDOM('<!doctype html><body></body>', { pretendToBeVisual: true })
  global.window = dom.window
  global.document = dom.window.document
  ;({ default: mermaid } = await import('mermaid'))
  mermaid.initialize({ startOnLoad: false, securityLevel: 'loose' })
} catch {
  console.error('SKIPPED: mermaid and jsdom are not installed.')
  console.error('  npm --prefix scripts/mermaid-check install')
  process.exit(0)
}

const files = fs.readdirSync(DOCS).filter((f) => f.endsWith('.md'))
let blocks = 0
let bad = 0

for (const f of files) {
  const src = fs.readFileSync(path.join(DOCS, f), 'utf8')
  const found = [...src.matchAll(/```mermaid\n([\s\S]*?)```/g)].map((m) => m[1])
  for (const [i, b] of found.entries()) {
    blocks++
    try {
      await mermaid.parse(b)
    } catch (e) {
      bad++
      console.error(`${f} block ${i + 1}: ${String(e.message || e).split('\n')[0]}`)
    }
  }
}

if (bad > 0) {
  console.error(`\n${bad} of ${blocks} diagram(s) do NOT parse.`)
  process.exit(1)
}
console.log(`All ${blocks} mermaid diagrams across ${files.length} documents parse.`)
