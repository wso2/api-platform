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
	"errors"
	"net/http"
	"os"
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

	subKey, inKey, outKey, err := rt.buildWebSubApiPolicyChains(oldBinding, defaultVhost(""))
	if err != nil {
		t.Fatalf("failed to build initial policy chains: %v", err)
	}

	rt.hub.RegisterBinding(hub.ChannelBinding{
		Name:              oldBinding.Name,
		SubscribeChainKey: subKey,
		InboundChainKey:   inKey,
		OutboundChainKey:  outKey,
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

	newSubKey, newInKey, newOutKey, err := rt.buildWebSubApiPolicyChains(updatedBinding, defaultVhost(""))
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

func TestNewManagedServerEnablesTLSWhenFilesExist(t *testing.T) {
	certFile := writeTestTLSFile(t, "tls.crt")
	keyFile := writeTestTLSFile(t, "tls.key")

	rt := &Runtime{
		cfg: &config.Config{
			Server: config.ServerConfig{
				WebSubTLSEnabled:  true,
				WebSubTLSCertFile: certFile,
				WebSubTLSKeyFile:  keyFile,
			},
		},
	}

	server := rt.newManagedServer("WebSub", 8080, http.NewServeMux(), true)
	if !server.tls {
		t.Fatal("expected TLS to be enabled when certificate and key files exist")
	}
	if server.certFile != certFile {
		t.Fatalf("expected cert file %q, got %q", certFile, server.certFile)
	}
	if server.keyFile != keyFile {
		t.Fatalf("expected key file %q, got %q", keyFile, server.keyFile)
	}
}

func TestNewManagedServerDoesNotEnableTLSForInvalidTLSFiles(t *testing.T) {
	existingKey := writeTestTLSFile(t, "tls.key")

	tests := []struct {
		name     string
		certFile string
		keyFile  string
	}{
		{
			name:     "empty certificate path",
			certFile: "",
			keyFile:  existingKey,
		},
		{
			name:     "missing certificate file",
			certFile: t.TempDir() + "/missing.crt",
			keyFile:  existingKey,
		},
		{
			name:     "empty key path",
			certFile: writeTestTLSFile(t, "tls.crt"),
			keyFile:  "",
		},
		{
			name:     "missing key file",
			certFile: writeTestTLSFile(t, "second-tls.crt"),
			keyFile:  t.TempDir() + "/missing.key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &Runtime{
				cfg: &config.Config{
					Server: config.ServerConfig{
						WebSubTLSEnabled:  true,
						WebSubTLSCertFile: tt.certFile,
						WebSubTLSKeyFile:  tt.keyFile,
					},
				},
			}

			server := rt.newManagedServer("WebSub", 8080, http.NewServeMux(), true)
			if server.tls {
				t.Fatal("expected TLS to remain disabled when certificate or key validation fails")
			}
		})
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

func newTestNoopPolicy(policy.PolicyMetadata, map[string]interface{}) (policy.Policy, error) {
	return testNoopPolicy{}, nil
}

func writeTestTLSFile(t *testing.T, name string) string {
	t.Helper()

	filePath := t.TempDir() + "/" + name
	if err := os.WriteFile(filePath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write test TLS file: %v", err)
	}
	return filePath
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
