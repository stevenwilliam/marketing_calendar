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
| A01 | Broken access control | Deny-by-default; every handler declares a permission; every query scoped by company **and** site in the SQL; group-level roles explicit | `adapter/http` middleware, `adapter/postgres` | `security_authz_test.go` — full matrix, asserting what each role **cannot** reach; `security_idor_test.go` — every cross-company read returns 404 |
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

| Permission | Mktg Staff | Mktg Head | Finance Head | Operation | CFO | Bus Analyst | IT | Superadmin |
|---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:--:|
| `promo.view` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `promo.create` | ✓ | ✓ | | | | | | ✓ |
| `promo.manage` | | ✓ | | | | | | ✓ |
| `target.view` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `target.manage` | | ✓ | ✓ | | ✓ | | | ✓ |
| `report.view` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `report.export` | | ✓ | ✓ | ✓ | ✓ | ✓ | | ✓ |
| `import.run` | | | | | | | ✓ | ✓ |
| `import.view` | | ✓ | ✓ | | | ✓ | ✓ | ✓ |
| `site.manage` | | | | | | | ✓ | ✓ |
| `settings.manage` | | | | | | | ✓ | ✓ |
| `audit.view` | | | | | ✓ | | ✓ | ✓ |
| `force_release` | | | | | | | | ✓ |

Approval rights are **not** in this table — they come from the chain
configuration (BR-4.2), which is the point of a configurable chain.

Three choices in that grid are deliberate and worth contesting rather than
inheriting:

- **`report.export` is denied to Marketing Staff and to IT.** Export is how the
  whole sales history leaves the building; the roles that need the numbers get
  it, the role that creates plans and the role that administers the box do not.
- **`audit.view` includes the CFO.** Somebody outside IT has to be able to see
  who bypassed a control, or the audit trail only protects the people who can
  read it.
- **`superadmin` is a separate role, not a permission on `it`.** It is
  break-glass: force-release and revive, nothing else it does not already have.

> **Segregation-of-duties note, stated rather than hidden.** `superadmin` will
> in practice be held by an IT account, which means a technical role can
> release a promotion the business has not approved. The controls that make
> that survivable are all in place — a typed reason is mandatory (BR-4.8), an
> audit row names the actor (BR-8.2), and the plan carries `force_released`
> onto the promotion report (BR-7.5) — but the residual risk is real and is
> Steven's to accept or to reassign. See `00-README-and-decisions.md` §3 Q29.

**Scoping.** Every query filters `company_id` against the caller's companies,
and `site_id` against their site scope where one is set. A `user_role` row with
`company_id IS NULL` is group-level and sees all three brands (D19).

**IDOR.** Asking for another company's promotion returns `404`, not `403` — the
existence of the row is not disclosed. Tested per resource, per role.

## 5. The controls specific to this product

| Risk | Control | Proven by |
|---|---|---|
| A promotion goes live unapproved | status is `RELEASED` only when the chain completes or a superadmin force-releases | `approval_test.go::TestNotReleasedUntilChainComplete` |
| An approved promotion is edited | approved plans immutable; an edit creates a new version re-entering the chain (BR-4.7) | `TestApprovedPlanIsImmutable` |
| Someone approves their own plan | refused at every step (BR-4.11) | `TestCreatorCannotApprove` |
| Two approvers race | row lock plus unique index; exactly one wins (BR-4.10) | `approval_concurrency_test.go` |
| A chain is edited to remove an inconvenient approver | instances bind to a chain **version**; in-flight plans keep theirs (BR-4.3) | `TestChainChangeDoesNotAffectInFlight` |
| A bypass leaves no trace | force-release and lead-time override both require a typed reason and write an audit row (BR-4.8, D27) | `TestForceReleaseRequiresReason` |
| The whole sales history walks out | `report.export` is a separate permission from `report.view`; every export is audited with row count and filters | `security_authz_test.go` |
| An import silently doubles revenue | idempotent by `(site, date, receipt_no)` (BR-6.3) | `importer_test.go::TestReimportIsIdempotent` |
| History is quietly rewritten | `audit_log`, `approval_event` and `import_run` refuse UPDATE and DELETE by trigger | `schema_test.go::TestAppendOnly` |

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
