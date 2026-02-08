/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package xdsclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewDefaultConfig tests that default config is created correctly
func TestNewDefaultConfig(t *testing.T) {
	serverAddr := "localhost:18000"
	config := NewDefaultConfig(serverAddr)

	assert.NotNil(t, config)
	assert.Equal(t, serverAddr, config.ServerAddress)
	assert.Equal(t, DefaultNodeID, config.NodeID)
	assert.Equal(t, DefaultCluster, config.Cluster)
	assert.Equal(t, DefaultConnectTimeout, config.ConnectTimeout)
	assert.Equal(t, DefaultRequestTimeout, config.RequestTimeout)
	assert.Equal(t, DefaultInitialReconnectDelay, config.InitialReconnectDelay)
	assert.Equal(t, DefaultMaxReconnectDelay, config.MaxReconnectDelay)
	assert.False(t, config.TLSEnabled)
	assert.Empty(t, config.TLSCertPath)
	assert.Empty(t, config.TLSKeyPath)
	assert.Empty(t, config.TLSCAPath)
}

// TestValidate_ValidConfig tests validation with valid config
func TestValidate_ValidConfig(t *testing.T) {
	config := &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "test-node",
		Cluster:               "test-cluster",
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        5 * time.Second,
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
		TLSEnabled:            false,
	}

	err := config.Validate()
	assert.NoError(t, err)
}

// TestValidate_EmptyServerAddress tests validation fails with empty server address
func TestValidate_EmptyServerAddress(t *testing.T) {
	config := &Config{
		ServerAddress:         "",
		NodeID:                "test-node",
		Cluster:               "test-cluster",
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        5 * time.Second,
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server address is required")
}

// TestValidate_EmptyNodeID tests validation fails with empty node ID
func TestValidate_EmptyNodeID(t *testing.T) {
	config := &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "",
		Cluster:               "test-cluster",
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        5 * time.Second,
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node ID is required")
}

// TestValidate_EmptyCluster tests validation fails with empty cluster
func TestValidate_EmptyCluster(t *testing.T) {
	config := &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "test-node",
		Cluster:               "",
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        5 * time.Second,
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cluster is required")
}

// TestValidate_ZeroConnectTimeout tests validation fails with zero connect timeout
func TestValidate_ZeroConnectTimeout(t *testing.T) {
	config := &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "test-node",
		Cluster:               "test-cluster",
		ConnectTimeout:        0,
		RequestTimeout:        5 * time.Second,
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connect timeout must be positive")
}

// TestValidate_NegativeConnectTimeout tests validation fails with negative connect timeout
func TestValidate_NegativeConnectTimeout(t *testing.T) {
	config := &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "test-node",
		Cluster:               "test-cluster",
		ConnectTimeout:        -5 * time.Second,
		RequestTimeout:        5 * time.Second,
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connect timeout must be positive")
}

// TestValidate_ZeroRequestTimeout tests validation fails with zero request timeout
func TestValidate_ZeroRequestTimeout(t *testing.T) {
	config := &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "test-node",
		Cluster:               "test-cluster",
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        0,
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request timeout must be positive")
}

// TestValidate_NegativeRequestTimeout tests validation fails with negative request timeout
func TestValidate_NegativeRequestTimeout(t *testing.T) {
	config := &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "test-node",
		Cluster:               "test-cluster",
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        -5 * time.Second,
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request timeout must be positive")
}

// TestValidate_ZeroInitialReconnectDelay tests validation fails with zero initial reconnect delay
func TestValidate_ZeroInitialReconnectDelay(t *testing.T) {
	config := &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "test-node",
		Cluster:               "test-cluster",
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        5 * time.Second,
		InitialReconnectDelay: 0,
		MaxReconnectDelay:     60 * time.Second,
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "initial reconnect delay must be positive")
}

// TestValidate_NegativeInitialReconnectDelay tests validation fails with negative initial reconnect delay
func TestValidate_NegativeInitialReconnectDelay(t *testing.T) {
	config := &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "test-node",
		Cluster:               "test-cluster",
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        5 * time.Second,
		InitialReconnectDelay: -1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "initial reconnect delay must be positive")
}

// TestValidate_ZeroMaxReconnectDelay tests validation fails with zero max reconnect delay
func TestValidate_ZeroMaxReconnectDelay(t *testing.T) {
	config := &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "test-node",
		Cluster:               "test-cluster",
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        5 * time.Second,
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     0,
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max reconnect delay must be positive")
}

// TestValidate_NegativeMaxReconnectDelay tests validation fails with negative max reconnect delay
func TestValidate_NegativeMaxReconnectDelay(t *testing.T) {
	config := &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "test-node",
		Cluster:               "test-cluster",
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        5 * time.Second,
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     -60 * time.Second,
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max reconnect delay must be positive")
}

// TestValidate_TLSEnabledWithoutCertPath tests validation fails when TLS enabled but no cert path
func TestValidate_TLSEnabledWithoutCertPath(t *testing.T) {
	config := &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "test-node",
		Cluster:               "test-cluster",
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        5 * time.Second,
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
		TLSEnabled:            true,
		TLSCertPath:           "",
		TLSKeyPath:            "/path/to/key.pem",
		TLSCAPath:             "/path/to/ca.pem",
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TLS cert path is required when TLS is enabled")
}

// TestValidate_TLSEnabledWithoutKeyPath tests validation fails when TLS enabled but no key path
func TestValidate_TLSEnabledWithoutKeyPath(t *testing.T) {
	config := &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "test-node",
		Cluster:               "test-cluster",
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        5 * time.Second,
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
		TLSEnabled:            true,
		TLSCertPath:           "/path/to/cert.pem",
		TLSKeyPath:            "",
		TLSCAPath:             "/path/to/ca.pem",
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TLS key path is required when TLS is enabled")
}

// TestValidate_TLSEnabledWithoutCAPath tests validation fails when TLS enabled but no CA path
func TestValidate_TLSEnabledWithoutCAPath(t *testing.T) {
	config := &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "test-node",
		Cluster:               "test-cluster",
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        5 * time.Second,
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
		TLSEnabled:            true,
		TLSCertPath:           "/path/to/cert.pem",
		TLSKeyPath:            "/path/to/key.pem",
		TLSCAPath:             "",
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TLS CA path is required when TLS is enabled")
}

// TestValidate_TLSEnabledWithAllPaths tests validation passes when TLS enabled with all paths
func TestValidate_TLSEnabledWithAllPaths(t *testing.T) {
	config := &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "test-node",
		Cluster:               "test-cluster",
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        5 * time.Second,
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
		TLSEnabled:            true,
		TLSCertPath:           "/path/to/cert.pem",
		TLSKeyPath:            "/path/to/key.pem",
		TLSCAPath:             "/path/to/ca.pem",
	}

	err := config.Validate()
	assert.NoError(t, err)
}

// TestValidate_TLSDisabledWithPaths tests validation passes when TLS disabled even with paths present
func TestValidate_TLSDisabledWithPaths(t *testing.T) {
	config := &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "test-node",
		Cluster:               "test-cluster",
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        5 * time.Second,
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
		TLSEnabled:            false,
		TLSCertPath:           "/path/to/cert.pem",
		TLSKeyPath:            "/path/to/key.pem",
		TLSCAPath:             "/path/to/ca.pem",
	}

	err := config.Validate()
	assert.NoError(t, err)
}
