# me.md — how Steven actually works

**What this is.** `99-steven-preference.md` is what Steven *says* — the rules he
has written down. This file is what he *does* — the patterns observed across
five projects (`ruuma`, `healthy_catering`, `evermore`, `thenie_v2`,
`marketing_calendar`), **138 logged decisions** — ruuma 46, marketing_calendar
43, evermore 31, healthy_catering 18, thenie_v2 none — and the sessions that
produced them.

Read both. The preference file tells you the standard; this one tells you how
to read the man, so you spend your questions on things that actually need
asking.

**This file is portable.** It is about Steven, not about any one project. Copy
it beside `99-steven-preference.md` into every new repo.

*Compiled 2026-09-07 from the projects on `claudedev`.*

---

## 1. How he writes

Lowercase. Minimal punctuation. No greeting, no sign-off, no preamble. Short
imperative clauses, often comma-spliced into one line:

> `move the 51-Marketing-calendar-new-project.md to respected path, and create all end to end remaining documents in there to start full project, and dont stop until finish`

> `create whatsapp floating buttom in buttom right, and make the footer sticky at buttom`

He writes in English as a second language, fast, and does not proofread.
**Read for intent, never for the letter.** Observed spellings and what they
mean:

| He types | He means |
|---|---|
| `buttom` | button — *or* bottom. Context decides; sometimes both in one sentence |
| `fiture` | feature |
| `moderen` | modern |
| `miss leading` | misleading |
| `respected path` | the respective/appropriate path |
| `real all documents` | read all documents |
| `alot` | a lot |
| `i still not see` | I still don't see |
| `become more elegant` | make it more elegant |

None of these is a new term. Do not build a `buttom` component.

**He does not explain why.** A request arrives as a request. The reasoning is
available if you ask, but asking costs him a round trip, so infer first and
state your inference.

## 2. How he decides

**One or two words, and they are real decisions.**

> `permit it` · `no limit` · `yes` · `all defaults` · `fixed lists, maintained via backend`

Terse is not tentative. Take a one-word `yes` as settled and move — asking him
to confirm a confirmation is the fastest way to waste his time.

**Silence takes the proposal.** Recorded in `healthy_catering` D10: *"silence
on an item takes it."* If you offer a recommendation and he answers the other
questions but not that one, he has accepted it. Log it as *decided by default*
so it stays visible and reversible.

**The `ven:` convention.** When you send a numbered question batch and he pastes
it back, a line beginning `ven:` is his answer to the question above it.
Everything after `ven:` is instruction.

**Offered a choice, he usually takes the narrower rule.** Given "a fixed
recipient list" versus "the list plus every actor in the chain", he chose the
fixed list (`marketing_calendar` D32). Given a nullable budget column "in case",
he chose no column at all (D29). He would rather add a thing later than carry a
half-built one now.

## 3. What he reliably cares about

Counted across every decision log on the server:

| Theme | Appearances | What it means in practice |
|---|---|---|
| **Configurable without a deploy** | 16 | If it could change — a lead time, a cutoff, a recipient list, a tax rate, a bank account — it is a `sys_parameters` row with CRUD, not a constant. He *will* retune it in production. |
| **Audit** | 19 | Who did it, when, and why. Append-only. Especially for anything that bypasses a control. |
| **Reversible over optimal** | 10 | He picks the option that can be undone. Copy rather than move (`healthy_catering` D4). Coexist on a second port rather than stop a running service (`evermore` D21). |
| **Phase 1 versus later** | 28 | Very comfortable deferring, and explicit about it. "for now" and "phase 1" are real boundaries, not hedges. |
| **Money as integers** | every project | Whole rupiah in `BIGINT`. Never negotiable. |
| **Manual where money moves** | 7 | Bank transfer, manual verification, no auto-refund. He does not want the machine moving money unattended. |

Also, without exception, in every project: pipe-delimited CSV export on every
grid (Indonesian data has commas in it constantly), a search box on every list,
and contrast that is measured rather than eyeballed.

## 4. How he reports a problem

**Symptom only. He will not tell you where to look.**

> `i cant visit the web from my laptop`

That is the whole report. Diagnosis is your job, and the first move is
**check the runbooks before touching anything** — twice now the cause was
already written down in `RUN-WHEN-BACK.md` from a previous project.

**The specific trap, which has now cost time on two projects:** his machine
does not reach the dev server from the physical LAN. It arrives through the
VMware host adapter as `172.16.0.1`. A `ufw` rule scoped to `192.168.88.0/24`
looks correct and silently drops every packet. Check
`sudo grep -a 'DPT=<port>' /var/log/ufw.log` first.

**And the reason it was missed both times:** verification was done with `curl`
*on the server*, which never traverses the firewall. **Verify from another
machine, or you have verified nothing.**

## 5. How he gives design feedback

He reacts to what he sees, in his own words, and expects you to translate:

> `the yellow color background is /menu not a good color, use the same green color of background in homepage`

> `rework the overall web design become more elegant`

> `i still not see any background image or animation`

Three things follow:

1. **He names a reference, not a specification.** "the same green as the
   homepage" is the whole brief. Go and measure what that green actually is.
2. **When he has a colour in mind he gives it** — *"choose moderen color
   template, i prefer #778aab, others is mix and match"*. The hex is fixed; the
   rest is yours.
3. **"play with the color" means exercise judgement, not ask.** So does *"any
   input or additional fiture is welcome"* — that is an explicit invitation to
   propose things he did not think of, and he means it.

**His aesthetic choice never overrides AA.** He chose a palette in
`marketing_calendar` that put a 2.41 border on every input; the fix was to
correct it and tell him, not to ship it or to ask. Measure, correct, report the
number.

## 6. Control words

| He types | Effect |
|---|---|
| `coding stop`, `code stop` | **Change nothing.** No edits, files, commits, migrations, deploys or config, until lifted. Holds across turns. |
| `coding start`, `code start` | Hold lifted, resume. |

He uses both spellings interchangeably — `coding stop` 13 times, `code start`
10. Treat them as the same word.

**The hold is scoped to a project, not to the session.** Observed verbatim:

> `dont touch this project since it in "code stop" mode, but can do anything in marketing_calendar project`

**A later, more specific instruction overrides an earlier general one.** He said
`code stop` and then, in the same message, *"create the file and push to git"*.
The narrow instruction wins. When the two genuinely conflict and the action is
destructive, say what you would do and wait — but do not use the hold as an
excuse to ignore a direct request.

## 7. How he runs a build

**Documents first, then build to the end.** He confirms a specification, then
expects the whole thing without checkpoints. *"dont stop until finish"* appears
in almost every session.

During a build he does **not** want:

- milestone approval requests
- a choice between two reasonable options — pick the better one, write down why
- a blocker escalated mid-flight — work around it, note it, hand him the list at
  the end

**He interrupts mid-turn.** New instructions arrive while you are working, often
several in a row on different subjects. Absorb them and keep going; do not
restart your plan or ask which to do first. Handle the urgent one (a broken
site) before the tidy one (a docs rename).

**He works on several things at once** and expects you to hold the thread. In
one session: a docs rename, a firewall outage, a floating button, a sticky
footer, and a whole-site colour change — all mid-turn, all expected to land.

## 8. What he supplies, and what he expects you to decide

Across every project's *"Blocked on Steven"* list, what he reliably owns:

**His to supply** — real bank accounts, legal entity and NPWP, production
domains and TLS, SMTP relay and DNS records, API keys, brand artwork and
photography, real role names, the recipient lists, and the network ranges for
an allowlist.

**Yours to decide** — everything else. Schema shape, module boundaries, error
model, index strategy, test strategy, naming, and every default in a question
batch. He will overrule what he disagrees with, quickly and in three words.

## 9. Reading him correctly — five practical rules

1. **Infer, then state the inference.** "I read this as X; say so if not" costs
   him one word to correct and costs nothing if right.
2. **Never re-ask a settled thing.** A one-word answer is settled.
3. **Give him numbers, not opinions.** He has never argued with a measured
   ratio. He has repeatedly overruled a judgement call.
4. **Flag the consequence he did not ask about**, in a sentence or two, then
   carry on. He wants the flag; he does not want a discussion about it.
5. **Report what was verified and what was not, plainly.** "Done and tested"
   for something merely written is the one thing that damages trust with him,
   and it is written into his preference file twice.

## 10. Where this can be wrong

This is inferred from artefacts, not from asking him. Specifically:

- The typo table is observed, not confirmed. If `buttom` ever genuinely means
  something else, this file is misleading and should be corrected.
- "Silence takes the proposal" comes from one recorded decision
  (`healthy_catering` D10) and has held since, but it is a strong inference from
  a small base.
- Section 8's split is drawn from `Blocked on Steven` lists, which record what
  was *asked of* him — not necessarily everything he would want to own.

Correct this file in place when he contradicts it, and note the date. A
document about a person that nobody updates becomes a caricature.
