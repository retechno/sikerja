# Rencana Rilis 1 — KPI & penilaian

Centang `[x]` saat selesai. Urutan = urutan pengerjaan.

## A. Fondasi
- [x] 1. PLAN.md + DECISIONS.md
- [x] 2. Monorepo: `backend/`, `frontend/`, `docker-compose.yml`, `Makefile`, `.env.example`, README
- [x] 3. Backend: config, DB, Redis, logger, router, format respons/error
- [x] 4. Migrasi SQL + model semua tabel kecuali `leave_*` dan `holidays`

## B. Backend inti
- [x] 5. KpiScoreService + test (capaian, cap 120, higher/lower, realisasi 0, skor berbobot)
- [x] 6. AppraisalService (skor perilaku, skor akhir, predikat) + test (89,99/90; contoh H-07 → 93,4 Istimewa)
- [x] 7. Auth: login, refresh, logout, lupa/reset password, ganti password, rate limit Redis
- [x] 8. Middleware auth + RBAC + cek kepemilikan data (karyawan/manajer/HR/direktur)
- [x] 9. Audit log (`activity_log`)
- [x] 10. Master data: departemen, jabatan, karyawan, role, profil + foto, import/template Excel
- [x] 11. Periode (buat/buka/tutup/buka ulang) + template KPI (validasi 100%)
- [x] 12. Realisasi KPI: input, bukti (storage privat + unduh), validasi manajer, kunci periode
- [x] 13. Penilaian: self, atasan, finalisasi
- [x] 14. Dashboard (karyawan/manajer/direktur) + cache Redis, tanpa N+1
- [x] 15. Laporan + export Excel & PDF
- [x] 16. Seeder `cmd/seed` (departemen, user, template, aspek, 6 periode + data acak)
- [x] 17. `docs/api.yaml`
- [x] 18. Test otorisasi + periode tertutup

## C. Frontend
- [x] 19. Scaffold Vue 3 + TS + Tailwind + router + Pinia + axios, layout (sidebar/header/drawer)
- [x] 20. H-01 Login (+ lupa/reset password, ganti password, profil)
- [x] 21. H-02/H-03/H-04 Dashboard
- [x] 22. H-05 KPI Saya, H-06 Penilaian Tim, H-07 Form Penilaian
- [x] 23. H-12 Karyawan (+ import), H-13 Template KPI, H-14 Periode
- [x] 24. H-15 Laporan (+ export), H-16 Pengaturan (departemen & jabatan)
- [x] 25. Test Vitest: tabel KPI & form penilaian

## D. Penutup
- [x] 26. `make test` & `make lint` bersih
- [x] 27. Push, ringkasan akhir di bawah

## Ringkasan akhir

Diselesaikan di branch `feat/rilis-1` (belum di-merge ke `main`).

### Yang selesai (Rilis 1, modul 1–5)

- **Fondasi**: monorepo `backend/` + `frontend/`, `docker-compose.yml` (MySQL, Redis, Mailpit, API, worker, web), `Makefile`,
  `.env.example`, README. Migrasi SQL + model GORM untuk semua tabel kecuali `leave_*` dan `holidays` (+ `password_resets`).
- **Auth**: login, refresh (rotasi), logout, lupa password (token via email lewat antrian asynq → Mailpit), reset, ganti password,
  profil + foto. Rate limit login 5× gagal → jeda 1 menit (Redis). Cookie httpOnly/SameSite=Lax, CSRF header.
- **Otorisasi**: middleware role + cek kepemilikan di service pada setiap endpoint (karyawan = data sendiri, manajer = timnya,
  HR/Direktur = semua). Diuji dengan matriks 30 endpoint × 4 role.
- **Rumus**: `KpiScoreService` (capaian, cap 120, higher/lower, realisasi 0 = 120, skor berbobot) dan `AppraisalService`
  (skor perilaku, skor akhir berbobot periode, predikat). Contoh H-07 (95,7 + 88,0 → 93,4 Istimewa) jadi test case.
- **Fitur**: master data (departemen, jabatan, karyawan, role), import karyawan Excel + template unduhan, template KPI (bobot wajib 100%),
  periode (buka/tutup/buka kembali, diaudit), input realisasi + bukti (storage privat, unduh dengan cek hak akses), validasi manajer,
  self-assessment, penilaian atasan, finalisasi, tiga dasbor (cache Redis 5 menit, tanpa N+1), laporan + ekspor Excel dan PDF,
  audit log (validasi KPI, ubah nilai, finalisasi, buka/tutup periode).
- **Seeder** `go run ./cmd/seed`: 4 departemen, jabatan, 1 direktur, 1 HR, 1 manajer + 5 karyawan per departemen (nama Indonesia,
  password `password`), template KPI persis tabel SPEC, 5 aspek perilaku, 6 periode (5 ditutup + 1 aktif) dengan data acak yang deterministik.
- **Halaman**: H-01 s/d H-07 dan H-12 s/d H-16 (bagian cuti H-16 ditunda), plus lupa/reset password dan profil.
- **API**: `docs/api.yaml` (dijaga sinkron dengan router oleh test).

### Kriteria selesai Rilis 1 (SPEC)

- [x] HR bisa import karyawan dari Excel dan mengatur atasan serta role.
- [x] Template KPI keempat departemen terisi dari seed, bobot tervalidasi 100%.
- [x] Karyawan input realisasi, manajer validasi, skor dan predikat terhitung benar (lolos test).
- [x] Ketiga dasbor tampil sesuai role; laporan bisa diekspor ke Excel dan PDF.
- [x] Karyawan tidak bisa membuka data karyawan lain (test otorisasi).

### Bukti verifikasi

| Pemeriksaan | Hasil |
| --- | --- |
| `go test ./...` (backend) | lulus — rumus, otorisasi, periode tertutup, auth, rate limit, import, ekspor, dasbor (query konstan), seeder, paritas model↔migrasi, paritas `api.yaml`↔router |
| `golangci-lint run`, `go vet`, `gofmt` | 0 temuan |
| Vitest (frontend) | 54 test lulus — tabel KPI, form penilaian, halaman KPI Saya, skor, format |
| ESLint, Prettier, `vue-tsc`, `vite build` | bersih |
| Migrasi + seed + API terhadap **MySQL 8 dan Redis sungguhan** | berjalan; rate limit (429 + `Retry-After: 60`) dan 403 lintas-tim terbukti lewat HTTP |
| UI di browser sungguhan (Playwright) | login, dasbor 3 role, input realisasi, validasi, penilaian (93,4/96,9 Istimewa), tambah karyawan, template, periode, laporan + unduh Excel, pengaturan, tampilan ponsel 360 px |

Bug yang ditemukan dan diperbaiki lewat pengujian itu: upsert nilai atasan menimpa self-assessment; nama tabel `activity_log`;
`v-model` input angka mengirim number sehingga form KPI tidak terkirim; halaman melebar di ponsel karena `sr-only` di tabel.

### Yang belum / catatan jujur

- **Belum**: modul 6 (izin & cuti: H-08–H-11, tab cuti/hari libur di H-16, kartu saldo cuti & persetujuan di dasbor) dan modul 7
  (notifikasi otomatis — lonceng & endpoint ada, tetapi belum ada yang membuat notifikasi). Itu Rilis 2.
- **Belum**: GitHub Actions/CI, Dockerfile produksi + Nginx, `docs/DEPLOY.md` (Prompt 3). Lihat `docs/PROMPTS.md`.
- **Tidak bisa diverifikasi di mesin ini**: `make dev` (tidak ada `make` di Windows ini) dan container `api`/`web`/`worker` berjalan penuh di
  Docker — image-nya berhasil dibangun dan MySQL/Redis/Mailpit berjalan di Docker, tetapi Docker Desktop mulai membalas HTTP 500 saat
  saya menjalankan stack penuh, sehingga langkah itu tidak tuntas. Perintah setara sudah dicoba dari host (lihat README). Mohon jalankan
  `make dev` sekali di mesin Anda.
- Keputusan yang perlu dikonfirmasi pemilik produk (rinci di `docs/DECISIONS.md`): skor tidak dipotong di 100 (capaian bisa 120 sehingga
  skor bisa >100); Go 1.26 (bukan 1.23) karena dependensi terbaru; satu berkas bukti per realisasi; nilai atasan disembunyikan dari
  karyawan sampai final; HR/Direktur tanpa departemen; shadcn-vue tidak dipakai (komponen sendiri dengan token mockup).
- Mockup yang belum diikuti persis: kolom "Penilai" di laporan, filter "Batas penilaian atasan" di periode, "Kirim pengingat",
  beberapa berkas bukti per realisasi, kartu saldo cuti.

### Cara mencoba per role (akun demo, password `password`)

1. `make dev`, buka http://localhost:5173 (email lupa password: http://localhost:8025).
2. **Karyawan** `rina.wulandari@sikerja.test` — Dashboard (KPI 95,7), *KPI Saya* → klik *Ubah* pada "Instalasi" → isi realisasi + bukti → kirim;
   *Penilaian Saya* → isi self-assessment → kirim. Coba buka `/penilaian/<id lain>` → ditolak.
3. **Manajer** `bayu.pratama@sikerja.test` — *Penilaian Tim*: setujui/kembalikan realisasi, lalu *Mulai menilai* Rina → skor akhir muncul langsung → *Kirim penilaian*.
4. **HR** `sinta.maharani@sikerja.test` — *Karyawan* (tambah, ubah role/atasan, *Import Excel* + unduh template), *Template KPI* (ubah bobot ≠ 100% → tidak bisa simpan),
   *Periode* (tutup/buka kembali), *Laporan* (Excel/PDF), *Pengaturan*; finalisasi lewat *Penilaian Tim*/form penilaian (syarat: semua KPI tervalidasi).
5. **Direktur** `hendra.gunawan@sikerja.test` — Dashboard perusahaan (per departemen, tren 6 periode, sebaran predikat), *Laporan*, *Penilaian Manajer*, finalisasi.

### Daftar akun demo lengkap

| Role | Nama | Email |
| --- | --- | --- |
| Direktur | Hendra Gunawan | hendra.gunawan@sikerja.test |
| HR | Sinta Maharani | sinta.maharani@sikerja.test |
| Manajer IT Support | Bayu Pratama | bayu.pratama@sikerja.test |
| Karyawan IT Support | Rina Wulandari, Dimas Saputra, Agus Setiawan, Putri Maharani, Fajar Nugroho | `nama.depan.nama.belakang@sikerja.test` |
| Manajer Customer Service | Indah Permatasari | indah.permatasari@sikerja.test |
| Karyawan Customer Service | Melati Anggraini, Wahyu Hidayat, Sari Puspita, Eko Prasetyo, Dewi Lestari | idem |
| Manajer Sales & Marketing | Rudi Hartono | rudi.hartono@sikerja.test |
| Karyawan Sales & Marketing | Reza Firmansyah, Nanda Kusuma, Citra Lestari, Ayu Permata, Bagas Saputra | idem |
| Manajer Developer | Satria Nugraha | satria.nugraha@sikerja.test |
| Karyawan Developer | Larasati Dewi, Yoga Prasetyo, Andi Kurniawan, Mega Wulan, Tegar Aditya | idem |
