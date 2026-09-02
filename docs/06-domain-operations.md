# 06 — Domain operations and runbooks

**Date:** 2026-09-01 · Operational logic that is neither a business rule nor a
deployment step. Every path here uses `vi` and absolute paths.

---

## 1. Working-day arithmetic

The single most error-prone calculation in the system (BR-3.3).

```
earliest_start(today, N):
    d = today                       # Asia/Jakarta
    counted = 0
    while counted < N:
        d = d + 1 day
        if weekday(d) not in (Sat, Sun) and d not in holidays:
            counted = counted + 1
    return d
```

Worked, with N = 7 and no holidays: today Tue 1 Sep 2026 → counts Wed 2, Thu 3,
Fri 4, Mon 7, Tue 8, Wed 9, Thu 10 → **earliest start 10 September 2026**,
which is the example in the brief.

**The holiday table is not optional.** Idul Fitri moves each year and carries a
multi-day national holiday; in 2027 it falls in March. Computed on weekdays
alone the lead time silently shortens by up to a week, and the number still
looks like seven.

Maintenance: an administrator loads the year's holidays each December. The
seed ships the current and next year (D22). `RUN-WHEN-BACK.md` carries the
reminder.

## 2. Approval operations

### 2.1 A plan is stuck

1. `/approvals/{id}` shows the current step, the eligible roles and who has
   already acted.
2. If nobody holds the role, an administrator grants it — the queue is
   role-based, so the plan appears immediately, with no re-submission.
3. If the start date is close, check the auto-cancel date: it is
   `start_date − promo.auto_cancel_days_before`.
4. Last resort: a superadmin force-releases, with a reason (BR-4.8). The plan
   is flagged `force_released` and the promotion report shows it, which is the
   point.

### 2.2 A plan was auto-cancelled and should not have been

A superadmin revives it (BR-4.6) with a reason. It returns to the step it was
pending at. The cancellation event stays in the history — it is append-only,
and the revival is a second event, not an erasure.

### 2.3 Changing the approval chain

Editing a chain creates a **new version** (BR-4.3). Plans in flight keep the
version they were bound to. The response reports how many instances are
unaffected; if that number surprises the administrator, it is because plans are
open, and that is the right moment to notice.

## 3. Import operations

### 3.0 The transaction CSV contract

There is **no POS export format to match yet** (D30 / Q24). This is our
contract; when the third-party integration is agreed, it becomes a second
implementation of the same importer port and nothing above the port moves.

Pipe-delimited, the same convention as every export the product produces
(BR-7.2). UTF-8, LF line endings, RFC 4180 quoting with `|` as the separator.

```
site_code|business_date|pos_receipt_no|sales_type|promo_code|order_mode|gross_amount_idr
MXX-001|2026-09-01|R-000198231|promo|PRM-7QK2|dine_in|185000
MXX-001|2026-09-01|R-000198232|normal||take_away|42000
#TOTAL|2|227000
```

| Column | Type | Rule |
|---|---|---|
| `site_code` | text | must exist in master data, else `UNKNOWN_SITE` |
| `business_date` | `YYYY-MM-DD` | the trading day in **`Asia/Jakarta`**, never UTC |
| `pos_receipt_no` | text | one row per receipt (BR-6.2); the idempotency key with site and date |
| `sales_type` | enum | `normal` or `promo`, nothing else |
| `promo_code` | text | required when `promo`, **empty** when `normal` (BR-6.6) |
| `order_mode` | enum | `dine_in` or `take_away` |
| `gross_amount_idr` | integer | whole rupiah. **No decimal point, no thousands separator, no currency symbol** |

**The header row is required** and is matched by name, not by position — a POS
that reorders its columns must not silently shift every amount into the wrong
field. An unknown column name is a rejected file, not an ignored column.

**The `#TOTAL` trailer is required**: row count and the sum of
`gross_amount_idr`. It is what §3.4 reconciles against, and it is the only
thing that catches a truncated upload — which otherwise arrives looking exactly
like a quiet day of trading.

Filename convention: `txn_YYYYMMDD_NN.csv` in `import.drop_path`. A file is
identified for idempotency by its **checksum**, not its name, so renaming a
file does not let it in twice (BR-6.3, BR-6.4).

### 3.1 The nightly run

```bash
cd /home/dev/projects/marketing_calendar
/home/dev/projects/marketing_calendar/bin/mc job import-transactions
```

Reads the drop directory in `import.drop_path`, loads each unseen file, and
writes an `import_run` row per file.

### 3.2 A day was imported wrong

Re-import the corrected file for that site and day. The importer replaces that
site-day's rows inside one transaction (BR-6.3). Re-importing an **identical**
file reports zero inserted and is not an error.

### 3.3 Rows were rejected

`/imports/{id}/rejections` lists each with its reason and original line
(BR-6.5). The common causes:

| Reason | Meaning | Fix |
|---|---|---|
| `UNKNOWN_SITE` | site code not in master data | create the site, re-import |
| `UNKNOWN_PROMO` | `promo_id` not a released plan | check the POS mapping |
| `PROMO_INCONSISTENT` | `sales_type=promo` with no promo id, or the reverse | POS export defect |
| `DUPLICATE_RECEIPT` | already loaded | none — informational |
| `NEGATIVE_AMOUNT` | below zero | POS export defect; never coerced |

Nothing is silently repaired (BR-6.6).

### 3.4 Reconciliation

The reconciliation screen compares, per site and day: rows in the file, rows
loaded, rows rejected, and the gross total. A mismatch between the file's own
trailer total and the loaded total is flagged. This is the check that catches a
truncated upload, which otherwise looks like a quiet day of trading.

## 4. Month-end

1. Confirm every day of the month has an `import_run` — a missing day is the
   usual cause of a report that looks wrong.
2. Run target-versus-actual per brand.
3. Export the promotion report to CSV (pipe-delimited) for the marketing review.
4. Note any plan flagged `force_released`; those are the ones a reviewer will
   ask about.
5. **Read `notify.release_recipients` aloud.** A fixed list is the decision
   (D32) and going stale is its known failure mode — somebody leaves, and the
   release email keeps being delivered to a mailbox nobody opens. One minute a
   month is the whole mitigation.

## 4a. Retention — there is none

`audit_log`, `approval_event`, `import_run` and `import_rejection` are kept
**indefinitely** (BR-8.5, D33). No purge job exists, and writing one is a
decision to reverse, not a tidy-up.

If growth is ever raised: these are event tables, not transaction tables. The
table that grows with trading volume is `history_txn`, and the answer there is
partitioning (`03-data-model.md` §6). Measure before proposing a delete —
"the audit log is large" has never once been checked against the actual figure
in a conversation that proposed truncating it.

## 5. Backup and restore

Covered operationally in `09-deployment.md`. The check that belongs here: a
restore is not proven by a successful `pg_restore`. It is proven by running
`migrate:status` against the restored database and by opening the promotion
report for a month that had data.

## 6. Parameters worth knowing

| Key | Default | Effect |
|---|---|---|
| `promo.lead_time_working_days` | `7` | earliest promo start (BR-3.3) |
| `promo.auto_cancel_days_before` | `5` | when an incomplete chain is cancelled (BR-4.6) |
| `promo.overlap_warning_enabled` | `true` | the acknowledgement gate (BR-3.6) |
| `notify.release_recipients` | list | **exactly** who is emailed on release — chain actors are not appended (BR-4.12, D32) |
| `import.drop_path` | `/srv/mc/import` | where the nightly job looks |
| `report.max_export_rows` | `100000` | export guard |

Changing any of these takes effect on the next evaluation. None requires a
deploy — that is the point of `sys_parameters` (BR-1.6).
