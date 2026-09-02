# marketing_calendar — build status

✅ done & tested · 🟡 partial · ⬜ not started

**A ✅ here has been re-earned by running the gate in THIS repo.** Nothing is
inherited from another project.

**Last updated:** 2026-09-02 — Q22–Q28 answered by Steven and folded into the
documents (D28–D34). Still no application code.

---

## Deployed right now

**Nothing.** There is no running service, no database and no deployment. The
repository contains documents, the portable preference file, the `impeccable`
skill and the contrast checker.

---

## Milestones

| # | Milestone | State | Notes |
|---|---|---|---|
| M0 | Documents | ✅ | This set. **Awaiting Steven's confirmation** — D2–D22 were taken by default, not answered. D28–D34 *are* his answers (Q22–Q28), and are folded in |
| M1 | Environment & config | ⬜ | |
| M2 | Schema | ⬜ | Designed in `03`; no migration written |
| M3 | Domain | ⬜ | Designed in `02`; no code |
| M4 | Identity & RBAC | ⬜ | Matrix designed in `12` |
| M5 | Master data | ⬜ | |
| M6 | Approval engine | ⬜ | |
| M7 | Targets | ⬜ | |
| M8 | Promotions | ⬜ | |
| M9 | Auto-cancel job | ⬜ | |
| M10 | Importer | ⬜ | |
| M11 | Reports | ⬜ | |
| M12 | Notifications | ⬜ | |
| M13 | Web UI | ⬜ | Design system in `10`; palette rebuilt on `#778AAB` (D34) and re-measured |
| M14 | Security hardening | ⬜ | Control map in `12`; **every "proven by" names a test that does not exist yet** |
| M15 | Deployment | ⬜ | Handbook in `09`; never executed |
| M16 | Guides | ⬜ | |

---

## What has actually been run

| Gate | Result |
|---|---|
| `scripts/contrast.py` | **29/29 pairings measured** on the `#778AAB` palette (D34), and each verified present in `design.md` |
| The contrast guard's own failure mode | **verified**: with `design.md` absent the script exits 1 rather than passing silently |
| Everything else | **not run — there is no code** |

---

## Decisions taken by default, not by Steven

**D2–D22** in `00-README-and-decisions.md` are the 21 questions from
`PROMPT.md` §9, each resolved by its proposed default so the specification
could be complete and internally consistent.

**Reviewing them is the first task.** While no code exists, reversing any of
them costs a document edit. After M2 lands, several of them cost a migration.

The three most expensive to reverse later:

| Decision | Why it is expensive later |
|---|---|
| **D6** — `history_txn` is per receipt | an aggregate cannot be un-summed; changing the grain after history is loaded means reloading it |
| **D12** — approval locks the plan, edits create versions | the versioning is structural in `03`; retrofitting it means moving columns between tables |
| **D18** — TOTP mandatory for everyone | easy to relax, awkward to impose after accounts exist |

---

## Not built — be clear about it

Everything. There is no application. `03` describes a schema that does not
exist, `12` maps controls to tests that have not been written, and `09`
describes a deployment that has never been run.

The documents are a specification, not a report.

---

## Blocked on Steven

| # | Item | Effect |
|---|---|---|
| 1 | **Confirm or amend D2–D22** | The build starts from these; they are cheap to change now |
| 2 | Q29 — who holds `superadmin`, IT or the CFO? | Default applied: IT. It is the force-release role, so this is a segregation-of-duties call, not a technical one |
| 3 | Q30 — is Business Analyst an approval step? | Default applied: no, reads and exports only |
| 4 | Q31 — is Operation one role or one per brand? | Default applied: one. A per-brand split makes step 3 an `ANY_OF` over three roles — a back-office edit |
| 5 | Q32 — should Marketing Staff hold `report.export`? | Default applied: no. It is how the whole sales history leaves the building |
| 6 | A port on `claudedev`, and the networks users arrive from | `13a` B4 and B7 |
| 7 | The GitHub repository | Created; the remote is set to `git@github.com:stevenwilliam/marketing_calendar.git` |

**Q22–Q28 are answered and closed** — see D28–D34. Nothing above blocks the
build; each carries an applied default.
