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

package it

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// AuthUser holds credentials for a test user
type AuthUser struct {
	Username string
	Password string
}

// Config holds configuration for the test suite
type Config struct {
	GatewayControllerURL       string
	RouterURL                  string
	PolicyEngineURL            string
	SampleBackendURL           string
	EchoBackendURL             string
	MockJWKSURL                string
	MockAzureContentSafetyURL  string
	MockAWSBedrockGuardrailURL string
	MockEmbeddingProviderURL   string
	RedisURL                   string
	HTTPTimeout                time.Duration
	Users                      map[string]AuthUser
}

// MockJWKSPort is the port for mock-jwks service
const MockJWKSPort = "8082"

// MockAzureContentSafetyPort is the port for mock-azure-content-safety service
const MockAzureContentSafetyPort = "8084"

// MockAWSBedrockGuardrailPort is the port for mock-aws-bedrock-guardrail service
const MockAWSBedrockGuardrailPort = "8083"

// MockEmbeddingProviderPort is the port for mock-embedding-provider service
const MockEmbeddingProviderPort = "8085"

// RedisPort is the port for redis service
const RedisPort = "6379"

// DefaultConfig returns the default test configuration
func DefaultConfig() *Config {
	return &Config{
		GatewayControllerURL:       fmt.Sprintf("http://localhost:%s", GatewayControllerPort),
		RouterURL:                  fmt.Sprintf("http://localhost:%s", RouterPort),
		PolicyEngineURL:            "http://localhost:9002",
		SampleBackendURL:           "http://localhost:9080",
		EchoBackendURL:             "http://localhost:9081",
		MockJWKSURL:                fmt.Sprintf("http://localhost:%s", MockJWKSPort),
		MockAzureContentSafetyURL:  fmt.Sprintf("http://localhost:%s", MockAzureContentSafetyPort),
		MockAWSBedrockGuardrailURL: fmt.Sprintf("http://localhost:%s", MockAWSBedrockGuardrailPort),
		MockEmbeddingProviderURL:   fmt.Sprintf("http://localhost:%s", MockEmbeddingProviderPort),
		RedisURL:                   fmt.Sprintf("localhost:%s", RedisPort),
		HTTPTimeout:                10 * time.Second,
		Users: map[string]AuthUser{
			"admin": {Username: "admin", Password: "admin"},
		},
	}
}

// TestState holds the shared state for BDD test scenarios
type TestState struct {
	// Config holds the test configuration
	Config *Config

	// HTTPClient is the HTTP client for making requests
	HTTPClient *http.Client

	// LastRequest stores the most recent HTTP request
	LastRequest *http.Request

	// LastResponse stores the most recent HTTP response
	LastResponse *http.Response

	// LastError stores the most recent error
	LastError error

	// Context stores arbitrary key-value data for steps
	Context map[string]interface{}

	// mutex protects concurrent access to state
	mutex sync.RWMutex
}

// NewTestState creates a new TestState with default configuration
func NewTestState() *TestState {
	config := DefaultConfig()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true, // ⚠️ test/dev only
	}
	return &TestState{
		Config: config,
		HTTPClient: &http.Client{
			Timeout:   config.HTTPTimeout,
			Transport: transport,
		},
		Context: make(map[string]interface{}),
	}
}

// Reset clears all state between scenarios
func (s *TestState) Reset() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Close any open response body
	if s.LastResponse != nil && s.LastResponse.Body != nil {
		s.LastResponse.Body.Close()
	}

	s.LastRequest = nil
	s.LastResponse = nil
	s.LastError = nil
	s.Context = make(map[string]interface{})
}

// SetContextValue stores a value in the context
func (s *TestState) SetContextValue(key string, value interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.Context[key] = value
}

// GetContextValue retrieves a value from the context
func (s *TestState) GetContextValue(key string) (interface{}, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	val, ok := s.Context[key]
	return val, ok
}

// GetContextString retrieves a string value from the context
func (s *TestState) GetContextString(key string) (string, bool) {
	val, ok := s.GetContextValue(key)
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetContextInt retrieves an int value from the context
func (s *TestState) GetContextInt(key string) (int, bool) {
	val, ok := s.GetContextValue(key)
	if !ok {
		return 0, false
	}
	i, ok := val.(int)
	return i, ok
}
