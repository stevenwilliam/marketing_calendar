# 10 — Design system

**Date:** 2026-09-01 · The working numbers live in
`.claude/skills/impeccable/design.md`; that is the sheet to have open while
building. This document carries the reasoning and the components.
`scripts/contrast.py` measures every pairing and **asserts the sheet contains
each ratio**, so the two cannot drift.

---

## 1. What this application is, visually

An internal data application used for hours a day by staff who are trying to
get a promotion approved before a deadline. It spans three brands, so it is
**brand-neutral itself** and lets each brand's colour appear only as an accent
on the rows that belong to it.

Priorities, in order: legibility of dense tables · unambiguous status · fast
scanning of numbers · everything else.

There is no marketing surface here (D26), so no hero, no display typeface and
no personality typography. Those cost legibility at 13px and buy nothing.

## 2. Palette — the Modernist system (D39)

**Steven supplied a design guideline** (artifact `f896dbfe`, saved verbatim at
`docs/design/mockup.html`). It is the identity and it is kept: **Archivo**,
a vivid red accent on warm neutrals, a near-black sidebar, **square corners**,
Indonesian copy. It supersedes the `#778AAB` palette of D34 entirely.

**Six of its values fail WCAG AA.** CLAUDE.md §7 makes AA a hard rule and says
contrast is calculated, so each failing value is retuned to the nearest passing
value **taken from Steven's own tonal ramp**. The identity survives; the
numbers are honest. Full table and the rejected originals: `design.md` §2–§3.

| Token | Value | Role |
|---|---|---|
| `--color-bg` | `#f3f2f2` | the page |
| `--color-surface` | `#eae9e9` | cards, panels, inputs |
| `--color-paper` | `#FFFFFF` | calendar cells, data tables |
| `--color-text` | `#201e1d` | primary text — 14.86 on bg — and the sidebar ground |
| `--color-muted` | `rgba(32,30,29,.65)` | secondary — 4.96 |
| `--color-accent` | `#ec3013` | **fills, rules, focus rings — never text** — 3.76 |
| `--color-accent-ink` | `#ae1800` | the accent when it carries text — 6.41 |
| `--color-divider` | `rgba(32,30,29,.55)` | control boundary — 3.66 |
| `--color-success` | `#145F38` | 6.90 |
| `--color-warn` | `#6F4400` | 7.51 |
| `--color-danger` | `#9E1C28` | 7.09 |
| `--color-info` | `#0F5F73` | 6.47 |
| `--brand-maxx` | `#6B3B2A` | Maxx Coffee, 9.19 |
| `--brand-ruuma` | `#7A2E63` | Ruuma, 8.76 |
| `--brand-sunshine` | `#7C4A00` | Sunshine, 7.40 |

### What was changed, and why

| Guideline value | Used for | Measured | Now |
|---|---|---:|---|
| `#ec3013` fill, `#f3f2f2` label | **the primary button** | **3.76** | `#ae1800` fill → 6.41 |
| `#ec3013` as text | links, ghost buttons, kickers | **3.76** | `#ae1800` → 6.41 |
| divider at **40%** | every input border and table rule | **2.41** | 55% → 3.66 |
| muted text at **55%** | captions, meta, kickers | **3.66** | 65% → 4.96 |
| table `th` at **60%** | every column header | **4.23** | 65% → 4.96 |
| sidebar text at **45%** | the line under the user's name | **4.06** | 62% → 6.46 |

> **The divider is the one that matters.** At 40% it measures **2.41** against
> a 3:1 floor — it is the visible edge of every input in the product, and it
> fails by six times the margin that got `#8A97A3` rejected before this palette
> existed. Nothing about it looks wrong; that is exactly why it is measured.

> **`#ec3013` is a fill, not an ink.** 3.76 is a pass as a rule, a boundary or
> a focus ring, and a fail as text at the 14px the buttons actually use. The
> vivid red stays everywhere it does not have to be read: the 2px rules, the
> active nav bar, focus rings, the chip's left border.

> **Danger and the brand are both red.** Unavoidable in a red-branded product,
> and mitigated as the rules already require — every status carries a glyph and
> a word, never colour alone.

## 3. Typography

**Archivo**, self-hosted from `web/public/fonts/`, never a CDN. Headings and
buttons at weight **800**, nav and labels 600, body 400. 15px base, 1.55 line
height. Headings h1 42 · h2 32 · h3 25 · h4 20 · h5 16 · h6 13 uppercase.

**Radius is `0` everywhere** — the system's signature.

**Every money and count column is `tabular-nums` and right-aligned.** A column
of rupiah that does not line up cannot be scanned, which is the only reason
the column exists.

## 4. Components

### Status pill

Status is **never colour alone** — every pill carries a word, and where space
is tight a glyph too.

| Status | Colour | Glyph |
|---|---|---|
| Draft | muted | — |
| Pending approval | warn | ◷ |
| Released | success | ✓ |
| Rejected | danger | ✕ |
| Cancelled | muted | ⊘ |
| Force-released | info + `!` | ! |

Force-released is deliberately distinct. A reviewer must be able to see at a
glance which promotions bypassed the chain (BR-4.8).

### Data table

Every table: a debounced search box above it (BR-7.1), an **Export CSV**
button beside the search (BR-7.2), sortable headers, sticky header on scroll,
`overflow-x: auto` on its own wrapper so the page never scrolls sideways, and
a zero state that says what to do rather than "No data".

### Approval timeline

A vertical list of steps: step name, eligible roles, who decided, when, and the
reason where present. Pending steps show the eligible roles so the viewer knows
who they are waiting for. This is the screen that answers "why is this stuck",
and it is the reason the module exists.

### Date range picker

Two-month view, keyboard operable. **Dates inside the lead time are disabled
and say why** — "Paling cepat 10 Sep 2026 (7 hari kerja)" — rather than being
silently ungreyed (BR-3.3, and the disabled-states-explain-themselves rule).
Public holidays are marked.

### Target grid

Twelve month cells plus the year cell. The **variance between the year target
and the sum of the months is displayed prominently and is never an error**
(BR-2.3). Colour marks over and under; the number carries the meaning.

### Brand accent

A 3px left border on a row or card in the brand's colour, plus the brand name
in text. The colour is a fast scan aid, never the only identification.

## 5. Layout

Mobile-first at 360px. A CFO approving from a phone is a real persona, so
the approval inbox and the approve/reject action must work at that width.

| Breakpoint | Layout |
|---|---|
| 360–767 | one column, tables scroll in their wrapper, actions stack |
| 768–1023 | two columns where it helps, filters inline |
| 1024+ | sidebar navigation, tables full width |

## 6. Accessibility

AA minimum, **calculated not eyeballed** (`scripts/contrast.py`).

- Visible focus ring on everything focusable: **2px `--color-accent`, 2px
  offset** — 3.76, which clears 1.4.11. This is the accent doing the job it is
  good at.
- Real `<label>` for every field. Errors announced, associated, and stating
  what to do.
- Keyboard-operable date picker; the calendar is not mouse-only.
- 44px touch targets for standalone controls.
- `prefers-reduced-motion` and `prefers-color-scheme` respected.
- Colour never the only signal.

## 7. Dark theme

A token swap, not a second stylesheet. **None of the ratios above carries over**
— every pairing is re-measured for dark before dark ships, and until it has
been, dark is not claimed as supported.

The **sidebar** is already a dark surface and its three inks are measured
(14.86 / 8.29 / 6.46). That is a component on a dark ground, not a dark theme,
and it is not a claim that dark ships.

## 8. Language

Indonesian is the default, English second, both through message catalogues
(D20). No inline strings. A missing key is a build-time failure, not a screen
rendering `promo.status.pending` to the CFO.
