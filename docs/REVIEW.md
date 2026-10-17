# Review SiKerja terhadap SPEC.md & CLAUDE.md

Tanggal 17 Okt 2026 · branch `chore/review-deploy` · cakupan: Rilis 1 dan 2 (Rilis 3 memang di luar cakupan SPEC versi ini).

Status: **selesai** = ada dan diuji · **sebagian** = ada tetapi ada bagian yang kurang · **belum** = tidak ada.
Path relatif terhadap `backend/internal/` (BE) atau `frontend/src/` (FE).

## Ringkasan

- Semua modul 1–7 dan 16 halaman (H-01…H-16) **ada dan terhubung ke API**. Hasil review menemukan **satu kekurangan SPEC**
  (sisa saldo akhir tahun tidak bisa diatur) dan **satu kekurangan mockup** (kolom *Penilai* di laporan); keduanya sudah ditutup
  di branch ini. Tiga detail mockup yang tidak tertulis di SPEC tetap **belum** (tabel 6, dengan alasan).
- Keamanan: tiap route diperiksa otomatis oleh test, ditambah dua kelemahan yang ditemukan dan diperbaiki (batas laju hanya per
  akun; IP klien di belakang proxy tidak dipercaya/diteruskan). Lihat bagian 4.
- Test sekarang dijalankan juga pada **MySQL 8.4 dan Redis sungguhan** (seluruh `go test ./...` lulus di sana), dan CI memakai itu.
- Image produksi, `docker-compose.prod.yml`, skrip deploy/backup/restore **diuji berjalan** (bagian 7).

## 1. Modul (SPEC → "Modul & fitur")

| # | Fitur SPEC | Status | Lokasi |
| --- | --- | --- | --- |
| 1 | Login email + password, ingat saya | selesai | BE `service/auth.go`, `handler/auth.go` · FE `pages/auth/LoginPage.vue` |
| 1 | Lupa & reset password (token 1 jam, email via antrean) | selesai | BE `service/auth.go`, `mail/mail.go` · FE `pages/auth/ForgotPasswordPage.vue`, `ResetPasswordPage.vue` |
| 1 | Ubah profil, foto, ganti password | selesai | BE `service/employee.go` (Me/UpdateMe/foto), `service/auth.go` · FE `pages/profile/ProfilePage.vue` |
| 2 | Departemen, jabatan | selesai | BE `service/master.go` · FE `pages/settings/SettingsPage.vue` |
| 2 | Karyawan (NIK, atasan, tanggal masuk, status), role, banyak role per orang | selesai | BE `service/employee.go` · FE `pages/master/EmployeesPage.vue`, `EmployeeFormModal.vue` |
| 2 | Import karyawan dari Excel + template unduhan | selesai | BE `service/employee_import.go` · FE `pages/master/ImportModal.vue` |
| 3 | Template KPI per departemen, bobot wajib 100 %, arah, satuan, sumber | selesai | BE `service/template.go` · FE `pages/master/KpiTemplatesPage.vue` |
| 3 | Periode (bulanan/kuartalan; Draft → Aktif → Ditutup; buka kembali hanya HR + audit) | selesai | BE `service/period.go` · FE `pages/master/PeriodsPage.vue` |
| 3 | Input realisasi + lampiran bukti, validasi manajer, kunci periode | selesai | BE `service/kpi.go`, `storage/storage.go` · FE `pages/kpi/MyKpiPage.vue`, `components/kpi/KpiTable.vue` |
| 4 | Self-assessment, penilaian atasan, skor & predikat otomatis, finalisasi HR/Direktur | selesai | BE `service/appraisal.go`, `appraisal_calc.go`, `kpi_score.go` · FE `pages/appraisal/*`, `components/appraisal/*` |
| 5 | Dashboard karyawan / manajer / direktur | selesai | BE `service/dashboard.go`, `cache.go` · FE `pages/dashboard/*` |
| 5 | Laporan + export Excel dan PDF | selesai | BE `service/report.go` (kolom *Penilai* ditambah di review ini; PDF tanpa kolom itu karena lebar halaman) · FE `pages/report/ReportPage.vue` |
| 6 | Jenis izin/cuti (tahunan, sakit, izin, dinas luar, khusus) + kuota | selesai | BE `service/leave_master.go`, `seed/leave.go` · FE `pages/settings/LeaveTypesTab.vue` |
| 6 | Form pengajuan: jenis, tanggal, alasan, lampiran, pengganti | selesai | BE `service/leave.go` · FE `pages/leave/LeaveRequestPage.vue` |
| 6 | Persetujuan bertingkat, saldo otomatis, pembatalan, riwayat, kalender tim | selesai | BE `service/leave.go`, `leave_flow.go` · FE `pages/leave/{ApprovalsPage,LeaveHistoryPage,CalendarPage}.vue` |
| 6 | (tambahan) jadwal kerja per departemen, izin per jam, istirahat 12–13 dikurangi | selesai | BE `service/schedule.go`, `workingdays.go` · FE `pages/settings/WorkSchedulesTab.vue` |
| 7 | Notifikasi di aplikasi (lonceng) | selesai | BE `service/notify.go` · FE `components/layout/NotificationBell.vue` |
| 7 | Email: pengajuan baru, hasil, periode dibuka, pengingat tenggat (harian 08.00) | selesai | BE `service/notify.go`, `reminder.go`, `mail/mail.go`, `cmd/worker/main.go` |
| 7 | WhatsApp sebagai kanal tambahan (cukup "interface mudah ditambah") | selesai (antarmuka); kanalnya Rilis 3 | BE `service/notify.go` (`Channel`) |

## 2. Aturan bisnis (SPEC → "Aturan bisnis")

| Aturan | Status | Lokasi / bukti |
| --- | --- | --- |
| Skor akhir = 70 % KPI + 30 % perilaku, bobot diatur HR per periode | selesai | `service/appraisal_calc.go`, `period.go`; test contoh H-07 → 93,4 |
| Capaian per KPI, batas 120, arah lebih rendah, realisasi 0 → 120 | selesai | `service/kpi_score.go` + `kpi_score_test.go` |
| Predikat (≥ 90 / 80 / 70 / 60) | selesai | `service/appraisal_calc.go` (uji 89,99 vs 90) |
| Periode: realisasi hanya saat Aktif; Ditutup mengunci; buka kembali hanya HR + audit | selesai | `service/period.go`, `kpi.go`; `TestClosedPeriodIsLocked`, `TestOnlyHRCanReopenAndItIsAudited` |
| Cuti tahunan 12 hari, masa kerja ≥ 12 bulan, kuota bisa diubah HR | selesai | `service/leave.go` (`MinTenureMonths`), `leave_master.go` |
| Hari kerja tanpa Sabtu/Minggu/libur; izin setengah hari 0,5 | selesai | `service/workingdays.go`, `schedule.go` (kini mengikuti jadwal departemen & jam) |
| Cuti sakit/khusus tidak memotong saldo tahunan; izin punya kuota 6 | selesai | `seed/leave.go`, `leave_flow.go` (`trackedType`) |
| Saldo dipotong saat disetujui penuh, kembali bila dibatalkan | selesai | `service/leave.go` (`deduct`, `Cancel`); 8 persetujuan serentak → terpotong sekali |
| **Sisa saldo akhir tahun tidak diakumulasi (default), bisa diubah lewat pengaturan** | **sebagian → selesai di review ini** | Sebelumnya hanya "hangus"; sekarang saklar `leave.carry_over` (H-16 → Jenis cuti): `service/leave_master.go` (`ensureBalance`, `Policy`), `GET/PUT /leave-policy`, FE `LeaveTypesTab.vue`; `TestCarryOverIsOffByDefaultAndConfigurable` |
| Ditolak bila saldo kurang / tanggal tumpang tindih | selesai | `service/leave.go`; test saldo & tumpang tindih |
| Alur: karyawan → atasan → HR; Manajer → Direktur; setengah hari cukup atasan; batal sebelum tanggal mulai | selesai | `service/leave_flow.go` (`stagesFor`) |

## 3. Halaman (SPEC → "Daftar halaman")

| Kode | Halaman | Status | Lokasi FE (`pages/`) |
| --- | --- | --- | --- |
| H-01 | Login | selesai | `auth/LoginPage.vue` |
| H-02 | Dashboard Karyawan (skor, progres KPI, saldo cuti, pengajuan terakhir) | selesai | `dashboard/EmployeeDashboard.vue` |
| H-03 | Dashboard Manajer (peringkat, di bawah target, penilaian tertunda, persetujuan menunggu) | selesai | `dashboard/ManagerDashboard.vue` |
| H-04 | Dashboard Direktur (per departemen, tren 6 periode, sebaran, 5 terbaik & terendah) | selesai | `dashboard/DirectorDashboard.vue` |
| H-05 | KPI Saya | selesai | `kpi/MyKpiPage.vue` |
| H-06 | Penilaian Tim | selesai | `appraisal/TeamAppraisalPage.vue` |
| H-07 | Form Penilaian | selesai | `appraisal/AppraisalPage.vue` |
| H-08 | Pengajuan Izin & Cuti (+ versi ponsel M08) | selesai | `leave/LeaveRequestPage.vue` |
| H-09 | Riwayat Pengajuan | selesai | `leave/LeaveHistoryPage.vue` |
| H-10 | Persetujuan | selesai | `leave/ApprovalsPage.vue` |
| H-11 | Kalender Tim | selesai | `leave/CalendarPage.vue` |
| H-12 | Karyawan | selesai | `master/EmployeesPage.vue` |
| H-13 | Template KPI | selesai | `master/KpiTemplatesPage.vue` |
| H-14 | Periode Penilaian | selesai | `master/PeriodsPage.vue` |
| H-15 | Laporan (kolom Penilai ditambah) | selesai | `report/ReportPage.vue` |
| H-16 | Pengaturan (departemen, jabatan, jenis & kuota cuti, hari libur, notifikasi, jam kerja) | selesai | `settings/SettingsPage.vue` + tab |

## 4. Keamanan & performa (permintaan review no. 2)

| Pemeriksaan | Hasil | Bukti |
| --- | --- | --- |
| Semua endpoint lewat auth | **lulus** | `TestEveryRouteExceptPublicRequiresLogin` mengenumerasi *seluruh* route terdaftar dan menuntut 401 tanpa login, kecuali 6 route publik (`healthz`, login, refresh, logout, lupa & reset password). Route baru yang lupa dipasangi `Auth` membuat test gagal. |
| Cek role | lulus | grup `RequireRoles` di `handler/router.go`; `TestRoleMatrix` (endpoint × 4 role) |
| Cek kepemilikan data | lulus | di service (`canSeeLeave`, `loadEmployee`/`teamMembers`, dll.): karyawan = data sendiri, manajer = timnya; 404 (bukan 403) untuk data di luar jangkauan agar keberadaannya tak bocor. `TestEmployeeCannotOpenOthersDataOverHTTP`, `TestManagerCannotOpenOtherTeamOverHTTP`, `TestEmployeeCannotOpenOrChangeOthersData` |
| Rate limit login | lulus, **diperkuat** | Sebelumnya hanya per akun (5 gagal → 1 menit). Kelemahan: penebak yang berganti-ganti email tidak terbatas, dan lupa-password bisa dipakai membanjiri email. Kini ditambah batas per IP via Redis: login 20/menit, lupa password 5/menit, reset 10/menit, refresh 60/menit (`middleware.RateLimit`). Test: `TestLoginRateLimitOverHTTP`, `TestAuthEndpointsAreRateLimitedPerIP`, `TestForwardedForIgnoredFromUntrustedClient` |
| IP klien di belakang proxy | **diperbaiki** | Sebelumnya `X-Forwarded-For` dipercaya dari siapa pun (bisa dipalsukan untuk lolos batas IP) sekaligus tak ada konfigurasi proxy tepercaya. Kini `TRUSTED_PROXIES`; kosong = abaikan header itu. Diuji pada stack produksi: IP asli muncul di log API |
| Cookie httpOnly + Secure di production | lulus | `handler/auth.go` (`HttpOnly`, `SameSite=Lax`, `Secure` = `COOKIE_SECURE`, default true di `APP_ENV=production`); `TestLoginSetsHttpOnlyLaxCookiesAndNoTokenInBody`, `TestSecureCookieInProduction`. Token tidak ada di body maupun localStorage |
| CORS hanya domain web | lulus | `middleware.CORS` — allow-list `CORS_ORIGINS` (compose produksi: `APP_URL`); `TestCORSOnlyForConfiguredOrigin` |
| CSRF | lulus | header `X-Requested-With` wajib untuk metode yang mengubah data + SameSite=Lax; `TestCSRFHeaderRequiredForMutations` |
| Lampiran tidak bisa diakses tanpa izin | lulus | disimpan di luar web root (`STORAGE_DIR`, volume terpisah, dir `0750`); diunduh lewat `GET /kpi/results/:id/evidence`, `/leave/requests/:id/attachment`, `/employees/:id/photo` yang memeriksa hak akses; tipe & ukuran dicek (PDF/JPG/PNG, 5 MB). `TestEvidenceUploadAndAccessControl`, `TestEvidenceUploadDownloadOverHTTP`, `TestLeaveAttachmentOverHTTP` |
| Tidak ada N+1 di dashboard | lulus | `TestDashboardsHaveConstantQueryCount` (jumlah query tetap saat anggota tim bertambah). Ditambah di review ini: `TestLeaveListsHaveConstantQueryCount` untuk antrean persetujuan, ringkasan, kalender, riwayat |
| Cache Redis 5 menit | lulus | `service/cache.go` (`dashboardTTL = 5 * time.Minute`, versi kunci dinaikkan saat data berubah agar tidak basi); Redis mati → jatuh ke query langsung |
| Header keamanan | lulus | `middleware.SecurityHeaders` + Nginx web (`nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`, `Permissions-Policy`) |
| Email SMTP | **diperbaiki** | Dengan akun SMTP, TLS kini wajib di production (sebelumnya oportunistik → bisa turun ke teks polos); port 465 memakai TLS langsung |
| Secret | lulus | `JWT_SECRET` min. 32 karakter, aplikasi menolak start di production bila kurang; `.env` tak di-commit |

Catatan jujur: tidak ada pemindaian dependensi/kerentanan otomatis (`govulncheck`, `npm audit`) di pipeline; disarankan
ditambahkan sebagai langkah berikutnya. Tidak ada CSP ketat karena font dimuat dari Google Fonts dan belum diuji di browser.

## 5. Yang diperbaiki di review ini

| Temuan | Perbaikan |
| --- | --- |
| Sisa saldo akhir tahun tidak bisa diatur (SPEC "bisa diubah lewat pengaturan") | saklar carry-over (backend, API, UI H-16, test) |
| Laporan H-15 tanpa kolom *Penilai* (mockup) | kolom di tabel dan export Excel |
| Batas laju login hanya per akun | batas per IP + `TRUSTED_PROXIES` |
| SMTP oportunistik di production | TLS wajib bila memakai akun |
| Test hanya berjalan di SQLite/miniredis | mode `TEST_MYSQL_DSN`/`TEST_REDIS_ADDR` (skema dari migrasi asli); semua test lulus di MySQL 8.4 + Redis 7 |
| Tidak ada cara membuat akun pertama di production tanpa data demo | `go run ./cmd/seed -bootstrap` (service `bootstrap`) |
| Tak ada CI, Dockerfile produksi, panduan deploy | `.github/workflows/ci.yml`, `backend/Dockerfile`, `frontend/Dockerfile`, `docker-compose.prod.yml`, `docs/DEPLOY.md`, `deploy/*` |

## 6. Yang masih belum / sebagian (jujur)

| Item | Status | Alasan |
| --- | --- | --- |
| Batas penilaian atasan per periode (mockup H-14), "Kirim pengingat" manual (H-06) | belum | Hanya ada di mockup, tidak di SPEC. Pengingat tenggat otomatis (3 & 1 hari) sudah ada. Butuh kolom baru di `appraisal_periods` dan keputusan produk soal tenggat. |
| Beberapa berkas bukti per realisasi | belum | Mockup menunjukkan satu daftar; SPEC hanya "lampiran". Saat ini satu berkas per realisasi (DECISIONS). |
| Kolom Penilai di export PDF | sebagian | Ada di tabel layar dan Excel; lebar halaman PDF sudah penuh. |
| Antarmuka penyesuaian saldo per karyawan | belum | Tidak ada di SPEC (hanya kuota per jenis). Perlu bila HR ingin memberi tambahan hari ke satu orang. |
| Hari libur 2027 dan seterusnya | **risiko operasional** | Daftar bawaan hanya 2026 (`OfficialHolidays`). Mulai Jan 2027 HR harus menambahkannya di H-16, jika tidak, hitung hari kerja menghitung hari libur sebagai hari kerja. Ada di DEPLOY.md. |
| Verifikasi visual H-08…H-11 dan halaman Jam kerja di browser, layar 360 px | belum | Terhalang jaringan Docker–host yang macet saat pengerjaan; hanya diuji lewat Vitest dan API. |
| Email end-to-end ke SMTP nyata / Mailpit, pemicu cron 08.00 diamati sungguhan | belum | Logikanya diuji; pada stack produksi worker terbukti menjadwalkan `0 8 * * *` zona `Asia/Jakarta`, tetapi belum ada pengiriman yang diamati. |
| Rilis 3 (integrasi data KPI, 360°, WhatsApp, GPS, aplikasi mobile) | di luar cakupan | SPEC menyebutnya rilis berikutnya. |

## 7. Infrastruktur yang diuji berjalan

Pada mesin pengembang (Docker Desktop), dengan project terpisah `sikerjaprod` lalu dihapus:

- `docker build --target api|worker` dan image `web`: sukses. Binary statis di distroless, user non-root, rootfs read-only.
- `migrate` → `bootstrap` (dua kali: kedua menolak/aman) → `up -d`: semua service `healthy`.
- Lewat Nginx web: `/healthz` 200, fallback SPA, login 200 dengan cookie `HttpOnly`, header keamanan, unggah foto ke volume
  `storage` (rootfs read-only tetap bisa menulis ke volume), unduh foto 200, rate limit per IP → 429 pada percobaan ke-21.
- `backup.sh` menghasilkan dump valid; `restore.sh` memulihkan data (nama karyawan yang dirusak kembali) dan login tetap bisa sesudahnya.
- Yang **tidak** diuji: skrip pada Ubuntu sungguhan (diuji di Git Bash Windows, jadi arsip `storage-*.tar.gz` gagal karena
  perbedaan path Windows–Docker, bukan karena skrip), `certbot`/TLS, `deploy.sh` utuh (butuh remote git), cron.
- Workflow GitHub Actions **belum pernah dijalankan** (tidak bisa dijalankan dari sini); sintaksnya divalidasi dan langkah-langkahnya
  (lint, test pada MySQL+Redis, build) dijalankan satu per satu secara lokal.
