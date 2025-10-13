# API Configuration Lifecycle Implementation

## Overview

This document describes the complete lifecycle of an API configuration from submission through deployment, update, and deletion in the Gateway system.

## Lifecycle Stages

### 1. Configuration Submission

**User Action**: Submit API configuration via REST API

```bash
curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/json" \
  -d @api-config.yaml
```

**Gateway-Controller Processing**:

1. **Request Reception** (pkg/api/handlers/handlers.go)
   - Gin router receives POST /apis request
   - Handler extracts Content-Type (application/json or application/x-yaml)
   - Bind request body to models.APIConfig struct

2. **Parsing** (pkg/config/parser.go)
   ```go
   func ParseAPIConfig(data []byte, contentType string) (*models.APIConfig, error) {
       var config models.APIConfig

       switch contentType {
       case "application/json":
           if err := json.Unmarshal(data, &config); err != nil {
               return nil, fmt.Errorf("invalid JSON: %w", err)
           }
       case "application/x-yaml":
           if err := yaml.Unmarshal(data, &config); err != nil {
               return nil, fmt.Errorf("invalid YAML: %w", err)
           }
       default:
           return nil, errors.New("unsupported content type")
       }

       return &config, nil
   }
   ```

3. **Validation** (pkg/config/validator.go)
   ```go
   func (v *Validator) Validate(config *models.APIConfig) ValidationErrors {
       var errors ValidationErrors

       // Required field validation
       if config.Data.Name == "" {
           errors = append(errors, ValidationError{
               Field:   "data.name",
               Message: "Name is required",
           })
       }

       // Context path validation
       if !strings.HasPrefix(config.Data.Context, "/") {
           errors = append(errors, ValidationError{
               Field:   "data.context",
               Message: "Context must start with /",
           })
       }

       // Operations validation
       for i, op := range config.Data.Operations {
           if !isValidHTTPMethod(op.Method) {
               errors = append(errors, ValidationError{
                   Field:   fmt.Sprintf("data.operations[%d].method", i),
                   Message: fmt.Sprintf("Invalid HTTP method: %s", op.Method),
               })
           }
       }

       // Upstream URL validation
       for i, upstream := range config.Data.Upstream {
           if _, err := url.Parse(upstream.URL); err != nil {
               errors = append(errors, ValidationError{
                   Field:   fmt.Sprintf("data.upstream[%d].url", i),
                   Message: "Invalid URL format",
               })
           }
       }

       return errors
   }
   ```

4. **Identity Generation**
   ```go
   id := fmt.Sprintf("%s/%s", config.Data.Name, config.Data.Version)
   // Example: "PetStore/v1"
   ```

5. **Duplicate Check**
   ```go
   if _, err := memStore.Get(id); err == nil {
       return 409, ErrorResponse{
           Status:  "error",
           Message: fmt.Sprintf("API configuration '%s' already exists", id),
       }
   }
   ```

### 2. Persistence

**Atomic Storage Operation**:

```go
func (h *Handler) saveConfiguration(id string, config *models.APIConfig) error {
    // Start database transaction
    err := h.db.Update(func(tx *bolt.Tx) error {
        bucket := tx.Bucket([]byte("apis"))

        // Serialize configuration
        data, err := json.Marshal(config)
        if err != nil {
            return fmt.Errorf("failed to serialize config: %w", err)
        }

        // Write to database
        if err := bucket.Put([]byte(id), data); err != nil {
            return fmt.Errorf("failed to write to db: %w", err)
        }

        return nil
    })

    if err != nil {
        return err
    }

    // Update in-memory maps (only after successful DB write)
    h.memStore.Set(id, config)

    return nil
}
```

**Storage Layers**:
- **bbolt Database**: Persistent storage, survives restarts
- **In-Memory Maps**: Fast runtime access for xDS translation

### 3. xDS Snapshot Generation

**Trigger Snapshot Update**:

```go
func (h *Handler) updateXDSSnapshot() error {
    // Get ALL configurations from in-memory maps
    allConfigs := h.memStore.GetAll()

    // Generate new snapshot (SotW: complete state)
    if err := h.xdsServer.UpdateSnapshot(allConfigs); err != nil {
        h.logger.Error("Failed to update xDS snapshot", zap.Error(err))
        return err
    }

    h.logger.Info("xDS snapshot updated",
        zap.Int("config_count", len(allConfigs)),
        zap.Int64("snapshot_version", h.xdsServer.GetVersion()),
    )

    return nil
}
```

**Snapshot Creation** (pkg/xds/snapshot.go):

```go
func (sm *SnapshotManager) UpdateSnapshot(configs []*models.APIConfig) error {
    // Increment version atomically
    version := sm.version.Add(1)
    versionStr := strconv.FormatInt(version, 10)

    sm.logger.Debug("Generating xDS snapshot",
        zap.String("version", versionStr),
        zap.Int("api_count", len(configs)),
    )

    // Translate ALL configs to Envoy resources
    translator := NewTranslator()
    resources, err := translator.TranslateToEnvoyResources(configs)
    if err != nil {
        return fmt.Errorf("translation failed: %w", err)
    }

    // Create snapshot with complete state
    snapshot, err := cache.NewSnapshot(
        versionStr,
        map[resource.Type][]types.Resource{
            resource.ListenerType: resources.Listeners,
            resource.RouteType:    resources.Routes,
            resource.ClusterType:  resources.Clusters,
        },
    )
    if err != nil {
        return fmt.Errorf("snapshot creation failed: %w", err)
    }

    // Validate snapshot consistency
    if err := snapshot.Consistent(); err != nil {
        return fmt.Errorf("inconsistent snapshot: %w", err)
    }

    // Push to cache (SotW approach)
    ctx := context.Background()
    if err := sm.cache.SetSnapshot(ctx, sm.nodeID, snapshot); err != nil {
        return fmt.Errorf("failed to set snapshot: %w", err)
    }

    sm.logger.Info("xDS snapshot pushed",
        zap.String("version", versionStr),
        zap.String("node_id", sm.nodeID),
    )

    return nil
}
```

### 4. Router Configuration Update

**xDS Stream Communication**:

1. Router maintains persistent gRPC stream to Controller
2. Controller detects new snapshot version in cache
3. Controller sends DiscoveryResponse with new resources
4. Router validates and applies configuration

**Router Side** (Envoy):
- Receives LDS, RDS, CDS updates
- Validates configuration syntax
- Applies gracefully (drains in-flight connections)
- ACKs successful application to Controller

**Timeline**:
```
T+0s:  User submits configuration
T+0.1s: Validation complete
T+0.2s: Database write complete
T+0.3s: In-memory maps updated
T+0.5s: xDS snapshot generated
T+1s:   Router receives xDS update
T+2s:   Router applies configuration
T+5s:   New routes active and serving traffic
```

### 5. Configuration Updates

**Update Existing Configuration**:

```bash
curl -X PUT http://localhost:9090/apis/PetStore/v1 \
  -H "Content-Type: application/json" \
  -d @updated-config.yaml
```

**Processing**:

1. **Retrieve Existing Configuration**
   ```go
   existingConfig, err := h.memStore.Get(id)
   if err != nil {
       return 404, ErrorResponse{Message: "API not found"}
   }
   ```

2. **Validate Updated Configuration**
   - Same validation as create
   - Ensure name/version unchanged (immutable composite key)

3. **Generate Configuration Diff** (debug mode)
   ```go
   if h.logger.Level() == zap.DebugLevel {
       diff := generateDiff(existingConfig, newConfig)
       h.logger.Debug("Configuration diff",
           zap.String("id", id),
           zap.String("diff", diff),
       )
   }
   ```

4. **Atomic Update**
   ```go
   // Update database
   if err := h.db.Update(id, newConfig); err != nil {
       return 500, ErrorResponse{Message: "Failed to update"}
   }

   // Update in-memory maps
   h.memStore.Set(id, newConfig)

   // Trigger xDS snapshot update
   h.updateXDSSnapshot()
   ```

5. **Audit Log**
   ```go
   h.logger.Info("API configuration updated",
       zap.String("id", id),
       zap.String("operation", "update"),
       zap.String("user", "admin"), // From auth context
       zap.Time("timestamp", time.Now()),
   )
   ```

### 6. Configuration Deletion

**Delete Configuration**:

```bash
curl -X DELETE http://localhost:9090/apis/PetStore/v1
```

**Processing**:

1. **Verify Existence**
   ```go
   if _, err := h.memStore.Get(id); err != nil {
       return 404, ErrorResponse{Message: "API not found"}
   }
   ```

2. **Remove from Storage**
   ```go
   // Delete from database
   if err := h.db.Delete(id); err != nil {
       return 500, ErrorResponse{Message: "Failed to delete"}
   }

   // Remove from in-memory maps
   h.memStore.Delete(id)
   ```

3. **Update xDS Snapshot** (removes routes from Router)
   ```go
   // Generate new snapshot WITHOUT deleted config
   allConfigs := h.memStore.GetAll()
   h.xdsServer.UpdateSnapshot(allConfigs)
   ```

4. **Router Behavior**
   - Receives updated snapshot without deleted API routes
   - Gracefully drains in-flight requests to deleted API
   - Returns 404 for new requests to deleted routes

### 7. Configuration Queries

**List All APIs**:

```bash
curl http://localhost:9090/apis
```

**Response**:
```json
{
  "status": "success",
  "count": 5,
  "apis": [
    {
      "id": "PetStore/v1",
      "name": "PetStore",
      "version": "v1",
      "context": "/petstore/v1",
      "created_at": "2025-10-13T10:00:00Z",
      "updated_at": "2025-10-13T10:30:00Z"
    },
    ...
  ]
}
```

**Get Specific API**:

```bash
curl http://localhost:9090/apis/PetStore/v1
```

**Response**:
```json
{
  "status": "success",
  "api": {
    "version": "api-platform.wso2.com/v1",
    "kind": "http/rest",
    "data": {
      "name": "PetStore",
      "version": "v1",
      "context": "/petstore/v1",
      "operations": [...],
      "upstream": [...]
    }
  }
}
```

## Error Handling

### Validation Errors

```json
{
  "status": "error",
  "message": "Configuration validation failed",
  "errors": [
    {
      "field": "data.context",
      "message": "Context must start with / and cannot end with /"
    },
    {
      "field": "data.operations[0].method",
      "message": "Invalid HTTP method: INVALID"
    }
  ]
}
```

### Conflict Errors

```json
{
  "status": "error",
  "message": "API configuration already exists",
  "error": "Conflict: API 'PetStore/v1' already deployed"
}
```

### Internal Errors

```json
{
  "status": "error",
  "message": "Internal server error",
  "error": "Failed to persist configuration"
}
```

## Audit Logging

Every lifecycle operation generates an audit log entry:

```json
{
  "timestamp": "2025-10-13T10:30:00Z",
  "level": "info",
  "operation": "create|update|delete|query",
  "api_id": "PetStore/v1",
  "status": "success|failed",
  "user": "admin",
  "details": {
    "snapshot_version": 15,
    "config_count": 10
  }
}
```

## Performance Considerations

- **In-Memory First**: All xDS translations read from in-memory maps (not database)
- **Atomic Updates**: Database + memory updated together (no inconsistent state)
- **Snapshot Caching**: go-control-plane cache handles snapshot distribution efficiently
- **Concurrent Operations**: Storage layer uses RWMutex for thread safety

---

**Document Version**: 1.0
**Last Updated**: 2025-10-13
**Status**: Active Development
