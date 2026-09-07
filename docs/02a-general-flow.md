# 02a — General flow: master data → transaction → report

**The visual companion to `02-business-rules.md`.** Same `BR-x.y` IDs, drawn
rather than written. Where this disagrees with `02`, `02` is right.

**Date:** 2026-09-07

---

## 1. The whole thing on one page

```mermaid
flowchart TD
    subgraph MD["① MASTER DATA — set up once, changed rarely"]
        CO[company<br/>Maxx · Ruuma · Sunshine]
        SI[site]
        SG[site_group<br/>always ONE brand]
        HO[holiday<br/>drives the lead time]
        RO[role + permission]
        US[app_user<br/>1..n companies, explicit]
        CH[approval_chain<br/>versioned]
        PA[sys_parameters<br/>lead time, auto-cancel]
    end

    subgraph TX["② TRANSACTION — the daily work"]
        PL[promotion_plan<br/>+ version]
        AP[approval_instance<br/>+ append-only events]
        REL[status = RELEASED]
        POS[(POS)]
        IMP[import_run]
        HT[history_txn<br/>one row per receipt]
        TG[sales_target]
    end

    subgraph RP["③ REPORT — what any of it was worth"]
        PR[Promo report<br/>plan vs actual]
        TA[Target vs actual]
        CSV[Pipe-delimited CSV]
    end

    CO --> SI --> SG
    CO --> US
    RO --> US
    CO --> CH
    SG --> PL
    US -->|creates| PL
    PA -.->|lead time N| PL
    HO -.->|working days| PL
    PL --> AP
    CH -.->|bound to a VERSION| AP
    US -->|approves, in that company| AP
    AP --> REL
    PA -.->|auto-cancel on day M| AP

    POS -->|nightly CSV| IMP --> HT
    SI -.->|site_code| HT
    REL -.->|promo_code → promo_id| HT

    HT --> PR
    PL --> PR
    HT --> TA
    TG --> TA
    SI --> TG
    PR --> CSV
    TA --> CSV
```

Solid arrows are **things that flow**. Dotted arrows are **things that
constrain** — the parameter that decides a date, the chain version an instance
is pinned to, the released plan an imported row is allowed to point at.

---

## 2. Setup order — nothing here is optional

You cannot skip a box. Each one is the foreign key of the next.

```mermaid
flowchart LR
    A[company] --> B[site]
    B --> C[site_group<br/>auto-created per site]
    B --> D[multi-site group<br/>hand-made]
    A --> E[role granted<br/>per company]
    E --> F[user can log in<br/>and see something]
    A --> G[approval_chain v1]
    H[holiday calendar] --> I[lead time is correct]
    J[sys_parameters] --> I
    C --> K[a promotion can be planned]
    D --> K
    G --> K
    F --> K
    I --> K
```

| Skip this | What happens |
|---|---|
| A company | Nothing else can exist; every table carries `company_id` |
| A site | No site group, so nothing to target |
| A role grant | The user logs in successfully and **sees nothing**. Not an error — deny by default is the resting state (BR-5.1) |
| An approval chain | Submitting refuses: *"tidak ada rantai persetujuan aktif"* |
| The holiday calendar | Lead time computes on weekdays alone and returns a date the rule never allowed — **and the number still looks like seven** (BR-3.3) |

---

## 3. A promotion, end to end

```mermaid
stateDiagram-v2
    [*] --> DRAFT: created (BR-3.1)
    DRAFT --> DRAFT: edited freely
    DRAFT --> PENDING: submit passes the gates
    note right of DRAFT
        Submit checks, in order:
        1. every field present
        2. end >= start
        3. lead time in WORKING days (BR-3.3)
        4. overlaps acknowledged (BR-3.6)
    end note

    PENDING --> PENDING: step n approved (BR-4.4)
    PENDING --> RELEASED: final step approved
    PENDING --> REJECTED: rejected with a reason (BR-4.5)
    PENDING --> CANCELLED: M days before start, chain incomplete (BR-4.6)
    PENDING --> RELEASED: superadmin force-release (BR-4.8)

    REJECTED --> DRAFT: creator edits
    CANCELLED --> PENDING: superadmin revives, with a reason
    RELEASED --> DRAFT: an edit makes a NEW VERSION (BR-4.7)

    RELEASED --> [*]
```

**The one that matters:** `RELEASED → DRAFT` is not an edit. It creates
**version n+1** and re-enters the chain at step 1. Version n stays readable,
exactly as it was approved. If an approved plan could be edited, the approval
would mean nothing.

---

## 4. Who may approve, and when

```mermaid
flowchart TD
    S[Someone opens a pending plan] --> Q1{Is the plan<br/>still PENDING?}
    Q1 -->|no| R1[Refused: sudah selesai]
    Q1 -->|yes| Q2{Do they hold the step's role<br/>IN THE PLAN'S COMPANY?}
    Q2 -->|no| R2[Refused: bukan langkah Anda<br/>BR-4.4a]
    Q2 -->|yes| Q3{Did they create<br/>this plan?}
    Q3 -->|yes| R3[Refused: SELF_APPROVAL<br/>BR-4.11]
    Q3 -->|no| Q4{Have they already<br/>decided this step?}
    Q4 -->|yes| R4[Refused: ALREADY_DECIDED<br/>BR-4.10]
    Q4 -->|no| OK[Event written, step advances]
    OK --> Q5{Was that the<br/>last step?}
    Q5 -->|no| N[Next step opens]
    Q5 -->|yes| REL[RELEASED]
```

All five refusals are named codes, not a generic 403 — an approval screen that
says only "forbidden" is impossible to debug.

Two approvers acting at the same instant are serialised by
`SELECT … FOR UPDATE` inside one transaction, with a partial unique index
behind it. Exactly one wins; the other reads the step that has already moved.

---

## 5. Where a number in a report comes from

This is the subtle one, and the place a report most easily flatters itself.

```mermaid
flowchart LR
    subgraph FILE["Nightly CSV"]
        L1["MXX-001 | 2026-09-01 | R-…1 | promo | PRM-7QK2 | dine_in | 185000"]
        L2["MXX-001 | 2026-09-01 | R-…2 | normal | | take_away | 42000"]
    end

    L1 --> V{site_code known?<br/>promo_code RELEASED?<br/>amount whole rupiah?}
    L2 --> V
    V -->|no| REJ[import_rejection<br/>reason + original line]
    V -->|yes| HT[(history_txn)]

    HT --> F{sales_type}
    F -->|promo, promo_id set| PA[Counts as PROMO actual<br/>for THAT plan]
    F -->|normal, promo_id null| NA[Counts as NORMAL sales]

    PA --> R1[Promo report:<br/>target vs actual, variance]
    NA --> R2[Target vs actual]
    PA --> R2
```

> **Actuals are attributed by `promo_id`, never by date range (BR-7.6).**
> A transaction inside a promotion's dates but not tagged with its id is normal
> sales. Counting it as promo revenue would make every promotion look better
> than it was, and a `CHECK` constraint stops a `normal` row carrying a
> `promo_id` at all.

---

## 6. The three clocks

```mermaid
flowchart LR
    subgraph N["Nightly, unattended"]
        T1["01:00 — mc job import<br/>drop dir → history_txn<br/>then move to processed/ or failed/"]
        T2["02:00 — mc job auto-cancel<br/>chains incomplete M days out"]
    end
    subgraph M["Monthly, by hand"]
        M1[Set targets]
        M2[Plan promotions]
        M3[Month-end: check every day imported,<br/>run the reports,<br/>read the recipient list aloud]
    end
    subgraph Y["Yearly, by hand"]
        Y1["Every December:<br/>load next year's holidays,<br/>replace estimates with the decree"]
    end
    T1 --> T2
```

Both jobs are **idempotent**: re-running one, or catching up a missed day,
does the right thing. Import identity is the file's **checksum**, not its name.

---

## 7. What each import kind touches

```mermaid
flowchart LR
    H{Header row<br/>decides the kind}
    H -->|pos_receipt_no| A[transactions → history_txn]
    H -->|year + amount| B[target_year → sales_target]
    H -->|year + month + amount| C[target_month → sales_target]
    H -->|holiday_date + name| D[holiday → holiday]
    A --> RA[Promo & target reports]
    B --> RB[Target vs actual]
    C --> RB
    D --> RD[Lead-time arithmetic]
```

The kind comes from the **header, never the filename**: the drop directory is
unattended, and a file renamed by hand must still be parsed by what is in it.

---

## 8. Reading this against the rules

| Flow step | Rules that govern it |
|---|---|
| Site → site group | BR-1.3 (one brand, enforced by composite FK) |
| User → company | BR-5.4 (explicit, one or more, no wildcard) |
| Plan → submit | BR-3.1, BR-3.2, BR-3.3, BR-3.5, BR-3.6 |
| Submit → chain | BR-4.1, BR-4.2, BR-4.3 (bound to a version) |
| Approve | BR-4.4, BR-4.4a, BR-4.10, BR-4.11 |
| Release | BR-4.12 (fixed recipient list) |
| Edit after approval | BR-4.7 (new version, chain restarts) |
| Auto-cancel | BR-4.6 |
| CSV → history_txn | BR-6.1 … BR-6.6 |
| Targets | BR-2.1 … BR-2.7 (**BR-2.3: months need not sum to the year**) |
| Report | BR-7.5, BR-7.6, BR-7.7 |
| Anything audited | BR-8.1 … BR-8.5 |
