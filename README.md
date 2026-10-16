# SiKerja

Aplikasi internal untuk mengukur kinerja karyawan (KPI & penilaian) serta mengajukan
izin dan cuti, untuk tim IT Support, Customer Service, Sales & Marketing, dan Developer.

Status: tahap perencanaan — kode belum dibuat.

| Dokumen | Isi |
| --- | --- |
| [docs/SPEC.md](docs/SPEC.md) | Spesifikasi lengkap: role, modul, KPI default, rumus skor, aturan cuti, halaman, model data |
| [docs/mockup/](docs/mockup/index.html) | Mockup HTML statis 16 halaman + 1 tampilan ponsel |
| [docs/PROMPTS.md](docs/PROMPTS.md) | Prompt Claude Code untuk membangun aplikasi per tahap |
| [CLAUDE.md](CLAUDE.md) | Stack dan aturan kode yang dibaca otomatis oleh Claude Code |

**Stack:** Vue 3 + TypeScript (frontend), Go 1.23 + Gin + GORM (backend), MySQL 8, Redis.

## Melihat mockup

Buka `docs/mockup/index.html` di browser (cukup double-click file-nya).

## Mulai membangun

```bash
git clone https://github.com/retechno/sikerja.git
cd sikerja
claude
```

Lalu paste Prompt 1 dari [docs/PROMPTS.md](docs/PROMPTS.md).
