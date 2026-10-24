# MVP-4: Central download gateway

## Added flow

1. `POST /api/v1/public/files/:id/downloads` checks publication policy, preservation state, SHA-256 metadata, and at least one verified placement on an online node seen within two minutes. It returns a job ID and a one-time bearer token. Only the token hash is persisted.
2. Node agents poll `POST /api/v1/nodes/:id/delivery-jobs/claim` using their existing node bearer credential. This is separate from the existing `tasks` API and does not create or mutate store/replicate/delete tasks.
3. The selected agent validates the stored object size and SHA-256 locally, then streams it to `POST /api/v1/nodes/:id/delivery-jobs/:jobID/content?lease_token=...`.
4. The API validates the received byte count and SHA-256, writes to a temporary file in `DELIVERY_DIR`, and atomically renames it before marking the job `ready`.
5. `GET /api/v1/public/downloads/:id` reports status; `GET /api/v1/public/downloads/:id/content` streams through the central API after re-checking public eligibility. Both require `Authorization: Bearer <access_token>`.

## New configuration

`DELIVERY_DIR` defaults to `./data/delivery`. Compose mounts it as its own `delivery_data` named volume, separate from `staging_data` and node object volumes.

## Migration

New additive migration: `atlas/migrations/20261024000000_mvp4_download_gateway.sql`.
It adds `download_jobs` and `download_audit_logs`; it does not alter existing task or placement rows. The compose development flow refreshes Atlas migration checksums before applying migrations.

## Audit

`download_audit_logs` records request accept/deny, preparation success/failure, delivery success/failure, selected node where known, bytes transferred, duration in milliseconds, and reason. Raw bearer tokens are not logged or stored.

## Verification limitation

`gofmt` and `git diff --check` pass. Full `go test` could not run in the build environment because this repository requires Go 1.27 and the toolchain download is blocked by unavailable network access. Run tests and migration validation locally before applying to the persistent database.
