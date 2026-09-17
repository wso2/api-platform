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
	"strings"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// JWTSteps provides JWT authentication specific step definitions
type JWTSteps struct {
	state        *TestState
	httpSteps    *steps.HTTPSteps
	currentToken string
	mockJWKSURL  string
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
	ctx.Step(`^I set the Authorization header to the JWT token$`, j.iSetAuthorizationToJWTToken)
	ctx.Step(`^I get a JWT token from the mock JWKS server with issuer "([^"]*)" and scope "([^"]*)"$`, j.iGetJWTTokenWithIssuerAndScope)
	ctx.Step(`^I get a JWT token from the mock JWKS server with issuer "([^"]*)" and claims "([^"]*)"$`, j.iGetJWTTokenWithIssuerAndClaims)
	ctx.Step(`^I get a JWT token from the mock JWKS server with issuer "([^"]*)", scope "([^"]*)" and claims "([^"]*)"$`, j.iGetJWTTokenWithIssuerScopeAndClaims)
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

// iGetJWTTokenWithIssuerAndScope fetches a JWT token from the mock JWKS server with specific issuer and scope
func (j *JWTSteps) iGetJWTTokenWithIssuerAndScope(issuer, scope string) error {
	tokenURL := j.mockJWKSURL + "/token"
	if issuer != "" {
		tokenURL = tokenURL + "?issuer=" + url.QueryEscape(issuer)
	}
	if scope != "" {
		if issuer != "" {
			tokenURL = tokenURL + "&"
		} else {
			tokenURL = tokenURL + "?"
		}
		tokenURL = tokenURL + "scope=" + url.QueryEscape(scope)
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

// iGetJWTTokenWithIssuerAndClaims fetches a token carrying arbitrary custom claims. The claims
// argument is a comma-separated list of key=value pairs (e.g. "department=platform,role=internal").
func (j *JWTSteps) iGetJWTTokenWithIssuerAndClaims(issuer, claims string) error {
	return j.iGetJWTTokenWithIssuerScopeAndClaims(issuer, "", claims)
}

// iGetJWTTokenWithIssuerScopeAndClaims fetches a token carrying both a scope and arbitrary custom
// claims, needed to exercise policies that combine the `scopes` and `claims` params on one
// operation. claims is a comma-separated list of key=value pairs; each becomes a claim_<key>=<value>
// query param on the mock server.
func (j *JWTSteps) iGetJWTTokenWithIssuerScopeAndClaims(issuer, scope, claims string) error {
	q := url.Values{}
	if issuer != "" {
		q.Set("issuer", issuer)
	}
	if scope != "" {
		q.Set("scope", scope)
	}
	if err := addClaimParams(q, claims); err != nil {
		return err
	}

	tokenURL := j.mockJWKSURL + "/token"
	if enc := q.Encode(); enc != "" {
		tokenURL = tokenURL + "?" + enc
	}
	return j.fetchTokenFrom(tokenURL)
}

// addClaimParams parses a comma-separated "key=value" list into claim_<key>=<value> query params.
func addClaimParams(q url.Values, claims string) error {
	for _, pair := range strings.Split(claims, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("invalid claim pair %q, expected key=value", pair)
		}
		q.Set("claim_"+strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
	}
	return nil
}

// fetchTokenFrom retrieves a token from the mock server URL and stores it as the current token.
func (j *JWTSteps) fetchTokenFrom(tokenURL string) error {
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

// iSetAuthorizationToJWTToken puts the current token in the Authorization
// header without sending anything.
//
// The steps above couple fetching a token to issuing one specific request
// shape. A2A needs the token on request shapes they do not cover — a JSON-RPC
// POST carrying an operation envelope, an SSE stream — so this separates
// "carry the credential" from "make the call".
func (j *JWTSteps) iSetAuthorizationToJWTToken() error {
	if j.currentToken == "" {
		return fmt.Errorf("no JWT token available - call 'I get a JWT token from the mock JWKS server' first")
	}
	j.httpSteps.SetHeader("Authorization", "Bearer "+j.currentToken)
	return nil
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
