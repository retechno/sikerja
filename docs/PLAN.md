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
- **Stack Docker penuh sudah diverifikasi** lewat `docker compose up -d` (6 container sehat, `/healthz` 200, login lewat proxy web 200,
  seeder di container jalan). Percobaan ini menemukan bug di `.air.*.toml` (`\.` tidak valid di string TOML) dan sudah diperbaiki.
  Yang belum dicoba hanya target `make dev` itu sendiri (tidak ada `make` di Windows ini); isinya hanya membungkus perintah di atas.
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


---

# Rencana Rilis 2 — Izin & cuti, notifikasi

- [x] 1. Migrasi `000002_leave` + model: `leave_types`, `leave_balances`, `leave_requests`, `leave_approvals`, `holidays`, `app_settings`; repository
- [x] 2. Seeder: 7 jenis cuti, hari libur & cuti bersama 2026 (**dicek ulang HR**), saldo awal semua karyawan, 15 contoh pengajuan
- [x] 3. `WorkingDayService` (tanpa Sabtu/Minggu/libur; setengah hari = 0,5) + test tabel
- [x] 4. `LeaveService`: transaksi + row lock, validasi saldo/tumpang tindih/masa kerja/lampiran, alur bertingkat, potong & kembalikan saldo
- [x] 5. Endpoint + `docs/api.yaml` (disinkronkan otomatis dengan router oleh test)
- [x] 6. Halaman H-08 (hari kerja live dari API, versi ponsel M08), H-09, H-10, H-11
- [x] 7. H-16: tab Jenis cuti, Hari libur, Notifikasi; kartu saldo & persetujuan menunggu di dasbor; menu + lencana + tab bawah ponsel
- [x] 8. Notifikasi: `Notifier`/`Channel`, email via asynq, lonceng dapat dibuka ke tautan, scheduler harian pengingat tenggat
- [x] 9. Test: hari kerja (akhir pekan/libur), saldo kurang, tumpang tindih, setuju/tolak/batal, pengembalian saldo, persetujuan bersamaan
- [x] 10. `go test`, golangci-lint, ESLint, Prettier, vue-tsc, Vitest bersih

## Ringkasan akhir Rilis 2

**Hasil verifikasi**
- Backend `go test ./...` lulus (termasuk 8 goroutine yang menyetujui pengajuan sama → tepat satu berhasil, saldo terpotong sekali),
  golangci-lint 0 isu. Frontend: Vitest 115 test lulus, ESLint/Prettier/vue-tsc bersih.
- Diuji terhadap **MySQL 8.4 asli** (bukan hanya SQLite): migrasi 000002, seeder `EnsureLeave` pada database Rilis 1 yang sudah terisi,
  dan 6 permintaan setuju serentak ke pengajuan yang sama → 1×200, 4×409, saldo terpakai 5 dari 12 (terpotong sekali). Permintaan ke-6
  tidak sampai ke API (proxy Vite di Docker menjatuhkannya), jadi hanya 5 yang tercatat di log API.
- **Belum diverifikasi secara visual di browser**: jaringan Docker Desktop ke host macet (port yang dipublikasikan tidak menjawab
  padahal API sehat di dalam container; memori hampir habis), sehingga halaman H-08–H-11 hanya diuji lewat Vitest, belum
  dilihat/discreenshot dan **belum diuji di layar 360 px**. Mohon dicoba sekali di mesin Anda (`make dev`, login `rina.wulandari@sikerja.test`).
- **Email**: pengiriman dan isi email diuji dengan pengirim palsu; belum dicek end-to-end ke Mailpit.
- **Scheduler 08.00**: logikanya diuji (kunci harian, 3 & 1 hari, tidak mengirim ulang); pemicu cron asynq belum diamati berjalan sungguhan.

**Belum / catatan**
- Daftar hari libur 2026 harus dicek HR (lihat DECISIONS). Kanal WhatsApp belum ada (hanya antarmuka `Channel`).
- Penyesuaian saldo per karyawan oleh HR tidak punya antarmuka (kuota per jenis bisa diubah di H-16).
- CI GitHub Actions, Dockerfile produksi/Nginx, `docs/DEPLOY.md` (Prompt 3) masih belum.


---

# Review & persiapan deploy (Prompt 3)

- [x] 1. `docs/REVIEW.md`: status tiap fitur/halaman SPEC + lokasi file
- [x] 2. Cek keamanan: semua route butuh login (test enumerasi), role/kepemilikan, rate limit (+ per IP), cookie, CORS, lampiran, N+1 (+ antrean cuti), cache 5 menit
- [x] 3. Perbaikan "sebagian/belum": saklar sisa saldo akhir tahun; kolom Penilai di laporan; IP klien/proxy tepercaya; SMTP TLS wajib
- [x] 4. GitHub Actions: golangci-lint, `go test` pada MySQL + Redis, `npm ci`, lint, Vitest, build frontend, build image
- [x] 5. Dockerfile multi-stage api/worker (distroless) + web (Nginx), `docker-compose.prod.yml`, `docs/DEPLOY.md`, `deploy/` (deploy, backup, restore, cron, logrotate)
- [x] 6. Commit & push (branch `chore/review-deploy`)

Detail temuan, perbaikan, dan yang masih belum ada di [REVIEW.md](REVIEW.md). Workflow Actions belum pernah dijalankan di GitHub.
