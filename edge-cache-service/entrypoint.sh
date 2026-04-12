#!/bin/sh
set -e

mkdir -p "${CACHE__DIR}"
chown app:app "${CACHE__DIR}"
exec gosu app "$@"
