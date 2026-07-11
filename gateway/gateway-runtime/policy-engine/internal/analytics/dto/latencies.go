/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package dto

// Latencies represents latency attributes in an analytics event. All values are
// in milliseconds: this shape feeds Moesif, which treats them as milliseconds.
// Finer-grained gateway/backend timings for the traffic log live in the separate
// TrafficLogLatencies (microseconds) so the units Moesif depends on never change.
type Latencies struct {
	ResponseLatency          int64 `json:"responseLatency"`
	BackendLatency           int64 `json:"backendLatency"`
	RequestMediationLatency  int64 `json:"requestMediationLatency"`
	ResponseMediationLatency int64 `json:"responseMediationLatency"`
	Duration                 int64 `json:"duration"`
}

// TrafficLogLatencies holds gateway/backend timings for a traffic-log event, in
// microseconds. It is intentionally separate from Latencies (which is shaped for
// Moesif and expressed in milliseconds) so the traffic log can carry
// finer-grained timings without changing the units Moesif depends on. The
// timepoint labels (DS_RX_BEG, US_TX_BEG, …) refer to Envoy's request/response
// timeline as reported through the ALS CommonProperties.
type TrafficLogLatencies struct {
	// DurationUs is the total request duration: downstream request received →
	// downstream response sent (DS_RX_BEG → DS_TX_END).
	DurationUs int64 `json:"durationUs"`
	// RequestMediationLatencyUs is the gateway request overhead: downstream request
	// fully received → first byte sent upstream (DS_RX_END → US_TX_BEG).
	RequestMediationLatencyUs int64 `json:"requestMediationLatencyUs"`
	// ResponseMediationLatencyUs is the gateway response overhead: first upstream
	// response byte → first downstream response byte (US_RX_BEG → DS_TX_BEG).
	ResponseMediationLatencyUs int64 `json:"responseMediationLatencyUs"`
	// BackendLatencyUs is the backend TTFB: upstream request fully sent →
	// first upstream response byte (US_TX_END → US_RX_BEG).
	BackendLatencyUs int64 `json:"backendLatencyUs"`
}

// GetResponseLatency returns the response latency.
func (l *Latencies) GetResponseLatency() int64 {
	return l.ResponseLatency
}

// SetResponseLatency sets the response latency.
func (l *Latencies) SetResponseLatency(responseLatency int64) {
	l.ResponseLatency = responseLatency
}

// GetBackendLatency returns the backend latency.
func (l *Latencies) GetBackendLatency() int64 {
	return l.BackendLatency
}

// SetBackendLatency sets the backend latency.
func (l *Latencies) SetBackendLatency(backendLatency int64) {
	l.BackendLatency = backendLatency
}

// GetRequestMediationLatency returns the request mediation latency.
func (l *Latencies) GetRequestMediationLatency() int64 {
	return l.RequestMediationLatency
}

// SetRequestMediationLatency sets the request mediation latency.
func (l *Latencies) SetRequestMediationLatency(requestMediationLatency int64) {
	l.RequestMediationLatency = requestMediationLatency
}

// GetResponseMediationLatency returns the response mediation latency.
func (l *Latencies) GetResponseMediationLatency() int64 {
	return l.ResponseMediationLatency
}

// SetResponseMediationLatency sets the response mediation latency.
func (l *Latencies) SetResponseMediationLatency(responseMediationLatency int64) {
	l.ResponseMediationLatency = responseMediationLatency
}

func (l *Latencies) GetDuration() int64 {
	return l.Duration
}

func (l *Latencies) SetDuration(duration int64) {
	l.Duration = duration
}
