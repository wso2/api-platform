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
    receiver:
      type: websub
    broker-driver:
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
    receiver:
      type: websocket
      path: /stream
    broker-driver:
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
    receiver:
      type: websub
    broker-driver:
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
    receiver:
      type: websocket
      path: /ws
    broker-driver:
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
		{"repo-watcher", "v1", "issues", "6a72e9e6ebbde08da54961b430989a7b8ac451a384ab36665543fc548563e199"},
		{"repo-watcher", "v1", "pull-requests", "b4b6bfa33cf4697e35c7fa322f03faacedc69a2647cd595d591d465bf717b3aa"},
		{"order-api", "v2", "orders", "9951c9bf27fa5944a610477cec7be45248147a8751108ef307317f2fb7ab0511"},
		{"repo/watcher", "v1", "/api/er3", "43a2f2a18018d0010b084f602b32b8f2989957f381aa3154711fdb3171ee2509"},
		{"repo_watcher", "v1/test", "pull_requests", "297f30b5b806966ab875d35a30472308e606c4c72c0198b2bc5f1fef302b0a85"},
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
	tests := []struct {
		apiName  string
		version  string
		expected string
	}{
		{"repo-watcher", "v1", "9b20e5d2847060d60712b7e8858cb7a8c3fa638fa4fc443cccb8d6cc1daaef99"},
		{"repo/watcher", "v1/test", "1c45f90f00ebdbfe43419f76e0d468b200e4f26db9587d6ea7cbc87b5aaa8d23"},
	}

	for _, tt := range tests {
		got := WebSubApiSubscriptionTopic(tt.apiName, tt.version)
		if got != tt.expected {
			t.Errorf("WebSubApiSubscriptionTopic(%q, %q) = %q, want %q",
				tt.apiName, tt.version, got, tt.expected)
		}
	}
}

func TestNormalizeTopicSegment(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"issues", "issues"},
		{"/api/er3", "_2f_api_2f_er3"},
		{"pull_requests", "pull__requests"},
		{"v1/test", "v1_2f_test"},
		{"topic#42", "topic_23_42"},
		{" topic ", "_20_topic_20_"},
	}

	for _, tt := range tests {
		if got := NormalizeTopicSegment(tt.input); got != tt.expected {
			t.Errorf("NormalizeTopicSegment(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestWebSubApiBasePath(t *testing.T) {
	tests := []struct {
		name    string
		context string
		version string
		want    string
	}{
		{
			name:    "base context appends version",
			context: "/repos",
			version: "v1",
			want:    "/repos/v1",
		},
		{
			name:    "template context resolves version once",
			context: "/repos/$version",
			version: "v1",
			want:    "/repos/v1",
		},
		{
			name:    "resolved context does not duplicate version",
			context: "/repos/v1",
			version: "v1",
			want:    "/repos/v1",
		},
		{
			name:    "template with version in middle is preserved",
			context: "/$version/repos",
			version: "v1",
			want:    "/v1/repos",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WebSubApiBasePath(tt.context, tt.version)
			if got != tt.want {
				t.Fatalf("WebSubApiBasePath(%q, %q) = %q, want %q", tt.context, tt.version, got, tt.want)
			}
		})
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
