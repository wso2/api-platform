# File-Based Authentication

File-based auth (also called `basic` mode) stores a user list in the Platform API configuration file. It requires no external IDP and is intended for local development and demos.

## How It Works

When `auth_mode = "basic"`, the AI Workspace login page renders a username/password form. Credentials are sent to the Platform API, which validates them against a hashed user list in `config-platform-api.toml`. On success, the Platform API issues a signed JWT that the UI stores and sends with subsequent API requests.

## Configuration

### 1. Set auth mode in `configs/config.toml`

```toml
auth_mode = "basic"
```

### 2. Define users in `configs/config-platform-api.toml`

```toml
[auth.file_based]
enabled = true

[[auth.file_based.users]]
username     = "admin"
password_hash = "$2a$10$..."   # bcrypt hash of the password
role         = "admin"

[[auth.file_based.users]]
username     = "viewer"
password_hash = "$2a$10$..."
role         = "viewer"
```

### 3. Generate password hashes

Use any standard bcrypt tool. Example with `htpasswd` (Apache utils):

```bash
htpasswd -bnBC 10 "" "your-password" | tr -d ':\n'
```

Or with Python:

```python
import bcrypt
print(bcrypt.hashpw(b"your-password", bcrypt.gensalt(rounds=10)).decode())
```

The Quick Start bundle ships with a default `admin` / `admin` credential — **change this before any shared deployment**.

## Default Credentials (Quick Start only)

| Username | Password |
|----------|----------|
| `admin`  | `admin`  |

## Organization in File-Based Mode

In file-based mode, all users belong to a single organization defined in `config-platform-api.toml`:

```toml
[auth.file_based.organization]
id     = ""     # Leave empty to auto-generate a UUID on first start
name   = "My Organization"
handle = "my-org"
```

If `id` is left empty, the Platform API generates a stable UUID on first startup and writes it back to the config. This UUID persists across restarts as long as the config file is preserved.

## Limitations

- Single organization only (multi-tenancy requires OIDC).
- User list is static — changes require restarting the Platform API container.
- Not suitable for production or shared environments.

For production, switch to [OIDC auth](oidc-auth.md).
