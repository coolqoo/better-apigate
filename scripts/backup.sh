#!/bin/sh
set -eu
umask 077
apigate_backup=${1:-"better-apigate-$(date -u +%Y%m%dT%H%M%SZ).dump"}
# The connection URL is supplied to the client through its environment.
docker compose run --rm --no-deps -T postgres-tools -c 'exec pg_dump --dbname="$APIGATE_DATABASE_DSN" --format=custom' > "$apigate_backup"
printf 'Saved PostgreSQL backup to %s\n' "$apigate_backup"
