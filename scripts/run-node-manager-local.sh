#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
docker build -f Dockerfile.node -t archive-node-agent:local .
docker compose -f deploy/node-manager/compose.yaml build node-manager
docker compose -f deploy/node-manager/compose.yaml up -d node-manager
printf '\nArchive Node Manager: http://localhost:8780\n'
printf 'Logs: docker compose -f deploy/node-manager/compose.yaml logs -f node-manager\n'
