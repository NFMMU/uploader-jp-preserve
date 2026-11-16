SHELL := /bin/sh

.PHONY: up down logs test test-unit test-integration test-web test-surplus-cleanup build migrate-diff migrate-apply migrate-hash generate check

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f api postgres

test: test-unit test-web

test-unit:
	go test ./...

test-integration:
	RUN_CATALOG_DB_INTEGRATION=1 TEST_DATABASE_URL=postgres://postgres:postgres@localhost:55432/archive_test?sslmode=disable go test ./internal/api

test-web:
	cd web && npm test


test-surplus-cleanup:
	./scripts/test-surplus-cleanup.sh

build:
	go build ./cmd/api ./cmd/generate

# Schema source of truth: atlas/
migrate-diff:
	cd atlas && atlas migrate diff $(name) --env local

migrate-hash:
	atlas migrate hash --dir file://atlas/migrations --force

migrate-apply: migrate-hash
	cd atlas && atlas migrate apply --env local

generate:
	go run ./cmd/generate

check: test build
