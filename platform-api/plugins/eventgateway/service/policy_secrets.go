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

// Write-only policy parameters for the event gateway's own artifact kinds.
//
// The rules live in internal/policysecret and the shared wire-level helpers in
// internal/service; this file is only the traversal, which differs here because
// a WebSub or WebBroker artifact attaches policies per event type rather than
// per operation. Both kinds carry them in two places — once for all channels,
// and once per named channel — and a credential in either is still a credential.

package service

import (
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/policysecret"
	coreservice "github.com/wso2/api-platform/platform-api/internal/service"
)

// blankWebSubPolicySecrets empties the declared write-only parameters in a
// mapped WebSub response.
func (s *WebSubAPIService) blankWebSubPolicySecrets(orgUUID string, resp *api.WebSubAPI) error {
	if resp == nil {
		return nil
	}
	resolver, err := coreservice.PolicySecretResolver(s.policySecrets, orgUUID)
	if err != nil {
		return err
	}
	blankWebSubEventPolicies(resolver, resp.AllChannels)
	for name, channel := range resp.Channels {
		blankWebSubChannelPolicies(resolver, &channel)
		resp.Channels[name] = channel
	}
	return nil
}

// blankWebSubEventPolicies covers the all-channel block.
func blankWebSubEventPolicies(resolver policysecret.Resolver, in *api.WebSubAllChannelPolicies) {
	if in == nil {
		return
	}
	for _, event := range []*api.WebSubEventPolicies{
		in.OnSubscription, in.OnUnsubscription, in.OnMessageReceived, in.OnMessageDelivery,
	} {
		if event != nil {
			coreservice.BlankPolicySecrets(resolver, event.Policies)
		}
	}
}

// blankWebSubChannelPolicies covers one named channel.
func blankWebSubChannelPolicies(resolver policysecret.Resolver, in *api.WebSubChannel) {
	for _, event := range []*api.WebSubEventPolicies{
		in.OnSubscription, in.OnUnsubscription, in.OnMessageReceived, in.OnMessageDelivery,
	} {
		if event != nil {
			coreservice.BlankPolicySecrets(resolver, event.Policies)
		}
	}
}

// preserveWebSubPolicySecrets carries stored WebSub policy credentials onto an
// update that echoed them back empty.
//
// Channels are matched by name, which is how they are keyed in storage and on
// the wire, so adding or removing one does not move a credential between them.
func (s *WebSubAPIService) preserveWebSubPolicySecrets(orgUUID string,
	existing, updated *model.WebSubAPIConfiguration) error {
	if existing == nil || updated == nil {
		return nil
	}
	resolver, err := coreservice.PolicySecretResolver(s.policySecrets, orgUUID)
	if err != nil {
		return err
	}
	preserveWebSubAllChannel(resolver, existing.AllChannels, updated.AllChannels)
	for name, updatedChannel := range updated.Channels {
		storedChannel, ok := existing.Channels[name]
		if !ok {
			continue
		}
		preserveWebSubEvent(resolver, storedChannel.OnSubscription, updatedChannel.OnSubscription)
		preserveWebSubEvent(resolver, storedChannel.OnUnsubscription, updatedChannel.OnUnsubscription)
		preserveWebSubEvent(resolver, storedChannel.OnMessageReceived, updatedChannel.OnMessageReceived)
		preserveWebSubEvent(resolver, storedChannel.OnMessageDelivery, updatedChannel.OnMessageDelivery)
	}
	return nil
}

// preserveWebSubAllChannel is the all-channel counterpart.
func preserveWebSubAllChannel(resolver policysecret.Resolver, stored, updated *model.WebSubAllChannelPolicies) {
	if stored == nil || updated == nil {
		return
	}
	preserveWebSubEvent(resolver, stored.OnSubscription, updated.OnSubscription)
	preserveWebSubEvent(resolver, stored.OnUnsubscription, updated.OnUnsubscription)
	preserveWebSubEvent(resolver, stored.OnMessageReceived, updated.OnMessageReceived)
	preserveWebSubEvent(resolver, stored.OnMessageDelivery, updated.OnMessageDelivery)
}

// preserveWebSubEvent carries one event type's credentials forward.
func preserveWebSubEvent(resolver policysecret.Resolver, stored, updated *model.WebSubEventPolicies) {
	if stored == nil || updated == nil {
		return
	}
	coreservice.PreservePolicySecrets(resolver, stored.Policies, updated.Policies)
}

// blankWebBrokerPolicySecrets empties the declared write-only parameters in a
// mapped WebBroker response.
func (s *WebBrokerAPIService) blankWebBrokerPolicySecrets(orgUUID string, resp *api.WebBrokerAPI) error {
	if resp == nil {
		return nil
	}
	resolver, err := coreservice.PolicySecretResolver(s.policySecrets, orgUUID)
	if err != nil {
		return err
	}
	if resp.AllChannels != nil {
		for _, event := range []*api.WebBrokerEventPolicies{
			resp.AllChannels.OnConnectionInit, resp.AllChannels.OnProduce, resp.AllChannels.OnConsume,
		} {
			if event != nil {
				coreservice.BlankPolicySecrets(resolver, event.Policies)
			}
		}
	}
	for name, channel := range resp.Channels {
		for _, event := range []*api.WebBrokerEventPolicies{
			channel.OnConnectionInit, channel.OnProduce, channel.OnConsume,
		} {
			if event != nil {
				coreservice.BlankPolicySecrets(resolver, event.Policies)
			}
		}
		resp.Channels[name] = channel
	}
	return nil
}

// preserveWebBrokerPolicySecrets carries stored WebBroker policy credentials
// onto an update that echoed them back empty. Channels are matched by name.
func (s *WebBrokerAPIService) preserveWebBrokerPolicySecrets(orgUUID string,
	existing, updated *model.WebBrokerAPIConfiguration) error {
	if existing == nil || updated == nil {
		return nil
	}
	resolver, err := coreservice.PolicySecretResolver(s.policySecrets, orgUUID)
	if err != nil {
		return err
	}
	if existing.AllChannels != nil && updated.AllChannels != nil {
		preserveWebBrokerEvent(resolver, existing.AllChannels.OnConnectionInit, updated.AllChannels.OnConnectionInit)
		preserveWebBrokerEvent(resolver, existing.AllChannels.OnProduce, updated.AllChannels.OnProduce)
		preserveWebBrokerEvent(resolver, existing.AllChannels.OnConsume, updated.AllChannels.OnConsume)
	}
	for name, updatedChannel := range updated.Channels {
		storedChannel, ok := existing.Channels[name]
		if !ok {
			continue
		}
		preserveWebBrokerEvent(resolver, storedChannel.OnConnectionInit, updatedChannel.OnConnectionInit)
		preserveWebBrokerEvent(resolver, storedChannel.OnProduce, updatedChannel.OnProduce)
		preserveWebBrokerEvent(resolver, storedChannel.OnConsume, updatedChannel.OnConsume)
	}
	return nil
}

// preserveWebBrokerEvent carries one event type's credentials forward.
func preserveWebBrokerEvent(resolver policysecret.Resolver, stored, updated *model.WebBrokerEventPolicies) {
	if stored == nil || updated == nil {
		return
	}
	coreservice.PreservePolicySecrets(resolver, stored.Policies, updated.Policies)
}
