# SiKerja

Aplikasi internal untuk mengukur kinerja karyawan (KPI & penilaian) serta mengajukan izin dan cuti,
untuk tim IT Support, Customer Service, Sales & Marketing, dan Developer.

**Status:** Rilis 1 (KPI & penilaian) dan Rilis 2 (izin & cuti, notifikasi) selesai; Rilis 3 belum dikerjakan —
lihat [docs/PLAN.md](docs/PLAN.md) untuk rincian yang selesai/belum.

| Dokumen | Isi |
| --- | --- |
| [docs/SPEC.md](docs/SPEC.md) | Spesifikasi lengkap: role, modul, KPI default, rumus skor, aturan cuti, halaman, model data |
| [docs/api.yaml](docs/api.yaml) | Kontrak API (OpenAPI 3) — dijaga sinkron dengan router lewat test |
| [docs/DECISIONS.md](docs/DECISIONS.md) | Asumsi & keputusan yang tidak tertulis di SPEC |
| [docs/PLAN.md](docs/PLAN.md) | Checklist Rilis 1 + ringkasan akhir + cara mencoba per role |
| [docs/mockup/](docs/mockup/index.html) | Mockup HTML statis (buka `index.html` di browser) |
| [CLAUDE.md](CLAUDE.md) | Stack dan aturan kode untuk Claude Code |

**Stack:** Vue 3 + TypeScript + Tailwind (frontend) · Go + Gin + GORM (backend) · MySQL 8 · Redis · Mailpit (email lokal).

## Prasyarat

- Docker + Docker Compose v2
- (opsional, untuk test/lint di luar Docker) Go 1.26+, Node 22+, `make`

## Menjalankan (satu perintah)

```bash
git clone https://github.com/retechno/sikerja.git
cd sikerja
make dev          # membuat .env, membangun & menjalankan semua service, mengisi data demo
```

`make dev` menjalankan: MySQL, Redis, Mailpit, API (hot reload Air), worker, dan web (Vite). Setelah siap:

| Layanan | Alamat |
| --- | --- |
| Aplikasi web | http://localhost:5173 |
| API (health check) | http://localhost:8080/healthz — ubah port host lewat `API_PORT` di `.env` bila bentrok |
| Email (Mailpit, untuk tautan lupa password) | http://localhost:8025 |
| MySQL / Redis dari host | `127.0.0.1:3307` / `127.0.0.1:6380` |

Tanpa `make` (mis. Windows polos), jalankan perintah yang setara:

```bash
cp .env.example .env
docker compose up --build -d
docker compose run --rm api go run ./cmd/seed     # data awal + demo (aman diulang; tidak menimpa)
docker compose logs -f api web
```

Perintah lain: `make down` (berhenti), `make fresh` (kosongkan & isi ulang data demo — **menghapus data**),
`make migrate`, `make test`, `make lint`, `make help`.

## Akun demo

Password semua akun: **`password`**. Data demo berisi 6 periode bulanan terakhir (5 ditutup + 1 aktif)
lengkap dengan realisasi dan penilaian agar dasbor langsung terisi.

| Role | Nama | Email | Mencoba apa |
| --- | --- | --- | --- |
| Direktur | Hendra Gunawan | `hendra.gunawan@sikerja.test` | Dasbor perusahaan, laporan, menilai manajer, finalisasi |
| HR / Admin | Sinta Maharani | `sinta.maharani@sikerja.test` | Karyawan & import Excel, template KPI, periode, laporan + ekspor, pengaturan, finalisasi |
| Manajer IT Support | Bayu Pratama | `bayu.pratama@sikerja.test` | Dasbor tim, validasi realisasi, menilai anggota |
| Karyawan IT Support | Rina Wulandari | `rina.wulandari@sikerja.test` | Dasbor, input realisasi + bukti, self-assessment (angka sama dengan mockup H-05) |
| Karyawan IT Support | Fajar Nugroho | `fajar.nugroho@sikerja.test` | Mencoba membuka data Rina → ditolak (403) |

Manajer lain: `indah.permatasari@` (Customer Service), `rudi.hartono@` (Sales & Marketing),
`satria.nugraha@` (Developer). Setiap departemen punya 5 karyawan; daftar lengkap dicetak oleh
`go run ./cmd/seed` dan ada di [docs/PLAN.md](docs/PLAN.md). Semua email berakhiran `@sikerja.test`.

Tautan **lupa password** dikirim lewat email ke Mailpit (http://localhost:8025).

## Struktur repo

```
backend/     Go: cmd/{api,worker,seed,migrate}, internal/{config,handler,middleware,service,repository,model,...}, migrations/
frontend/    Vue 3 + TS: src/{pages,components,api,stores,router,lib}
docs/        SPEC, api.yaml, DECISIONS, PLAN, mockup
docker-compose.yml  Makefile  .env.example
```

## Pengembangan & kualitas

```bash
make test        # go test (SQLite in-memory + miniredis, tanpa Docker) + Vitest
make lint        # gofmt, go vet, golangci-lint + ESLint, Prettier, vue-tsc
```

Hal yang diuji otomatis: rumus capaian/skor/predikat (termasuk batas 89,99/90 dan realisasi 0),
contoh H-07 (KPI 95,7 + perilaku 88,0 → 93,4 Istimewa), otorisasi per role dan kepemilikan data
(karyawan tidak bisa membuka data orang lain, manajer tidak bisa membuka tim lain), periode
tertutup tidak bisa diubah, rate limit login, CSRF, unduhan bukti, import Excel, ekspor, dan
kesesuaian `docs/api.yaml` dengan router. Skema model GORM juga dicek sama dengan migrasi SQL.

Menjalankan backend/frontend langsung dari host (tanpa container API/web):

```bash
docker compose up -d mysql redis mailpit
cd backend  && DB_DSN='sikerja:sikerja@tcp(127.0.0.1:3307)/sikerja?charset=utf8mb4&parseTime=true&loc=Local' \
              REDIS_ADDR=127.0.0.1:6380 SMTP_HOST=127.0.0.1 go run ./cmd/seed && go run ./cmd/api
cd frontend && npm install && VITE_API_PROXY=http://localhost:8080 npm run dev
```

## Keamanan singkat

Password bcrypt · JWT access 15 menit + refresh 7 hari di cookie httpOnly (SameSite=Lax, Secure di production),
refresh dirotasi dan bisa dicabut · rate limit login 5× gagal → jeda 1 menit (Redis) · proteksi CSRF
(header `X-Requested-With`) · otorisasi role + cek kepemilikan data di service · lampiran di storage
privat, jenis berkas dicek dari isinya, diunduh lewat endpoint yang memeriksa hak akses · audit log
untuk validasi KPI, perubahan nilai, finalisasi, dan buka/tutup periode.

Di production wajib mengisi `JWT_SECRET` (≥ 32 karakter acak) dan `COOKIE_SECURE=true`.
