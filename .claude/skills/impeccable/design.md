# marketing_calendar — design reference

The working numbers. `docs/10-design-system.md` carries the reasoning; this is
the sheet to have open while building, so nothing here is a judgement call —
every ratio below was measured with `scripts/contrast.py` and can be
re-measured. The script also **asserts that every ratio it computes appears in
this file**, so the two cannot drift apart silently.

**This file is per project.** The `impeccable` skill beside it is portable; a
palette is not.

---

## 1. Type

**Archivo**, self-hosted, from Steven's design guideline (D39). Headings and
buttons are weight **800**; body is 400; nav and labels 600.

| Role | Family | Notes |
| --- | --- | --- |
| Heading / button / nav | **Archivo 800**, self-hosted | the whole personality of the system |
| UI / body | **Archivo 400** | 15px base, 1.55 line height |
| Numeric | Archivo with `font-variant-numeric: tabular-nums` | every money and count column |
| Mono | ui-monospace stack | codes, ids, CSV previews |

Never a font CDN — it hands a third party every visitor's IP and the page they
are on, and this application is deliberately not public. The `.woff2` files are
served from `web/public/fonts/`.

| Token | Size | | Token | Weight |
| --- | --- | --- | --- | --- |
| `--text-xs` | 11px | | `--w-body` | 400 |
| `--text-sm` | 13px | | `--w-medium` | 600 |
| `--text-base` | 15px | | `--w-heading` | 800 |
| `--text-lg` | 20px | | | |
| `--text-xl` | 25px | | | |
| `--text-2xl` | 32px | | | |

Headings: h1 42 · h2 32 · h3 25 · h4 20 · h5 16 · h6 13 uppercase with
`0.08em` tracking. Line height 1.12, letter-spacing `-0.015em`.

**Radius is 0 everywhere.** `--radius-sm/md/lg` are all `0px`. That is the
system's signature and is not a value to soften.

---

## 2. Palette

Steven's design guideline (artifact `f896dbfe`) is the identity and is kept:
Archivo, a vivid red accent on warm neutrals, a near-black sidebar, square
corners. **Six of its values fail WCAG AA**, and CLAUDE.md §7 makes AA a hard
rule, so each is retuned to the nearest passing value **from Steven's own
ramp** — the identity survives and the numbers are honest. The rejected
originals are in `scripts/contrast.py` so they cannot drift back in.

| Token | Hex / value | Role |
| --- | --- | --- |
| `--color-bg` | `#f3f2f2` | the page |
| `--color-surface` | `#eae9e9` | cards, panels, inputs |
| `--color-paper` | `#FFFFFF` | calendar cells and data tables |
| `--color-text` | `#201e1d` | primary text — **and the sidebar ground** |
| `--color-muted` | `rgba(32,30,29,0.65)` | secondary text |
| `--color-accent` | `#ec3013` | **fills, 2px rules, focus rings — never text** |
| `--color-accent-ink` | `#ae1800` | the accent when it must carry text |
| `--color-accent-hover` | `#7c1405` | primary button hover |
| `--color-divider` | `rgba(32,30,29,0.55)` | control boundary — must clear 3:1 |
| `--color-success` | `#145F38` | approved, released, loaded |
| `--color-warn` | `#6F4400` | pending, near a deadline |
| `--color-danger` | `#9E1C28` | rejected, cancelled |
| `--color-info` | `#0F5F73` | informational, force-released |
| `--color-holiday` | `#fbe4e8` | calendar cell tint for a public holiday |
| `--brand-maxx` | `#6B3B2A` | Maxx Coffee |
| `--brand-ruuma` | `#7A2E63` | Ruuma |
| `--brand-sunshine` | `#7C4A00` | Sunshine |

Steven's tonal ramps (`--color-neutral-*`, `--color-accent-*`,
`--color-accent-2-*`) are carried over unchanged; `accent-700` is what
`--color-accent-ink` points at.

> **`--color-accent` `#ec3013` is a fill, not an ink.** It measures **3.76** on
> the page: a pass as a rule, a boundary or a focus ring, and a **fail** as
> text at any size the product actually uses. The primary button's label is
> 14px, which is not large text, so 3:1 does not apply to it. Anything that has
> to be *read* in the accent colour uses `--color-accent-ink`.

> **`--color-danger` and the brand accent are both red.** Unavoidable in a
> red-branded product. Mitigated the way rule 1 already requires: every status
> carries a glyph and a word, never colour alone.

---

## 3. Measured contrast — check here before choosing a colour

**Text on the three grounds:**

| Ink | on bg | on surface | on paper | |
| --- | ---: | ---: | ---: | --- |
| `#201e1d` text | 14.86 | 13.70 | 16.60 | AAA |
| `rgba(32,30,29,.65)` muted | 4.96 | 4.78 | 5.16 | AA |
| `#ae1800` accent ink | 6.41 | 5.91 | 7.17 | AA / AAA |

`#ae1800` on `--color-accent-100` `#fff2ef` is **6.55** — the accent tag.

**The primary button** — `--color-bg` on a filled ground:

| Fill | Ratio | |
| --- | ---: | --- |
| `#ae1800` accent-ink (rest) | 6.41 | AA |
| `#7c1405` accent-800 (hover) | 9.59 | AAA |
| `#145F38` success | 6.90 | AA |
| `#9E1C28` danger | 7.09 | AAA |

**The sidebar**, `#201e1d` ground:

| Ink | Ratio | |
| --- | ---: | --- |
| `#f3f2f2` primary | 14.86 | AAA |
| `rgba(243,242,242,.72)` dim | 8.29 | AAA |
| `rgba(243,242,242,.62)` faint | 6.46 | AA |

**Status inks** — a pill sits on the page or on a white table row:

| Ink | on bg | on paper | |
| --- | ---: | ---: | --- |
| `#6F4400` warn | 7.51 | 8.39 | AAA |
| `#9E1C28` danger | 7.09 | 7.92 | AAA |
| `#145F38` success | 6.90 | 7.71 | AA / AAA |
| `#0F5F73` info | 6.47 | 7.23 | AA / AAA |

**The holiday tint** — a public holiday cell is `#fbe4e8` behind ordinary cell
content, so everything the cell already draws has to survive it:

| Ink on `#fbe4e8` | Ratio | |
| --- | ---: | --- |
| `#201e1d` ink | 13.73 | AAA |
| `rgba(32,30,29,.65)` muted | 4.80 | AA |
| `#9E1C28` danger | 6.55 | AA |

> The tint is only **1.21** against the white cell beside it, and that is
> deliberate: a background strong enough to be unmissable would fight the
> promotion chips sitting on top of it. So it **cannot be the only signal** —
> a holiday cell also carries the holiday's name. Rule 1, applied to a
> background rather than to a pill.

**Brand accents**, on the white of a table row:

| Brand | Ratio | |
| --- | ---: | --- |
| `#6B3B2A` Maxx Coffee | 9.19 | AAA |
| `#7A2E63` Ruuma | 8.76 | AAA |
| `#7C4A00` Sunshine | 7.40 | AAA |

**Non-text — WCAG 1.4.11, the 3:1 boundary.** A border that has to be *found*
is a control boundary and must clear 3:1.

| Token | on bg | on surface | on paper | |
| --- | ---: | ---: | ---: | --- |
| `--color-divider` `rgba(32,30,29,.55)` | 3.66 | 3.57 | 3.78 | ✓ |
| `--color-accent` as a rule or focus ring | 3.76 | — | 4.20 | ✓ |

### The six values from the guideline that are rejected

Each is in the checker by name, with its measured number, so it cannot be
quietly restored by someone copying from the mockup.

| Guideline value | Where it was used | Measured | Replaced by |
| --- | --- | ---: | --- |
| `#ec3013` fill + `--color-bg` label | **the primary button** | **3.76** | `#ae1800` fill → 6.41 |
| `#ec3013` as text | links, `.btn-ghost`, `.card-kicker`, `.tag-outline` | **3.76** | `#ae1800` → 6.41 |
| `--color-divider` at **40%** | input border, table rules, nav edge | **2.41** | 55% → 3.66 |
| muted text at **55%** | `.text-muted`, `.mc-kick`, `figcaption` | **3.66** | 65% → 4.96 |
| `.table th` at **60%** | every column header | **4.23** | 65% → 4.96 |
| sidebar text at **45%** | the footer line under the user's name | **4.06** | 62% → 6.46 |

> The divider is the one to understand. At 40% it is **2.41** — it fails the
> same 3:1 control-boundary floor that `#8A97A3` failed by 0.02 before this
> palette existed, and it fails it by six times as much. It is the border of
> every input in the product.

## 4. Rules that are not taste

1. **Status is never colour alone.** Every status pill carries a glyph or a
   word as well as a colour. A red pill and a green pill are the same pill to
   roughly one man in twelve.
2. **A border that has to be found clears 3:1.** Use `--border`. `--hairline`
   is for separating two filled surfaces and is never a control edge or a focus
   ring.
3. **Money and counts are `tabular-nums`, right-aligned.** A column of rupiah
   that does not line up cannot be scanned, which is the only reason the column
   exists.
4. **13px is the floor**, and only for secondary text that already clears AA at
   that size.
5. **A disabled control states its reason** next to it. A grey box that will not
   respond is a bug report waiting to be filed.
6. **An element that paints a background-image must also set a
   background-color.** Otherwise its real contrast cannot be measured, and a
   checker will silently measure the text against whatever is behind it.
7. **Dark theme is a token swap, not a second stylesheet.** Every pairing above
   is re-measured for dark before dark ships; none of these numbers carries
   over. The sidebar is already a dark surface and its three inks are measured
   — that is a component, not a theme, and it is not a claim that dark ships.
8. **`#ec3013` fills; `#ae1800` reads.** The brand accent is 3.76 — a pass as a
   rule, a boundary and a focus ring, a fail as text at 14px. If a human has to
   read it, it is `--color-accent-ink`. One colour, two verdicts; this is the
   single easiest mistake to make in this palette.
9. **Radius is 0.** Every `--radius-*` token is `0px`. It is the system's
   signature, not an oversight to round off.
