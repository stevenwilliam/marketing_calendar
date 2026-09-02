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

Anchored on **`#778AAB`** — Steven's colour (D34). It is a 218° slate blue, and
at **3.50 on white** it is a *boundary*, not an *ink*: it is the control border
and the chrome fill, and `--primary` is the same hue darkened until it can
carry text. Every other value is chosen around that anchor and measured.

| Token | Hex | Role |
| --- | --- | --- |
| `--canvas` | `#F2F5F9` | the page — a cool tint of the accent hue |
| `--surface` | `#FFFFFF` | cards, tables, panels |
| `--ink` | `#151B24` | primary text |
| `--ink-muted` | `#475466` | secondary text |
| `--primary` | `#2E4C7E` | primary action, focus ring — `#778AAB` at hue 218, darkened |
| `--accent` | `#778AAB` | **Steven's colour** — control boundary, chrome fill, chip |
| `--success` | `#146B3C` | approved, loaded |
| `--warn` | `#845000` | pending, near a deadline |
| `--danger` | `#9E1C28` | rejected, cancelled |
| `--info` | `#0F5F73` | informational, force-released |
| `--border` | `#778AAB` | **control boundary** — the accent, and it must clear 3:1 |
| `--hairline` | `#DDE3EC` | decorative separator — no 3:1 duty |
| `--brand-maxx` | `#6B3B2A` | Maxx Coffee |
| `--brand-ruuma` | `#7A2E63` | Ruuma |
| `--brand-sunshine` | `#7C4A00` | Sunshine |

Hue separation, so two meanings never arrive as the same colour: primary 218°,
accent 218° (deliberately the same family), info 192°, success 148°, warn 36°,
danger 355°, Maxx 16°, Ruuma 318°, Sunshine 36°.

> **Sunshine and `--warn` share the 36° amber family.** They are never on the
> same element — a brand accent is a 3px left border plus the brand name, a
> status is a pill with a glyph and a word — and rule 1 below means neither is
> carried by colour alone. It is a known adjacency, not an oversight.

---

## 3. Measured contrast — check here before choosing a colour

**Text on the two grounds:**

| Ink | on canvas | on surface | |
| --- | ---: | ---: | --- |
| `#151B24` ink | 15.82 | 17.30 | AAA |
| `#475466` muted | 7.04 | 7.70 | AAA |
| `#2E4C7E` primary | 7.83 | 8.57 | AAA |

**Status inks, on both grounds** — a pill sits on either:

| Ink | on canvas | on surface | |
| --- | ---: | ---: | --- |
| `#9E1C28` danger | 7.25 | 7.92 | AAA |
| `#0F5F73` info | 6.61 | 7.23 | AA / AAA |
| `#845000` warn | 6.14 | 6.71 | AA |
| `#146B3C` success | 6.00 | 6.57 | AA |

**White on a filled button:**

| Fill | Ratio | |
| --- | ---: | --- |
| `#2E4C7E` primary | 8.57 | AAA |
| `#9E1C28` danger | 7.92 | AAA |
| `#146B3C` success | 6.57 | AA |

**Filling with the accent** — `#778AAB` takes **dark ink, never white**:

| Pairing | Ratio | |
| --- | ---: | --- |
| `#151B24` ink on `#778AAB` | 4.95 | AA — the sanctioned fill |
| white on `#778AAB` | 3.50 | **FAIL** — see the rejection below |

**Brand accents** — both as ink on white and as a fill with white ink, because
the ratio is symmetric and both uses appear:

| Brand | Ratio | |
| --- | ---: | --- |
| `#6B3B2A` Maxx Coffee | 9.19 | AAA |
| `#7A2E63` Ruuma | 8.76 | AAA |
| `#7C4A00` Sunshine | 7.40 | AAA |

**Non-text — WCAG 1.4.11, the 3:1 boundary.** A border that has to be *found*
is a control boundary and must clear 3:1. A line that merely separates two
filled surfaces need not. Confusing the two is how an input ends up with an
edge nobody can see.

| Token | on canvas | on surface | |
| --- | ---: | ---: | --- |
| `--border` / `--accent` `#778AAB` | 3.20 | 3.50 | ✓ the real control boundary |
| `--primary` as a focus ring | 7.83 | — | ✓ |
| `--hairline` `#DDE3EC` | 1.18 | 1.29 | ✗ decorative only |

> **`#8A97A3` is rejected as a border and should not be re-proposed.** It reads
> as a perfectly reasonable grey and measures **2.98** on white — under the 3:1
> floor by 0.02. It is recorded in `scripts/contrast.py` so the checker keeps
> saying so.

> **White text on `#778AAB` is rejected**, and this is the mistake the palette
> invites: **3.50** is a *pass* as a control boundary and a *fail* as text, and
> it is the same number doing both jobs. A filled accent chip takes `--ink`.
> Also recorded in the checker.

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
   over. `#778AAB` is reserved as the dark-theme `--primary`, where it has the
   headroom it lacks on white — but that is a plan, not a measurement, and dark
   is not claimed until the checker has the numbers.
8. **`#778AAB` fills with `--ink`, never with white.** It is the accent, the
   control boundary and the chrome, and it is 3.50 — which passes as an edge
   and fails as text. One number, two verdicts; read rule 2 before using it.
