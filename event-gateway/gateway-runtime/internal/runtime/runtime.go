/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"time"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/admin"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/binding"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/config"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/hub"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/pkg/engine"
)

// Runtime orchestrates all event gateway components.
// It owns the lifecycle of the policy engine, hub, admin server,
// and all per-channel entrypoint+endpoint pairs.
type Runtime struct {
	cfg         *config.Config
	rawConfig   map[string]interface{}
	engine      *engine.Engine
	hub         *hub.Hub
	registry    *connectors.Registry
	admin       *admin.Server
	endpoints   []connectors.Endpoint
	entrypoints []connectors.Entrypoint
	servers     []*http.Server // shared HTTP servers for port sharing
}

// New creates a new Runtime. After creation:
//  1. Call Engine() to register policies
//  2. Call LoadChannels() to parse bindings and create per-channel entrypoint+endpoint pairs
//  3. Call Run() to start all components
func New(cfg *config.Config, rawConfig map[string]interface{}, registry *connectors.Registry) (*Runtime, error) {
	eng, err := engine.New(rawConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy engine: %w", err)
	}

	if cfg.PolicyEngine.ChainsFile != "" {
		if err := eng.LoadChainsFromFile(cfg.PolicyEngine.ChainsFile); err != nil {
			return nil, fmt.Errorf("failed to load policy chains file: %w", err)
		}
	}

	return &Runtime{
		cfg:       cfg,
		rawConfig: rawConfig,
		engine:    eng,
		hub:       hub.NewHub(eng),
		registry:  registry,
		admin:     admin.NewServer(cfg.Server.AdminPort),
	}, nil
}

// Engine returns the policy engine for registering policies.
func (r *Runtime) Engine() *engine.Engine {
	return r.engine
}

// LoadChannels parses channel bindings and creates per-channel entrypoint+endpoint pairs.
// Each channel (API) gets its own isolated entrypoint and endpoint connection.
// Entrypoints of the same type share an HTTP port via a shared mux.
func (r *Runtime) LoadChannels(channelsPath string) error {
	bindings, err := binding.ParseChannels(channelsPath)
	if err != nil {
		return fmt.Errorf("failed to parse channels: %w", err)
	}

	if len(bindings) == 0 {
		slog.Info("No channel bindings configured")
		return nil
	}

	// Create shared HTTP muxes for port sharing.
	// WebSocket entrypoints share one port; WebSub entrypoints share another.
	wsMux := http.NewServeMux()
	websubMux := http.NewServeMux()
	hasWS := false
	hasWebSub := false

	for _, b := range bindings {
		inKey, outKey, err := r.buildPolicyChains(b)
		if err != nil {
			return fmt.Errorf("failed to build chains for binding %q: %w", b.Name, err)
		}

		vhost := defaultVhost(b.Vhost)

		r.hub.RegisterBinding(hub.ChannelBinding{
			Name:             b.Name,
			Mode:             b.Mode,
			Context:          b.Context,
			Version:          b.Version,
			Vhost:            vhost,
			InboundChainKey:  inKey,
			OutboundChainKey: outKey,
			EndpointTopic:    b.Endpoint.Topic,
			Ordering:         b.Endpoint.Ordering,
		})

		// Create a dedicated endpoint for this channel.
		endpointType := resolveEndpointType(b)
		endpoint, err := r.registry.CreateEndpoint(endpointType)
		if err != nil {
			return fmt.Errorf("failed to create endpoint %q for binding %q: %w", endpointType, b.Name, err)
		}
		r.endpoints = append(r.endpoints, endpoint)

		// Select the shared mux based on entrypoint type.
		entrypointType := resolveEntrypointType(b)
		var mux *http.ServeMux
		switch entrypointType {
		case "websub":
			mux = websubMux
			hasWebSub = true
		default:
			mux = wsMux
			hasWS = true
		}

		ch := connectors.ChannelInfo{
			Name:          b.Name,
			Mode:          b.Mode,
			Context:       b.Context,
			Version:       b.Version,
			Vhost:         vhost,
			EndpointTopic: b.Endpoint.Topic,
			Ordering:      b.Endpoint.Ordering,
		}

		ep, err := r.registry.CreateEntrypoint(entrypointType, connectors.EntrypointConfig{
			Channel:   ch,
			Processor: r.hub,
			Endpoint:  endpoint,
			RuntimeID: r.cfg.RuntimeID,
			Mux:       mux,
		})
		if err != nil {
			return fmt.Errorf("failed to create entrypoint for binding %q: %w", b.Name, err)
		}
		r.entrypoints = append(r.entrypoints, ep)

		slog.Info("Registered channel binding",
			"name", b.Name,
			"mode", b.Mode,
			"entrypoint", entrypointType,
			"endpoint", endpointType,
			"endpoint_topic", b.Endpoint.Topic,
		)
	}

	// Create shared HTTP servers.
	if hasWS {
		r.servers = append(r.servers, &http.Server{
			Addr:    fmt.Sprintf(":%d", r.cfg.Server.WebSocketPort),
			Handler: wsMux,
		})
	}
	if hasWebSub {
		r.servers = append(r.servers, &http.Server{
			Addr:    fmt.Sprintf(":%d", r.cfg.Server.WebSubPort),
			Handler: websubMux,
		})
	}

	return nil
}

// Run starts all components and blocks until ctx is cancelled.
func (r *Runtime) Run(ctx context.Context) error {
	r.admin.Start()

	// Start shared HTTP servers.
	for _, srv := range r.servers {
		srv := srv
		go func() {
			slog.Info("Starting shared HTTP server", "addr", srv.Addr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("Shared HTTP server error", "addr", srv.Addr, "error", err)
			}
		}()
	}

	for _, ep := range r.entrypoints {
		if err := ep.Start(ctx); err != nil {
			return fmt.Errorf("failed to start entrypoint: %w", err)
		}
	}

	r.admin.SetReady(true)
	slog.Info("Event gateway is ready", "runtime_id", r.cfg.RuntimeID)

	<-ctx.Done()

	slog.Info("Shutting down event gateway...")
	r.admin.SetReady(false)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i := len(r.entrypoints) - 1; i >= 0; i-- {
		if err := r.entrypoints[i].Stop(shutdownCtx); err != nil {
			slog.Error("Failed to stop entrypoint", "error", err)
		}
	}

	for _, endpoint := range r.endpoints {
		if err := endpoint.Close(); err != nil {
			slog.Error("Failed to close endpoint", "error", err)
		}
	}

	// Shutdown shared HTTP servers.
	for _, srv := range r.servers {
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("Failed to shutdown HTTP server", "addr", srv.Addr, "error", err)
		}
	}

	if err := r.admin.Stop(shutdownCtx); err != nil {
		slog.Error("Failed to stop admin server", "error", err)
	}

	slog.Info("Event gateway shutdown complete")
	return nil
}

func (r *Runtime) buildPolicyChains(b binding.Binding) (inboundKey, outboundKey string, err error) {
	vhost := defaultVhost(b.Vhost)

	switch b.Mode {
	case "websub":
		channelPath := path.Join(b.Context, b.Name)
		inboundKey = binding.GenerateRouteKey("SUB", channelPath, vhost)
		outboundKey = binding.GenerateRouteKey("DELIVER", channelPath, vhost)

		hubPath := path.Join(b.Context, "_hub")
		if err := r.buildChain(binding.GenerateRouteKey("REGISTER", hubPath, vhost), b.Policies.Inbound); err != nil {
			return "", "", err
		}
		if err := r.buildChain(binding.GenerateRouteKey("HUB_SUBSCRIBE", hubPath, vhost), b.Policies.Inbound); err != nil {
			return "", "", err
		}

	case "protocol-mediation":
		channelPath := path.Join(b.Context, b.Entrypoint.Path)
		inboundKey = binding.GenerateRouteKey("PUBLISH", channelPath, vhost)
		outboundKey = binding.GenerateRouteKey("DELIVER", channelPath, vhost)

	default:
		return "", "", fmt.Errorf("unknown binding mode: %s", b.Mode)
	}

	if err := r.buildChain(inboundKey, b.Policies.Inbound); err != nil {
		return "", "", err
	}
	if err := r.buildChain(outboundKey, b.Policies.Outbound); err != nil {
		return "", "", err
	}

	return inboundKey, outboundKey, nil
}

func (r *Runtime) buildChain(routeKey string, policies []binding.PolicyRef) error {
	if len(policies) == 0 {
		return nil
	}

	specs := make([]engine.PolicySpec, len(policies))
	for i, p := range policies {
		specs[i] = engine.PolicySpec{
			Name:       p.Name,
			Version:    p.Version,
			Enabled:    true,
			Parameters: p.Params,
		}
	}

	chain, err := r.engine.BuildChain(routeKey, specs)
	if err != nil {
		return fmt.Errorf("failed to build chain %s: %w", routeKey, err)
	}
	r.engine.RegisterChain(routeKey, chain)
	return nil
}

// resolveEntrypointType determines the entrypoint factory name from the binding.
// If not explicitly set, it defaults based on mode.
func resolveEntrypointType(b binding.Binding) string {
	if b.Entrypoint.Type != "" {
		return b.Entrypoint.Type
	}
	switch b.Mode {
	case "websub":
		return "websub"
	case "protocol-mediation":
		return "websocket"
	}
	return b.Mode
}

// resolveEndpointType determines the endpoint factory name from the binding.
// Defaults to "kafka" if not explicitly set.
func resolveEndpointType(b binding.Binding) string {
	if b.Endpoint.Type != "" {
		return b.Endpoint.Type
	}
	return "kafka"
}

func defaultVhost(vhost string) string {
	if vhost == "" {
		return "*"
	}
	return vhost
}
