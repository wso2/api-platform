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
	"encoding/json"
	"fmt"

	"github.com/cucumber/godog"
)

func (m *mtlsSteps) recordUploadedIdentity(name string) {
	m.uploadedIdentityNames = append(m.uploadedIdentityNames, name)
}

// uploadGatewayIdentityRaw pools a gateway identity: a certificate with
// usage "identity" and its private key. certExt is ".crt" for a bare leaf or
// ".chain.crt" for a leaf plus its intermediate chain.
func (m *mtlsSteps) uploadGatewayIdentityRaw(fixture, name, certExt string) error {
	m.recordUploadedIdentity(name)

	certPEM, err := m.readFixtureFile(fixture, certExt)
	if err != nil {
		return err
	}
	keyPEM, err := m.readFixtureFile(fixture, ".key")
	if err != nil {
		return err
	}

	body := map[string]any{
		"name":        name,
		"usage":       "identity",
		"certificate": string(certPEM),
		"privateKey":  string(keyPEM),
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode gateway identity upload body: %w", err)
	}

	m.httpSteps.SetHeader("Content-Type", "application/json")
	return m.httpSteps.SendPOSTToService("gateway-controller", "/certificates", &godog.DocString{Content: string(bodyBytes)})
}

func (m *mtlsSteps) uploadGatewayIdentityFixture(fixture, name string) error {
	return m.uploadGatewayIdentityRaw(fixture, name, ".crt")
}

func (m *mtlsSteps) uploadGatewayIdentityFixtureWithChain(fixture, name string) error {
	return m.uploadGatewayIdentityRaw(fixture, name, ".chain.crt")
}

// gatewayIdentityFixtureIsStored uploads the fixture and fails unless the
// gateway answered 201.
func (m *mtlsSteps) gatewayIdentityFixtureIsStored(fixture, name string) error {
	if err := m.uploadGatewayIdentityFixture(fixture, name); err != nil {
		return err
	}
	resp := m.httpSteps.LastResponse()
	if resp == nil {
		return fmt.Errorf("expected gateway identity to be stored, but no response was received")
	}
	if resp.StatusCode != 201 {
		return fmt.Errorf("expected gateway identity to be stored (status 201), got %d: %s", resp.StatusCode, string(m.httpSteps.LastBody()))
	}
	return nil
}

func (m *mtlsSteps) recordUploadedIdentityNameFromJSON(body string) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return
	}
	if name, ok := parsed["name"].(string); ok && name != "" {
		m.recordUploadedIdentity(name)
	}
}

// uploadGatewayIdentityRawBody posts a templated docstring body to
// /certificates. The body sets its own usage field.
func (m *mtlsSteps) uploadGatewayIdentityRawBody(body *godog.DocString) error {
	resolved, err := m.resolveTemplates(body.Content)
	if err != nil {
		return err
	}
	m.recordUploadedIdentityNameFromJSON(resolved)

	m.httpSteps.SetHeader("Content-Type", "application/json")
	return m.httpSteps.SendPOSTToService("gateway-controller", "/certificates", &godog.DocString{Content: resolved})
}

// findGatewayIdentityIDByName resolves a name among usage=identity
// certificates only, so an identity is never confused with a certificate.
func (m *mtlsSteps) findGatewayIdentityIDByName(name string) (string, error) {
	if err := m.httpSteps.SendGETToService("gateway-controller", "/certificates?usage=identity"); err != nil {
		return "", err
	}
	var parsed struct {
		Certificates []map[string]any `json:"certificates"`
	}
	if err := json.Unmarshal(m.httpSteps.LastBody(), &parsed); err != nil {
		return "", fmt.Errorf("failed to parse gateway identity list while resolving %q: %w", name, err)
	}
	for _, item := range parsed.Certificates {
		if fmt.Sprint(item["name"]) == name {
			return fmt.Sprint(item["id"]), nil
		}
	}
	return "", fmt.Errorf("no gateway identity named %q found", name)
}

// deleteGatewayIdentityNamed deletes the named gateway identity, failing
// with a clear error rather than a bare 404 when the name is not found.
func (m *mtlsSteps) deleteGatewayIdentityNamed(name string) error {
	id, err := m.findGatewayIdentityIDByName(name)
	if err != nil {
		return fmt.Errorf("failed to delete gateway identity %q: %w", name, err)
	}
	return m.httpSteps.SendDELETEToService("gateway-controller", "/certificates/"+id)
}

// updateGatewayIdentityWithFixtureAndChain rotates the named gateway identity
// in place to the fixture's chain and key.
func (m *mtlsSteps) updateGatewayIdentityWithFixtureAndChain(name, fixture string) error {
	id, err := m.findGatewayIdentityIDByName(name)
	if err != nil {
		return fmt.Errorf("failed to update gateway identity %q: %w", name, err)
	}

	certPEM, err := m.readFixtureFile(fixture, ".chain.crt")
	if err != nil {
		return err
	}
	keyPEM, err := m.readFixtureFile(fixture, ".key")
	if err != nil {
		return err
	}

	body := map[string]any{
		"certificate": string(certPEM),
		"privateKey":  string(keyPEM),
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode gateway identity update body: %w", err)
	}

	m.httpSteps.SetHeader("Content-Type", "application/json")
	return m.httpSteps.SendPUTToService("gateway-controller", "/certificates/"+id, &godog.DocString{Content: string(bodyBytes)})
}

// updateCertificateWithIdentityFixture PUTs an identity-shaped body to a
// non-identity certificate, which the gateway must refuse.
func (m *mtlsSteps) updateCertificateWithIdentityFixture(name, fixture string) error {
	id, err := m.findCertificateIDByName(name)
	if err != nil {
		return fmt.Errorf("failed to update certificate %q: %w", name, err)
	}

	certPEM, err := m.readFixtureFile(fixture, ".crt")
	if err != nil {
		return err
	}
	keyPEM, err := m.readFixtureFile(fixture, ".key")
	if err != nil {
		return err
	}

	body := map[string]any{
		"certificate": string(certPEM),
		"privateKey":  string(keyPEM),
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode certificate update body: %w", err)
	}

	m.httpSteps.SetHeader("Content-Type", "application/json")
	return m.httpSteps.SendPUTToService("gateway-controller", "/certificates/"+id, &godog.DocString{Content: string(bodyBytes)})
}
