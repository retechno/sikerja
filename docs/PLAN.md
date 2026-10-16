# Rencana Rilis 1 — KPI & penilaian

Centang `[x]` saat selesai. Urutan = urutan pengerjaan.

## A. Fondasi
- [x] 1. PLAN.md + DECISIONS.md
- [ ] 2. Monorepo: `backend/`, `frontend/`, `docker-compose.yml`, `Makefile`, `.env.example`, README
- [ ] 3. Backend: config, DB, Redis, logger, router, format respons/error
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
- [ ] 16. Seeder `cmd/seed` (departemen, user, template, aspek, 6 periode + data acak)
- [ ] 17. `docs/api.yaml`
- [ ] 18. Test otorisasi + periode tertutup

## C. Frontend
- [ ] 19. Scaffold Vue 3 + TS + Tailwind + router + Pinia + axios, layout (sidebar/header/drawer)
- [ ] 20. H-01 Login (+ lupa/reset password, ganti password, profil)
- [ ] 21. H-02/H-03/H-04 Dashboard
- [ ] 22. H-05 KPI Saya, H-06 Penilaian Tim, H-07 Form Penilaian
- [ ] 23. H-12 Karyawan (+ import), H-13 Template KPI, H-14 Periode
- [ ] 24. H-15 Laporan (+ export), H-16 Pengaturan (departemen & jabatan)
- [ ] 25. Test Vitest: tabel KPI & form penilaian

## D. Penutup
- [ ] 26. `make test` & `make lint` bersih
- [ ] 27. Push, ringkasan akhir di bawah

## Ringkasan akhir
_(diisi di akhir pekerjaan)_
