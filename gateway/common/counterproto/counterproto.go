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

// Package counterproto is the wire contract of the shared counter service between a
// policy engine and its gateway-controller (sdk/core/counters).
//
// It lives in gateway/common because both sides must agree on it and neither can
// import the other. The messages travel over a gRPC bidirectional stream on the
// controller's policy server, encoded as JSON by Codec, so the contract needs no
// generated code.
//
// Every second the engine sends a SyncRequest: the amounts added since its last
// acknowledged request, under a ReportID, and the keys it reads. The controller applies
// the adds once per ReportID (a resent request is not applied again) and answers with
// a SyncResponse carrying the totals of those keys, which include every gateway's adds.
package counterproto

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	// ServiceName is the gRPC service name.
	ServiceName = "wso2.apip.counters.v1.Counters"
	// SyncMethod is the bidirectional stream method.
	SyncMethod = "Sync"
	// FullSyncMethod is the method path a client opens.
	FullSyncMethod = "/" + ServiceName + "/" + SyncMethod
	// CodecName is the gRPC content subtype of Codec.
	CodecName = "counterproto-json"
)

// Key names one counter: sdk/core/counters.Key on the wire.
type Key struct {
	Namespace string `json:"namespace"`
	ID        string `json:"id"`
	// Window is the start of the counter's period, Unix seconds; 0 for none.
	Window int64 `json:"window,omitempty"`
}

// WindowTime returns the window as a time.
func (k Key) WindowTime() time.Time {
	if k.Window == 0 {
		return time.Time{}
	}
	return time.Unix(k.Window, 0).UTC()
}

// KeyFor builds a wire key.
func KeyFor(namespace, id string, window time.Time) Key {
	k := Key{Namespace: namespace, ID: id}
	if !window.IsZero() {
		k.Window = window.Unix()
	}
	return k
}

// Add is an amount added to one counter.
type Add struct {
	Key   Key     `json:"key"`
	Delta float64 `json:"delta"`
}

// Total is the shared total of one counter.
type Total struct {
	Key   Key     `json:"key"`
	Total float64 `json:"total"`
}

// SyncRequest goes from the policy engine to its controller.
type SyncRequest struct {
	// Source identifies the engine for the lifetime of its process.
	Source string `json:"source"`
	// ReportID identifies Adds; a request with a ReportID already applied is not
	// applied again. Empty when Adds is empty.
	ReportID string `json:"reportId,omitempty"`
	Adds     []Add  `json:"adds,omitempty"`
	// Keys are the counters the engine reads; their totals come back.
	Keys []Key `json:"keys,omitempty"`
}

// SyncResponse goes from the controller to the policy engine.
type SyncResponse struct {
	// ReportID acknowledges the request's adds: they are in Totals.
	ReportID string  `json:"reportId,omitempty"`
	Totals   []Total `json:"totals,omitempty"`
}

// Codec encodes the messages as JSON. It satisfies gRPC's encoding.Codec.
type Codec struct{}

// Marshal encodes v.
func (Codec) Marshal(v any) ([]byte, error) {
	switch v.(type) {
	case *SyncRequest, *SyncResponse:
		return json.Marshal(v)
	default:
		return nil, fmt.Errorf("counterproto: cannot encode %T", v)
	}
}

// Unmarshal decodes data into v.
func (Codec) Unmarshal(data []byte, v any) error {
	switch v.(type) {
	case *SyncRequest, *SyncResponse:
		return json.Unmarshal(data, v)
	default:
		return fmt.Errorf("counterproto: cannot decode into %T", v)
	}
}

// Name returns CodecName.
func (Codec) Name() string { return CodecName }
