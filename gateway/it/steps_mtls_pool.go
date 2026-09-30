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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

func (m *mtlsSteps) recordUploaded(name string) {
	m.uploadedNames = append(m.uploadedNames, name)
}

// uploadRaw uploads a certificate body, with an optional "match" clause
// (e.g. a dnsSANs narrowing on a relay entry) included when match is non-nil.
func (m *mtlsSteps) uploadRaw(name, certPEM, usage, role string, match map[string]any) error {
	m.recordUploaded(name)

	body := map[string]any{
		"name":        name,
		"certificate": certPEM,
	}
	if usage != "" {
		body["usage"] = usage
	}
	if role != "" {
		body["role"] = role
	}
	if match != nil {
		body["match"] = match
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode certificate upload body: %w", err)
	}

	m.httpSteps.SetHeader("Content-Type", "application/json")
	if err := m.httpSteps.SendPOSTToService("gateway-controller", "/certificates", &godog.DocString{Content: string(bodyBytes)}); err != nil {
		return err
	}
	m.notePoolChangeIfAccepted(usage)
	return nil
}

// notePoolChangeIfAccepted records a client authority pool change when the
// last certificate mutation, on a certificate of the given usage, succeeded.
func (m *mtlsSteps) notePoolChangeIfAccepted(usage string) {
	if usage != "downstream" {
		return
	}
	if resp := m.httpSteps.LastResponse(); resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		markClientAuthorityPoolChanged(m.state)
	}
}

func (m *mtlsSteps) uploadFixtureNoUsage(fixture, name string) error {
	certPEM, err := m.readFixtureCert(fixture)
	if err != nil {
		return err
	}
	return m.uploadRaw(name, string(certPEM), "", "", nil)
}

func (m *mtlsSteps) uploadFixtureWithUsage(fixture, name, usage string) error {
	certPEM, err := m.readFixtureCert(fixture)
	if err != nil {
		return err
	}
	return m.uploadRaw(name, string(certPEM), usage, "", nil)
}

func (m *mtlsSteps) uploadFixtureWithUsageAndRole(fixture, name, usage, role string) error {
	certPEM, err := m.readFixtureCert(fixture)
	if err != nil {
		return err
	}
	return m.uploadRaw(name, string(certPEM), usage, role, nil)
}

// uploadFixtureWithUsageRoleAndDNSSAN uploads the fixture narrowed by a single
// DNS SAN, so a relay entry vouches only for a connection carrying that SAN.
func (m *mtlsSteps) uploadFixtureWithUsageRoleAndDNSSAN(fixture, name, usage, role, dnsSAN string) error {
	certPEM, err := m.readFixtureCert(fixture)
	if err != nil {
		return err
	}
	return m.uploadRaw(name, string(certPEM), usage, role, map[string]any{
		"dnsSANs": []string{dnsSAN},
	})
}

func (m *mtlsSteps) uploadFixturesWithUsage(fixtureList, name, usage string) error {
	fixtures := strings.Split(fixtureList, ",")
	var buf bytes.Buffer
	for i, f := range fixtures {
		data, err := m.readFixtureCert(strings.TrimSpace(f))
		if err != nil {
			return err
		}
		if i > 0 {
			buf.WriteString("\n")
		}
		buf.Write(data)
	}
	return m.uploadRaw(name, buf.String(), usage, "", nil)
}

func (m *mtlsSteps) expectPooled() error {
	resp := m.httpSteps.LastResponse()
	if resp == nil {
		return fmt.Errorf("expected certificate to be pooled, but no response was received")
	}
	if resp.StatusCode != 201 {
		return fmt.Errorf("expected certificate to be pooled (status 201), got %d: %s", resp.StatusCode, string(m.httpSteps.LastBody()))
	}
	return nil
}

func (m *mtlsSteps) pooledFixtureWithUsage(fixture, name, usage string) error {
	if err := m.uploadFixtureWithUsage(fixture, name, usage); err != nil {
		return err
	}
	return m.expectPooled()
}

func (m *mtlsSteps) pooledFixtureNoUsage(fixture, name string) error {
	if err := m.uploadFixtureNoUsage(fixture, name); err != nil {
		return err
	}
	return m.expectPooled()
}

func (m *mtlsSteps) uploadRawBody(body *godog.DocString) error {
	resolved, err := m.resolveTemplates(body.Content)
	if err != nil {
		return err
	}
	m.recordUploadedNameFromJSON(resolved)

	m.httpSteps.SetHeader("Content-Type", "application/json")
	if err := m.httpSteps.SendPOSTToService("gateway-controller", "/certificates", &godog.DocString{Content: resolved}); err != nil {
		return err
	}
	var parsed struct {
		Usage string `json:"usage"`
	}
	if json.Unmarshal([]byte(resolved), &parsed) == nil {
		m.notePoolChangeIfAccepted(parsed.Usage)
	}
	return nil
}

func (m *mtlsSteps) recordUploadedNameFromJSON(body string) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return
	}
	if name, ok := parsed["name"].(string); ok && name != "" {
		m.recordUploaded(name)
	}
}

// certificateUploadLimitBytes mirrors the controller's default
// max_certificate_upload_bytes, which the test configuration leaves unset.
const certificateUploadLimitBytes = 1 << 20

// uploadOversizedBody sends a body a tenth over the upload limit through a
// client with its own timeout: the controller answers 413 and closes the
// connection while the body is still being written, and a write that stalls
// behind that close must fail the scenario rather than hang the suite.
func (m *mtlsSteps) uploadOversizedBody(name, usage string) error {
	m.recordUploaded(name)

	payload := strings.Repeat("A", certificateUploadLimitBytes+certificateUploadLimitBytes/10)
	body := map[string]any{
		"name":        name,
		"usage":       usage,
		"certificate": payload,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode oversized certificate upload body: %w", err)
	}

	m.httpSteps.SetHeader("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	return m.httpSteps.SendToServiceWithClient(client, http.MethodPost, "gateway-controller", "/certificates", bodyBytes)
}

// deployWithFixtureValues expands {{thumbprint "name"}} markers and deploys
// the configuration like the plain deploy step.
func (m *mtlsSteps) deployWithFixtureValues(body *godog.DocString) error {
	resolved, err := m.resolveThumbprintTemplates(body.Content)
	if err != nil {
		return err
	}
	return deployAPIConfiguration(m.state, m.httpSteps, resolved)
}

// updateWithFixtureValues expands {{thumbprint "name"}} markers and updates
// the API like the plain update step.
func (m *mtlsSteps) updateWithFixtureValues(apiName string, body *godog.DocString) error {
	resolved, err := m.resolveThumbprintTemplates(body.Content)
	if err != nil {
		return err
	}
	return updateAPIConfiguration(m.state, m.httpSteps, apiName, resolved)
}

// currentList parses the certificates array from the last HTTP response. It
// does not cache, because scenarios interleave list requests with other calls.
func (m *mtlsSteps) currentList() ([]map[string]any, error) {
	var parsed struct {
		Certificates []map[string]any `json:"certificates"`
	}
	if err := json.Unmarshal(m.httpSteps.LastBody(), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse certificate list response: %w", err)
	}
	return parsed.Certificates, nil
}

func (m *mtlsSteps) findInList(name string) (map[string]any, error) {
	list, err := m.currentList()
	if err != nil {
		return nil, err
	}
	for _, item := range list {
		if fmt.Sprint(item["name"]) == name {
			return item, nil
		}
	}
	return nil, nil
}

func (m *mtlsSteps) listShouldContain(name string) error {
	item, err := m.findInList(name)
	if err != nil {
		return err
	}
	if item == nil {
		list, _ := m.currentList()
		return fmt.Errorf("expected certificate list to contain %q, got %d entries", name, len(list))
	}
	return nil
}

func (m *mtlsSteps) listShouldNotContain(name string) error {
	item, err := m.findInList(name)
	if err != nil {
		return err
	}
	if item != nil {
		return fmt.Errorf("expected certificate list to not contain %q", name)
	}
	return nil
}

func (m *mtlsSteps) listedCertShouldHaveFieldEqualTo(name, field, value string) error {
	item, err := m.findInList(name)
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("certificate %q not found in last list response", name)
	}
	actual, ok := item[field]
	if !ok {
		return fmt.Errorf("certificate %q has no field %q", name, field)
	}
	actualStr := fmt.Sprint(actual)
	if actualStr != value {
		return fmt.Errorf("expected certificate %q field %q to equal %q, got %q", name, field, value, actualStr)
	}
	return nil
}

func (m *mtlsSteps) listedCertShouldNotHaveField(name, field string) error {
	item, err := m.findInList(name)
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("certificate %q not found in last list response", name)
	}
	if _, ok := item[field]; ok {
		return fmt.Errorf("expected certificate %q to not have field %q, got %v", name, field, item[field])
	}
	return nil
}

func (m *mtlsSteps) warningsOf(item map[string]any) []map[string]any {
	raw, ok := item["warnings"]
	if !ok || raw == nil {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	var out []map[string]any
	for _, w := range arr {
		if entry, ok := w.(map[string]any); ok {
			out = append(out, entry)
		}
	}
	return out
}

func (m *mtlsSteps) listedCertShouldHaveWarningWithCode(name, code string) error {
	item, err := m.findInList(name)
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("certificate %q not found in last list response", name)
	}
	for _, w := range m.warningsOf(item) {
		if fmt.Sprint(w["code"]) == code {
			return nil
		}
	}
	return fmt.Errorf("certificate %q has no warning with code %q, got %v", name, code, item["warnings"])
}

func (m *mtlsSteps) listedCertShouldHaveWarningWithField(name, field string) error {
	item, err := m.findInList(name)
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("certificate %q not found in last list response", name)
	}
	for _, w := range m.warningsOf(item) {
		if fmt.Sprint(w["field"]) == field {
			return nil
		}
	}
	return fmt.Errorf("certificate %q has no warning with field %q, got %v", name, field, item["warnings"])
}

func (m *mtlsSteps) listedCertShouldHaveNoWarnings(name string) error {
	item, err := m.findInList(name)
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("certificate %q not found in last list response", name)
	}
	if warnings := m.warningsOf(item); len(warnings) != 0 {
		return fmt.Errorf("expected certificate %q to have no warnings, got %v", name, item["warnings"])
	}
	return nil
}

func (m *mtlsSteps) validationErrors() ([]map[string]any, error) {
	var parsed struct {
		Errors []map[string]any `json:"errors"`
	}
	if err := json.Unmarshal(m.httpSteps.LastBody(), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse validation error response: %w", err)
	}
	return parsed.Errors, nil
}

func (m *mtlsSteps) validationErrorWithMessage(field, message string) error {
	message, err := m.resolveNotAfterTemplates(message)
	if err != nil {
		return err
	}
	errs, err := m.validationErrors()
	if err != nil {
		return err
	}
	for _, e := range errs {
		if fmt.Sprint(e["field"]) == field && fmt.Sprint(e["message"]) == message {
			return nil
		}
	}
	return fmt.Errorf("no validation error for field %q with message %q found in %v", field, message, errs)
}

func (m *mtlsSteps) validationErrorContaining(field, substr string) error {
	errs, err := m.validationErrors()
	if err != nil {
		return err
	}
	for _, e := range errs {
		if fmt.Sprint(e["field"]) == field && strings.Contains(fmt.Sprint(e["message"]), substr) {
			return nil
		}
	}
	return fmt.Errorf("no validation error for field %q containing %q found in %v", field, substr, errs)
}

func (m *mtlsSteps) validationErrorAny(field string) error {
	errs, err := m.validationErrors()
	if err != nil {
		return err
	}
	for _, e := range errs {
		if fmt.Sprint(e["field"]) == field {
			return nil
		}
	}
	return fmt.Errorf("no validation error for field %q found in %v", field, errs)
}

func (m *mtlsSteps) jsonFieldShouldBeSubjectOfFixture(field, fixture string) error {
	var parsed map[string]any
	if err := json.Unmarshal(m.httpSteps.LastBody(), &parsed); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}
	actual, ok := parsed[field]
	if !ok {
		return fmt.Errorf("JSON response has no field %q", field)
	}

	cert, err := m.parseFixtureCert(fixture)
	if err != nil {
		return err
	}
	expected := cert.Subject.String()

	actualStr := fmt.Sprint(actual)
	if actualStr != expected {
		return fmt.Errorf("expected field %q to be the subject of fixture %q (%q), got %q", field, fixture, expected, actualStr)
	}
	return nil
}

// clientAuthorityPoolIsEmpty deletes every usage: downstream certificate as the
// current user. A 404 is ignored. A 409, a reference that a configuration
// deleted by this or the previous scenario can still hold, is retried until
// certificateDeleteTimeout, first after those configurations have left the
// controller's store. Any other non-2xx status fails.
func (m *mtlsSteps) clientAuthorityPoolIsEmpty() error {
	if err := m.httpSteps.SendGETToService("gateway-controller", "/certificates?usage=downstream"); err != nil {
		return err
	}
	var parsed struct {
		Certificates []map[string]any `json:"certificates"`
	}
	if err := json.Unmarshal(m.httpSteps.LastBody(), &parsed); err != nil {
		return fmt.Errorf("failed to parse certificate list while emptying the client authority pool: %w", err)
	}

	storeConverged := false
	for _, item := range parsed.Certificates {
		id := fmt.Sprint(item["id"])
		deadline := time.Now().Add(certificateDeleteTimeout)
		for {
			if err := m.httpSteps.SendDELETEToService("gateway-controller", "/certificates/"+id); err != nil {
				return err
			}
			resp := m.httpSteps.LastResponse()
			if resp == nil || resp.StatusCode != http.StatusConflict || time.Now().After(deadline) {
				break
			}
			if !storeConverged {
				m.waitForDeletedConfigsToLeaveController(append(append([]string{}, m.previousConfigNames...), scenarioConfigNames(m.state)...))
				storeConverged = true
			} else {
				time.Sleep(cleanupReferencePollInterval * 4)
			}
		}
		resp := m.httpSteps.LastResponse()
		if resp != nil && resp.StatusCode != http.StatusNotFound && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
			return fmt.Errorf("failed to delete certificate %q (id %s) while emptying the client authority pool: status %d", item["name"], id, resp.StatusCode)
		}
		m.notePoolChangeIfAccepted("downstream")
	}
	return nil
}

// responseWarnings reads the warnings array from the last response, under
// either "status.warnings" or top-level "warnings".
func (m *mtlsSteps) responseWarnings() ([]map[string]any, error) {
	var parsed map[string]any
	if err := json.Unmarshal(m.httpSteps.LastBody(), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response while inspecting warnings: %w", err)
	}
	if status, ok := parsed["status"].(map[string]any); ok {
		if raw, ok := status["warnings"].([]any); ok {
			return toWarningEntries(raw), nil
		}
	}
	if raw, ok := parsed["warnings"].([]any); ok {
		return toWarningEntries(raw), nil
	}
	return nil, nil
}

func toWarningEntries(raw []any) []map[string]any {
	var out []map[string]any
	for _, w := range raw {
		if entry, ok := w.(map[string]any); ok {
			out = append(out, entry)
		}
	}
	return out
}

func (m *mtlsSteps) responseShouldIncludeWarningWithCodeForField(code, field string) error {
	warnings, err := m.responseWarnings()
	if err != nil {
		return err
	}
	for _, w := range warnings {
		if fmt.Sprint(w["code"]) == code && fmt.Sprint(w["field"]) == field {
			return nil
		}
	}
	return fmt.Errorf("no warning with code %q for field %q found in %v", code, field, warnings)
}

// responseShouldIncludeWarningWithCode checks for a warning code regardless
// of field.
func (m *mtlsSteps) responseShouldIncludeWarningWithCode(code string) error {
	warnings, err := m.responseWarnings()
	if err != nil {
		return err
	}
	for _, w := range warnings {
		if fmt.Sprint(w["code"]) == code {
			return nil
		}
	}
	return fmt.Errorf("no warning with code %q found in %v", code, warnings)
}

func (m *mtlsSteps) responseShouldIncludeNoWarnings() error {
	warnings, err := m.responseWarnings()
	if err != nil {
		return err
	}
	if len(warnings) != 0 {
		return fmt.Errorf("expected no warnings, got %v", warnings)
	}
	return nil
}

// findCertificateByName lists the certificates and returns the entry named
// name.
func (m *mtlsSteps) findCertificateByName(name string) (map[string]any, error) {
	if err := m.httpSteps.SendGETToService("gateway-controller", "/certificates"); err != nil {
		return nil, err
	}
	var parsed struct {
		Certificates []map[string]any `json:"certificates"`
	}
	if err := json.Unmarshal(m.httpSteps.LastBody(), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse certificate list while resolving %q: %w", name, err)
	}
	for _, item := range parsed.Certificates {
		if fmt.Sprint(item["name"]) == name {
			return item, nil
		}
	}
	return nil, fmt.Errorf("no certificate named %q found", name)
}

func (m *mtlsSteps) findCertificateIDByName(name string) (string, error) {
	item, err := m.findCertificateByName(name)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(item["id"]), nil
}

func (m *mtlsSteps) deleteCertificateNamed(name string) error {
	item, err := m.findCertificateByName(name)
	if err != nil {
		return fmt.Errorf("failed to delete certificate %q: %w", name, err)
	}
	return m.deleteListedCertificate(item)
}

// certificateReferenceTimeout and certificateReferencePollInterval bound
// deleteCertificateNamedOnceUnreferenced.
const (
	certificateReferenceTimeout      = 5 * time.Second
	certificateReferencePollInterval = 100 * time.Millisecond
)

// deleteCertificateNamedOnceUnreferenced polls the listing until the named
// certificate's referencedByApis is 0, then deletes it.
func (m *mtlsSteps) deleteCertificateNamedOnceUnreferenced(name string) error {
	deadline := time.Now().Add(certificateReferenceTimeout)
	for {
		item, err := m.findCertificateByName(name)
		if err != nil {
			return fmt.Errorf("failed to delete certificate %q: %w", name, err)
		}
		refs, ok := item["referencedByApis"].(float64)
		if !ok {
			return fmt.Errorf("certificate %q is listed without a referencedByApis count", name)
		}
		if refs == 0 {
			return m.deleteListedCertificate(item)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("certificate %q was still referenced by %v APIs after %s", name, refs, certificateReferenceTimeout)
		}
		time.Sleep(certificateReferencePollInterval)
	}
}

// deleteListedCertificate deletes a certificate entry from a listing.
func (m *mtlsSteps) deleteListedCertificate(item map[string]any) error {
	if err := m.httpSteps.SendDELETEToService("gateway-controller", "/certificates/"+fmt.Sprint(item["id"])); err != nil {
		return err
	}
	m.notePoolChangeIfAccepted(fmt.Sprint(item["usage"]))
	return nil
}
