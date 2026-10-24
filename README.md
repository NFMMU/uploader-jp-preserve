# uploader.jp 魚拓作戦 — MVP-3: distributed preservation

Go API, PostgreSQL catalog, and Go node agents. This implementation follows system design v1.3. Local Docker Compose starts three isolated demo nodes (one A node and two B nodes) so a spare target is available for failure-recovery tests; for production, each agent should run on its own host and storage volume.

## MVP-3 scope

- A Go node agent enrolls with the central API using a shared enrollment secret, then stores its per-node bearer token in a persistent file with mode `0600`.
- The agent reports actual filesystem capacity/free space, managed-object usage, the participant's requested quota, and its last-seen time.
- Nodes A/B enroll separately and report real capacity, available bytes, used bytes, requested quota, and heartbeat status.
- An administrator uploads a file to temporary central staging. The API computes SHA-256 and size and creates independent storage tasks for two eligible online nodes.
- Each node downloads from the authenticated API, writes into its own managed storage, re-hashes the final on-disk object, and reports its result.
- A file is marked `verified` only when at least two placements are verified on online nodes. A single remaining healthy copy is `degraded`; task failures are retried up to five attempts.
- If a verified file needs another copy, the API queues a `replicate` task on a verified source node. That agent verifies its local object and uploads it to central staging; the target node then stores and verifies it through the normal storage workflow.
- A background reconciler marks stale nodes offline, marks unverified placements on those nodes missing, updates file preservation state, and queues replacement replicas when a healthy source and eligible target are available. Previously verified placement metadata is retained when a node is merely offline; the copy is treated as unavailable until the node returns.
- The required healthy replica count is two, with one extra healthy replica allowed (maximum three). If more than three online verified placements exist, the reconciler queues one surplus deletion at a time, only when no task for that file is active, every online node holding the file has been continuously online for at least five minutes, and the newest verified placement is at least five minutes old. Deletion runs as a node-agent task; the placement is marked `deleted` only after the agent confirms success. Failed or uncertain deletion is not counted as healthy.
- Administrators can inspect placements, request replication, and trigger reconciliation. Task and placement actions are recorded in audit logs.

This is an MVP-3 implementation slice, not a production-ready deployment. Central staging is temporary relay space for transfers, not a permanent third replica. Use only test or authorized content while validating this workflow.

## Schema/code generation contract

1. Edit `atlas/schema.hcl` (the schema source of truth).
2. Generate a versioned migration with `atlas migrate diff <name> --env local`.
3. Review and commit the generated SQL migration and `atlas.sum`.
4. Apply migrations with `atlas migrate apply --env local`.
5. Regenerate Go models and typed query code with `make generate`.

Do not use GORM `AutoMigrate`, and do not hand-edit generated model/query files. MVP-2 uses SQL for the two lease/credential columns not yet present in the existing generated structs.

## Local development with Docker Compose

Requirements: Docker Desktop / Docker Compose. Go 1.23+ and Atlas CLI are needed only when editing schema or generating code locally.

```sh
cp .env.example .env
docker compose up --build
```

Compose runs PostgreSQL, applies Atlas migrations, starts the API, and starts three local Go node agents (`local-node-a`, `local-node-b`, and `local-node-c`). Each agent enrolls independently and saves its credential in its own persistent volume. On first startup, allow a short time for all three first heartbeats.

```sh
curl -i http://localhost:8080/healthz
docker compose ps
docker compose logs --tail=100 node-agent node-agent-b node-agent-c
```

API listens on `http://localhost:8080`. Health endpoint: `GET /healthz`.

### First login

Use `BOOTSTRAP_ADMIN_EMAIL` and `BOOTSTRAP_ADMIN_PASSWORD` from `.env`:

```sh
curl -s http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.invalid","password":"replace-this-bootstrap-password"}'
```

The first administrator is created only when the users table is empty. The bootstrap password is hashed with bcrypt; it is not stored in plaintext. After initial setup, remove the bootstrap password from deployment environment configuration.

## End-to-end MVP-3 smoke test

Set the access token from login. Do not paste credentials or bearer tokens into logs or chat.

```sh
export TOKEN='your-admin-access-token'

curl -sS http://localhost:8080/api/v1/nodes \
  -H "Authorization: Bearer $TOKEN"

ARTIFACT=$(curl -sS -X POST http://localhost:8080/api/v1/artifacts \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"title":"MVP-3 test artifact","source_url":"https://www.uploader.jp/"}')
export ARTIFACT_ID=$(printf '%s' "$ARTIFACT" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')

FILE=$(curl -sS -X POST "http://localhost:8080/api/v1/artifacts/$ARTIFACT_ID/files" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"original_filename":"sample.txt","size_bytes":0,"media_type":"text/plain","redistribution_policy":"pending"}')
export FILE_ID=$(printf '%s' "$FILE" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')

printf 'uploader.jp archive MVP-3 test\n' > /tmp/archive-sample.txt
curl -i -X POST "http://localhost:8080/api/v1/files/$FILE_ID/ingest" \
  -H "Authorization: Bearer $TOKEN" \
  -F 'file=@/tmp/archive-sample.txt;type=text/plain' -F 'priority=3'

curl -sS 'http://localhost:8080/api/v1/tasks' \
  -H "Authorization: Bearer $TOKEN"
curl -sS "http://localhost:8080/api/v1/files/$FILE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

Expected result: the ingest endpoint returns `202 Accepted` with two tasks and two target nodes. Both tasks should become `completed`; the placements endpoint should report `healthy_replica_count: 2`; file `preservation_status` becomes `verified`; `public_download_enabled` remains `false` because the redistribution policy is `pending`.

```sh
curl -sS "http://localhost:8080/api/v1/files/$FILE_ID/placements" \
  -H "Authorization: Bearer $TOKEN"
```

To manually ask the system to create another copy for an existing file, or to test the placement repair path, use `POST /api/v1/files/:id/replicate`. The API schedules a source-node transfer only when another healthy copy is needed and an eligible target node is available. To trigger an immediate health/placement reconciliation during testing, call `POST /api/v1/admin/placements/reconcile`.

To inspect node state or task failures:

```sh
curl -sS http://localhost:8080/api/v1/nodes -H "Authorization: Bearer $TOKEN"
curl -sS 'http://localhost:8080/api/v1/tasks?status=failed' -H "Authorization: Bearer $TOKEN"
docker compose logs --tail=200 node-agent node-agent-b node-agent-c api migrate
```

## MVP-1 API surface

- `POST /api/v1/auth/login`, `GET /api/v1/me`
- `GET /api/v1/artifacts?q=...`, `POST /api/v1/artifacts`, `GET /api/v1/artifacts/:id`
- `POST /api/v1/artifacts/:id/files`, `GET /api/v1/files`, `GET /api/v1/files/:id`
- `PATCH /api/v1/files/:id/policy`, `PATCH /api/v1/files/:id/publication`
- `GET /api/v1/export?format=json|csv`
- `POST /api/v1/public-requests`, `GET /api/v1/public-requests`, `PATCH /api/v1/public-requests/:id`
- `POST /api/v1/admin/users`, `GET /api/v1/admin/users`, `GET /api/v1/admin/audit-logs`

## MVP-2 and MVP-3 API surface

- `POST /api/v1/nodes/register` — one-time enrollment with `X-Node-Enrollment-Token`
- `POST /api/v1/nodes/:id/heartbeat` — node-token authenticated capacity report
- `GET /api/v1/nodes` — admin/reviewer node list
- `GET /api/v1/tasks?status=...` — admin/reviewer task list
- `POST /api/v1/tasks/:id/retry` — admin-only retry for a failed storage task; optionally send a replacement `file` as multipart data if staged source content is missing or corrupt
- `POST /api/v1/files/:id/ingest` — admin-only multipart upload; queues two placements on separate eligible nodes
- `GET /api/v1/files/:id/placements` — admin/reviewer placement status and healthy replica count
- `POST /api/v1/files/:id/replicate` — admin-only manual request to create a replacement replica when needed
- `POST /api/v1/admin/placements/reconcile` — admin-only immediate reconciliation
- `POST /api/v1/nodes/:id/tasks/claim` — node-token authenticated store, replicate, or surplus-delete task claim
- `GET /api/v1/nodes/:id/tasks/:taskID/content?lease_token=...` — authenticated central-staged task content for target storage
- `POST /api/v1/nodes/:id/tasks/:taskID/replica-content?lease_token=...` — source-node authenticated relay upload for a replica
- `POST /api/v1/nodes/:id/tasks/:taskID/result` — authenticated hash/size result

The node credential is returned only at enrollment; the database stores only its SHA-256 hash. Do not expose the enrollment secret. Node storage is not exposed directly to end users.

### Administrator retry example

If the staged source is intact, a retry can be queued without uploading a replacement:

```sh
curl -sS -X POST "http://localhost:8080/api/v1/tasks/$TASK_ID/retry" \
  -H "Authorization: Bearer $TOKEN"
```

If the staged source is missing or fails integrity verification, provide the correct original file (it must match the file object's recorded size and SHA-256):

```sh
curl -sS -X POST "http://localhost:8080/api/v1/tasks/$TASK_ID/retry" \
  -H "Authorization: Bearer $TOKEN" \
  -F 'file=@/path/to/correct-file'
```

## MVP-3 placement behavior

- New ingests require two online nodes with sufficient actual free space and requested participant quota. If fewer than two nodes are eligible, ingestion returns `409 Conflict` rather than silently claiming the file is fully replicated.
- Each target has its own task ID and temporary staged payload. A successful target removes its own staged payload; a failed target keeps it available for administrator retry.
- A replication task is claimed by the source node, which re-checks the source object's size and SHA-256 before relaying it. The API validates the bytes again before queuing target storage. The target agent then uses the same write-and-verify path as an initial ingest.
- The file becomes `verified` only with at least two healthy verified placements. One healthy copy is `degraded`; if no verified copy is currently reachable, the file is `missing`; a terminal storage error with no verified placement is `error`.
- Automatic reconciliation runs every 30 seconds and treats a node with no heartbeat for two minutes as offline. It does not delete physical data on a node that has gone offline.
- Surplus policy: keep the required two healthy replicas plus one spare. When there are four or more healthy replicas, queue a deletion task for the newest verified placement only after all online nodes holding the file have been continuously online for at least five minutes and that placement was verified at least five minutes ago. At most one deletion task per file runs at a time, and no deletion is queued while any other task for that file is active. The node agent deletes only the exact managed storage key; deletion of an already-absent object is idempotent. Placement status changes to `deleted` only after success is reported. A failed/uncertain deletion is marked `error` and never counted as a healthy replica.

## Capacity settings

- `NODE_CAPACITY_BYTES` is the participant's requested quota. `0` means no participant-defined quota; actual free disk space still constrains scheduling.
- The agent reports actual filesystem total/free bytes and the size of managed objects. The API checks both actual available space and requested quota before assigning an upload.
- Capacity is re-reported every 30 seconds. The API considers a node schedulable only when its heartbeat is recent.

## Safety defaults and limitations

- New file objects default to `preservation_status=uncollected`, `redistribution_policy=unknown`, and `public_download_enabled=false`.
- Preserving a file is separate from permission to redistribute it. MVP-3 does not provide a public download route.
- Uploads are limited to 5 GiB per request in this MVP. Large-file resumable uploads are not implemented.
- Each execution run is limited to five attempts. An administrator can retry a terminally failed storage task; retry resets `attempt_count` to `0`, revalidates staged content against the expected size and SHA-256, and gives the task up to five new attempts. Prior task-result events and retry actions remain in the audit log.
- Failed tasks retain their staged source payload for possible administrator retry. On retry, a replacement payload may be uploaded with `-F 'file=@/path/to/correct-file'`; it must match the file object's expected size and SHA-256. A successful task removes its staged payload. The node agent removes stale unverified destination data and abandoned `.partial-*` files before each attempt; failed payloads are never marked `verified`.
- Multipart field `priority` on ingestion is optional (default `3`), values must be `0`–`5`, and the current scheduler orders `5` first and `0` last.
- The local Compose enrollment token is for development only. Production needs a unique secret, TLS, stronger node identity/lifecycle controls, rate limits, monitoring, backup/restore testing, and a documented credential rotation/revocation process.
- The local three-node Compose setup demonstrates placement and repair logic on one physical host only; it does not provide independent failure domains. Production nodes should run on separate hosts/storage systems.
- The reconciler can queue a replacement only when at least one verified source and a different eligible target node are online. If one of exactly two nodes is offline, the remaining copy stays degraded until the offline node returns or a third node joins.
- Source-page crawling, direct object upload from external sources, central delivery gateway, OTel/Grafana, and public request escalation are later milestones.
- Do not expose the service publicly until TLS termination, secrets, backups, rate limiting, and operational controls are configured.

### Optional fourth node for surplus-replica cleanup testing

The Compose file includes `node-agent-d` with its own `node_data_d` volume. It is intended for local testing of the cleanup rule that retains two required replicas plus one spare. To test the real transfer and deletion path, first bring node D online, temporarily stop nodes B and C so the normal repair path creates a verified copy on D, then restart B and C. After all four copies and all relevant nodes have remained stable for at least five minutes, reconciliation should delete only the newest surplus placement. Do not remove Docker volumes or edit placement rows manually.

## Automated surplus-replica regression test

Run the surplus cleanup integration tests against a disposable PostgreSQL instance on `127.0.0.1:55432`:

```sh
./scripts/test-surplus-cleanup.sh
```

The script starts a separate `postgres-test` service using `docker-compose.test.yml`, applies the Atlas SQL migrations, runs the `internal/api` integration tests, and removes only that temporary test container/network on exit. It does not connect to the application's PostgreSQL service on port `5432` and does not remove Docker volumes. The integration test refuses to run unless the database URL points to localhost database `archive_test`.

The test covers stable four-replica cleanup, idempotency, retaining three replicas, blocking cleanup while a task is active, and cooldowns after a node comes online or a replica is verified.
