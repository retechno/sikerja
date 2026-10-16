# Prompt Claude Code — SiKerja

Cara pakai: clone repo, masuk ke folder repo, jalankan `claude`, lalu paste prompt
satu per satu. Cek hasil tiap tahap sebelum lanjut ke prompt berikutnya.
Claude Code otomatis membaca `CLAUDE.md` (stack & aturan), jadi prompt di bawah
fokus pada pekerjaan tiap tahap.

---

## Prompt 1 — Fondasi proyek + Rilis 1 (KPI & penilaian)

```
Baca CLAUDE.md, docs/SPEC.md seluruhnya, dan lihat docs/mockup/ sebelum menulis kode.
Kerjakan Rilis 1 (modul 1–5 di SPEC). Jangan berhenti untuk bertanya; catat asumsi
di docs/DECISIONS.md.

1. Buat docs/PLAN.md berisi checklist pekerjaan Rilis 1, kerjakan berurutan, centang
   yang selesai.
2. Setup monorepo sesuai CLAUDE.md: backend/, frontend/, docker-compose.yml, Makefile,
   .env.example, README (cara install, menjalankan, akun demo). Pastikan `make dev`
   menjalankan semuanya.
3. Migrasi + model untuk semua tabel di "Model data" KECUALI leave_* dan holidays.
4. Auth: login, refresh, logout, lupa password (token via email), ganti password,
   rate limit login 5x gagal → jeda 1 menit (Redis).
5. Otorisasi role + kepemilikan data di setiap endpoint (lihat CLAUDE.md).
6. Seeder (go run ./cmd/seed):
   - 4 departemen, jabatan, 1 direktur, 1 HR, 1 manajer + 5 karyawan per departemen
     (nama Indonesia), password semua "password".
   - Template KPI default persis seperti tabel "KPI default per departemen".
   - 5 aspek perilaku.
   - 6 periode bulanan terakhir (5 ditutup + 1 aktif) dengan realisasi & penilaian
     acak yang realistis agar dashboard langsung terisi.
7. Service: KpiScoreService (capaian per KPI, cap 120, arah higher/lower, realisasi 0
   untuk lower = 120, skor KPI berbobot) dan AppraisalService (skor perilaku, skor
   akhir dengan bobot periode, predikat). Rumus PERSIS seperti "Aturan bisnis".
8. Endpoint (tulis di docs/api.yaml) + halaman Vue H-01 s/d H-07 dan H-12 s/d H-16
   (bagian cuti di H-16 ditunda), mengikuti mockup.
9. Import karyawan dari Excel (H-12) + unduh template; export laporan H-15 ke Excel
   dan PDF.
10. Upload bukti KPI ke storage privat + endpoint unduh yang mengecek hak akses.
11. Audit log untuk validasi KPI, perubahan nilai, finalisasi, buka/tutup periode.
12. Test:
    - Backend: capaian, skor, predikat (termasuk batas 89,99/90 dan realisasi 0),
      otorisasi (karyawan tidak bisa buka data orang lain, manajer tidak bisa buka
      tim lain), periode tertutup tidak bisa diubah. Contoh H-07 di SPEC
      (KPI 95,7 + perilaku 88,0 → 93,4 Istimewa) wajib jadi test case.
    - Frontend: tabel KPI dan form penilaian.
13. `make test` dan `make lint` sampai bersih. Commit kecil-kecil, lalu push.
14. Di akhir, tulis ringkasan di docs/PLAN.md: yang selesai, yang belum, dan cara
    mencoba per role (akun demo).
```

---

## Prompt 2 — Rilis 2 (izin & cuti)

```
Lanjutkan SiKerja. Baca CLAUDE.md, docs/SPEC.md (fokus modul 6 Izin & cuti, modul 7
Notifikasi, "Aturan bisnis" → Saldo cuti & Alur persetujuan, halaman H-08 s/d H-11
dan H-16), docs/PLAN.md, docs/DECISIONS.md, docs/api.yaml, dan mockup terkait.

1. Migrasi + model: leave_types, leave_balances, leave_requests, leave_approvals,
   holidays. Seeder: jenis cuti sesuai SPEC (tahunan 12, izin 6, sakit, dinas luar,
   cuti khusus), hari libur nasional & cuti bersama tahun berjalan (catat di
   DECISIONS.md agar HR mengecek ulang), saldo awal semua karyawan, contoh pengajuan.
2. WorkingDayService: hari kerja tanpa Sabtu, Minggu, dan tabel holidays; setengah
   hari = 0,5.
3. LeaveService dengan transaksi DB + row lock saat memotong saldo: validasi saldo &
   tumpang tindih, alur status Menunggu Atasan → Menunggu HR → Disetujui / Ditolak /
   Dibatalkan, potong saldo saat disetujui penuh, kembalikan saat dibatalkan, pengaju
   Manajer → tahap 1 oleh Direktur, izin setengah hari cukup atasan langsung.
4. Endpoint (perbarui docs/api.yaml) + halaman Vue H-08 (jumlah hari kerja dihitung
   API secara live), H-09, H-10, H-11, tab cuti & hari libur di H-16, kartu saldo &
   persetujuan menunggu di dashboard. Ikuti mockup, termasuk M08 untuk ponsel.
5. Notifikasi: tabel notifications + email via asynq. Event: pengajuan baru,
   disetujui, ditolak, periode dibuka, pengingat tenggat (scheduler harian).
   Interface Notifier agar WhatsApp mudah ditambah. Lonceng di header menampilkan
   yang belum dibaca.
6. Test: hari kerja (melewati akhir pekan & libur), saldo kurang, tumpang tindih,
   alur setuju/tolak/batal, pengembalian saldo, dua persetujuan bersamaan tidak
   memotong saldo dua kali.
7. `make test` & `make lint` bersih, update docs/PLAN.md, commit kecil-kecil, push.
```

---

## Prompt 3 — Review & persiapan deploy

```
Review seluruh kode SiKerja terhadap docs/SPEC.md dan CLAUDE.md:
1. Buat docs/REVIEW.md: tabel setiap fitur & halaman di SPEC → status (selesai /
   sebagian / belum) + lokasi file.
2. Cek keamanan: semua endpoint lewat auth + cek role/kepemilikan, rate limit login,
   cookie httpOnly + Secure di production, CORS hanya untuk domain web, lampiran tidak
   bisa diakses tanpa izin, tidak ada N+1 query di dashboard (Preload/join + cache
   Redis 5 menit).
3. Perbaiki semua yang "sebagian"/"belum" untuk Rilis 1 & 2.
4. GitHub Actions: golangci-lint, go test (service MySQL & Redis), npm ci, lint,
   vitest, build frontend.
5. Dockerfile multi-stage untuk api/worker (binary statis, image distroless) dan web
   (build Vite disajikan Nginx yang mem-proxy /api). docker-compose.prod.yml +
   docs/DEPLOY.md (Ubuntu, migrasi saat deploy, backup MySQL harian, rotasi log).
6. Commit & push.
```
