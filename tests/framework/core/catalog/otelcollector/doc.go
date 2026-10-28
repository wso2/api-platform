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

// Package otelcollector declares the OTLP collector component: it receives the spans and
// analytics log records a gateway exports and serves them back to the tracing steps.
//
// It is not a testbench service. OTLP over gRPC is not an HTTP handler, and the gateway's
// exporters always post to /v1/traces, so the collector cannot be addressed by a block path
// prefix the way the testbench's stateful services are. It keeps what it receives in memory,
// so it is a per-block component: one block's spans are never observable from another.
package otelcollector
