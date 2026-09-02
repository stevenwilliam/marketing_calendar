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

## 2. Palette

Every value measured, never estimated. Full table in `design.md` §2–§3.

**The palette is anchored on `#778AAB`, Steven's colour (D34).** It is a 218°
slate blue that measures **3.50 on white**, which decides its job for it: at
that ratio it is a *boundary*, not an *ink*. So it is the **control border** on
every input, the chrome fill, and the accent chip — and `--primary`, the colour
that has to carry text and fill buttons, is the same 218° hue darkened until it
does. Steven's colour is on every screen; it is simply on the edges rather than
in the words.

| Token | Hex | Role |
|---|---|---|
| `--canvas` | `#F2F5F9` | the page — a cool tint of the accent hue |
| `--surface` | `#FFFFFF` | cards, tables, panels |
| `--ink` | `#151B24` | primary text — 15.82 on canvas, 17.30 on surface |
| `--ink-muted` | `#475466` | secondary — 7.04 / 7.70 |
| `--primary` | `#2E4C7E` | primary action and focus ring — 7.83 / 8.57 |
| `--accent` | `#778AAB` | **Steven's colour** — border, chrome, chip; 3.20 / 3.50 |
| `--success` | `#146B3C` | 6.00 / 6.57 |
| `--warn` | `#845000` | 6.14 / 6.71 |
| `--danger` | `#9E1C28` | 7.25 / 7.92 |
| `--info` | `#0F5F73` | 6.61 / 7.23 |
| `--border` | `#778AAB` | **control boundary** — the accent; clears 1.4.11 |
| `--hairline` | `#DDE3EC` | decorative separator, 1.18 / 1.29 — no 3:1 duty |
| `--brand-maxx` | `#6B3B2A` | Maxx Coffee, 9.19 |
| `--brand-ruuma` | `#7A2E63` | Ruuma, 8.76 |
| `--brand-sunshine` | `#7C4A00` | Sunshine, 7.40 |

Two rejections are recorded in `scripts/contrast.py` so they survive the next
person's good idea:

> `#8A97A3` was the obvious border grey and is **rejected**: 2.98 on white,
> under the 3:1 floor by 0.02.

> **White text on `#778AAB` is rejected.** 3.50 is a *pass* as a control
> boundary and a *fail* as text — the same number, two verdicts. A filled
> accent chip takes `--ink` (4.95), never white.

Hues are separated so two meanings never arrive as the same colour: primary
218°, accent 218° (the same family, deliberately), info 192°, success 148°,
warn 36°, danger 355°, Maxx 16°, Ruuma 318°, Sunshine 36°. **Sunshine and
`--warn` share the amber family** — a known adjacency, not an oversight: they
never appear on the same element, and neither is ever carried by colour alone.

## 3. Typography

Inter, self-hosted, variable. No display face. 15px base — this is a dense
data application and the extra row per screen is worth more than the extra
point of size. 13px is the floor and only for secondary text.

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

- Visible focus ring on everything focusable: 3px `--primary`, 2px offset.
  Never `--hairline` — a ring nobody can see is not a ring.
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

`#778AAB` is reserved as the dark-theme `--primary`: against a dark ground it
has the headroom it lacks on white, so Steven's colour becomes the action
colour there rather than the boundary. That is the plan and not a measurement —
it enters `design.md` when `scripts/contrast.py` has the numbers.

## 8. Language

Indonesian is the default, English second, both through message catalogues
(D20). No inline strings. A missing key is a build-time failure, not a screen
rendering `promo.status.pending` to the CFO.
