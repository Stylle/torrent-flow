#!/bin/sh
set -eu
if [ "$(id -u)" = 0 ]; then
  mkdir -p "${DATA_DIR:-/data}"
  chown flow:flow "${DATA_DIR:-/data}"
  exec su-exec flow:flow /app/flow "$@"
fi
exec /app/flow "$@"
