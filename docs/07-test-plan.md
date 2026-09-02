# 07 — Test plan

**Date:** 2026-09-01 · A rule with no test is not enforced. Test names
reference `BR-x.y` so a rule can be traced to the thing that proves it.

---

## 1. Strategy

| Layer | What | Where |
|---|---|---|
| Domain unit | pure logic, exhaustive, no I/O | `internal/domain/*/…_test.go` |
| Integration | real PostgreSQL, constraints, concurrency | `test/*_test.go` against `marketing_calendar_test` |
| HTTP | the real router, real DB, real auth | `test/api_*_test.go` |
| Visual | screenshots + **computed styles** on the running app | `tools/shot` |
| Security | authz matrix, IDOR, injection | `test/security_*_test.go` |

**"Done" means run.** A test that was written but not executed is not a test.

## 2. The tests that must exist before the module ships

### 2.1 Money (BR-1.1, BR-1.2)

- Every amount is `int64`; no float appears in any money path.
- Half-up basis points at 0, 1, 1100, 9999 bps.
- Overflow refused rather than wrapped.
- A guard test asserting no `float` type appears in any `*_idr` field —
  **derived from the AST, not from a text grep of the source**, because a
  comment mentioning `float` would otherwise satisfy a naive check.

### 2.2 Working days and lead time (BR-3.3)

The matrix, and it is the one most likely to be wrong:

- N = 7 from Tue 1 Sep 2026, no holidays → **10 Sep 2026** (the brief's example)
- a run crossing a weekend
- a run crossing a **single** public holiday
- a run crossing the **multi-day Idul Fitri** block
- a holiday falling on a Saturday (must not double-count)
- N = 0 and N = 1 boundaries
- N configurable: changing the parameter changes the answer
- evaluated in `Asia/Jakarta` when the server clock is UTC and it is 23:30 UTC
  — the business date is already tomorrow in Jakarta

### 2.3 Targets (BR-2)

- **The twelve month targets need not sum to the year target.** A cart of
  months summing over and under both save successfully. *This test exists to
  stop someone "fixing" it into a validation.*
- The variance figure is correct in both directions.
- Zero is accepted; negative is rejected.
- Roll-up to group, brand and company is the sum of the sites.
- Editing a closed period is allowed and writes an audit row (BR-2.7).

### 2.4 Approval (BR-4) — the heart of it

- The full chain completes step by step and releases.
- **The default chain is five steps** (BR-4.2): Marketing Head → Business
  Analyst → Finance Head → Operation → CFO. A plan is not `RELEASED` until all
  five are satisfied, and step 5 cannot be reached early.
- **An approver holding the step's role in the *wrong company* is refused**
  (BR-4.4a, D37): an `operation` user assigned only to Maxx Coffee cannot
  approve a Ruuma plan, and the instance never appears in their inbox. Tested
  as an authorisation case, not only as a filter — the refusal is asserted on
  the POST, not just the absence from the list.
- **`ANY_OF` with two roles**: either role satisfies the step; the other cannot
  then also decide it.
- `ALL_OF`: every role must act.
- A later step cannot be approved before an earlier one (`STEP_NOT_OPEN`).
- **The creator cannot approve their own plan** at any step (BR-4.11).
- **The same actor cannot decide the same step twice** (BR-4.10).
- **Concurrency:** two eligible approvers acting simultaneously on one
  `ANY_OF` step — exactly one succeeds, and the instance advances exactly one
  step. Twenty goroutines, and the assertion is on the final state, not on
  timing.
- Rejection returns to the creator with the reason retained, and the history
  survives (BR-4.5).
- **A chain edited mid-flight does not disturb an open instance** (BR-4.3) —
  the instance keeps its bound version, and a new plan gets the new one.
- **An approved plan is immutable** (BR-4.7): a `PUT` returns `PLAN_LOCKED`,
  and a new version re-enters at step 1.
- Force-release without a reason is refused; with one, it completes every step
  and sets `force_released` (BR-4.8).
- `approval_event` refuses `UPDATE` and `DELETE` (BR-4.9).
- **The engine has no import of the promotion package**, and drives a
  fabricated subject type in a unit test (architecture §2).

### 2.5 Auto-cancel (BR-4.6)

- A plan incomplete on the Mth day before start is cancelled by the job.
- A plan fully approved is untouched.
- The job is idempotent — running it twice cancels once.
- Cancellation notifies the creator and the pending approver.
- A superadmin revive returns it to the pending step, and both events remain.

### 2.6 Overlap (BR-3.6)

- Two plans on the same group with overlapping dates → `OVERLAP_UNACKNOWLEDGED`.
- Groups **sharing one site** also overlap — the check is on site membership,
  not group identity. *This is the case a naive implementation misses.*
- Retrying with the acknowledgement succeeds and records it.
- Adjacent, non-overlapping ranges do not warn (end = start − 1 day).

### 2.7 Single-brand site groups (BR-1.3, D31)

- Adding a Ruuma site to a Maxx Coffee group is refused **by the database**,
  not only by the handler — the test writes it with raw SQL, bypassing the
  application, and asserts the constraint fires. A rule that only the
  application enforces is a rule the next writer can skip.
- The API returns `422 CROSS_BRAND_MEMBER`, not a leaked constraint name.
- A cross-brand campaign is two plans, and both approve independently.

### 2.8 Import (BR-6)

- A clean file loads and the counts match.
- **Re-importing the identical file inserts zero rows and is not an error.**
- Re-importing a corrected file for one site-day replaces that day.
- An unknown site, an unknown promo, a negative amount and an inconsistent
  `sales_type`/`promo_id` pair are each rejected with the right reason and the
  original line.
- A partial failure does not leave half a file loaded.
- `import_run` refuses `UPDATE` and `DELETE`.
- **The CSV contract of `06` §3.0 is tested as a contract** (D30): a file whose
  header columns are reordered still loads correctly by name; a file with an
  unknown column is rejected whole; a `#TOTAL` trailer that disagrees with the
  rows fails the file. The truncated-upload case is the one that matters —
  without the trailer it is indistinguishable from a quiet trading day.
- A renamed but byte-identical file still imports zero rows: identity is the
  checksum, not the filename.

### 2.9 Reporting (BR-7)

- CSV is pipe-delimited, RFC 4180, with a UTF-8 BOM.
- A value containing `|`, `"` and a newline survives the round trip.
- Formula injection: `=`, `+`, `-`, `@`, tab and CR are each prefixed.
- **The export matches the screen** — same filters, same search, same sort,
  same row count.
- Promo actuals are attributed by `promo_id`, never by date window (BR-7.6): a
  `normal` transaction inside a promo's dates does not appear in its actuals.

### 2.10 Security

Per `12-security.md`, and at minimum:

- The permission matrix of `12-security.md` §4, for all eight roles of BR-5.7,
  asserting for every role both what it **can** and what it **cannot** reach.
  Specifically: **IT** is refused `report.export` while every business role
  including Marketing Staff holds it (D38); only `superadmin` reaches
  `force_release`; only CFO, IT and superadmin reach `audit.view`.
- **`POST /users` with an empty company list is refused** `422`, and a user
  created with one company cannot read another's rows (BR-5.4, D37). The
  absence of a wildcard is asserted directly: no `user_role` row may have a
  NULL `company_id`, checked against the schema rather than the handler.
- IDOR: every read of another company's or another site's row returns `404`.
- A user with no TOTP enrolment can reach only the enrolment flow.
- Login does not distinguish an unknown email from a wrong password — the
  responses are byte-identical apart from the trace id.
- Refresh rotation, and reuse revoking the family.
- No endpoint leaks a driver error, a table name or a column name.

### 2.11 Visual

`tools/shot` at 360px and 1440px, on the running app:

- Contrast measured against the **actually painted** background, walking the
  ancestor chain.
- An element painting a gradient with no `background-color` is reported, not
  skipped.
- Screen-reader-only text is excluded — it is never painted.
- No horizontal overflow; touch targets ≥ 44px; both fonts loaded.

## 3. Data

- The integration suite runs against `marketing_calendar_test` and
  **truncates to a known state in `TestMain`**, driven off the catalogue so a
  table added later is cleaned without anyone remembering.
- The suite must pass under `go test -shuffle=on`, repeatedly. Order dependence
  is a defect in the suite, not a quirk.
- Fixtures use a **fixed clock**. Anything date-sensitive that reads the wall
  clock is untestable around midnight and will fail exactly once a year.

## 4. Gates before a ✅ in PROGRESS.md

1. `go vet ./...` clean.
2. `go test ./...` — every package, output read.
3. `go test ./test/... -shuffle=on` — several consecutive runs.
4. The concurrency tests actually contended (they log the winner count).
5. `scripts/contrast.py` — every pairing matches the sheet.
6. `tools/shot` — clean against the **running** service.
7. Docs updated in the same commit.

## 5. What is not tested in phase 1, and why

- Load beyond the seeded 24 months. The N2/N3 budgets are asserted against
  seeded data; a real load test waits for real data volumes.
- Email delivery to a real MTA. Phase 1 asserts against mailpit and records
  that in `RUN-WHEN-BACK.md`.
- Browser matrix beyond Chromium. Stated, not assumed.
