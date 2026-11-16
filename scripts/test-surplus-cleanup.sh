#!/bin/sh
set -eu
exec "$(dirname "$0")/test-integration.sh" "$@"
