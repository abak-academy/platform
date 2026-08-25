# GOROOT override for this machine (Go installed via Homebrew Cellar, not /usr/local/go).
# Override with `make GOROOT=... <target>` on other machines.
GOROOT ?= /opt/homebrew/Cellar/go/1.26.3/libexec
GO := $(GOROOT)/bin/go
COMPOSE := docker compose -f deploy/compose/local.yml
DATABASE_URL ?= postgres://akademi:akademi@localhost:5432/akademi?sslmode=disable

.PHONY: help up down logs obs-up obs-down api worker web migrate-up migrate-down tidy build

help: ## list targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

up: ## start infra (postgres, redis, minio)
	$(COMPOSE) up -d

down: ## stop infra
	$(COMPOSE) down

logs: ## tail infra logs
	$(COMPOSE) logs -f

obs-up: ## start infra + monitoring repro stack (grafana at :3002)
	$(COMPOSE) --profile observability up -d

obs-down: ## stop everything, including the monitoring repro stack
	$(COMPOSE) --profile observability down --remove-orphans

api: ## run the API server on :8080
	cd backend && $(GO) run ./cmd/api

worker: ## run the outbox worker
	cd backend && $(GO) run ./cmd/worker

web: ## run the Next.js dev server on :3000
	cd web && npm run dev

build: ## compile both Go binaries to backend/bin
	cd backend && $(GO) build -o bin/api ./cmd/api && $(GO) build -o bin/worker ./cmd/worker

migrate-up: ## apply DB migrations (needs golang-migrate CLI)
	migrate -path backend/db/migrations -database "$(DATABASE_URL)" up

migrate-down: ## roll back the last migration (needs golang-migrate CLI)
	migrate -path backend/db/migrations -database "$(DATABASE_URL)" down 1

tidy: ## resolve Go dependencies
	cd backend && $(GO) mod tidy
