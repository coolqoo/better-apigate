#!/bin/sh
set -eu
if [ "$#" -ne 1 ]; then printf '%s\n' 'Usage: scripts/restore.sh BACKUP.dump'; exit 1; fi
# An operator invokes this only after reviewing the backup and stopping traffic.
docker compose stop gateway gateway-two
docker compose exec -T postgres pg_restore -U apigate -d apigate --clean --if-exists --single-transaction < "$1"
printf '%s\n' 'Restore complete. Verify the ledger and restart the gateway explicitly.'
