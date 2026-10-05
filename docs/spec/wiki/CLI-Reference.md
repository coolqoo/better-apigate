# CLI Reference

The `apigate` CLI provides commands for managing your API gateway.

---

## Global Options

```bash
apigate [command] [flags]

Global Flags:
  -c, --config string   Config file path (default "apigate.yaml")
  -h, --help            Show help
```

---

## Server Commands

### Start Server

```bash
# Start with defaults
better-apigate serve

# Custom port (via config or environment)
APIGATE_SERVER_PORT=8080 better-apigate serve
```

### Interactive Setup

```bash
better-apigate init
```

### Validate Configuration

```bash
better-apigate validate
```

### Version

```bash
better-apigate version
```

---

## Admin User Management

Admin users can log into the web dashboard.

```bash
# List admin users
better-apigate admin list

# Create admin user
better-apigate admin create --email admin@example.com
# You will be prompted for a password

# Reset admin password
better-apigate admin reset-password admin@example.com

# Delete admin user
better-apigate admin delete admin@example.com
```

---

## User Management

```bash
# List users
apigate users list

# Get user by ID or email
apigate users get <user-id-or-email>

# Create user
apigate users create --email user@example.com --name "John Doe"

# Activate/deactivate user
apigate users activate <user-id-or-email>
apigate users deactivate <user-id-or-email>

# Set user password
apigate users set-password <user-id-or-email>

# Delete user
apigate users delete <user-id>
```

**Note**: `apigate users` is deprecated. Use `better-apigate mod users` instead.

---

## Plan Management

```bash
# List plans
apigate plans list

# Get plan details
apigate plans get <plan-id>

# Create plan
apigate plans create \
  --id pro \
  --name "Pro" \
  --rate-limit 600 \
  --requests 100000 \
  --price 2900

# Enable/disable plan
apigate plans enable <plan-id>
apigate plans disable <plan-id>

# Delete plan
apigate plans delete <plan-id>
```

**Available flags for `plans create`:**
- `--id` (required) - Plan ID
- `--name` (required) - Plan name
- `--description` - Plan description
- `--rate-limit` - Requests per minute (default: 60)
- `--requests` - Requests per month, -1 = unlimited (default: 1000)
- `--price` - Monthly price in cents (default: 0)
- `--overage` - Overage price in cents per request (default: 0)
- `--default` - Set as default plan

**Note**: `apigate plans` is deprecated. Use `better-apigate mod plans` instead.

---

## API Key Management

```bash
# List all keys
apigate keys list

# List keys for a user
apigate keys list --user <user-id>

# Create key
apigate keys create --user <user-id>
apigate keys create --user <user-id> --name "Production Key"

# Revoke key
apigate keys revoke <key-id>
```

**Note**: `apigate keys` is deprecated. Use `better-apigate mod api_keys` instead.

---

## Route Management

```bash
# List routes
apigate routes list

# Get route details
apigate routes get <route-id>

# Create route (--name, --path, --upstream are required)
apigate routes create \
  --name "API v1" \
  --path "/api/v1/*" \
  --upstream <upstream-id>

# Create with all options
apigate routes create \
  --name "API v1" \
  --path "/api/v1/*" \
  --upstream <upstream-id> \
  --match prefix \
  --methods "GET,POST" \
  --protocol http \
  --priority 10 \
  --rewrite "/v1"

# Enable/disable route
apigate routes enable <route-id>
apigate routes disable <route-id>

# Delete route
apigate routes delete <route-id>
```

**Available flags for `routes create`:**
- `--name` (required) - Route name
- `--path` (required) - Path pattern to match
- `--upstream` (required) - Upstream ID
- `--match` - Match type: exact, prefix, regex (default: prefix)
- `--methods` - HTTP methods, comma-separated (empty = all)
- `--protocol` - Protocol: http, http_stream, sse, websocket (default: http)
- `--priority` - Route priority, higher matches first (default: 0)
- `--rewrite` - Path rewrite expression

**Note**: `apigate routes` is deprecated. Use `better-apigate mod routes` instead.

---

## Settings Management

Settings are stored in the database and can be any key-value pair.

```bash
# List all settings
better-apigate settings list

# Get a setting
better-apigate settings get <key>

# Set a setting
better-apigate settings set <key> <value>

# Set encrypted setting (for secrets)
better-apigate settings set <key> <value> --encrypted

# Delete a setting
better-apigate settings delete <key>
```

---

## Usage Statistics

Usage commands require specifying a user via `--user` or `--email`:

```bash
# Usage summary for current period
better-apigate usage summary --user <user-id>
better-apigate usage summary --email user@example.com

# Usage history (last N periods)
better-apigate usage history --user <user-id>
better-apigate usage history --user <user-id> --periods 12

# Recent requests
better-apigate usage recent --user <user-id>
better-apigate usage recent --email user@example.com --limit 50
```

**Available flags:**
- `--user` - User ID
- `--email` - User email (alternative to --user)
- `--periods` - Number of periods for history (default: 6)
- `--limit` - Number of recent requests (default: 20)

---

## Module-Based Commands

The `better-apigate mod` command provides CRUD operations through the module system:

```bash
# List available modules
better-apigate mod

# Examples
better-apigate mod users list
better-apigate mod plans get free
better-apigate mod upstreams create --name "API" --url "https://api.example.com"
better-apigate mod routes list
better-apigate mod api_keys list
better-apigate mod groups list
better-apigate mod certificates list
```

**Available modules:**
- `users` - User accounts
- `plans` - Pricing plans
- `routes` - API routes
- `upstreams` - Backend services
- `api_keys` - API keys
- `groups` - Team/organization groups
- `group_members` - Group memberships
- `certificates` - TLS certificates
- `webhooks` - Webhook configurations
- `settings` - System settings

---

## Interactive Shell

```bash
better-apigate shell
```

Starts an interactive shell for running multiple commands.

---

## Environment Variables

Configuration via environment variables (see [[Configuration]] for full list):

```bash
# Required
APIGATE_UPSTREAM_URL=https://api.backend.com

# Server
APIGATE_SERVER_HOST=0.0.0.0
APIGATE_SERVER_PORT=8080

# Database
APIGATE_DATABASE_DSN=apigate.db

# Logging
APIGATE_LOG_LEVEL=info
APIGATE_LOG_FORMAT=json

# Features
APIGATE_METRICS_ENABLED=true
APIGATE_OPENAPI_ENABLED=true
```

---

## See Also

- [[Configuration]] - Full configuration reference
- [[Module-System]] - Module-based architecture
