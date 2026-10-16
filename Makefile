# SiKerja — perintah umum. Butuh: make, Docker (+ Compose v2), Go, Node 22.
# Di Windows tanpa make: jalankan perintah di kolom kanan secara langsung (lihat README).

COMPOSE ?= docker compose
.PHONY: help dev up down logs build test test-backend test-frontend lint lint-backend lint-frontend migrate seed fresh env

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

test-frontend: ## Vitest
	cd frontend && npm run test

lint: lint-backend lint-frontend ## Semua lint

lint-backend: ## gofmt + go vet + golangci-lint (bila terpasang)
	cd backend && test -z "$$(gofmt -l .)" || (gofmt -l . && echo "gofmt: ada berkas yang belum diformat" && exit 1)
	cd backend && go vet ./...
	cd backend && (command -v golangci-lint >/dev/null && golangci-lint run ./... || echo "golangci-lint tidak terpasang — dilewati")

lint-frontend: ## ESLint + Prettier + vue-tsc
	cd frontend && npm run lint && npm run format:check && npm run typecheck
