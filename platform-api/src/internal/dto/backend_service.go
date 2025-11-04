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

import "time"

// BackendService represents a backend service configuration
type BackendService struct {
	ID             string                `json:"id,omitempty" yaml:"id,omitempty"`
	OrganizationID string                `json:"organizationId,omitempty" yaml:"organizationId,omitempty"`
	Name           string                `json:"name" yaml:"name"`
	Description    string                `json:"description,omitempty" yaml:"description,omitempty"`
	IsDefault      bool                  `json:"isDefault,omitempty" yaml:"isDefault,omitempty"`
	Endpoints      []BackendEndpoint     `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
	Timeout        *TimeoutConfig        `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Retries        int                   `json:"retries,omitempty" yaml:"retries,omitempty"`
	LoadBalance    *LoadBalanceConfig    `json:"loadBalance,omitempty" yaml:"loadBalance,omitempty"`
	CircuitBreaker *CircuitBreakerConfig `json:"circuitBreaker,omitempty" yaml:"circuitBreaker,omitempty"`
	CreatedAt      time.Time             `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
	UpdatedAt      time.Time             `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
}

// BackendEndpoint represents a backend endpoint
type BackendEndpoint struct {
	URL         string             `json:"url,omitempty" yaml:"url,omitempty"`
	Description string             `json:"description,omitempty" yaml:"description,omitempty"`
	HealthCheck *HealthCheckConfig `json:"healthCheck,omitempty" yaml:"healthCheck,omitempty"`
	Weight      int                `json:"weight,omitempty" yaml:"weight,omitempty"`
	MTLS        *MTLSConfig        `json:"mtls,omitempty" yaml:"mtls,omitempty"`
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	Enabled            bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Interval           int  `json:"interval,omitempty" yaml:"interval,omitempty"`
	Timeout            int  `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	UnhealthyThreshold int  `json:"unhealthyThreshold,omitempty" yaml:"unhealthyThreshold,omitempty"`
	HealthyThreshold   int  `json:"healthyThreshold,omitempty" yaml:"healthyThreshold,omitempty"`
}

// TimeoutConfig represents timeout configuration
type TimeoutConfig struct {
	Connect int `json:"connect,omitempty" yaml:"connect,omitempty"`
	Read    int `json:"read,omitempty" yaml:"read,omitempty"`
	Write   int `json:"write,omitempty" yaml:"write,omitempty"`
}

// LoadBalanceConfig represents load balancing configuration
type LoadBalanceConfig struct {
	Algorithm string `json:"algorithm,omitempty" yaml:"algorithm,omitempty"`
	Failover  bool   `json:"failover,omitempty" yaml:"failover,omitempty"`
}

// CircuitBreakerConfig represents circuit breaker configuration
type CircuitBreakerConfig struct {
	Enabled            bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	MaxConnections     int  `json:"maxConnections,omitempty" yaml:"maxConnections,omitempty"`
	MaxPendingRequests int  `json:"maxPendingRequests,omitempty" yaml:"maxPendingRequests,omitempty"`
	MaxRequests        int  `json:"maxRequests,omitempty" yaml:"maxRequests,omitempty"`
	MaxRetries         int  `json:"maxRetries,omitempty" yaml:"maxRetries,omitempty"`
}
