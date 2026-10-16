# Keputusan & asumsi

Setiap hal yang tidak tertulis (atau ambigu) di SPEC/CLAUDE.md dicatat di sini.
Format: **keputusan** — alasan.

## Lingkungan & proses

- **Kerja di branch `feat/rilis-1`, bukan langsung ke `main`** — push ke branch itu; merge lewat PR.
- **`docs/SPEC.md` lokal yang belum ter-commit diganti versi `origin/main`** — versi remote
  lebih baru (ada rumus bobot periode, kuota izin 6 hari, CLAUDE.md, mockup). Salinan lama
  disimpan di scratchpad sesi, tidak masuk repo.
- **`make` dan `golangci-lint` tidak terpasang di mesin pengembang (Windows)** — Makefile tetap
  ditulis sesuai CLAUDE.md. Verifikasi dilakukan dengan menjalankan perintah yang sama
  secara langsung (`go test ./...`, `go vet`, `npm run test`, dst.) atau via Docker.
- **Test backend memakai SQLite pure-Go (`glebarez/sqlite`) + miniredis** — tidak ada gcc di
  mesin ini dan test tidak boleh bergantung pada MySQL/Redis yang berjalan. Skema test dibuat
  dari model (`AutoMigrate`); skema produksi dari `backend/migrations/*.sql`. Ada test yang
  membandingkan kolom model dengan migrasi agar tidak melenceng (lihat PLAN).

## Arsitektur

- **Prefix API `/api/v1`** (CLAUDE.md), respons `{data, meta}` / `{error:{code,message,fields}}`.
- **Repository layer** dibuat sebagai interface per agregat kecil; service bergantung pada
  interface, implementasi GORM. Test service memakai SQLite lewat implementasi GORM yang sama.
- **Skor memakai `shopspring/decimal`** (CLAUDE.md). Pembulatan: 2 desimal (half-up) disimpan,
  tampil 1 desimal.
- **Worker asynq (`cmd/worker`)**: email lupa-password dikirim lewat antrian asynq. Notifikasi
  periode/pengingat tenggat adalah modul 7 (Rilis 2) dan tidak dikerjakan.
- **CSRF**: cookie `SameSite=Lax` + semua request mutasi wajib header `X-Requested-With: XMLHttpRequest`
  (tidak bisa dikirim lintas-origin tanpa preflight CORS). CORS hanya untuk origin web.
- **shadcn-vue** dipakai secara terbatas; komponen UI utama (tombol, kartu, tabel, badge) ditulis
  sendiri dengan Tailwind mengikuti token warna mockup, supaya persis dengan mockup.

## Aturan bisnis yang diperjelas

- **Skor tidak dipotong di 100.** Capaian maks 120 sehingga skor KPI bisa >100; SPEC hanya
  menetapkan cap 120 per KPI, jadi skor akhir tidak dibatasi lagi. (Perlu dikonfirmasi pemilik produk.)
- **Hanya realisasi berstatus `validated` yang masuk skor KPI.** Indikator tanpa realisasi
  tervalidasi dihitung capaian 0 (termasuk arah "lebih rendah") dan ditandai "belum valid"
  di UI (sesuai H-07). Realisasi 0 *yang diinput* pada arah lebih rendah = 120.
- **Realisasi yang diinput manajer langsung berstatus `validated`.** Realisasi karyawan
  berstatus `submitted`; mengubah realisasi yang sudah divalidasi mengembalikannya ke `submitted`.
- **Manajer memvalidasi/menilai anggota yang `department_id`-nya sama dengan departemennya**
  (SPEC: "anggota departemennya"), bukan hanya bawahan langsung. Manajer tidak menilai dirinya
  sendiri; penilai manajer adalah Direktur (`manager_id`).
- **Alur status penilaian**: `draft` → `self_done` → `manager_done` → `final`. Manajer boleh menilai
  walau self-assessment belum diisi (agar tidak buntu); karyawan tidak bisa mengubah self-assessment
  setelah `manager_done`. Finalisasi (HR/Direktur) hanya dari `manager_done`, hanya saat periode aktif.
- **Skor perilaku butuh 5 aspek terisi oleh atasan**; selama belum, skor perilaku/akhir = kosong.
- **Skor KPI dihitung ulang dari indikator template aktif** saat penilaian dihitung; skor yang
  tersimpan di `appraisals` dibekukan saat finalisasi dan periode ditutup.
- **Predikat dihitung dari skor yang sudah dibulatkan 2 desimal** (89,99 → Baik; 90,00 → Istimewa).
- **Template KPI**: simpan ditolak jika total bobot ≠ 100 (toleransi 0,001). Indikator yang dihapus
  di-soft-delete agar realisasi lama tetap terbaca.
- **Periode**: `draft → active → closed`; HR bisa membuka kembali `closed → active` (diaudit).
  Boleh lebih dari satu periode aktif; `kpi_weight + behavior_weight` harus 100.
- **Seed periode**: 5 bulan terakhir sebelum bulan berjalan ditutup, bulan berjalan aktif.
- **Upload**: PDF/JPG/PNG maks 5 MB, diperiksa lewat ekstensi + sniffing konten; disimpan di
  `STORAGE_DIR` (di luar web root) dengan nama acak.
- **Rate limit login**: kunci per email (huruf kecil) di Redis; 5 kegagalan berturut-turut →
  429 dengan `Retry-After: 60`; berhasil me-reset hitungan.
- **Token reset password**: acak 32 byte, hanya hash SHA-256 yang disimpan, berlaku 1 jam, sekali pakai
  (tabel tambahan `password_resets`). Respons "lupa password" selalu 200 (tanpa membocorkan email).
- **Refresh token** disimpan (jti) di Redis agar bisa dicabut saat logout / ganti password; di-rotasi tiap refresh.
- **Notifikasi (lonceng)**: endpoint daftar/tandai-baca disediakan; pembuatan notifikasi
  otomatis ditunda ke Rilis 2.
- **H-16 Pengaturan**: Rilis 1 hanya tab Departemen dan Jabatan; jenis cuti, hari libur, notifikasi ditunda.
- **Sidebar/halaman cuti (H-08–H-11)** tidak dibuat; menu cuti tidak ditampilkan (bukan tombol mati).
  Kartu saldo cuti di dashboard karyawan juga ditunda.
- **Dashboard manajer**: kotak "persetujuan menunggu" ditunda ke Rilis 2.
- **Data demo**: email `nama.depan@sikerja.test`, password `password` untuk semua akun (lihat README).
- **Cache dashboard Redis 5 menit** per (role, scope, periode); dibuang saat ada perubahan realisasi/penilaian/periode.

## Tambahan (lapisan data & skor)

- **`employees.department_id` dan `positions.department_id` boleh NULL** — Direktur dan HR tidak
  termasuk dalam 4 departemen di SPEC; daripada menambah departemen kelima, keduanya tanpa departemen
  (jabatan "Direktur"/"Staf HR" lintas departemen). Mereka tidak punya template KPI maupun penilaian.
- **Pembulatan bertahap 2 desimal**: capaian per KPI, skor KPI, skor perilaku, dan skor akhir masing-masing
  dibulatkan half-up ke 2 desimal (itu nilai yang disimpan di DB), tampil 1 desimal. Dengan data mockup H-05
  skor KPI = 95,65 → tampil 95,7 persis seperti mockup; tanpa pembulatan antara hasilnya 95,649 → 95,6.
- **Repository + `Store.Tx`**: service menerima `repository.Store` dan memakai `store.R()` untuk query biasa atau
  `store.Tx(func(Repos) error)` untuk transaksi. Fungsi repository belum menerima `context.Context`.
- **Tabel `activity_log` bernama tunggal** (sesuai SPEC) — GORM defaultnya jamak, dipaksa lewat `TableName()`.
- **Test paritas model ↔ migrasi** (`internal/model/model_test.go`) membandingkan kolom GORM dengan `CREATE TABLE`
  di `000001_init.up.sql`; test ini sudah menangkap satu kesalahan nama tabel.
