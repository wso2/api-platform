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
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// JWTSteps provides JWT authentication specific step definitions
type JWTSteps struct {
	state         *TestState
	httpSteps     *steps.HTTPSteps
	currentToken  string
	mockJWKSURL   string
}

// NewJWTSteps creates a new JWTSteps instance
func NewJWTSteps(state *TestState, httpSteps *steps.HTTPSteps, mockJWKSURL string) *JWTSteps {
	return &JWTSteps{
		state:       state,
		httpSteps:   httpSteps,
		mockJWKSURL: mockJWKSURL,
	}
}

// Register registers all JWT step definitions
func (j *JWTSteps) Register(ctx *godog.ScenarioContext) {
	ctx.Step(`^I get a JWT token from the mock JWKS server$`, j.iGetJWTToken)
	ctx.Step(`^I get a JWT token from the mock JWKS server with issuer "([^"]*)"$`, j.iGetJWTTokenWithIssuer)
	ctx.Step(`^I send a GET request to "([^"]*)" with the JWT token$`, j.iSendGETRequestWithJWTToken)
	ctx.Step(`^I send a POST request to "([^"]*)" with the JWT token$`, j.iSendPOSTRequestWithJWTToken)
	ctx.Step(`^I send a GET request to "([^"]*)" with JWT in header "([^"]*)"$`, j.iSendGETRequestWithJWTInHeader)
}

// Reset clears JWT state between scenarios
func (j *JWTSteps) Reset() {
	j.currentToken = ""
}

// iGetJWTToken fetches a JWT token from the mock JWKS server with default issuer
func (j *JWTSteps) iGetJWTToken() error {
	return j.iGetJWTTokenWithIssuer("")
}

// iGetJWTTokenWithIssuer fetches a JWT token from the mock JWKS server with a specific issuer
func (j *JWTSteps) iGetJWTTokenWithIssuer(issuer string) error {
	tokenURL := j.mockJWKSURL + "/token"
	if issuer != "" {
		tokenURL = tokenURL + "?issuer=" + url.QueryEscape(issuer)
	}

	log.Printf("DEBUG: Fetching JWT token from %s", tokenURL)

	resp, err := j.state.HTTPClient.Get(tokenURL)
	if err != nil {
		return fmt.Errorf("failed to get JWT token from mock JWKS server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mock JWKS server returned status %d", resp.StatusCode)
	}

	tokenBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read token response: %w", err)
	}

	j.currentToken = string(tokenBytes)
	log.Printf("DEBUG: Obtained JWT token (length: %d)", len(j.currentToken))

	return nil
}

// iSendGETRequestWithJWTToken sends a GET request with the current JWT token in Authorization header
func (j *JWTSteps) iSendGETRequestWithJWTToken(url string) error {
	if j.currentToken == "" {
		return fmt.Errorf("no JWT token available - call 'I get a JWT token from the mock JWKS server' first")
	}

	// Clear any existing Authorization header and set the JWT token
	j.httpSteps.SetHeader("Authorization", "Bearer "+j.currentToken)
	return j.httpSteps.SendGETRequest(url)
}

// iSendPOSTRequestWithJWTToken sends a POST request with the current JWT token in Authorization header
func (j *JWTSteps) iSendPOSTRequestWithJWTToken(url string) error {
	if j.currentToken == "" {
		return fmt.Errorf("no JWT token available - call 'I get a JWT token from the mock JWKS server' first")
	}

	j.httpSteps.SetHeader("Authorization", "Bearer "+j.currentToken)
	return j.httpSteps.ISendPOSTRequest(url)
}

// iSendGETRequestWithJWTInHeader sends a GET request with the JWT token in a custom header
func (j *JWTSteps) iSendGETRequestWithJWTInHeader(url, headerName string) error {
	if j.currentToken == "" {
		return fmt.Errorf("no JWT token available - call 'I get a JWT token from the mock JWKS server' first")
	}

	j.httpSteps.SetHeader(headerName, j.currentToken)
	return j.httpSteps.ISendGETRequest(url)
}

// RegisterJWTSteps registers JWT step definitions with the scenario context
func RegisterJWTSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps, jwtSteps *JWTSteps) {
	if jwtSteps != nil {
		jwtSteps.Register(ctx)
	}
}
