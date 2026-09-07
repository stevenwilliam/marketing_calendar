# 16 — UAT scenario handbook

**For the people who will sign off, not for the people who built it.**

Every scenario below is written so a business user can run it alone: who to log
in as, exactly what to do, and **what the system must do** — including the
cases where the correct answer is a refusal.

**Date:** 2026-09-07 · Environment: `http://192.168.88.101:8094/`
Password for every seeded account: `MarketingCalendar2026!`

---

## How to use this

- Work through a scenario **in order**. Several depend on the one before.
- The **Expected** column is the pass condition. Anything else is a failure,
  including "it worked but said something different".
- When something fails, note the **`trace_id`** from the error message. It
  appears in the server log for that exact request.
- A scenario marked **⛔ must be refused** is testing that the system says no.
  Those are the ones worth running twice: a control that has never been seen
  refusing is a control nobody knows works.

### Accounts

| Login | Role | Sees |
|---|---|---|
| `rina.hartono@sfg.local` | Staf Marketing | Maxx Coffee only |
| `budi.santoso@sfg.local` | Kepala Marketing | all three |
| `sari.dewi@sfg.local` | Analis Bisnis | all three |
| `agus.pratama@sfg.local` | Kepala Keuangan | all three |
| `dedi.kurniawan@sfg.local` | Operasional | **Maxx Coffee only** |
| `lina.wijaya@sfg.local` | Operasional | all three |
| `hendra.gunawan@sfg.local` | CFO | all three |
| `it.support@sfg.local` | IT | all three |
| `it.admin@sfg.local` | Superadmin | all three |

---

## A — Access

| # | Do this | Expected |
|---|---|---|
| A1 | Log in as `rina.hartono@sfg.local` | Lands on **Kalender**. Sidebar shows Kalender, Rencana promo, Kotak persetujuan, Target, Laporan — **not** Master data, Pengguna, Pengaturan, Audit |
| A2 | ⛔ Log in with the right email and a wrong password | Refused: *"surel atau kata sandi salah"* |
| A3 | ⛔ Log in with an email that does not exist | **The same message, word for word.** Any difference tells an attacker which addresses are real |
| A4 | ⛔ Get the password wrong six times in a row | Account locks. **The correct password is now refused too** — the lock must not reveal that you had found it |
| A5 | As `rina.hartono`, open **Rencana promo** | Only **Maxx Coffee** plans. The brand tabs show Maxx alone, because she is assigned one brand |
| A6 | Log in as `budi.santoso`, open **Rencana promo** | Tabs for **Semua merek, Maxx Coffee, Ruuma, Sunshine**, each with its own count |
| A7 | As `it.support`, open a report and look for **Ekspor CSV** | **The button is absent.** IT administers the system; carrying the sales history out of it is not part of that |
| A8 | As `rina.hartono`, open a report and look for **Ekspor CSV** | **Present.** The person planning a promotion needs the numbers behind it |

---

## B — Creating a promotion

Log in as **`rina.hartono@sfg.local`**.

| # | Do this | Expected |
|---|---|---|
| B1 | **Rencana promo → Buat rencana**. Fill everything, set **Tanggal mulai to 2 days from today**, submit | ⛔ Refused. The message names the **earliest permitted date** and says how many working days: *"paling cepat … (7 hari kerja)"* |
| B2 | Change the start date to **45 days out**, submit again | Either it submits, or it warns about an overlap — go to B3 |
| B3 | If a warning appears, read it | Lists the promotions it clashes with, their dates, and **how many shops overlap**. It is a confirmation, not a refusal |
| B4 | Press **Saya paham, tetap ajukan** | Submits. Status becomes **Menunggu**, at step 1 — Marketing Head |
| B5 | Pick the group **Maxx Jakarta** and check which shops it covers, then create a second promotion on the single-shop group **Maxx Coffee Plaza Indonesia** for overlapping dates | ⛔ It **still warns** — the two groups are different but share a shop. This is the case that matters |
| B6 | Try to save a promotion with **Tanggal selesai before Tanggal mulai** | ⛔ Refused, naming the end-date field |
| B7 | Type `185.000` into **Target penjualan** | Only the digits are kept and the field shows `185.000` as a formatted number — the value stored is 185000, not 185 |
| B8 | Save a plan as **Simpan draf** with the promo rule left empty | **Allowed.** A draft may be incomplete |
| B9 | Now press **Simpan dan ajukan** on that same incomplete draft | ⛔ Refused, naming the missing field. The gate is at submit, not at save |

---

## C — The approval chain

Uses the plan from **B4**.

| # | Log in as | Do this | Expected |
|---|---|---|---|
| C1 | `rina.hartono` | Open her own submitted plan and try to approve it | ⛔ The **Setujui** button is disabled and says why: the creator may not approve their own plan |
| C2 | `hendra.gunawan` (CFO, step 5) | Open the plan, try to approve | ⛔ Refused: *"bukan langkah Anda"*. Step 5 cannot act while step 1 is open |
| C3 | `budi.santoso` (step 1) | **Kotak persetujuan** → the plan is listed → **Setujui langkah 1** | Approved. The plan moves to step 2, **Business Analyst** |
| C4 | `budi.santoso` | Look at **Kotak persetujuan** again | The plan is **gone**. It is no longer waiting for him |
| C5 | `sari.dewi` (step 2) | Approve | Moves to step 3, Finance Head |
| C6 | `agus.pratama` (step 3) | Approve | Moves to step 4, Operation |
| C7 | `dedi.kurniawan` (Operation, **Maxx only**) | Approve the Maxx plan | Approved. Moves to step 5, CFO |
| C8 | `hendra.gunawan` (step 5) | Approve | **Status becomes Dirilis.** Only now |
| C9 | anyone | Open the plan and read **Rantai persetujuan** | All five steps, each with **who** decided and **when** |

### C10 — the company-scoping case ⛔

| Log in as | Do this | Expected |
|---|---|---|
| `dedi.kurniawan` | He holds **Operasional in Maxx Coffee only**. Find a **Ruuma** plan sitting at the Operation step | It is **not in his Kotak persetujuan at all**, and opening it directly refuses. `lina.wijaya` holds the same role across all three and *can* act on it |

### C11 — rejection

| Log in as | Do this | Expected |
|---|---|---|
| `budi.santoso` | Submit a fresh plan, then **Tolak** it with the reason box empty | ⛔ The confirm button stays disabled until a reason is typed |
| | Type a reason and confirm | Status **Ditolak**. The creator can edit and resubmit, and the chain restarts at step 1 with the whole history intact |

---

## D — Approval is permanent

| # | Do this | Expected |
|---|---|---|
| D1 | Open the **Dirilis** plan from C8 as `rina.hartono` and press **Buat versi baru** | The form opens with a banner: the plan is locked and saving creates a **new version** |
| D2 | Change the target and save | A **version 2** appears. Status returns to **Draf** and it must go through all five steps again |
| D3 | Open **Riwayat versi** | **Version 1 is still there, unchanged**, exactly as it was approved |
| D4 | Look for a way to undo an approval | ⛔ **There is none.** A mistake is corrected by rejecting while the chain is open, or by a new version |

---

## E — Targets

Log in as **`budi.santoso@sfg.local`** → **Target**.

| # | Do this | Expected |
|---|---|---|
| E1 | Set the twelve months for one shop to values that **do not** add up to its year target | **Accepted.** The **Selisih** column shows the difference. It is never an error — this is the single most important rule on this screen |
| E2 | Read the note under the grid | It says so explicitly: the months need not sum to the year |
| E3 | Set a target to **0** | Accepted. Zero is meaningful — a shop closed that month |
| E4 | ⛔ Set a target to a negative number | Refused |
| E5 | Change a target for a month that has already ended | Allowed, **and it appears in the Audit trail** |
| E6 | As `dedi.kurniawan` (Operasional), open **Target** | The grid is read-only. Clicking a cell does nothing and the tooltip says he lacks `target.manage` |

---

## F — Import

Log in as **`it.support@sfg.local`** → **Impor**.

| # | Do this | Expected |
|---|---|---|
| F1 | Press **Unduh template** on each of the four kinds | Four CSVs download: transaksi, target tahunan, target bulanan, hari libur |
| F2 | **Drag one of them onto the dashed box** without changing it | Uploads and loads. The kind shown matches the template you dropped |
| F3 | Drop the **same file again** | **Dilewati** — *"sudah pernah dimuat"*. Not an error |
| F4 | Rename that file and drop it again | **Still skipped.** Identity is the file's checksum, not its name |
| F5 | Edit a transaction template: change one `site_code` to `NOPE-001`, drop it | **Sebagian**. The good rows load; click the red rejection count to see `UNKNOWN_SITE` **and the original line** |
| F6 | Edit an amount to `185.000` and drop it | ⛔ That row is rejected `BAD_AMOUNT`. It is **never guessed at** — the alternative is dividing revenue by a thousand |
| F7 | Delete rows from a file but leave `#TOTAL` claiming the old count | ⛔ **Gagal**, nothing loads: *"berkas terpotong?"*. A truncated upload otherwise looks exactly like a quiet trading day |
| F8 | Take a **target bulanan** template, put the same shop/month/type on two lines, drop it | ⛔ The second is rejected `DUPLICATE_IN_FILE` |
| F9 | Load a target file, then load a **corrected** version of the same targets | The figures are **overwritten**, not duplicated |
| F10 | Drop a file that is not a CSV | Refused before upload, naming the file |
| F11 | Use the **keyboard only**: Tab to **Pilih berkas**, press Enter | The file picker opens. Drag-and-drop is an addition, never the only way in |
| F12 | Check the **Jenis** filter on the history table | Filters the runs by kind |

---

## G — Holidays and the lead time

| # | Do this | Expected |
|---|---|---|
| G1 | **Kalender**, move to **August 2026** | 17 August is tinted **pink** and shows *Hari Kemerdekaan Republik Indonesia*. The name is there as well as the colour |
| G2 | Read the legend under the calendar | Explains the tint, the `~` marker, and that holidays count toward the promo lead time |
| G3 | Move to a year from **2028 onward** | Some holidays carry `~`. These are **estimates** — the lunar dates are set by decree and cannot be computed |
| G4 | **Master data → Hari libur**, set the year to 2029 | Each row is marked **Dikonfirmasi** or **Perkiraan** |
| G5 | Change a 2029 estimate to the real decreed date and mark it confirmed | Saved. It is now Dikonfirmasi |
| G6 | Ask IT to re-run `mc seed` | ⛔ Your confirmed date is **untouched**. The seed never replaces a confirmed date with its own estimate |
| G7 | Create a promotion whose earliest legal start falls just after a run of holidays | The earliest date the system offers **skips the holidays**, not just the weekends |

---

## H — Reports and export

| # | Do this | Expected |
|---|---|---|
| H1 | **Laporan → Laporan promo**, filter to **Dirilis**, press **Ekspor CSV** | Downloads. **Only released promotions are in the file** — the export is what the screen shows |
| H2 | Open the CSV in Excel via *Data → From Text/CSV*, delimiter `\|` | Columns line up. Indonesian names are not mangled |
| H3 | Look at a promotion with a **zero** target | Capaian shows **—**, not `0%`. A percentage of nothing is undefined, and 0% would read as a total miss |
| H4 | Compare a promotion's **Aktual** with the transactions in its date range | Actuals count only transactions **tagged with that promo id**. Untagged sales in the same window are normal sales |
| H5 | Find a plan that was **force-released** | Flagged as such in the report. A reviewer must see which promotions bypassed the chain |
| H6 | As `hendra.gunawan` (CFO), open **Audit** and search `report.export` | Your export is recorded, with its row count and filters |

---

## I — Breaking glass

Log in as **`it.admin@sfg.local`** (Superadmin).

| # | Do this | Expected |
|---|---|---|
| I1 | Open a plan stuck mid-chain, press **Rilis paksa** with no reason | ⛔ Confirm stays disabled until a reason is typed |
| I2 | Type a reason and confirm | Released, and tagged **Rilis paksa** wherever it appears |
| I3 | Open **Audit** | The force-release is there: who, when, from which address, and the reason |
| I4 | Ask IT to run `mc job auto-cancel` after moving a pending plan's start date to 3 days away | The plan is **Dibatalkan**. The audit row shows **no person** — the scheduler is not a human |
| I5 | Press **Hidupkan kembali** with a reason | Returns to the step it was waiting at. **The cancellation is still in the history** — a revival is a second event, not an erasure |
| I6 | ⛔ Try to delete an audit row | There is no way to. The database refuses updates, deletes and truncation |

---

## J — On a phone

Open the site on a phone, or narrow the browser to **360px**.

| # | Do this | Expected |
|---|---|---|
| J1 | Log in | Works. Nothing is cut off and the page never scrolls sideways |
| J2 | Tap the **☰** | The menu opens |
| J3 | Open **Kotak persetujuan** | Readable. A CFO approving from a phone is a real situation |
| J4 | Open a plan and approve it | Works at this width |
| J5 | Open a wide table | The **table** scrolls, not the page |

---

## K — Sign-off

| Area | Scenarios | Pass | Fail | Notes |
|---|---|:--:|:--:|---|
| A — Access | A1–A8 | | | |
| B — Creating | B1–B9 | | | |
| C — Chain | C1–C11 | | | |
| D — Permanence | D1–D4 | | | |
| E — Targets | E1–E6 | | | |
| F — Import | F1–F12 | | | |
| G — Holidays | G1–G7 | | | |
| H — Reports | H1–H6 | | | |
| I — Break glass | I1–I6 | | | |
| J — Phone | J1–J5 | | | |

**Signed off by:** ______________________  **Role:** ______________  **Date:** __________

> A failure is not a blocker by default. Record it, decide together whether it
> blocks release, and put anything deferred into `RUN-WHEN-BACK.md` with a name
> against it. A deferred failure with nobody's name on it is a failure that
> ships.
