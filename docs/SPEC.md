# Spesifikasi Aplikasi SiKerja

Versi 16 Okt 2026 · sumber: dokumen "Spesifikasi Aplikasi SiKerja" · mockup: [`docs/mockup/`](mockup/index.html)

## Ringkasan

SiKerja adalah aplikasi web internal untuk mengukur kinerja karyawan lewat KPI dan mengelola izin serta cuti dalam satu tempat.

Perusahaan bergerak di bidang IT dan menjual PMS (Property Management System) untuk hotel. Empat departemen tercakup: IT Support, Customer Service, Sales & Marketing, dan Developer.

**Tujuan:**

- Penilaian kinerja yang objektif dan terukur untuk tiap departemen, dengan KPI dan bobot berbeda.
- Satu dashboard bagi manajemen untuk melihat performa per karyawan, tim, dan perusahaan.
- Pengajuan izin dan cuti tanpa kertas, dengan persetujuan bertingkat dan saldo cuti otomatis.

**Di luar ruang lingkup versi pertama:** penggajian (payroll), rekrutmen, absensi mesin fingerprint, dan aplikasi mobile native.

## Pengguna & hak akses

Ada empat role; setiap orang melihat data sesuai cakupannya saja.

| Role | Contoh jabatan | Cakupan data | Hak utama |
| --- | --- | --- | --- |
| Karyawan | Staf IT Support, CS, Sales, Developer | Data diri sendiri | Lihat KPI & skor sendiri, isi self-assessment, input realisasi KPI manual, ajukan izin/cuti |
| Manajer | Kepala departemen / team lead | Anggota departemennya | Semua hak Karyawan + validasi realisasi KPI, nilai anggota tim, setujui izin/cuti tahap 1 |
| HR / Admin | Staf HR | Seluruh perusahaan | Kelola karyawan, departemen, template KPI, periode penilaian, jenis & kuota cuti, hari libur; persetujuan cuti tahap akhir |
| Direktur | Direktur / owner | Seluruh perusahaan (baca) | Dashboard perusahaan, laporan, finalisasi hasil penilaian |

Satu orang bisa punya lebih dari satu role (misalnya Manajer yang juga Karyawan).

## Modul & fitur

Aplikasi terdiri dari tujuh modul; modul 1–5 masuk rilis pertama, modul 6–7 rilis kedua.

1. **Autentikasi & profil** — login email + password, lupa password, ubah profil dan foto, ganti password.
2. **Master data (HR)** — departemen, jabatan, karyawan (NIK, nama, email, departemen, jabatan, atasan langsung, tanggal masuk, status aktif), role pengguna, import karyawan dari Excel.
3. **KPI**
   - Template KPI per departemen: nama indikator, satuan, target, bobot (%), arah (makin tinggi makin baik / makin rendah makin baik), sumber data (manual atau integrasi).
   - Total bobot per template wajib 100%.
   - Periode penilaian: bulanan atau kuartalan, status Draft → Aktif → Ditutup.
   - Input realisasi KPI oleh karyawan (dengan lampiran bukti) atau manajer; manajer memvalidasi.
4. **Penilaian kinerja**
   - Self-assessment karyawan untuk aspek perilaku/kompetensi (skala 1–5) dan catatan pencapaian.
   - Penilaian atasan pada aspek yang sama + komentar dan rencana pengembangan.
   - Skor akhir dan predikat dihitung otomatis; Direktur atau HR menfinalisasi.
5. **Dashboard & laporan**
   - Dashboard karyawan: skor periode berjalan, progres tiap KPI, saldo cuti.
   - Dashboard manajer: peringkat tim, KPI di bawah target, penilaian yang belum selesai, pengajuan cuti menunggu persetujuan.
   - Dashboard direktur: skor rata-rata per departemen, tren 6 periode, sebaran predikat.
   - Export laporan ke Excel dan PDF.
6. **Izin & cuti**
   - Jenis: cuti tahunan, cuti sakit (wajib lampiran surat dokter jika > 1 hari), izin (setengah hari / satu hari), dinas luar, cuti khusus (menikah, melahirkan, duka).
   - Form pengajuan: jenis, tanggal mulai–selesai, alasan, lampiran, karyawan pengganti (opsional).
   - Persetujuan bertingkat, saldo cuti otomatis, kalender tim, riwayat pengajuan, pembatalan sebelum tanggal mulai.
7. **Notifikasi** — notifikasi di dalam aplikasi + email untuk pengajuan baru, hasil persetujuan, periode penilaian dibuka, dan pengingat tenggat penilaian. WhatsApp disiapkan sebagai kanal tambahan (interface notifikasi dibuat agar mudah ditambah).

**Pengembangan lanjutan (bukan rilis 1–2):** integrasi otomatis data KPI dari helpdesk/ticketing, CRM, dan Jira/GitHub; 360° feedback rekan kerja; check-in GPS untuk kunjungan ke hotel klien; aplikasi mobile.

## KPI default per departemen

Ini template awal (seed data) yang bisa diubah HR kapan saja; target adalah contoh per bulan dan bobot tiap departemen berjumlah 100%.

| Departemen | Indikator | Target / bulan | Arah | Bobot |
| --- | --- | --- | --- | --- |
| IT Support | Tiket terselesaikan | 120 tiket | Lebih tinggi | 25% |
| IT Support | Tiket selesai dalam SLA | 95% | Lebih tinggi | 25% |
| IT Support | Rata-rata waktu respons pertama | 15 menit | Lebih rendah | 15% |
| IT Support | Instalasi / training PMS di hotel klien | 4 kegiatan | Lebih tinggi | 20% |
| IT Support | Rating kepuasan klien | 4,5 dari 5 | Lebih tinggi | 15% |
| Customer Service | Rata-rata waktu respons chat/telepon | 5 menit | Lebih rendah | 25% |
| Customer Service | Kasus selesai tanpa eskalasi | 80% | Lebih tinggi | 25% |
| Customer Service | Skor kepuasan (CSAT) | 90% | Lebih tinggi | 30% |
| Customer Service | Jumlah kasus ditangani | 300 kasus | Lebih tinggi | 20% |
| Sales & Marketing | Pendapatan tercapai vs target | 100% | Lebih tinggi | 35% |
| Sales & Marketing | Hotel klien baru | 3 hotel | Lebih tinggi | 20% |
| Sales & Marketing | Demo PMS dilakukan | 12 demo | Lebih tinggi | 15% |
| Sales & Marketing | Konversi lead ke deal | 20% | Lebih tinggi | 15% |
| Sales & Marketing | Perpanjangan kontrak klien | 90% | Lebih tinggi | 15% |
| Developer | Task selesai tepat waktu | 90% | Lebih tinggi | 30% |
| Developer | Story point diselesaikan | 40 poin | Lebih tinggi | 20% |
| Developer | Bug setelah rilis | 3 bug | Lebih rendah | 25% |
| Developer | Rilis sesuai jadwal | 100% | Lebih tinggi | 15% |
| Developer | Code review dikerjakan | 15 review | Lebih tinggi | 10% |

Aspek perilaku yang dinilai sama untuk semua departemen: disiplin, kerja sama tim, inisiatif, komunikasi, dan tanggung jawab.

## Aturan bisnis

Skor akhir = 70% skor KPI + 30% skor perilaku, dalam skala 0–100; persentase ini bisa diatur HR per periode.

**Capaian per KPI** (dibatasi maksimum 120 agar satu KPI tidak menutupi yang lain):

```
lebih tinggi lebih baik : capaian = min(realisasi / target × 100, 120)
lebih rendah lebih baik : capaian = min(target / realisasi × 100, 120)
```

Jika realisasi 0 pada KPI "lebih rendah lebih baik" (misalnya 0 bug), capaian = 120.

```
skor KPI      = Σ (capaian_i × bobot_i)          bobot dalam pecahan, total 1,00
skor perilaku = rata-rata nilai atasan (1–5) × 20
skor akhir    = skor KPI × bobot_kpi_periode + skor perilaku × bobot_perilaku_periode
```

Nilai self-assessment ditampilkan sebagai pembanding, tetapi tidak masuk perhitungan. Skor ditampilkan 1 desimal.

Contoh (mockup H-07): KPI 95,7 dan nilai atasan rata-rata 4,4 → perilaku 88,0 → skor akhir 0,7 × 95,7 + 0,3 × 88,0 = 93,4 (Istimewa).

**Predikat:**

| Skor akhir | Predikat |
| --- | --- |
| ≥ 90 | Istimewa |
| 80 – 89,99 | Baik |
| 70 – 79,99 | Cukup |
| 60 – 69,99 | Perlu Perbaikan |
| < 60 | Kurang |

**Periode penilaian:** realisasi hanya bisa diinput saat periode Aktif; setelah Ditutup data terkunci dan hanya HR yang bisa membuka kembali (tercatat di audit log).

**Saldo cuti:**

- Cuti tahunan 12 hari kerja per tahun untuk karyawan dengan masa kerja ≥ 12 bulan; kuota per jenis cuti bisa diubah HR.
- Lama cuti dihitung dalam hari kerja: Sabtu, Minggu, dan hari libur nasional/cuti bersama (dari tabel hari libur) tidak dihitung.
- Izin setengah hari = 0,5 hari. Cuti sakit dan cuti khusus tidak memotong saldo cuti tahunan. Izin punya kuota sendiri (default 6 hari/tahun).
- Saldo dipotong saat pengajuan disetujui penuh dan dikembalikan jika dibatalkan.
- Sisa saldo akhir tahun tidak diakumulasi (default), bisa diubah lewat pengaturan.
- Pengajuan ditolak sistem jika saldo kurang atau tanggalnya tumpang tindih dengan pengajuan lain.

**Alur persetujuan izin/cuti:**

1. Karyawan mengajukan → status *Menunggu Atasan*.
2. Atasan langsung menyetujui → *Menunggu HR*, atau menolak dengan alasan → *Ditolak*.
3. HR menyetujui → *Disetujui* (saldo dipotong), atau menolak → *Ditolak*.
4. Karyawan bisa membatalkan selama tanggal mulai belum lewat → *Dibatalkan*.

Jika pengaju adalah Manajer, tahap 1 dilakukan oleh Direktur. Izin setengah hari cukup disetujui atasan langsung.

## Daftar halaman

Ada 16 halaman utama; kode halaman (H-xx) dipakai juga di mockup (`docs/mockup/`) dan prompt pengembangan.

| Kode | Halaman | Role | Isi utama |
| --- | --- | --- | --- |
| H-01 | Login | Semua | Email, password, ingat saya, lupa password |
| H-02 | Dashboard Karyawan | Karyawan | Skor periode berjalan + predikat, progres tiap KPI, saldo cuti, pengajuan terakhir |
| H-03 | Dashboard Manajer | Manajer | Peringkat tim, KPI di bawah target, penilaian tertunda, persetujuan menunggu |
| H-04 | Dashboard Direktur | Direktur | Skor rata-rata per departemen, tren 6 periode, sebaran predikat, 5 terbaik & 5 terendah |
| H-05 | KPI Saya | Karyawan | Tabel KPI periode aktif: target, realisasi, capaian, bobot; input realisasi + upload bukti |
| H-06 | Penilaian Tim | Manajer | Daftar anggota + status penilaian; validasi realisasi KPI |
| H-07 | Form Penilaian | Karyawan, Manajer | Ringkasan KPI, nilai perilaku 1–5 (self vs atasan), komentar, rencana pengembangan, skor akhir |
| H-08 | Pengajuan Izin & Cuti | Karyawan | Form pengajuan, kartu saldo per jenis, perhitungan hari kerja otomatis |
| H-09 | Riwayat Pengajuan | Karyawan | Daftar pengajuan + status + timeline persetujuan, batalkan |
| H-10 | Persetujuan | Manajer, HR, Direktur | Antrian pengajuan, detail, setujui/tolak dengan catatan |
| H-11 | Kalender Tim | Semua | Kalender bulanan siapa yang cuti/izin per departemen |
| H-12 | Karyawan | HR | Daftar, tambah/ubah, import Excel, atur atasan & role |
| H-13 | Template KPI | HR | Template per departemen, indikator, bobot (validasi total 100%) |
| H-14 | Periode Penilaian | HR | Buat periode, buka/tutup, atur persentase KPI vs perilaku |
| H-15 | Laporan | HR, Direktur | Filter periode/departemen, tabel skor, export Excel/PDF |
| H-16 | Pengaturan | HR | Departemen, jabatan, jenis & kuota cuti, hari libur, notifikasi |

Semua halaman memakai layout yang sama: sidebar kiri (menu sesuai role), header dengan pemilih periode, notifikasi, dan menu profil. Tampilan harus nyaman di layar ponsel (sidebar menjadi drawer).

**Gaya visual (dari mockup):** font Plus Jakarta Sans; warna utama `#0E5A52`, aksen `#E8A33D`, sidebar `#0F2421`, latar `#F4F5F1`, teks `#18201E`, teks sekunder `#5B6662`; sudut kartu 16 px; target sentuh minimal 44 px.

## Arsitektur & teknologi

SiKerja terdiri dari dua aplikasi dalam satu repo: frontend Vue 3 (SPA) dan backend REST API berbahasa Go. Keduanya di-deploy bersama di satu server lewat Docker Compose.

```
Browser / ponsel                 API Go 1.23 (Gin)                       Penyimpanan
Vue 3 + TypeScript   ── JSON ──▶  Handler & middleware (JWT, role)  ──▶  MySQL 8 (GORM)
Pinia, Tailwind CSS               Service (skor KPI, predikat,      ──▶  Redis (cache & antrian)
disajikan oleh Nginx                       saldo cuti, hari kerja)  ──▶  Storage privat (lampiran)
                                  Worker asynq (notifikasi,         ──▶  Email (SMTP)
                                           pengingat tenggat)
```

Browser hanya bicara dengan API Go lewat JSON; semua aturan skor dan cuti ada di lapisan service Go agar mudah diuji dengan `go test`.

| Lapisan | Pilihan |
| --- | --- |
| Frontend | Vue 3 + TypeScript, Vite, Vue Router, Pinia, Tailwind CSS, shadcn-vue, vue-chartjs, VeeValidate + Zod |
| Backend | Go 1.23, Gin (HTTP), GORM (ORM), golang-migrate (migrasi), go-playground/validator |
| Database | MySQL 8 (PostgreSQL juga bisa) |
| Cache & antrian | Redis, dengan hibiken/asynq untuk job notifikasi dan jadwal harian |
| Autentikasi | JWT access token (15 menit) + refresh token (7 hari) di cookie httpOnly; password bcrypt |
| Pendukung | excelize (import/export Excel), maroto (PDF), wneessen/go-mail (email), zerolog (log) |
| Test & kualitas | go test + testify, golangci-lint, Vitest + Vue Test Utils, ESLint + Prettier, GitHub Actions |
| Lingkungan lokal | Docker Compose (api, web, mysql, redis, mailpit) + Air untuk hot reload Go |

Struktur repo: `backend/` (`cmd/api` untuk server, `cmd/worker` untuk antrian, `internal/{config,handler,middleware,service,repository,model}`, `migrations/`) dan `frontend/` (`src/pages` dengan subfolder per modul: dashboard, kpi, appraisal, leave, master, report, settings; `src/api` untuk klien HTTP). Kontrak API ditulis di `docs/api.yaml` (OpenAPI 3) supaya frontend dan backend bisa dikerjakan paralel.

## Model data

Ada 19 tabel utama; semua memakai `id` bigint, `created_at`/`updated_at`, dan soft delete untuk data master.

| Tabel | Kolom penting | Relasi |
| --- | --- | --- |
| `departments` | name, code, head_id | head_id → employees |
| `positions` | name, department_id | → departments |
| `users` | email, password, is_active, last_login_at | 1–1 employees |
| `employees` | user_id, nik, name, phone, department_id, position_id, manager_id, join_date, status, photo | manager_id → employees |
| `roles` / `user_roles` | name (employee, manager, hr, director) | dicek oleh middleware RBAC di Go |
| `kpi_templates` | department_id, name, is_active | → departments |
| `kpi_indicators` | kpi_template_id, name, unit, target, weight, direction (higher/lower), source (manual/integration), sort | → kpi_templates |
| `appraisal_periods` | name, type (monthly/quarterly), start_date, end_date, status (draft/active/closed), kpi_weight, behavior_weight | — |
| `kpi_results` | period_id, employee_id, indicator_id, actual, achievement, evidence_path, status (submitted/validated/rejected), validated_by | → periods, employees, kpi_indicators |
| `behavior_aspects` | name, description, sort | — |
| `appraisals` | period_id, employee_id, appraiser_id, kpi_score, behavior_score, final_score, grade, employee_comment, manager_comment, development_plan, status (draft/self_done/manager_done/final) | unik per (period, employee) |
| `appraisal_scores` | appraisal_id, aspect_id, self_score, manager_score | → appraisals, behavior_aspects |
| `leave_types` | name, code, annual_quota, deducts_balance, requires_attachment, allow_half_day | — |
| `leave_balances` | employee_id, leave_type_id, year, quota, used | unik per (employee, type, year) |
| `leave_requests` | employee_id, leave_type_id, start_date, end_date, half_day, days, reason, attachment_path, substitute_id, status | → employees, leave_types |
| `leave_approvals` | leave_request_id, approver_id, level (1/2), action, note, acted_at | → leave_requests |
| `holidays` | date, name, type (nasional/cuti_bersama) | — |
| `notifications` | user_id, type, data (JSON), read_at | — |
| `activity_log` | user_id, action, subject_type, subject_id, old_values, new_values | audit semua perubahan penting |

## Kebutuhan non-fungsional & keamanan

Aplikasi harus aman untuk data karyawan, cepat di koneksi kantor biasa, dan mudah dirawat tim developer internal.

- **Keamanan:** password di-hash (bcrypt), proteksi CSRF, rate limit login (5 kali gagal → jeda 1 menit), otorisasi setiap endpoint lewat middleware role ditambah cek kepemilikan data di service, file lampiran disimpan di storage privat dan diunduh lewat endpoint yang mengecek hak akses.
- **Privasi:** karyawan tidak bisa melihat skor karyawan lain; manajer hanya timnya.
- **Audit:** perubahan nilai, validasi KPI, persetujuan cuti, dan buka-tutup periode tercatat (siapa, kapan, nilai lama → baru).
- **Performa:** halaman dashboard tampil < 2 detik untuk 200 karyawan; query dashboard memakai Preload GORM (tanpa N+1 query) dan cache Redis 5 menit.
- **Tampilan:** responsif (ponsel 360 px sampai desktop), bahasa Indonesia, format tanggal `16 Okt 2026`, angka desimal pakai koma.
- **Kualitas kode:** test otomatis (go test + testify di backend, Vitest di frontend) untuk perhitungan skor, saldo cuti, hitung hari kerja, dan alur persetujuan; golangci-lint serta ESLint + Prettier; CI di GitHub Actions.
- **Deployment:** server Linux (Ubuntu) dengan Docker Compose; Nginx menyajikan build Vue dan meneruskan /api ke binary Go; backup database harian.

## Tahapan rilis & kriteria selesai

Pembangunan dibagi tiga rilis; rilis 1 sudah bisa dipakai untuk penilaian bulanan pertama.

1. **Rilis 1 — KPI & penilaian:** fondasi proyek, login & role, master data, template KPI, periode, input & validasi realisasi, form penilaian, tiga dashboard, laporan + export.
2. **Rilis 2 — Izin & cuti:** jenis & saldo cuti, hari libur, form pengajuan, persetujuan bertingkat, riwayat, kalender tim, notifikasi email.
3. **Rilis 3 — Lanjutan:** integrasi data KPI otomatis (helpdesk, CRM, Jira/GitHub), 360° feedback, notifikasi WhatsApp, check-in GPS, aplikasi mobile.

**Kriteria selesai rilis 1:**

- [ ] HR bisa import karyawan dari Excel dan mengatur atasan serta role.
- [ ] Template KPI keempat departemen terisi dari seed dan bobotnya tervalidasi 100%.
- [ ] Karyawan input realisasi, manajer validasi, skor dan predikat terhitung benar (lolos test).
- [ ] Ketiga dashboard tampil sesuai role, laporan bisa diekspor ke Excel dan PDF.
- [ ] Karyawan tidak bisa membuka data karyawan lain (diuji dengan test otorisasi).

**Kriteria selesai rilis 2:**

- [ ] Pengajuan berjalan dari karyawan → atasan → HR, termasuk tolak dan batal.
- [ ] Hari kerja dihitung tanpa akhir pekan dan hari libur; saldo terpotong dan kembali dengan benar.
- [ ] Notifikasi email terkirim di setiap perubahan status.
- [ ] Kalender tim menampilkan cuti yang disetujui.
