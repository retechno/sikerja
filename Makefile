# SiKerja — perintah umum. Butuh: make, Docker (+ Compose v2), Go, Node 22.
# Di Windows tanpa make: jalankan perintah di kolom kanan secara langsung (lihat README).

COMPOSE ?= docker compose
.PHONY: help dev up down logs build test test-backend test-mysql test-frontend lint lint-backend lint-frontend migrate seed fresh env prod-build prod-up

help: ## Daftar perintah
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*## "}{printf "  make %-16s %s\n", $$1, $$2}'

env: ## Buat .env dari .env.example bila belum ada
	@test -f .env || cp .env.example .env

dev: env ## Jalankan semuanya (MySQL, Redis, Mailpit, API, worker, web), isi data demo bila kosong, lalu ikuti log
	$(COMPOSE) up --build -d
	$(COMPOSE) run --rm api go run ./cmd/seed
	@echo ""
	@echo "Web      : http://localhost:5173"
	@echo "API      : http://localhost:$${API_PORT:-8080}/healthz"
	@echo "Mailpit  : http://localhost:8025"
	@echo "Akun demo: lihat README (password semua: password)"
	$(COMPOSE) logs -f api web

up: env ## Jalankan di latar belakang tanpa log
	$(COMPOSE) up --build -d

down: ## Hentikan semua container (data tetap)
	$(COMPOSE) down

logs: ## Ikuti log
	$(COMPOSE) logs -f

build: ## Build binary backend dan bundle frontend
	cd backend && go build -o bin/ ./cmd/...
	cd frontend && npm run build

migrate: ## Terapkan migrasi database (di container api)
	$(COMPOSE) run --rm api go run ./cmd/migrate

seed: ## Isi data awal + demo bila database masih kosong
	$(COMPOSE) run --rm api go run ./cmd/seed

fresh: ## Kosongkan lalu isi ulang data demo (HAPUS data yang ada)
	$(COMPOSE) run --rm api go run ./cmd/seed -fresh

test: test-backend test-frontend ## Semua test

test-backend: ## go test (SQLite in-memory + miniredis, tanpa Docker)
	cd backend && go test ./...

test-mysql: ## go test pada MySQL & Redis sungguhan dari docker compose (port 3307/6380), seperti CI
	cd backend && TEST_MYSQL_DSN="root:$${MYSQL_ROOT_PASSWORD:-root}@tcp(127.0.0.1:3307)/?charset=utf8mb4&parseTime=true&loc=Local" TEST_REDIS_ADDR=127.0.0.1:6380 go test -p 1 -count=1 ./...

test-frontend: ## Vitest
	cd frontend && npm run test

lint: lint-backend lint-frontend ## Semua lint

lint-backend: ## gofmt + go vet + golangci-lint (bila terpasang)
	cd backend && test -z "$$(gofmt -l .)" || (gofmt -l . && echo "gofmt: ada berkas yang belum diformat" && exit 1)
	cd backend && go vet ./...
	cd backend && (command -v golangci-lint >/dev/null && golangci-lint run ./... || echo "golangci-lint tidak terpasang — dilewati")

prod-build: ## Build image produksi (butuh .env dari .env.production.example)
	docker compose -f docker-compose.prod.yml build

prod-up: ## Migrasi lalu jalankan stack produksi (lihat docs/DEPLOY.md)
	docker compose -f docker-compose.prod.yml up -d mysql redis
	docker compose -f docker-compose.prod.yml run --rm migrate
	docker compose -f docker-compose.prod.yml up -d

lint-frontend: ## ESLint + Prettier + vue-tsc
	cd frontend && npm run lint && npm run format:check && npm run typecheck
