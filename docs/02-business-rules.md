# 02 — Business rules (NORMATIVE)

**This document wins.** Where it conflicts with any other document on product
logic, this one is correct and the other is stale. Rules carry `BR-x.y` IDs;
code comments and test names reference those IDs so a rule can be traced to the
thing that enforces it.

**Date:** 2026-09-01 · **Status:** written, awaiting Steven's confirmation

---

## BR-1 — Foundations

**BR-1.1 Money is integers.** Every monetary amount is whole Indonesian rupiah
stored as `BIGINT`. Floating point is prohibited in any code path touching
money. Sen is not represented.

**BR-1.2 Rates are basis points**, rounded half-up:
`floor((amount * bps + 5000) / 10000)`. 11% is `1100`.

**BR-1.3 A site group is a set of sites, all in one brand.** `site_group` has
many `site` members through `site_group_member`. When a site is created, a site
group named after it is created automatically containing exactly that site, so
a single-site promotion needs no set-up.
*(D23 — the brief's `site_group(site_id)` shape cannot express a multi-site
group, which is the thing promotions target.)*

**Every member must belong to the group's own company.** A group spanning Maxx
Coffee and Ruuma is **rejected**, not warned. The database enforces it through
a composite foreign key, not an application check. *(D31)*

**BR-1.4 Time.** All timestamps are stored `timestamptz` in UTC. Every business
date — a promotion's start, a target's month, a transaction's business day — is
evaluated in `Asia/Jakarta` and converted explicitly. Server-local time is
never used.

**BR-1.5 Scope.** Every row that belongs to a company carries `company_id`.
Every query for such a row filters on the caller's permitted companies **in the
query**, not after fetching. Uniqueness constraints are per company, never
global.

**BR-1.6 Configurable values are parameters.** Anything the business might
change without a deploy is a `sys_parameters` row: lead-time days, auto-cancel
day, recipient lists, feature toggles, thresholds. A constant in a handler is a
defect.

---

## BR-2 — Targets

**BR-2.1 Grain.** A target is set for one **site**, one **period**, one **sales
type**. Sales type is `normal` or `promo`. *(D9)*

**BR-2.2 Two period kinds.** `YEAR` targets carry a year. `MONTH` targets carry
a year and a month. Both are stored in one table distinguished by period kind.

**BR-2.3 The months need not sum to the year.** A year target of Rp 1.2bn with
twelve month targets summing to Rp 1.35bn is **valid and must not be blocked.**
The system displays the variance; it never enforces agreement.
*This is explicit in the brief and is the rule most likely to be "helpfully"
broken by a well-meaning validation.*

**BR-2.4 A target amount is non-negative.** Zero is meaningful (a site that is
closed that month). Negative is rejected.

**BR-2.5 Who sets targets.** A role holding `target.manage`. No approval chain
on targets in phase 1. *(D10)*

**BR-2.6 Roll-up is arithmetic, never storage.** A group, brand or company
target is the sum of its site targets, computed on read. There is no stored
aggregate to drift.

**BR-2.7 Editing a past target is permitted but audited.** Changing the target
for a month that has closed is sometimes legitimate (a correction) and always
suspicious. It is allowed, it writes an audit row, and the report shows that
the target was amended after the period ended.

---

## BR-3 — Promotion planning

**BR-3.1 Required fields.** A promotion plan has: a name, a **site group**, a
**start date**, an **end date**, a **target sales amount**, a **target receipt
count**, an **order mode**, and a **promo rule** as free text. All are required
to submit; a draft may be incomplete.

**There is no budget or discount-cost field in phase 1, and no budget limit.**
*(D29)* The consequence is stated plainly so nobody discovers it in a meeting:
the promotion report shows **revenue, not margin**, so a promotion that lifted
sales while destroying margin reads as a success. Adding a nullable
`budget_idr` later is one migration; the phase-2 promo P&L is the real answer
(`08-roadmap.md`).

**BR-3.2 The period may span months.** Start and end are picked as a range and
may cross month and year boundaries. End must be on or after start.

**BR-3.3 Lead time.** The earliest permitted start date is **N working days**
after today, where N is `promo.lead_time_working_days` (default **7**).

- Working days exclude Saturdays, Sundays, and any date in the **holiday
  calendar** for the relevant country. *(D22)*
- Counting is *exclusive of today, inclusive of the resulting day*: with N = 7
  and today 1 September 2026 (a Tuesday, no holidays), the earliest start is
  **10 September 2026**.
- The rule is evaluated at **submit**, not at draft-save, so a draft left open
  overnight does not silently become invalid.
- A superadmin may override with a typed reason, which is audited. *(BR-4.8)*

> Weekday arithmetic alone is not sufficient. Around Idul Fitri, Christmas and
> Nyepi it yields a date the lead time never legitimately allowed, and the
> error is invisible because the number still looks like seven.

**BR-3.4 Order mode** is one of `dine_in` or `take_away`. A promotion applies to
exactly one. *(A promotion for both is two plans, which keeps measurement
unambiguous.)*

**BR-3.5 Targets on a promotion are non-negative**, and the receipt-count target
is a whole number.

**BR-3.6 Overlapping promotions are allowed, and warned.** Two promotions whose
date ranges overlap on the same site group — or on groups sharing any site —
are permitted, because a stacked promotion is sometimes intended. Creation
raises a **blocking-acknowledgement warning** listing the overlaps, and the
calendar flags both. *(D13)*

**BR-3.7 A plan belongs to exactly one company.** A site group cannot span
companies at all (BR-1.3), so a plan cannot either. **A cross-brand campaign is
modelled as one plan per brand** — permanently, not provisionally. Each brand
then approves its own, which is what the separate finance and operations
sign-off is for. *(D31)*

---

## BR-4 — Approval

**BR-4.1 The engine is generic.** Approval operates on a **subject type** and a
**subject id**. `promotion_plan` is the first subject type. Nothing in the
engine knows what a promotion is. *(D25)*

**BR-4.2 A chain is an ordered list of steps.** Each step names **one or more
roles** and a satisfaction rule:

- `ANY_OF` — one holder of any listed role approves the step. This is how
  `Role_3 OR Role_4` is expressed.
- `ALL_OF` — every listed role must approve before the step completes.

Default chain, in the group's real role names *(D14, renamed by D28, with
Business Analyst inserted by D36)*:

| Step | Roles | Rule |
|---|---|---|
| 1 | Marketing Head | ANY_OF |
| 2 | Business Analyst | ANY_OF |
| 3 | Finance Head | ANY_OF |
| 4 | Operation | ANY_OF |
| 5 | CFO | ANY_OF |

The creator is **Marketing Staff**. The either/or step from D14 collapsed to a
single role because the group has one Operation function, not a brand head and
an operations head — the `ANY_OF` machinery stays, because a step with two
roles is exactly how it comes back.

> **Five steps and a seven-working-day lead time leave very little slack, and
> that is a tuning problem the parameters already solve.** A plan submitted at
> the earliest permitted start (BR-3.3, 7 working days ≈ 9–11 calendar days
> ahead) is auto-cancelled 5 calendar days before start (BR-4.6), so the whole
> chain has roughly **4 to 6 calendar days — about one step per day**. That is
> feasible and unforgiving. Both numbers are `sys_parameters` rows precisely so
> the first month of real use retunes them rather than a deploy.

**BR-4.3 A chain is versioned, and a plan is bound to the version it started
with.** Changing the configured chain affects **only plans created afterwards**.
A plan in flight keeps its chain, its steps and its rules, even if an
administrator rewrites the chain mid-flight. *This is explicit in the brief.*

**BR-4.4 Approval is sequential.** Step *n+1* opens only when step *n* is
satisfied. An approver at a later step cannot approve early.

**BR-4.4a An approver must hold the step's role *in the subject's company*.**
Holding `operation` for Maxx Coffee does not permit approving a Ruuma plan, and
the approver's inbox lists only the instances whose company they are assigned
to. Roles are not per brand — the **company assignment on the user is** (BR-5.4,
D37) — so this is the rule that makes one `operation` role safe across three
brands. It is enforced in the eligibility query, not checked after loading.
*(D37)*

**BR-4.5 Rejection returns the plan to the creator, with a mandatory reason.**
The plan moves to `REJECTED`, retains its full history, and the creator may
edit and resubmit — which starts the chain again from step 1. *(D15)*

**BR-4.6 Auto-cancel.** A scheduled job runs daily. Any plan not yet fully
approved on the **Mth day before its start date** — M is
`promo.auto_cancel_days_before` (default **5**) — is moved to `CANCELLED`.

- The job writes an audit row naming the job as the actor.
- It notifies the creator and every approver whose step was pending.
- A superadmin may **revive** a plan cancelled this way, with a reason; the
  plan returns to its pending step. *(D11)*
- If the plan's start date is closer than M days at creation time — impossible
  under BR-3.3 unless overridden — the job cancels it on its next run.

**BR-4.7 Approval locks the plan.** Once step 1 has been approved, the plan's
substantive fields — dates, site group, targets, order mode, rule text — are
immutable. An edit creates a **new version** which re-enters the chain at step
1; the previous version is retained and remains readable. *(D12)*

> If an approved plan can be edited, the approval means nothing. This is the
> most important control in the module.

**BR-4.8 Superadmin force-release** completes every outstanding step at once.
It requires a **typed reason**, writes an audit row naming the superadmin, and
marks the plan as force-released so the report can distinguish it from a
plan that went through the chain. *(D27)*

**BR-4.9 An approval decision is final and append-only.** There is no "undo".
A mistake is corrected by rejecting (if the chain is still open) or by a new
version. The `approval_event` table takes no updates and no deletes.

**BR-4.10 One decision per approver per step.** The same user cannot approve the
same step twice, and two approvers acting simultaneously must not both
complete it. Enforced by a row lock inside one transaction plus a unique index,
and proven by a concurrency test.

**BR-4.11 The creator may not approve their own plan**, at any step, even if
they hold the role. A superadmin force-release is the documented escape.

**BR-4.12 On final approval the plan is `RELEASED`** and a notification email is
sent to a **fixed recipient list maintained in the back office** —
`notify.release_recipients`, a `sys_parameters` row with admin CRUD. The list is
**exactly** who is emailed; chain actors are not appended automatically. *(D32)*

> Workflow notifications are a different thing and are unaffected: the creator
> is told when their plan is approved, rejected (BR-4.5) or auto-cancelled
> (BR-4.6), and a pending approver is told there is something in their queue.
> Those go to the people involved. The **release announcement** goes to the
> list.

Because the list is fixed, it goes stale silently — that is the known failure
mode of this decision. Two mitigations: every change to it is audited (BR-8.2),
and `06-domain-operations.md` §4 puts reviewing it in the month-end routine.

---

## BR-5 — Identity and access

**BR-5.1 Deny by default.** A user has no permission until a role grants it.
Every handler declares the permission it requires.

**BR-5.2 Accounts are created by an administrator.** There is no self-service
registration; there is no public surface on which to offer one.

**BR-5.3 TOTP is mandatory for every account.** A user without a confirmed TOTP
enrolment can reach only the enrolment flow. *(D18)* Passwords are argon2id.

**BR-5.4 A user is assigned one or more companies, explicitly.** Creating a
user selects the companies that user works in — **one or more, never none, and
never an implicit "all"**. A user may hold different roles in different
companies. Someone who works across the group is given all three companies by
an administrator ticking three boxes. *(D19, made explicit by D37)*

> **There is no `company_id IS NULL` meaning "every company".** It was the
> original design and it is withdrawn, because an implicit superset **grows
> silently**: add a fourth brand and every group-level account can see its sales
> the moment the row is inserted, with no decision and no audit entry. An
> explicit list grants nobody anything until somebody chooses. Deny by default
> (BR-5.1) has to mean the same thing tomorrow as it does today.

**BR-5.5 Site scoping.** A role may additionally be limited to a set of sites.
Where present, every read is filtered to those sites in the query.

**BR-5.6 Sessions.** Access tokens ~15 minutes; refresh tokens rotate on use,
are stored hashed, and are revocable. Re-presenting a rotated refresh token
revokes the whole family — it is the signature of a stolen token.

**BR-5.7 The role catalogue.** Phase 1 ships these roles — the group's real
ones *(D28)*. The permission matrix is in `12-security.md` §4.

| Code | Label (id-ID) | Label (en) | In the chain |
|---|---|---|:--:|
| `marketing_staff` | Staf Marketing | Marketing Staff | creates |
| `marketing_head` | Kepala Marketing | Marketing Head | step 1 |
| `business_analyst` | Analis Bisnis | Business Analyst | step 2 |
| `finance_head` | Kepala Keuangan | Finance Head | step 3 |
| `operation` | Operasional | Operation | step 4 |
| `cfo` | CFO | CFO | step 5 |
| `it` | IT | IT | — |
| `superadmin` | Superadmin | Superadmin | force-release |

`it` is the administrator role — users, roles, sites, holidays, parameters,
imports. **`superadmin` is held by IT** *(D35)*, which is a segregation-of-duties
trade Steven has made explicitly; see `12-security.md` §4.
Roles are data, not code: adding one is a row and a permission grant, and the
chain that uses it is a back-office edit.

---

## BR-6 — Transaction data

**BR-6.1 Source.** Transactions arrive by nightly CSV import in **our own
column contract**, specified in `06-domain-operations.md` §3.0. There is no POS
export format to match yet; when the third-party integration is settled, it
arrives as a second implementation of the same port and nothing above the port
changes. *(D5, D30)*

**BR-6.2 Grain is one row per receipt.** A row carries: business date, site,
sales type (`normal` / `promo`), promotion id where applicable, **order mode**,
**gross amount**, and the POS receipt identifier. *(D6, D24)*

> The brief's fact table had neither receipt count nor order mode, and a
> promotion is planned with a target for both. Aggregated rows cannot be
> un-summed later, so the grain decision is irreversible once history is
> loaded.

**BR-6.3 Import is idempotent.** A row is identified by
`(site_id, business_date, pos_receipt_no)`. Re-importing a file that has
already been loaded **updates nothing and inserts nothing** and reports zero new
rows. Re-importing a corrected file for the same day replaces that day's rows
for that site within one transaction.

**BR-6.4 Every import run is recorded** — file name, checksum, row counts
(read, inserted, skipped, rejected), the actor, and the outcome. The table is
append-only.

**BR-6.5 A rejected row never silently disappears.** Rows failing validation are
written to a rejection table with the reason and the original line, and the
reconciliation screen shows them.

**BR-6.6 A transaction referencing an unknown site or promotion is rejected**,
not coerced. Reject, never silently repair.

---

## BR-7 — Reporting

**BR-7.1 Every list has a debounced search box.** No exceptions.

**BR-7.2 Every grid exports CSV with a pipe (`|`) delimiter.** It is a real RFC
4180 file with `|` as the separator; a value containing a pipe, a quote or a
newline survives the round trip.

**BR-7.3 Formula injection is neutralised.** Any cell beginning `=`, `+`, `-`,
`@`, tab or CR is prefixed with an apostrophe. A CSV is an executable document
in Excel.

**BR-7.4 An export reflects the screen.** The current filters, search term and
sort are applied to the export. Exporting something other than what is
displayed is worse than no export.

**BR-7.5 The promotion report** shows, per promotion: the plan (targets, dates,
group, order mode), the actual sales and receipt count from `history_txn`
attributed to that promotion, the variance in both absolute and percentage
terms, and whether the plan was force-released.

**BR-7.6 Actuals are attributed by `promo_id`**, not by date range. A
transaction inside a promotion's dates but not tagged with its id is
`normal` sales, and counting it as promo sales would flatter every report.

**BR-7.7 Target versus actual** is available at site, group, brand and company,
for a month or a year, split by sales type.

---

## BR-8 — Audit

**BR-8.1 Append-only.** `audit_log` and `approval_event` take no updates and no
deletes; their migrations say so and a trigger enforces it.

**BR-8.2 What is always audited:** every approval decision; every force-release
and every revive, with the reason; every lead-time override; every change to a
chain, a role, a permission or a parameter; every target edit after its period
closed; every import run.

**BR-8.3 An audited action records** actor, action, subject type and id, before
and after state where meaningful, reason where required, IP address, and the
timestamp in UTC.

**BR-8.4 Secret-flagged parameter values are masked** in the audit trail as
well as in the UI and logs.

**BR-8.5 Nothing is purged.** `audit_log`, `approval_event`, `import_run` and
`import_rejection` are kept **indefinitely**. There is no retention window and
no purge job, and one must not be added without a decision that reverses this.
*(D33)*

> The cost is growth, and it is bounded and known: these tables carry events,
> not transactions. `history_txn` is the table that grows with trading volume,
> and its size is addressed by partitioning (`03-data-model.md` §6), not by
> deletion.

---

## Rule-to-enforcement map

Filled in as the build lands; a rule with no enforcement column is not done.

| Rule | Enforced by | Proven by |
|---|---|---|
| BR-1.1 | `internal/domain/money`, `BIGINT` columns | `money_test.go` |
| BR-1.3 | composite FK on `site_group_member` | `schema_test.go::TestCrossBrandGroupMemberRefused` |
| BR-2.3 | *no* validation — deliberately absent | `target_test.go::TestMonthsNeedNotSumToYear` |
| BR-3.3 | `internal/domain/calendar` + holiday table | `calendar_test.go`, incl. Idul Fitri |
| BR-3.6 | overlap query + acknowledgement flag | `promo_test.go::TestOverlapWarns` |
| BR-4.3 | `chain_version_id` on the plan | `approval_test.go::TestChainChangeDoesNotAffectInFlight` |
| BR-4.7 | status guard + version table | `approval_test.go::TestApprovedPlanIsImmutable` |
| BR-4.10 | `FOR UPDATE` + unique index | `approval_concurrency_test.go` |
| BR-4.11 | creator check in the domain | `approval_test.go::TestCreatorCannotApprove` |
| BR-4.12 | `notify.release_recipients` parameter | `notify_test.go::TestReleaseGoesToTheListOnly` |
| BR-6.3 | unique `(site,date,receipt_no)` | `importer_test.go::TestReimportIsIdempotent` |
| BR-7.3 | `platform/csvexport` | `csv_test.go::TestFormulaInjection` |
| BR-8.1 | `refuse_mutation()` trigger | `schema_test.go::TestAppendOnly` |
| BR-8.5 | no purge job exists — deliberately absent | reviewed; `grep` for a delete on `audit_log` in CI |
