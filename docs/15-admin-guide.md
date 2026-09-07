# 15 — Panduan administrator

**Untuk IT dan superadmin.** Operasional harian, konfigurasi, dan hal-hal yang
mudah salah.

**Tanggal:** 2026-09-02 · Pemasangan ada di `13-production-deployment-handbook.md`.

---

## 1. Pembagian peran

**IT** mengurus kotaknya: pengguna, peran, toko, parameter, hari libur, impor,
audit. IT **tidak** memegang `report.export` — mengelola sistem bukan alasan
untuk membawa seluruh riwayat penjualan keluar darinya.

**Superadmin** adalah peran pecah-kaca terpisah: **rilis paksa** dan
**menghidupkan kembali** rencana, keduanya wajib beralasan dan tercatat.

> Superadmin dipegang oleh akun IT (keputusan D35). Artinya peran teknis dapat
> merilis promo yang belum disetujui bisnis. Yang membuat itu tertanggung:
> alasan wajib diketik, baris audit menyebut pelakunya, laporan promo menandai
> rencana yang dirilis paksa, dan **CFO dapat membaca jejak audit** — supaya
> pelanggaran terlihat oleh orang di luar peran yang bisa melakukannya.

---

## 2. Membuat pengguna

**Pengguna → Tambah pengguna.**

Isi nama, surel, kata sandi awal, satu **peran**, dan **satu atau lebih
perusahaan**. Panjang minimum kata sandi diatur di
**Pengaturan → `auth.password_min_length`** (bawaan **8**).

> **Tidak ada nilai "semua perusahaan".** Pengguna yang bekerja lintas merek
> diberi ketiganya secara eksplisit — tiga centang, tiga baris. Ini bukan
> kerepotan tanpa alasan: dulu ada bentuk "kosong berarti semua", dan bentuk
> seperti itu **tumbuh diam-diam**. Begitu merek keempat dimasukkan, setiap
> akun lama otomatis dapat melihat penjualannya, tanpa ada yang memutuskan dan
> tanpa satu baris audit pun.

Kata sandi dinilai dari **panjangnya**, bukan campuran simbol. Aturan komposisi
mendorong orang ke "Password1!"; panjang yang benar-benar mahal bagi penyerang.

> Bawaannya **8**, diturunkan dari 12 atas permintaan pemilik pada 2026-09-07
> (D45). Delapan karakter tanpa aturan komposisi tidak tahan terhadap tebakan
> luring kalau tabel `app_user` pernah bocor — yang menahannya di sini adalah
> **TOTP wajib**, penguncian akun setelah beberapa kegagalan, dan aplikasi yang
> tidak menghadap publik. Naikkan kembali lewat parameter, tanpa deploy, kalau
> salah satu dari ketiganya berubah.

Pengguna mendaftarkan TOTP sendiri saat login pertama. Anda tidak pernah
melihat kuncinya.

### Faktor kedua (TOTP)

**Sedang dimatikan.** `auth.totp_required` = `false` (D46). Login hanya surel
dan kata sandi.

Menyalakannya kembali:

**Pengaturan → `auth.totp_required` → `true`.** Berlaku pada login berikutnya.
Pengguna yang belum pernah mendaftar akan melihat layar pendaftaran dan kunci
sekali-tampil; yang sudah pernah mendaftar memakai kunci lamanya.

> Perlu diketahui sebelum memutuskan untuk membiarkannya mati: panjang minimum
> kata sandi diturunkan ke 8 (D45) dengan alasan ada tiga pengaman lain —
> TOTP wajib, penguncian akun, dan tidak menghadap publik. Mematikan TOTP
> menghapus satu dari ketiganya. Yang tersisa adalah penguncian setelah
> beberapa kegagalan dan daftar izin IP di nginx, untuk aplikasi yang satu
> login-nya melihat penjualan ketiga merek.

### Mengatur ulang TOTP

Hanya relevan kalau `auth.totp_required` bernilai `true`.

Perangkat autentikator hilang berarti akunnya tidak bisa masuk sama sekali.
Hapus pendaftarannya lewat basis data, lalu minta pengguna login ulang — layar
pendaftaran akan muncul lagi:

```bash
set -a && . /etc/marketing_calendar/marketing_calendar.env && set +a
psql "$MC_DATABASE_URL" -c "DELETE FROM user_totp WHERE user_id = (SELECT user_id FROM app_user WHERE lower(email) = lower('orang@sfg.co.id'));"
```

Pastikan identitasnya lewat jalur lain sebelum melakukan ini. Ini praktis
adalah "lewati faktor kedua sekali".

### Menonaktifkan pengguna

```bash
psql "$MC_DATABASE_URL" -c "UPDATE app_user SET is_active = false WHERE lower(email) = lower('orang@sfg.co.id');"
psql "$MC_DATABASE_URL" -c "UPDATE refresh_token SET revoked_at = now() WHERE user_id = (SELECT user_id FROM app_user WHERE lower(email) = lower('orang@sfg.co.id')) AND revoked_at IS NULL;"
```

Baris kedua penting: menonaktifkan akun menghentikan login berikutnya, mencabut
token menghentikan sesi yang sedang berjalan.

---

## 3. Parameter sistem

**Pengaturan → Parameter sistem.** Berlaku pada evaluasi berikutnya, tanpa
deploy. Setiap perubahan tercatat di audit.

| Kunci | Bawaan | Efek |
|---|---|---|
| `promo.lead_time_working_days` | `7` | Tanggal mulai paling cepat, dalam **hari kerja** |
| `promo.auto_cancel_days_before` | `5` | Kapan rantai yang belum selesai dibatalkan, dalam **hari kalender** |
| `promo.overlap_warning_enabled` | `true` | Gerbang pengakuan tumpang tindih |
| `notify.release_recipients` | daftar | **Satu-satunya** penerima surel rilis |
| `import.drop_path` | `/srv/marketing_calendar/import` | Tempat job malam mencari berkas |
| `report.max_export_rows` | `100000` | Batas satu ekspor |
| `auth.lockout_threshold` | `5` | Kegagalan sebelum akun terkunci |
| `auth.lockout_minutes` | `15` | Lama penguncian |

### Dua angka yang saling menentukan

Masa tenggang dan pembatalan otomatis bekerja berpasangan. Dengan rantai lima
langkah, rencana yang diajukan pada hari paling awal punya sekitar **4–6 hari
kalender** untuk lima persetujuan — kira-kira satu langkah per hari, tanpa
kelonggaran untuk akhir pekan atau penyetuju yang cuti.

Kalau pembatalan otomatis mulai sering terjadi, **ukur dulu**:

```sql
-- Berapa lama sebenarnya jarak antar langkah?
SELECT i.subject_id,
       e.step_no,
       e.occurred_at - LAG(e.occurred_at) OVER (PARTITION BY e.instance_id ORDER BY e.step_no) AS jeda
  FROM approval_event e
  JOIN approval_instance i ON i.instance_id = e.instance_id
 WHERE e.action = 'APPROVE'
 ORDER BY i.subject_id, e.step_no;
```

Baru setelah itu naikkan `promo.lead_time_working_days` atau turunkan
`promo.auto_cancel_days_before`.

### Daftar penerima rilis

`notify.release_recipients` adalah **daftar tetap**. Aktor rantai **tidak**
ditambahkan otomatis (keputusan D32) — daftar itu persis daftarnya.

Kegagalan khasnya adalah menjadi basi diam-diam: seseorang pindah, dan surel
rilis terus terkirim ke kotak yang tak pernah dibuka. Dua penangkalnya:
perubahan tercatat di audit, dan **membacanya keras-keras saat tutup bulan**
ada di rutinitas `06-domain-operations.md` §4.

---

## 4. Hari libur

**Master data → Hari libur.** Muat hari libur nasional tahun berikutnya
**setiap Desember**.

Ini bukan pekerjaan administratif kecil: kalender inilah yang menggerakkan masa
tenggang. Tanpa hari libur, "7 hari kerja" dihitung dari hari kerja saja dan
memberi tanggal yang tidak pernah sah di sekitar Idul Fitri, Natal dan Nyepi —
dan **kesalahannya tidak terlihat, karena angkanya tetap tujuh**.

Idul Fitri bergerak setiap tahun; itulah sebabnya tabel ini dipelihara manusia
dan tidak dihitung.

---

## 5. Rantai persetujuan

**Pengaturan → Rantai persetujuan.** Per merek. Setiap langkah punya nama,
aturan (**salah satu peran** atau **semua peran**), dan peran-perannya.

> **Menyimpan membuat versi baru, tidak mengubah yang lama.** Rencana yang
> sedang berjalan tetap terikat pada versi tempat mereka mulai. Seorang
> administrator tidak dapat menghapus penyetuju yang tidak nyaman dari rencana
> yang sudah dalam proses — dan itu memang tujuannya.

Butuh dua peran yang mana saja boleh menyetujui satu langkah? Pilih **salah
satu peran** dan cantumkan keduanya.

---

## 6. Impor transaksi

Job berjalan otomatis pukul **01:00 Asia/Jakarta**. Menjalankan manual:

```bash
cd /home/dev/projects/marketing_calendar
set -a && . /etc/marketing_calendar/marketing_calendar.env && set +a
/home/dev/projects/marketing_calendar/bin/mc job import
```

### Kontrak berkas

Pipa sebagai pemisah, UTF-8, akhiran baris LF:

```
site_code|business_date|pos_receipt_no|sales_type|promo_code|order_mode|gross_amount_idr
MXX-001|2026-09-01|R-000198231|promo|PRM-7QK2|dine_in|185000
MXX-001|2026-09-01|R-000198232|normal||take_away|42000
#TOTAL|2|227000
```

- **Header wajib, dicocokkan berdasarkan nama.** POS yang menukar urutan kolom
  tidak boleh diam-diam memindahkan setiap nilai ke kolom yang salah.
- **Kolom tak dikenal menolak seluruh berkas** — artinya POS mengubah ekspornya
  dan tidak ada yang memberi tahu.
- **`#TOTAL` wajib**: jumlah baris dan jumlah rupiah.
- `gross_amount_idr` adalah **rupiah bulat**: tanpa titik, tanpa koma, tanpa
  simbol. `185.000` ditolak, bukan ditebak — menebaknya bisa membagi pendapatan
  dengan seribu.
- Identitas berkas adalah **checksum**-nya. Mengganti nama tidak membuatnya
  masuk dua kali.

### Membaca hasilnya

| Hasil | Arti |
|---|---|
| **Berhasil** | Semua baris masuk |
| **Sebagian** | Sebagian baris ditolak; sisanya masuk. Buka jumlah "Ditolak" untuk alasan tiap baris |
| **Dilewati** | Checksum ini sudah pernah dimuat. Bukan kesalahan |
| **Gagal** | Tidak ada yang masuk. Biasanya jumlah baris tidak cocok dengan trailer — berkas terpotong |

**Jumlah baris** pada trailer yang menangkap unggahan terpotong, dan unggahan
terpotong itulah satu-satunya kegagalan yang tampak persis seperti hari sepi.
**Jumlah rupiah** hanya ditegakkan kalau tidak ada baris yang ditolak: berkas
dengan satu baris buruk memang berjumlah lebih kecil dari trailer-nya, dan
menggagalkan seluruh berkas karenanya akan kehilangan satu malam perdagangan
demi menjaga sesuatu yang sudah dijaga jumlah baris.

### Memuat ulang satu hari

Letakkan berkas yang sudah dikoreksi untuk toko dan tanggal itu, lalu jalankan
lagi. Importer **mengganti** baris hari-toko tersebut dalam satu transaksi.
Memuat ulang berkas yang **identik** melaporkan nol dan bukan kesalahan.

---

## 7. Pembatalan otomatis

Pukul **02:00 Asia/Jakarta**. Menjalankan manual:

```bash
/home/dev/projects/marketing_calendar/bin/mc job auto-cancel
```

Idempoten: hanya menyentuh rencana yang masih menunggu, dan rencana yang
dibatalkannya tidak lagi menunggu. Menjalankannya dua kali sehari, atau mengejar
hari yang terlewat, sama-sama benar.

Pelakunya adalah penjadwal, bukan orang — baris auditnya sengaja tidak
menyebut nama siapa pun.

---

## 8. Rilis paksa dan pemulihan

Keduanya hanya untuk **superadmin**, keduanya **wajib beralasan**, keduanya
menulis baris audit.

**Rilis paksa** menyelesaikan seluruh langkah sekaligus. Rencananya ditandai
`force_released` dan **muncul dengan tanda itu di laporan promo** — supaya
peninjau dapat melihat sekilas mana yang melewati rantai. Itulah gunanya.

**Menghidupkan kembali** hanya berlaku untuk rencana yang dibatalkan otomatis.
Rencana kembali ke langkah tempat ia menunggu; pembatalannya tetap ada di
riwayat.

Sebelum memakai rilis paksa, coba dulu yang lebih murah: kalau langkahnya
menunggu peran yang belum dipegang siapa pun, **berikan perannya**. Antrean
berbasis peran, jadi rencana langsung muncul tanpa perlu diajukan ulang.

---

## 9. Jejak audit

**Audit** — dapat dicari, dan **tidak dapat diubah**. Basis data menolak
UPDATE, DELETE, dan juga TRUNCATE. Tidak ada pekerjaan pembersihan dan tidak ada
batas waktu simpan; membuatnya berarti membalik keputusan, bukan merapikan.

Yang selalu tercatat: setiap keputusan persetujuan; setiap rilis paksa dan
pemulihan beserta alasannya; setiap pengesampingan masa tenggang; setiap
perubahan rantai, peran, izin atau parameter; setiap ekspor CSV dengan jumlah
baris dan filternya; setiap perubahan target; setiap jalannya impor.

---

## 10. Pemeliharaan

### Sesi kedaluwarsa

```bash
/home/dev/projects/marketing_calendar/bin/mc job purge-sessions
```

Hanya tabel sesi. Audit dan riwayat persetujuan **tidak pernah** dihapus.

### Antrean notifikasi

```bash
/home/dev/projects/marketing_calendar/bin/mc job notify
```

Kegagalan kirim tidak pernah menghalangi persetujuan: keputusan sudah tersimpan,
pesannya mengantre dan dicoba ulang sampai lima kali.

### Kesehatan

```bash
curl -s http://127.0.0.1:8093/healthz    # proses hidup
curl -s http://127.0.0.1:8093/readyz     # basis data terjangkau
sudo journalctl -u marketing-calendar -f
systemctl list-timers 'marketing-calendar*' --no-pager
```

### Pemeriksaan tutup bulan

1. Pastikan setiap hari dalam bulan itu punya `import_run` — hari yang hilang
   adalah penyebab paling umum laporan terlihat aneh.
2. Jalankan target versus aktual per merek.
3. Ekspor laporan promo ke CSV untuk rapat marketing.
4. Catat rencana yang bertanda **rilis paksa**; itulah yang akan ditanyakan
   peninjau.
5. **Baca `notify.release_recipients` keras-keras.** Satu menit sebulan adalah
   seluruh penangkal daftar yang menjadi basi.

---

## 11. Pertanyaan yang sering muncul

**"Kenapa saya tidak bisa mengunduh CSV?"** — IT sengaja tidak memegang
`report.export`. Minta peran bisnis yang memegangnya.

**"Kenapa rencana ini tidak muncul di kotak persetujuan saya?"** — salah satu
dari: bukan langkah Anda; peran Anda ada di merek lain; Anda pembuatnya; atau
Anda sudah memutuskan di langkah itu.

**"Bisakah saya menghapus baris audit yang salah?"** — tidak. Tambahkan
peristiwa yang mengoreksinya; itulah arti append-only.

**"Bisakah satu kelompok toko mencakup dua merek?"** — tidak. Basis data
menolaknya lewat kunci asing, bukan sekadar memperingatkan. Kampanye lintas
merek adalah satu rencana per merek.

**"Kenapa capaian menampilkan '—'?"** — targetnya nol. Persentase dari nol tidak
terdefinisi, dan menampilkan 0% akan terbaca seperti gagal total.
