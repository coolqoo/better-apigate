#!/bin/sh
set -eu
if [ -e .env ]; then printf '%s\n' '.env already exists; keeping its secrets.'; exit 0; fi
umask 077
apigate_pg_password=$(openssl rand -hex 24)
apigate_redis_password=$(openssl rand -hex 24)
apigate_key_secret=$(openssl rand -hex 32)
apigate_setup_token=$(openssl rand -hex 32)
cat > .env <<ENV
POSTGRES_PASSWORD=$apigate_pg_password
POSTGRES_SHARED_BUFFERS=1GB
REDIS_PASSWORD=$apigate_redis_password
APIGATE_API_KEY_SECRET=$apigate_key_secret
APIGATE_SETUP_TOKEN=$apigate_setup_token
APIGATE_PUBLIC_URL=http://localhost:8080
ENV
printf '%s\n' 'Created .env with fresh deployment secrets. Start with docker compose up --build.'
