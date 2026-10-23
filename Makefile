SHELL := /bin/sh

.PHONY: up down logs test build migrate-diff migrate-apply migrate-hash generate check

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f api postgres

test:
	go test ./...

build:
	go build ./cmd/api ./cmd/generate

# Schema source of truth: atlas/schema.hcl
migrate-diff:
	atlas migrate diff $(name) --env local

migrate-hash:
	atlas migrate hash --dir file://atlas/migrations

migrate-apply:
	atlas migrate apply --env local

generate:
	go run ./cmd/generate

check: test build
