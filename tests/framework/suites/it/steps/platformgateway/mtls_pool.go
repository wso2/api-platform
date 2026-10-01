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

package platformgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/testpki"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// certificateUploadLimitBytes is the controller's default certificate upload limit, which the
// suite's gateway configuration leaves unset.
const certificateUploadLimitBytes = 1 << 20

// Context keys for a fixture's certificate and key, JSON-string escaped so a request body can
// embed them, and for its subject as the controller renders it.
func fixturePEMKey(fixture string) string     { return "fixture." + fixture + ".pem" }
func fixtureKeyPEMKey(fixture string) string  { return "fixture." + fixture + ".key" }
func fixtureSubjectKey(fixture string) string { return "fixture." + fixture + ".subject" }

// fixtureEncryptedKeyPEMKey names the passphrase-protected form of a fixture's key.
func fixtureEncryptedKeyPEMKey(fixture string) string { return "fixture." + fixture + ".encryptedKey" }

func (g *Gateway) registerMTLSPoolSteps(sc *godog.ScenarioContext) {
	sc.Before(publishFixtureMaterial)
	sc.Step(`^I upload the certificate fixture "([^"]*)" as "([^"]*)"$`, g.uploadUpstreamFixture)
	sc.Step(`^I upload the certificate fixtures? "([^"]*)" as "([^"]*)" with usage "([^"]*)" and role "([^"]*)"(?: and dns SAN "([^"]*)")?$`,
		g.uploadFixtureWithRole)
	sc.Step(`^I upload to the certificates endpoint the body:$`, g.uploadCertificateBody)
	sc.Step(`^I upload a certificate body a tenth over the upload limit as "([^"]*)" with usage "([^"]*)"$`,
		g.uploadOversizedCertificate)
	sc.Step(`^I delete the certificate named "([^"]*)"$`, g.deleteCertificateNamed)
	sc.Step(`^I delete the certificate named "([^"]*)" once no API references it$`, g.deleteCertificateOnceUnreferenced)
	sc.Step(`^the client authority pool is empty$`, g.requireCleanGateway)
	sc.Step(`^the certificate list should (contain|not contain) "([^"]*)"$`, certificateListContains)
	sc.Step(`^the listed certificate "([^"]*)" should have "([^"]*)" equal to ("[^"]*"|true|false|-?\d+)$`,
		listedCertificateFieldEquals)
	sc.Step(`^the listed certificate "([^"]*)" should not have field "([^"]*)"$`, listedCertificateLacksField)
	sc.Step(`^the listed certificate "([^"]*)" should have a warning with (code|field) "([^"]*)"$`,
		listedCertificateHasWarning)
	sc.Step(`^the listed certificate "([^"]*)" should have no warnings$`, listedCertificateHasNoWarnings)
	sc.Step(`^the response should list a validation error for field "([^"]*)"(?: (with message|containing) "([^"]*)")?$`,
		validationErrorListed)
}

// publishFixtureMaterial publishes every fixture's certificate, key and subject for an @mtls
// scenario, so request bodies and expectations can name them as context values.
func publishFixtureMaterial(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
	if sc == nil || !hasTag(scenarioTags(sc), tagMTLS) {
		return ctx, nil
	}
	fixtures, err := testpki.Default()
	if err != nil {
		return ctx, err
	}
	for _, name := range fixtures.Names() {
		fixture, _ := fixtures.Get(name)
		values := map[string]string{
			fixturePEMKey(name):     jsonEscaped(fixture.CertPEM),
			fixtureKeyPEMKey(name):  jsonEscaped(fixture.KeyPEM),
			fixtureSubjectKey(name): fixture.Certificate.Subject.String(),
		}
		if len(fixture.KeyPEM) > 0 {
			encrypted, err := fixture.EncryptedKeyPEM(encryptedKeyPassphrase)
			if err != nil {
				return ctx, err
			}
			values[fixtureEncryptedKeyPEMKey(name)] = jsonEscaped(encrypted)
		}
		for key, value := range values {
			if err := tcontext.Set(ctx, key, value); err != nil {
				return ctx, err
			}
		}
	}
	return ctx, nil
}

// jsonEscaped returns b as the contents of a JSON string, without the surrounding quotes.
func jsonEscaped(b []byte) string {
	encoded, _ := json.Marshal(string(b))
	return string(encoded[1 : len(encoded)-1])
}

// ── Uploads ────────────────────────────────────────────────────────────────────

func (g *Gateway) uploadUpstreamFixture(ctx context.Context, fixture, name string) error {
	_, err := g.uploadFixture(ctx, fixture, name, nil)
	return err
}

// uploadFixtureWithRole uploads a fixture with a usage and role, narrowed to one DNS SAN when
// dnsSAN is given.
func (g *Gateway) uploadFixtureWithRole(ctx context.Context, fixture, name, usage, role, dnsSAN string) error {
	fields := map[string]any{"usage": usage, "role": role}
	if dnsSAN != "" {
		fields["match"] = map[string][]string{"dnsSANs": {dnsSAN}}
	}
	_, err := g.uploadFixture(ctx, fixture, name, fields)
	return err
}

// uploadFixture uploads one fixture under a generated name with the given extra fields.
func (g *Gateway) uploadFixture(ctx context.Context, fixture, nameExpr string, fields map[string]any) (*httpx.Response, error) {
	name, err := g.generatedCertificateName(ctx, nameExpr)
	if err != nil {
		return nil, err
	}
	pemBundle, err := fixturePEMBundle(fixture)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"name": name, "certificate": pemBundle}
	for k, v := range fields {
		body[k] = v
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encoding the certificate upload: %w", err)
	}
	return g.postCertificate(ctx, name, payload)
}

// uploadCertificateBody uploads a JSON body as written, after expanding context values. The
// body is sent unchanged so a scenario can send fields and names the controller must refuse.
func (g *Gateway) uploadCertificateBody(ctx context.Context, doc *godog.DocString) error {
	body, err := stepscommon.Expand(ctx, doc.Content)
	if err != nil {
		return err
	}
	var named struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal([]byte(body), &named)
	_, err = g.postCertificate(ctx, named.Name, []byte(body))
	return err
}

// uploadOversizedCertificate uploads a body a tenth over the controller's upload limit.
func (g *Gateway) uploadOversizedCertificate(ctx context.Context, nameExpr, usage string) error {
	name, err := g.generatedCertificateName(ctx, nameExpr)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{
		"name":        name,
		"usage":       usage,
		"certificate": strings.Repeat("A", certificateUploadLimitBytes+certificateUploadLimitBytes/10),
	})
	if err != nil {
		return fmt.Errorf("encoding the oversized certificate upload: %w", err)
	}
	_, err = g.postCertificate(ctx, name, payload)
	return err
}

// postCertificate posts a certificate upload as the scenario's caller, publishes the response
// and registers an accepted certificate for cleanup at once.
func (g *Gateway) postCertificate(ctx context.Context, name string, payload []byte) (*httpx.Response, error) {
	markGatewayChanged(ctx)
	url, err := g.serviceURL(ctx, "gateway-controller", "/certificates")
	if err != nil {
		return nil, err
	}
	resp, err := g.funnel.Post(ctx, url, g.headerWith(ctx, "Content-Type", "application/json"), payload)
	if err != nil {
		return nil, err
	}
	if !resp.Succeeded() {
		return resp, nil
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp.Body, &created); err != nil || created.ID == "" {
		return resp, fmt.Errorf("certificate %q was accepted without an id: %s", name, resp.Describe())
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindCertificate, ID: created.ID, Actor: "admin",
		Description: "certificate " + name + " uploaded by " + scenarioLabel(ctx),
	}); err != nil {
		deleteURL, urlErr := g.serviceURL(ctx, "gateway-controller", "/certificates/"+created.ID)
		if urlErr == nil {
			if deleteErr := g.compensateDelete(ctx, deleteURL, g.scenarioHeaders(ctx)); deleteErr != nil {
				return resp, fmt.Errorf("registering certificate %q for cleanup: %w; compensation failed: %v", name, err, deleteErr)
			}
		}
		return resp, fmt.Errorf("registering certificate %q for cleanup: %w", name, err)
	}
	return resp, nil
}

// ── Deletes ────────────────────────────────────────────────────────────────────

// storedCertificate is one entry of the controller's certificate listing.
type storedCertificate struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	ReferencedByAPIs *int   `json:"referencedByApis"`
}

// certificateNamed finds a certificate by name in the controller's full listing, read as admin
// so the lookup never depends on the caller whose delete is under test.
func (g *Gateway) certificateNamed(ctx context.Context, name string) (storedCertificate, error) {
	var listing struct {
		Certificates []storedCertificate `json:"certificates"`
	}
	if err := g.readJSON(ctx, "gateway-controller", "/certificates", &listing); err != nil {
		return storedCertificate{}, err
	}
	for _, c := range listing.Certificates {
		if c.Name == name {
			return c, nil
		}
	}
	return storedCertificate{}, fmt.Errorf("no certificate named %q is stored", name)
}

func (g *Gateway) deleteCertificateNamed(ctx context.Context, nameExpr string) error {
	name, err := stepscommon.Expand(ctx, nameExpr)
	if err != nil {
		return err
	}
	stored, err := g.certificateNamed(ctx, name)
	if err != nil {
		return fmt.Errorf("deleting certificate %q: %w", name, err)
	}
	return g.deleteCertificate(ctx, stored)
}

// deleteCertificateOnceUnreferenced waits until the controller counts no API naming the
// certificate, then deletes it. A count above zero is the only state it waits through.
func (g *Gateway) deleteCertificateOnceUnreferenced(ctx context.Context, nameExpr string) error {
	name, err := stepscommon.Expand(ctx, nameExpr)
	if err != nil {
		return err
	}
	var stored storedCertificate
	err = awaitReadState(ctx, fmt.Sprintf("waiting for no API to reference certificate %q", name),
		func(ctx context.Context) error {
			c, lookupErr := g.certificateNamed(ctx, name)
			if lookupErr != nil {
				return lookupErr
			}
			if c.ReferencedByAPIs == nil {
				return fmt.Errorf("certificate %q is listed without a referencedByApis count", name)
			}
			if *c.ReferencedByAPIs > 0 {
				return tolerated("certificate %q is still referenced by %d APIs", name, *c.ReferencedByAPIs)
			}
			stored = c
			return nil
		})
	if err != nil {
		return err
	}
	return g.deleteCertificate(ctx, stored)
}

// deleteCertificate deletes a stored certificate as the scenario's caller, publishes the
// response and, once it is gone, drops it from cleanup.
func (g *Gateway) deleteCertificate(ctx context.Context, stored storedCertificate) error {
	markGatewayChanged(ctx)
	url, err := g.serviceURL(ctx, "gateway-controller", "/certificates/"+stored.ID)
	if err != nil {
		return err
	}
	resp, err := g.funnel.Delete(ctx, url, g.scenarioHeaders(ctx))
	if err != nil {
		return err
	}
	if resp.Succeeded() {
		if reg, regErr := cleanup.Of(ctx); regErr == nil {
			reg.Deregister(cleanup.KindCertificate, stored.ID)
		}
	}
	return nil
}

// ── Assertions on the published response ───────────────────────────────────────

// publishedCertificates returns the certificates array of the published listing. A body whose
// certificates is not an array, or that carries no totalCount, is not a listing.
func publishedCertificates(ctx context.Context) ([]map[string]any, *httpx.Response, error) {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return nil, nil, err
	}
	var listing struct {
		Certificates *[]map[string]any `json:"certificates"`
		TotalCount   *int              `json:"totalCount"`
	}
	if err := json.Unmarshal(resp.Body, &listing); err != nil || listing.Certificates == nil || listing.TotalCount == nil {
		return nil, resp, fmt.Errorf("the published response is not a certificate listing: %s", resp.Describe())
	}
	return *listing.Certificates, resp, nil
}

// listedCertificate returns the named entry of the published listing, failing when it is absent.
func listedCertificate(ctx context.Context, nameExpr string) (map[string]any, string, error) {
	name, err := stepscommon.Expand(ctx, nameExpr)
	if err != nil {
		return nil, "", err
	}
	certs, resp, err := publishedCertificates(ctx)
	if err != nil {
		return nil, name, err
	}
	for _, c := range certs {
		if c["name"] == name {
			return c, name, nil
		}
	}
	return nil, name, fmt.Errorf("certificate %q is not in the listing: %s", name, resp.Describe())
}

func certificateListContains(ctx context.Context, mode, nameExpr string) error {
	name, err := stepscommon.Expand(ctx, nameExpr)
	if err != nil {
		return err
	}
	certs, resp, err := publishedCertificates(ctx)
	if err != nil {
		return err
	}
	found := false
	for _, c := range certs {
		if c["name"] == name {
			found = true
			break
		}
	}
	if found != (mode == "contain") {
		return fmt.Errorf("expected the certificate listing to %s %q: %s", mode, name, resp.Describe())
	}
	return nil
}

// listedCertificateFieldEquals compares a listed field with a JSON value: a quoted string, a
// boolean or an integer. The type must match as well as the value.
func listedCertificateFieldEquals(ctx context.Context, nameExpr, field, raw string) error {
	cert, name, err := listedCertificate(ctx, nameExpr)
	if err != nil {
		return err
	}
	expanded, err := stepscommon.Expand(ctx, raw)
	if err != nil {
		return err
	}
	var want any
	if err := json.Unmarshal([]byte(expanded), &want); err != nil {
		return fmt.Errorf("expected value %s is not a JSON string, boolean or number: %w", raw, err)
	}
	got, ok := cert[field]
	if !ok {
		return fmt.Errorf("certificate %q is listed without field %q: %v", name, field, cert)
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("certificate %q field %q: expected %v (%T), got %v (%T)", name, field, want, want, got, got)
	}
	return nil
}

func listedCertificateLacksField(ctx context.Context, nameExpr, field string) error {
	cert, name, err := listedCertificate(ctx, nameExpr)
	if err != nil {
		return err
	}
	if v, ok := cert[field]; ok {
		return fmt.Errorf("expected certificate %q to be listed without field %q, got %v", name, field, v)
	}
	return nil
}

// certificateWarnings returns the entry's warnings; a missing warnings field is none.
func certificateWarnings(cert map[string]any) []map[string]any {
	raw, _ := cert["warnings"].([]any)
	var out []map[string]any
	for _, w := range raw {
		if entry, ok := w.(map[string]any); ok {
			out = append(out, entry)
		}
	}
	return out
}

func listedCertificateHasWarning(ctx context.Context, nameExpr, key, value string) error {
	cert, name, err := listedCertificate(ctx, nameExpr)
	if err != nil {
		return err
	}
	for _, w := range certificateWarnings(cert) {
		if w[key] == value {
			return nil
		}
	}
	return fmt.Errorf("certificate %q has no warning with %s %q, got %v", name, key, value, cert["warnings"])
}

func listedCertificateHasNoWarnings(ctx context.Context, nameExpr string) error {
	cert, name, err := listedCertificate(ctx, nameExpr)
	if err != nil {
		return err
	}
	if warnings := certificateWarnings(cert); len(warnings) > 0 {
		return fmt.Errorf("expected certificate %q to have no warnings, got %v", name, cert["warnings"])
	}
	return nil
}

// validationErrorListed asserts the published error response lists an error for field, with
// exactly the given message, a message containing the given text, or any message.
func validationErrorListed(ctx context.Context, field, mode, text string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	expected, err := stepscommon.Expand(ctx, text)
	if err != nil {
		return err
	}
	if field, err = stepscommon.Expand(ctx, field); err != nil {
		return err
	}
	var body struct {
		Errors []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return fmt.Errorf("the published response is not a JSON error response: %w (%s)", err, resp.Describe())
	}
	for _, e := range body.Errors {
		if e.Field != field {
			continue
		}
		switch mode {
		case "with message":
			if e.Message == expected {
				return nil
			}
		case "containing":
			if strings.Contains(e.Message, expected) {
				return nil
			}
		default:
			return nil
		}
	}
	return fmt.Errorf("no validation error for field %q %s %q: %s", field, mode, expected, resp.Describe())
}
