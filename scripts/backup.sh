#!/bin/sh
set -eu
umask 077
apigate_backup=${1:-"apigate-$(date -u +%Y%m%dT%H%M%SZ).dump"}
docker compose exec -T postgres pg_dump -U apigate -d apigate --format=custom > "$apigate_backup"
printf 'Saved PostgreSQL backup to %s\n' "$apigate_backup"
