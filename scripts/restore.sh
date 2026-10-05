#!/bin/sh
set -eu
if [ "$#" -ne 1 ]; then printf '%s\n' 'Usage: scripts/restore.sh BACKUP.dump'; exit 1; fi
if [ ! -r "$1" ]; then printf '%s\n' 'Backup is not readable.' >&2; exit 1; fi
# Stop every gateway using this database before an operator-approved restore.
docker compose --profile replica stop gateway gateway-two
docker compose run --rm --no-deps -T postgres-tools -c 'exec pg_restore --dbname="$APIGATE_DATABASE_DSN" --clean --if-exists --single-transaction' < "$1"
printf '%s\n' 'Restore complete. Verify the ledger and restart the gateway explicitly.'
