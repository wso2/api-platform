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
	"crypto/tls"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/binding"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/config"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/hub"
	enginepkg "github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/pkg/engine"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func TestRemoveWebSubApiBinding_ClearsStalePolicyChains(t *testing.T) {
	eng, err := enginepkg.New(nil)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	if err := eng.RegisterPolicy(&policy.PolicyDefinition{
		Name:    "test-noop",
		Version: "v1.0.0",
	}, newTestNoopPolicy); err != nil {
		t.Fatalf("failed to register test policy: %v", err)
	}

	rt := &Runtime{
		engine:              eng,
		hub:                 hub.NewHub(eng),
		activeReceivers:     make(map[string]connectors.Receiver),
		activeBrokerDrivers: make(map[string]connectors.BrokerDriver),
		bindingPaths:        make(map[string][]string),
		bindingTopics:       make(map[string][]string),
		websubMux:           NewDynamicMux(),
	}

	oldBinding := binding.WebSubApiBinding{
		APIID:   "api-1",
		Name:    "githubser",
		Context: "/proj1/githubser",
		Version: "v1.0",
		Policies: binding.PolicyBindings{
			Subscribe: []binding.PolicyRef{{Name: "test-noop", Version: "v1.0.0"}},
			Inbound:   []binding.PolicyRef{{Name: "test-noop", Version: "v1.0.0"}},
			Outbound:  []binding.PolicyRef{{Name: "test-noop", Version: "v1.0.0"}},
		},
	}

	subKey, unsubKey, inKey, outKey, _, err := rt.buildWebSubApiPolicyChains(oldBinding, defaultVhost(""))
	if err != nil {
		t.Fatalf("failed to build initial policy chains: %v", err)
	}

	rt.hub.RegisterBinding(hub.ChannelBinding{
		Name:                oldBinding.Name,
		SubscribeChainKey:   subKey,
		UnsubscribeChainKey: unsubKey,
		InboundChainKey:     inKey,
		OutboundChainKey:    outKey,
	})

	if got := eng.GetChain(subKey); got == nil {
		t.Fatal("expected subscribe chain to be registered")
	}
	if got := eng.GetChain(inKey); got == nil {
		t.Fatal("expected inbound chain to be registered")
	}
	if got := eng.GetChain(outKey); got == nil {
		t.Fatal("expected outbound chain to be registered")
	}

	if err := rt.RemoveWebSubApiBinding(oldBinding.Name); err != nil {
		t.Fatalf("RemoveWebSubApiBinding returned error: %v", err)
	}

	if got := eng.GetChain(subKey); got != nil {
		t.Fatal("expected subscribe chain to be removed with binding")
	}
	if got := eng.GetChain(inKey); got != nil {
		t.Fatal("expected inbound chain to be removed with binding")
	}
	if got := eng.GetChain(outKey); got != nil {
		t.Fatal("expected outbound chain to be removed with binding")
	}

	updatedBinding := oldBinding
	updatedBinding.Policies = binding.PolicyBindings{}

	newSubKey, _, newInKey, newOutKey, _, err := rt.buildWebSubApiPolicyChains(updatedBinding, defaultVhost(""))
	if err != nil {
		t.Fatalf("failed to build updated policy chains: %v", err)
	}

	if newSubKey != subKey || newInKey != inKey || newOutKey != outKey {
		t.Fatal("expected route keys to remain stable across policy-only updates")
	}
	if got := eng.GetChain(subKey); got != nil {
		t.Fatal("expected empty-policy redeploy to leave subscribe chain unregistered")
	}
	if got := eng.GetChain(inKey); got != nil {
		t.Fatal("expected empty-policy redeploy to leave inbound chain unregistered")
	}
	if got := eng.GetChain(outKey); got != nil {
		t.Fatal("expected empty-policy redeploy to leave outbound chain unregistered")
	}
}

func TestJoinNormalizedTopic_NormalizesUnsupportedCharacters(t *testing.T) {
	got := binding.JoinNormalizedTopic("/orders/eu", "v1/test", "order_events")
	// SHA-256 hash of "10:/orders/eu|7:v1/test|12:order_events|"
	want := "2836e285c333e251f0929fbbbe31dec3bb7d61423761103d790bcef2429b194b"
	if got != want {
		t.Fatalf("JoinNormalizedTopic() = %q, want %q", got, want)
	}
}

func TestWebSubSubscriptionSyncTopic_UsesConfigOverrideWhenSet(t *testing.T) {
	rt := &Runtime{
		cfg: &config.Config{
			WebSub: config.WebSubConfig{
				SubscriptionsTopicName: "websub.subscriptions",
			},
		},
	}

	got := rt.webSubSubscriptionSyncTopic("repo-watcher", "v1.0")
	want := binding.WebSubApiTopicName("repo-watcher", "v1.0", "websub.subscriptions")
	if got != want {
		t.Fatalf("webSubSubscriptionSyncTopic() = %q, want %q", got, want)
	}
}

func TestWebSubSubscriptionSyncTopic_FallsBackToDerivedTopic(t *testing.T) {
	rt := &Runtime{cfg: &config.Config{}}

	got := rt.webSubSubscriptionSyncTopic("repo-watcher", "v1.0")
	want := binding.WebSubApiTopicName("repo-watcher", "v1.0", "__subscriptions")
	if got != want {
		t.Fatalf("webSubSubscriptionSyncTopic() = %q, want %q", got, want)
	}
}

func TestStartReceiverWithRetry_RetriesUntilSuccess(t *testing.T) {
	previousInitial := initialReceiverStartBackoff
	previousMax := maxReceiverStartBackoff
	initialReceiverStartBackoff = time.Millisecond
	maxReceiverStartBackoff = time.Millisecond
	defer func() {
		initialReceiverStartBackoff = previousInitial
		maxReceiverStartBackoff = previousMax
	}()

	rt := &Runtime{}
	receiver := &flakyReceiver{failuresLeft: 2}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := rt.startReceiverWithRetry(ctx, "test-binding", receiver); err != nil {
		t.Fatalf("startReceiverWithRetry returned error: %v", err)
	}
	if got := receiver.Attempts(); got != 3 {
		t.Fatalf("expected 3 start attempts, got %d", got)
	}
}

func TestStartReceiverWithRetry_StopsWhenContextIsCanceled(t *testing.T) {
	previousInitial := initialReceiverStartBackoff
	previousMax := maxReceiverStartBackoff
	initialReceiverStartBackoff = time.Millisecond
	maxReceiverStartBackoff = time.Millisecond
	defer func() {
		initialReceiverStartBackoff = previousInitial
		maxReceiverStartBackoff = previousMax
	}()

	rt := &Runtime{}
	receiver := &flakyReceiver{failuresLeft: 100}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	if err := rt.startReceiverWithRetry(ctx, "test-binding", receiver); err == nil {
		t.Fatal("expected startReceiverWithRetry to return an error when context is canceled")
	}
	if got := receiver.Attempts(); got < 2 {
		t.Fatalf("expected multiple start attempts before cancellation, got %d", got)
	}
}

func TestNewManagedServerRejectsMissingTLSFiles(t *testing.T) {
	rt := &Runtime{
		cfg: &config.Config{
			Server: config.ServerConfig{
				WebSubTLSEnabled:  true,
				WebSubTLSCertFile: filepath.Join(t.TempDir(), "missing.crt"),
				WebSubTLSKeyFile:  filepath.Join(t.TempDir(), "missing.key"),
			},
		},
	}

	_, err := rt.newManagedServer("WebSub-HTTPS", 8443, http.NewServeMux(), rt.cfg.Server.WebSubTLSCertFile, rt.cfg.Server.WebSubTLSKeyFile, serverTLSOptions{})
	if err == nil {
		t.Fatal("expected newManagedServer to fail when TLS files are missing")
	}
	if !strings.Contains(err.Error(), "invalid TLS configuration for WebSub-HTTPS server") {
		t.Fatalf("expected wrapped TLS configuration error, got %q", err.Error())
	}
}

func TestNewManagedServerAcceptsReadableTLSFiles(t *testing.T) {
	tempDir := t.TempDir()
	certPath := filepath.Join(tempDir, "tls.crt")
	keyPath := filepath.Join(tempDir, "tls.key")
	if err := os.WriteFile(certPath, []byte("cert"), 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	rt := &Runtime{
		cfg: &config.Config{
			Server: config.ServerConfig{
				WebSubTLSEnabled:          true,
				WebSubTLSCertFile:         certPath,
				WebSubTLSKeyFile:          keyPath,
				WebSubTLSMinVersion:       "TLS1_2",
				WebSubTLSMaxVersion:       "TLS1_3",
				WebSubTLSCurvePreferences: "X25519MLKEM768,X25519,P-256",
			},
		},
	}

	server, err := rt.newManagedServer("WebSub-HTTPS", 8443, http.NewServeMux(), certPath, keyPath, webSubServerTLSOptions(rt.cfg.Server))
	if err != nil {
		t.Fatalf("expected newManagedServer to succeed, got %v", err)
	}
	if !server.tls {
		t.Fatal("expected TLS to be enabled on the managed server")
	}
	if server.certFile != certPath {
		t.Fatalf("expected cert path %q, got %q", certPath, server.certFile)
	}
	if server.keyFile != keyPath {
		t.Fatalf("expected key path %q, got %q", keyPath, server.keyFile)
	}
	if server.server.TLSConfig == nil {
		t.Fatal("expected TLSConfig to be set on the managed server")
	}
	if server.server.TLSConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("expected MinVersion TLS1.2, got %x", server.server.TLSConfig.MinVersion)
	}
	if server.server.TLSConfig.MaxVersion != tls.VersionTLS13 {
		t.Fatalf("expected MaxVersion TLS1.3, got %x", server.server.TLSConfig.MaxVersion)
	}
	wantCurves := []tls.CurveID{tls.X25519MLKEM768, tls.X25519, tls.CurveP256}
	if !reflect.DeepEqual(server.server.TLSConfig.CurvePreferences, wantCurves) {
		t.Fatalf("expected curve preferences %v, got %v", wantCurves, server.server.TLSConfig.CurvePreferences)
	}
}

func TestNewManagedServerRejectsInvalidTLSTuning(t *testing.T) {
	tempDir := t.TempDir()
	certPath := filepath.Join(tempDir, "tls.crt")
	keyPath := filepath.Join(tempDir, "tls.key")
	if err := os.WriteFile(certPath, []byte("cert"), 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	rt := &Runtime{cfg: &config.Config{}}

	_, err := rt.newManagedServer("WebSub-HTTPS", 8443, http.NewServeMux(), certPath, keyPath, serverTLSOptions{CurvePreferences: "not-a-curve"})
	if err == nil {
		t.Fatal("expected newManagedServer to fail on an invalid curve name")
	}
	if !strings.Contains(err.Error(), "invalid TLS configuration for WebSub-HTTPS server") {
		t.Fatalf("expected wrapped TLS configuration error, got %q", err.Error())
	}
}

func TestNewManagedServerDefaultsToHybridPQCCurvesWhenUnset(t *testing.T) {
	tempDir := t.TempDir()
	certPath := filepath.Join(tempDir, "tls.crt")
	keyPath := filepath.Join(tempDir, "tls.key")
	if err := os.WriteFile(certPath, []byte("cert"), 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	rt := &Runtime{cfg: &config.Config{}}

	server, err := rt.newManagedServer("WebSub-HTTPS", 8443, http.NewServeMux(), certPath, keyPath, serverTLSOptions{})
	if err != nil {
		t.Fatalf("expected newManagedServer to succeed, got %v", err)
	}
	wantCurves := []tls.CurveID{tls.X25519MLKEM768, tls.X25519, tls.CurveP256, tls.CurveP384}
	if !reflect.DeepEqual(server.server.TLSConfig.CurvePreferences, wantCurves) {
		t.Fatalf("expected default hybrid PQC curve preferences %v, got %v", wantCurves, server.server.TLSConfig.CurvePreferences)
	}
}

func TestNewManagedServerAppliesConfiguredTimeouts(t *testing.T) {
	rt := &Runtime{
		cfg: &config.Config{
			Server: config.ServerConfig{
				ReadTimeout:    11 * time.Second,
				WriteTimeout:   22 * time.Second,
				IdleTimeout:    33 * time.Second,
				MaxHeaderBytes: 4096,
			},
		},
	}

	server, err := rt.newManagedServer("WebSub-HTTP", 8080, http.NewServeMux(), "", "", serverTLSOptions{})
	if err != nil {
		t.Fatalf("expected newManagedServer to succeed, got %v", err)
	}
	if server.server.ReadTimeout != 11*time.Second {
		t.Errorf("expected ReadTimeout=11s, got %v", server.server.ReadTimeout)
	}
	if server.server.WriteTimeout != 22*time.Second {
		t.Errorf("expected WriteTimeout=22s, got %v", server.server.WriteTimeout)
	}
	if server.server.IdleTimeout != 33*time.Second {
		t.Errorf("expected IdleTimeout=33s, got %v", server.server.IdleTimeout)
	}
	if server.server.MaxHeaderBytes != 4096 {
		t.Errorf("expected MaxHeaderBytes=4096, got %d", server.server.MaxHeaderBytes)
	}
}

func TestNewManagedServerWebSocketRejectsMissingTLSFiles(t *testing.T) {
	rt := &Runtime{
		cfg: &config.Config{
			Server: config.ServerConfig{
				WebSocketTLSEnabled:  true,
				WebSocketTLSCertFile: filepath.Join(t.TempDir(), "missing.crt"),
				WebSocketTLSKeyFile:  filepath.Join(t.TempDir(), "missing.key"),
			},
		},
	}

	_, err := rt.newManagedServer("WebSocket-HTTPS", 8444, http.NewServeMux(), rt.cfg.Server.WebSocketTLSCertFile, rt.cfg.Server.WebSocketTLSKeyFile, serverTLSOptions{})
	if err == nil {
		t.Fatal("expected newManagedServer to fail when TLS files are missing")
	}
	if !strings.Contains(err.Error(), "invalid TLS configuration for WebSocket-HTTPS server") {
		t.Fatalf("expected wrapped TLS configuration error, got %q", err.Error())
	}
}

func TestNewManagedServerWebSocketAcceptsReadableTLSFiles(t *testing.T) {
	tempDir := t.TempDir()
	certPath := filepath.Join(tempDir, "tls.crt")
	keyPath := filepath.Join(tempDir, "tls.key")
	if err := os.WriteFile(certPath, []byte("cert"), 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	rt := &Runtime{
		cfg: &config.Config{
			Server: config.ServerConfig{
				WebSocketTLSEnabled:      true,
				WebSocketTLSCertFile:     certPath,
				WebSocketTLSKeyFile:      keyPath,
				WebSocketTLSCipherSuites: "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			},
		},
	}

	server, err := rt.newManagedServer("WebSocket-HTTPS", 8444, http.NewServeMux(), certPath, keyPath, webSocketServerTLSOptions(rt.cfg.Server))
	if err != nil {
		t.Fatalf("expected newManagedServer to succeed, got %v", err)
	}
	if !server.tls {
		t.Fatal("expected TLS to be enabled on the managed server")
	}
	if server.certFile != certPath {
		t.Fatalf("expected cert path %q, got %q", certPath, server.certFile)
	}
	if server.keyFile != keyPath {
		t.Fatalf("expected key path %q, got %q", keyPath, server.keyFile)
	}
	if server.server.TLSConfig == nil {
		t.Fatal("expected TLSConfig to be set on the managed server")
	}
	wantCiphers := []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256}
	if !reflect.DeepEqual(server.server.TLSConfig.CipherSuites, wantCiphers) {
		t.Fatalf("expected cipher suites %v, got %v", wantCiphers, server.server.TLSConfig.CipherSuites)
	}
}

func TestAddWebBrokerApiBinding_RedeployDoesNotPanic(t *testing.T) {
	eng, err := enginepkg.New(nil)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	reg := connectors.NewRegistry()
	reg.RegisterBrokerDriver("kafka", func(_ map[string]interface{}) (connectors.BrokerDriver, error) {
		return &testBrokerDriver{}, nil
	})
	reg.RegisterReceiver("websocket-broker-api", func(cfg connectors.ReceiverConfig) (connectors.Receiver, error) {
		cfg.Mux.HandleFunc(cfg.Channel.Context, func(http.ResponseWriter, *http.Request) {})
		return &flakyReceiver{}, nil
	})

	rt := &Runtime{
		cfg:                 &config.Config{},
		engine:              eng,
		hub:                 hub.NewHub(eng),
		registry:            reg,
		activeReceivers:     make(map[string]connectors.Receiver),
		activeBrokerDrivers: make(map[string]connectors.BrokerDriver),
		bindingPaths:        make(map[string][]string),
		bindingTopics:       make(map[string][]string),
		websubMux:           NewDynamicMux(),
		wsMux:               NewDynamicMux(),
	}

	wbb := binding.WebBrokerApiBinding{
		APIID:   "api-1",
		Name:    "webbroker-test",
		Context: "/default/webbroker-test",
		Version: "v1.0",
		BrokerDriver: binding.BrokerDriverSpec{
			Type: "kafka",
		},
	}

	if err := rt.AddWebBrokerApiBinding(wbb); err != nil {
		t.Fatalf("first AddWebBrokerApiBinding failed: %v", err)
	}
	if _, ok := rt.bindingPaths[wbb.Name]; !ok {
		t.Fatal("expected bindingPaths to be populated after first add")
	}

	if err := rt.RemoveWebBrokerApiBinding(wbb.Name); err != nil {
		t.Fatalf("RemoveWebBrokerApiBinding failed: %v", err)
	}
	if _, ok := rt.bindingPaths[wbb.Name]; ok {
		t.Fatal("expected bindingPaths to be cleared after remove")
	}

	// Before the fix this panicked: "pattern already registered" on the http.ServeMux.
	if err := rt.AddWebBrokerApiBinding(wbb); err != nil {
		t.Fatalf("second AddWebBrokerApiBinding (redeploy) failed: %v", err)
	}
}

type testNoopPolicy struct{}

type flakyReceiver struct {
	mu           sync.Mutex
	failuresLeft int
	attempts     int
}

func (r *flakyReceiver) Start(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.attempts++
	if r.failuresLeft > 0 {
		r.failuresLeft--
		return errors.New("broker unavailable")
	}
	return nil
}

func (r *flakyReceiver) Stop(context.Context) error {
	return nil
}

func (r *flakyReceiver) Attempts() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.attempts
}

type testBrokerDriver struct{}

func (testBrokerDriver) Publish(_ context.Context, _ string, _ *connectors.Message) error {
	return nil
}
func (testBrokerDriver) Subscribe(_ string, _ []string, _ connectors.MessageHandler) (connectors.Receiver, error) {
	return &flakyReceiver{}, nil
}
func (testBrokerDriver) SubscribeManual(_ string, _ []string, _ connectors.MessageHandler) (connectors.Receiver, error) {
	return &flakyReceiver{}, nil
}
func (testBrokerDriver) Replay(_ context.Context, _ string, _ connectors.MessageHandler) error {
	return nil
}
func (testBrokerDriver) Watch(_ context.Context, _ string, _ string, _ connectors.MessageHandler) (connectors.Receiver, error) {
	return &flakyReceiver{}, nil
}
func (testBrokerDriver) TopicExists(_ context.Context, _ string) (bool, error) { return true, nil }
func (testBrokerDriver) EnsureTopics(_ context.Context, _ []string, _ map[string]map[string]string) error {
	return nil
}
func (testBrokerDriver) EnsureCompactedTopic(_ context.Context, _ string) error { return nil }
func (testBrokerDriver) DeleteTopics(_ context.Context, _ []string) error       { return nil }
func (testBrokerDriver) Close() error                                           { return nil }

func newTestNoopPolicy(policy.PolicyMetadata, map[string]interface{}) (policy.Policy, error) {
	return testNoopPolicy{}, nil
}

func (testNoopPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

func (testNoopPolicy) OnRequestHeaders(context.Context, *policy.RequestHeaderContext, map[string]interface{}) policy.RequestHeaderAction {
	return policy.UpstreamRequestHeaderModifications{}
}

func (testNoopPolicy) OnResponseHeaders(context.Context, *policy.ResponseHeaderContext, map[string]interface{}) policy.ResponseHeaderAction {
	return policy.DownstreamResponseHeaderModifications{}
}
