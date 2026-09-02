# 12 — Security

**Date:** 2026-09-01 · Target **OWASP ASVS v4 Level 2**, every **OWASP Top 10
(2021)** category covered. Each control names where it is implemented **and the
test that proves it**. A control with no test is not a control.

**Status: nothing below is built yet.** Every "Proven by" names the test that
must exist before the control is claimed. `PROGRESS.md` tracks which are real.

---

## 1. Threat model in one paragraph

An internal application, not public facing, holding the sales figures of three
brands and the approval trail for promotional spend. The realistic threats are
**insider** rather than anonymous: a staff member seeing another brand's
numbers, an approval bypassed or forged, a promotion edited after approval, an
export walking out with the whole sales history, and a stolen session on an
unattended machine. The controls below are weighted accordingly.

## 2. OWASP Top 10 (2021)

| # | Category | Control | Where | Proven by |
|---|---|---|---|---|
| A01 | Broken access control | Deny-by-default; every handler declares a permission; every query scoped by company **and** site in the SQL; company assignment is an explicit NOT NULL list, never a wildcard | `adapter/http` middleware, `adapter/postgres` | `security_authz_test.go` — full matrix, asserting what each role **cannot** reach; `security_idor_test.go` — every cross-company read returns 404 |
| A02 | Cryptographic failures | argon2id passwords; TOTP secrets encrypted at rest; refresh tokens stored as SHA-256; TLS in front; secrets only from env | `platform/security`, `09` §10 | `security_test.go::TestPasswordIsArgon2id`, `TestRefreshTokenStoredHashed` |
| A03 | Injection | Placeholders everywhere, no string-built SQL; output encoded per context; **CSV formula injection neutralised** | `adapter/postgres`, `platform/sanitize`, `platform/csvexport` | `csv_test.go::TestFormulaInjection`; `security_injection_test.go` |
| A04 | Insecure design | Approval is a domain state machine with the transition table as a test; approved plans immutable; append-only history | `domain/approval` | `approval_test.go` (BR-4 matrix) |
| A05 | Security misconfiguration | CSP, HSTS **only over TLS**, `nosniff`, `X-Frame-Options: DENY`, referrer policy; trusted proxies limited to loopback; `/metrics` not exposed | `adapter/http/middleware.go`, `deploy/nginx-*.conf` | `security_headers_test.go`; a header check from **another machine** |
| A06 | Vulnerable components | Pinned versions, `go list -m -u`, `npm audit` in CI; dependencies earn their place | `go.mod`, CI | CI job |
| A07 | Identification & auth failures | **Mandatory TOTP**; lockout after repeated failures; rotating refresh with family revocation on reuse; login does not distinguish unknown email from wrong password | `app/auth` | `auth_test.go::TestUnknownEmailAndWrongPasswordAreIdentical`, `TestRefreshReuseRevokesFamily`, `TestLockout` |
| A08 | Software & data integrity | Migrations are the source of truth, checksummed, forward-only; import idempotent and recorded | `platform/database`, `app/importer` | `migrate_test.go::TestChecksumDriftRefused`; `importer_test.go::TestReimportIsIdempotent` |
| A09 | Logging & monitoring failures | Structured logs with a trace id; every approval, override and parameter change audited append-only; log values sanitised against forgery | `platform/logging`, `audit` | `schema_test.go::TestAuditLogAppendOnly`; `sanitize_test.go::TestLogValue` |
| A10 | SSRF | No user-supplied URL is fetched. The only outbound calls are SMTP and WAHA, both to configured hosts | `adapter/notify` | reviewed; no fetch-by-URL endpoint exists |

## 3. Authentication and session

| Control | Detail |
|---|---|
| Passwords | argon2id, 64 MiB / t=3 / p=4, PHC-encoded. A `CHECK` constraint refuses anything not starting `$argon2id$`, so a bcrypt or plaintext value cannot be loaded by a fixture |
| MFA | **TOTP mandatory for every account** (D18, BR-5.3). Without a confirmed enrolment a user reaches only the enrolment flow |
| Lockout | after repeated failures, for a bounded window; the correct password is then refused too |
| Enumeration | login returns an identical response for unknown email and wrong password, and spends the same work — a dummy hash is verified so timing does not leak |
| Access token | ~15 minutes, HS256 with the algorithm pinned; `alg=none` and algorithm-confusion refused |
| Refresh token | rotates on use, stored SHA-256, revocable; **re-presenting a rotated token revokes the entire family** |
| Logout | revokes the presented token; `jti` denylist for the access token's remaining life |

## 4. Authorisation

Deny by default. A user has no permission until a role grants it.

**Permission matrix** — the phase-1 set. The test asserts, for every role, both
what it can and what it **cannot** reach; the second half is the half that
catches regressions.

The role names are the group's real ones (**BR-5.7**, D28).

| Permission | Mktg Staff | Mktg Head | Bus Analyst | Finance Head | Operation | CFO | IT | Superadmin |
|---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:--:|
| `promo.view` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `promo.create` | ✓ | ✓ | | | | | | ✓ |
| `promo.manage` | | ✓ | | | | | | ✓ |
| `target.view` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `target.manage` | | ✓ | | ✓ | | ✓ | | ✓ |
| `report.view` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `report.export` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | | ✓ |
| `import.run` | | | | | | | ✓ | ✓ |
| `import.view` | | ✓ | ✓ | ✓ | | | ✓ | ✓ |
| `site.manage` | | | | | | | ✓ | ✓ |
| `user.manage` | | | | | | | ✓ | ✓ |
| `settings.manage` | | | | | | | ✓ | ✓ |
| `audit.view` | | | | | | ✓ | ✓ | ✓ |
| `force_release` | | | | | | | | ✓ |

Approval rights are **not** in this table — they come from the chain
configuration (BR-4.2), which is the point of a configurable chain.

Three choices in that grid are deliberate:

- **`report.export` reaches every business role, including Marketing Staff**
  *(D38)*. Steven's call: the person who plans the promotion needs the numbers
  behind it, and refusing the export while granting `report.view` only pushes
  the same data out through a screenshot. **IT is still denied it** — the role
  that administers the box has no business reason to carry the sales history
  out of it. Every export is audited with its row count and its filters, which
  is the control that now does the work.
- **`audit.view` is CFO, IT and superadmin.** Somebody outside IT has to be
  able to see who bypassed a control, or the audit trail only protects the
  people who can read it.
- **`superadmin` is a separate role, not a permission on `it`.** It is
  break-glass: force-release and revive, nothing else it does not already have.

> **Segregation of duties: the trade is made, not overlooked.** `superadmin` is
> held by IT *(D35, Steven's decision)*. A technical role can therefore release
> a promotion the business has not approved. What makes that survivable is
> already built: a typed reason is mandatory (BR-4.8), an audit row names the
> actor (BR-8.2), and the plan carries `force_released` onto the promotion
> report (BR-7.5) where a reviewer sees it. What is **not** mitigated is IT
> approving its own bypass — `audit.view` reaching the CFO is the compensating
> control, and it is the reason that ✓ is in the grid.

**Scoping.** Every query filters `company_id` against the caller's companies,
and `site_id` against their site scope where one is set. A user's companies are
**an explicit list, chosen at user creation, one or more** (BR-5.4, D37): three
`user_role` rows are what "sees all three brands" looks like. `company_id` is
`NOT NULL`, so there is no wildcard row that quietly widens when a fourth brand
is added.

**Approval eligibility is scoped the same way** (BR-4.4a): holding `operation`
for Maxx Coffee does not approve a Ruuma plan. This is what lets one
`operation` role serve three brands without a role per brand — the separation
lives on the user, not in the role name.

**IDOR.** Asking for another company's promotion returns `404`, not `403` — the
existence of the row is not disclosed. Proven in
`test/authz_test.go::TestListsAreScopedInTheQuery`, which also asserts the
failure mode that matters most: an **empty** company scope returns nothing
rather than everything, because a missing filter reads exactly like no filter.

## 5. The controls specific to this product

| Risk | Control | Proven by |
|---|---|---|
| A promotion goes live unapproved | status is `RELEASED` only when the chain completes or a superadmin force-releases | `approval_test.go::TestNotReleasedUntilChainComplete`, and walked end to end against the running service |
| An approved promotion is edited | approved plans immutable; an edit creates a new version re-entering the chain (BR-4.7) | `TestApprovedPlanIsImmutable` |
| Someone approves their own plan | refused at every step (BR-4.11) | `TestCreatorCannotApprove` |
| Two approvers race | `SELECT … FOR UPDATE` plus a partial unique index; exactly one wins (BR-4.10) | `test/integration_test.go::TestTwoApproversRaceExactlyOneWins` — two goroutines released together against a real database |
| A chain is edited to remove an inconvenient approver | instances bind to a chain **version**; in-flight plans keep theirs (BR-4.3) | `approval_test.go::TestChainChangeDoesNotAffectInFlight` **and** `test/integration_test.go::TestChainChangeDoesNotAffectInFlight` against a real chain rewrite |
| A bypass leaves no trace | force-release and lead-time override both require a typed reason and write an audit row (BR-4.8, D27) | `TestForceReleaseRequiresReason` |
| The whole sales history walks out | `report.export` is a separate permission from `report.view`; every export is audited with row count and filters | `security_authz_test.go` |
| An import silently doubles revenue | idempotent by `(site, date, receipt_no)` (BR-6.3) | `importer_test.go::TestReimportIsIdempotent` |
| History is quietly rewritten | `audit_log`, `approval_event`, `import_run` and `import_rejection` refuse UPDATE, DELETE **and TRUNCATE** by trigger (D40) | `test/integration_test.go::TestAppendOnlyRefusesUpdateDeleteAndTruncate` |
| A site-scoped user reads another site | `user_site_scope` filtered **in the query**; an empty scope means unrestricted, an excluded request returns nothing (BR-5.5) | `test/authz_test.go::TestSiteScopeFiltersInTheQuery` |
| One export drains the whole history | `report.max_export_rows` refused **before** a byte is written, since a streaming CSV cannot become a clean error once the 200 is out | `internal/app/report.go::checkExportCap` |

## 6. Input handling

Validated on **both** sides, from one source. The frontend validates for
feedback; the backend validates because the frontend can be bypassed with
`curl`.

- Normalise (trim, Unicode NFC, case-fold) **before** validating.
- **Reject, never silently repair.** A value out of range is an error the user
  can act on, not a quietly different value.
- Allow-list enums; deny by default.
- Encode on the way **out** for the destination context: HTML, attribute, URL,
  **CSV cell**, log line, filename.
- Request bodies capped; uploads type-sniffed **from the bytes**, never from
  the client's `Content-Type`.

## 7. Data protection

- Secrets only from the environment; `.env` git-ignored; `.env.example` is the
  documented surface. No default admin password — the first account is created
  by an explicit command that prints a one-time enrolment link.
- Secret-flagged `sys_parameters` masked in the UI, in logs, and in the audit
  trail.
- Backups are encrypted at rest and their restore is **tested**, not assumed
  (`09` §12).
- The sales history is the crown jewel. It never leaves except through an
  audited export.

## 8. Network

Not public facing (D17). nginx binds the internal network with an IP allowlist;
the Go service binds loopback. TLS terminates at nginx. HSTS is sent **only**
once TLS actually terminates.

`X-Forwarded-For` is trusted only from the loopback proxy. Without that,
any client can forge the header and with it the rate-limit key and the audit
log's IP.

**Verification is from another machine.** `curl` on the server never traverses
the firewall and will report success while every real user is blocked.

## 9. Rate limits

| Endpoint | Limit |
|---|---|
| `/auth/login`, `/auth/totp` | tight, per IP and per account |
| `/auth/refresh` | moderate, per IP |
| write endpoints | moderate, per user |
| `/reports/*.csv` | low — an export is expensive and is the exfiltration path |

## 10. What is deliberately not done in phase 1

| Not done | Why | Revisit |
|---|---|---|
| SSO / SAML | no IdP in place | when the group adopts one |
| Field-level encryption of sales figures | database-level access is already restricted; it would break every aggregate | if the DB moves off-premise |
| Anomaly detection on exports | no baseline yet | once export volume is known |
| Penetration test | pre-launch activity | before go-live |
