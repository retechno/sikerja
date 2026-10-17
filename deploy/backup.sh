#!/usr/bin/env bash
# Backup harian SiKerja: dump MySQL + arsip folder lampiran. Dipanggil cron (lihat docs/DEPLOY.md).
#   BACKUP_DIR   folder tujuan            (default /var/backups/sikerja)
#   KEEP_DAYS    lama simpan, dalam hari  (default 14)
#   COMPOSE_FILE berkas compose produksi  (default <repo>/docker-compose.prod.yml)
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/sikerja}"
KEEP_DAYS="${KEEP_DAYS:-14}"
COMPOSE_FILE="${COMPOSE_FILE:-$REPO/docker-compose.prod.yml}"
STAMP="$(date +%Y%m%d-%H%M%S)"
compose() { docker compose -f "$COMPOSE_FILE" --project-directory "$REPO" "$@"; }

umask 077
mkdir -p "$BACKUP_DIR"
DB_OUT="$BACKUP_DIR/db-$STAMP.sql.gz"
FILES_OUT="$BACKUP_DIR/storage-$STAMP.tar.gz"

echo "[$(date -Is)] backup dimulai"

# --single-transaction: dump konsisten tanpa mengunci tabel. Password dibaca dari environment container (tidak muncul di argumen).
compose exec -T mysql sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysqldump -uroot --single-transaction --routines --triggers --no-tablespaces --default-character-set=utf8mb4 "$MYSQL_DATABASE"' \
  | gzip -9 > "$DB_OUT.tmp"
gzip -t "$DB_OUT.tmp"
[ "$(gzip -dc "$DB_OUT.tmp" | head -c 200 | wc -c)" -gt 100 ] || { echo "dump kosong/rusak" >&2; rm -f "$DB_OUT.tmp"; exit 1; }
mv "$DB_OUT.tmp" "$DB_OUT"

# lampiran (bukti KPI, surat dokter, foto) ada di volume Docker `storage`
VOLUME="${COMPOSE_PROJECT_NAME:-sikerja}_storage"
docker run --rm -v "$VOLUME":/data:ro -v "$BACKUP_DIR":/backup alpine:3.20 \
  tar czf "/backup/$(basename "$FILES_OUT")" -C /data .

# hapus backup lama
find "$BACKUP_DIR" -maxdepth 1 -type f \( -name 'db-*.sql.gz' -o -name 'storage-*.tar.gz' \) -mtime +"$KEEP_DAYS" -print -delete

echo "[$(date -Is)] backup selesai: $(basename "$DB_OUT") ($(du -h "$DB_OUT" | cut -f1)), $(basename "$FILES_OUT") ($(du -h "$FILES_OUT" | cut -f1))"
