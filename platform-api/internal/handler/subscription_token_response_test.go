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

// A subscription token belongs to one subscriber, so a read returns it to that subscriber
// and to nobody else. These tests pin that: the response builders never carry the token on
// their own, and the gate that adds it matches on the caller's identity rather than on
// their organization.

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

const (
	ownerIdentity = "owner-idp-subject"
	storedToken   = "0123456789abcdef"
)

// newTokenContractHandler builds a SubscriptionHandler whose collaborators are never
// reached: the fixture below leaves artifact, organization and actor identifiers empty,
// and every resolver short-circuits on an empty identifier.
func newTokenContractHandler() *SubscriptionHandler {
	return NewSubscriptionHandler(
		service.NewSubscriptionService(nil, nil, nil, nil, nil, nil, nil, nil),
		nil,
		service.NewIdentityService(nil),
		nil,
	)
}

// subscriptionWithToken is a stored subscription whose token is populated, which is the
// only case in which a response could carry it.
func subscriptionWithToken() *model.Subscription {
	return &model.Subscription{
		UUID:              "sub-1",
		SubscriberID:      ownerIdentity,
		SubscriptionToken: storedToken,
		Status:            model.SubscriptionStatusActive,
	}
}

// requestAs is a read request from the caller with the given identity, which is what
// subscriptions.subscriber_id is compared against. The identity is injected as the user
// id rather than as a sub claim because GetActorIdentityFromRequest falls back to it, and
// a sub claim cannot be set from outside the middleware package.
func requestAs(identity string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/v0.9/subscriptions/sub-1", nil)
	if identity == "" {
		return r
	}
	return middleware.WithUserID(r, identity)
}

func TestToSubscriptionResponse_OmitsSubscriptionToken(t *testing.T) {
	h := newTokenContractHandler()

	resp, err := h.toSubscriptionResponse(subscriptionWithToken(), "")
	if err != nil {
		t.Fatalf("toSubscriptionResponse: %v", err)
	}
	if v, present := resp["subscriptionToken"]; present {
		t.Errorf("builder carries subscriptionToken = %v, want the key to be absent", v)
	}
	if resp["id"] != "sub-1" {
		t.Errorf("id = %v, want sub-1", resp["id"])
	}
}

func TestToSubscriptionResponseWithMaps_OmitsSubscriptionToken(t *testing.T) {
	h := newTokenContractHandler()

	resp := h.toSubscriptionResponseWithMaps(subscriptionWithToken(), "", nil, nil, nil)
	if v, present := resp["subscriptionToken"]; present {
		t.Errorf("builder carries subscriptionToken = %v, want the key to be absent", v)
	}
	if resp["id"] != "sub-1" {
		t.Errorf("id = %v, want sub-1", resp["id"])
	}
}

func TestAddOwnerSubscriptionToken_GivesTheTokenToItsSubscriber(t *testing.T) {
	resp := map[string]any{}
	added := addOwnerSubscriptionToken(resp, requestAs(ownerIdentity), subscriptionWithToken())

	if !added {
		t.Error("addOwnerSubscriptionToken reported no disclosure to the subscription's own subscriber")
	}
	if resp["subscriptionToken"] != storedToken {
		t.Errorf("subscriptionToken = %v, want the subscriber's own token %q", resp["subscriptionToken"], storedToken)
	}
}

// Another user of the same organization is the case that matters here: the organization
// claim is satisfied, the subscriber is not, and the token must not be returned.
func TestAddOwnerSubscriptionToken_WithholdsTheTokenFromEveryoneElse(t *testing.T) {
	cases := []struct {
		name         string
		caller       string
		subscriberID string
	}{
		{"another user in the same organization", "other-idp-subject", ownerIdentity},
		{"an unauthenticated caller", "", ownerIdentity},
		{"a machine-credential subscription", subscriberSystemActor, subscriberSystemActor},
		{"a subscription whose user is gone", constants.DeletedUser, constants.DeletedUser},
		{"a subscription with no subscriber recorded", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sub := subscriptionWithToken()
			sub.SubscriberID = tc.subscriberID

			resp := map[string]any{}
			added := addOwnerSubscriptionToken(resp, requestAs(tc.caller), sub)

			if added {
				t.Error("addOwnerSubscriptionToken reported a disclosure, want none")
			}
			if v, present := resp["subscriptionToken"]; present {
				t.Errorf("response carries subscriptionToken = %v, want the key to be absent", v)
			}
		})
	}
}
