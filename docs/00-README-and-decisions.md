# marketing_calendar — Document Set

**Version:** 0.1 (documents written, awaiting Steven's confirmation)
**Date:** 1 September 2026
**Status:** the brief landed on 2026-09-01 and is stored verbatim at
`PROMPT.md`. The documents below are written. **No application code until
Steven confirms them**, per `CLAUDE.md` §9 step 2.

---

## 1. What this document set is

The engineering and product spec for **marketing_calendar**, an internal
superapp for Maxx Coffee (coffee shops), Ruuma (restaurants) and Sunshine
(catering). House style — how Steven works, and his stack, database and
security preferences — lives in `99-steven-preference.md` and is portable
between projects.

`02-business-rules.md` is **normative**: where it conflicts with any other
document, it wins. Build and working conventions live in `../CLAUDE.md`.

| # | Document | Purpose | State |
|---|---|---|---|
| 00 | This file | Index, decision log, open questions | ✅ |
| 01 | `01-PRD.md` | Problem, personas, scope, requirements, metrics | ✅ |
| 02 | `02-business-rules.md` | **Normative** business logic, `BR-x.y` | ✅ |
| 03 | `03-data-model.md` | Schema, ERD, DDL, constraints, indexes | ✅ |
| 04 | `04-api-specification.md` | REST contract, error model, idempotency, auth | ✅ |
| 05 | `05-architecture-and-nfr.md` | Architecture, security, performance, observability | ✅ |
| 06 | `06-domain-operations.md` | Operational logic and runbooks | ✅ |
| 07 | `07-test-plan.md` | Strategy, critical scenarios, QA checklist | ✅ |
| 08 | `08-roadmap.md` | Phasing and sequencing | ✅ |
| 09 | `09-deployment.md` | Production deployment, TLS, backups, rollback | ✅ |
| 10 | `10-design-system.md` | Palette with measured contrast, typography, components | ✅ |
| 11 | `11-local-dev-setup.md` | Local/dev environment, everyday commands | ✅ |
| 12 | `12-security.md` | ASVS L2 / Top-10 control map, abuse cases | ✅ |
| 13a | `13a-development-server-preparation.md` | Dev-server handbook | ✅ |
| 99 | `99-steven-preference.md` | Portable engineering DNA — project-agnostic | ✅ |
| — | `PROMPT.md` | Steven's brief, verbatim | ✅ |
| — | `PROGRESS.md` | Live build status | ✅ |
| — | `RUN-WHEN-BACK.md` | Steps needing an interactive terminal | ✅ |

---

## 2. Decision log

Record every decision that changes behaviour here, with a date, and reflect it
in the affected docs the same day.

> **D2–D22 were decided by their proposed defaults, not by Steven.** The brief
> in `PROMPT.md` §9 posed 21 questions, each with a default. Steven asked for
> the documents to be written before answering, so each default was taken so
> the specification could be complete and internally consistent. **Every one is
> reversible and costs only a document edit while no code exists.** They are
> marked *(default)* below. Reversing one after the build starts costs more, so
> they are the first thing to review.

| ID | Date | Decision | Rationale | Docs touched |
|----|------|----------|-----------|--------------|
| D1 | 2026-09-01 | **Adopt Steven's house style verbatim.** `99-steven-preference.md` copied unchanged; `CLAUDE.md` generated from its §3–§9 plus the brief. Hexagonal Go, gin + gorm + PostgreSQL, money as integers with raw SQL on money paths, UUIDv7, numbered forward-only migrations, search box on every list, pipe-delimited CSV export, configurable values in `sys_parameters`, docs updated in the same commit, auto-commit and push to `main`. | Proven across ruuma and evermore; the preference file is written to be project-agnostic precisely so it can be copied. | CLAUDE.md, 99 |
| D2 | 2026-09-01 | *(default)* **Phase 1 is the marketing calendar only**, with the platform modules built to support it: identity, masterdata, approval, notify, reporting, audit, params, importer. | The platform modules are not optional overhead — they are what makes the second and twentieth business module cheap. | 01, 08 |
| D3 | 2026-09-01 | *(default)* **All three brands from day one.** | The model is multi-company either way; excluding a brand saves nothing and hides multi-tenant bugs until later. | 01, 03 |
| D4 | 2026-09-01 | *(default)* **Capacity assumption: up to 200 sites total.** | Sizes indexes and pagination without over-engineering. Nothing in the design breaks at 2,000. | 03, 05 |
| D5 | 2026-09-01 | *(default)* **`history_txn` arrives as a nightly CSV import**, through an idempotent, re-runnable importer with a reconciliation screen. | A POS API may replace it later; the importer sits behind a port so that is an adapter change. Idempotency is the requirement either way — re-importing a day must not double the sales. | 02 BR-6, 03, 06 |
| D6 | 2026-09-01 | *(default)* **The fact table is per receipt**, not aggregated per site/day. | Receipt count is a promotion target. An aggregate cannot be un-summed, and the decision is irreversible once history is loaded. | 02 BR-6.2, 03 |
| D7 | 2026-09-01 | *(default)* **Rupiah only, `BIGINT` whole rupiah.** | Sen is obsolete in retail. Multi-currency is a schema change, not a rewrite, if a fourth brand ever needs it. | 02 BR-1, 03 |
| D8 | 2026-09-01 | *(default)* **24 months of history loaded.** | Enough for year-on-year comparison on the promo report. | 06 |
| D9 | 2026-09-01 | *(default)* **Targets are per site, per month, per sales type**, rolled up to group, brand and company for display. | The lowest grain anyone sets a number at. Rolling up is arithmetic; splitting down is guesswork. | 02 BR-2, 03 |
| D10 | 2026-09-01 | *(default)* **A Finance or Marketing Head role sets targets; no approval chain on targets in phase 1.** | Approval on targets can be added later by pointing the same generic engine at the target subject type. | 02 BR-2.5, 04 |
| D11 | 2026-09-01 | *(default)* **A promotion whose approval chain is incomplete on the Nth day before start is auto-cancelled** (N configurable, default 5). Audited, notifying the creator and the pending approver, and revivable by a superadmin. | Steven's rule that nothing automated cancels a *customer booking* does not bite — this is an internal plan. The safeguards make it recoverable. | 02 BR-4.6, 06 |
| D12 | 2026-09-01 | *(default)* **Approval locks the plan. An edit creates a new version that re-enters the chain from the start**; the previous version is retained. | If an approved plan can be edited, the approval means nothing. This is the single most important control in the module. | 02 BR-4.7, 03 |
| D13 | 2026-09-01 | *(default)* **Overlapping promotions on one site group are allowed, with a loud warning** at creation and a flag on the calendar. | A stacked promotion is sometimes intended. Blocking it outright would be wrong; letting it pass silently is how a store ends up running two conflicting offers. | 02 BR-3.6 |
| D14 | 2026-09-01 | *(default)* **Default approval chain:** Marketing Staff creates → Marketing Manager → Finance Manager → (Brand Head **OR** Operations Head) → Director. Superadmin may force-release. | Matches the shape in the brief, including the either/or step. Fully reconfigurable in the back office. | 02 BR-4, 03 |
| D15 | 2026-09-01 | *(default)* **Rejection returns the plan to the creator with a mandatory reason**; the plan keeps its full history. | Killing the plan loses the work and the audit trail. | 02 BR-4.5 |
| D16 | 2026-09-01 | *(default)* **Email notifications in phase 1; WhatsApp behind the same port for later.** | WAHA is the documented provider, per `99` §9. | 05, 06 |
| D17 | 2026-09-01 | *(default)* **Reached over the internal network with an nginx IP allowlist, TLS on.** | Not public facing. The Go service binds loopback; nginx is the only way in. | 09, 12 |
| D18 | 2026-09-01 | *(default)* **TOTP is mandatory for every staff account.** | One login sees all three brands' sales. Admin-only MFA would leave the largest blast radius unprotected. | 12, 02 BR-5.3 |
| D19 | 2026-09-01 | *(default)* **A user may hold roles in more than one company**, and a group-level role sees all three brands. | The finance and director roles are group-level in a three-brand group; forcing one account per brand would guarantee shared logins. | 02 BR-5, 03, 12 |
| D20 | 2026-09-01 | *(default)* **Indonesian and English, via message catalogues, Indonesian as the default.** | No inline strings, from the first string. | 10, 11 |
| D21 | 2026-09-01 | *(default)* **`Asia/Jakarta` operating zone, UTC storage.** | Business-day logic converts explicitly. | 02 BR-1.4, 03 |
| D22 | 2026-09-01 | *(default)* **An Indonesian public-holiday calendar is required and administrator-maintained**, seeded for the current and next year. | "7 working days" computed on weekdays alone gives the wrong date around Idul Fitri, Christmas and Nyepi — a promotion would be approved for a date the lead time never legitimately allowed. | 02 BR-3.3, 03, 06 |
| D23 | 2026-09-01 | **`site_group` is a many-to-many.** The brief put `site_id` on the group row, which makes it one-site-per-group; a promotion targets many stores. Modelled as `site_group` plus `site_group_member`, with the auto-created per-site group being a group with exactly one member. | The brief's shape cannot express the thing the brief asks for. Flagged rather than implemented as written. | 02 BR-1.3, 03 |
| D24 | 2026-09-01 | **`history_txn` gains `receipt_count` (implicitly, via per-receipt grain) and `order_mode`.** The brief's fact table has neither, yet promotions carry a target receipt count and an order mode. | Without them the promotion report cannot be produced at all. | 02 BR-6.2, 03 |
| D25 | 2026-09-01 | **The approval engine is generic from day one.** `approval` takes a subject type and subject id; promotions are its first consumer and are not special-cased inside it. | A superapp needs approvals for purchase orders, leave, discounts and price changes. A chain written inside the promotion module gets rewritten five times. | 05, 02 BR-4 |
| D26 | 2026-09-01 | **No SEO baseline.** `99` §13 does not apply to this project. | There is no public page. Building titles-for-search, Open Graph, `robots.txt`, `sitemap.xml` and JSON-LD would be dead code. | CLAUDE.md §1, 05 |
| D27 | 2026-09-01 | **Superadmin force-release requires a typed reason and writes an audit row.** | A bypass of an approval chain with no recorded justification is the control most likely to be questioned in a review. | 02 BR-4.8, 12 |

---

## 3. Open questions

Answers go here as they land; each one that changes behaviour becomes a
decision.

**Q1–Q21 are the brief's question batch, answered by default (D2–D22).**
Steven should review those first — while no code exists, reversing any of them
costs a document edit.

Genuinely unanswered, and none of them blocks the documents:

| # | Question | Why it matters | Proposed default |
|---|---|---|---|
| Q22 | What are the **real role names** in the group today? | D14 invented plausible ones. The permission matrix is easy to change now and tedious once accounts exist. | Use D14's names; rename on Steven's list |
| Q23 | Does a promotion need a **budget or discount cost** field? | Without it the promo report shows revenue but not margin, and a promotion that raised sales while destroying margin looks like a success. | Add `budget_idr` as optional in phase 1, and a promo P&L in phase 2 |
| Q24 | Is there an existing **POS export format** to match? | The importer's column mapping depends on it entirely. | Define our own CSV contract; write an adapter when the real format arrives |
| Q25 | Should a site group be **restricted to one brand**? | A group spanning Maxx Coffee and Ruuma is probably a mistake, but might be a deliberate cross-brand campaign. | Allow it, warn at creation |
| Q26 | Who receives the **release email** — a fixed list, or the approvers plus a list? | Recipient lists silently going stale is a common failure. | A maintained list in the back office, plus every actor in the chain |
| Q27 | Retention: how long are **audit and approval events** kept? | Append-only tables grow forever without an answer. | Indefinitely in phase 1; revisit at 24 months |
| Q28 | Does the group have an existing **brand palette** for internal tools? | `10-design-system.md` currently proposes a neutral one with measured contrast. | Use the proposed palette until Steven supplies artwork |

---

## 4. How to read this set in order

1. `PROMPT.md` — what Steven asked for, verbatim.
2. `01-PRD.md` — the problem and the scope.
3. `02-business-rules.md` — **normative.** Everything else defers to it.
4. `03-data-model.md` — how the rules are stored and constrained.
5. `05-architecture-and-nfr.md` — how the code is arranged.
6. `08-roadmap.md` — the order it gets built in.
7. `PROGRESS.md` — what is actually done, at any moment.
