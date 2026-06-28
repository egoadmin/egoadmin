#!/bin/sh
set -e

SERVICE_NAME=${SERVICE:-${ATLAS_SERVICE:-}}

if [ -n "$ATLAS_MIGRATION_DIR" ]; then
  MIGRATION_DIR=$ATLAS_MIGRATION_DIR
elif [ -n "$SERVICE_NAME" ]; then
  MIGRATION_DIR="file://atlas/migrations/$SERVICE_NAME"
else
  echo "Error: SERVICE or ATLAS_SERVICE is not set. Use gateway or user." >&2
  exit 1
fi

if [ -z "$ATLAS_URL" ]; then
  echo "Error: ATLAS_URL is not set." >&2
  exit 1
fi

if ! command -v atlas >/dev/null 2>&1; then
  echo "Error: atlas command is not available." >&2
  exit 1
fi

atlas migrate apply \
  --url "$ATLAS_URL" \
  --dir "$MIGRATION_DIR"
