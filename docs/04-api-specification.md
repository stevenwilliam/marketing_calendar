# 04 — API specification

**Date:** 2026-09-01 · Base path `/api/v1` · JSON in, JSON out.
Implements `02-business-rules.md`; where they differ, `02` wins.

---

## 1. Conventions

- **Auth:** `Authorization: Bearer <access token>`, ~15 minute lifetime.
  Refresh tokens rotate and live in an `HttpOnly` cookie scoped to
  `/api/v1/auth`.
- **Authorisation:** every endpoint below names the permission it requires.
  Absence of that permission is `403`, never `404`-as-a-hint and never a
  partial result.
- **Scoping:** every list and every read is filtered by the caller's permitted
  companies and sites **in the query** (BR-1.5). Asking for another company's
  id returns `404`, not `403` — the existence of the row is not disclosed.
- **Pagination:** `?page=1&per_page=50`, `per_page` capped at 200. Responses
  carry `{"data": [...], "page": {...}}`.
- **Search:** every list endpoint accepts `?q=` (BR-7.1).
- **Sorting:** `?sort=field` / `?sort=-field`, allow-listed per endpoint.
- **Idempotency:** every unsafe endpoint accepts `Idempotency-Key`. A repeat
  with the same key returns the first result rather than acting twice.
- **Money** is always an integer field named `*_idr`. Never a string, never a
  float.
- **Dates** are `YYYY-MM-DD` business dates in `Asia/Jakarta`. Timestamps are
  RFC 3339 UTC.

## 2. Error model

One shape, everywhere:

```json
{ "error": {
    "code": "VALIDATION_FAILED",
    "message": "Tanggal mulai promo terlalu awal.",
    "fields": [ { "field": "start_date", "code": "LEAD_TIME",
                  "message": "Paling cepat 10 September 2026." } ],
    "trace_id": "0f6d…" } }
```

`code` is stable and machine-readable; `message` is localised and may be
reworded. A driver error is never serialised — it is logged with the
`trace_id`.

| Code | HTTP | Meaning |
|---|---|---|
| `VALIDATION_FAILED` | 400 | field-level problems in `fields` |
| `UNAUTHENTICATED` | 401 | absent, malformed or expired token |
| `TOTP_REQUIRED` | 401 | password accepted, second factor outstanding (BR-5.3) |
| `FORBIDDEN` | 403 | authenticated, lacks the permission |
| `NOT_FOUND` | 404 | absent, or outside the caller's scope |
| `CONFLICT` | 409 | state conflict, see below |
| `LEAD_TIME_VIOLATION` | 409 | start date inside the lead time (BR-3.3) |
| `OVERLAP_UNACKNOWLEDGED` | 409 | overlapping promo, needs acknowledgement (BR-3.6) |
| `PLAN_LOCKED` | 409 | edit attempted on an approved plan (BR-4.7) |
| `STEP_NOT_OPEN` | 409 | approving a step that is not current (BR-4.4) |
| `ALREADY_DECIDED` | 409 | this actor already decided this step (BR-4.10) |
| `SELF_APPROVAL` | 409 | creator approving own plan (BR-4.11) |
| `REASON_REQUIRED` | 400 | reject / force-release / revive without a reason |
| `RATE_LIMITED` | 429 | with `Retry-After` |
| `INTERNAL` | 500 | with a `trace_id` and nothing else |

## 3. Auth

| Method | Path | Permission | Notes |
|---|---|---|---|
| POST | `/auth/login` | — | email + password. Returns `TOTP_REQUIRED` and a short-lived challenge; never a session (BR-5.3) |
| POST | `/auth/totp` | — | challenge + 6-digit code → access token + refresh cookie |
| POST | `/auth/totp/enrol` | authenticated | returns the provisioning URI; first login must complete this |
| POST | `/auth/refresh` | — | rotates; re-presenting a rotated token revokes the family (BR-5.6) |
| POST | `/auth/logout` | authenticated | revokes the presented refresh token |
| GET | `/auth/me` | authenticated | identity, roles, permissions, company scope |

Login, TOTP and refresh are rate limited per IP.

## 3a. Users and roles

| Method | Path | Permission |
|---|---|---|
| GET | `/users` | `user.manage` — `?q=` search, filter by company and role |
| POST | `/users` | `user.manage` |
| GET/PUT | `/users/{id}` | `user.manage` |
| POST/DELETE | `/users/{id}/roles` | `user.manage` — grant or revoke a role **in a named company** |
| POST | `/users/{id}/reset-totp` | `user.manage` — forces re-enrolment; audited |
| GET | `/roles`, `/permissions` | `user.manage` |

- **`POST /users` requires at least one `{role, company}` grant.** An empty
  `companies` array is `422 VALIDATION` naming the field, not a user who can log
  in and see nothing by accident (BR-5.4, D37).
- A grant names its company explicitly. There is no "all companies" value; a
  user who works across the group gets three grants and the audit log has three
  rows saying so.
- Revoking the last grant is permitted — it is how access is removed — but
  returns the count so the caller sees what they did.
- Every grant and revoke writes an audit row (BR-8.2).

## 4. Master data

| Method | Path | Permission |
|---|---|---|
| GET/POST | `/companies`, `/companies/{id}` | `company.view` / `company.manage` |
| GET/POST | `/sites`, `/sites/{id}` | `site.view` / `site.manage` |
| GET/POST | `/site-groups`, `/site-groups/{id}` | `sitegroup.view` / `sitegroup.manage` |
| POST/DELETE | `/site-groups/{id}/members` | `sitegroup.manage` |
| GET/POST | `/holidays`, `/holidays/{id}` | `holiday.view` / `holiday.manage` |
| GET/PUT | `/parameters`, `/parameters/{key}` | `settings.view` / `settings.manage` |

- Creating a site creates its system group in the same transaction (BR-1.3).
- A system group refuses membership edits and deletion: `409 CONFLICT`.
- `POST /site-groups/{id}/members` with a site from another company returns
  `422 CROSS_BRAND_MEMBER` (BR-1.3, D31). The database would refuse it anyway;
  the handler names it so the user gets a sentence instead of a constraint.
- Secret-flagged parameters return `"••••••"` unless the caller holds
  `settings.manage`, and are masked in logs regardless.

## 5. Targets

| Method | Path | Permission |
|---|---|---|
| GET | `/targets` | `target.view` |
| PUT | `/targets` | `target.manage` |
| GET | `/targets/summary` | `target.view` |

`GET /targets?year=2026&site_id=…&period_kind=MONTH&sales_type=normal`

`PUT /targets` upserts a batch — a year's twelve months arrive in one call.

`GET /targets/summary?year=2026&group_by=site|group|brand|company` returns the
year target, the sum of the month targets, and **the variance between them**,
because BR-2.3 makes disagreement legal and the number is the point.

```json
{ "year_target_idr": 1200000000,
  "months_sum_idr":  1350000000,
  "variance_idr":     150000000,
  "variance_pct":          12.5 }
```

## 6. Promotions

| Method | Path | Permission | Notes |
|---|---|---|---|
| GET | `/promotions` | `promo.view` | filters: status, company, group, date window, order mode; `?q=` |
| POST | `/promotions` | `promo.create` | creates a DRAFT |
| GET | `/promotions/{id}` | `promo.view` | current version plus version history |
| PUT | `/promotions/{id}` | `promo.create` | edits the DRAFT; `409 PLAN_LOCKED` once approval has begun (BR-4.7) |
| POST | `/promotions/{id}/submit` | `promo.create` | validates lead time and overlap, opens the approval instance |
| POST | `/promotions/{id}/versions` | `promo.create` | new version of a locked or rejected plan; re-enters the chain at step 1 |
| GET | `/promotions/calendar` | `promo.view` | month grid across brands |
| POST | `/promotions/{id}/cancel` | `promo.manage` | manual cancel, reason required |
| POST | `/promotions/{id}/revive` | `superadmin` | revives an auto-cancelled plan, reason required (BR-4.6) |

**Submit** is where the rules bite:

- Start date inside the lead time → `409 LEAD_TIME_VIOLATION`, with the
  earliest permitted date in `fields`. A superadmin may retry with
  `{"lead_time_override_reason": "…"}` (BR-3.3, audited).
- An overlap → `409 OVERLAP_UNACKNOWLEDGED` listing the overlapping plans.
  Retrying with `{"overlap_acknowledged": true}` proceeds (BR-3.6).

## 7. Approval

| Method | Path | Permission |
|---|---|---|
| GET | `/approvals/inbox` | authenticated |
| GET | `/approvals/{instance_id}` | `promo.view` |
| POST | `/approvals/{instance_id}/approve` | step role **in the subject's company** (BR-4.4a) |
| POST | `/approvals/{instance_id}/reject` | step role in the subject's company, reason required |
| POST | `/approvals/{instance_id}/force-release` | `superadmin`, reason required |
| GET/POST | `/approval-chains` | `settings.manage` |
| POST | `/approval-chains/{id}/versions` | `settings.manage` |

- `/approvals/inbox` is the queue: every instance whose **current** step names a
  role the caller holds, in the caller's companies.
- Approve and reject take the row lock and are safe under concurrency
  (BR-4.10). A second decision by the same actor on the same step is
  `409 ALREADY_DECIDED`.
- The creator approving their own plan is `409 SELF_APPROVAL` (BR-4.11).
- Editing a chain **creates a version**; instances in flight keep the version
  they were bound to (BR-4.3). The response says how many instances are
  unaffected, so the administrator can see that.

## 8. Transactions and import

| Method | Path | Permission |
|---|---|---|
| POST | `/imports` | `import.run` |
| GET | `/imports` | `import.view` |
| GET | `/imports/{id}` | `import.view` |
| GET | `/imports/{id}/rejections` | `import.view` |
| GET | `/transactions` | `txn.view` |

`POST /imports` takes a multipart CSV. The response reports rows read,
inserted, skipped-as-duplicate and rejected (BR-6.4). Re-posting an identical
file reports **zero inserted** and is not an error (BR-6.3).

## 9. Reports

| Method | Path | Permission |
|---|---|---|
| GET | `/reports/promotion` | `report.view` |
| GET | `/reports/target-vs-actual` | `report.view` |
| GET | `/reports/{name}.csv` | `report.export` |

Every report endpoint accepts the same filters as its screen, and the `.csv`
form returns the same rows the screen is showing (BR-7.4):

```
Content-Type: text/csv; charset=utf-8
Content-Disposition: attachment; filename="promotion-report-2026-09.csv"
```

Pipe-delimited, RFC 4180 quoted, formula-guarded, UTF-8 BOM so Excel on Windows
reads Indonesian names correctly (BR-7.2, BR-7.3).

## 10. Operational

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/healthz` | none | liveness; returns the build commit |
| GET | `/readyz` | none | pings PostgreSQL; `503` names the failing dependency |
| GET | `/metrics` | internal only | Prometheus |

`/metrics` is not exposed through nginx to the user network.
