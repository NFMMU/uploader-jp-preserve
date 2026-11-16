SHELL := /bin/sh

.PHONY: up down logs test test-all test-unit test-integration test-web build migrate-diff migrate-apply migrate-hash generate check

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f api postgres

test: test-unit test-web

test-all: test test-integration

test-unit:
	go test ./...

test-integration:
	sh ./scripts/test-integration.sh

test-web:
	cd web && npm test


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
