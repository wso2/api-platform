# Gateway-Controller xDS Implementation

## Overview

This document details the implementation of the xDS v3 server in Gateway-Controller using go-control-plane library with the State-of-the-World (SotW) protocol.

## Architecture

### Component Flow

```
User API Request → REST Handler → Validator → Storage Layer
                                                    ↓
                                         In-Memory Maps + bbolt
                                                    ↓
                                            xDS Translator
                                                    ↓
                                            Snapshot Cache
                                                    ↓
                                            xDS gRPC Server
                                                    ↓
                                            Router (Envoy)
```

### Key Components

1. **REST API Layer** (`pkg/api/handlers/`)
   - Handles HTTP requests from users
   - Validates API configurations
   - Triggers configuration updates

2. **Storage Layer** (`pkg/storage/`)
   - `memory.go`: In-memory maps for fast access
   - `bbolt.go`: Persistent storage implementation
   - Atomic updates to both memory and disk

3. **xDS Server** (`pkg/xds/`)
   - `server.go`: gRPC xDS server setup
   - `snapshot.go`: Snapshot cache management
   - `translator.go`: API config → Envoy resources

## Data Structures

### In-Memory Maps (pkg/storage/memory.go)

```go
type MemoryStore struct {
    mu sync.RWMutex

    // Primary map: key = "name/version", value = full API config
    apis map[string]*models.APIConfig

    // Index for fast lookups
    apisByName map[string][]string // name → list of versions
}

func (m *MemoryStore) Get(id string) (*models.APIConfig, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    config, exists := m.apis[id]
    if !exists {
        return nil, ErrNotFound
    }
    return config, nil
}

func (m *MemoryStore) Set(id string, config *models.APIConfig) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.apis[id] = config

    // Update index
    versions := m.apisByName[config.Data.Name]
    m.apisByName[config.Data.Name] = append(versions, config.Data.Version)

    return nil
}
```

### xDS Snapshot (pkg/xds/snapshot.go)

```go
type SnapshotManager struct {
    cache cache.SnapshotCache
    version atomic.Int64
    nodeID string
}

func (sm *SnapshotManager) UpdateSnapshot(configs []*models.APIConfig) error {
    // Increment version
    version := sm.version.Add(1)
    versionStr := strconv.FormatInt(version, 10)

    // Translate configs to Envoy resources
    listeners := sm.translateToListeners(configs)
    routes := sm.translateToRoutes(configs)
    clusters := sm.translateToClusters(configs)

    // Create snapshot
    snapshot, err := cache.NewSnapshot(
        versionStr,
        map[resource.Type][]types.Resource{
            resource.ListenerType: listeners,
            resource.RouteType:    routes,
            resource.ClusterType:  clusters,
        },
    )
    if err != nil {
        return fmt.Errorf("failed to create snapshot: %w", err)
    }

    // Update cache (SotW: complete state)
    if err := sm.cache.SetSnapshot(context.Background(), sm.nodeID, snapshot); err != nil {
        return fmt.Errorf("failed to set snapshot: %w", err)
    }

    return nil
}
```

## xDS Translation Logic

### API Config → Envoy Resources (pkg/xds/translator.go)

```go
func (t *Translator) TranslateToEnvoyResources(configs []*models.APIConfig) (*EnvoyResources, error) {
    resources := &EnvoyResources{
        Listeners: []types.Resource{},
        Routes:    []types.Resource{},
        Clusters:  []types.Resource{},
    }

    // Create single HTTP listener for all APIs
    listener := t.createHTTPListener()
    resources.Listeners = append(resources.Listeners, listener)

    // Create route config with virtual hosts for each API
    routeConfig := t.createRouteConfig(configs)
    resources.Routes = append(resources.Routes, routeConfig)

    // Create clusters for each backend service
    for _, config := range configs {
        for _, upstream := range config.Data.Upstream {
            cluster := t.createCluster(config, upstream)
            resources.Clusters = append(resources.Clusters, cluster)
        }
    }

    return resources, nil
}

func (t *Translator) createHTTPListener() *listener.Listener {
    manager := &hcm.HttpConnectionManager{
        StatPrefix: "ingress_http",
        RouteSpecifier: &hcm.HttpConnectionManager_Rds{
            Rds: &hcm.Rds{
                RouteConfigName: "route_config_0",
                ConfigSource: &core.ConfigSource{
                    ResourceApiVersion: resource.DefaultAPIVersion,
                    ConfigSourceSpecifier: &core.ConfigSource_Ads{
                        Ads: &core.AggregatedConfigSource{},
                    },
                },
            },
        },
        HttpFilters: []*hcm.HttpFilter{
            {Name: "envoy.filters.http.router"},
        },
        AccessLog: t.createAccessLogConfig(),
    }

    pbst, err := anypb.New(manager)
    if err != nil {
        panic(err)
    }

    return &listener.Listener{
        Name: "listener_0",
        Address: &core.Address{
            Address: &core.Address_SocketAddress{
                SocketAddress: &core.SocketAddress{
                    Address: "0.0.0.0",
                    PortSpecifier: &core.SocketAddress_PortValue{
                        PortValue: 8081,
                    },
                },
            },
        },
        FilterChains: []*listener.FilterChain{
            {
                Filters: []*listener.Filter{
                    {
                        Name: "envoy.filters.network.http_connection_manager",
                        ConfigType: &listener.Filter_TypedConfig{
                            TypedConfig: pbst,
                        },
                    },
                },
            },
        },
    }
}

func (t *Translator) createRouteConfig(configs []*models.APIConfig) *route.RouteConfiguration {
    var virtualHosts []*route.VirtualHost

    for _, config := range configs {
        vhost := &route.VirtualHost{
            Name:    fmt.Sprintf("%s_%s", config.Data.Name, config.Data.Version),
            Domains: []string{"*"},
            Routes:  t.createRoutes(config),
        }
        virtualHosts = append(virtualHosts, vhost)
    }

    return &route.RouteConfiguration{
        Name:         "route_config_0",
        VirtualHosts: virtualHosts,
    }
}

func (t *Translator) createRoutes(config *models.APIConfig) []*route.Route {
    var routes []*route.Route

    for _, operation := range config.Data.Operations {
        // Build full path: context + operation path
        fullPath := config.Data.Context + operation.Path

        // Create route match
        match := &route.RouteMatch{
            PathSpecifier: &route.RouteMatch_Path{
                Path: fullPath,
            },
        }

        // Add method match if specified
        if operation.Method != "" {
            match.Headers = []*route.HeaderMatcher{
                {
                    Name: ":method",
                    HeaderMatchSpecifier: &route.HeaderMatcher_ExactMatch{
                        ExactMatch: operation.Method,
                    },
                },
            }
        }

        // Create route action
        action := &route.Route_Route{
            Route: &route.RouteAction{
                ClusterSpecifier: &route.RouteAction_Cluster{
                    Cluster: t.getClusterName(config),
                },
            },
        }

        routes = append(routes, &route.Route{
            Match:  match,
            Action: action,
        })
    }

    return routes
}
```

## Startup Sequence

### Gateway-Controller Initialization (cmd/controller/main.go)

```go
func main() {
    // 1. Initialize logger
    logger := logger.NewLogger()

    // 2. Initialize storage
    db, err := storage.NewBBoltStorage("./data/gateway-controller.db")
    if err != nil {
        logger.Fatal("Failed to initialize storage", zap.Error(err))
    }
    defer db.Close()

    // 3. Load configurations from database to memory
    memStore := storage.NewMemoryStore()
    configs, err := db.LoadAll()
    if err != nil {
        logger.Fatal("Failed to load configurations", zap.Error(err))
    }

    for _, config := range configs {
        memStore.Set(config.GetID(), config)
    }
    logger.Info("Loaded configurations", zap.Int("count", len(configs)))

    // 4. Initialize xDS server
    xdsServer := xds.NewServer(memStore, logger)
    go xdsServer.Start(":18000")

    // 5. Generate initial xDS snapshot
    if err := xdsServer.UpdateSnapshot(memStore.GetAll()); err != nil {
        logger.Fatal("Failed to generate initial snapshot", zap.Error(err))
    }

    // 6. Initialize REST API server
    apiServer := api.NewServer(memStore, db, xdsServer, logger)
    router := gin.Default()
    apiServer.RegisterRoutes(router)

    // 7. Start REST API server
    if err := router.Run(":9090"); err != nil {
        logger.Fatal("Failed to start API server", zap.Error(err))
    }
}
```

## Configuration Update Flow

```go
// REST Handler (pkg/api/handlers/handlers.go)
func (h *Handler) CreateAPI(c *gin.Context) {
    var config models.APIConfig
    if err := c.ShouldBindJSON(&config); err != nil {
        c.JSON(400, ErrorResponse{Message: "Invalid JSON"})
        return
    }

    // Validate
    if err := h.validator.Validate(&config); err != nil {
        c.JSON(400, ErrorResponse{Errors: err})
        return
    }

    // Generate ID
    id := fmt.Sprintf("%s/%s", config.Data.Name, config.Data.Version)

    // Check for duplicates
    if _, err := h.memStore.Get(id); err == nil {
        c.JSON(409, ErrorResponse{Message: "API already exists"})
        return
    }

    // Persist to database (atomic)
    if err := h.db.Save(id, &config); err != nil {
        h.logger.Error("Failed to save configuration", zap.Error(err))
        c.JSON(500, ErrorResponse{Message: "Internal error"})
        return
    }

    // Update in-memory maps
    h.memStore.Set(id, &config)

    // Trigger xDS snapshot update
    allConfigs := h.memStore.GetAll()
    if err := h.xdsServer.UpdateSnapshot(allConfigs); err != nil {
        h.logger.Error("Failed to update xDS snapshot", zap.Error(err))
        // Note: Config is already saved, log error but return success
    }

    // Audit log
    h.logger.Info("API configuration created",
        zap.String("id", id),
        zap.String("name", config.Data.Name),
        zap.String("version", config.Data.Version),
    )

    c.JSON(201, CreateResponse{
        Status:    "success",
        Message:   "API configuration created successfully",
        ID:        id,
        CreatedAt: time.Now(),
    })
}
```

## Testing

### Unit Tests (tests/unit/translator_test.go)

```go
func TestTranslateToEnvoyResources(t *testing.T) {
    translator := xds.NewTranslator()

    configs := []*models.APIConfig{
        {
            Data: models.APIData{
                Name:    "PetStore",
                Version: "v1",
                Context: "/petstore/v1",
                Operations: []models.Operation{
                    {Method: "GET", Path: "/pets"},
                    {Method: "POST", Path: "/pets"},
                },
                Upstream: []models.Upstream{
                    {URL: "http://api.example.com"},
                },
            },
        },
    }

    resources, err := translator.TranslateToEnvoyResources(configs)
    assert.NoError(t, err)
    assert.Len(t, resources.Listeners, 1)
    assert.Len(t, resources.Routes, 1)
    assert.Len(t, resources.Clusters, 1)

    // Verify route config has correct virtual host
    routeConfig := resources.Routes[0].(*route.RouteConfiguration)
    assert.Equal(t, "PetStore_v1", routeConfig.VirtualHosts[0].Name)
    assert.Len(t, routeConfig.VirtualHosts[0].Routes, 2)
}
```

### Integration Tests (tests/integration/xds_test.go)

```go
func TestXDSIntegration(t *testing.T) {
    // Setup test environment
    memStore := storage.NewMemoryStore()
    xdsServer := xds.NewServer(memStore, logger)
    go xdsServer.Start(":18000")
    defer xdsServer.Stop()

    // Add test configuration
    config := &models.APIConfig{...}
    memStore.Set("TestAPI/v1", config)

    // Update snapshot
    err := xdsServer.UpdateSnapshot(memStore.GetAll())
    assert.NoError(t, err)

    // Connect test Envoy client
    // Verify Envoy receives configuration
    // ...
}
```

## Debugging

### Enable Debug Logging

```bash
export LOG_LEVEL=debug
```

Debug logs include:
- Full xDS snapshot payloads
- Configuration diff before/after updates
- Envoy client connection events
- Snapshot cache hits/misses

### Inspect xDS State

```bash
# View current snapshot version
curl http://localhost:9090/debug/xds/version

# View snapshot contents (debug endpoint)
curl http://localhost:9090/debug/xds/snapshot

# View in-memory map contents
curl http://localhost:9090/debug/storage/memory
```

---

**Document Version**: 1.0
**Last Updated**: 2025-10-13
**Status**: Active Development
