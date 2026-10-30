#!/bin/sh
set -eu

cd "$(dirname "$0")/.."
compose_file="docker-compose.test.yml"
project_name="uploader-jp-archive-surplus-test"

test_compose() {
  docker compose -p "$project_name" -f "$compose_file" "$@"
}

cleanup() {
  test_compose down >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

test_compose up -d --wait postgres-test
for migration in atlas/migrations/*.sql; do
  test_compose exec -T postgres-test psql -U archive_test -d archive_test -v ON_ERROR_STOP=1 < "$migration"
done

RUN_SURPLUS_DB_INTEGRATION=1 TEST_DATABASE_URL='postgres://archive_test:archive_test_password@127.0.0.1:55432/archive_test?sslmode=disable' go test ./internal/api -run '^(TestScheduleSurplusPlacementDeletionIntegration|TestReconcileRepairsOfflineReplicaIntegration|TestRetryStoreTaskIntegration|TestRetryErrorHandlingIntegration)$' -count=1 -v
