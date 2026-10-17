#!/usr/bin/env bash
# Deploy / update SiKerja di server: tarik kode, build image, migrasi database, nyalakan ulang.
#   ./deploy/deploy.sh            (branch saat ini)
#   REF=v1.2.0 ./deploy/deploy.sh (tag/branch tertentu)
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO"
COMPOSE=(docker compose -f docker-compose.prod.yml)

[ -f .env ] || { echo ".env belum ada: cp .env.production.example .env lalu isi nilainya" >&2; exit 1; }

echo "==> Mengambil kode"
git fetch --tags --prune origin
if [ -n "${REF:-}" ]; then git checkout "$REF"; else git pull --ff-only; fi
export TAG="$(git rev-parse --short HEAD)"

echo "==> Backup sebelum deploy"
if [ -f "$REPO/.deployed" ]; then "$REPO/deploy/backup.sh"; else echo "(deploy pertama: tidak ada data untuk dibackup)"; fi

echo "==> Build image ($TAG)"
"${COMPOSE[@]}" build

echo "==> Menyalakan MySQL & Redis"
"${COMPOSE[@]}" up -d mysql redis

echo "==> Migrasi database"
"${COMPOSE[@]}" run --rm migrate

echo "==> Menyalakan aplikasi"
"${COMPOSE[@]}" up -d --remove-orphans

echo "==> Menunggu sehat"
for i in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:${WEB_PORT:-8081}/healthz" >/dev/null 2>&1; then
    echo "OK — SiKerja berjalan (versi $TAG)"
    touch "$REPO/.deployed"
    docker image prune -f >/dev/null
    exit 0
  fi
  sleep 2
done
echo "GAGAL: /healthz tidak menjawab. Lihat: ${COMPOSE[*]} logs --tail=100 api" >&2
exit 1
