#!/bin/sh
set -eu
exec sh "$(dirname "$0")/test-integration.sh" "$@"
