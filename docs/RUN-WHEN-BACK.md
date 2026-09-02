# RUN-WHEN-BACK

Steps that need Steven, an interactive terminal, or a machine that does not
exist yet. Everything else is done and running.

**Date:** 2026-09-02

---

## 1. Decisions only Steven can make

| # | Item | Where it bites | What I did meanwhile |
|---|---|---|---|
| 1 | **The nginx allowlist ranges.** The office subnets and the VPN range. | `deploy/nginx-marketing-calendar.conf` — the `allow` lines are the whole of "not public facing" | Allowed `127.0.0.1` and `192.168.88.0/24` (the dev LAN). The VPN line is present and **commented out**, because guessing a range that grants access is worse than leaving it shut |
| 2 | **Port 8093** on `claudedev`. | Everything | 8081, 8082, 8090 and 8091 are taken by other projects. Picked 8093, recorded as D43. Change `MC_HTTP_ADDR` and the nginx `upstream` together if it must move |
| 3 | **The real release-recipient list.** | `notify.release_recipients` in **Pengaturan** | Seeded with `marketing@sfg.local, operasional@sfg.local`. These are placeholders and will silently deliver nowhere |
| 4 | **The nine demo accounts.** | `mc seed` creates them sharing one password | Fine on the dev server, **must be deleted before real data**. The handbook says so at the point of running the seed |
| 5 | **R1 — is Business Analyst step 2 or step 1?** | The default chain | Read "after marketing" as *after the Marketing Head step*. If it meant after Marketing Staff creates, it is a back-office chain edit |
| 6 | **Confirm or amend D2–D22.** | The whole build rests on them | Built to the defaults. Reversing one now costs more than it did |

---

## 2. Needs a production machine

None of this can be executed here, and none of it is claimed as done.

- **TLS.** `certbot --nginx` needs the hostname to resolve. On an internal name
  the HTTP-01 challenge will not work — use DNS-01 or an internal CA. The
  handbook (`13` §7) has the commands and marks them not yet run.
- **The allowlist from outside.** `curl` on the server proves nothing about who
  else can reach it. Test from an allowed machine (expect 200) and from one
  outside (expect 403). I did prove the mechanism works: a request from a
  container got **403**, from localhost and the LAN **200**.
- **A restore.** `pg_restore` succeeding is not a restore. Run `mc migrate
  status` against the restored database and open a promotion report for a month
  that had data. `13` §9 has the exact commands.
- **Backups.** `mc-backup.sh`, its service and its timer are written out in
  `13` §9 and are **not installed here** — the dev server is not where the data
  that matters lives.

---

## 3. Things worth doing early, that I could not

- **Load test (N2/N3).** The 500 ms budget at 200 sites and 24 months is
  unmeasured. There are 9 sites and 2,800 transactions here, which proves
  correctness and nothing about the budget.
- **Screen reader and keyboard-only pass.** Contrast is measured (39/39
  pairings). Nobody has driven the approval screen with a keyboard alone or
  listened to it.
- **Dark theme.** Not shipped and deliberately not claimed. The sidebar is a
  dark *component* with its three inks measured; that is not a theme.
- **Next year's holidays.** 2026 and 2027 are seeded. Somebody loads 2028 in
  December 2027, and the lead time is quietly wrong around Idul Fitri if they
  do not.

---

## 4. Left alone on purpose

- **`user_site_scope` is enforced but unused.** No seeded account has a site
  scope. The filter is in the query and tested in both directions, so setting
  one will work — but nobody has set one in anger.
- **Idempotency-Key.** The table exists; no handler consumes it. Adding it
  later is a middleware, not a reshape.
- **S3/MinIO.** Configuration surface only. Nothing stores an object yet.
- **WhatsApp.** The `Sender` port has one implementation. WAHA is phase 2, and
  the port is the reason that stays cheap.

---

## 5. Blocked and worked around

**The design canvas at `claude.ai/design/p/0a1c8b9c-…` is unreachable from this
machine** — it returns 403 to every tool available here, and it is not among
the artifacts shared with this account. I built to the artifact
`f896dbfe` instead, which *was* reachable and contained the full Modernist
system plus mockups for all ten screens. It is saved verbatim at
`docs/design/mockup.html`.

If the canvas holds something the artifact did not, point me at it and the
difference is a token edit: every colour, size and radius is a variable in
`web/tailwind.config.js` and `web/src/index.css`, measured in
`scripts/contrast.py`, and recorded in `design.md`.

**Six values in the guideline fail WCAG AA** and were retuned to the nearest
passing value from Steven's own ramp (D39). The originals are recorded in the
contrast checker by name so they cannot drift back in from the mockup. The one
worth knowing: the divider at 40% measures **2.41** against a 3:1 floor, and it
is the visible edge of every input in the product.
