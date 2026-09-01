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

| Role | Family | Notes |
| --- | --- | --- |
| UI / body | **Inter** (variable), self-hosted | everything |
| Numeric | Inter with `font-variant-numeric: tabular-nums` | every money and count column |
| Mono | ui-monospace stack | codes, ids, CSV previews |

No display face. This is an internal tool used for hours a day; a personality
typeface earns nothing and costs legibility at 13px in a dense table.

Never a font CDN — it hands a third party every visitor's IP and the page they
are on, and this application is deliberately not public.

| Token | Size | | Token | Weight |
| --- | --- | --- | --- | --- |
| `--text-xs` | 12px | | `--w-body` | 400 |
| `--text-sm` | 13px | | `--w-medium` | 500 |
| `--text-base` | 15px | | `--w-strong` | 600 |
| `--text-lg` | 18px | | `--w-heading` | 650 |
| `--text-xl` | 22px | | | |
| `--text-2xl` | 28px | | | |

15px base rather than 16: this is a dense data application and the extra row
per screen matters more than it does on a marketing page. 13px is the floor,
and only for secondary text that already clears AA.

---

## 2. Palette

| Token | Hex | Role |
| --- | --- | --- |
| `--canvas` | `#F4F6F8` | the page |
| `--surface` | `#FFFFFF` | cards, tables, panels |
| `--ink` | `#16202A` | primary text |
| `--ink-muted` | `#4A5A6A` | secondary text |
| `--primary` | `#0F5C6B` | primary action, focus ring |
| `--success` | `#1B6B3A` | approved, loaded |
| `--warn` | `#8A5A00` | pending, near a deadline |
| `--danger` | `#A31621` | rejected, cancelled, over budget |
| `--info` | `#1B4F9C` | informational |
| `--border` | `#7C8A97` | **control boundary** — must clear 3:1 |
| `--hairline` | `#DFE4E9` | decorative separator — no 3:1 duty |
| `--brand-maxx` | `#5B3A29` | Maxx Coffee |
| `--brand-ruuma` | `#8A2B3B` | Ruuma |
| `--brand-sunshine` | `#7A5A00` | Sunshine |

---

## 3. Measured contrast — check here before choosing a colour

**Text on the two grounds:**

| Ink | on canvas | on surface | |
| --- | ---: | ---: | --- |
| `#16202A` ink | 15.21 | 16.48 | AAA |
| `#4A5A6A` muted | 6.54 | 7.09 | AA / AAA |
| `#0F5C6B` primary | 7.01 | 7.60 | AAA |

**Status inks on surface:**

| Ink | Ratio | |
| --- | ---: | --- |
| `#1B4F9C` info | 7.94 | AAA |
| `#A31621` danger | 7.80 | AAA |
| `#1B6B3A` success | 6.54 | AA |
| `#8A5A00` warn | 5.93 | AA |

**White on a filled button:**

| Fill | Ratio | |
| --- | ---: | --- |
| `#0F5C6B` primary | 7.60 | AAA |
| `#A31621` danger | 7.80 | AAA |
| `#1B6B3A` success | 6.54 | AA |

**Brand accents** — both as ink on white and as a fill with white ink, because
the ratio is symmetric and both uses appear:

| Brand | Ratio | |
| --- | ---: | --- |
| `#5B3A29` Maxx Coffee | 10.09 | AAA |
| `#8A2B3B` Ruuma | 8.44 | AAA |
| `#7A5A00` Sunshine | 6.38 | AA |

**Non-text — WCAG 1.4.11, the 3:1 boundary.** A border that has to be *found*
is a control boundary and must clear 3:1. A line that merely separates two
filled surfaces need not. Confusing the two is how an input ends up with an
edge nobody can see.

| Token | on canvas | on surface | |
| --- | ---: | ---: | --- |
| `--border` `#7C8A97` | 3.26 | 3.54 | ✓ the real control boundary |
| `--primary` as a focus ring | 7.01 | — | ✓ |
| `--hairline` `#DFE4E9` | 1.18 | 1.28 | ✗ decorative only |

> **`#8A97A3` is rejected as a border and should not be re-proposed.** It reads
> as a perfectly reasonable grey and measures **2.98** on white — under the 3:1
> floor by 0.02. It is recorded in `scripts/contrast.py` so the checker keeps
> saying so.

---

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
   over.
