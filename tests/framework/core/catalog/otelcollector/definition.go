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

package otelcollector

import (
	"time"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
)

// EnvImageOTelCollector overrides the collector image, for CI to pin a build.
const EnvImageOTelCollector = "APIP_IT_IMAGE_OTEL_COLLECTOR"

// Name is the component name suite files reference and the DNS alias the gateway exports to.
const Name = "otel-collector"

const (
	// GRPCPort receives OTLP/gRPC traces.
	GRPCPort = 4317
	// HTTPPort receives OTLP/HTTP traces at /v1/traces and log records at /v1/logs.
	HTTPPort = 4318
	// APIPort serves what the collector received, at /test/spans and /test/logs.
	APIPort = 8080
)

const imgOTelCollector = "ghcr.io/wso2/api-platform/mock-otlp-collector:test"

// Collector returns the OTLP collector component definition.
//
// The image is built from tests/mock-servers/mock-otlp-collector by the framework Makefile's
// otel-collector target, exactly as the A2A trip-planner image is.
func Collector() *components.Definition {
	return &components.Definition{
		Name:  Name,
		Image: shared.Image(EnvImageOTelCollector, imgOTelCollector),
		Alias: Name,
		Endpoints: []components.Endpoint{
			{Name: "grpc", Port: GRPCPort, Scheme: "grpc", AwaitListening: true},
			{Name: "http", Port: HTTPPort, Scheme: "http", AwaitListening: true},
			{Name: "api", Port: APIPort, Scheme: "http", AwaitListening: true},
		},
		Health: &components.HealthCheck{
			Endpoint: "api", Path: "/test/health", ExpectStatus: 200,
			Timeout: 30 * time.Second, Interval: time.Second,
		},
		Limits: components.ResourceLimits{CPUs: 0.25, MemoryMB: 128},
	}
}
