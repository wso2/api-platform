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

package it

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// mtlsFixturesDir holds the generated certificate and key fixtures, relative
// to the `it` module root that tests run from.
const mtlsFixturesDir = "resources/mtls-pki"

// mtlsPemPlaceholder, mtlsKeyPlaceholder and mtlsEncryptedKeyPlaceholder match
// {{pem "name"}} / {{key "name"}} / {{encryptedkey "name"}} template markers
// inside a docstring request body.
var (
	mtlsPemPlaceholder          = regexp.MustCompile(`\{\{pem "([^"]+)"\}\}`)
	mtlsKeyPlaceholder          = regexp.MustCompile(`\{\{key "([^"]+)"\}\}`)
	mtlsEncryptedKeyPlaceholder = regexp.MustCompile(`\{\{encryptedkey "([^"]+)"\}\}`)
)

// mtlsNotAfterPlaceholder matches {{notafter "name"}} template markers,
// expanded to fixture "name"'s certificate NotAfter timestamp in RFC3339.
var mtlsNotAfterPlaceholder = regexp.MustCompile(`\{\{notafter "([^"]+)"\}\}`)

// mtlsThumbprintPlaceholder matches {{thumbprint "name"}} template markers
// inside a docstring request body, expanded to fixture "name"'s SHA-256
// thumbprint (64 lowercase hex characters of the certificate's DER bytes).
var mtlsThumbprintPlaceholder = regexp.MustCompile(`\{\{thumbprint "([^"]+)"\}\}`)

// mtlsSteps holds the mTLS step definitions and the certificate names this
// scenario attempted to upload, for best-effort cleanup.
type mtlsSteps struct {
	state     *TestState
	httpSteps *steps.HTTPSteps
	jwtSteps  *JWTSteps

	uploadedNames []string

	// uploadedIdentityNames tracks gateway identities this scenario attempted
	// to upload. Cleanup deletes them before uploadedNames.
	uploadedIdentityNames []string

	// resumingClient keeps a TLS session cache across requests so a later
	// request on a new connection offers any session the gateway let it cache.
	resumingClient *http.Client

	// previousConfigNames holds the API and Agent names the previous @mtls
	// scenario deployed, whose deletes may still be converging.
	previousConfigNames []string
}

// RegisterMTLSSteps registers the mTLS step definitions and returns the
// instance so other step groups can reuse its fixture helpers.
func RegisterMTLSSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps, jwtSteps *JWTSteps) *mtlsSteps {
	m := &mtlsSteps{state: state, httpSteps: httpSteps, jwtSteps: jwtSteps}

	ctx.Before(func(c context.Context, sc *godog.Scenario) (context.Context, error) {
		m.uploadedNames = nil
		m.uploadedIdentityNames = nil
		m.resumingClient = nil
		if scenarioHasTag(sc, "@mtls") {
			markMTLSScenario(m.state)
		}
		return c, nil
	})
	ctx.After(func(c context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		// A pooled certificate cannot be removed while a deployed API still
		// references it, so APIs and Agents are deleted first.
		if !scenarioHasTag(sc, "@mtls") {
			return c, nil
		}
		// The next scenario starts from a gateway that has applied every
		// delete: routes gone, pool converged, no listener warming.
		cleanupDeployedAPIs(m.state, m.httpSteps)
		cleanupDeployedAgents(m.state, m.httpSteps)
		m.previousConfigNames = scenarioConfigNames(m.state)
		if err := waitForDeployedRoutesRemoved(m.state); err != nil {
			return c, err
		}
		m.cleanupTrackedCertificates()
		if err := waitForClientAuthorityPool(m.state); err != nil {
			return c, err
		}
		if err := waitForEnvoyListenersActive(m.state); err != nil {
			return c, err
		}
		return c, nil
	})

	// Uploading
	ctx.Step(`^I upload the certificate fixture "([^"]*)" as "([^"]*)" with usage "([^"]*)" and role "([^"]*)" and dns SAN "([^"]*)"$`, m.uploadFixtureWithUsageRoleAndDNSSAN)
	ctx.Step(`^I upload the certificate fixture "([^"]*)" as "([^"]*)" with usage "([^"]*)" and role "([^"]*)"$`, m.uploadFixtureWithUsageAndRole)
	ctx.Step(`^I upload the certificate fixture "([^"]*)" as "([^"]*)" with usage "([^"]*)"$`, m.uploadFixtureWithUsage)
	ctx.Step(`^I upload the certificate fixture "([^"]*)" as "([^"]*)"$`, m.uploadFixtureNoUsage)
	ctx.Step(`^I upload the certificate fixtures "([^"]*)" as "([^"]*)" with usage "([^"]*)"$`, m.uploadFixturesWithUsage)
	ctx.Step(`^the certificate fixture "([^"]*)" is pooled as "([^"]*)" with usage "([^"]*)"$`, m.pooledFixtureWithUsage)
	ctx.Step(`^the certificate fixture "([^"]*)" is pooled as "([^"]*)"$`, m.pooledFixtureNoUsage)
	ctx.Step(`^I upload to the certificates endpoint the body:$`, m.uploadRawBody)
	ctx.Step(`^I upload a certificate body a tenth over the upload limit as "([^"]*)" with usage "([^"]*)"$`, m.uploadOversizedBody)

	// Gateway identities: certificates pooled with usage "identity".
	ctx.Step(`^I upload the gateway identity fixture "([^"]*)" with its chain as "([^"]*)"$`, m.uploadGatewayIdentityFixtureWithChain)
	ctx.Step(`^I upload the gateway identity fixture "([^"]*)" as "([^"]*)"$`, m.uploadGatewayIdentityFixture)
	ctx.Step(`^the gateway identity fixture "([^"]*)" is stored as "([^"]*)"$`, m.gatewayIdentityFixtureIsStored)
	ctx.Step(`^I upload to the certificates endpoint the identity body:$`, m.uploadGatewayIdentityRawBody)
	ctx.Step(`^I delete the gateway identity named "([^"]*)"$`, m.deleteGatewayIdentityNamed)
	ctx.Step(`^I update the gateway identity "([^"]*)" with the fixture "([^"]*)" and its chain$`, m.updateGatewayIdentityWithFixtureAndChain)
	ctx.Step(`^I update the certificate "([^"]*)" with the identity fixture "([^"]*)"$`, m.updateCertificateWithIdentityFixture)

	// Deploying and updating with fixture-derived values
	ctx.Step(`^I deploy this API configuration with fixture values:$`, m.deployWithFixtureValues)
	ctx.Step(`^I update the API "([^"]*)" with this configuration with fixture values:$`, m.updateWithFixtureValues)

	// Listing assertions, run against the last response
	ctx.Step(`^the certificate list should contain "([^"]*)"$`, m.listShouldContain)
	ctx.Step(`^the certificate list should not contain "([^"]*)"$`, m.listShouldNotContain)
	ctx.Step(`^the listed certificate "([^"]*)" should have "([^"]*)" equal to "?([^"]*)"?$`, m.listedCertShouldHaveFieldEqualTo)
	ctx.Step(`^the listed certificate "([^"]*)" should not have field "([^"]*)"$`, m.listedCertShouldNotHaveField)
	ctx.Step(`^the listed certificate "([^"]*)" should have a warning with code "([^"]*)"$`, m.listedCertShouldHaveWarningWithCode)
	ctx.Step(`^the listed certificate "([^"]*)" should have a warning with field "([^"]*)"$`, m.listedCertShouldHaveWarningWithField)
	ctx.Step(`^the listed certificate "([^"]*)" should have no warnings$`, m.listedCertShouldHaveNoWarnings)

	// Validation errors
	ctx.Step(`^the response should list a validation error for field "([^"]*)" with message "([^"]*)"$`, m.validationErrorWithMessage)
	ctx.Step(`^the response should list a validation error for field "([^"]*)" containing "([^"]*)"$`, m.validationErrorContaining)
	ctx.Step(`^the response should list a validation error for field "([^"]*)"$`, m.validationErrorAny)

	// Fixture-derived assertions
	ctx.Step(`^the JSON response field "([^"]*)" should be the subject of fixture "([^"]*)"$`, m.jsonFieldShouldBeSubjectOfFixture)

	// Deletion
	ctx.Step(`^I delete the certificate named "([^"]*)"$`, m.deleteCertificateNamed)
	ctx.Step(`^I delete the certificate named "([^"]*)" once no API references it$`, m.deleteCertificateNamedOnceUnreferenced)

	// HTTPS listener probing
	ctx.Step(`^the HTTPS listener should request a client certificate$`, m.httpsListenerShouldRequestClientCertificate)
	ctx.Step(`^the HTTPS listener should not request a client certificate$`, m.httpsListenerShouldNotRequestClientCertificate)
	ctx.Step(`^the HTTPS listener should stop requesting a client certificate$`, m.httpsListenerShouldStopRequestingClientCertificate)
	ctx.Step(`^the HTTPS listener should present the certificate in "([^"]*)"$`, m.httpsListenerShouldPresentCertificateFile)

	// Requests carrying or omitting a client certificate
	ctx.Step(`^I send a GET request to "([^"]*)" with client certificate "([^"]*)" and its chain$`, m.getWithClientCertificateAndChain)
	ctx.Step(`^I send a GET request to "([^"]*)" with client certificate "([^"]*)" and header "([^"]*)" carrying certificate "([^"]*)" encoded as "([^"]*)"$`, m.getWithClientCertificateAndHeaderCertificateEncoded)
	ctx.Step(`^I send a GET request to "([^"]*)" with client certificate "([^"]*)" and header "([^"]*)" carrying certificate "([^"]*)"$`, m.getWithClientCertificateAndHeaderCertificate)
	ctx.Step(`^the backend's X-Forwarded-Client-Cert should name certificate "([^"]*)"$`, m.echoedXFCCShouldName)
	ctx.Step(`^the backend's X-Forwarded-Client-Cert should not name certificate "([^"]*)"$`, m.echoedXFCCShouldNotName)
	ctx.Step(`^I send a GET request to "([^"]*)" with client certificate "([^"]*)"$`, m.getWithClientCertificate)
	ctx.Step(`^I send a GET request to "([^"]*)" with client certificate "([^"]*)" on a resumable TLS session$`, m.getWithClientCertificateOnResumableSession)
	ctx.Step(`^I send a GET request to "([^"]*)" on a new connection from the same TLS session cache$`, m.getWithCachedTLSSession)
	ctx.Step(`^the gateway should have run a full TLS handshake$`, m.gatewayShouldHaveRunFullTLSHandshake)
	ctx.Step(`^I send a GET request to "([^"]*)" with no client certificate and header "([^"]*)" carrying certificate "([^"]*)"$`, m.getWithNoClientCertificateAndHeaderCertificate)
	ctx.Step(`^I send a GET request to "([^"]*)" with no client certificate$`, m.getWithNoClientCertificate)
	ctx.Step(`^I send a GET request to "([^"]*)" with header "([^"]*)" carrying certificate "([^"]*)"$`, m.getWithHeaderCertificate)
	ctx.Step(`^I send a GET request to "([^"]*)" with the JWT token and client certificate "([^"]*)"$`, m.getWithJWTTokenAndClientCertificate)
	ctx.Step(`^I send a GET request to "([^"]*)" with the JWT token and no client certificate$`, m.getWithJWTTokenAndNoClientCertificate)

	// Client authority pool state
	ctx.Step(`^the client authority pool is empty$`, m.clientAuthorityPoolIsEmpty)
	ctx.Step(`^the gateway has applied the client authority pool$`, func() error {
		return waitForClientAuthorityPool(m.state)
	})

	// Deploy-response warnings
	ctx.Step(`^the response should include a warning with code "([^"]*)" for field "([^"]*)"$`, m.responseShouldIncludeWarningWithCodeForField)
	ctx.Step(`^the response should include a warning with code "([^"]*)"$`, m.responseShouldIncludeWarningWithCode)
	ctx.Step(`^the response should include no warnings$`, m.responseShouldIncludeNoWarnings)

	return m
}

func (m *mtlsSteps) readFixtureFile(fixture, ext string) ([]byte, error) {
	path := filepath.Join(mtlsFixturesDir, fixture+ext)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate fixture %q: %w", path, err)
	}
	return data, nil
}

func (m *mtlsSteps) readFixtureCert(fixture string) ([]byte, error) {
	return m.readFixtureFile(fixture, ".crt")
}

func (m *mtlsSteps) parseFixtureCert(fixture string) (*x509.Certificate, error) {
	data, err := m.readFixtureCert(fixture)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("fixture %q does not contain a PEM certificate", fixture)
	}
	return x509.ParseCertificate(block.Bytes)
}

// resolveTemplates replaces {{pem}}, {{key}}, {{encryptedkey}} and
// {{notafter}} markers with the named fixture's JSON-string-escaped value.
func (m *mtlsSteps) resolveTemplates(body string) (string, error) {
	var firstErr error

	substitute := func(re *regexp.Regexp, ext string, input string) string {
		return re.ReplaceAllStringFunc(input, func(match string) string {
			sub := re.FindStringSubmatch(match)
			fixture := sub[1]
			data, err := m.readFixtureFile(fixture, ext)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return match
			}
			escaped, err := json.Marshal(string(data))
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to encode fixture %q: %w", fixture, err)
				}
				return match
			}
			// Strip the quotes: the placeholder already sits inside a quoted
			// JSON string in the template.
			return string(escaped[1 : len(escaped)-1])
		})
	}

	substituteNotAfter := func(input string) string {
		return mtlsNotAfterPlaceholder.ReplaceAllStringFunc(input, func(match string) string {
			sub := mtlsNotAfterPlaceholder.FindStringSubmatch(match)
			fixture := sub[1]
			value, err := m.notAfterOf(fixture)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return match
			}
			escaped, err := json.Marshal(value)
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to encode notAfter for fixture %q: %w", fixture, err)
				}
				return match
			}
			return string(escaped[1 : len(escaped)-1])
		})
	}

	body = substitute(mtlsPemPlaceholder, ".crt", body)
	body = substitute(mtlsKeyPlaceholder, ".key", body)
	body = substitute(mtlsEncryptedKeyPlaceholder, ".encrypted.key", body)
	body = substituteNotAfter(body)
	if firstErr != nil {
		return "", firstErr
	}
	return body, nil
}

// thumbprintOf returns the fixture's SHA-256 thumbprint as lowercase hex
// over the certificate's DER bytes, the form the mtls-auth policy expects.
func (m *mtlsSteps) thumbprintOf(fixture string) (string, error) {
	cert, err := m.parseFixtureCert(fixture)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:]), nil
}

// resolveThumbprintTemplates replaces {{thumbprint "name"}} markers with the
// fixture's thumbprint. Hex needs no escaping.
func (m *mtlsSteps) resolveThumbprintTemplates(body string) (string, error) {
	var firstErr error
	resolved := mtlsThumbprintPlaceholder.ReplaceAllStringFunc(body, func(match string) string {
		sub := mtlsThumbprintPlaceholder.FindStringSubmatch(match)
		fixture := sub[1]
		thumb, err := m.thumbprintOf(fixture)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return match
		}
		return thumb
	})
	if firstErr != nil {
		return "", firstErr
	}
	return resolved, nil
}

// notAfterOf returns the fixture certificate's NotAfter as RFC3339 in UTC.
func (m *mtlsSteps) notAfterOf(fixture string) (string, error) {
	cert, err := m.parseFixtureCert(fixture)
	if err != nil {
		return "", err
	}
	return cert.NotAfter.UTC().Format(time.RFC3339), nil
}

// resolveNotAfterTemplates replaces {{notafter "name"}} markers in plain
// text, such as an expected error message, without JSON escaping.
func (m *mtlsSteps) resolveNotAfterTemplates(s string) (string, error) {
	var firstErr error
	resolved := mtlsNotAfterPlaceholder.ReplaceAllStringFunc(s, func(match string) string {
		sub := mtlsNotAfterPlaceholder.FindStringSubmatch(match)
		fixture := sub[1]
		value, err := m.notAfterOf(fixture)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return match
		}
		return value
	})
	if firstErr != nil {
		return "", firstErr
	}
	return resolved, nil
}
