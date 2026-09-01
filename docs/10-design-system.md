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

| Token | Hex | Role |
|---|---|---|
| `--canvas` | `#F4F6F8` | the page |
| `--surface` | `#FFFFFF` | cards, tables, panels |
| `--ink` | `#16202A` | primary text — 15.21 on canvas, 16.48 on surface |
| `--ink-muted` | `#4A5A6A` | secondary — 6.54 / 7.09 |
| `--primary` | `#0F5C6B` | primary action and focus ring — 7.01 / 7.60 |
| `--success` | `#1B6B3A` | 6.54 on surface |
| `--warn` | `#8A5A00` | 5.93 on surface |
| `--danger` | `#A31621` | 7.80 on surface |
| `--info` | `#1B4F9C` | 7.94 on surface |
| `--border` | `#7C8A97` | **control boundary**, 3.26 / 3.54 — clears 1.4.11 |
| `--hairline` | `#DFE4E9` | decorative separator, 1.18 / 1.28 — no 3:1 duty |
| `--brand-maxx` | `#5B3A29` | Maxx Coffee, 10.09 |
| `--brand-ruuma` | `#8A2B3B` | Ruuma, 8.44 |
| `--brand-sunshine` | `#7A5A00` | Sunshine, 6.38 |

> `#8A97A3` was the obvious border grey and is **rejected**: 2.98 on white,
> under the 3:1 floor by 0.02. It is recorded in the checker so the rejection
> survives the next person's good idea.

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

Mobile-first at 360px. A director approving from a phone is a real persona, so
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

## 8. Language

Indonesian is the default, English second, both through message catalogues
(D20). No inline strings. A missing key is a build-time failure, not a screen
rendering `promo.status.pending` to a director.
