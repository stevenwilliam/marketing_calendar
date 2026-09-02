# marketing_calendar — engineering & product DNA

This file is the contract for how this project is built. Read it first, every
session, before touching code or docs. Where it conflicts with a habit, this
file wins. Where it conflicts with `docs/02-business-rules.md` on *product*
logic, that document wins — this file governs *how* we build, not *what* the
product does.

It is generated from `docs/99-steven-preference.md` §3–§9, which is Steven's
portable engineering DNA, plus the brief in `docs/PROMPT.md`. **Those are the
sources; this is the local application of them.** When they disagree, this file
wins — it is the newer, more specific decision.

---

## 1. What this is

**Owner:** stevenwilliam (itdept.sfg@gmail.com)
**Brief:** `docs/PROMPT.md`, written 2026-09-01
**Started:** 2026-09-01

An **internal superapp** for a group operating three brands:

| Brand | Business | `site_type` |
|---|---|---|
| **Maxx Coffee** | coffee shops | `coffee_shop` |
| **Ruuma** | restaurants | `restaurant` |
| **Sunshine** | catering | `catering` |

One application, many capabilities, **self-hosted and not public facing**.
Every user is a staff member. There is no customer surface, no public sign-up
and no anonymous access.

**Phase 1 is the Marketing Calendar** — targets, promotion planning, a
configurable approval chain and reporting — built on shared platform modules.

### Deviations from the standing preference, and why

| Deviation | Reason |
|---|---|
| **No SEO baseline.** `99` §13 does not apply. | There is no public page. No titles-for-search, Open Graph, `robots.txt`, `sitemap.xml` or JSON-LD. |
| **No customer self-registration.** | Accounts are created by an administrator. |
| **Automated cancellation exists.** `99` §8 says nothing automated cancels a customer's booking. | The 5-day job cancels an incomplete *internal promotion plan*, not a customer booking. It is audited, notifies the creator and the pending approver, is configurable, and is revivable by a superadmin. |

Everything else in `99-steven-preference.md` stands unaltered.

---

## 2. Architecture — non-negotiable

Hexagonal / clean layering. Dependencies point **inward only**:
`adapter → app → domain`, with `platform` available to all. `domain` imports no
framework, no driver, no `net/http`, no SQL.

```
cmd/api/main.go            # thin entrypoint: wire + run (serve, migrate, seed, job)
internal/
  domain/                  # pure business logic + types; exhaustively unit-tested; no I/O
    approval/              #   the generic chain state machine
    promo/                 #   promotion lifecycle
    target/                #   target arithmetic and variance
    calendar/              #   working days, lead time, holidays
    money/                 #   IDR as integers
  app/                     # use-cases; orchestrates domain + ports
  adapter/
    http/                  #   handlers, request/response mapping
    postgres/              #   repositories (raw SQL on money paths)
    storage/               #   S3 / MinIO
    notify/                #   email / WhatsApp / outbound
  platform/                # cross-cutting, business-agnostic, portable
    config/ logging/ metrics/ apierror/ id/ security/ ratelimit/ database/
    sanitize/ csvexport/ i18n/
db/
  migrations/NNNN_name.up.sql + NNNN_name.down.sql
  embed.go                 # go:embed migrations
web/                       # React 18 + Vite + TypeScript + Tailwind
```

`internal/platform/*` is **portable**. Carry it over from an existing project
and adapt rather than reinvent — `/home/dev/projects/evermore/internal/platform/`
and `/home/dev/projects/ruuma/internal/platform/` both have proven shapes for
`config`, `logging`, `apierror`, `id`, `security`, `ratelimit`, `sanitize` and
`csvexport`.

### The module rule

Two layers, and the split is load-bearing.

**Platform modules** — built once, reused by every business module:
`identity`, `masterdata`, `approval`, `notify`, `reporting`, `audit`, `params`,
`importer`.

**Business modules** — phase 1 is `marketing` (targets, promotions, reports).

> **The approval engine is generic from day one.** A superapp will need
> approvals for purchase orders, leave, discounts, write-offs and price
> changes. A chain written inside the promotion module gets rewritten five
> times. `approval` takes a subject type and a subject id; promotions are its
> first consumer and must not be special-cased inside it.

---

## 3. Stack

Backend: **Go (latest)** · **`gin`** · **`gorm`** + `gorm.io/driver/postgres` ·
**PostgreSQL (latest major)** · `golang-jwt/jwt/v5` · `google/uuid` (v7) ·
S3/MinIO (`minio-go/v7`) · Prometheus (`client_golang`) · `golang.org/x/crypto`.
Standard library first; a dependency has to earn its place.

Frontend: **React 18** + **Vite** + **TypeScript** + **Tailwind**,
`web/src/{components,lib,pages}`. Pin React to 18, not 19. Node 20. No PWA.

ORM is `gorm`. **Exception:** any code path touching money uses explicit
`gorm.Exec`/`Raw` with placeholders and integer arithmetic — never the ORM for
money math.

Not defaults, do not reach for them unprompted: automigrate as the source of
truth, GraphQL, microservices, Kubernetes, a NoSQL primary store, SSR
frameworks, CSS-in-JS.

---

## 4. Hard rules

- **Money is integers.** Whole rupiah as `BIGINT`, integer arithmetic
  throughout. Floating point is prohibited in any code path touching money.
  Rates are basis points, rounded half-up:
  `floor((amount * bps + 5000) / 10000)`.
- **IDs are UUIDv7.** Human-facing codes use CSPRNG + Crockford base32.
- **The domain layer is pure and exhaustively unit-tested.** Adapters get
  integration tests.
- **Migrations are forward-only in production**, numbered, each with a matching
  `.down.sql`, embedded via `go:embed`. The migrations are the source of truth.
- **The database enforces the invariant**, not just the application — foreign
  keys, `NOT NULL`, `CHECK`, partial and unique indexes.
- **Concurrency is tested, not assumed.** Two approvers acting at once, and the
  same approver acting twice, must not both succeed: `SELECT … FOR UPDATE`
  inside one transaction, with a test that proves it.
- **Timestamps are `timestamptz` in UTC.** Business-day logic converts to
  `Asia/Jakarta` explicitly — never server-local.
- **History tables are append-only** — approval events, audit log, import runs.
  No updates, no deletes; the migration spells that out.
- **Every input is validated and sanitized on both sides.** The frontend
  validates for *feedback*; the backend validates because the frontend can be
  bypassed with `curl`. Same rules, one source. Sanitize in **and** encode out
  for the context — HTML, attribute, URL, CSV cell, log line, filename.
  **Reject, never silently repair.** Normalise before validating.
- **Deny-by-default authorization.** Every handler declares its permission;
  every object read is scoped by **company and site in the query**, not checked
  afterwards. Negative authz and IDOR tests per role and per resource.
- **Passwords are argon2id.** Access tokens ~15 min, refresh tokens rotating,
  stored hashed and revocable, `jti` denylist on logout. **TOTP is mandatory
  for every staff account** — one login sees all three brands' sales.
- **Errors are typed** through `platform/apierror`; one JSON error model. Never
  leak driver errors to clients.
- **Secrets only via config/env.** Nothing secret in git. `.env.example` is the
  documented surface; the real `.env` is git-ignored.

Security targets **OWASP ASVS v4 Level 2** and covers every **OWASP Top 10
(2021)** category in `docs/12-security.md`, mapping each control to where it is
implemented **and to the test that proves it**.

---

## 5. Docs discipline

- Docs live in `docs/`, numbered per `99-steven-preference.md` §10.
  `docs/02-business-rules.md` is **normative** — rules carry `BR-x.y` IDs and
  code comments and test names reference those IDs.
- **Keep all docs in sync on every decision**, in the same commit as the
  change. A decision that isn't in the docs didn't happen. Every
  behaviour-changing decision gets a dated row in the `00` decision log naming
  the docs it touched.
- `docs/PROGRESS.md` is live build status (✅ done & tested · 🟡 partial ·
  ⬜ not started). **A ✅ has to be re-earned by running the gate, never
  inherited.**
- `docs/RUN-WHEN-BACK.md` holds steps needing an interactive terminal.
- `docs/PROMPT.md` is Steven's brief, verbatim. It is the source, not a record
  of what was built.
- `docs/99-steven-preference.md` is portable and project-agnostic. Improvements
  that are not specific to this project belong there, so they reach the next
  project too.

---

## 6. Working conventions

- **`.claude/skills/impeccable/SKILL.md` is the standard of work.** Read it
  before writing a change and again before reporting one as done.
- **Owner is Steven, nickname "ven".** When he answers a quoted list of
  questions, a line beginning `ven:` is his answer to the question above it.
- **`coding stop` means change nothing** — no edits, no new files, no commits,
  no migrations, no deploys, no config changes — until he says `coding start`.
  It is a hard gate and it **holds across turns**. A new request while the hold
  is on is a request to discuss and plan, not a licence to resume. If unsure
  whether the hold is on, it is.
- **Ask everything at once, up front, with a default per question.** One batch
  before starting, not a drip of questions mid-build.
- **Once the docs and business rules are agreed, build to the end without
  stopping.** Do not pause for milestone approval, and **do not let a blocker
  stop the build** — work around it, note it, keep going on everything that
  does not depend on it, and hand him the whole list at the end.
- **Never stop partway.** If the plan says "build all modules A–Z", build all.
- **Update related documents on every interaction** — including talk-only turns
  that settle a decision.
- **Auto-commit + push after every completed change**, without asking. Small,
  focused commits, conventional-commit messages. `main` is the working branch.
- **Tell the truth about what was verified.** If a test did not run, say so and
  put the step in `RUN-WHEN-BACK.md`. Never report "done and tested" for
  something only written.
- **Verify visual work by looking at it.** Screenshot the rendered page and
  probe computed styles; do not conclude from reading CSS.
- **Editor is `vi`** in every runbook and docs example — never `nano`.
- **OS/server guides use full absolute paths**, never relative ones.
- Prefer editing existing files and reusing `platform/*` over new scaffolding.

---

## 7. Product & UI conventions

- **Search box on every list.** Every screen rendering a list or table has a
  debounced search box that filters it. No exceptions.
- **Every report and every data grid ships an Export to CSV button**, and the
  delimiter is a **pipe (`|`)**, never a comma. It is still a real RFC 4180 CSV
  with `|` as the separator, guarded against formula injection (`=`, `+`, `-`,
  `@`, tab, CR get an apostrophe), and it exports **what the screen is
  currently showing** — filters and search included.
- **Configurable values live in `sys_parameters`.** Anything that could change
  without a code change — lead-time days, the auto-cancel day, notification
  recipients, thresholds, feature toggles — is a row in that table, not a
  constant, and ships with full CRUD behind an admin permission, attributed via
  `updated_by`, with secret-flagged values masked in UI and logs.
- **Operational timings are parameters too.** The 7-working-day lead time and
  the 5-day auto-cancel are the obvious two, and Steven will retune both.
- **Accessibility is AA minimum**, and contrast is *calculated*, not eyeballed
  (`scripts/contrast.py`). Visible focus rings, real labels, keyboard-operable
  pickers, announced errors, `prefers-reduced-motion` and
  `prefers-color-scheme` respected. Colour is never the only signal.
- **Mobile-first**, designed at 360px, light and dark as tokens.
- **Multi-language via message catalogues**, never inline strings.
  Indonesian is the default; English is the second locale.
- **Disabled states explain themselves** — show the reason, not a grey box.
- **An approval decision is irreversible from the UI.** Approve and reject both
  write an append-only event. Correcting a mistake means a new version, not an
  edit.

---

## 8. Document control

Always update related documents in the same commit as the change — PRD,
business rules, data model, API spec, deployment/user/admin guides. A change
whose docs are stale is not done.

---

## 9. Delivery workflow

1. **Initial git setup** — repo, remotes, conventions, this file. ← done
   2026-09-01
2. **Steven — preparation.** He gives the brief, tuning, and final
   confirmation. ← the brief is `docs/PROMPT.md`; the 21 open questions in it
   were **answered by their proposed defaults** so the documents could be
   written (D2–D22). Every one is reversible; see the `00` decision log.
   **Steven answered Q22–Q32 on 2026-09-02** — real role names plus CFO, no
   budget field, our own CSV contract, single-brand site groups, a fixed
   release-recipient list, no retention limit, a palette anchored on `#778AAB`,
   `superadmin` held by IT, Business Analyst as approval step 2, an explicit
   one-or-more company assignment per user, and `report.export` for Marketing
   Staff. Those are D28–D38, and they are folded into every affected document.
3. **Claude — build all documents A→Z.** ← done 2026-09-01
4. **Claude — build all modules in one shot, A→Z.** Do not stop partway.
5. **Claude — test, debug and security-harden, A→Z.** Do not stop partway.
6. **Claude — production deployment handbook** (copy-paste, empty machine, full
   absolute paths), **then** the user guide, **then** the admin guide.

---

## 10. Locale / environment

- **Money:** Indonesian rupiah, `BIGINT` whole rupiah. Sen is obsolete in
  retail, so the rupiah is the minor unit.
- **Timezone:** operating zone `Asia/Jakarta`; storage is UTC and business-day
  logic converts explicitly.
- **Languages:** `id-ID` (default) and `en`, via message catalogues from the
  first string.
- **Working days** need an Indonesian public-holiday calendar, maintained by an
  administrator. Weekday arithmetic alone gives the wrong lead time around Idul
  Fitri, Christmas and Nyepi.

Development runs on the shared dev server `claudedev` at
`/home/dev/projects/marketing_calendar`, per-project config at
`/etc/marketing_calendar/marketing_calendar.env`, nginx reverse-proxying a
local port, PostgreSQL native and shared (one database plus
`marketing_calendar_test`), Docker for satellites only — MinIO, mailpit, WAHA.

**Not public facing.** nginx binds the internal network with an IP allowlist,
and the Go service binds loopback only.
