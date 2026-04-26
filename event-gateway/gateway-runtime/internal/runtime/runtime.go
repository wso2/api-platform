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
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/common/version"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/admin"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/binding"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/config"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/hub"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/pkg/engine"
)

var (
	initialReceiverStartBackoff = 1 * time.Second
	maxReceiverStartBackoff     = 30 * time.Second
)

// Runtime orchestrates all event gateway components.
// It owns the lifecycle of the policy engine, hub, admin server,
// and all per-channel receiver+broker-driver pairs.
type Runtime struct {
	cfg           *config.Config
	rawConfig     map[string]interface{}
	engine        *engine.Engine
	hub           *hub.Hub
	registry      *connectors.Registry
	admin         *admin.Server
	brokerDrivers []connectors.BrokerDriver
	receivers     []connectors.Receiver
	servers       []*managedServer // shared servers for port sharing

	// Dynamic binding management (xDS mode)
	mu                   sync.RWMutex
	activeReceivers      map[string]connectors.Receiver
	activeBrokerDrivers  map[string]connectors.BrokerDriver
	bindingPaths         map[string][]string // name → registered mux paths
	bindingTopics        map[string][]string // name → Kafka topics (data + internal sub)
	websubMux            *DynamicMux
	websubServer         *managedServer
	webSubServersCreated bool // true if LoadChannels created WebSub servers
	runCtx               context.Context
	running              bool // true after Run() starts servers
}

type managedServer struct {
	name     string
	server   *http.Server
	tls      bool
	certFile string
	keyFile  string
}

// New creates a new Runtime. After creation:
//  1. Call Engine() to register policies
//  2. Call LoadChannels() to parse bindings and create per-channel receiver+broker-driver pairs
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
		cfg:                 cfg,
		rawConfig:           rawConfig,
		engine:              eng,
		hub:                 hub.NewHub(eng),
		registry:            registry,
		admin:               admin.NewServer(cfg.Server.AdminPort),
		activeReceivers:     make(map[string]connectors.Receiver),
		activeBrokerDrivers: make(map[string]connectors.BrokerDriver),
		bindingPaths:        make(map[string][]string),
		bindingTopics:       make(map[string][]string),
		websubMux:           NewDynamicMux(),
	}, nil
}

// Engine returns the policy engine for registering policies.
func (r *Runtime) Engine() *engine.Engine {
	return r.engine
}

// LoadChannels parses channel bindings and creates per-channel receiver+broker-driver pairs.
// Supports both legacy flat bindings and WebSubApi multi-channel bindings.
func (r *Runtime) LoadChannels(channelsPath string) error {
	parseResult, err := binding.ParseChannels(channelsPath)
	if err != nil {
		return fmt.Errorf("failed to parse channels: %w", err)
	}

	if len(parseResult.Bindings) == 0 && len(parseResult.WebSubApiBindings) == 0 {
		slog.Info("No channel bindings configured")
		return nil
	}

	// Create shared HTTP muxes for port sharing.
	wsMux := http.NewServeMux()
	websubMux := http.NewServeMux()
	hasWS := false
	hasWebSub := false

	// Process legacy flat bindings (protocol-mediation, legacy websub).
	for _, b := range parseResult.Bindings {
		subKey, inKey, outKey, err := r.buildPolicyChains(b)
		if err != nil {
			return fmt.Errorf("failed to build chains for binding %q: %w", b.Name, err)
		}

		vhost := defaultVhost(b.Vhost)

		qualifiedTopic := qualifyTopicName(b.Context, b.Version, b.BrokerDriver.Topic)

		r.hub.RegisterBinding(hub.ChannelBinding{
			APIID:             b.APIID,
			Name:              b.Name,
			Mode:              b.Mode,
			Context:           b.Context,
			Version:           b.Version,
			Vhost:             vhost,
			SubscribeChainKey: subKey,
			InboundChainKey:   inKey,
			OutboundChainKey:  outKey,
			BrokerDriverTopic: qualifiedTopic,
			Ordering:          b.BrokerDriver.Ordering,
		})

		brokerDriverType := resolveBrokerDriverType(b)
		brokerDriver, err := r.registry.CreateBrokerDriver(brokerDriverType, b.BrokerDriver.Config)
		if err != nil {
			return fmt.Errorf("failed to create broker-driver %q for binding %q: %w", brokerDriverType, b.Name, err)
		}
		r.brokerDrivers = append(r.brokerDrivers, brokerDriver)

		receiverType := resolveReceiverType(b)
		var mux *http.ServeMux
		switch receiverType {
		case "websub":
			mux = websubMux
			hasWebSub = true
		default:
			mux = wsMux
			hasWS = true
		}

		ch := connectors.ChannelInfo{
			Name:              b.Name,
			Mode:              b.Mode,
			Context:           b.Context,
			Version:           b.Version,
			Vhost:             vhost,
			PublicTopic:       b.BrokerDriver.Topic,
			BrokerDriverTopic: qualifiedTopic,
			Ordering:          b.BrokerDriver.Ordering,
		}

		ep, err := r.registry.CreateReceiver(receiverType, connectors.ReceiverConfig{
			Channel:      ch,
			Processor:    r.hub,
			BrokerDriver: brokerDriver,
			RuntimeID:    r.cfg.RuntimeID,
			Mux:          mux,
		})
		if err != nil {
			return fmt.Errorf("failed to create receiver for binding %q: %w", b.Name, err)
		}
		r.receivers = append(r.receivers, ep)

		slog.Info("Registered channel binding",
			"name", b.Name,
			"mode", b.Mode,
			"receiver", receiverType,
			"broker-driver", brokerDriverType,
			"broker_driver_topic", b.BrokerDriver.Topic,
		)
	}

	// Process WebSubApi bindings (multi-channel per API).
	for _, wsb := range parseResult.WebSubApiBindings {
		vhost := defaultVhost(wsb.Vhost)

		// Build channel-name → Kafka-topic map.
		channels := make(map[string]string, len(wsb.Channels))
		var allKafkaTopics []string
		for _, ch := range wsb.Channels {
			kafkaTopic := binding.WebSubApiTopicName(wsb.Name, wsb.Version, ch.Name)
			channels[ch.Name] = kafkaTopic
			allKafkaTopics = append(allKafkaTopics, kafkaTopic)
		}
		internalSubTopic := binding.WebSubApiSubscriptionTopic(wsb.Name, wsb.Version)

		// Build policy chains for the API.
		subKey, inKey, outKey, err := r.buildWebSubApiPolicyChains(wsb, vhost)
		if err != nil {
			return fmt.Errorf("failed to build chains for WebSubApi %q: %w", wsb.Name, err)
		}

		r.hub.RegisterBinding(hub.ChannelBinding{
			APIID:             wsb.APIID,
			Name:              wsb.Name,
			Mode:              "websub",
			Context:           wsb.Context,
			Version:           wsb.Version,
			Vhost:             vhost,
			SubscribeChainKey: subKey,
			InboundChainKey:   inKey,
			OutboundChainKey:  outKey,
			Channels:          channels,
		})

		// Create broker-driver.
		brokerDriverType := "kafka"
		if wsb.BrokerDriver.Type != "" {
			brokerDriverType = wsb.BrokerDriver.Type
		}
		brokerDriver, err := r.registry.CreateBrokerDriver(brokerDriverType, wsb.BrokerDriver.Config)
		if err != nil {
			return fmt.Errorf("failed to create broker-driver for WebSubApi %q: %w", wsb.Name, err)
		}
		r.brokerDrivers = append(r.brokerDrivers, brokerDriver)

		hasWebSub = true

		ch := connectors.ChannelInfo{
			Name:             wsb.Name,
			Mode:             "websub",
			Context:          wsb.Context,
			Version:          wsb.Version,
			Vhost:            vhost,
			Channels:         channels,
			InternalSubTopic: internalSubTopic,
		}

		ep, err := r.registry.CreateReceiver("websub", connectors.ReceiverConfig{
			Channel:      ch,
			Processor:    r.hub,
			BrokerDriver: brokerDriver,
			RuntimeID:    r.cfg.RuntimeID,
			Mux:          websubMux,
		})
		if err != nil {
			return fmt.Errorf("failed to create receiver for WebSubApi %q: %w", wsb.Name, err)
		}
		r.receivers = append(r.receivers, ep)

		slog.Info("Registered WebSubApi binding",
			"name", wsb.Name,
			"context", wsb.Context,
			"version", wsb.Version,
			"channels", len(wsb.Channels),
			"kafka_topics", allKafkaTopics,
		)
	}

	// Create shared HTTP servers.
	if hasWS {
		wsServer, err := r.newManagedServer("WebSocket", r.cfg.Server.WebSocketPort, wsMux, false)
		if err != nil {
			return fmt.Errorf("failed to create WebSocket server: %w", err)
		}
		r.servers = append(r.servers, wsServer)
	}
	if hasWebSub && r.cfg.Server.WebSubEnabled {
		// Create HTTP server
		websubHTTPServer, err := r.newManagedServer("WebSub-HTTP", r.cfg.Server.WebSubHTTPPort, websubMux, false)
		if err != nil {
			return fmt.Errorf("failed to create WebSub HTTP server: %w", err)
		}
		r.servers = append(r.servers, websubHTTPServer)
		// Create HTTPS server if TLS is enabled
		if r.cfg.Server.WebSubTLSEnabled {
			websubHTTPSServer, err := r.newManagedServer("WebSub-HTTPS", r.cfg.Server.WebSubHTTPSPort, websubMux, true)
			if err != nil {
				return fmt.Errorf("failed to create WebSub HTTPS server: %w", err)
			}
			r.servers = append(r.servers, websubHTTPSServer)
		}
		r.webSubServersCreated = true // Mark that LoadChannels created WebSub servers
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
			r.runServer(srv)
		}()
	}

	// If in xDS mode, ensure the websub server is started for dynamic bindings.
	r.mu.Lock()
	if !r.webSubServersCreated && r.websubServer == nil && r.cfg.ControlPlane.Enabled && r.cfg.Server.WebSubEnabled {
		// Create and start HTTP server
		websubHTTPServer, err := r.newManagedServer("WebSub-HTTP", r.cfg.Server.WebSubHTTPPort, r.websubMux, false)
		if err != nil {
			r.mu.Unlock()
			return fmt.Errorf("failed to create WebSub HTTP server: %w", err)
		}
		r.servers = append(r.servers, websubHTTPServer)
		go func() {
			r.runServer(websubHTTPServer)
		}()
		// Create and start HTTPS server if TLS is enabled
		if r.cfg.Server.WebSubTLSEnabled {
			websubHTTPSServer, err := r.newManagedServer("WebSub-HTTPS", r.cfg.Server.WebSubHTTPSPort, r.websubMux, true)
			if err != nil {
				r.mu.Unlock()
				return fmt.Errorf("failed to create WebSub HTTPS server: %w", err)
			}
			r.websubServer = websubHTTPSServer
			go func() {
				r.runServer(websubHTTPSServer)
			}()
		}
		// Note: r.websubServer is only set when TLS is enabled; HTTP-only mode leaves it nil
		// to avoid double-shutdown since the HTTP server is already in r.servers
	}
	r.runCtx = ctx
	r.running = true
	r.mu.Unlock()

	// Start receivers that were added before Run() (static mode).
	for i, ep := range r.receivers {
		if err := r.startReceiverWithRetry(ctx, fmt.Sprintf("startup-%d", i), ep); err != nil {
			return fmt.Errorf("failed to start receiver: %w", err)
		}
	}

	// Start any dynamically added receivers that were queued before Run().
	r.mu.RLock()
	pendingReceivers := make(map[string]connectors.Receiver, len(r.activeReceivers))
	for name, ep := range r.activeReceivers {
		pendingReceivers[name] = ep
	}
	r.mu.RUnlock()
	for name, ep := range pendingReceivers {
		if err := r.startReceiverWithRetry(ctx, name, ep); err != nil {
			return fmt.Errorf("failed to start dynamic receiver: %w", err)
		}
	}

	r.admin.SetReady(true)
	slog.Info("Event gateway is ready", "runtime_id", r.cfg.RuntimeID)

	<-ctx.Done()

	slog.Info("Shutting down event gateway...")
	r.admin.SetReady(false)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop static receivers.
	for i := len(r.receivers) - 1; i >= 0; i-- {
		if err := r.receivers[i].Stop(shutdownCtx); err != nil {
			slog.Error("Failed to stop receiver", "error", err)
		}
	}

	// Stop dynamic receivers.
	r.mu.Lock()
	r.runCtx = nil
	for name, ep := range r.activeReceivers {
		if err := ep.Stop(shutdownCtx); err != nil {
			slog.Error("Failed to stop dynamic receiver", "name", name, "error", err)
		}
	}
	for name, bd := range r.activeBrokerDrivers {
		if err := bd.Close(); err != nil {
			slog.Error("Failed to close dynamic broker-driver", "name", name, "error", err)
		}
	}
	r.mu.Unlock()

	for _, bd := range r.brokerDrivers {
		if err := bd.Close(); err != nil {
			slog.Error("Failed to close broker-driver", "error", err)
		}
	}

	// Shutdown shared HTTP servers.
	for _, srv := range r.servers {
		if err := srv.server.Shutdown(shutdownCtx); err != nil {
			slog.Error("Failed to shutdown server", "name", srv.name, "addr", srv.server.Addr, "error", err)
		}
	}

	// Shutdown xDS-mode websub server if created.
	r.mu.RLock()
	wsSrv := r.websubServer
	r.mu.RUnlock()
	if wsSrv != nil {
		if err := wsSrv.server.Shutdown(shutdownCtx); err != nil {
			slog.Error("Failed to shutdown WebSub server", "addr", wsSrv.server.Addr, "error", err)
		}
	}

	if err := r.admin.Stop(shutdownCtx); err != nil {
		slog.Error("Failed to stop admin server", "error", err)
	}

	slog.Info("Event gateway shutdown complete")
	return nil
}

func (r *Runtime) newManagedServer(name string, port int, handler http.Handler, allowTLS bool) (*managedServer, error) {
	server := &managedServer{
		name: name,
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: handler,
		},
	}

	if allowTLS && r.cfg.Server.WebSubTLSEnabled {
		if err := ensureReadableTLSAsset(r.cfg.Server.WebSubTLSCertFile, "server.websub_tls_cert_file"); err != nil {
			return nil, fmt.Errorf("invalid TLS configuration for %s server: %w", name, err)
		}
		if err := ensureReadableTLSAsset(r.cfg.Server.WebSubTLSKeyFile, "server.websub_tls_key_file"); err != nil {
			return nil, fmt.Errorf("invalid TLS configuration for %s server: %w", name, err)
		}
		server.tls = true
		server.certFile = r.cfg.Server.WebSubTLSCertFile
		server.keyFile = r.cfg.Server.WebSubTLSKeyFile
	}

	return server, nil
}

func ensureReadableTLSAsset(filePath, fieldName string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s file %q does not exist", fieldName, filePath)
		}
		return fmt.Errorf("failed to access %s file %q: %w", fieldName, filePath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s path %q must be a file, not a directory", fieldName, filePath)
	}

	fileHandle, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("%s file %q is not readable: %w", fieldName, filePath, err)
	}
	return fileHandle.Close()
}

func (r *Runtime) runServer(srv *managedServer) {
	protocol := "HTTP"
	errMsg := "server error"
	if srv.tls {
		protocol = "HTTPS"
	}

	slog.Info("Starting server", "name", srv.name, "protocol", protocol, "addr", srv.server.Addr)

	var err error
	if srv.tls {
		errMsg = "HTTPS server error"
		err = srv.server.ListenAndServeTLS(srv.certFile, srv.keyFile)
	} else {
		errMsg = "HTTP server error"
		err = srv.server.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		slog.Error(errMsg, "name", srv.name, "addr", srv.server.Addr, "error", err)
	}
}

func (r *Runtime) startReceiverWithRetry(ctx context.Context, name string, receiver connectors.Receiver) error {
	backoff := initialReceiverStartBackoff
	for {
		err := receiver.Start(ctx)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return fmt.Errorf("receiver start canceled for %q: %w", name, ctx.Err())
		}

		slog.Warn("Receiver start failed, will retry",
			"name", name,
			"backoff", backoff,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return fmt.Errorf("receiver start canceled for %q: %w", name, ctx.Err())
		case <-time.After(backoff):
		}

		backoff = min(backoff*2, maxReceiverStartBackoff)
	}
}

func (r *Runtime) buildPolicyChains(b binding.Binding) (subscribeKey, inboundKey, outboundKey string, err error) {
	vhost := defaultVhost(b.Vhost)

	switch b.Mode {
	case "websub":
		channelPath := path.Join(b.Context, b.Name)
		hubPath := path.Join(b.Context, "_hub")
		subscribeKey = binding.GenerateRouteKey("SUBSCRIBE", hubPath, vhost)
		inboundKey = binding.GenerateRouteKey("SUB", channelPath, vhost)
		outboundKey = binding.GenerateRouteKey("DELIVER", channelPath, vhost)

	case "protocol-mediation":
		channelPath := path.Join(b.Context, b.Receiver.Path)
		inboundKey = binding.GenerateRouteKey("PUBLISH", channelPath, vhost)
		outboundKey = binding.GenerateRouteKey("DELIVER", channelPath, vhost)

	default:
		return "", "", "", fmt.Errorf("unknown binding mode: %s", b.Mode)
	}

	if err := r.buildChain(subscribeKey, b.Policies.Subscribe); err != nil {
		return "", "", "", err
	}
	if err := r.buildChain(inboundKey, b.Policies.Inbound); err != nil {
		return "", "", "", err
	}
	if err := r.buildChain(outboundKey, b.Policies.Outbound); err != nil {
		return "", "", "", err
	}

	return subscribeKey, inboundKey, outboundKey, nil
}

func (r *Runtime) buildWebSubApiPolicyChains(wsb binding.WebSubApiBinding, vhost string) (subscribeKey, inboundKey, outboundKey string, err error) {
	basePath := binding.WebSubApiBasePath(wsb.Context, wsb.Version)
	hubPath := basePath + "/hub"

	// Subscribe chain: hub path (subscribe/unsubscribe requests).
	subscribeKey = binding.GenerateRouteKey("SUBSCRIBE", hubPath, vhost)
	// Inbound chain: webhook-receiver path (data ingress).
	inboundKey = binding.GenerateRouteKey("SUB", basePath+"/webhook-receiver", vhost)
	// Outbound chain: delivery path (data delivery to subscribers).
	outboundKey = binding.GenerateRouteKey("DELIVER", hubPath, vhost)

	if err := r.buildChain(subscribeKey, wsb.Policies.Subscribe); err != nil {
		return "", "", "", err
	}
	if err := r.buildChain(inboundKey, wsb.Policies.Inbound); err != nil {
		return "", "", "", err
	}
	if err := r.buildChain(outboundKey, wsb.Policies.Outbound); err != nil {
		return "", "", "", err
	}

	return subscribeKey, inboundKey, outboundKey, nil
}

func (r *Runtime) buildChain(routeKey string, policies []binding.PolicyRef) error {
	if len(policies) == 0 {
		return nil
	}

	specs := make([]engine.PolicySpec, len(policies))
	for i, p := range policies {
		specs[i] = engine.PolicySpec{
			Name:       p.Name,
			Version:    version.MajorVersion(p.Version),
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

func (r *Runtime) unregisterBindingChains(b *hub.ChannelBinding) {
	if b == nil {
		return
	}
	if b.SubscribeChainKey != "" {
		r.engine.UnregisterChain(b.SubscribeChainKey)
	}
	if b.InboundChainKey != "" {
		r.engine.UnregisterChain(b.InboundChainKey)
	}
	if b.OutboundChainKey != "" {
		r.engine.UnregisterChain(b.OutboundChainKey)
	}
}

// AddWebSubApiBinding dynamically adds a WebSubApi binding at runtime (xDS mode).
func (r *Runtime) AddWebSubApiBinding(wsb binding.WebSubApiBinding) error {
	r.mu.Lock()

	vhost := defaultVhost(wsb.Vhost)

	// Build channel-name → Kafka-topic map.
	channels := make(map[string]string, len(wsb.Channels))
	for _, ch := range wsb.Channels {
		kafkaTopic := binding.WebSubApiTopicName(wsb.Name, wsb.Version, ch.Name)
		channels[ch.Name] = kafkaTopic
	}
	internalSubTopic := binding.WebSubApiSubscriptionTopic(wsb.Name, wsb.Version)

	// Build policy chains for the API.
	subKey, inKey, outKey, err := r.buildWebSubApiPolicyChains(wsb, vhost)
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("failed to build chains for WebSubApi %q: %w", wsb.Name, err)
	}

	r.hub.RegisterBinding(hub.ChannelBinding{
		APIID:             wsb.APIID,
		Name:              wsb.Name,
		Mode:              "websub",
		Context:           wsb.Context,
		Version:           wsb.Version,
		Vhost:             vhost,
		SubscribeChainKey: subKey,
		InboundChainKey:   inKey,
		OutboundChainKey:  outKey,
		Channels:          channels,
	})

	// Create broker-driver.
	brokerDriverType := "kafka"
	if wsb.BrokerDriver.Type != "" {
		brokerDriverType = wsb.BrokerDriver.Type
	}
	brokerDriver, err := r.registry.CreateBrokerDriver(brokerDriverType, wsb.BrokerDriver.Config)
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("failed to create broker-driver for WebSubApi %q: %w", wsb.Name, err)
	}
	r.activeBrokerDrivers[wsb.Name] = brokerDriver

	// Track the mux paths so RemoveWebSubApiBinding can deregister them.
	basePath := binding.WebSubApiBasePath(wsb.Context, wsb.Version)
	r.bindingPaths[wsb.Name] = []string{basePath + "/hub", basePath + "/webhook-receiver"}

	// Track all Kafka topics for cleanup on removal.
	allTopics := make([]string, 0, len(channels)+1)
	for _, kafkaTopic := range channels {
		allTopics = append(allTopics, kafkaTopic)
	}
	allTopics = append(allTopics, internalSubTopic)
	r.bindingTopics[wsb.Name] = allTopics

	ch := connectors.ChannelInfo{
		Name:             wsb.Name,
		Mode:             "websub",
		Context:          wsb.Context,
		Version:          wsb.Version,
		Vhost:            vhost,
		Channels:         channels,
		InternalSubTopic: internalSubTopic,
	}

	receiver, err := r.registry.CreateReceiver("websub", connectors.ReceiverConfig{
		Channel:      ch,
		Processor:    r.hub,
		BrokerDriver: brokerDriver,
		RuntimeID:    r.cfg.RuntimeID,
		Mux:          r.websubMux,
	})
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("failed to create receiver for WebSubApi %q: %w", wsb.Name, err)
	}
	r.activeReceivers[wsb.Name] = receiver

	startNow := r.running
	startCtx := r.runCtx
	r.mu.Unlock()

	// If runtime is already running, start the receiver immediately.
	if startNow {
		if startCtx == nil {
			startCtx = context.Background()
		}
		if err := r.startReceiverWithRetry(startCtx, wsb.Name, receiver); err != nil {
			return fmt.Errorf("failed to start receiver for WebSubApi %q: %w", wsb.Name, err)
		}
	}

	slog.Info("Dynamically added WebSubApi binding",
		"name", wsb.Name,
		"context", wsb.Context,
		"version", wsb.Version,
		"channels", len(wsb.Channels),
	)

	return nil
}

// RemoveWebSubApiBinding dynamically removes a WebSubApi binding at runtime (xDS mode).
func (r *Runtime) RemoveWebSubApiBinding(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Stop and remove receiver.
	if receiver, ok := r.activeReceivers[name]; ok {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := receiver.Stop(ctx); err != nil {
			slog.Error("Failed to stop receiver during removal", "name", name, "error", err)
		}
		delete(r.activeReceivers, name)
	}

	// Delete Kafka topics (data + internal subscription) before closing the broker driver.
	if bd, ok := r.activeBrokerDrivers[name]; ok {
		if topics, ok := r.bindingTopics[name]; ok && len(topics) > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := bd.DeleteTopics(ctx, topics); err != nil {
				slog.Error("Failed to delete Kafka topics during removal", "name", name, "error", err)
			}
		}
		delete(r.bindingTopics, name)
	}

	// Close and remove broker driver.
	if bd, ok := r.activeBrokerDrivers[name]; ok {
		if err := bd.Close(); err != nil {
			slog.Error("Failed to close broker-driver during removal", "name", name, "error", err)
		}
		delete(r.activeBrokerDrivers, name)
	}

	// Dynamic xDS updates replace the complete policy state for a binding.
	// Remove the old route keys before the binding disappears so empty-policy
	// redeploys do not keep executing stale chains.
	r.unregisterBindingChains(r.hub.GetBinding(name))

	// Remove hub binding.
	r.hub.RemoveBinding(name)

	// Deregister HTTP routes from the mux.
	if paths, ok := r.bindingPaths[name]; ok {
		for _, p := range paths {
			r.websubMux.Remove(p)
		}
		delete(r.bindingPaths, name)
	}

	slog.Info("Dynamically removed WebSubApi binding", "name", name)
	return nil
}

// Hub returns the hub instance (for testing/inspection).
func (r *Runtime) Hub() *hub.Hub {
	return r.hub
}

// resolveReceiverType determines the receiver factory name from the binding.
// If not explicitly set, it defaults based on mode.
func resolveReceiverType(b binding.Binding) string {
	if b.Receiver.Type != "" {
		return b.Receiver.Type
	}
	switch b.Mode {
	case "websub":
		return "websub"
	case "protocol-mediation":
		return "websocket"
	}
	return b.Mode
}

// resolveBrokerDriverType determines the broker-driver factory name from the binding.
// Defaults to "kafka" if not explicitly set.
func resolveBrokerDriverType(b binding.Binding) string {
	if b.BrokerDriver.Type != "" {
		return b.BrokerDriver.Type
	}
	return "kafka"
}

func defaultVhost(vhost string) string {
	if vhost == "" {
		return "*"
	}
	return vhost
}

// qualifyTopicName generates a unique topic name in the format context.version.topic.
// For example: context="/orders", version="v1", topic="order-events" → "orders.v1.order-events".
func qualifyTopicName(ctx, version, topic string) string {
	ctx = strings.TrimPrefix(ctx, "/")
	return fmt.Sprintf("%s.%s.%s", ctx, version, topic)
}
