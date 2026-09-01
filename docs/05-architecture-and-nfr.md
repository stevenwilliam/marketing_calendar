# 05 — Architecture and non-functional requirements

**Date:** 2026-09-01

---

## 1. Shape

Hexagonal, dependencies inward only: `adapter → app → domain`, `platform`
available to all. The domain imports no framework, no driver, no `net/http` and
no SQL.

```
cmd/api/main.go              serve · migrate · migrate:status · seed · job
internal/
  domain/
    money/                   IDR as int64, half-up basis points
    calendar/                working days, lead time, holidays      (BR-3.3)
    approval/                the chain state machine                (BR-4)
    promo/                   promotion lifecycle + overlap          (BR-3)
    target/                  target arithmetic and variance         (BR-2)
  app/
    auth/  masterdata/  promotion/  approvalflow/  importer/  reporting/
  adapter/
    http/                    handlers, middleware, request mapping
    postgres/                repositories; raw SQL on money paths
    storage/                 MinIO
    notify/                  email now, WhatsApp behind the same port
  platform/
    config logging metrics apierror id security ratelimit database
    sanitize csvexport i18n
db/migrations/               NNNN_name.up.sql + .down.sql, go:embed
web/                         React 18 + Vite + TS + Tailwind
```

## 2. Why the approval engine is generic

`approval` knows a **subject type** and a **subject id**. It does not know what
a promotion is, and nothing in it may branch on `promotion_plan`.

A superapp will need approvals for purchase orders, leave, discounts,
write-offs and price changes. A chain written inside the promotion module is
rewritten for each of those. The cost of generality here is one indirection;
the cost of skipping it is five reimplementations and five subtly different
audit trails.

The test that keeps it honest: `approval` has **no import** of `promo`, and a
unit test drives the engine with a fabricated subject type.

## 3. Module boundaries

| Layer | May import | May never import |
|---|---|---|
| `domain/*` | other `domain/*`, stdlib | `app`, `adapter`, `platform/database`, any driver |
| `app/*` | `domain/*`, `platform/*`, ports it declares | `adapter/http` |
| `adapter/*` | `app`, `domain`, `platform` | another adapter |
| `platform/*` | stdlib, small libraries | anything project-specific |

Enforced by a test that walks the import graph, not by review.

## 4. Ports

| Port | Phase 1 implementation | Documented swap |
|---|---|---|
| `notify.Sender` | SMTP (mailpit in dev) | WAHA WhatsApp |
| `storage.Store` | MinIO | S3 |
| `importer.Source` | CSV upload | POS API |
| `clock.Clock` | system | fixed, in tests |

Every outbound integration has at least two implementations planned, so
swapping a provider is an adapter change and never a reshape of the core flow.

`clock.Clock` matters more than it looks: lead time and auto-cancel are date
arithmetic, and a test that cannot fix "today" cannot test them.

## 5. The scheduled job

One `job` subcommand, run by a systemd timer, not an in-process ticker — so a
restart cannot skip a day and two instances cannot both run it.

| Job | Cadence | Does |
|---|---|---|
| `auto-cancel-promos` | daily 01:00 WIB | BR-4.6 |
| `import-transactions` | daily 02:00 WIB | BR-6, if a drop file is present |

Each takes an advisory lock so a manual run and the timer cannot overlap, and
each writes a row to `job` recording start, end and outcome.

## 6. Non-functional requirements

| # | Requirement | How it is met | How it is proven |
|---|---|---|---|
| N1 | Not public facing | nginx on the internal network + IP allowlist; Go binds loopback | `12-security.md`; a connection test from outside the allowlist |
| N2 | List screens < 500 ms at 200 sites / 24 months | indexes in `03` §6; pagination capped at 200 | a seeded load test in `07` |
| N3 | Month promo report < 3 s across all brands | `promo_id` partial index; aggregate in SQL, not in Go | same |
| N4 | WCAG AA | tokens with measured contrast, `scripts/contrast.py` | contrast run + a computed-style probe |
| N5 | id-ID default, en second | message catalogues; no inline strings | a test that fails on a missing key |
| N6 | UTC storage, Asia/Jakarta business dates | `timestamptz`; explicit conversion | `calendar` unit tests across a DST-free but holiday-heavy year |
| N7 | ASVS L2 | `12-security.md` control map | the test named beside each control |

### Availability and failure

- A single node. Losing it loses the service; the data is protected by backups
  (`09`), not by redundancy. That is a deliberate phase-1 trade and it is
  written down rather than assumed.
- The database is the only stateful dependency. MinIO holds nothing whose loss
  is unrecoverable in phase 1.
- If SMTP is down, the release notification is queued in `notification_log` and
  retried; **approval still completes.** Notification failure must never block a
  business transition.

## 7. Observability

- **Structured logs**, one line per request: method, path, status, duration,
  trace id, actor. User-supplied values pass through `sanitize.LogValue` so a
  newline cannot forge a line.
- **Trace id** on every request, echoed in `X-Request-Id` and in every error
  body, so a user can quote something a log search will find.
- **Prometheus** at `/metrics`, internal only: request duration by route,
  approval instances by status and age, import rows by outcome, job outcomes.
- **The metric that matters:** the age of the oldest pending approval. It is
  the number that predicts an auto-cancellation.

## 8. Performance notes

- `history_txn` is the only table that grows without bound. At 200 sites × ~300
  receipts/day × 24 months it is on the order of 4 million rows — comfortable
  for PostgreSQL with the indexes in `03` §6.
- Monthly partitioning is the documented next step **if** the report slows. It
  is not done in phase 1 because it complicates the importer for no measured
  benefit, and "measured" is the operative word.
- Reports aggregate in SQL. Pulling rows into Go to sum them is the thing that
  will break N3 first.

## 9. What is deliberately not here

No microservices, no message broker, no Kubernetes, no CQRS, no event sourcing,
no GraphQL, no SSR. One Go binary, one database, one nginx. Each of those is
a real option later and none of them earns its complexity at three brands and a
few hundred sites.
