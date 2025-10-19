# Quick Start: SQLite Migration for Gateway-Controller

**Feature**: Database Migration from BBolt to SQLite
**Branch**: `005-migrate-sqlite-db`
**Audience**: Gateway operators, DevOps engineers, developers

## Overview

The gateway-controller has migrated from BBolt to SQLite for persistent storage of API configurations. This guide covers:
- What changed and why
- How to configure the new SQLite storage
- Migration path for existing deployments
- Troubleshooting common issues

---

## What Changed

### Before (BBolt)

```yaml
# config.yaml (OLD - no longer supported)
storage:
  mode: persistent
  database_path: /data/gateway-controller.db  # BBolt database

# Features:
# - Embedded key-value store
# - Audit logging enabled
# - No SQL query capabilities
```

### After (SQLite)

```yaml
# config.yaml (NEW)
storage:
  type: sqlite
  sqlite:
    path: /data/gateway.db  # SQLite database file path

# Features:
# - Embedded relational database
# - Standard SQL tooling for inspection
# - NO audit logging (feature removed)
# - Better ecosystem support
# - Future migration path to PostgreSQL
```

### Key Differences

| Aspect | BBolt (Old) | SQLite (New) |
|--------|-------------|--------------|
| **Database Type** | Key-value store | Relational database |
| **Query Tools** | None (binary format) | sqlite3 CLI, DB Browser |
| **Audit Logging** | ✅ Enabled | ❌ Removed |
| **Config Setting** | `storage.database_path` | `storage.type` + `storage.sqlite.path` |
| **File Extensions** | `.db` | `.db`, `.db-wal`, `.db-shm` |
| **Migration Support** | Limited | Easy (SQL export/import) |

---

## For New Deployments

### 1. Configuration (Persistent Mode)

**Default configuration** (`config.yaml`):

```yaml
# Server configuration
server:
  api_port: 9090
  xds_port: 18000
  shutdown_timeout: 15s

# Storage configuration (SQLite)
storage:
  type: sqlite
  sqlite:
    path: /data/gateway.db  # SQLite database file path

# Router configuration
router:
  access_logs:
    enabled: true
    format: json
  listener_port: 8080

# Logging
logging:
  level: info
  format: json
```

### 2. Start Gateway-Controller

#### Docker (Recommended)

```bash
docker run -d \
  --name gateway-controller \
  -p 9090:9090 \
  -p 18000:18000 \
  -v $(pwd)/data:/data \
  wso2/gateway-controller:latest
```

**Important**: Mount `/data` volume to persist the SQLite database across container restarts.

#### Binary

```bash
# Ensure data directory exists
mkdir -p ./data

# Start controller
./bin/controller --config /path/to/config.yaml
```

### 3. Verify Database Creation

```bash
# Check that database files were created
ls -lh ./data/

# Expected output:
# gateway.db        (main database file)
# gateway.db-wal    (Write-Ahead Log - transactions)
# gateway.db-shm    (shared memory for WAL)
```

### 4. Test API Configuration

```bash
# Create an API configuration
curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<EOF
version: api-platform.wso2.com/v1
kind: http/rest
data:
  name: Test API
  version: v1.0
  context: /test
  upstream:
    - url: http://httpbin.org
  operations:
    - method: GET
      path: /get
EOF

# Verify it was persisted
curl http://localhost:9090/apis

# Restart controller
docker restart gateway-controller  # or kill and restart binary

# Verify configuration survived restart
curl http://localhost:9090/apis
```

---

## For Existing Deployments (Migration from BBolt)

### Migration Strategy

**IMPORTANT**: No automated migration is provided. This is a clean-slate migration.

#### Option 1: Recreate Configurations (Recommended)

1. **Export existing API configurations** before upgrade:
   ```bash
   # Save all API configurations to YAML files
   curl http://localhost:9090/apis > apis-backup.json
   ```

2. **Upgrade gateway-controller** to SQLite version

3. **Re-apply API configurations** via REST API:
   ```bash
   # Recreate each API from backup
   curl -X POST http://localhost:9090/apis \
     -H "Content-Type: application/yaml" \
     --data-binary @weather-api.yaml
   ```

#### Option 2: Manual Data Migration (Advanced)

If you have many configurations and want to preserve timestamps:

1. **Stop gateway-controller**
   ```bash
   docker stop gateway-controller
   ```

2. **Extract data from BBolt** (Go script required):
   ```go
   // extract-bbolt.go
   package main

   import (
       "encoding/json"
       "fmt"
       "os"
       "go.etcd.io/bbolt"
   )

   func main() {
       db, _ := bbolt.Open("./data/gateway-controller.db", 0600, nil)
       defer db.Close()

       db.View(func(tx *bbolt.Tx) error {
           bucket := tx.Bucket([]byte("apis"))
           if bucket == nil {
               return fmt.Errorf("apis bucket not found")
           }

           c := bucket.Cursor()
           for k, v := c.First(); k != nil; k, v = c.Next() {
               fmt.Printf("%s\n", v)  // Print JSON
           }
           return nil
       })
   }
   ```

   ```bash
   go run extract-bbolt.go > apis-export.jsonl
   ```

3. **Start new gateway-controller** with SQLite

4. **Import configurations** via REST API

### Migration Checklist

- [ ] Backup existing BBolt database (`/data/gateway-controller.db`)
- [ ] Export all API configurations via REST API
- [ ] Update configuration file (`storage.database_path` → `storage.type` and `storage.sqlite.path`)
- [ ] Remove audit logging references (no longer supported)
- [ ] Upgrade gateway-controller to SQLite version
- [ ] Verify new database files created (`gateway.db`, `gateway.db-wal`, `gateway.db-shm`)
- [ ] Re-apply API configurations
- [ ] Verify Envoy receives xDS updates correctly
- [ ] Test API traffic through gateway
- [ ] Delete old BBolt database after confirming migration success

---

## Memory-Only Mode (Testing)

For development or testing without persistent storage:

```yaml
# config-memory-only.yaml
server:
  api_port: 9090
  xds_port: 18000

storage:
  type: memory  # No database files created

router:
  listener_port: 8080

logging:
  level: debug
  format: console
```

**Behavior**:
- No database files created
- API configurations lost on restart
- Faster startup (no database loading)
- Ideal for CI/CD testing

```bash
# Start in memory-only mode
./bin/controller --config config-memory-only.yaml

# Or via environment variable
GC_STORAGE_TYPE=memory ./bin/controller
```

---

## Operational Tasks

### Inspecting the Database

#### Using sqlite3 CLI

```bash
# Connect to database
sqlite3 ./data/gateway.db

# List all APIs
sqlite> SELECT name, version, status FROM api_configs;

# View specific API (pretty-print JSON)
sqlite> SELECT json(configuration) FROM api_configs WHERE name = 'Weather API';

# Count configurations by status
sqlite> SELECT status, COUNT(*) FROM api_configs GROUP BY status;

# Exit
sqlite> .quit
```

#### Using DB Browser for SQLite (GUI)

1. Download from https://sqlitebrowser.org/
2. Open `./data/gateway.db`
3. Browse tables, run queries, export data

### Database Backup

```bash
# 1. Checkpoint WAL to consolidate changes
sqlite3 ./data/gateway.db "PRAGMA wal_checkpoint(TRUNCATE);"

# 2. Copy database file
cp ./data/gateway.db ./backups/gateway-$(date +%Y%m%d-%H%M%S).db

# 3. Verify backup integrity
sqlite3 ./backups/gateway-*.db "PRAGMA integrity_check;"
```

**Automated Backup** (cron job):

```bash
# /etc/cron.daily/gateway-db-backup.sh
#!/bin/bash
sqlite3 /data/gateway.db "PRAGMA wal_checkpoint(TRUNCATE);"
cp /data/gateway.db /backups/gateway-$(date +%Y%m%d).db
find /backups -name "gateway-*.db" -mtime +7 -delete  # Keep 7 days
```

### Database Restore

```bash
# 1. Stop gateway-controller
docker stop gateway-controller

# 2. Replace database file
cp ./backups/gateway-20251019.db ./data/gateway.db

# 3. Remove WAL files (force clean state)
rm -f ./data/gateway.db-wal ./data/gateway.db-shm

# 4. Verify integrity
sqlite3 ./data/gateway.db "PRAGMA integrity_check;"

# 5. Start gateway-controller
docker start gateway-controller
```

---

## Troubleshooting

### Issue 1: Database Locked Error

**Symptom**:
```
ERROR: failed to open database: database is locked
```

**Cause**: Another process has the database file open, or unclean shutdown left stale lock files.

**Solution**:
```bash
# 1. Check for processes using the database
lsof ./data/gateway.db

# 2. Kill any stray processes
kill -9 <PID>

# 3. Remove WAL and SHM files (only if no processes running)
rm -f ./data/gateway.db-wal ./data/gateway.db-shm

# 4. Restart gateway-controller
docker restart gateway-controller
```

### Issue 2: Database File Not Created

**Symptom**:
```
ERROR: unable to create database file: permission denied
```

**Cause**: Data directory doesn't exist or has incorrect permissions.

**Solution**:
```bash
# Create data directory
mkdir -p ./data

# Fix permissions (for Docker)
chmod 777 ./data

# Restart gateway-controller
docker restart gateway-controller
```

### Issue 3: Configurations Lost After Restart

**Symptom**: All API configurations disappear after restarting gateway-controller.

**Cause**: Running in memory-only mode or data volume not mounted.

**Solution**:
```bash
# Check storage configuration
grep -A 3 "storage:" config.yaml

# Verify it's set to sqlite
storage:
  type: sqlite
  sqlite:
    path: /data/gateway.db

# For Docker, ensure volume is mounted
docker run -v $(pwd)/data:/data ...
```

### Issue 4: Corrupted Database

**Symptom**:
```
ERROR: database disk image is malformed
```

**Cause**: Unclean shutdown, disk failure, or file system corruption.

**Solution**:
```bash
# 1. Attempt recovery
sqlite3 ./data/gateway.db ".recover" | sqlite3 ./data/gateway-recovered.db

# 2. Verify recovered database
sqlite3 ./data/gateway-recovered.db "PRAGMA integrity_check;"

# 3. Replace original
mv ./data/gateway.db ./data/gateway-corrupted.db.bak
mv ./data/gateway-recovered.db ./data/gateway.db

# 4. If recovery fails, restore from backup
cp ./backups/gateway-latest.db ./data/gateway.db
```

---

## Performance Tuning

### Expected Performance

For 100 API configurations:

| Operation | Expected Time |
|-----------|---------------|
| Get API by ID | < 10ms |
| Get API by name/version | < 50ms |
| Create API | < 100ms |
| Update API | < 100ms |
| List all APIs | < 200ms |

### Monitoring Database Size

```bash
# Check database file size
du -h ./data/gateway.db

# View detailed stats
sqlite3 ./data/gateway.db ".dbinfo"

# Expected growth: ~5-10 KB per API configuration
```

### WAL Checkpointing

WAL files can grow unbounded if not checkpointed. Automated checkpointing runs on close, but for long-running instances:

```bash
# Manual checkpoint (consolidates WAL into main database)
sqlite3 ./data/gateway.db "PRAGMA wal_checkpoint(TRUNCATE);"

# Automate with cron (daily at 2 AM)
0 2 * * * sqlite3 /data/gateway.db "PRAGMA wal_checkpoint(TRUNCATE);"
```

---

## Configuration Reference

### Storage Settings

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `storage.type` | string | `sqlite` | Storage type: "sqlite", "postgres" (future), or "memory" |
| `storage.sqlite.path` | string | `/data/gateway.db` | SQLite database file path (when type=sqlite) |
| `storage.postgres.host` | string | N/A | PostgreSQL host (future support) |
| `storage.postgres.port` | integer | N/A | PostgreSQL port (future support) |
| `storage.postgres.database` | string | N/A | PostgreSQL database name (future support) |
| `storage.postgres.user` | string | N/A | PostgreSQL username (future support) |
| `storage.postgres.password` | string | N/A | PostgreSQL password (future support) |
| `storage.postgres.sslmode` | string | N/A | PostgreSQL SSL mode (future support) |

### Environment Variable Overrides

```bash
# Override storage type
export GC_STORAGE_TYPE=memory

# Override SQLite database path
export GC_STORAGE_SQLITE_PATH=/custom/path/gateway.db

# Start controller
./bin/controller
```

---

## Next Steps

1. **Read the data model**: See [data-model.md](./data-model.md) for detailed schema documentation
2. **Review implementation plan**: See [plan.md](./plan.md) for technical details
3. **Test your deployment**: Use memory-only mode for local testing before production
4. **Set up backups**: Implement automated daily backups
5. **Monitor performance**: Track API operation latencies and database growth

---

## FAQ

### Q: Can I use an external database like PostgreSQL?

**A**: Not yet. SQLite is embedded for simplicity. Future versions will support PostgreSQL for multi-instance deployments via:

```yaml
storage:
  type: postgres
  postgres:
    host: localhost
    port: 5432
    database: gateway
    user: gateway_user
    password: secret
    sslmode: require
```

The schema is designed for easy migration to PostgreSQL.

### Q: What happened to audit logging?

**A**: The audit logging feature was removed to simplify the codebase. If you need audit trails, consider:
- Application-level logging (structured logs via Zap)
- External observability tools (Datadog, Prometheus, Grafana)
- Database query logs (`sqlite3 .log stderr`)

### Q: Can I run multiple gateway-controller instances with one SQLite database?

**A**: **No**. SQLite is designed for single-process access. For multi-instance deployments, plan to migrate to PostgreSQL in the future.

### Q: How do I downgrade back to BBolt if needed?

**A**: Downgrade is **not supported**. The BBolt implementation has been completely removed. If you need to revert:
1. Export API configurations from SQLite version
2. Downgrade to BBolt version
3. Re-apply configurations via REST API

### Q: Is the database encrypted?

**A**: No. SQLite databases are stored in plain text. If you need encryption:
- Use filesystem-level encryption (LUKS, BitLocker, etc.)
- Consider external databases with encryption support (PostgreSQL with SSL)

### Q: Can I query the database while the controller is running?

**A**: **Yes**, for read-only queries. SQLite's WAL mode enables concurrent reads. Use read-only mode to be safe:

```bash
sqlite3 "file:./data/gateway.db?mode=ro" "SELECT * FROM api_configs;"
```

---

## Support

- **Documentation**: See `gateway/gateway-controller/README.md`
- **Issues**: Report bugs at `https://github.com/wso2/api-platform/issues`
- **Logs**: Check gateway-controller logs for detailed error messages
  ```bash
  docker logs gateway-controller
  ```
