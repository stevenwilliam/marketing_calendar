# 08 — Roadmap

**Date:** 2026-09-01 · Order and rationale. `PROGRESS.md` is the live status.

---

## Phase 1 — the marketing calendar

Built in this order because each milestone unblocks the next, and because the
platform modules are what make every later business module cheap.

| # | Milestone | Delivers | Gate |
|---|---|---|---|
| M0 | Documents | this set, confirmed by Steven | Steven's sign-off |
| M1 | Environment | repo, config, `sys_parameters`, health endpoints | service boots from `/etc/marketing_calendar/…env`, secrets masked in the startup line |
| M2 | Schema | every migration, every constraint from `03` | applied to a real PostgreSQL; a test proves each constraint **refuses** a bad write |
| M3 | Domain | `money`, `calendar`, `approval`, `promo`, `target` | the `07` matrices pass, referencing `BR-x.y` |
| M4 | Identity & RBAC | accounts, argon2id, **mandatory TOTP**, roles, permissions | the negative-authz matrix passes for every role |
| M5 | Master data | company, site, site group (+ auto system group), holidays | creating a site creates its group in one transaction; the database **refuses** a cross-brand group member (D31) |
| M6 | Approval engine | generic chains, versions, ANY_OF/ALL_OF, inbox | concurrency test; a chain edit does not disturb an open instance |
| M7 | Targets | year and month, variance, roll-up | the months-need-not-sum test passes |
| M8 | Promotions | create, submit, lead time, overlap, versions | lead time correct across Idul Fitri |
| M9 | Auto-cancel job | the daily job, notify, revive | idempotent; proven against a fixed clock |
| M10 | Importer | CSV import to our own contract (D30), idempotency, rejections, reconciliation | re-import inserts zero; a truncated file fails on the `#TOTAL` trailer |
| M11 | Reports | promotion report, target vs actual, pipe CSV | export matches the screen |
| M12 | Notifications | email on release to the fixed list (D32) | a failed send does not block approval; a test proves chain actors are **not** silently appended |
| M13 | Web UI | every screen, mobile-first, search everywhere | `tools/shot` clean at 360 and 1440 |
| M14 | Security hardening | the `12` control map green | every control has a passing test |
| M15 | Deployment | systemd, nginx, TLS, backups | verified from **another machine**, not with curl on the server |
| M16 | Guides | deployment handbook → user guide → admin guide | a colleague can follow them |

**M6 before M8 is deliberate.** Building promotions first and retrofitting the
chain is how the engine ends up promotion-shaped.

## Phase 2 — measurement

Once the calendar has real data, the questions become about money rather than
process.

- **Promo P&L** — discount cost against incremental sales. A promotion that
  raised revenue and destroyed margin currently looks like a success, because
  phase 1 carries no budget or discount-cost field at all (D29). This is the
  first thing phase 2 fixes, and it starts with a nullable `budget_idr`.
- **Cannibalisation** — normal sales during a promo against the preceding
  weeks, so a promotion that merely moved sales forward is visible.
- **Promo calendar wall** — a month grid across all brands; the screen a
  marketing manager lives in.
- **Target approval** — point the existing chain at the target subject type.
  Cheap, because the engine is generic.
- **POS integration** — a second implementation of the importer port, replacing
  the nightly CSV with the third party's own format (D30). The CSV contract in
  `06-domain-operations.md` §3.0 stays as the fallback and as the shape the
  adapter maps onto.

## Phase 3 — the rest of the superapp

Each reuses identity, master data, approval, notify, reporting and audit. That
reuse is the entire argument for building them properly in phase 1.

| Area | Modules |
|---|---|
| Operations | daily sales close and cash reconciliation · inventory, stock take, wastage · purchasing and goods receipt (**reuses approval**) · recipe/BOM and food cost · supplier master |
| People | roster and shift planning · leave requests (**reuses approval**) · attendance |
| Governance | document repository with expiry alerts (permits, halal, certifications) · asset register and maintenance · incident and complaint log |

## Sequencing principles

1. **Platform before business.** The second module is cheap only if the first
   built the shared pieces properly.
2. **Schema and constraints before handlers.** The database is where the
   invariant lives; an application-only rule is a rule until someone writes SQL.
3. **The generic engine before its first consumer.**
4. **Nothing is ✅ until its gate has been run.** Inherited green is not green.

## Explicitly deferred, with the reason

| Deferred | Why | Trigger to revisit |
|---|---|---|
| Monthly partitioning of `history_txn` | complicates the importer for no measured benefit | the promo report misses N3 |
| Multi-currency | one country, one currency | a fourth brand outside Indonesia |
| Native mobile apps | the web app is mobile-first | field staff need offline |
| A public API | nothing outside the group consumes this | a partner integration |
| High availability | single node is an accepted phase-1 trade | the business declares a downtime cost |
