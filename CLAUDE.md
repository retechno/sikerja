# SiKerja — panduan untuk Claude Code

SiKerja adalah aplikasi internal untuk KPI, penilaian kinerja, serta izin & cuti karyawan
di perusahaan IT penjual PMS hotel (departemen: IT Support, Customer Service,
Sales & Marketing, Developer).

## Sumber kebenaran

- `docs/SPEC.md` — spesifikasi lengkap. Kode halaman (H-01 s/d H-16), nama tabel,
  rumus skor, predikat, dan aturan cuti di sana WAJIB diikuti.
- `docs/mockup/` — mockup HTML statis per halaman (buka `index.html`). Ikuti susunan,
  warna, dan komponennya. Data di mockup hanya contoh, jangan di-hard-code.
- `docs/api.yaml` — kontrak API (OpenAPI 3). Perbarui setiap kali endpoint berubah.
- `docs/DECISIONS.md` — catat setiap asumsi atau keputusan yang tidak tertulis di SPEC.
- `docs/PLAN.md` — checklist pekerjaan; centang item yang selesai.

Jika spesifikasi ambigu: pilih opsi paling masuk akal, catat di `docs/DECISIONS.md`,
lanjutkan. Jangan berhenti untuk bertanya.

## Stack

**Backend (`backend/`)** — Go 1.23, Gin, GORM (MySQL 8), golang-migrate
(`backend/migrations/*.sql`), go-playground/validator, golang-jwt/jwt v5, bcrypt,
Redis + hibiken/asynq, excelize, maroto, wneessen/go-mail, zerolog,
shopspring/decimal untuk angka skor.

Struktur:
```
backend/
  cmd/api        server HTTP
  cmd/worker     worker asynq (notifikasi, jadwal harian)
  cmd/seed       data awal & data demo
  internal/config, handler, middleware, service, repository, model, dto
  migrations/
```
Handler tipis → logika bisnis di `service` → akses DB hanya di `repository`
(lewat interface agar service mudah di-mock).

**Frontend (`frontend/`)** — Vue 3 + TypeScript (`<script setup>`), Vite, Vue Router,
Pinia, Tailwind CSS, shadcn-vue, vue-chartjs, VeeValidate + Zod, axios.
Halaman di `src/pages/<modul>/` (dashboard, kpi, appraisal, leave, master, report,
settings); klien HTTP di `src/api/`.

**Infra** — `docker-compose.yml` (api, worker, web, mysql, redis, mailpit), Air untuk
hot reload Go, `Makefile` dengan target `dev`, `test`, `lint`, `migrate`, `seed`.

## Aturan API

- Semua endpoint di bawah `/api/v1`, JSON.
- Respons sukses `{ "data": ..., "meta": ... }`; error
  `{ "error": { "code", "message", "fields" } }`.
- Auth: access token JWT 15 menit + refresh token 7 hari, keduanya cookie httpOnly,
  SameSite=Lax, Secure di production. Jangan simpan token di localStorage.
- Otorisasi di SETIAP endpoint: middleware role (employee, manager, hr, director)
  + cek kepemilikan di service (karyawan hanya data sendiri, manajer hanya timnya).
- Lampiran disimpan di luar web root; diunduh lewat endpoint yang mengecek hak akses.
  Maks 5 MB, PDF/JPG/PNG.
- Perubahan penting (validasi KPI, nilai, finalisasi, buka/tutup periode,
  persetujuan cuti) dicatat di tabel `activity_log`.

## UI

- Semua teks berbahasa Indonesia. Tanggal `16 Okt 2026`, desimal pakai koma, skor 1 desimal.
- Font Plus Jakarta Sans. Warna: utama `#0E5A52`, aksen `#E8A33D`, sidebar `#0F2421`,
  latar `#F4F5F1`, teks `#18201E`, teks sekunder `#5B6662`.
- Layout: sidebar kiri sesuai role, header dengan pemilih periode, notifikasi, profil.
  Di ponsel sidebar jadi drawer. Target sentuh minimal 44 px.

## Kualitas

- Backend: table-driven test (go test + testify) untuk semua rumus dan aturan bisnis.
- Frontend: Vitest + Vue Test Utils untuk komponen penting.
- `make test` dan `make lint` harus bersih sebelum commit.
- Commit kecil dengan format `feat(kpi): ...`, `fix(leave): ...`, `test: ...`, `docs: ...`.
