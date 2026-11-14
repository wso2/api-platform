/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package service

import (
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/policysecret"
)

// awsManifestStore is a store backed by a manifest declaring the AWS secret
// write-only, so the traversal resolves through the real index.
func awsManifestStore() *policysecret.Store {
	manifest := []byte(`[{"name": "aws-authentication", "version": "v0",
	  "writeOnlyParams": [["awsSecretAccessKey"]]}]`)
	return policysecret.NewStore(func(string) ([][]byte, error) {
		return [][]byte{manifest}, nil
	})
}

func apiPolicies(value string) *[]api.Policy {
	params := map[string]interface{}{"awsSecretAccessKey": value}
	return &[]api.Policy{{Name: "aws-authentication", Version: "v0", Params: &params}}
}

func modelPolicies(value string) []model.Policy {
	params := map[string]interface{}{"awsSecretAccessKey": value}
	return []model.Policy{{Name: "aws-authentication", Version: "v0", Params: &params}}
}

// A WebSub artifact attaches policies for all channels and again per named
// channel. Both are covered, and every event type within them.
func TestBlankWebSubPolicySecretsCoversAllChannelsAndEachChannel(t *testing.T) {
	resp := &api.WebSubAPI{
		AllChannels: &api.WebSubAllChannelPolicies{
			OnSubscription:    &api.WebSubEventPolicies{Policies: apiPolicies("all-sub-secret")},
			OnMessageDelivery: &api.WebSubEventPolicies{Policies: apiPolicies("all-delivery-secret")},
		},
		Channels: map[string]api.WebSubChannel{
			"orders": {
				OnMessageReceived: &api.WebSubEventPolicies{Policies: apiPolicies("orders-secret")},
			},
		},
	}

	svc := &WebSubAPIService{}
	svc.SetPolicySecrets(awsManifestStore())
	if err := svc.blankWebSubPolicySecrets("org-a", resp); err != nil {
		t.Fatalf("blankWebSubPolicySecrets returned %v", err)
	}

	secretOf := func(in *api.WebSubEventPolicies) interface{} {
		return (*(*in.Policies)[0].Params)["awsSecretAccessKey"]
	}
	if got := secretOf(resp.AllChannels.OnSubscription); got != "" {
		t.Errorf("allChannels.onSubscription = %v, want the empty string", got)
	}
	if got := secretOf(resp.AllChannels.OnMessageDelivery); got != "" {
		t.Errorf("allChannels.onMessageDelivery = %v, want the empty string", got)
	}
	if got := secretOf(resp.Channels["orders"].OnMessageReceived); got != "" {
		t.Errorf("channels.orders.onMessageReceived = %v, want the empty string", got)
	}
}

// Channels are matched by name, so a credential cannot move between them when
// one is added or removed.
func TestPreserveWebSubPolicySecretsMatchesChannelsByName(t *testing.T) {
	existing := &model.WebSubAPIConfiguration{
		Channels: map[string]model.WebSubChannel{
			"orders":    {OnMessageReceived: &model.WebSubEventPolicies{Policies: modelPolicies("orders-secret")}},
			"shipments": {OnMessageReceived: &model.WebSubEventPolicies{Policies: modelPolicies("shipments-secret")}},
		},
	}
	updated := &model.WebSubAPIConfiguration{
		Channels: map[string]model.WebSubChannel{
			"shipments": {OnMessageReceived: &model.WebSubEventPolicies{Policies: modelPolicies("")}},
			"returns":   {OnMessageReceived: &model.WebSubEventPolicies{Policies: modelPolicies("")}},
		},
	}

	svc := &WebSubAPIService{}
	svc.SetPolicySecrets(awsManifestStore())
	if err := svc.preserveWebSubPolicySecrets("org-a", existing, updated); err != nil {
		t.Fatalf("preserveWebSubPolicySecrets returned %v", err)
	}

	secretOf := func(name string) interface{} {
		return (*updated.Channels[name].OnMessageReceived.Policies[0].Params)["awsSecretAccessKey"]
	}
	if got := secretOf("shipments"); got != "shipments-secret" {
		t.Errorf("shipments = %v, want the value stored against it", got)
	}
	if got := secretOf("returns"); got != "" {
		t.Errorf("returns = %v, want nothing carried into a channel that did not exist", got)
	}
}

func TestBlankWebBrokerPolicySecretsCoversAllChannelsAndEachChannel(t *testing.T) {
	resp := &api.WebBrokerAPI{
		AllChannels: &api.WebBrokerAllChannelPolicies{
			OnConnectionInit: &api.WebBrokerEventPolicies{Policies: apiPolicies("connect-secret")},
		},
		Channels: map[string]api.WebBrokerChannel{
			"events": {OnProduce: &api.WebBrokerEventPolicies{Policies: apiPolicies("produce-secret")}},
		},
	}

	svc := &WebBrokerAPIService{}
	svc.SetPolicySecrets(awsManifestStore())
	if err := svc.blankWebBrokerPolicySecrets("org-a", resp); err != nil {
		t.Fatalf("blankWebBrokerPolicySecrets returned %v", err)
	}

	if got := (*(*resp.AllChannels.OnConnectionInit.Policies)[0].Params)["awsSecretAccessKey"]; got != "" {
		t.Errorf("allChannels.onConnectionInit = %v, want the empty string", got)
	}
	if got := (*(*resp.Channels["events"].OnProduce.Policies)[0].Params)["awsSecretAccessKey"]; got != "" {
		t.Errorf("channels.events.onProduce = %v, want the empty string", got)
	}
}
