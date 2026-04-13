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

package binding

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseChannels_WebSubApi(t *testing.T) {
	yaml := `
channels:
  - kind: WebSubApi
    name: repo-watcher
    version: v1
    context: /repos
    channels:
      - name: issues
      - name: pull-requests
      - name: commits
    entrypoint:
      type: websub
    endpoint:
      type: kafka
      config:
        brokers:
          - kafka:29092
    policies:
      inbound: []
      outbound: []
`
	path := writeTempYAML(t, yaml)
	result, err := ParseChannels(path)
	if err != nil {
		t.Fatalf("ParseChannels failed: %v", err)
	}

	if len(result.Bindings) != 0 {
		t.Errorf("expected 0 legacy bindings, got %d", len(result.Bindings))
	}

	if len(result.WebSubApiBindings) != 1 {
		t.Fatalf("expected 1 WebSubApi binding, got %d", len(result.WebSubApiBindings))
	}

	wsb := result.WebSubApiBindings[0]
	if wsb.Kind != "WebSubApi" {
		t.Errorf("expected kind WebSubApi, got %q", wsb.Kind)
	}
	if wsb.Name != "repo-watcher" {
		t.Errorf("expected name repo-watcher, got %q", wsb.Name)
	}
	if wsb.Version != "v1" {
		t.Errorf("expected version v1, got %q", wsb.Version)
	}
	if wsb.Context != "/repos" {
		t.Errorf("expected context /repos, got %q", wsb.Context)
	}
	if len(wsb.Channels) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(wsb.Channels))
	}

	expectedChannels := []string{"issues", "pull-requests", "commits"}
	for i, ch := range wsb.Channels {
		if ch.Name != expectedChannels[i] {
			t.Errorf("channel %d: expected %q, got %q", i, expectedChannels[i], ch.Name)
		}
	}
}

func TestParseChannels_LegacyBinding(t *testing.T) {
	yaml := `
channels:
  - name: live-prices
    mode: protocol-mediation
    context: /prices
    version: v1
    entrypoint:
      type: websocket
      path: /stream
    endpoint:
      type: kafka
      topic: price-updates
      config:
        brokers:
          - kafka:29092
    policies:
      inbound: []
      outbound: []
`
	path := writeTempYAML(t, yaml)
	result, err := ParseChannels(path)
	if err != nil {
		t.Fatalf("ParseChannels failed: %v", err)
	}

	if len(result.WebSubApiBindings) != 0 {
		t.Errorf("expected 0 WebSubApi bindings, got %d", len(result.WebSubApiBindings))
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 legacy binding, got %d", len(result.Bindings))
	}

	b := result.Bindings[0]
	if b.Name != "live-prices" {
		t.Errorf("expected name live-prices, got %q", b.Name)
	}
	if b.Mode != "protocol-mediation" {
		t.Errorf("expected mode protocol-mediation, got %q", b.Mode)
	}
}

func TestParseChannels_Mixed(t *testing.T) {
	yaml := `
channels:
  - kind: WebSubApi
    name: events-api
    version: v2
    context: /events
    channels:
      - name: orders
      - name: payments
    entrypoint:
      type: websub
    endpoint:
      type: kafka
      config:
        brokers:
          - kafka:29092
    policies:
      inbound: []
      outbound: []

  - name: stream-api
    mode: protocol-mediation
    context: /stream
    version: v1
    entrypoint:
      type: websocket
      path: /ws
    endpoint:
      type: kafka
      topic: stream-data
      config:
        brokers:
          - kafka:29092
    policies:
      inbound: []
      outbound: []
`
	path := writeTempYAML(t, yaml)
	result, err := ParseChannels(path)
	if err != nil {
		t.Fatalf("ParseChannels failed: %v", err)
	}

	if len(result.WebSubApiBindings) != 1 {
		t.Errorf("expected 1 WebSubApi binding, got %d", len(result.WebSubApiBindings))
	}
	if len(result.Bindings) != 1 {
		t.Errorf("expected 1 legacy binding, got %d", len(result.Bindings))
	}
}

func TestWebSubApiTopicName(t *testing.T) {
	tests := []struct {
		apiName     string
		version     string
		channelName string
		expected    string
	}{
		{"repo-watcher", "v1", "issues", "repo-watcher.v1.issues"},
		{"repo-watcher", "v1", "pull-requests", "repo-watcher.v1.pull-requests"},
		{"order-api", "v2", "orders", "order-api.v2.orders"},
	}

	for _, tt := range tests {
		got := WebSubApiTopicName(tt.apiName, tt.version, tt.channelName)
		if got != tt.expected {
			t.Errorf("WebSubApiTopicName(%q, %q, %q) = %q, want %q",
				tt.apiName, tt.version, tt.channelName, got, tt.expected)
		}
	}
}

func TestWebSubApiSubscriptionTopic(t *testing.T) {
	got := WebSubApiSubscriptionTopic("repo-watcher", "v1")
	expected := "repo-watcher.v1.__subscriptions"
	if got != expected {
		t.Errorf("WebSubApiSubscriptionTopic = %q, want %q", got, expected)
	}
}

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "channels.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}
