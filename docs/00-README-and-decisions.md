# marketing_calendar — Document Set

**Version:** 0.4 (design guideline landed; D28–D39 recorded)
**Date:** 2 September 2026 (written 1 September 2026)
**Status:** the brief landed on 2026-09-01 and is stored verbatim at
`PROMPT.md`. The documents below are written, and Steven's answers to Q22–Q28
are folded in (D28–D38). **No application code until Steven confirms the set**,
per `CLAUDE.md` §9 step 2.

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
| D14 | 2026-09-01 | *(default, **superseded by D28** — kept because a decision log records what was decided, not what is current)* **Default approval chain:** Marketing Staff creates → Marketing Manager → Finance Manager → (Brand Head **OR** Operations Head) → Director. Superadmin may force-release. | Matches the shape in the brief, including the either/or step. Fully reconfigurable in the back office. | 02 BR-4, 03 |
| D15 | 2026-09-01 | *(default)* **Rejection returns the plan to the creator with a mandatory reason**; the plan keeps its full history. | Killing the plan loses the work and the audit trail. | 02 BR-4.5 |
| D16 | 2026-09-01 | *(default)* **Email notifications in phase 1; WhatsApp behind the same port for later.** | WAHA is the documented provider, per `99` §9. | 05, 06 |
| D17 | 2026-09-01 | *(default)* **Reached over the internal network with an nginx IP allowlist, TLS on.** | Not public facing. The Go service binds loopback; nginx is the only way in. | 09, 12 |
| D18 | 2026-09-01 | *(default)* **TOTP is mandatory for every staff account.** | One login sees all three brands' sales. Admin-only MFA would leave the largest blast radius unprotected. | 12, 02 BR-5.3 |
| D19 | 2026-09-01 | *(default, **amended by D37** — the multi-company part stands; the group-level wildcard is withdrawn)* **A user may hold roles in more than one company**, and a group-level role sees all three brands. | The finance and director roles are group-level in a three-brand group; forcing one account per brand would guarantee shared logins. | 02 BR-5, 03, 12 |
| D20 | 2026-09-01 | *(default)* **Indonesian and English, via message catalogues, Indonesian as the default.** | No inline strings, from the first string. | 10, 11 |
| D21 | 2026-09-01 | *(default)* **`Asia/Jakarta` operating zone, UTC storage.** | Business-day logic converts explicitly. | 02 BR-1.4, 03 |
| D22 | 2026-09-01 | *(default)* **An Indonesian public-holiday calendar is required and administrator-maintained**, seeded for the current and next year. | "7 working days" computed on weekdays alone gives the wrong date around Idul Fitri, Christmas and Nyepi — a promotion would be approved for a date the lead time never legitimately allowed. | 02 BR-3.3, 03, 06 |
| D23 | 2026-09-01 | **`site_group` is a many-to-many.** The brief put `site_id` on the group row, which makes it one-site-per-group; a promotion targets many stores. Modelled as `site_group` plus `site_group_member`, with the auto-created per-site group being a group with exactly one member. | The brief's shape cannot express the thing the brief asks for. Flagged rather than implemented as written. | 02 BR-1.3, 03 |
| D24 | 2026-09-01 | **`history_txn` gains `receipt_count` (implicitly, via per-receipt grain) and `order_mode`.** The brief's fact table has neither, yet promotions carry a target receipt count and an order mode. | Without them the promotion report cannot be produced at all. | 02 BR-6.2, 03 |
| D25 | 2026-09-01 | **The approval engine is generic from day one.** `approval` takes a subject type and subject id; promotions are its first consumer and are not special-cased inside it. | A superapp needs approvals for purchase orders, leave, discounts and price changes. A chain written inside the promotion module gets rewritten five times. | 05, 02 BR-4 |
| D26 | 2026-09-01 | **No SEO baseline.** `99` §13 does not apply to this project. | There is no public page. Building titles-for-search, Open Graph, `robots.txt`, `sitemap.xml` and JSON-LD would be dead code. | CLAUDE.md §1, 05 |
| D27 | 2026-09-01 | **Superadmin force-release requires a typed reason and writes an audit row.** | A bypass of an approval chain with no recorded justification is the control most likely to be questioned in a review. | 02 BR-4.8, 12 |
| D28 | 2026-09-02 | **The group's real role names replace D14's invented ones (Q22).** Steven's list, plus CFO added the same day: `marketing_staff`, `marketing_head`, `finance_head`, `operation`, `cfo`, `business_analyst`, `it`, `superadmin`. The default chain becomes **Marketing Head → Finance Head → Operation → CFO**, created by Marketing Staff *(amended the same day by D36, which inserts Business Analyst at step 2)*. `business_analyst` reads and exports and does not approve; `it` is the administrator role; `superadmin` stays a separate break-glass role. | Real names now, while the matrix is a document. D14's either/or step collapses to one role because the group has a single Operation function — the `ANY_OF` machinery stays, since a two-role step is exactly how it returns. | 02 BR-4.2 / BR-5.7, 12 §4, 01 §3, 03 §7 |
| D29 | 2026-09-02 | **No budget or discount-cost field, and no budget limit, in phase 1 (Q23).** `budget_idr` is removed from `promotion_plan_version`. | Steven: "no budget promo limit for now". A nullable column nobody fills is worse than no column — it invites a half-built P&L. The consequence is stated in BR-3.1 rather than discovered: the promo report shows **revenue, not margin**. Phase 2's promo P&L is the real answer, and it starts by adding this column back. | 02 BR-3.1, 03 §4, 08 |
| D30 | 2026-09-02 | **The importer defines our own pipe-delimited CSV contract (Q24)**, specified in `06` §3.0: header matched by name, `#TOTAL` trailer required, file identity by checksum. A POS adapter is a second implementation of the same port, in phase 2. | Steven: "will discuss integration to third party later". Waiting for a format that does not exist yet blocks M10; a contract we control does not. The port is the whole reason the wait costs nothing. | 02 BR-6.1, 06 §3.0, 07 §2.8, 08 |
| D31 | 2026-09-02 | **A site group is restricted to one brand (Q25) — rejected, not warned.** Enforced in the database by a composite foreign key: `site_group_member` carries `company_id`, and both its foreign keys include it. A cross-brand campaign is one plan per brand, permanently. | Steven: "yes restricted". A warning is for something that might be intended; this is not. Enforcing it with a composite FK rather than a trigger or an application check means the next writer cannot route around it. | 02 BR-1.3 / BR-3.7, 03 §2 / §5.6, 04 §4, 07 §2.7 |
| D32 | 2026-09-02 | **The release email goes to a fixed list maintained in the back office (Q26)** — `notify.release_recipients`, a `sys_parameters` row. Chain actors are **not** appended automatically. Workflow notifications to the creator and pending approvers are unaffected. | Steven: "fixed lists, maintained via backend". The default would have appended every actor; he chose the narrower rule, so the list is exactly the list. Its known failure is going stale, so changes are audited and `06` §4 puts reading it into the month-end routine. | 02 BR-4.12, 01 §4.1, 06 §4 / §6, 08 M12 |
| D33 | 2026-09-02 | **No retention limit (Q27).** `audit_log`, `approval_event`, `import_run` and `import_rejection` are kept indefinitely. No purge job exists, and adding one requires reversing this decision. | Steven: "no limit". These are event tables, not transaction tables; the table that grows with trading is `history_txn`, and its answer is partitioning, not deletion. | 02 BR-8.5, 06 §4a |
| D34 | 2026-09-02 | *(**superseded by D39** — Steven supplied a real design guideline on 2026-09-02)* **The palette is rebuilt around `#778AAB` (Q28).** Steven's colour measures **3.50 on white**, which decides its job: it is the **control boundary**, the chrome and the accent chip — not an ink. `--primary` `#2E4C7E` is the same 218° hue darkened until it can carry text (8.57). Canvas `#F2F5F9`, ink `#151B24`, muted `#475466`, success `#146B3C`, warn `#845000`, danger `#9E1C28`, info `#0F5F73`, brands `#6B3B2A` / `#7A2E63` / `#7C4A00`. All 29 pairings measured by `scripts/contrast.py`, which passes. | Steven: "choose moderen color template, i prefer #778aab, others is mix and match". The one trap is recorded in the checker: **3.50 passes as a border and fails as text**, so white on `#778AAB` is rejected and a filled accent chip takes `--ink` (4.95). `#778AAB` is reserved as the dark-theme primary, where it has the headroom — a plan, not yet a measurement. | 10 §2 / §7, `design.md` §2–§3, `scripts/contrast.py` |
| D35 | 2026-09-02 | **`superadmin` is held by IT (Q29).** | Steven: "superadmin = it". The trade is explicit: a technical role can force-release a promotion the business never approved. Compensating controls — mandatory typed reason, audit row naming the actor, `force_released` on the promotion report, and `audit.view` reaching the CFO so the bypass is visible outside IT. | 12 §4, 02 BR-5.7 |
| D36 | 2026-09-02 | **Business Analyst is an approver, at step 2 (Q30).** The chain becomes Marketing Head → **Business Analyst** → Finance Head → Operation → CFO. | Steven: "business analyst is approver also, after marketing". The BA validates the numbers before Finance is asked whether they are affordable, which is the right order. Read as *after the Marketing Head step*, since Marketing Staff creates rather than approves. **Five steps against a 7-working-day lead time and a 5-day auto-cancel leaves about one step per day** — noted in BR-4.2 and `06` §6, and both numbers are parameters. | 02 BR-4.2 / BR-5.7, 01 §3, 03 §7, 07 §2.4 |
| D37 | 2026-09-02 | **A user is assigned one or more companies explicitly at creation; `user_role.company_id` becomes `NOT NULL` (Q31).** The "NULL means all companies" group-level marker from D19 is withdrawn. Approval eligibility is company-scoped (new **BR-4.4a**), which is what lets a single `operation` role serve three brands. | Steven: "when create user, it will choose company (1 or more)". An implicit superset **grows silently** — insert a fourth brand and every group-level account can see its sales with no decision and no audit row. An explicit list grants nobody anything until somebody chooses. It also answers the per-brand-Operation question without adding roles: the separation lives on the user. | 02 BR-4.4a / BR-5.4, 03 §2 / §5.5a, 04 §3a, 12 §4, 07 §2.4 / §2.10 |
| D38 | 2026-09-02 | **`report.export` is granted to Marketing Staff (Q32)**, and to every other business role. **IT remains denied.** | Steven: "permit it". The person planning a promotion needs the numbers behind it, and denying the export while granting `report.view` only routes the same data out through a screenshot. The control that does the work is the audit row on every export, with row count and filters. IT stays out: administering the box is not a reason to carry the sales history off it. | 12 §4, 07 §2.10 |
| D39 | 2026-09-02 | **Steven's design guideline replaces the D34 palette.** Artifact `f896dbfe`, saved verbatim at `docs/design/mockup.html`: the **Modernist** system — Archivo (800/600/400), accent `#ec3013` on warm neutrals `#f3f2f2`/`#eae9e9`, near-black `#201e1d` sidebar, **radius 0**, Indonesian copy, and mockups for all ten screens. The identity is kept exactly. **Six of its values fail WCAG AA and are retuned to the nearest passing value from Steven's own tonal ramp**: the primary button and accent text go to `#ae1800` (3.76 → 6.41), the divider from 40% to 55% (2.41 → 3.66), muted text from 55% to 65% (3.66 → 4.96), table headers from 60% to 65% (4.23 → 4.96), sidebar tertiary from 45% to 62% (4.06 → 6.46). All 39 pairings measured; `scripts/contrast.py` passes and carries every rejected original by name. | Steven chose the look; CLAUDE.md §7 makes AA a hard rule and says contrast is *calculated*. Shipping the guideline unmeasured would have put a **2.41** border on every input in the product and a **3.76** label on the most-used button. Retuning inside his own ramp keeps the design his and the numbers true. `#ec3013` stays vivid everywhere it is not read — 2px rules, the active nav bar, focus rings, the chip border. | 10 §2–§3 / §6–§7, `design.md` §1–§3, `scripts/contrast.py`, `docs/design/mockup.html` |

---

## 3. Open questions

Answers go here as they land; each one that changes behaviour becomes a
decision.

**Q1–Q21 are the brief's question batch, answered by default (D2–D22).**
Steven should review those first — while no code exists, reversing any of them
costs a document edit.

**Q22–Q28 were answered by Steven on 2026-09-02** and became D28–D34. His
answers, verbatim:

| # | Question | Steven's answer | Decision |
|---|---|---|---|
| Q22 | Real role names | "marketing staff, marketing head, operation, business analyst, finance head, it" — then, the same day: "add CFO to role" | **D28** |
| Q23 | Budget / discount-cost field | "no budget promo limit for now" | **D29** |
| Q24 | Existing POS export format | "will discuss integration to third party later" | **D30** |
| Q25 | Site group restricted to one brand | "yes restricted" | **D31** |
| Q26 | Release-email recipients | "fixed lists, maintained via backend" | **D32** |
| Q27 | Audit / approval retention | "no limit" | **D33** |
| Q28 | Brand palette | "choose moderen color template, i prefer #778aab, others is mix and match" | **D34** |

Still open, and none of them blocks the build. Each carries an applied default,
so the documents are complete either way:

**Q29–Q32 were answered by Steven on 2026-09-02** and became D35–D38:

| # | Question | Steven's answer | Decision |
|---|---|---|---|
| Q29 | Who holds `superadmin` | "superadmin = it" | **D35** |
| Q30 | Is Business Analyst in the chain | "business analyst is approver also, after marketing" | **D36** |
| Q31 | Is Operation one role or one per brand | "when create user, it will choose company (1 or more)" | **D37** |
| Q32 | Does `report.export` reach Marketing Staff | "permit it" | **D38** |

**Nothing is open.** One reading was taken rather than asked, and it is cheap
to correct:

| # | Reading taken | Alternative | Cost to change |
|---|---|---|---|
| R1 | "after marketing" = **after the Marketing Head step**, so Business Analyst is step 2 | If it meant "after Marketing Staff creates", the BA is step 1 and the Marketing Head follows | A back-office chain edit, or one line of the seed before M6 |

---

## 4. How to read this set in order

1. `PROMPT.md` — what Steven asked for, verbatim.
2. `01-PRD.md` — the problem and the scope.
3. `02-business-rules.md` — **normative.** Everything else defers to it.
4. `03-data-model.md` — how the rules are stored and constrained.
5. `05-architecture-and-nfr.md` — how the code is arranged.
6. `08-roadmap.md` — the order it gets built in.
7. `PROGRESS.md` — what is actually done, at any moment.
