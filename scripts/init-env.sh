#!/bin/sh
set -eu
if [ -e .env ]; then printf '%s\n' '.env already exists; keeping its values.'; exit 0; fi
umask 077
apigate_key_secret=$(openssl rand -hex 32)
apigate_setup_token=$(openssl rand -hex 32)
cat > .env <<ENV
# Add the connection URLs for your existing PostgreSQL and Redis services.
APIGATE_DATABASE_DSN=
APIGATE_REDIS_URL=
APIGATE_API_KEY_SECRET=$apigate_key_secret
APIGATE_SETUP_TOKEN=$apigate_setup_token
APIGATE_PUBLIC_URL=http://localhost:8080
APIGATE_IMAGE=coolqoo/better-apigate:1.0.0
ENV
printf '%s\n' 'Created .env with fresh gateway secrets. Set APIGATE_DATABASE_DSN and APIGATE_REDIS_URL before starting Docker Compose.'
