# 01 — Product requirements

**Project:** marketing_calendar — internal superapp
**Brands:** Maxx Coffee (coffee shops) · Ruuma (restaurants) · Sunshine (catering)
**Status:** phase 1 specified, awaiting Steven's confirmation
**Date:** 2026-09-01

---

## 1. The problem

The group runs three brands across a few hundred sites. Promotions are planned
in spreadsheets, approved over chat, and measured — when they are measured at
all — by exporting POS data and pivoting it by hand.

Four things go wrong, repeatedly:

1. **A promotion goes live that nobody senior approved.** Approval happens in a
   chat thread, so there is no record of who agreed to what, and no way to stop
   an unapproved promotion reaching a store.
2. **A promotion goes live with no lead time.** Stores need notice to brief
   staff, print materials and adjust stock. A promotion announced on Friday for
   Monday is a promotion that runs badly.
3. **Nobody can say whether a promotion worked.** The target was in one
   spreadsheet, the actual sales in a POS export, and joining them is a manual
   job that mostly does not happen.
4. **Targets and actuals live apart.** The year target, the month targets and
   what actually sold are three separate artefacts, so variance is discovered
   late.

## 2. What phase 1 delivers

One internal application where a promotion is **planned, approved, released and
measured** in one place, against targets held in the same system.

Explicitly **not** in phase 1: anything customer-facing, POS replacement,
inventory, purchasing, HR. See `08-roadmap.md`.

## 3. Personas

The group's real roles (**BR-5.7**, D28).

| Persona | Role | What they need |
|---|---|---|
| **Marketing Staff** | Creates promotions | A fast create form, a calendar of what is already planned, and to know where a submission is stuck |
| **Marketing Head** | Step 1 approver | A queue of what is waiting for them, with enough detail to decide without opening a spreadsheet |
| **Finance Head** | Step 2 approver | The target impact, and whether this promotion is affordable against the month |
| **Operation** | Step 3 approver | Whether the sites can actually run it |
| **CFO** | Final approver | A one-screen summary; they will approve from a phone |
| **Business Analyst** | Reads and exports | Target versus actual across brands, and a CSV of whatever is on screen — no approval rights |
| **IT** | Configures | Users, roles, chains, parameters, holiday calendar, recipient lists, imports |
| **Superadmin** | Breaks glass | Force-release, revive a cancelled plan — both audited |

## 4. Scope — phase 1

### 4.1 Platform

| Capability | Requirement |
|---|---|
| Identity | Staff accounts created by an administrator. Argon2id passwords. **Mandatory TOTP.** Rotating refresh tokens, revocable. |
| Authorisation | Deny by default. Every handler declares a permission. Every query scoped by company and site. |
| Master data | Company, site, site group with many-to-many membership, holiday calendar. Full CRUD with search. |
| Approval engine | **Generic.** Configurable ordered steps; a step may be satisfied by any one of several roles. Versioned so a chain change does not disturb plans in flight. |
| Notifications | Email in phase 1, behind a port that WhatsApp will also implement. The release announcement goes to a **fixed list** maintained in the back office (D32). |
| Reporting | Query → grid → pipe-delimited CSV, one implementation reused by every report. |
| Audit | Append-only. Who, what, when, from where, and why for anything requiring a reason. |
| Parameters | `sys_parameters` with admin CRUD. Lead time, auto-cancel day, recipient lists, feature toggles. |
| Importer | Idempotent nightly transaction import with a reconciliation view. |

### 4.2 Marketing calendar

| Capability | Requirement |
|---|---|
| Year target | Set per site, per sales type, for a year |
| Month target | Set per site, per month, per sales type. **The twelve months need not sum to the year.** Show variance; never block. |
| Promotion planning | Date range spanning months, name, site group (single-brand, D31), target sales, target receipts, order mode, free-text rule. **No budget field** (D29) |
| Lead time | Earliest start is **N working days** ahead (default 7, configurable), counted against the Indonesian holiday calendar |
| Auto-cancel | On the **Mth day** before start (default 5), cancel any plan whose approval is incomplete |
| Approval | Configurable chain, either/or steps, superadmin force-release with a reason |
| Release | On final approval, send an email to the fixed maintained recipient list |
| Calendar view | A month grid of what is planned, across brands |
| Promotion report | Planned versus actual, by site, group, brand and order mode, exportable |

## 5. Requirements

### 5.1 Functional

- **F1** A promotion cannot start earlier than the configured lead time.
- **F2** A promotion is not visible as *released* until the chain completes.
- **F3** An approved promotion cannot be silently edited.
- **F4** Every approval decision is attributable and permanent.
- **F5** Targets and actuals appear on one screen with the variance computed.
- **F6** Every list has a search box; every grid exports pipe-delimited CSV.
- **F7** Re-importing a day's transactions does not double the sales.
- **F8** A superadmin can force-release and can revive an auto-cancelled plan;
  both require a reason and are audited.

### 5.2 Non-functional

- **N1** Not public facing. Internal network, IP allowlist, TLS.
- **N2** A list screen responds in under 500 ms at 200 sites and 24 months of
  history.
- **N3** The promotion report for one month across all brands renders in under
  3 seconds.
- **N4** WCAG AA, contrast measured not eyeballed, mobile-first at 360px.
- **N5** Indonesian default, English second, message catalogues only.
- **N6** Storage in UTC, business dates in `Asia/Jakarta`.
- **N7** OWASP ASVS v4 Level 2; every control mapped to the test that proves it.

## 6. Success metrics

| Metric | Baseline | Target |
|---|---|---|
| Promotions released without a complete approval chain | unknown, believed non-zero | **0** |
| Promotions starting inside the lead time | frequent | **0** without an audited override |
| Time from plan creation to release | days, untracked | measured, and visible per step |
| Promotions with a measured result | few | **100%** |
| Manual spreadsheet steps to produce the promo report | several | **0** |

## 7. Out of scope for phase 1

Customer-facing anything · POS replacement · inventory, purchasing, recipes ·
payroll and rostering · loyalty · multi-currency · public API for third
parties · mobile native apps (the web app is mobile-first and that is the
phase-1 answer).

## 8. Assumptions, and what breaks if they are wrong

| Assumption | If wrong |
|---|---|
| Transactions arrive nightly as CSV (D5) | The importer's adapter changes; the port and everything above it do not |
| Per-receipt grain is available from the POS (D6) | Receipt-count targets cannot be measured; the target field becomes advisory |
| Up to 200 sites (D4) | Index and pagination tuning, not a redesign |
| Rupiah only (D7) | A currency column and a rate table; every money path already integer-safe |
| Approval roles as in D14 | A configuration change, not a code change — which is the point of the generic engine |
