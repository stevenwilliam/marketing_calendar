# 14 — Panduan pengguna

**Untuk staf yang merencanakan, menyetujui dan mengukur promo.**
Aplikasi ini internal. Tidak ada halaman publik, tidak ada pendaftaran mandiri,
dan setiap akun dibuat oleh administrator.

**Tanggal:** 2026-09-02

---

## 1. Masuk

1. Buka alamat internal aplikasi dari jaringan kantor atau VPN.
2. Masukkan surel dan kata sandi, lalu tekan **Lanjutkan**.
3. Masukkan **enam angka** dari aplikasi autentikator Anda.

**Kata sandi saja tidak pernah cukup.** Setiap akun staf wajib memakai TOTP,
karena satu akun dapat melihat penjualan ketiga merek.

### Login pertama

Layar akan menampilkan sebuah **kunci** sekali saja. Pindai atau salin ke
aplikasi autentikator (Google Authenticator, Authy, 1Password, apa pun yang
mendukung TOTP), lalu masukkan enam angkanya. Setelah itu kunci tidak
ditampilkan lagi.

Kehilangan perangkat autentikator berarti administrator harus mengatur ulang
pendaftaran TOTP Anda. Itu disengaja.

### Kalau gagal masuk

Percobaan berulang mengunci akun sementara — dan selama terkunci, **kata sandi
yang benar pun ditolak**. Ini disengaja: kalau tidak, penguncian justru
memberi tahu penyerang bahwa mereka sudah menemukan kata sandinya.

---

## 2. Yang Anda lihat tergantung peran dan merek

Menu di kiri hanya menampilkan yang boleh Anda buka. Anda juga hanya melihat
data **merek yang ditugaskan kepada Anda** — bukan sebagian yang disembunyikan,
melainkan tidak pernah dimuat sama sekali.

| Peran | Yang dikerjakan |
|---|---|
| Staf Marketing | Membuat rencana promo, melihat kalender, mengunduh laporan |
| Kepala Marketing | Langkah 1 persetujuan; juga membuat dan menetapkan target |
| Analis Bisnis | Langkah 2 persetujuan; membaca dan mengunduh laporan |
| Kepala Keuangan | Langkah 3 persetujuan; menetapkan target |
| Operasional | Langkah 4 persetujuan |
| CFO | Langkah 5 (terakhir); menetapkan target; melihat jejak audit |
| IT | Master data, pengguna, parameter, impor transaksi |
| Superadmin | Rilis paksa dan menghidupkan kembali rencana — keduanya beralasan dan tercatat |

---

## 3. Kalender

Menu **Kalender** menampilkan satu bulan, lintas merek. Setiap promo muncul di
setiap hari ia berjalan, dengan garis kiri berwarna merek dan namanya.
Di bawahnya ada daftar semua promo yang berjalan bulan itu.

- Panah kiri dan kanan berpindah bulan; **Hari ini** kembali ke bulan berjalan.
- Kotak pencarian menyaring bulan yang sedang tampil.
- Klik promo mana pun untuk membuka detailnya.

---

## 4. Membuat rencana promo

**Rencana promo → Buat rencana.**

| Isian | Catatan |
|---|---|
| Merek | Menentukan kelompok toko yang bisa dipilih. Tidak bisa dipindah setelah dibuat |
| Kelompok toko | Selalu milik satu merek. Setiap toko punya kelompok otomatis berisi dirinya sendiri |
| Nama promo | Yang muncul di kalender dan laporan |
| Tanggal mulai & selesai | Boleh melewati batas bulan dan tahun |
| Target penjualan | Rupiah penuh, tanpa sen |
| Target jumlah struk | Berapa banyak transaksi yang diharapkan |
| Mode pesanan | Makan di tempat **atau** bawa pulang — satu promo satu mode |
| Aturan promo | Teks bebas. Inilah yang dibaca penyetuju dan toko |

**Simpan draf** menyimpan tanpa mengajukan. Draf boleh belum lengkap.
**Simpan dan ajukan** memeriksa semuanya lalu memasukkannya ke rantai
persetujuan.

### Masa tenggang

Tanggal mulai paling cepat adalah **7 hari kerja** dari hari ini — bukan tujuh
hari kalender. Akhir pekan dan hari libur nasional tidak dihitung, sehingga
di sekitar Idul Fitri, Natal dan Nyepi tanggalnya mundur lebih jauh dari yang
diduga.

Kalau terlalu cepat, sistem menolak dan **menyebutkan tanggal paling cepatnya**,
misalnya *"paling cepat 11 Sep 2026 (7 hari kerja)"*. Aturan ini dicek saat
**mengajukan**, bukan saat menyimpan draf — jadi draf yang ditinggal semalam
tidak diam-diam menjadi tidak sah.

Angka 7 dapat diubah administrator tanpa deploy.

### Promo yang tumpang tindih

Kalau ada promo lain berjalan **pada toko yang sama** di rentang tanggal yang
beririsan, sistem menampilkan daftarnya dan meminta konfirmasi.

Ini **bukan penolakan**: promo bertumpuk kadang memang disengaja. Yang tidak
boleh terjadi adalah tanpa sadar. Perhatikan bahwa dua kelompok toko yang
berbeda tetap dianggap tumpang tindih kalau berbagi satu toko saja.

---

## 5. Rantai persetujuan

Rencana yang diajukan berjalan melalui lima langkah, berurutan:

**Kepala Marketing → Analis Bisnis → Kepala Keuangan → Operasional → CFO**

- Langkah berikutnya baru terbuka setelah langkah sebelumnya selesai.
- Penyetuju di langkah selanjutnya **tidak bisa** menyetujui lebih awal.
- **Pembuat rencana tidak boleh menyetujui rencananya sendiri**, di langkah mana
  pun, sekalipun ia memegang perannya.
- Peran harus dipegang **di merek rencana itu**. Operasional untuk Maxx Coffee
  tidak dapat menyetujui rencana Ruuma.

Rencana baru berstatus **Dirilis** setelah kelima langkah selesai.

### Kotak persetujuan

**Kotak persetujuan** hanya berisi yang benar-benar menunggu Anda: langkah saat
ini menyebut peran yang Anda pegang di merek itu, Anda bukan pembuatnya, dan
Anda belum memutuskan di langkah itu. Kalau kosong, memang tidak ada.

### Menyetujui dan menolak

Buka rencananya, baca, lalu tekan **Setujui langkah N** atau **Tolak**.

Penolakan **wajib beralasan**. Rencana kembali ke pembuatnya dengan seluruh
riwayat tetap utuh; pembuat dapat memperbaiki dan mengajukan ulang, dan rantai
dimulai lagi dari langkah 1.

> **Keputusan bersifat final.** Tidak ada tombol batal. Koreksi dilakukan
> dengan menolak (kalau rantai masih terbuka) atau dengan versi baru.

### Kenapa rencana saya berhenti?

Buka rencananya. **Rantai persetujuan** menunjukkan setiap langkah, siapa yang
sudah memutuskan dan kapan, serta langkah mana yang sedang menunggu.
Kalau langkah itu menunggu peran yang belum dipegang siapa pun di merek
tersebut, administrator perlu memberikan peran itu — antrean berbasis peran,
jadi rencana langsung muncul tanpa perlu diajukan ulang.

---

## 6. Pembatalan otomatis

Rencana yang **belum selesai disetujui 5 hari sebelum tanggal mulai** dibatalkan
otomatis setiap dini hari. Pembuat dan penyetuju yang sedang menunggu diberi
tahu, dan alasannya tercatat.

Rencana yang dibatalkan begini dapat **dihidupkan kembali oleh superadmin**
dengan alasan tertulis. Ia kembali ke langkah tempat ia menunggu; pembatalannya
tetap tersimpan di riwayat, karena pemulihan adalah peristiwa kedua, bukan
penghapusan.

> Lima langkah persetujuan dengan masa tenggang 7 hari kerja menyisakan sekitar
> **satu langkah per hari**. Kalau pembatalan otomatis mulai sering terjadi,
> itu bukan kerusakan — itu tanda kedua angka perlu disetel ulang. Beri tahu
> administrator.

---

## 7. Mengubah rencana yang sudah disetujui

**Rencana yang sudah masuk rantai tidak dapat diubah.** Kalau bisa, persetujuan
tidak berarti apa-apa.

Menyimpan perubahan pada rencana terkunci membuat **versi baru** yang masuk
rantai dari langkah 1. Versi sebelumnya tetap tersimpan dan terbaca di
**Riwayat versi**.

Pengakuan tumpang tindih dan pengesampingan masa tenggang **tidak ikut** ke
versi baru: pengakuan atas tanggal lama tidak mengatakan apa pun tentang
tanggal baru.

---

## 8. Target

**Target** menampilkan kisi dua belas bulan per toko, ditambah target tahun.
Klik sel mana pun untuk mengubahnya (kalau Anda punya izinnya), Enter untuk
menyimpan, Escape untuk membatalkan.

> **Jumlah dua belas bulan tidak harus sama dengan target tahun.**
> Kolom **Selisih** menampilkan bedanya, dan itu **bukan kesalahan**. Sistem
> sengaja tidak memiliki validasi yang memaksa keduanya cocok.

Target diubah untuk bulan yang sudah lewat? Boleh, dan tercatat di audit.

---

## 9. Laporan

Dua laporan, masing-masing dengan pencarian, filter, dan **Ekspor CSV**.

**Laporan promo** — rencana versus aktual per promo: target dan aktual
penjualan, selisih, capaian persen, target dan aktual struk, serta apakah
rencana itu **dirilis paksa**.

**Target vs aktual** — per toko dan periode, dipisah jenis penjualan.

Dua hal yang perlu dipahami saat membaca:

- **Aktual promo dihitung dari transaksi yang ditandai dengan id promo**, bukan
  dari rentang tanggal. Transaksi di dalam periode promo yang tidak ditandai
  adalah penjualan normal. Menghitungnya sebagai promo akan membuat setiap
  laporan terlihat lebih bagus dari kenyataannya.
- **Capaian terhadap target nol ditampilkan sebagai "—", bukan 0%.** Persentase
  dari nol tidak terdefinisi; menampilkan 0% akan terbaca seperti gagal total
  untuk toko yang memang belum diberi target.

### Ekspor CSV

Tombol **Ekspor CSV** mengunduh **persis yang sedang tampil di layar** —
filter, pencarian dan urutan ikut.

Pemisahnya adalah **pipa (`|`)**, bukan koma. Saat membuka di Excel, pilih
*Data → From Text/CSV* dan tentukan `|` sebagai pemisah.

---

## 10. Arti setiap status

| Status | Arti |
|---|---|
| ○ **Draf** | Belum diajukan. Boleh diubah bebas |
| ◷ **Menunggu** | Sedang dalam rantai persetujuan |
| ✓ **Dirilis** | Kelima langkah selesai. Promo boleh berjalan |
| ✕ **Ditolak** | Dikembalikan ke pembuat dengan alasan. Boleh diperbaiki dan diajukan ulang |
| ⊘ **Dibatalkan** | Dibatalkan otomatis. Dapat dihidupkan kembali oleh superadmin |
| ! **Rilis paksa** | Melewati rantai atas keputusan superadmin, dengan alasan tertulis |

Warna tidak pernah menjadi satu-satunya penanda: setiap status membawa lambang
**dan** kata.

---

## 11. Kalau ada yang salah

Pesan kesalahan menyebutkan **apa yang harus dilakukan**, bukan sekadar bahwa
ada yang gagal. Kalau muncul kode `trace_id`, sertakan saat melapor — dengan
itu administrator dapat menemukan permintaan Anda persis di log.

| Yang Anda lihat | Artinya |
|---|---|
| "paling cepat …" | Tanggal mulai di dalam masa tenggang. Pakai tanggal yang disebutkan |
| "ada promo lain yang tumpang tindih" | Bukan penolakan. Baca daftarnya, lalu konfirmasi kalau memang disengaja |
| "pembuat rencana tidak boleh menyetujui rencananya sendiri" | Minta rekan yang memegang peran itu |
| "bukan langkah Anda" | Rencana belum sampai ke langkah Anda, atau peran Anda ada di merek lain |
| "Anda sudah memutuskan pada langkah ini" | Keputusan Anda sudah tercatat |
| "rencana yang sudah disetujui tidak dapat diubah" | Simpan sebagai versi baru |
| Layar kosong padahal berhasil masuk | Akun Anda belum diberi merek. Hubungi IT |
