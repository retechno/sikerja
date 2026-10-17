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
- **Skor KPI di layar = skor sementara**: semua realisasi yang tidak ditolak dihitung (mockup H-05/H-07 menjumlahkan
  juga indikator "menunggu validasi" dan menandainya "belum valid"). **Finalisasi baru boleh bila semua indikator
  berstatus `validated`**, jadi skor yang dibekukan selalu berasal dari data tervalidasi. Indikator tanpa realisasi
  (atau ditolak) = capaian 0, termasuk arah "lebih rendah" (bukan 120); realisasi 0 yang *diinput* pada arah
  lebih rendah = 120.
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

## Tambahan (alur KPI & penilaian)

- **Satu berkas bukti per realisasi** (kolom `evidence_path` tunggal di SPEC); mengunggah lagi menggantikan yang lama.
  Mockup H-05 menampilkan beberapa berkas — ditunda. Jenis berkas ditentukan dari isi (sniffing), bukan ekstensi.
  Mengganti bukti pada realisasi yang sudah divalidasi mengembalikannya ke `submitted`.
- **Nilai atasan, komentar atasan, rencana pengembangan, skor perilaku, skor akhir, dan predikat disembunyikan dari
  karyawan sampai penilaiannya `final`** (catatan di mockup H-07: "Terlihat oleh karyawan setelah penilaian final").
- **Penilaian yang sudah `final` terkunci** (realisasi, validasi, nilai) walau periodenya masih aktif.
- **Periode tertutup → HTTP 423 (Locked)**; periode draft → 400.
- **Direktur menilai/memvalidasi Manajer** (atasan langsung = `manager_id`); Manajer tidak bisa menilai dirinya sendiri.
  HR tidak memvalidasi KPI (hanya melihat dan memfinalisasi).
- **HR dan Direktur tidak punya penilaian** (tanpa departemen/template); mereka tidak muncul di daftar tim.
- **Token akses tidak membawa role**: Actor dimuat dari DB tiap request (3 query ringan) agar perubahan role,
  departemen, atau penonaktifan langsung berlaku.
- **Folder `dto/`** dari CLAUDE.md tidak dibuat; struct input/output didefinisikan di dekat service yang memakainya.

## Tambahan (infrastruktur)

- **Go 1.26, bukan 1.23**: dependensi terbaru (maroto v2.4, go-redis v9.23, x/crypto, validator, migrate) mensyaratkan Go ≥ 1.25/1.26,
  dan toolchain lokal sudah 1.26. `go.mod` = `go 1.26.1`; image Docker `golang:1.26`. Versi di SPEC/CLAUDE.md ("Go 1.23") perlu diperbarui.
- **Vite memproxy `/api` ke API** di mode dev sehingga browser dan API satu origin (cookie SameSite=Lax jalan tanpa CORS). CORS tetap aktif
  untuk origin di `CORS_ORIGINS`.
- **Port host**: web 5173, API 8080, MySQL 3307, Redis 6380, Mailpit UI 8025 — MySQL/Redis digeser agar tidak bentrok dengan instalasi lokal.
- **`.gitattributes`** memaksa LF supaya diff bersih di Windows.

## Tambahan (Rilis 2 — izin & cuti, notifikasi)

**Perlu dicek ulang HR / pemilik produk**
- **Hari libur 2026 diisi dari ingatan atas SKB Tiga Menteri, bukan salinan resmi** (`service.OfficialHolidays`, juga di-seed dan
  tombol "Impor dari daftar resmi"): libur nasional + cuti bersama (mis. Idul Fitri 21–22 Mar, cuti bersama 20/23/24 Mar; Idul Adha
  27 Mei; Tahun Baru Islam 16 Jun; Maulid 25 Agu; cuti bersama Natal 24 Des). **HR wajib mencocokkan dengan SKB terbaru** sebelum
  dipakai — tanggal yang keliru langsung mengubah hitungan hari kerja. Tahun lain belum ada daftar bawaan (tambah manual).
- **Kuota cuti khusus**: menikah maks 3 hari kerja, duka 2, melahirkan 65 hari kerja (≈ 3 bulan). Angka ini asumsi; ikuti kebijakan
  perusahaan dan UU Ketenagakerjaan yang berlaku. Disimpan sebagai `max_days_per_request` (batas per pengajuan), bukan saldo tahunan.
- **Batas pembatalan = hari sebelum tanggal mulai** (mockup H-09: "dibatalkan sampai 29 Okt" untuk cuti mulai 30 Okt). SPEC hanya
  menulis "selama tanggal mulai belum lewat". Pembatalan hanya oleh pemilik, boleh setelah disetujui (saldo dikembalikan).
- **Pengajuan boleh mundur maksimal 7 hari** (mis. cuti sakit yang dilaporkan belakangan); lebih dari itu ditolak.

**Model & aturan**
- **Kolom tambahan di `leave_types`** (di luar SPEC): `attachment_min_days` (lampiran wajib bila hari > N; sakit = 1),
  `min_tenure_months` (cuti tahunan 12), `max_days_per_request`, `sort`, `is_active`. `annual_quota` NULL = tidak dibatasi.
  `deducts_balance` = saldo tahunan dikelola & dipotong saat disetujui (cuti tahunan **dan izin**, yang punya kuota sendiri);
  jenis lain hanya dicatat. Cuti khusus dipecah tiga (menikah/melahirkan/duka) mengikuti mockup H-16.
- **Kolom tambahan di `leave_requests`**: `number` (mis. `CT-2026-0142`: kode jenis + tahun + id), `half_session` (pagi/siang),
  `deducted` (penanda saldo sudah dipotong agar pengembalian tepat), `cancelled_at`. Dua izin setengah hari di tanggal sama boleh
  bila sesinya berbeda; selain itu tanggal tumpang tindih ditolak.
- **Tabel tambahan `app_settings`** (saklar notifikasi H-16). Default semua aktif; mematikan saklar menghentikan notifikasi dalam
  aplikasi **dan** email untuk jenis itu.
- **Masa kerja ≥ 12 bulan** dicek pada **tanggal mulai** cuti (bukan 1 Januari). Saldo dibuat malas (saat pertama dibaca/diajukan) per
  tahun, bukan lewat job 1 Januari; sisa tahun lalu tidak diakumulasi.
- **Saldo "tersedia" = kuota − terpakai − menunggu** (mockup H-08). Pengajuan baru diperiksa terhadap angka itu; saat persetujuan
  terakhir saldo diperiksa **lagi di bawah kunci baris** (`SELECT … FOR UPDATE`) sehingga kuota yang dikecilkan HR atau dua persetujuan
  bersamaan tidak membuat saldo minus atau terpotong dua kali (409 bila tak cukup; HR bisa menolak).
- **Pengajuan tidak boleh melintasi pergantian tahun** untuk jenis berkuota (saldo per tahun); ajukan terpisah.
- **Alur persetujuan**: karyawan → atasan langsung (`manager_id`, atau manajer departemen) → HR; Manajer → Direktur → HR; **Direktur
  → langsung HR; HR → langsung Direktur** (HR tidak menyetujui pengajuannya sendiri; SPEC diam soal ini). Setengah hari: tahap pertama
  saja. Tidak ada yang bisa menyetujui pengajuannya sendiri. Penolakan wajib beralasan.
- **Pengajuan orang di luar jangkauan menjawab 404** (bukan 403) agar keberadaannya tidak bocor; penyetuju yang bukan tahapnya → 403.
- **Kalender tim terbuka untuk semua role** dan memperlihatkan nama + jenis (termasuk "sakit", sesuai mockup H-11), tidak pernah alasan
  atau lampiran. Default memilih departemen sendiri.
- **Lampiran** (surat dokter/tugas): satu berkas per pengajuan, PDF/JPG/PNG ≤ 5 MB, jenis dicek dari isi; diunduh lewat endpoint
  dengan hak akses yang sama dengan detail pengajuan.
- **Calon pengganti** = rekan aktif satu departemen (endpoint `GET /leave/substitutes`, karena `/employees` khusus HR).

**Notifikasi**
- `service.Notifier` + `service.Channel`: notifikasi dalam aplikasi selalu disimpan; kanal tambahan (email sekarang) dipasang di
  `app.NewNotifier`. **WhatsApp** cukup mengimplementasikan `Channel` (`Deliver`) dan mendaftarkannya di sana.
- Email dikirim lewat antrean asynq (`email:notice`) oleh worker; pengiriman **best-effort** — gagal dicatat di log, tidak
  menggagalkan operasi bisnis (retry otomatis 5×).
- Peristiwa: pengajuan baru → penyetuju tahap berjalan; disetujui tahap 1 → pengaju (kemajuan) + penyetuju tahap 2; disetujui
  penuh / ditolak (beserta alasan) → pengaju; dibatalkan → penyetuju yang sedang/sudah menyetujui; periode dibuka → semua karyawan
  aktif (bukan saat dibuka ulang); pengingat tenggat → lihat di bawah.
- **Pengingat tenggat** dijalankan scheduler asynq di worker tiap hari 08.00 (`APP_TIMEZONE`, default Asia/Jakarta) untuk periode aktif
  yang `end_date`-nya 3 atau 1 hari lagi: karyawan yang self-assessment belum dikirim, dan manajer yang masih punya anggota belum
  dinilai. Kunci Redis per periode per hari mencegah kirim ganda. Tenggat = `end_date` periode.
- Dasbor Karyawan/Manajer/Direktur mengambil saldo & antrean dari `GET /leave/summary` (tidak di-cache 5 menit seperti dasbor KPI).
- Ponsel: bilah tab bawah (Beranda, KPI/Setuju, Cuti, Kalender, Profil) sesuai M08; sidebar tetap jadi drawer.

**Seeder**: `go run ./cmd/seed` pada database Rilis 1 yang sudah terisi kini hanya menambahkan data izin & cuti (`EnsureLeave`) bila
belum ada. Contoh pengajuan Rina Wulandari dibuat relatif terhadap hari ini agar angkanya sama dengan mockup (cuti tahunan 2
terpakai + 2 menunggu, izin 1,5, sakit 2) saat dijalankan sekitar Okt 2026.

## Tambahan (jadwal kerja & izin per jam)

Permintaan: hari kerja bisa termasuk Sabtu dan ada hitungan jam (Sen–Jum 08.00–17.00; Sen–Jum 09.00–17.00 + Sabtu 09.00–15.00),
diatur per departemen, shift untuk CS menyusul. Jawaban pemilik produk: jadwal **per departemen**; izin sebagian hari = **jam kerja hari itu
sebagai penyebut** (saldo tetap dalam hari).

- **Tabel `work_schedules` + `work_schedule_days`** (weekday 0 = Minggu … 6 = Sabtu, jam mulai/selesai; hari tanpa baris = libur mingguan) dan
  `departments.schedule_id` (NULL = jadwal default). Migrasi 000003 langsung mengisi dua jadwal bawaan: **"Jam kantor" (Sen–Jum 08.00–17.00, default)**
  dan **"Jam kantor + Sabtu" (Sen–Jum 09.00–17.00, Sab 09.00–15.00)**. **Saya tidak menebak departemen mana yang memakai jadwal kedua** — semua departemen
  tetap di jadwal default sampai HR memasangnya di H-16 → Departemen & jabatan → Jadwal kerja. Tab "Jam kerja" untuk membuat/ubah jadwal; shift CS kelak
  cukup berupa jadwal baru yang dipasang ke departemen CS.
- **Hari kerja mengikuti jadwal departemen pengaju** (HR/Direktur tanpa departemen memakai jadwal default). Sabtu terhitung 1 hari kerja hanya bila jadwalnya memuat
  Sabtu; hari libur nasional/cuti bersama tetap tidak dihitung. Satu titik penentu (`scheduleFor`) sehingga jadwal per karyawan/shift bisa ditambah tanpa mengubah
  pemanggil. Mengubah jadwal **tidak menghitung ulang** pengajuan yang sudah ada.
- **Jam istirahat dikurangi** (permintaan pemilik produk: istirahat 12.00–13.00). Istirahat adalah atribut jadwal (`work_schedules.break_start/break_end`,
  bawaan 12.00–13.00 pada kedua jadwal, bisa diubah HR per jadwal atau dikosongkan). Jam kerja efektif hari itu = selesai − mulai − irisan dengan istirahat
  (Sen–Jum 08–17 = 8 jam; Sabtu 09–15 = 5 jam).
- **Izin sebagian hari = jam mulai–selesai** pada satu hari kerja, di dalam jam kerja hari itu: `hari = jam efektif izin ÷ jam efektif hari itu`, dibulatkan
  2 desimal (08.00–10.00 dari 8 jam = 0,25; Senin 11.00–14.00 = 2 jam efektif karena istirahat terpotong = 0,25; Sabtu 09.00–11.30 dari 5 jam = 0,5).
  "Paruh pagi/siang" membagi jam **efektif** hari itu sama panjang (jadwal 08–17: pagi 08.00–12.00, siang 13.00–17.00; Sabtu 09–15: pagi 09.00–11.30,
  siang 11.30–15.00); server yang menghitung batasnya. Minimal 15 menit efektif (rentang yang hanya berisi istirahat ditolak); seluruh jam kerja ditolak
  (ajukan sebagai satu hari penuh). Hanya jenis dengan "boleh setengah hari" (Izin) yang bisa sebagian hari. Dua izin di tanggal sama boleh bila jamnya
  tidak beririsan. Tetap satu tahap persetujuan (atasan langsung).
- Kolom saldo/hari dilebarkan ke `DECIMAL(6,2)` (sebelumnya 1 desimal) agar 0,22 tersimpan; `leave_requests.start_time/end_time` tambahan, `half_session`
  hanya terisi bila rentang tepat paruh pertama/kedua. Data lama (hanya sesi) tetap terbaca.
- Konsekuensi: saldo izin bisa berpecahan (mis. 0,22); tampilan memakai koma dan hingga 2 desimal.
