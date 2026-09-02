#!/usr/bin/env python3
"""Measure WCAG contrast ratios for the marketing_calendar palette.

CLAUDE.md §7: "Accessibility is AA minimum, and contrast is *calculated*, not
eyeballed." This is the tool that calculates it. design.md §3 is the recorded
output; run this to re-earn those numbers rather than trusting them.

Usage:
    scripts/contrast.py                 # check every documented pairing
    scripts/contrast.py '#2E4C7E' '#FFFFFF'
    scripts/contrast.py --alpha 'rgba(119,138,171,0.60)' '#FFFFFF'
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

CANVAS  = "#F2F5F9"   # the page — a cool tint of the accent hue
SURFACE = "#FFFFFF"   # cards, tables, panels
INK     = "#151B24"   # primary text
MUTED   = "#475466"   # secondary text
PRIMARY = "#2E4C7E"   # primary action — #778AAB darkened to the same hue (218)
ACCENT  = "#778AAB"   # Steven's colour (D34): control boundary and chrome fill
SUCCESS = "#146B3C"
WARN    = "#845000"
DANGER  = "#9E1C28"
INFO    = "#0F5F73"
MAXX    = "#6B3B2A"   # Maxx Coffee
RUUMA   = "#7A2E63"   # Ruuma
SUN     = "#7C4A00"   # Sunshine
BORDER  = ACCENT      # the control boundary IS the accent — must clear 1.4.11's 3:1
HAIRLINE = "#DDE3EC"  # decorative separator — carries no 3:1 duty

CHECKS: list[tuple[str, str, str, dict]] = [
    ("ink on canvas",             INK, CANVAS, {}),
    ("ink on surface",            INK, SURFACE, {}),
    ("muted on canvas",           MUTED, CANVAS, {}),
    ("muted on surface",          MUTED, SURFACE, {}),
    ("primary text on canvas",    PRIMARY, CANVAS, {}),
    ("primary text on surface",   PRIMARY, SURFACE, {}),
    ("white on primary fill",     "#FFFFFF", PRIMARY, {}),
    ("success on canvas",         SUCCESS, CANVAS, {}),
    ("success on surface",        SUCCESS, SURFACE, {}),
    ("warn on canvas",            WARN, CANVAS, {}),
    ("warn on surface",           WARN, SURFACE, {}),
    ("danger on canvas",          DANGER, CANVAS, {}),
    ("danger on surface",         DANGER, SURFACE, {}),
    ("info on canvas",            INFO, CANVAS, {}),
    ("info on surface",           INFO, SURFACE, {}),
    ("white on success fill",     "#FFFFFF", SUCCESS, {}),
    ("white on danger fill",      "#FFFFFF", DANGER, {}),
    ("Maxx Coffee on surface",    MAXX, SURFACE, {}),
    ("Ruuma on surface",          RUUMA, SURFACE, {}),
    ("Sunshine on surface",       SUN, SURFACE, {}),
    ("white on Maxx fill",        "#FFFFFF", MAXX, {}),
    ("white on Ruuma fill",       "#FFFFFF", RUUMA, {}),
    ("white on Sunshine fill",    "#FFFFFF", SUN, {}),
    ("control border on canvas",  BORDER, CANVAS, {"non_text": True}),
    ("control border on surface", BORDER, SURFACE, {"non_text": True}),
    ("focus ring on canvas",      PRIMARY, CANVAS, {"non_text": True}),
    # The sanctioned way to FILL with Steven's colour: dark ink on it, never white.
    ("ink on accent fill",        INK, ACCENT, {}),
    # Recorded as REJECTED so they are not re-proposed.
    # #8A97A3 reads as a perfectly reasonable grey and misses the 3:1
    # control-boundary floor by 0.02.
    ("#8A97A3 as a border (REJECTED)", "#8A97A3", SURFACE, {"non_text": True}),
    # White on the accent is the mistake this palette invites: 3.50 passes as a
    # BORDER and fails as TEXT, and the same number does both jobs.
    ("white on accent fill (REJECTED as text)", "#FFFFFF", ACCENT, {}),
]

RECORDED = {
    "ink on canvas": 15.82,
    "ink on surface": 17.30,
    "muted on canvas": 7.04,
    "muted on surface": 7.70,
    "primary text on canvas": 7.83,
    "primary text on surface": 8.57,
    "white on primary fill": 8.57,
    "success on canvas": 6.00,
    "success on surface": 6.57,
    "warn on canvas": 6.14,
    "warn on surface": 6.71,
    "danger on canvas": 7.25,
    "danger on surface": 7.92,
    "info on canvas": 6.61,
    "info on surface": 7.23,
    "white on success fill": 6.57,
    "white on danger fill": 7.92,
    "Maxx Coffee on surface": 9.19,
    "Ruuma on surface": 8.76,
    "Sunshine on surface": 7.40,
    "white on Maxx fill": 9.19,
    "white on Ruuma fill": 8.76,
    "white on Sunshine fill": 7.40,
    "control border on canvas": 3.20,
    "control border on surface": 3.50,
    "focus ring on canvas": 7.83,
    "ink on accent fill": 4.95,
    "#8A97A3 as a border (REJECTED)": 2.98,
    "white on accent fill (REJECTED as text)": 3.50,
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
