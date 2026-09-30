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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Header encodings a front proxy might use for a relayed client certificate.
const (
	headerCertEncodingURL    = "url"    // default: the PEM text, URL-path-escaped
	headerCertEncodingPEM    = "pem"    // raw PEM text, newlines replaced by spaces
	headerCertEncodingBase64 = "base64" // bare base64 of the DER bytes, no PEM armor
)

// encodeCertificateForHeader renders the fixture certificate the way a front
// proxy would place it in a header. An empty encoding means "url".
func (m *mtlsSteps) encodeCertificateForHeader(name, encoding string) (string, error) {
	certPEM, err := m.readFixtureCert(name)
	if err != nil {
		return "", err
	}
	switch encoding {
	case "", headerCertEncodingURL:
		return url.PathEscape(string(certPEM)), nil
	case headerCertEncodingPEM:
		// HTTP headers cannot carry a literal newline; several proxies emit
		// the PEM text with newlines replaced by spaces instead of encoding it.
		return strings.ReplaceAll(string(certPEM), "\n", " "), nil
	case headerCertEncodingBase64:
		cert, err := m.parseFixtureCert(name)
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(cert.Raw), nil
	default:
		return "", fmt.Errorf("unknown certificate header encoding %q", encoding)
	}
}

// useHeaderForOneRequest sets a header for the next request and returns a
// function that restores the previous value, so it never leaks into a later
// request.
func (m *mtlsSteps) useHeaderForOneRequest(name, value string) func() {
	previous := m.httpSteps.Header(name)
	m.httpSteps.SetHeader(name, value)
	return func() {
		if previous != "" {
			m.httpSteps.SetHeader(name, previous)
		} else {
			m.httpSteps.RemoveHeader(name)
		}
	}
}

// getWithClientCertificateAndHeaderCertificate sends a request presenting
// certName as the TLS client certificate while relaying certFixture, encoded
// the default way (url), in headerName.
func (m *mtlsSteps) getWithClientCertificateAndHeaderCertificate(reqURL, certName, headerName, certFixture string) error {
	return m.getWithClientCertificateAndHeaderCertificateEncoded(reqURL, certName, headerName, certFixture, "")
}

// getWithClientCertificateAndHeaderCertificateEncoded is the same request
// with an explicit header encoding ("url", "pem", or "base64").
func (m *mtlsSteps) getWithClientCertificateAndHeaderCertificateEncoded(reqURL, certName, headerName, certFixture, encoding string) error {
	encoded, err := m.encodeCertificateForHeader(certFixture, encoding)
	if err != nil {
		return err
	}
	restore := m.useHeaderForOneRequest(headerName, encoded)
	defer restore()
	return m.getWithClientCertificate(reqURL, certName)
}

// getWithNoClientCertificateAndHeaderCertificate presents no TLS client
// certificate while relaying certFixture in headerName, so only the header
// claims an identity.
func (m *mtlsSteps) getWithNoClientCertificateAndHeaderCertificate(reqURL, headerName, certFixture string) error {
	encoded, err := m.encodeCertificateForHeader(certFixture, "")
	if err != nil {
		return err
	}
	restore := m.useHeaderForOneRequest(headerName, encoded)
	defer restore()
	return m.getWithNoClientCertificate(reqURL)
}

// getWithHeaderCertificate relays certFixture in headerName over plain HTTP
// or over HTTPS without a client certificate.
func (m *mtlsSteps) getWithHeaderCertificate(reqURL, headerName, certFixture string) error {
	return m.getWithNoClientCertificateAndHeaderCertificate(reqURL, headerName, certFixture)
}

// xfccHashPattern reads the Hash element of an x-forwarded-client-cert value.
var xfccHashPattern = regexp.MustCompile(`(?i)(?:^|;)Hash=([0-9a-f]{64})(?:;|$)`)

// echoedXFCC returns the x-forwarded-client-cert value the echo backend
// received, or "" when it received none.
func (m *mtlsSteps) echoedXFCC() (string, error) {
	var data struct {
		Headers map[string]interface{} `json:"headers"`
	}
	if err := json.Unmarshal(m.httpSteps.LastBody(), &data); err != nil {
		return "", fmt.Errorf("failed to parse the echo backend response: %w", err)
	}
	for key, value := range data.Headers {
		if strings.EqualFold(key, "x-forwarded-client-cert") {
			return fmt.Sprintf("%v", value), nil
		}
	}
	return "", nil
}

// echoedXFCCShouldName asserts the backend's x-forwarded-client-cert
// describes the fixture certificate: its Hash is the fixture's thumbprint
// and its Subject carries the fixture's common name.
func (m *mtlsSteps) echoedXFCCShouldName(fixture string) error {
	xfcc, err := m.echoedXFCC()
	if err != nil {
		return err
	}
	if xfcc == "" {
		return fmt.Errorf("expected the backend to receive x-forwarded-client-cert naming %q, got none", fixture)
	}
	cert, err := m.parseFixtureCert(fixture)
	if err != nil {
		return err
	}
	thumbprint, err := m.thumbprintOf(fixture)
	if err != nil {
		return err
	}
	match := xfccHashPattern.FindStringSubmatch(xfcc)
	if match == nil || !strings.EqualFold(match[1], thumbprint) {
		return fmt.Errorf("expected x-forwarded-client-cert Hash to be the thumbprint of %q (%s), got %q", fixture, thumbprint, xfcc)
	}
	if !strings.Contains(xfcc, "CN="+cert.Subject.CommonName) {
		return fmt.Errorf("expected x-forwarded-client-cert Subject to name CN=%s, got %q", cert.Subject.CommonName, xfcc)
	}
	return nil
}

// echoedXFCCShouldNotName asserts the backend's x-forwarded-client-cert, if
// any, does not describe the fixture certificate.
func (m *mtlsSteps) echoedXFCCShouldNotName(fixture string) error {
	xfcc, err := m.echoedXFCC()
	if err != nil {
		return err
	}
	thumbprint, err := m.thumbprintOf(fixture)
	if err != nil {
		return err
	}
	if match := xfccHashPattern.FindStringSubmatch(xfcc); match != nil && strings.EqualFold(match[1], thumbprint) {
		return fmt.Errorf("expected x-forwarded-client-cert not to name %q, got %q", fixture, xfcc)
	}
	return nil
}
