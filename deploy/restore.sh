#!/usr/bin/env bash
# Pulihkan database dari dump backup.sh. MENIMPA isi database saat ini.
#   ./deploy/restore.sh /var/backups/sikerja/db-20261017-010000.sql.gz
set -euo pipefail
[ $# -eq 1 ] && [ -f "$1" ] || { echo "pemakaian: $0 <berkas db-*.sql.gz>" >&2; exit 1; }

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-$REPO/docker-compose.prod.yml}"
compose() { docker compose -f "$COMPOSE_FILE" --project-directory "$REPO" "$@"; }

gzip -t "$1"
read -r -p "Ini MENIMPA database saat ini dengan $1. Ketik 'pulihkan' untuk lanjut: " ok
[ "$ok" = "pulihkan" ] || { echo "dibatalkan"; exit 1; }

echo "Menghentikan api & worker..."
compose stop web api worker
gzip -dc "$1" | compose exec -T mysql sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot --default-character-set=utf8mb4 "$MYSQL_DATABASE"'
echo "Menyalakan kembali..."
compose up -d
echo "Selesai. Cache dasbor otomatis menyesuaikan; cek https://<domain>/healthz"
