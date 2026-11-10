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

package stripe

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	maxDeliveryResponseBytes = 64 << 10
	// staleSignatureAge is older than the five minutes a receiver accepts.
	staleSignatureAge = time.Hour
)

// Signature kinds a resent event can carry.
const (
	signatureValid   = "valid"
	signatureInvalid = "invalid"
	signatureStale   = "stale"
)

// eventSpec is an event to create: its type and its object as Stripe renders it.
type eventSpec struct {
	kind   string
	object map[string]any
}

type event struct {
	ID         string
	Type       string
	ObjectID   string
	Created    int64
	Object     json.RawMessage
	Deliveries []delivery
}

type delivery struct {
	URL       string `json:"url"`
	Signature string `json:"signature"`
	Status    int    `json:"status"`
	Error     string `json:"error,omitempty"`
}

// send is one event to deliver to one endpoint.
type send struct {
	event      *event
	url        string
	secret     string
	apiVersion string
	signature  string
}

// body renders the event as an endpoint of the API version receives it.
func (e *event) body(apiVersion string) []byte {
	body, _ := json.Marshal(map[string]any{
		"id": e.ID, "object": "event", "type": e.Type, "created": e.Created, "api_version": nullable(apiVersion),
		"livemode": false, "pending_webhooks": 1, "data": map[string]any{"object": e.Object},
	})
	return body
}

func (e *event) json() map[string]any {
	return map[string]any{
		"id": e.ID, "type": e.Type, "objectId": e.ObjectID, "created": e.Created, "deliveries": e.Deliveries,
	}
}

// newEvents creates events and returns their deliveries to the endpoints that receive them. The
// caller holds the account lock and delivers after releasing it.
func (a *account) newEvents(now time.Time, specs ...eventSpec) []send {
	var sends []send
	for _, spec := range specs {
		object, _ := json.Marshal(spec.object)
		objectID, _ := spec.object["id"].(string)
		e := &event{ID: newID("evt"), Type: spec.kind, ObjectID: objectID, Created: now.Unix(), Object: object}
		a.events = append(a.events, e)
		if len(a.events) > maxRetainedEvents {
			a.events = a.events[len(a.events)-maxRetainedEvents:]
		}
		sends = append(sends, a.sendsOf(e, signatureValid)...)
	}
	return sends
}

// sendsOf returns an event's deliveries to the endpoints that receive its type now.
func (a *account) sendsOf(e *event, signature string) []send {
	var sends []send
	for _, target := range a.endpoints {
		if target.receives(e.Type) {
			sends = append(sends, send{
				event: e, url: target.URL, secret: target.Secret, apiVersion: target.APIVersion, signature: signature,
			})
		}
	}
	return sends
}

// deliver sends each event to its endpoint in order and records the answers.
func (s *Service) deliver(acct *account, sends []send) []delivery {
	results := make([]delivery, 0, len(sends))
	for _, item := range sends {
		result := s.post(item)
		acct.mu.Lock()
		item.event.Deliveries = append(item.event.Deliveries, result)
		acct.mu.Unlock()
		results = append(results, result)
	}
	return results
}

// sendLater delivers in the background, so an endpoint that calls back into the account while it
// handles an event does not wait on the request that created the event.
func (s *Service) sendLater(acct *account, sends []send) {
	if len(sends) == 0 {
		return
	}
	s.sending.Add(1)
	go func() {
		defer s.sending.Done()
		s.deliver(acct, sends)
	}()
}

func (s *Service) post(item send) delivery {
	result := delivery{URL: item.url, Signature: item.signature}
	body := item.event.body(item.apiVersion)
	timestamp := s.now()
	secret := item.secret
	switch item.signature {
	case signatureInvalid:
		secret = "whsec_invalid"
	case signatureStale:
		timestamp = timestamp.Add(-staleSignatureAge)
	}
	req, err := http.NewRequest(http.MethodPost, item.url, bytes.NewReader(body))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "Stripe/1.0 (+https://stripe.com/docs/webhooks)")
	req.Header.Set("Stripe-Signature", signatureHeader(secret, timestamp.Unix(), body))
	resp, err := s.client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDeliveryResponseBytes))
	result.Status = resp.StatusCode
	return result
}

// signatureHeader is Stripe's Stripe-Signature header: the time, and an HMAC-SHA256 of
// "<time>.<body>" with the endpoint's signing secret.
func signatureHeader(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "."))
	mac.Write(body)
	return "t=" + strconv.FormatInt(timestamp, 10) + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}
