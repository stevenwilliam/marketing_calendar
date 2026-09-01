# marketing_calendar

An internal superapp for **Maxx Coffee** (coffee shops), **Ruuma**
(restaurants) and **Sunshine** (catering). Self-hosted, **not public facing**,
modular.

**Phase 1: the marketing calendar** — sales targets, promotion planning, a
configurable approval chain, and reporting — on a shared platform of identity,
master data, approvals, notifications, reporting and audit.

## Status

**Documents only.** There is no application code, no database and no
deployment. `docs/PROGRESS.md` is the honest status and says so in detail.

## Read in this order

1. `docs/PROMPT.md` — the brief, verbatim
2. `docs/01-PRD.md` — problem and scope
3. `docs/02-business-rules.md` — **normative**; everything defers to it
4. `docs/03-data-model.md` — schema and constraints
5. `docs/05-architecture-and-nfr.md` — how the code is arranged
6. `docs/08-roadmap.md` — the build order
7. `docs/PROGRESS.md` — what is actually done

`CLAUDE.md` is the working contract. `docs/99-steven-preference.md` is the
portable engineering DNA. `.claude/skills/impeccable/` is the standard of work.

## First task

**Review D2–D22 in `docs/00-README-and-decisions.md`.** Those are the 21
questions from the brief, each resolved by its proposed default so the
specification could be written. While no code exists they cost a document edit
to change; after the schema lands, several cost a migration.

## Commands

```bash
make help
make check          # vet + tests + contrast
make contrast       # measure every colour pairing against design.md
```
