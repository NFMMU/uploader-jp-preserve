# uploader.jp 魚拓作戦 — MVP-2: single-node preservation

Go API, PostgreSQL catalog, and a Go node agent. This implementation follows design document v1.1 and the decisions made during the project conversation.

## MVP-2 scope

- A Go node agent enrolls with the central API using a shared enrollment secret, then stores its per-node bearer token in a persistent file with mode `0600`.
- The agent reports actual filesystem capacity/free space, managed-object usage, the participant's requested quota, and its last-seen time.
- An administrator uploads a file to temporary central staging. The API computes its SHA-256 and size, creates a storage task and placement, and keeps public downloads disabled.
- The node agent polls for work, claims a task with a lease, downloads the staged bytes over the authenticated API, writes to its managed storage directory, re-hashes the final on-disk file, and reports the result.
- The API compares the reported size/hash against the upload-time size/hash. Only a match marks the placement and file object `verified`.
- Failures are retried up to five attempts. Expired task leases can be reclaimed; lease tokens prevent an older attempt from reporting over a newer attempt.
- Task results are recorded in the audit log.

This is the single-node MVP-2 slice, not yet multi-node replication or production deployment. The API stages a temporary copy centrally until the node confirms verification. Use only test or authorized content while validating this workflow.

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
# Edit .env and set a unique JWT_SECRET (32+ bytes) and bootstrap admin password (12+ chars).
# For a public deployment, also replace NODE_ENROLLMENT_TOKEN with a random secret (32+ chars).
docker compose up --build
```

Compose runs PostgreSQL, applies Atlas migrations, starts the API, and starts a local Go node agent. The node agent enrolls automatically and saves its credentials in the persistent `node_data` volume. On first startup, allow a short time for its first heartbeat.

```sh
curl -i http://localhost:8080/healthz
docker compose ps
docker compose logs --tail=100 node-agent
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

## End-to-end MVP-2 smoke test

Set the access token from login. Do not paste credentials or bearer tokens into logs or chat.

```sh
export TOKEN='your-admin-access-token'

# 1. Confirm that the local node agent has enrolled and is online.
curl -sS http://localhost:8080/api/v1/nodes \
  -H "Authorization: Bearer $TOKEN"

# 2. Create a catalog entry (the source URL must be an HTTPS uploader.jp URL).
ARTIFACT=$(curl -sS -X POST http://localhost:8080/api/v1/artifacts \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"title":"MVP-2 test artifact","source_url":"https://www.uploader.jp/"}')
export ARTIFACT_ID=$(printf '%s' "$ARTIFACT" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')

# 3. Create the logical file record. It remains private regardless of policy.
FILE=$(curl -sS -X POST "http://localhost:8080/api/v1/artifacts/$ARTIFACT_ID/files" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"original_filename":"sample.txt","size_bytes":0,"media_type":"text/plain","redistribution_policy":"pending"}')
export FILE_ID=$(printf '%s' "$FILE" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')

# 4. Upload a small test file and queue storage on the node agent.
printf 'uploader.jp archive MVP-2 test\n' > /tmp/archive-sample.txt
curl -i -X POST "http://localhost:8080/api/v1/files/$FILE_ID/ingest" \
  -H "Authorization: Bearer $TOKEN" \
  -F 'file=@/tmp/archive-sample.txt;type=text/plain' -F 'priority=3'

# 5. Wait a few seconds, then inspect task and file status.
curl -sS 'http://localhost:8080/api/v1/tasks' \
  -H "Authorization: Bearer $TOKEN"
curl -sS "http://localhost:8080/api/v1/files/$FILE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

Expected result: the ingest endpoint returns `202 Accepted`; the task becomes `completed`; the placement and file preservation status become `verified`; the file has a SHA-256 value and actual size; `public_download_enabled` remains `false` because the redistribution policy is `pending`.

To inspect node state or task failures:

```sh
curl -sS http://localhost:8080/api/v1/nodes -H "Authorization: Bearer $TOKEN"
curl -sS 'http://localhost:8080/api/v1/tasks?status=failed' -H "Authorization: Bearer $TOKEN"
docker compose logs --tail=200 node-agent api migrate
```

## MVP-1 API surface

- `POST /api/v1/auth/login`, `GET /api/v1/me`
- `GET /api/v1/artifacts?q=...`, `POST /api/v1/artifacts`, `GET /api/v1/artifacts/:id`
- `POST /api/v1/artifacts/:id/files`, `GET /api/v1/files`, `GET /api/v1/files/:id`
- `PATCH /api/v1/files/:id/policy`, `PATCH /api/v1/files/:id/publication`
- `GET /api/v1/export?format=json|csv`
- `POST /api/v1/public-requests`, `GET /api/v1/public-requests`, `PATCH /api/v1/public-requests/:id`
- `POST /api/v1/admin/users`, `GET /api/v1/admin/users`, `GET /api/v1/admin/audit-logs`

## MVP-2 API surface

- `POST /api/v1/nodes/register` — one-time enrollment with `X-Node-Enrollment-Token`
- `POST /api/v1/nodes/:id/heartbeat` — node-token authenticated capacity report
- `GET /api/v1/nodes` — admin/reviewer node list
- `GET /api/v1/tasks?status=...` — admin/reviewer task list
- `POST /api/v1/files/:id/ingest` — admin-only multipart upload (`file` field)
- `POST /api/v1/nodes/:id/tasks/claim` — node-token authenticated task claim
- `GET /api/v1/nodes/:id/tasks/:taskID/content?lease_token=...` — authenticated task content
- `POST /api/v1/nodes/:id/tasks/:taskID/result` — authenticated hash/size result

The node credential is returned only at enrollment; the database stores only its SHA-256 hash. Do not expose the enrollment secret. Node storage is not exposed directly to end users.

## Capacity settings

- `NODE_CAPACITY_BYTES` is the participant's requested quota. `0` means no participant-defined quota; actual free disk space still constrains scheduling.
- The agent reports actual filesystem total/free bytes and the size of managed objects. The API checks both actual available space and requested quota before assigning an upload.
- Capacity is re-reported every 30 seconds. The API considers a node schedulable only when its heartbeat is recent.

## Safety defaults and limitations

- New file objects default to `preservation_status=uncollected`, `redistribution_policy=unknown`, and `public_download_enabled=false`.
- Preserving a file is separate from permission to redistribute it. MVP-2 does not provide a public download route.
- Uploads are limited to 5 GiB per request in this MVP. Large-file resumable uploads are not implemented.
- Task attempts are limited to five; multipart field `priority` is optional (default `3`), values must be `0`–`5`, and the current scheduler orders `5` first and `0` last.
- The local Compose enrollment token is for development only. Production needs a unique secret, TLS, stronger node identity/lifecycle controls, rate limits, monitoring, backup/restore testing, and a documented credential rotation/revocation process.
- Multi-node replication, source-page crawling, direct object upload from external sources, central delivery gateway, node removal, OTel/Grafana, and public request escalation are later milestones.
- Do not expose the service publicly until TLS termination, secrets, backups, rate limiting, and operational controls are configured.
