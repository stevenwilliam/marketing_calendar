# marketing_calendar — build status

✅ done & tested · 🟡 partial · ⬜ not started

**A ✅ here has been re-earned by running the gate in THIS repo.** Nothing is
inherited from another project.

**Last updated:** 2026-09-07 — the application is **built, running and
reachable** at **`http://192.168.88.101:8094/`** from `192.168.88.0/24` and
`172.16.0.0/24`. Steven's own superadmin account exists, and
`16-uat-scenario-handbook.md` is ready to run.

---

## Deployed right now

**Running.** `marketing-calendar.service` under systemd, plus two timers.

| Piece | State |
|---|---|
| Go service | active, loopback only, `127.0.0.1:8093` |
| Database | `marketing_calendar`, 9 migrations applied, seeded |
| nginx | `/etc/nginx/sites-available/marketing-calendar`, IP allowlist, verified 403 from outside |
| Reachable at | **`http://192.168.88.101:8094/`** — LAN only, ufw + nginx allowlist |
| Import timer | 01:00 Asia/Jakarta, armed |
| Auto-cancel timer | 02:00 Asia/Jakarta, armed |
| Frontend | built and embedded in the binary |

TLS is **not** configured on the dev server: there is no certificate for an
internal hostname. The production handbook covers it and it is marked as not
yet run.

---

## Milestones

| # | Milestone | State | Notes |
|---|---|---|---|
| M0 | Documents | ✅ | Set complete, D28–D43 recorded |
| M1 | Environment & config | ✅ | Boots from `/etc/marketing_calendar/…env`; secrets verified masked in the startup line |
| M2 | Schema | ✅ | 15 migrations on a real PostgreSQL; constraints proven to **refuse** bad writes, `TRUNCATE` included |
| M3 | Domain | ✅ | 5 packages, pure, no I/O; every test names its `BR-x.y` |
| M4 | Identity & RBAC | ✅ | argon2id, rotating refresh, TOTP **available but off by default** (D46); matrix tested in both directions for all 8 roles, and login tested with the factor both on and off |
| M5 | Master data | ✅ | Site creates its system group in one transaction; cross-brand member refused by the database |
| M6 | Approval engine | ✅ | Generic; concurrency test proves exactly one of two racing approvers wins |
| M7 | Targets | ✅ | Months-need-not-sum-to-year proven by a test that fails if the validation is added |
| M8 | Promotions | ✅ | Lead time correct across Idul Fitri; overlap detected across groups sharing a site |
| M9 | Auto-cancel job | ✅ | Ran live; cancelled a due plan; event carries `actor=NULL` |
| M10 | Importer | ✅ | Ran live; idempotent by checksum; rejections kept with reasons; truncation caught |
| M11 | Reports | ✅ | Promo and target-vs-actual, pipe CSV, export matches the screen; achievement is one measured badge (D56) and a zero is only a miss where selling was possible (BR-7.5b, D58, `promo.Verdict` unit-tested over all ten cases); the green line is `promo.achievement_green_bps` and moving it live was **watched flipping the badges and their labels together** (D59) |
| M12 | Notifications | ✅ | Queued not blocking; release goes to the fixed list only |
| M13 | Web UI | ✅ | 11 screens; verified in a real browser and **measured**, not eyeballed |
| M14 | Security hardening | ✅ | Controls built and tested, including site scoping (BR-5.5) and the export cap, both of which were written down as gaps and then closed rather than left as prose |
| M15 | Deployment | ✅ (dev) / 🟡 (production) | systemd + nginx verified on `claudedev`; production TLS never run |
| M16 | Guides | ✅ | `13` handbook, `14` user guide, `15` admin guide, `16` UAT scenario handbook |

---

## What has actually been run

| Gate | Result |
|---|---|
| `go test ./...` | **10 packages green**, run twice to prove repeatability |
| `go vet ./...` | clean |
| `gofmt -l internal cmd test` | clean |
| `scripts/contrast.py` | **39/39 pairings measured**, every ratio present in `design.md` |
| `npm audit` | **0 vulnerabilities** (4 fixed by upgrading react-router-dom and vite) |
| `tsc --noEmit` | clean |
| `nginx -t` | ok |
| Migrations | 9 applied to a real PostgreSQL |
| Constraints refusing bad writes | cross-brand member, normal-row-with-promo-id, append-only UPDATE/DELETE/TRUNCATE, NULL company_id — **all verified refusing** |
| Approval chain end to end | walked all 5 steps live with real TOTP codes |
| IP allowlist | **403 from a container, 200 from localhost and the LAN** |
| Browser rendering | screenshots at 1440 and 360; computed styles measured |

### What the run found

Five real defects, each fixed and covered:

1. **`TRUNCATE` bypassed the append-only guarantee.** A row trigger cannot fire
   on a statement-level operation. The first probe looked like a pass because
   the table was empty.
2. **`LoadFile` wrote transactions before the `import_run` row they reference**,
   and `import_run` is append-only so counts could not be back-filled.
3. **The trailer TOTAL failed a whole file for one bad row**, losing a night of
   trading to guard something the row count already guarded.
4. **`= ANY(?)` with a Go slice** — gorm expands slices for `IN (?)`, producing
   a malformed array literal.
5. **The money overflow guard could not fire**, because a sign-flip check does
   not trip when both operands sit under the domain bound.

Plus two in tests that would have gone quiet: a fixed-content import test that
passed vacuously on its second run, and a UUIDv7 prefix used as a unique
suffix — twice.

---

## Not built, or built and not proven

Be specific rather than reassuring.

| Item | State |
|---|---|
| **TLS in production** | Never run. No production machine exists. `13` §7 covers it |
| **Dark theme** | Not shipped. Not claimed. Ratios are not measured, so it is not supported |
| **TOTP** | Built, tested, and **switched off** by parameter. Not removed — the enrolment flow, `user_totp` and verification all remain |
| **WhatsApp notifications** | The `Sender` port exists with one implementation (SMTP). WAHA is phase 2 |
| **S3/MinIO** | Config surface exists; no code path uses object storage yet |
| **Prometheus metrics** | `/metrics` is exposed and localhost-only; no dashboards or alerts |
| **Idempotency-Key on writes** | Table exists; no handler consumes it |
| **Load testing (N2/N3)** | Not run. The 500 ms budget at 200 sites is unmeasured |
| **Accessibility audit beyond contrast** | Contrast measured; no screen-reader pass, no keyboard-only walkthrough |

---

## Blocked on Steven

| # | Item | Effect |
|---|---|---|
| 1 | **Confirm or amend D2–D22** | The build was made against these defaults |
| 2 | **The nginx allowlist ranges** | `deploy/nginx-…conf` allows `127.0.0.1`, `192.168.88.0/24` (dev LAN) and `172.16.0.0/24` — the range Steven's laptop actually arrives from, added once a 403 proved the guess wrong. Any other office or VPN range still has to be named |
| 3 | **Ports 8093/8094 acceptable?** | 8093 is the Go service (loopback); 8094 is nginx's LAN door. 8081/8082/8090/8091 are taken by other projects (D43, D44) |
| 4 | **The real release-recipient list** | Seeded with placeholders `marketing@sfg.local`, `operasional@sfg.local` |
| 5 | **The demo accounts** | Nine seeded accounts share one password. Delete them before real data. `steven.william@maxx-coffee.id` is a real account and is **not** one of them |
| 6 | R1 — is Business Analyst step 2 (taken) or step 1? | A chain edit either way |
| 7 | **The design canvas** | `claude.ai/design/p/…` is unreachable from the dev server (403). Built to the artifact `f896dbfe` instead, which was reachable |
| 8 | **`auth.password_min_length` is 8 and `auth.totp_required` is false** | Both on request (D45, D46). D45's argument rested on three controls and D46 removed one of them, so what now protects an account is lockout plus the nginx allowlist. Both are parameters — raise either in **Pengaturan**, no deploy |

---

## Decisions taken by default, not by Steven

**D2–D22** remain as they were. The three most expensive to reverse now that
code exists:

| Decision | Why it is expensive now |
|---|---|
| **D6** — `history_txn` is per receipt | 2,800 rows loaded; changing the grain means reloading |
| **D12** — approval locks the plan | Structural in the schema and the domain |
| **D18** — TOTP mandatory | ~~Nine accounts enrolled; relaxing is easy, imposing later is not~~ — **superseded by D46**, which switched the factor off at Steven's request. The enrolment flow and `user_totp` remain, so re-imposing it is a parameter change |
