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

package platformapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"

	controlplane "github.com/wso2/api-platform/tests/framework/core/catalog/platformapi"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

const (
	// portalEventsPath is platform-api's receiver for API Portal events.
	portalEventsPath = "/api/internal/v0.9/webhook/events"
	// portalEventSignatureHeader carries "t=<unix>,v1=<hex HMAC-SHA256 of "<t>.<body>">".
	portalEventSignatureHeader = "X-Api-Portal-Signature"
	// portalEventOrganization is the organization handle of platform-api's file mode.
	portalEventOrganization = "default"
)

// platformMeterKind is removed after the plans that price a meter (platformSubscriptionPlanKind),
// since platform-api refuses to delete a meter a plan still prices.
var platformMeterKind = cleanup.Kind{Name: "platform-api-meter", Order: 92}

// portalEventResources maps an API Portal event to the control-plane collection it creates or
// deletes a resource in, so the step records it for cleanup the way the request funnel does.
var portalEventResources = map[string]struct {
	collection string
	kind       cleanup.Kind
}{
	"meter":             {"/meters", platformMeterKind},
	"subscription_plan": {"/subscription-plans", platformSubscriptionPlanKind},
}

// RegisterPortalEvents binds the step that plays the API Portal: it sends platform-api a signed
// event, as the portal's webhook delivery does, and publishes platform-api's answer.
func RegisterPortalEvents(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^the API Portal sends the "([^"]*)" event(?: dated "([^"]*)")? to the control plane with data:$`,
		s.sendPortalEvent)
	sc.Step(`^the control plane (meter|subscription_plan) "([^"]*)" is registered for cleanup$`,
		s.registerPortalMadeResource)
}

// registerPortalMadeResource records a meter or plan the API Portal's own webhook delivery made
// in platform-api, which no step of this package saw created.
func (s *Steps) registerPortalMadeResource(ctx context.Context, aggregate, handle string) error {
	handle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	data, err := json.Marshal(map[string]string{"handle": handle})
	if err != nil {
		return err
	}
	return s.trackPortalEventResource(ctx, aggregate+".created", data)
}

// sendPortalEvent wraps data in the event envelope, signs it with the block's webhook secret
// and posts it. dated sets occurred_at, which platform-api compares to tell a stale event from
// a newer one; it is now when omitted.
func (s *Steps) sendPortalEvent(ctx context.Context, eventType, dated string, data *godog.DocString) error {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return fmt.Errorf("an API Portal event needs a type")
	}
	occurredAt := time.Now().UTC().Format(time.RFC3339Nano)
	if strings.TrimSpace(dated) != "" {
		expanded, err := stepscommon.Expand(ctx, dated)
		if err != nil {
			return err
		}
		if _, err := time.Parse(time.RFC3339, expanded); err != nil {
			return fmt.Errorf("event date %q must be RFC3339: %w", expanded, err)
		}
		occurredAt = expanded
	}
	content, err := stepscommon.Expand(ctx, data.Content)
	if err != nil {
		return err
	}
	if !json.Valid([]byte(content)) {
		return fmt.Errorf("the data of the %q event is not valid JSON", eventType)
	}
	eventID, err := randomEventID()
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{
		"event_id":    eventID,
		"event_type":  eventType,
		"occurred_at": occurredAt,
		"org":         map[string]string{"ref_id": portalEventOrganization},
		"data":        json.RawMessage(content),
	})
	if err != nil {
		return err
	}
	base, err := s.baseURL()
	if err != nil {
		return err
	}
	resp, err := s.funnel.Send(ctx, httpx.Request{
		Method: http.MethodPost,
		URL:    base + portalEventsPath,
		Headers: map[string]string{
			"Content-Type":             "application/json",
			portalEventSignatureHeader: signPortalEvent(controlplane.WebhookSecret(), body, time.Now()),
		},
		Body: body,
	})
	if err != nil {
		return fmt.Errorf("sending the %q event: %w", eventType, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	return s.trackPortalEventResource(ctx, eventType, []byte(content))
}

// trackPortalEventResource registers what an accepted created or updated event made, and
// deregisters what a deleted event removed. An update registers too: platform-api upserts, so
// an update may be what created the resource.
func (s *Steps) trackPortalEventResource(ctx context.Context, eventType string, data []byte) error {
	aggregate, action, ok := strings.Cut(eventType, ".")
	if !ok {
		return nil
	}
	resource, ok := portalEventResources[aggregate]
	if !ok {
		return nil
	}
	var subject struct {
		Handle string `json:"handle"`
	}
	if err := json.Unmarshal(data, &subject); err != nil || strings.TrimSpace(subject.Handle) == "" {
		return fmt.Errorf("the %q event names no handle to record for cleanup", eventType)
	}
	handle := strings.TrimSpace(subject.Handle)
	switch action {
	case "created", "updated":
		return s.registerTracked(ctx, resource.kind, handle, func(ctx context.Context) error {
			return s.deletePlatformResource(ctx, resource.kind, resource.collection, handle)
		}, func(reg *cleanup.Registry) error {
			return s.installPlatformDeleter(reg, resource.kind, resource.collection)
		})
	case "deleted":
		reg, err := cleanup.Of(ctx)
		if err != nil {
			return err
		}
		reg.Deregister(resource.kind, handle)
	}
	return nil
}

// signPortalEvent returns the signature header value platform-api's receiver verifies.
func signPortalEvent(secret string, body []byte, at time.Time) string {
	timestamp := strconv.FormatInt(at.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	return "t=" + timestamp + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

func randomEventID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generating an event id: %w", err)
	}
	return "evt-" + hex.EncodeToString(raw), nil
}
