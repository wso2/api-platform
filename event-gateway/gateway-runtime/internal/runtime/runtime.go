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
	"sync"
	"time"

	"github.com/wso2/api-platform/common/version"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/admin"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/binding"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/config"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/hub"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/systempolicies"
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
	mu                  sync.RWMutex
	activeReceivers     map[string]connectors.Receiver
	activeBrokerDrivers map[string]connectors.BrokerDriver
	bindingPaths        map[string][]string // name → registered mux paths
	bindingTopics       map[string][]string // name → Kafka topics (data + internal sub)
	websubMux           *DynamicMux
	wsMux               *DynamicMux // WebSocket mux for dynamic WebBrokerApi bindings
	runCtx              context.Context
	running             bool // true after Run() starts servers
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
		wsMux:               NewDynamicMux(),
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
	wsMux := NewDynamicMux()
	websubMux := http.NewServeMux()

	// Store wsMux for dynamic bindings
	r.wsMux = wsMux

	hasWS := false
	hasWebSub := false

	// Process legacy flat bindings (protocol-mediation, legacy websub).
	for _, b := range parseResult.Bindings {
		subKey, inKey, outKey, err := r.buildPolicyChains(b)
		if err != nil {
			return fmt.Errorf("failed to build chains for binding %q: %w", b.Name, err)
		}

		vhost := defaultVhost(b.Vhost)

		qualifiedTopic := binding.JoinNormalizedTopic(b.Context, b.Version, b.BrokerDriver.Topic)

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
		var mux connectors.RouteMux
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
		internalSubTopic := r.webSubSubscriptionSyncTopic(wsb.Name, wsb.Version)

		// Build policy chains for the API.
		subKey, unsubKey, inKey, outKey, chChainKeys, err := r.buildWebSubApiPolicyChains(wsb, vhost)
		if err != nil {
			return fmt.Errorf("failed to build chains for WebSubApi %q: %w", wsb.Name, err)
		}

		r.hub.RegisterBinding(hub.ChannelBinding{
			APIID:               wsb.APIID,
			Name:                wsb.Name,
			Mode:                "websub",
			Context:             wsb.Context,
			Version:             wsb.Version,
			Vhost:               vhost,
			SubscribeChainKey:   subKey,
			UnsubscribeChainKey: unsubKey,
			InboundChainKey:     inKey,
			OutboundChainKey:    outKey,
			Channels:            channels,
			ChannelChainKeys:    chChainKeys,
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

	// Process WebBrokerApi bindings (protocol mediation).
	for _, wbb := range parseResult.WebBrokerApiBindings {
		vhost := defaultVhost(wbb.Vhost)

		// Build API-level policy chains.
		apiConnInitKey, _, _, err := r.buildWebBrokerApiPolicyChains(wbb, vhost, "")
		if err != nil {
			return fmt.Errorf("failed to build API-level chains for WebBrokerApi %q: %w", wbb.Name, err)
		}

		// Build per-channel policy chains and collect topics.
		channelChains := make(map[string]ChannelPolicyChains)
		allTopics := []string{}                   // All topics (produce + consume) for ensuring they exist
		topicToChannel := make(map[string]string) // Only consume topics for subscription mapping

		for channelName, channelDef := range wbb.Channels {
			connInitKey, produceKey, consumeKey, err := r.buildWebBrokerApiPolicyChains(wbb, vhost, channelName)
			if err != nil {
				return fmt.Errorf("failed to build chains for channel %q in WebBrokerApi %q: %w", channelName, wbb.Name, err)
			}

			channelChains[channelName] = ChannelPolicyChains{
				ConnInitKey: connInitKey,
				ProduceKey:  produceKey,
				ConsumeKey:  consumeKey,
			}

			// Extract ALL topics (produce + consume) to ensure they exist in Kafka
			allChannelTopics := extractAllTopicsFromChannelPolicies(channelName, channelDef)
			allTopics = append(allTopics, allChannelTopics...)

			// Extract ONLY consume topics for subscription mapping
			consumeTopics := extractTopicsFromChannelPolicies(channelName, channelDef)
			for _, topic := range consumeTopics {
				topicToChannel[topic] = channelName
			}
		}

		// Register binding in hub.
		r.hub.RegisterBinding(hub.ChannelBinding{
			APIID:             wbb.APIID,
			Name:              wbb.Name,
			Mode:              "protocol-mediation",
			Context:           wbb.Context,
			Version:           wbb.Version,
			Vhost:             vhost,
			SubscribeChainKey: apiConnInitKey,
			InboundChainKey:   "", // Determined per-channel
			OutboundChainKey:  "", // Determined per-channel
		})

		// Create broker-driver.
		brokerDriverType := wbb.BrokerDriver.Type
		if brokerDriverType == "" {
			brokerDriverType = "kafka"
		}
		brokerDriverConfig := wbb.BrokerDriver.Config
		if brokerDriverConfig == nil {
			brokerDriverConfig = wbb.BrokerDriver.Properties
		}
		brokerDriver, err := r.registry.CreateBrokerDriver(brokerDriverType, brokerDriverConfig)
		if err != nil {
			return fmt.Errorf("failed to create broker-driver for WebBrokerApi %q: %w", wbb.Name, err)
		}
		r.brokerDrivers = append(r.brokerDrivers, brokerDriver)

		hasWS = true

		ch := connectors.ChannelInfo{
			Name:    wbb.Name,
			Mode:    "protocol-mediation",
			Context: wbb.Context,
			Version: wbb.Version,
			Vhost:   vhost,
			Topics:  allTopics,
			Metadata: map[string]interface{}{
				"channelChains":  channelChainsToMap(channelChains),
				"topicToChannel": topicToChannel,
				"channelNames":   getChannelNames(wbb.Channels),
			},
		}

		// Create WebBrokerApi receiver.
		receiverType := wbb.Receiver.Type
		if receiverType == "" {
			receiverType = "websocket"
		}

		ep, err := r.registry.CreateReceiver(receiverType+"-broker-api", connectors.ReceiverConfig{
			Channel:      ch,
			Processor:    r.hub,
			BrokerDriver: brokerDriver,
			RuntimeID:    r.cfg.RuntimeID,
			Mux:          wsMux,
		})
		if err != nil {
			return fmt.Errorf("failed to create receiver for WebBrokerApi %q: %w", wbb.Name, err)
		}
		r.receivers = append(r.receivers, ep)

		slog.Info("Registered WebBrokerApi binding",
			"name", wbb.Name,
			"context", wbb.Context,
			"version", wbb.Version,
			"receiver", receiverType,
			"topics", allTopics,
			"channels", len(wbb.Channels),
		)
	}

	// Create shared HTTP servers.
	if hasWS {
		wsServer, err := r.newManagedServer("WebSocket", r.cfg.Server.WebSocketPort, wsMux, "", "")
		if err != nil {
			return fmt.Errorf("failed to create WebSocket server: %w", err)
		}
		r.servers = append(r.servers, wsServer)
		// Create WSS server if TLS is enabled
		if r.cfg.Server.WebSocketTLSEnabled {
			wssServer, err := r.newManagedServer("WebSocket-HTTPS", r.cfg.Server.WebSocketHTTPSPort, wsMux, r.cfg.Server.WebSocketTLSCertFile, r.cfg.Server.WebSocketTLSKeyFile)
			if err != nil {
				return fmt.Errorf("failed to create WebSocket HTTPS server: %w", err)
			}
			r.servers = append(r.servers, wssServer)
		}
	}
	if hasWebSub && r.cfg.Server.WebSubEnabled {
		// Create HTTP server
		websubHTTPServer, err := r.newManagedServer("WebSub-HTTP", r.cfg.Server.WebSubHTTPPort, websubMux, "", "")
		if err != nil {
			return fmt.Errorf("failed to create WebSub HTTP server: %w", err)
		}
		r.servers = append(r.servers, websubHTTPServer)
		// Create HTTPS server if TLS is enabled
		if r.cfg.Server.WebSubTLSEnabled {
			websubHTTPSServer, err := r.newManagedServer("WebSub-HTTPS", r.cfg.Server.WebSubHTTPSPort, websubMux, r.cfg.Server.WebSubTLSCertFile, r.cfg.Server.WebSubTLSKeyFile)
			if err != nil {
				return fmt.Errorf("failed to create WebSub HTTPS server: %w", err)
			}
			r.servers = append(r.servers, websubHTTPSServer)
		}
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

	// If in xDS mode, ensure servers are started for dynamic bindings.
	r.mu.Lock()
	if r.cfg.ControlPlane.Enabled {
		// Create WebSocket server for dynamic WebBrokerApi bindings
		slog.Info("Creating WebSocket server for dynamic WebBrokerApi bindings", "port", r.cfg.Server.WebSocketPort)
		wsServer, err := r.newManagedServer("WebSocket", r.cfg.Server.WebSocketPort, r.wsMux, "", "")
		if err != nil {
			r.mu.Unlock()
			return fmt.Errorf("failed to create WebSocket server: %w", err)
		}
		r.servers = append(r.servers, wsServer)
		go func() {
			r.runServer(wsServer)
		}()

		// Create WSS server if TLS is enabled
		if r.cfg.Server.WebSocketTLSEnabled {
			slog.Info("Creating WebSocket HTTPS server for dynamic WebBrokerApi bindings", "port", r.cfg.Server.WebSocketHTTPSPort)
			wssServer, err := r.newManagedServer("WebSocket-HTTPS", r.cfg.Server.WebSocketHTTPSPort, r.wsMux, r.cfg.Server.WebSocketTLSCertFile, r.cfg.Server.WebSocketTLSKeyFile)
			if err != nil {
				r.mu.Unlock()
				return fmt.Errorf("failed to create WebSocket HTTPS server: %w", err)
			}
			r.servers = append(r.servers, wssServer)
			go func() {
				r.runServer(wssServer)
			}()
		}

		// Create WebSub servers for dynamic WebSubApi bindings
		if r.cfg.Server.WebSubEnabled {
			slog.Info("Creating WebSub HTTP server for dynamic WebSubApi bindings", "port", r.cfg.Server.WebSubHTTPPort)
			websubHTTPServer, err := r.newManagedServer("WebSub-HTTP", r.cfg.Server.WebSubHTTPPort, r.websubMux, "", "")
			if err != nil {
				r.mu.Unlock()
				return fmt.Errorf("failed to create WebSub HTTP server: %w", err)
			}
			r.servers = append(r.servers, websubHTTPServer)
			go func() {
				r.runServer(websubHTTPServer)
			}()

			// Create HTTPS server if TLS is enabled
			if r.cfg.Server.WebSubTLSEnabled {
				slog.Info("Creating WebSub HTTPS server for dynamic WebSubApi bindings", "port", r.cfg.Server.WebSubHTTPSPort)
				websubHTTPSServer, err := r.newManagedServer("WebSub-HTTPS", r.cfg.Server.WebSubHTTPSPort, r.websubMux, r.cfg.Server.WebSubTLSCertFile, r.cfg.Server.WebSubTLSKeyFile)
				if err != nil {
					r.mu.Unlock()
					return fmt.Errorf("failed to create WebSub HTTPS server: %w", err)
				}
				r.servers = append(r.servers, websubHTTPSServer)
				go func() {
					r.runServer(websubHTTPSServer)
				}()
			}
		}
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

	if err := r.admin.Stop(shutdownCtx); err != nil {
		slog.Error("Failed to stop admin server", "error", err)
	}

	slog.Info("Event gateway shutdown complete")
	return nil
}

func (r *Runtime) newManagedServer(name string, port int, handler http.Handler, certFile, keyFile string) (*managedServer, error) {
	server := &managedServer{
		name: name,
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: handler,
		},
	}

	if certFile != "" {
		if err := ensureReadableTLSAsset(certFile, name+" TLS cert file"); err != nil {
			return nil, fmt.Errorf("invalid TLS configuration for %s server: %w", name, err)
		}
		if err := ensureReadableTLSAsset(keyFile, name+" TLS key file"); err != nil {
			return nil, fmt.Errorf("invalid TLS configuration for %s server: %w", name, err)
		}
		server.tls = true
		server.certFile = certFile
		server.keyFile = keyFile
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

func (r *Runtime) buildWebSubApiPolicyChains(wsb binding.WebSubApiBinding, vhost string) (subscribeKey, unsubscribeKey, inboundKey, outboundKey string, channelChainKeys map[string]hub.ChannelChainKeySet, err error) {
	basePath := binding.WebSubApiBasePath(wsb.Context, wsb.Version)
	hubPath := basePath + "/hub"

	// Subscribe chain: hub path (subscribe requests).
	subscribeKey = binding.GenerateRouteKey("SUBSCRIBE", hubPath, vhost)
	// Unsubscribe chain: hub path (unsubscribe requests).
	unsubscribeKey = binding.GenerateRouteKey("UNSUBSCRIBE", hubPath, vhost)
	// Inbound chain: webhook-receiver path (data ingress / on_message_received).
	inboundKey = binding.GenerateRouteKey("SUB", basePath+"/webhook-receiver", vhost)
	// Outbound chain: delivery path (data delivery to subscribers / on_message_delivery).
	outboundKey = binding.GenerateRouteKey("DELIVER", hubPath, vhost)

	if err = r.buildChain(subscribeKey, wsb.Policies.Subscribe); err != nil {
		return "", "", "", "", nil, err
	}
	if err = r.buildChain(unsubscribeKey, wsb.Policies.Unsubscribe); err != nil {
		return "", "", "", "", nil, err
	}
	if err = r.buildChain(inboundKey, wsb.Policies.Inbound); err != nil {
		return "", "", "", "", nil, err
	}
	if err = r.buildChain(outboundKey, wsb.Policies.Outbound); err != nil {
		return "", "", "", "", nil, err
	}

	// Build per-channel policy chains.
	channelChainKeys = make(map[string]hub.ChannelChainKeySet, len(wsb.Channels))
	for _, ch := range wsb.Channels {
		if len(ch.Policies.Subscribe) == 0 && len(ch.Policies.Unsubscribe) == 0 && len(ch.Policies.Inbound) == 0 && len(ch.Policies.Outbound) == 0 {
			continue
		}
		chChannelPath := hubPath + "/" + ch.Name
		chSubKey := binding.GenerateRouteKey("SUBSCRIBE", chChannelPath, vhost)
		chUnsubKey := binding.GenerateRouteKey("UNSUBSCRIBE", chChannelPath, vhost)
		chInKey := binding.GenerateRouteKey("SUB", chChannelPath, vhost)
		chOutKey := binding.GenerateRouteKey("DELIVER", chChannelPath, vhost)
		if err = r.buildChain(chSubKey, ch.Policies.Subscribe); err != nil {
			return "", "", "", "", nil, err
		}
		if err = r.buildChain(chUnsubKey, ch.Policies.Unsubscribe); err != nil {
			return "", "", "", "", nil, err
		}
		if err = r.buildChain(chInKey, ch.Policies.Inbound); err != nil {
			return "", "", "", "", nil, err
		}
		if err = r.buildChain(chOutKey, ch.Policies.Outbound); err != nil {
			return "", "", "", "", nil, err
		}
		channelChainKeys[ch.Name] = hub.ChannelChainKeySet{
			SubscribeChainKey:   chSubKey,
			UnsubscribeChainKey: chUnsubKey,
			InboundChainKey:     chInKey,
			OutboundChainKey:    chOutKey,
		}
	}

	return subscribeKey, unsubscribeKey, inboundKey, outboundKey, channelChainKeys, nil
}

// ChannelPolicyChains holds policy chain keys for a single channel.
type ChannelPolicyChains struct {
	ConnInitKey string
	ProduceKey  string
	ConsumeKey  string
}

func (r *Runtime) buildWebBrokerApiPolicyChains(wbb binding.WebBrokerApiBinding, vhost string, channelName string) (connInitKey, produceKey, consumeKey string, err error) {
	basePath := wbb.Context
	if wbb.Version != "" {
		basePath = path.Join(wbb.Context, wbb.Version)
	}

	suffix := ""
	if channelName != "" {
		suffix = "_" + channelName
	}

	// Connection init chain: on_connection_init policies (applied during WebSocket handshake).
	connInitKey = binding.GenerateRouteKey("CONNECT_INIT"+suffix, basePath, vhost)
	// Produce chain: on_produce policies (client → broker).
	produceKey = binding.GenerateRouteKey("PRODUCE"+suffix, basePath, vhost)
	// Consume chain: on_consume policies (broker → client).
	consumeKey = binding.GenerateRouteKey("CONSUME"+suffix, basePath, vhost)

	var onConnInit, onProduce, onConsume []binding.PolicyRef

	if channelName == "" {
		// Build API-level policies
		onConnInit = wbb.Policies.OnConnectionInit
		onProduce = wbb.Policies.OnProduce
		onConsume = wbb.Policies.OnConsume
	} else {
		// Build channel-specific policies
		if channelDef, ok := wbb.Channels[channelName]; ok {
			onConnInit = channelDef.OnConnectionInit
			onProduce = channelDef.OnProduce
			onConsume = channelDef.OnConsume
		}
	}

	if err = r.buildChain(connInitKey, onConnInit); err != nil {
		return "", "", "", err
	}
	if err = r.buildChain(produceKey, onProduce); err != nil {
		return "", "", "", err
	}
	if err = r.buildChain(consumeKey, onConsume); err != nil {
		return "", "", "", err
	}

	return connInitKey, produceKey, consumeKey, nil
}

// extractTopicsFromChannelPolicies extracts Kafka topics to subscribe to from a channel's consumeFrom config.
// If consumeFrom is not specified, defaults to the normalized channel name.
// These topics are used for Kafka consumer subscription.
func extractTopicsFromChannelPolicies(channelName string, channelDef binding.WebBrokerChannelDef) []string {
	topics := make(map[string]bool) // Use map to deduplicate

	// Check consumeFrom field
	if channelDef.ConsumeFrom != nil && channelDef.ConsumeFrom.Topic != "" {
		topics[channelDef.ConsumeFrom.Topic] = true
	}

	// If no consumeFrom topics found, default to normalized channel name
	if len(topics) == 0 {
		topics[binding.NormalizeTopicSegment(channelName)] = true
	}

	// Convert map to slice
	result := make([]string, 0, len(topics))
	for topic := range topics {
		result = append(result, topic)
	}
	return result
}

// extractAllTopicsFromChannelPolicies extracts ALL Kafka topics (both produce and consume) from a channel's config.
// If produceTo/consumeFrom are not specified, defaults to the normalized channel name.
// Used to ensure all necessary topics exist in Kafka before the API starts.
func extractAllTopicsFromChannelPolicies(channelName string, channelDef binding.WebBrokerChannelDef) []string {
	topics := make(map[string]bool) // Use map to deduplicate
	hasProduceTopics := false
	hasConsumeTopics := false

	// Check produceTo field
	if channelDef.ProduceTo != nil && channelDef.ProduceTo.Topic != "" {
		topics[channelDef.ProduceTo.Topic] = true
		hasProduceTopics = true
	}

	// Check consumeFrom field
	if channelDef.ConsumeFrom != nil && channelDef.ConsumeFrom.Topic != "" {
		topics[channelDef.ConsumeFrom.Topic] = true
		hasConsumeTopics = true
	}

	// If no consume topics found, use normalized channel name as default
	if !hasConsumeTopics {
		topics[binding.NormalizeTopicSegment(channelName)] = true
	}

	// If no produce topics were found, also add normalized channel name for producing
	if !hasProduceTopics {
		topics[binding.NormalizeTopicSegment(channelName)] = true
	}

	// Convert map to slice
	result := make([]string, 0, len(topics))
	for topic := range topics {
		result = append(result, topic)
	}
	return result
}

// getChannelNames extracts channel names from the channels map.
func getChannelNames(channels map[string]binding.WebBrokerChannelDef) []string {
	names := make([]string, 0, len(channels))
	for name := range channels {
		names = append(names, name)
	}
	return names
}

// channelChainsToMap converts ChannelPolicyChains map to a map structure
// that can be easily accessed from other packages without type dependencies.
func channelChainsToMap(chains map[string]ChannelPolicyChains) map[string]map[string]string {
	result := make(map[string]map[string]string, len(chains))
	for channelName, chainKeys := range chains {
		result[channelName] = map[string]string{
			"ConnInitKey": chainKeys.ConnInitKey,
			"ProduceKey":  chainKeys.ProduceKey,
			"ConsumeKey":  chainKeys.ConsumeKey,
		}
	}
	return result
}

func (r *Runtime) buildChain(routeKey string, policies []binding.PolicyRef) error {
	if routeKey == "" {
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

	specs = systempolicies.Inject(specs, r.cfg, nil)

	if len(specs) == 0 {
		return nil
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
	if b.UnsubscribeChainKey != "" {
		r.engine.UnregisterChain(b.UnsubscribeChainKey)
	}
	if b.InboundChainKey != "" {
		r.engine.UnregisterChain(b.InboundChainKey)
	}
	if b.OutboundChainKey != "" {
		r.engine.UnregisterChain(b.OutboundChainKey)
	}
	for _, chKeys := range b.ChannelChainKeys {
		if chKeys.SubscribeChainKey != "" {
			r.engine.UnregisterChain(chKeys.SubscribeChainKey)
		}
		if chKeys.UnsubscribeChainKey != "" {
			r.engine.UnregisterChain(chKeys.UnsubscribeChainKey)
		}
		if chKeys.InboundChainKey != "" {
			r.engine.UnregisterChain(chKeys.InboundChainKey)
		}
		if chKeys.OutboundChainKey != "" {
			r.engine.UnregisterChain(chKeys.OutboundChainKey)
		}
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
	internalSubTopic := r.webSubSubscriptionSyncTopic(wsb.Name, wsb.Version)

	// Build policy chains for the API.
	subKey, unsubKey, inKey, outKey, chChainKeys, err := r.buildWebSubApiPolicyChains(wsb, vhost)
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("failed to build chains for WebSubApi %q: %w", wsb.Name, err)
	}

	r.hub.RegisterBinding(hub.ChannelBinding{
		APIID:               wsb.APIID,
		Name:                wsb.Name,
		Mode:                "websub",
		Context:             wsb.Context,
		Version:             wsb.Version,
		Vhost:               vhost,
		SubscribeChainKey:   subKey,
		UnsubscribeChainKey: unsubKey,
		InboundChainKey:     inKey,
		OutboundChainKey:    outKey,
		Channels:            channels,
		ChannelChainKeys:    chChainKeys,
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

// AddWebBrokerApiBinding dynamically adds a WebBrokerApi binding at runtime.
func (r *Runtime) AddWebBrokerApiBinding(wbb binding.WebBrokerApiBinding) error {
	r.mu.Lock()

	vhost := defaultVhost(wbb.Vhost)

	// Build API-level policy chains.
	apiConnInitKey, _, _, err := r.buildWebBrokerApiPolicyChains(wbb, vhost, "")
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("failed to build API-level chains for WebBrokerApi %q: %w", wbb.Name, err)
	}

	// Build per-channel policy chains and collect topics.
	channelChains := make(map[string]ChannelPolicyChains)
	allTopics := []string{}                             // All topics (produce + consume) for ensuring they exist
	topicToChannel := make(map[string]string)           // Only consume topics for subscription mapping
	channelTopics := make(map[string]map[string]string) // Channel-level topic mappings (produceTo, consumeFrom)

	for channelName, channelDef := range wbb.Channels {
		connInitKey, produceKey, consumeKey, err := r.buildWebBrokerApiPolicyChains(wbb, vhost, channelName)
		if err != nil {
			r.mu.Unlock()
			return fmt.Errorf("failed to build chains for channel %q in WebBrokerApi %q: %w", channelName, wbb.Name, err)
		}

		channelChains[channelName] = ChannelPolicyChains{
			ConnInitKey: connInitKey,
			ProduceKey:  produceKey,
			ConsumeKey:  consumeKey,
		}

		// Store channel topic mappings (use defaults if not specified)
		topicMapping := make(map[string]string)
		if channelDef.ProduceTo != nil && channelDef.ProduceTo.Topic != "" {
			topicMapping["produceTo"] = channelDef.ProduceTo.Topic
		} else {
			// Default: use normalized channel name for producing
			topicMapping["produceTo"] = binding.NormalizeTopicSegment(channelName)
		}
		if channelDef.ConsumeFrom != nil && channelDef.ConsumeFrom.Topic != "" {
			topicMapping["consumeFrom"] = channelDef.ConsumeFrom.Topic
		} else {
			// Default: use normalized channel name for consuming
			topicMapping["consumeFrom"] = binding.NormalizeTopicSegment(channelName)
		}
		channelTopics[channelName] = topicMapping

		// Extract ALL topics (produce + consume) to ensure they exist in Kafka
		allChannelTopics := extractAllTopicsFromChannelPolicies(channelName, channelDef)
		allTopics = append(allTopics, allChannelTopics...)

		// Extract ONLY consume topics for subscription mapping
		consumeTopics := extractTopicsFromChannelPolicies(channelName, channelDef)
		for _, topic := range consumeTopics {
			topicToChannel[topic] = channelName
		}

		slog.Info("Built policy chains for WebBrokerApi channel",
			"api", wbb.Name,
			"channel", channelName,
			"topics", allChannelTopics)
	}

	r.hub.RegisterBinding(hub.ChannelBinding{
		APIID:             wbb.APIID,
		Name:              wbb.Name,
		Mode:              "protocol-mediation",
		Context:           wbb.Context,
		Version:           wbb.Version,
		Vhost:             vhost,
		SubscribeChainKey: apiConnInitKey,
		InboundChainKey:   "", // Determined per-channel
		OutboundChainKey:  "", // Determined per-channel
	})

	// Create broker-driver.
	brokerDriverType := "kafka"
	if wbb.BrokerDriver.Type != "" {
		brokerDriverType = wbb.BrokerDriver.Type
	}
	brokerDriver, err := r.registry.CreateBrokerDriver(brokerDriverType, wbb.BrokerDriver.Config)
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("failed to create broker-driver for WebBrokerApi %q: %w", wbb.Name, err)
	}
	r.activeBrokerDrivers[wbb.Name] = brokerDriver

	ch := connectors.ChannelInfo{
		Name:    wbb.Name,
		Mode:    "protocol-mediation",
		Context: wbb.Context,
		Version: wbb.Version,
		Vhost:   vhost,
		Topics:  allTopics,
		Metadata: map[string]interface{}{
			"channelChains":  channelChainsToMap(channelChains),
			"topicToChannel": topicToChannel,
			"channelNames":   getChannelNames(wbb.Channels),
			"channelTopics":  channelTopics,
		},
	}

	// Determine receiver type (websocket, sse, etc.)
	receiverType := "websocket-broker-api"
	if wbb.Receiver.Type != "" {
		receiverType = wbb.Receiver.Type + "-broker-api"
	}

	receiver, err := r.registry.CreateReceiver(receiverType, connectors.ReceiverConfig{
		Channel:      ch,
		Processor:    r.hub,
		BrokerDriver: brokerDriver,
		RuntimeID:    r.cfg.RuntimeID,
		Mux:          r.wsMux,
	})
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("failed to create receiver for WebBrokerApi %q: %w", wbb.Name, err)
	}
	r.activeReceivers[wbb.Name] = receiver
	r.bindingPaths[wbb.Name] = []string{wbb.Context}

	startNow := r.running
	startCtx := r.runCtx
	r.mu.Unlock()

	// If runtime is already running, start the receiver immediately.
	if startNow {
		if startCtx == nil {
			startCtx = context.Background()
		}
		if err := r.startReceiverWithRetry(startCtx, wbb.Name, receiver); err != nil {
			return fmt.Errorf("failed to start receiver for WebBrokerApi %q: %w", wbb.Name, err)
		}
	}

	slog.Info("Dynamically added WebBrokerApi binding",
		"name", wbb.Name,
		"context", wbb.Context,
		"version", wbb.Version,
		"receiver_type", receiverType,
		"channels", len(wbb.Channels),
		"topics", allTopics)

	return nil
}

// RemoveWebBrokerApiBinding dynamically removes a WebBrokerApi binding at runtime.
func (r *Runtime) RemoveWebBrokerApiBinding(name string) error {
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

	// Close and remove broker driver.
	if bd, ok := r.activeBrokerDrivers[name]; ok {
		if err := bd.Close(); err != nil {
			slog.Error("Failed to close broker-driver during removal", "name", name, "error", err)
		}
		delete(r.activeBrokerDrivers, name)
	}

	// Remove binding chains.
	r.unregisterBindingChains(r.hub.GetBinding(name))

	// Remove hub binding.
	r.hub.RemoveBinding(name)

	// Deregister HTTP routes from the mux.
	if paths, ok := r.bindingPaths[name]; ok {
		for _, p := range paths {
			r.wsMux.Remove(p)
		}
		delete(r.bindingPaths, name)
	}

	slog.Info("Dynamically removed WebBrokerApi binding", "name", name)
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

func (r *Runtime) webSubSubscriptionSyncTopic(apiName, version string) string {
	suffix := "__subscriptions"
	if r != nil && r.cfg != nil && r.cfg.WebSub.SubscriptionsTopicName != "" {
		suffix = r.cfg.WebSub.SubscriptionsTopicName
	}
	return binding.WebSubApiTopicName(apiName, version, suffix)
}
