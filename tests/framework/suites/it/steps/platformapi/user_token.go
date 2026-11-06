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

package platformapi

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/cucumber/godog"

	controlplane "github.com/wso2/api-platform/tests/framework/core/catalog/platformapi"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// controlPlaneSigningKeyFile is the control plane's token-signing key within
// shared.ControlPlaneCrypto; platform-api verifies its tokens against the matching public half.
const controlPlaneSigningKeyFile = "keys/jwt_private.pem"

// actorOnlyClaims are login-token claims that name the logged-in actor rather than the
// organization or its grant, so a minted token never carries them unless its table sets them.
var actorOnlyClaims = []string{"username", "name"}

// RegisterUserToken binds the step that mints a control-plane token for a user other than the
// suite's file-mode actors, such as an IdP user identified by a UUID sub, with or without a
// display-name claim. Requests carry it with the control-plane request step's `with token`.
func RegisterUserToken(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^I mint a control-plane token stored as "([^"]*)" for the "([^"]*)" role with claims:$`,
		s.mintUserToken)
}

// mintUserToken logs in as actor, so the token carries the issuer, expiry, organization and
// grant platform-api accepts, replaces the actor's identity with the table's claims, and signs
// the result with the control plane's own key. A claim the table does not name, such as name,
// is absent from the token.
func (s *Steps) mintUserToken(ctx context.Context, storeAs, actor string, table *godog.Table) error {
	storeAs = strings.TrimSpace(storeAs)
	if storeAs == "" {
		return errors.New("a minted control-plane token needs a name to be stored as")
	}
	claims, err := stepscommon.ClaimsFromTable(ctx, table)
	if err != nil {
		return err
	}
	credentials, err := actorCredentials(actor)
	if err != nil {
		return err
	}
	base, err := s.baseURL()
	if err != nil {
		return err
	}
	login, err := controlplane.ControlPlaneLogin(ctx, base, credentials.Username, credentials.Password)
	if err != nil {
		return fmt.Errorf("authenticating to the control plane as %q: %w", credentials.Username, err)
	}
	token, err := signUserToken(login, claims, shared.ControlPlaneCrypto()[controlPlaneSigningKeyFile])
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, storeAs, token)
}

// rs256Header is the protected header of every token signUserToken produces.
const rs256Header = `{"alg":"RS256","typ":"JWT"}`

// signUserToken returns login's claims, minus the actor-only ones, overlaid with overrides and
// signed as a compact RS256 JWS by the PKCS #8 key in keyPEM. login's own signature is not
// checked: it comes straight from the control plane's login endpoint.
func signUserToken(login string, overrides map[string]any, keyPEM []byte) (string, error) {
	claims, err := jwsClaims(login)
	if err != nil {
		return "", fmt.Errorf("decoding the control-plane login token: %w", err)
	}
	for _, name := range actorOnlyClaims {
		delete(claims, name)
	}
	maps.Copy(claims, overrides)

	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return "", errors.New("the control-plane signing key is not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parsing the control-plane signing key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("the control-plane signing key is %T, not RSA", parsed)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("encoding the control-plane token claims: %w", err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString([]byte(rs256Header)) + "." +
		base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("signing the control-plane token: %w", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

// jwsClaims decodes the payload of a compact JWS as a JSON object.
func jwsClaims(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("not a compact JWS")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	if claims == nil {
		return nil, errors.New("the payload is not a JSON object")
	}
	return claims, nil
}
