#!/usr/bin/env python3
"""Measure WCAG contrast ratios for the marketing_calendar palette.

CLAUDE.md §7: "Accessibility is AA minimum, and contrast is *calculated*, not
eyeballed." This is the tool that calculates it. design.md §3 is the recorded
output; run this to re-earn those numbers rather than trusting them.

Usage:
    scripts/contrast.py                 # check every documented pairing
    scripts/contrast.py '#ae1800' '#f3f2f2'
    scripts/contrast.py --alpha 'rgba(32,30,29,0.55)' '#f3f2f2'
"""

from __future__ import annotations

import pathlib
import re
import sys

# --- WCAG 2.1 relative luminance and contrast ------------------------------


def parse_hex(s: str) -> tuple[int, int, int]:
    s = s.strip().lstrip("#")
    if len(s) == 3:
        s = "".join(c * 2 for c in s)
    if len(s) != 6:
        raise ValueError(f"not a hex colour: {s!r}")
    return int(s[0:2], 16), int(s[2:4], 16), int(s[4:6], 16)


RGBA_RE = re.compile(
    r"rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*([\d.]+)\s*)?\)"
)


def parse_rgba(s: str) -> tuple[int, int, int, float]:
    m = RGBA_RE.fullmatch(s.strip())
    if not m:
        raise ValueError(f"not an rgba() colour: {s!r}")
    r, g, b = (int(m.group(i)) for i in (1, 2, 3))
    a = float(m.group(4)) if m.group(4) is not None else 1.0
    return r, g, b, a


def composite(fg: tuple[int, int, int, float], bg: tuple[int, int, int]) -> tuple[int, int, int]:
    """Flatten a translucent colour onto an opaque background.

    A border at 0.60 alpha is not the colour you typed; it is what that colour
    becomes over the surface behind it. Measuring the unflattened value is how
    a 1.69 ratio gets recorded as if it were 11.32.
    """
    r, g, b, a = fg
    return tuple(round(a * c + (1 - a) * d) for c, d in zip((r, g, b), bg))


def _channel(c: int) -> float:
    s = c / 255.0
    return s / 12.92 if s <= 0.04045 else ((s + 0.055) / 1.055) ** 2.4


def luminance(rgb: tuple[int, int, int]) -> float:
    r, g, b = (_channel(c) for c in rgb)
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def ratio(a: tuple[int, int, int], b: tuple[int, int, int]) -> float:
    la, lb = luminance(a), luminance(b)
    hi, lo = max(la, lb), min(la, lb)
    return (hi + 0.05) / (lo + 0.05)


def verdict(r: float, *, large: bool = False, non_text: bool = False) -> str:
    """AA needs 4.5:1 for body text, 3:1 for large text and non-text."""
    if non_text:
        return "PASS 1.4.11" if r >= 3.0 else "FAIL 1.4.11"
    if large:
        if r >= 4.5:
            return "AAA (large)"
        return "AA (large)" if r >= 3.0 else "FAIL"
    if r >= 7.0:
        return "AAA"
    if r >= 4.5:
        return "AA"
    return "FAIL"


def to_rgb(s: str, over: tuple[int, int, int] | None = None) -> tuple[int, int, int]:
    if s.strip().startswith("rgb"):
        r, g, b, a = parse_rgba(s)
        if a >= 1.0:
            return (r, g, b)
        if over is None:
            raise ValueError("a translucent colour needs a background to composite over")
        return composite((r, g, b, a), over)
    return parse_hex(s)


# --- the documented palette -------------------------------------------------
#
# Every value here is measured, never estimated. The recorded ratios below are
# the ones written in .claude/skills/impeccable/design.md, and the script
# asserts the sheet actually contains them rather than claiming it does.

# --- the Modernist palette (D39) -------------------------------------------
#
# Steven's design guideline (artifact f896dbfe) is the identity: Archivo,
# vivid red on warm neutrals, a near-black sidebar, zero radius. It is kept.
#
# What is NOT kept is any value that fails WCAG AA, because CLAUDE.md §7 makes
# AA a hard rule and says contrast is calculated. Six of the guideline's values
# fail. Each is retuned to the nearest passing value FROM STEVEN'S OWN RAMP, so
# the identity survives and the numbers are honest. The failures are recorded
# below as REJECTED entries so they are not quietly reintroduced.

BG      = "#f3f2f2"   # --color-bg, the page
SURFACE = "#eae9e9"   # --color-surface, cards and panels
PAPER   = "#FFFFFF"   # calendar cells and data tables sit on white
TEXT    = "#201e1d"   # --color-text, and the sidebar ground
ACCENT  = "#ec3013"   # --color-accent — FILLS, RULES AND FOCUS RINGS ONLY
ACCENT_INK = "#ae1800"  # accent-700 — the accent when it must carry TEXT
ACCENT_HOVER = "#7c1405"  # accent-800 — primary button hover
MUTED   = "rgba(32,30,29,0.65)"    # secondary text (guideline had .55/.60)
DIVIDER = "rgba(32,30,29,0.55)"    # control boundary (guideline had .40)
SIDE_DIM = "rgba(243,242,242,0.72)"  # sidebar secondary text
SIDE_FAINT = "rgba(243,242,242,0.62)"  # sidebar tertiary (guideline had .45)

SUCCESS = "#145F38"
WARN    = "#6F4400"
DANGER  = "#9E1C28"
INFO    = "#0F5F73"
MAXX    = "#6B3B2A"   # Maxx Coffee
RUUMA   = "#7A2E63"   # Ruuma
SUN     = "#7C4A00"   # Sunshine
NONWORKING = "#fbe4e8" # calendar tint: weekend OR public holiday (D49, D51)

CHECKS: list[tuple[str, str, str, dict]] = [
    ("text on bg",                TEXT, BG, {}),
    ("text on surface",           TEXT, SURFACE, {}),
    ("text on paper",             TEXT, PAPER, {}),
    ("muted on bg",               MUTED, BG, {}),
    ("muted on surface",          MUTED, SURFACE, {}),
    ("muted on paper",            MUTED, PAPER, {}),
    ("accent ink on bg",          ACCENT_INK, BG, {}),
    ("accent ink on surface",     ACCENT_INK, SURFACE, {}),
    ("accent ink on paper",       ACCENT_INK, PAPER, {}),
    ("accent ink on accent-100",  ACCENT_INK, "#fff2ef", {}),
    ("bg on primary button",      BG, ACCENT_INK, {}),
    ("bg on primary hover",       BG, ACCENT_HOVER, {}),
    ("sidebar text on sidebar",   BG, TEXT, {}),
    ("sidebar dim on sidebar",    SIDE_DIM, TEXT, {}),
    ("sidebar faint on sidebar",  SIDE_FAINT, TEXT, {}),
    ("success on bg",             SUCCESS, BG, {}),
    ("success on paper",          SUCCESS, PAPER, {}),
    ("warn on bg",                WARN, BG, {}),
    ("warn on paper",             WARN, PAPER, {}),
    ("danger on bg",              DANGER, BG, {}),
    ("danger on paper",           DANGER, PAPER, {}),
    ("info on bg",                INFO, BG, {}),
    ("info on paper",             INFO, PAPER, {}),
    ("bg on success fill",        BG, SUCCESS, {}),
    ("bg on danger fill",         BG, DANGER, {}),
    ("Maxx Coffee on paper",      MAXX, PAPER, {}),
    ("Ruuma on paper",            RUUMA, PAPER, {}),
    ("Sunshine on paper",         SUN, PAPER, {}),
    # Non-text: WCAG 1.4.11's 3:1 boundary.
    ("divider on bg",             DIVIDER, BG, {"non_text": True}),
    ("divider on surface",        DIVIDER, SURFACE, {"non_text": True}),
    ("divider on paper",          DIVIDER, PAPER, {"non_text": True}),
    ("accent rule/ring on bg",    ACCENT, BG, {"non_text": True}),
    ("accent rule/ring on paper", ACCENT, PAPER, {"non_text": True}),
    # A non-working cell — weekend or public holiday — is a TINT behind
    # ordinary cell content, so everything the cell draws has to survive it.
    ("ink on non-working tint",   TEXT, NONWORKING, {}),
    ("muted on non-working tint", MUTED, NONWORKING, {}),
    ("danger on non-working tint", DANGER, NONWORKING, {}),
    # --- REJECTED: the guideline's own values, kept so they are not restored ---
    # The primary button is the most-used control in the product and its label
    # is 14px — too small to qualify as large text, so 3:1 does not apply.
    ("guideline btn-primary #ec3013 (REJECTED as text)", BG, ACCENT, {}),
    ("guideline accent as link text (REJECTED)", ACCENT, BG, {}),
    # A 40% divider is the input border, the table rule and the nav edge. Same
    # failure as the #8A97A3 border rejected before this palette existed.
    ("guideline divider 40% (REJECTED)", "rgba(32,30,29,0.40)", BG, {"non_text": True}),
    ("guideline muted 55% (REJECTED)", "rgba(32,30,29,0.55)", BG, {}),
    ("guideline table th 60% (REJECTED)", "rgba(32,30,29,0.60)", BG, {}),
    ("guideline sidebar 45% (REJECTED)", "rgba(243,242,242,0.45)", TEXT, {}),
]

RECORDED = {
    "text on bg": 14.86,
    "text on surface": 13.70,
    "text on paper": 16.60,
    "muted on bg": 4.96,
    "muted on surface": 4.78,
    "muted on paper": 5.16,
    "accent ink on bg": 6.41,
    "accent ink on surface": 5.91,
    "accent ink on paper": 7.17,
    "accent ink on accent-100": 6.55,
    "bg on primary button": 6.41,
    "bg on primary hover": 9.59,
    "sidebar text on sidebar": 14.86,
    "sidebar dim on sidebar": 8.29,
    "sidebar faint on sidebar": 6.46,
    "success on bg": 6.90,
    "success on paper": 7.71,
    "warn on bg": 7.51,
    "warn on paper": 8.39,
    "danger on bg": 7.09,
    "danger on paper": 7.92,
    "info on bg": 6.47,
    "info on paper": 7.23,
    "bg on success fill": 6.90,
    "bg on danger fill": 7.09,
    "Maxx Coffee on paper": 9.19,
    "Ruuma on paper": 8.76,
    "Sunshine on paper": 7.40,
    "divider on bg": 3.66,
    "divider on surface": 3.57,
    "divider on paper": 3.78,
    "accent rule/ring on bg": 3.76,
    "accent rule/ring on paper": 4.20,
    "ink on non-working tint": 13.73,
    "muted on non-working tint": 4.80,
    "danger on non-working tint": 6.55,
    "guideline btn-primary #ec3013 (REJECTED as text)": 3.76,
    "guideline accent as link text (REJECTED)": 3.76,
    "guideline divider 40% (REJECTED)": 2.41,
    "guideline muted 55% (REJECTED)": 3.66,
    "guideline table th 60% (REJECTED)": 4.23,
    "guideline sidebar 45% (REJECTED)": 4.06,
}


def main(argv: list[str]) -> int:
    args = [a for a in argv[1:] if a != "--alpha"]
    if len(args) == 2:
        ground = to_rgb(args[1])
        ink = to_rgb(args[0], over=ground)
        r = ratio(ink, ground)
        print(f"{args[0]} on {args[1]} = {r:.2f}  {verdict(r)}")
        return 0

    drift = 0
    width = max(len(label) for label, *_ in CHECKS)
    print(f"{'pairing'.ljust(width)}   ratio   verdict          recorded")
    print("-" * (width + 40))
    for label, ink_s, ground_s, opts in CHECKS:
        ground = to_rgb(ground_s)
        ink = to_rgb(ink_s, over=ground)
        r = ratio(ink, ground)
        rec = RECORDED.get(label)
        note = ""
        if rec is not None and abs(rec - r) > 0.015:
            note = f"  <-- DRIFT, design.md says {rec:.2f}"
            drift += 1
        recorded = f"{rec:.2f}" if rec is not None else "—"
        print(f"{label.ljust(width)}   {r:5.2f}   {verdict(r, **opts).ljust(15)}  {recorded}{note}")

    print()
    if drift:
        print(f"{drift} pairing(s) disagree with the recorded table — one of them is wrong.")
        return 1

    # The RECORDED table above is a hand-kept copy of design.md §3, and a
    # hand-kept copy drifts. The incident log has a guard that silently stopped
    # guarding because its oracle was stale, so this asserts the sheet and the
    # arithmetic still agree instead of claiming it.
    missing = _sheet_drift()
    if missing:
        print(f"{len(missing)} ratio(s) are NOT written in design.md §3:")
        for label, value in missing:
            print(f"  {value:.2f}  ({label})")
        print("The arithmetic and the sheet disagree. Update design.md.")
        return 1

    print(f"All {len(CHECKS)} pairings match, and every ratio appears in design.md §3.")
    return 0


def _sheet_drift() -> list[tuple[str, float]]:
    """Return recorded ratios that do not appear in design.md.

    Deliberately a weak check on strong data: it looks for the NUMBER, not for
    surrounding prose, because prose wanders and a check that reads prose will
    eventually be argued into agreeing with the bug.
    """
    sheet = pathlib.Path(__file__).resolve().parent.parent / ".claude/skills/impeccable/design.md"
    if not sheet.exists():
        # A missing oracle is a FAILURE, not a pass. Returning "no drift"
        # because the sheet is absent is how a guard silently stops guarding —
        # and it did exactly that here until this line was written.
        return [("design.md is MISSING — the sheet this checks against does not exist", 0.0)]
    text = sheet.read_text()
    out = []
    for label, value in RECORDED.items():
        if f"{value:.2f}" not in text:
            out.append((label, value))
    return out


if __name__ == "__main__":
    sys.exit(main(sys.argv))
