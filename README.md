# uploader.jp 魚拓作戦 — MVP-1 foundation

Go API and PostgreSQL schema for the uploader.jp preservation catalog. This implementation follows design document v1.1 and the project decisions made in conversation.

## Schema/code generation contract

1. Edit `atlas/schema.hcl` (the schema source of truth).
2. Generate a versioned migration with `atlas migrate diff <name> --env local`.
3. Review and commit the generated SQL migration and `atlas.sum`.
4. Apply migrations with `atlas migrate apply --env local`.
5. Regenerate Go models and typed query code with `make generate`.

Do not use GORM `AutoMigrate`, and do not hand-edit generated model/query files. Put business logic in `internal/api` and later service/repository packages.

## Local development

Requirements: Go 1.23+, Docker Compose, and Atlas CLI for schema changes.

```sh
cp .env.example .env
# Edit .env: set a unique JWT_SECRET (32+ bytes) and a bootstrap admin password (12+ chars).
# For the compose DB, POSTGRES_PASSWORD defaults to archive-dev-password; change it outside local dev.
atlas migrate hash --dir file://atlas/migrations
docker compose up --build
```

The initial Atlas migration is in `atlas/migrations/20261023000000_initial.sql`. For local Compose only, the migration container creates `atlas.sum` if it is absent, then applies migrations. Commit the resulting `atlas/migrations/atlas.sum` to version control. For production, generate and review checksums in development/CI; do not re-hash migrations automatically during deployment.

API listens on `http://localhost:8080`. Health: `GET /healthz`.

## First login

Use `BOOTSTRAP_ADMIN_EMAIL` and `BOOTSTRAP_ADMIN_PASSWORD` from `.env` to log in:

```sh
curl -s http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.invalid","password":"replace-this-bootstrap-password"}'
```

The first administrator is created only when the users table is empty. The bootstrap password is hashed with bcrypt; it is not stored in plaintext. After initial setup, remove the bootstrap password from deployment environment configuration.

## MVP-1 API surface

- `POST /api/v1/auth/login`, `GET /api/v1/me`
- `GET /api/v1/artifacts?q=...`, `POST /api/v1/artifacts`, `GET /api/v1/artifacts/:id`
- `POST /api/v1/artifacts/:id/files`, `GET /api/v1/files`, `GET /api/v1/files/:id`
- `PATCH /api/v1/files/:id/policy`, `PATCH /api/v1/files/:id/publication`
- `GET /api/v1/export?format=json|csv`
- `POST /api/v1/public-requests`, `GET /api/v1/public-requests`, `PATCH /api/v1/public-requests/:id`
- `POST /api/v1/admin/users`, `GET /api/v1/admin/users`, `GET /api/v1/admin/audit-logs`

Public request submission is unauthenticated by design. Review, export, and catalog endpoints require a bearer token. User/role administration is admin-only. Reviewers can update rights policy and process requests, but cannot create users or change system roles.

## Safety defaults and scope

- New files default to `preservation_status=uncollected`, `redistribution_policy=unknown`, and `public_download_enabled=false`.
- Supported redistribution states: `allowed`, `restricted`, `in_progress`, `pending`, `unknown`.
- Moving a policy away from `allowed` immediately disables the public-download flag.
- The public-download flag is not a download authorization by itself. The MVP-4 gateway must also check `preservation_status=verified` and a healthy placement before serving any bytes. No file-download endpoint or direct node access is implemented in MVP-1.
- `source_url` accepts HTTPS URLs on `uploader.jp` or its subdomains only.
- Accepted takedown/deletion requests disable public download immediately; actual node deletion tasks are part of a later milestone.
- Node capacity fields distinguish actual filesystem capacity/free space from the participant's configured requested capacity. Node-agent reporting is not implemented in MVP-1 yet.

## Environment variables for node participation

The future node agent will read these on each participant's node:

- `NODE_STORAGE_PATH`: local directory used for managed objects.
- `NODE_CAPACITY_BYTES`: participant's requested maximum participation quota. This is a participant setting, not a global system limit.

The agent will additionally report actual filesystem capacity/free bytes and bytes used by managed objects. The central service should schedule only within both the requested quota and real free-space limits. Never expose storage paths or node addresses to public API clients.

## Current limitations

This is the first implementation slice, not a production-ready archival service. Rate limiting, email confirmation, API-key/mTLS node authentication, actual ingestion, replication/task execution, gateway delivery, full request escalation, OTel/Grafana, and node deletion are subsequent milestones. Do not expose the service publicly until TLS termination, secrets, backups, rate limiting, and operational controls are configured.
