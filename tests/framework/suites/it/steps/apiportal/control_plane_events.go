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

package apiportal

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
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
	platformapisteps "github.com/wso2/api-platform/tests/framework/suites/it/steps/platformapi"
)

const (
	// controlPlaneEventsPath is platform-api's receiver for API Portal events.
	controlPlaneEventsPath = "/api/internal/v0.9/webhook/events"
	// controlPlaneEventSignatureHeader carries "t=<unix>,v1=<hex HMAC-SHA256 of "<t>.<body>">".
	controlPlaneEventSignatureHeader = "X-Api-Portal-Signature"
	// controlPlaneEventOrganization is the organization handle of platform-api's file mode.
	controlPlaneEventOrganization = "default"
	// controlPlaneEventStaleAge is past platform-api's default five-minute signature tolerance.
	controlPlaneEventStaleAge = 10 * time.Minute
	// lastControlPlaneEventKey holds the scenario's last event, for a redelivery of it.
	lastControlPlaneEventKey = "apiportal.lastControlPlaneEvent"
)

// The ways an event can reach platform-api's receiver, as the step text names them.
const (
	controlPlaneEventSigned      = "signed"
	controlPlaneEventUnsigned    = "unsigned"
	controlPlaneEventOtherSecret = "signed with another secret"
	controlPlaneEventStale       = "with a stale signature"
)

// sentControlPlaneEvent is an event as it was posted.
type sentControlPlaneEvent struct {
	eventType string
	body      []byte
}

// registerControlPlaneEventSteps binds the steps that test platform-api's receiver for API Portal
// events: they sign an event correctly or alter it to test the receiver's checks, and publish
// platform-api's answer. Events these steps send are not recorded for cleanup; a scenario whose
// event can create a resource registers it with the control-plane cleanup step.
func (s *Steps) registerControlPlaneEventSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the API Portal sends the "([^"]*)" event to the control plane (signed|unsigned|signed with another secret|with a stale signature) with data:$`,
		s.sendControlPlaneEventVariant)
	sc.Step(`^the API Portal sends the "([^"]*)" event for organization "([^"]*)" to the control plane with data:$`,
		s.sendControlPlaneEventForOrganization)
	sc.Step(`^the API Portal sends its last event to the control plane again$`, s.resendLastControlPlaneEvent)
}

// sendControlPlaneEventVariant posts an event signed as the portal's webhook delivery does, or the
// way a misbehaving or hostile caller would: without a signature, signed with another secret, or
// signed outside the receiver's replay window.
func (s *Steps) sendControlPlaneEventVariant(ctx context.Context, eventType, variant string, data *godog.DocString) error {
	return s.deliverControlPlaneEvent(ctx, eventType, controlPlaneEventOrganization, variant, data)
}

// sendControlPlaneEventForOrganization posts a signed event that names another organization.
func (s *Steps) sendControlPlaneEventForOrganization(ctx context.Context, eventType, organization string, data *godog.DocString) error {
	organization, err := stepscommon.Expand(ctx, organization)
	if err != nil {
		return err
	}
	if strings.TrimSpace(organization) == "" {
		return fmt.Errorf("an API Portal event needs an organization")
	}
	return s.deliverControlPlaneEvent(ctx, eventType, strings.TrimSpace(organization), controlPlaneEventSigned, data)
}

// resendLastControlPlaneEvent posts the scenario's previous event again, byte for byte and with the
// same event_id, under a fresh signature, as a redelivery of the same event would arrive.
func (s *Steps) resendLastControlPlaneEvent(ctx context.Context) error {
	value, err := tcontext.Resolve(ctx, lastControlPlaneEventKey)
	if err != nil {
		return fmt.Errorf("no API Portal event was sent in this scenario to send again: %w", err)
	}
	last, ok := value.(sentControlPlaneEvent)
	if !ok || len(last.body) == 0 {
		return fmt.Errorf("no API Portal event was sent in this scenario to send again")
	}
	header, err := controlPlaneEventSignature(controlPlaneEventSigned, controlplane.WebhookSecret(), last.body, time.Now())
	if err != nil {
		return err
	}
	return s.postControlPlaneEvent(ctx, last, header)
}

// deliverControlPlaneEvent builds the envelope, signs it as the variant says, posts it and records
// it as the scenario's last event.
func (s *Steps) deliverControlPlaneEvent(ctx context.Context, eventType, organization, variant string, data *godog.DocString) error {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return fmt.Errorf("an API Portal event needs a type")
	}
	if data == nil {
		return fmt.Errorf("the %q event needs data", eventType)
	}
	content, err := stepscommon.Expand(ctx, data.Content)
	if err != nil {
		return err
	}
	if !json.Valid([]byte(content)) {
		return fmt.Errorf("the data of the %q event is not valid JSON", eventType)
	}
	eventID, err := randomControlPlaneEventID()
	if err != nil {
		return err
	}
	occurredAt := time.Now().UTC().Format(time.RFC3339Nano)
	body, err := controlPlaneEventBody(eventID, eventType, occurredAt, organization, []byte(content))
	if err != nil {
		return err
	}
	header, err := controlPlaneEventSignature(variant, controlplane.WebhookSecret(), body, time.Now())
	if err != nil {
		return err
	}
	event := sentControlPlaneEvent{eventType: eventType, body: body}
	if err := tcontext.Set(ctx, lastControlPlaneEventKey, event); err != nil {
		return err
	}
	return s.postControlPlaneEvent(ctx, event, header)
}

// postControlPlaneEvent posts one event body and publishes platform-api's answer. It sends the
// event once, since a retry would be a redelivery. An empty signature sends the event unsigned.
func (s *Steps) postControlPlaneEvent(ctx context.Context, event sentControlPlaneEvent, signature string) error {
	base, err := platformapisteps.BaseURL(s.topo)
	if err != nil {
		return err
	}
	headers := map[string]string{"Content-Type": "application/json"}
	if signature != "" {
		headers[controlPlaneEventSignatureHeader] = signature
	}
	if _, err := httpx.NewFunnel(s.controlPlane, 0, 0).Send(ctx, httpx.Request{
		Method:  http.MethodPost,
		URL:     base + controlPlaneEventsPath,
		Headers: headers,
		Body:    event.body,
	}); err != nil {
		return fmt.Errorf("sending the %q event: %w", event.eventType, err)
	}
	return nil
}

// controlPlaneEventBody returns the event envelope.
func controlPlaneEventBody(eventID, eventType, occurredAt, organization string, data []byte) ([]byte, error) {
	return json.Marshal(map[string]any{
		"event_id":    eventID,
		"event_type":  eventType,
		"occurred_at": occurredAt,
		"org":         map[string]string{"ref_id": organization},
		"data":        json.RawMessage(data),
	})
}

// controlPlaneEventSignature returns the signature header for a variant; an unsigned event has none.
func controlPlaneEventSignature(variant, secret string, body []byte, now time.Time) (string, error) {
	switch variant {
	case controlPlaneEventSigned:
		return signControlPlaneEvent(secret, body, now), nil
	case controlPlaneEventUnsigned:
		return "", nil
	case controlPlaneEventOtherSecret:
		other := make([]byte, 32)
		if _, err := rand.Read(other); err != nil {
			return "", fmt.Errorf("generating another secret: %w", err)
		}
		return signControlPlaneEvent(hex.EncodeToString(other), body, now), nil
	case controlPlaneEventStale:
		return signControlPlaneEvent(secret, body, now.Add(-controlPlaneEventStaleAge)), nil
	default:
		return "", fmt.Errorf("unknown API Portal event variant %q", variant)
	}
}

// signControlPlaneEvent returns the signature header value platform-api's receiver verifies.
func signControlPlaneEvent(secret string, body []byte, at time.Time) string {
	timestamp := strconv.FormatInt(at.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	return "t=" + timestamp + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

// randomControlPlaneEventID returns a fresh event id, so that a replay check is the only source of
// a repeated id.
func randomControlPlaneEventID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generating an event id: %w", err)
	}
	return "evt-" + hex.EncodeToString(raw), nil
}
