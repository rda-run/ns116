# NS116 — DNS Manager

Open-source web interface for managing AWS Route53 DNS records with
multi-user support, role-based access control, and audit logging.

## Features

- **DNS Management** — Create, edit, and delete
  DNS records through a modern web UI (Highway Style)
- **Multi-User** — Multiple users with `admin` and `editor` roles
- **First-Run Setup** — Web-based initial admin account
  creation on first launch
- **Persistent Sessions** — PostgreSQL-backed sessions that
  survive server restarts
- **DNS Caching** — Zones and records are cached locally
  (5-min TTL) to reduce AWS API calls
- **Audit Logging** — All actions (login, logout, record
  changes, user management) are logged
- **Single Binary** — All assets (templates, CSS, JS,
  images, migrations) are embedded into the binary
- **PostgreSQL Backend** — Robust data storage with full SQL support

## Quick Start

### Prerequisites

- PostgreSQL database (v12 or later recommended)
- `config.yaml` with correct database DSN

### Setup

```bash
cp config.yaml.example config.yaml
# Edit config.yaml with your AWS credentials and Database DSN
make run
```

On first launch, the application will automatically apply database migrations.
Open `http://localhost:8080` — you'll be redirected to `/setup` to create your admin account.

## Configuration

Edit `config.yaml`:

```yaml
server:
  port: 8080
  host: "0.0.0.0"

database:
  # DSN (Data Source Name) connection string
  # Example: postgres://user:password@localhost:5432/ns116?sslmode=disable
  dsn: "postgres://ns116:ns116pass@localhost:5432/ns116?sslmode=disable"

aws:
  access_key_id: "AKIA..."
  secret_access_key: "wJal..."
  region: "us-east-1"

# Optional: restrict to specific zones (if empty, all zones are shown)
hosted_zones: []
#  - id: "Z1PA6795UKMFR9"
#    label: "example.com"
```

| Section | Description |
| --- | --- |
| `server` | Bind address and port |
| `database.dsn` | PostgreSQL connection string (including user, password, dbname) |
| `aws` | AWS credentials and region for DNS API access |
| `hosted_zones` | Optional allowlist of zone IDs to manage |

### LDAP Authentication

Optional: Enable LDAP to authenticate users against Active Directory
or OpenLDAP.

```yaml
ldap:
  enabled: true
  url: "ldaps://ldap.example.com:636"
  bind_dn: "CN=ns116-svc,OU=ServiceAccounts,DC=example,DC=com"
  bind_password: "secret"
  base_dn: "OU=Users,DC=example,DC=com"
  user_filter: "(sAMAccountName=%s)"     # or "(uid=%s)"
  username_attr: "sAMAccountName"        # or "uid"
  email_attr: "mail"
  # Optional: Custom group filter (e.g. for POSIX groups)
  # Default: (|(member=%s)(uniqueMember=%s))
  # For memberUid: (&(objectClass=posixGroup)(memberUid=%u))
  group_filter: ""
  group_mapping:
    admin: "CN=DNS-Admins,OU=Groups,DC=example,DC=com"
    editor: "CN=DNS-Editors,OU=Groups,DC=example,DC=com"
```

**Auto-Configuration Tool:**

We provide a helper script to automatically detect your LDAP server type (Active Directory, OpenLDAP, or POSIX) and generate the correct configuration for you.

```bash
./scripts/generate_ldap_config.sh
```

**Notes:**

1. **Group Mapping**: Access is denied unless the user belongs to
    at least one mapped group.
2. **Local Fallback**: When LDAP is enabled, local password login
    is restricted to the `admin` user only (for emergency access).
    All other users must login via LDAP.
3. **Auto-Provisioning**: LDAP users are automatically created in
    the local database on first login (password is not stored).

## Build

```bash
make build    # produces ./ns116 (linux/amd64, optimized)
make run      # development mode
make clean    # remove built binary
make docker   # build Docker image
```

## Docker

```bash
docker build -t ns116 .
docker run -p 8080:8080 \
  -v ./config.yaml:/app/config.yaml \
  ns116
```

### Quick apply

```bash
kubectl apply -f k8s/
```

## Architecture

```text
internal/
├── auth/          # Session management, authentication, RBAC middleware
├── config/        # YAML configuration loading
├── database/      # PostgreSQL layer (migrations, users, sessions, cache, audit)
├── handler/       # HTTP handlers (auth, zones, records, setup, admin)
├── model/         # Data models (User, Session, AuditEntry, Zone, Record)
├── server/        # Server wiring and routing
└── service/       # AWS DNS service with caching
web/
├── static/        # CSS, JS, images (embedded)
├── templates/     # HTML templates (embedded)
└── migrations/    # SQL migration files (embedded)
```

## User Roles

| Role | Permissions |
| --- | --- |
| `admin` | Full access: manage DNS records, manage users, view audit log |
| `editor` | DNS record management only |

## License

MIT

## Disclaimer

AWS, Amazon Web Services, and Route53 are trademarks of
Amazon.com, Inc. or its affiliates. NS116 is an independent,
open-source project and is not affiliated with, endorsed by,
or sponsored by Amazon.
