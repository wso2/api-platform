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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	httpmiddleware "github.com/wso2/api-platform/httpkit/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// One request can send several events to the same gateway (a deploy and its API
// key backfill). The event store drops an event whose ID it has already seen for
// that gateway, and gateways use the payload correlationId as their own event ID,
// so each event needs its own ID — never the request's correlation ID.
func TestBroadcastGivesEachEventItsOwnID(t *testing.T) {
	const requestID = "req-correlation-1"
	var ctx context.Context
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(httpmiddleware.CorrelationIDHeader, requestID)
	httpmiddleware.CorrelationIDMiddleware(newTestLogger())(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
	})).ServeHTTP(httptest.NewRecorder(), req)

	hub := &capturingEventHub{}
	svc := NewGatewayEventsService(hub, nil, newTestLogger())
	for _, name := range []string{"key-1", "key-2"} {
		if err := svc.BroadcastAPIKeyRevokedEvent(ctx, "gw-1", "user-1", &model.APIKeyRevokedEvent{ApiId: "api-1", KeyName: name}); err != nil {
			t.Fatalf("broadcast %s: %v", name, err)
		}
	}

	if len(hub.published) != 2 {
		t.Fatalf("published %d events, want 2", len(hub.published))
	}
	seen := map[string]bool{}
	for _, e := range hub.published {
		var payload struct {
			CorrelationID string `json:"correlationId"`
		}
		if err := json.Unmarshal([]byte(e.EventData), &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if e.EventID == requestID || payload.CorrelationID == requestID {
			t.Fatalf("event reuses the request correlation ID: eventID=%q correlationId=%q", e.EventID, payload.CorrelationID)
		}
		if payload.CorrelationID != e.EventID {
			t.Fatalf("payload correlationId %q differs from event ID %q", payload.CorrelationID, e.EventID)
		}
		if seen[e.EventID] {
			t.Fatalf("event ID %q sent twice", e.EventID)
		}
		seen[e.EventID] = true
	}
}
