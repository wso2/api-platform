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

package xdsclient

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/binding"
)

func TestHandlerHandleResources_SkipsUnchangedBindings(t *testing.T) {
	manager := &recordingBindingManager{}
	handler := NewHandler(manager, KafkaConfig{Brokers: []string{"kafka:29092"}})

	resource := EventChannelResource{
		UUID:    "api-1",
		Name:    "githubser",
		Kind:    "WebSubApi",
		Context: "/proj1/githubser",
		Version: "v1.0",
		Channels: []ChannelEntry{
			{Name: "issues"},
		},
		Receiver: ReceiverEntry{Type: "websub"},
		Policies: PoliciesEntry{
			Subscribe: []PolicyEntry{{Name: "api-key-auth", Version: "v1.0.1"}},
			Inbound:   []PolicyEntry{},
			Outbound:  []PolicyEntry{},
		},
	}

	resources := []*discoveryv3.Resource{{Resource: mustBuildEventChannelAny(t, resource)}}
	if err := handler.HandleResources(context.Background(), resources, "1"); err != nil {
		t.Fatalf("first HandleResources returned error: %v", err)
	}
	if err := handler.HandleResources(context.Background(), resources, "2"); err != nil {
		t.Fatalf("second HandleResources returned error: %v", err)
	}

	if got := manager.addedNames(); len(got) != 1 || got[0] != "githubser" {
		t.Fatalf("expected one add for githubser, got %#v", got)
	}
	if got := manager.removedNames(); len(got) != 0 {
		t.Fatalf("expected no removals for unchanged snapshot, got %#v", got)
	}
}

func TestHandlerHandleResources_UpdatesOnlyChangedBinding(t *testing.T) {
	manager := &recordingBindingManager{}
	handler := NewHandler(manager, KafkaConfig{Brokers: []string{"kafka:29092"}})

	initialResources := []*discoveryv3.Resource{
		{Resource: mustBuildEventChannelAny(t, EventChannelResource{
			UUID:    "api-1",
			Name:    "githubser",
			Kind:    "WebSubApi",
			Context: "/proj1/githubser",
			Version: "v1.0",
			Channels: []ChannelEntry{
				{Name: "issues"},
			},
			Receiver: ReceiverEntry{Type: "websub"},
			Policies: PoliciesEntry{
				Subscribe: []PolicyEntry{{Name: "api-key-auth", Version: "v1.0.1"}},
			},
		})},
		{Resource: mustBuildEventChannelAny(t, EventChannelResource{
			UUID:    "api-2",
			Name:    "gitlabser",
			Kind:    "WebSubApi",
			Context: "/proj1/gitlabser",
			Version: "v1.0",
			Channels: []ChannelEntry{
				{Name: "merge-requests"},
			},
			Receiver: ReceiverEntry{Type: "websub"},
			Policies: PoliciesEntry{
				Subscribe: []PolicyEntry{{Name: "api-key-auth", Version: "v1.0.1"}},
			},
		})},
	}
	if err := handler.HandleResources(context.Background(), initialResources, "1"); err != nil {
		t.Fatalf("initial HandleResources returned error: %v", err)
	}

	updatedResources := []*discoveryv3.Resource{
		{Resource: mustBuildEventChannelAny(t, EventChannelResource{
			UUID:    "api-1",
			Name:    "githubser",
			Kind:    "WebSubApi",
			Context: "/proj1/githubser",
			Version: "v1.0",
			Channels: []ChannelEntry{
				{Name: "issues"},
			},
			Receiver: ReceiverEntry{Type: "websub"},
			Policies: PoliciesEntry{
				Subscribe: []PolicyEntry{{Name: "basic-auth", Version: "v1.0.1"}},
			},
		})},
		{Resource: mustBuildEventChannelAny(t, EventChannelResource{
			UUID:    "api-2",
			Name:    "gitlabser",
			Kind:    "WebSubApi",
			Context: "/proj1/gitlabser",
			Version: "v1.0",
			Channels: []ChannelEntry{
				{Name: "merge-requests"},
			},
			Receiver: ReceiverEntry{Type: "websub"},
			Policies: PoliciesEntry{
				Subscribe: []PolicyEntry{{Name: "api-key-auth", Version: "v1.0.1"}},
			},
		})},
	}
	if err := handler.HandleResources(context.Background(), updatedResources, "2"); err != nil {
		t.Fatalf("updated HandleResources returned error: %v", err)
	}

	if got := manager.removedNames(); len(got) != 1 || got[0] != "githubser" {
		t.Fatalf("expected only githubser to be removed on change, got %#v", got)
	}
	if got := manager.addedCount("githubser"); got != 2 {
		t.Fatalf("expected githubser to be added twice, got %d", got)
	}
	if got := manager.addedCount("gitlabser"); got != 1 {
		t.Fatalf("expected gitlabser to be added once, got %d", got)
	}
}

type recordingBindingManager struct {
	added   []string
	removed []string
}

func (m *recordingBindingManager) AddWebSubApiBinding(wsb binding.WebSubApiBinding) error {
	m.added = append(m.added, wsb.Name)
	return nil
}

func (m *recordingBindingManager) RemoveWebSubApiBinding(name string) error {
	m.removed = append(m.removed, name)
	return nil
}

func (m *recordingBindingManager) addedNames() []string {
	out := append([]string(nil), m.added...)
	return out
}

func (m *recordingBindingManager) addedCount(name string) int {
	count := 0
	for _, added := range m.added {
		if added == name {
			count++
		}
	}
	return count
}

func (m *recordingBindingManager) removedNames() []string {
	out := append([]string(nil), m.removed...)
	sort.Strings(out)
	return out
}

func mustBuildEventChannelAny(t *testing.T, resource EventChannelResource) *anypb.Any {
	t.Helper()

	jsonBytes, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("failed to marshal event channel resource: %v", err)
	}

	payload := &structpb.Struct{}
	if err := payload.UnmarshalJSON(jsonBytes); err != nil {
		t.Fatalf("failed to unmarshal event channel JSON into struct: %v", err)
	}

	structBytes, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal struct: %v", err)
	}

	innerAny := &anypb.Any{
		TypeUrl: "type.googleapis.com/google.protobuf.Struct",
		Value:   structBytes,
	}

	innerBytes, err := proto.Marshal(innerAny)
	if err != nil {
		t.Fatalf("failed to marshal inner Any: %v", err)
	}

	return &anypb.Any{
		TypeUrl: EventChannelConfigTypeURL,
		Value:   innerBytes,
	}
}
