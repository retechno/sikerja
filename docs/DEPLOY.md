# Deploy SiKerja ke server Ubuntu

Panduan untuk satu server Ubuntu 22.04/24.04 dengan Docker Compose. Alur akhirnya:

```
Internet ──443──▶ Nginx host (TLS, certbot) ──▶ 127.0.0.1:8081 ──▶ container web (Nginx + build Vue)
                                                                       │  /api
                                                                       ▼
                                  container api (Go) ──▶ mysql · redis        container worker (email, pengingat 08.00)
                                  └── volume storage (lampiran, privat)
```

MySQL dan Redis **tidak** dibuka ke host; hanya container lain yang bisa mengaksesnya.
Image produksi: `api` dan `worker` berupa binary Go statis di atas `distroless` (tanpa shell, user non-root,
filesystem read-only); `web` berupa Nginx yang menyajikan hasil build Vite dan meneruskan `/api`.

Berkas terkait: `docker-compose.prod.yml`, `backend/Dockerfile`, `frontend/Dockerfile`, `.env.production.example`,
folder `deploy/` (skrip deploy, backup, restore, contoh Nginx host, cron, logrotate).

## 1. Persiapan server

Spesifikasi minimum: 2 vCPU, 2 GB RAM (disarankan 4 GB agar build image lega), disk 20 GB. DNS `A` record domain
(mis. `sikerja.perusahaan.id`) sudah mengarah ke IP server.

```bash
# Docker Engine + Compose v2 (repo resmi Docker)
sudo apt-get update && sudo apt-get install -y ca-certificates curl git nginx certbot python3-certbot-nginx ufw
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker "$USER"      # keluar-masuk sesi SSH agar berlaku

# firewall: hanya SSH + web
sudo ufw allow OpenSSH && sudo ufw allow 'Nginx Full' && sudo ufw --force enable

# rotasi log container (seluruh container di server ini)
sudo cp /opt/sikerja/deploy/docker-daemon.json /etc/docker/daemon.json   # setelah langkah 2
sudo systemctl restart docker
```

> `docker-compose.prod.yml` sudah membatasi log tiap service (5 × 10 MB), jadi `daemon.json` hanya pengaman untuk
> container lain di server yang sama. Port MySQL/Redis tidak dipublikasikan, jadi `ufw` tidak perlu membukanya.

## 2. Ambil kode & isi konfigurasi

```bash
sudo mkdir -p /opt/sikerja && sudo chown "$USER": /opt/sikerja
git clone https://github.com/retechno/sikerja.git /opt/sikerja && cd /opt/sikerja
git checkout main                       # atau tag rilis, mis. v1.0.0
cp .env.production.example .env && chmod 600 .env
```

Isi `.env` (seluruh nilai kosong wajib diisi; compose menolak start bila ada yang kurang):

| Variabel | Isi |
| --- | --- |
| `APP_URL` | URL publik, mis. `https://sikerja.perusahaan.id` (dipakai untuk CORS dan tautan di email) |
| `JWT_SECRET` | `openssl rand -hex 32` — **jangan diganti-ganti**: mengganti nilainya mengeluarkan semua pengguna |
| `MYSQL_PASSWORD`, `MYSQL_ROOT_PASSWORD`, `REDIS_PASSWORD` | `openssl rand -hex 16` (hanya huruf/angka agar aman di DSN) |
| `SMTP_*` | akun email keluar. Port 587 = STARTTLS, 465 = TLS langsung; bila memakai akun, TLS **wajib** di production |
| `ADMIN_EMAIL`, `ADMIN_NAME`, `ADMIN_PASSWORD` | akun HR pertama (langkah 4); hapus `ADMIN_PASSWORD` setelahnya |
| `COOKIE_SECURE` | biarkan `true`; cookie login hanya dikirim lewat HTTPS |

## 3. Nginx host + TLS

```bash
sudo cp deploy/nginx-host.conf /etc/nginx/sites-available/sikerja
sudo sed -i 's/sikerja.perusahaan.id/DOMAIN-ANDA/' /etc/nginx/sites-available/sikerja
sudo ln -s /etc/nginx/sites-available/sikerja /etc/nginx/sites-enabled/sikerja
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d DOMAIN-ANDA --redirect -m admin@perusahaan.id --agree-tos
# opsional tapi disarankan, di blok 443 yang dibuat certbot:
#   add_header Strict-Transport-Security "max-age=31536000" always;
```

Perpanjangan sertifikat otomatis lewat timer `certbot.timer` (cek: `systemctl list-timers | grep certbot`).

## 4. Deploy pertama

```bash
docker compose -f docker-compose.prod.yml build                    # beberapa menit pertama kali
docker compose -f docker-compose.prod.yml up -d mysql redis
docker compose -f docker-compose.prod.yml run --rm migrate         # buat tabel
docker compose -f docker-compose.prod.yml run --rm bootstrap       # data master + akun HR pertama (TANPA data demo)
docker compose -f docker-compose.prod.yml up -d
```

`bootstrap` mengisi: 4 departemen + jabatan, template KPI default (sesuai SPEC), aspek perilaku, jenis cuti,
jadwal kerja, hari libur 2026, dan **satu** akun HR. Aman dijalankan ulang (menolak bila sudah ada akun).
Setelah itu hapus `ADMIN_PASSWORD` dari `.env`, login sebagai `ADMIN_EMAIL`, dan **ganti password** di menu profil.

Cek:

```bash
docker compose -f docker-compose.prod.yml ps                 # semua "healthy" (worker tidak punya healthcheck)
curl -fsS https://DOMAIN-ANDA/healthz                        # {"status":"ok"}
```

Lalu di aplikasi (sebagai HR): tambah karyawan / **Import Excel**, atur atasan & role, cek **Hari libur** dan **Jam kerja**
di Pengaturan, buat periode penilaian pertama.

> **Hari libur harus diperbarui HR tiap tahun** (Pengaturan → Hari libur → impor/tambah). Daftar bawaan hanya mencakup
> tahun yang ada di kode, dan tanggal cuti bersama ditetapkan pemerintah — periksa ulang dengan SKB resmi.

## 5. Update / rilis baru

```bash
cd /opt/sikerja
./deploy/deploy.sh                   # branch saat ini
REF=v1.1.0 ./deploy/deploy.sh        # atau tag tertentu
```

Skrip: `git pull` → **backup** (kecuali deploy pertama) → build image (diberi tag commit) → nyalakan MySQL/Redis →
**migrasi** → `up -d` → menunggu `/healthz`. Bila backup atau migrasi gagal, skrip berhenti sebelum aplikasi diganti.

### Migrasi database

- Migrasi (`backend/migrations/*.sql`, golang-migrate) dijalankan **eksplisit** saat deploy lewat service `migrate`
  (`AUTO_MIGRATE=false` pada api produksi), sehingga tidak ada dua instance yang berebut migrasi.
- Migrasi bersifat maju-saja. Selalu ada backup tepat sebelumnya (dibuat `deploy.sh`).
- Migrasi manual: `docker compose -f docker-compose.prod.yml run --rm migrate`.

### Rollback

1. `git checkout <tag/commit sebelumnya>` lalu `./deploy/deploy.sh` kembali (image lama dibangun ulang).
2. Bila migrasi baru sudah terlanjur diterapkan dan versi lama tidak kompatibel: pulihkan database dari backup tepat
   sebelum deploy (bagian 6), baru jalankan versi lama.

## 6. Backup MySQL harian

`deploy/backup.sh` membuat, di `/var/backups/sikerja`:

- `db-YYYYMMDD-HHMMSS.sql.gz` — `mysqldump --single-transaction` (konsisten tanpa mengunci tabel), diverifikasi `gzip -t`;
- `storage-YYYYMMDD-HHMMSS.tar.gz` — arsip folder lampiran (bukti KPI, surat dokter, foto);

dan menghapus yang lebih tua dari `KEEP_DAYS` (default 14 hari).

```bash
sudo mkdir -p /var/backups/sikerja
sudo cp deploy/sikerja-backup.cron /etc/cron.d/sikerja-backup        # harian 01.15; sesuaikan path & jam
sudo cp deploy/logrotate-sikerja /etc/logrotate.d/sikerja             # rotasi log backup
sudo /opt/sikerja/deploy/backup.sh                                   # coba sekarang
```

**Salin backup ke luar server** (server mati = backup ikut hilang), mis. `rsync -a /var/backups/sikerja/ user@server-lain:/backup/sikerja/`
atau `rclone` ke penyimpanan cloud, dari cron terpisah.

### Pulihkan

```bash
./deploy/restore.sh /var/backups/sikerja/db-20261017-011500.sql.gz   # meminta konfirmasi, MENIMPA database
# lampiran (bila volume hilang):
docker run --rm -v sikerja_storage:/data -v /var/backups/sikerja:/backup alpine \
  tar xzf /backup/storage-20261017-011500.tar.gz -C /data
```

**Uji pulihkan paling tidak sekali** di server uji sebelum benar-benar membutuhkannya.

## 7. Rotasi log

| Log | Mekanisme |
| --- | --- |
| Container (`api`, `worker`, `web`, `mysql`, `redis`) | driver `json-file`, maks. 5 × 10 MB per service (`docker-compose.prod.yml`) + `/etc/docker/daemon.json` |
| Backup (`/var/log/sikerja-backup.log`) | `logrotate` mingguan, simpan 8 (`deploy/logrotate-sikerja`) |
| Nginx host | bawaan paket Ubuntu (`/etc/logrotate.d/nginx`) |

Lihat log: `docker compose -f docker-compose.prod.yml logs -f --tail=100 api worker`. Log API berformat JSON
(request, status, durasi, IP klien); IP klien asli diteruskan Nginx host → web → api lewat `X-Forwarded-For`
dan hanya dipercaya dari jaringan privat (`TRUSTED_PROXIES`).

## 8. Keamanan yang sudah diatur & yang perlu dijaga

Sudah di konfigurasi: HTTPS + cookie `Secure`/`HttpOnly`/`SameSite=Lax`, CORS hanya ke `APP_URL`, rate limit login
(5 gagal → jeda 1 menit per akun, ditambah 20/menit per IP), MySQL/Redis tidak terekspos, Redis ber-password,
container api/worker non-root + filesystem read-only + `cap_drop: ALL`, header keamanan di Nginx dan API,
`JWT_SECRET` wajib ≥ 32 karakter (aplikasi menolak start bila tidak).

Perlu dijaga: `.env` hanya terbaca root/pemilik (`chmod 600`); update OS berkala (`unattended-upgrades`);
`docker compose pull`/build ulang berkala untuk patch image dasar; pantau `/healthz` dengan layanan uptime;
akses SSH dengan kunci (matikan login password).

## 9. Pemecahan masalah

| Gejala | Periksa |
| --- | --- |
| `docker compose` menolak: "isi JWT_SECRET…" | variabel di `.env` kosong |
| Login berhasil tetapi langsung keluar lagi | akses lewat HTTP biasa sementara `COOKIE_SECURE=true`; gunakan HTTPS |
| `502` dari Nginx host | `docker compose ... ps` — `api` belum healthy; `logs api` (biasanya password DB/Redis di `.env` berubah setelah volume dibuat) |
| Email tidak terkirim | `logs worker`; cek `SMTP_*`; di production TLS wajib bila memakai akun |
| Pengingat 08.00 tidak muncul | `logs worker` memuat `jadwal` dan `zona`; cek `APP_TIMEZONE` |
| Ganti password MySQL setelah volume ada | variabel `MYSQL_*` hanya dipakai saat volume pertama dibuat; ubah dengan `ALTER USER` di dalam container |
