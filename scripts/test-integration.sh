#!/bin/sh
set -eu

cd "$(dirname "$0")/.."
compose_file="docker-compose.test.yml"
project_name="uploader-jp-archive-integration-test"

test_compose() {
  docker compose -p "$project_name" -f "$compose_file" "$@"
}

cleanup() {
  test_compose down >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

test_compose up -d --wait postgres-test
for migration in atlas/migrations/*.sql; do
  # Atlas wraps each file in a transaction unless the migration opts out with
  # "atlas:txmode none". Mirror that here so statements like LOCK TABLE work
  # inside a transaction while CREATE INDEX CONCURRENTLY runs outside one.
  if grep -q 'atlas:txmode none' "$migration"; then
    test_compose exec -T postgres-test psql -U archive_test -d archive_test -v ON_ERROR_STOP=1 < "$migration"
  else
    test_compose exec -T postgres-test psql -U archive_test -d archive_test -v ON_ERROR_STOP=1 -1 < "$migration"
  fi
done

TEST_DATABASE_URL='postgres://archive_test:archive_test_password@127.0.0.1:55432/archive_test?sslmode=disable' \
  RUN_CATALOG_DB_INTEGRATION=1 \
  RUN_SURPLUS_DB_INTEGRATION=1 \
  go test -race -coverprofile=coverage.out ./...
