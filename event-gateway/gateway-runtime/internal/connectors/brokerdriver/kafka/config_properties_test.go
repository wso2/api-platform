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

package kafka

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/config"
)

// globalKafkaConfig is a gateway whose own Kafka settings point somewhere else entirely. Every
// test here asserts the binding's own properties win, because the bug being fixed was exactly
// that they did not: nothing matched, and the connection silently used these instead.
func globalKafkaConfig() config.KafkaConfig {
	return config.KafkaConfig{
		Brokers:       []string{"global-broker:9092"},
		SASLMechanism: "plain",
		SASLUsername:  "global-user",
		SASLPassword:  "global-pass",
	}
}

// clientPropertySASLPlaintext is the most common connection written in Kafka client property
// names: a SASL/PLAIN login over an unencrypted transport. Every value is a one-element list,
// and every name is the client property name rather than this driver's option name.
func clientPropertySASLPlaintext() map[string]interface{} {
	return map[string]interface{}{
		"brokers":           []interface{}{"tenant-broker:9092"},
		"security.protocol": []interface{}{"SASL_PLAINTEXT"},
		"sasl.mechanism":    []interface{}{"PLAIN"},
		"sasl.username":     []interface{}{"tenant-user"},
		"sasl.password":     []interface{}{"tenant-secret"},
	}
}

func TestResolveConnectionConfig_AcceptsClientPropertyNamesForSASL(t *testing.T) {

	resolved, err := ResolveConnectionConfig(globalKafkaConfig(), clientPropertySASLPlaintext())
	if err != nil {
		t.Fatalf("ResolveConnectionConfig returned error: %v", err)
	}

	if !reflect.DeepEqual(resolved.Brokers, []string{"tenant-broker:9092"}) {
		t.Errorf("brokers = %v, want the binding's own", resolved.Brokers)
	}
	if resolved.SASLUsername != "tenant-user" {
		t.Errorf("sasl username = %q, want tenant-user", resolved.SASLUsername)
	}
	if resolved.SASLPassword != "tenant-secret" {
		t.Errorf("sasl password was not taken from the binding; the gateway would authenticate as itself")
	}
	if resolved.SASLMechanism != "plain" {
		t.Errorf("sasl mechanism = %q, want plain", resolved.SASLMechanism)
	}
	if resolved.TLS {
		t.Errorf("SASL_PLAINTEXT must not turn TLS on")
	}
}

// A gateway configured for TLS must still honour a binding that asks for a plaintext transport:
// the protocol the user chose describes their broker, not the gateway's default.
func TestResolveConnectionConfig_PlaintextProtocolTurnsOffGlobalTLS(t *testing.T) {

	global := globalKafkaConfig()
	global.TLS = true

	resolved, err := ResolveConnectionConfig(global, clientPropertySASLPlaintext())
	if err != nil {
		t.Fatalf("ResolveConnectionConfig returned error: %v", err)
	}
	if resolved.TLS {
		t.Errorf("security.protocol SASL_PLAINTEXT must turn TLS off")
	}
}

func TestResolveConnectionConfig_AcceptsInlineTrustStoreCertificates(t *testing.T) {

	overrides := clientPropertySASLPlaintext()
	overrides["security.protocol"] = []interface{}{"SASL_SSL"}
	overrides["ssl.truststore.type"] = []interface{}{"PEM"}
	overrides["ssl.truststore.certificates"] = []interface{}{"-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----"}

	resolved, err := ResolveConnectionConfig(globalKafkaConfig(), overrides)
	if err != nil {
		t.Fatalf("ResolveConnectionConfig returned error: %v", err)
	}
	if !resolved.TLS {
		t.Errorf("security.protocol SASL_SSL must turn TLS on")
	}
	if !strings.Contains(resolved.TLSCAPEM, "BEGIN CERTIFICATE") {
		t.Errorf("the inline CA certificate was not carried through: %q", resolved.TLSCAPEM)
	}
	if resolved.TLSCAFile != "" {
		t.Errorf("inline material must not be mistaken for a file path")
	}
}

// A binding written against this driver's own option names must keep working untouched,
// including when a client-property spelling sits alongside it.
func TestResolveConnectionConfig_DriverOptionNamesWin(t *testing.T) {

	resolved, err := ResolveConnectionConfig(globalKafkaConfig(), map[string]interface{}{
		"brokers":        []interface{}{"tenant-broker:9092"},
		"sasl_mechanism": "plain",
		"sasl_username":  "driver-user",
		"sasl_password":  "driver-secret",
		"sasl.username":  []interface{}{"client-property-user"},
		"sasl.password":  []interface{}{"client-property-secret"},
	})
	if err != nil {
		t.Fatalf("ResolveConnectionConfig returned error: %v", err)
	}
	if resolved.SASLUsername != "driver-user" || resolved.SASLPassword != "driver-secret" {
		t.Errorf("the driver's own option names must win, got %q/%q",
			resolved.SASLUsername, resolved.SASLPassword)
	}
}

// Silently dropping one of these would leave a binding that looks configured and cannot
// authenticate - the failure this translation exists to end.
func TestResolveConnectionConfig_RejectsPropertiesItCannotHonour(t *testing.T) {

	cases := map[string]string{
		"sasl.kerberos.keytab":                "Kerberos",
		"sasl.oauthbearer.token.endpoint.url": "OAUTHBEARER",
		"ssl.key.password":                    "encrypted private key",
	}
	for property, want := range cases {
		t.Run(property, func(t *testing.T) {
			overrides := clientPropertySASLPlaintext()
			overrides[property] = []interface{}{"a-value"}

			_, err := ResolveConnectionConfig(globalKafkaConfig(), overrides)
			if err == nil {
				t.Fatalf("expected %q to be rejected", property)
			}
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error should say why: got %v", err)
			}
		})
	}
}

// The control plane blanks a credential it will not return. A key present but empty is not a
// configured value and must not be read as one.
func TestResolveConnectionConfig_IgnoresAnEmptyUnsupportedProperty(t *testing.T) {

	overrides := clientPropertySASLPlaintext()
	overrides["ssl.key.password"] = []interface{}{""}

	if _, err := ResolveConnectionConfig(globalKafkaConfig(), overrides); err != nil {
		t.Fatalf("an empty unsupported property must not fail the connection: %v", err)
	}
}

func TestResolveConnectionConfig_RejectsAMultiValuedScalarProperty(t *testing.T) {

	overrides := clientPropertySASLPlaintext()
	overrides["sasl.username"] = []interface{}{"one", "two"}

	_, err := ResolveConnectionConfig(globalKafkaConfig(), overrides)
	if err == nil || !strings.Contains(err.Error(), "single value") {
		t.Fatalf("expected a single-value error, got %v", err)
	}
}

func TestResolveConnectionConfig_RejectsAnUnknownSecurityProtocol(t *testing.T) {

	overrides := clientPropertySASLPlaintext()
	overrides["security.protocol"] = []interface{}{"SASL_WHATEVER"}

	_, err := ResolveConnectionConfig(globalKafkaConfig(), overrides)
	if err == nil || !strings.Contains(err.Error(), "security.protocol") {
		t.Fatalf("expected an unsupported protocol error, got %v", err)
	}
}

// A PEM keystore file holds the client certificate and its key together, so the one path
// supplied fills both of this driver's options.
func TestResolveConnectionConfig_KeyStoreLocationFillsCertificateAndKey(t *testing.T) {

	path := writeTempPEM(t)

	overrides := clientPropertySASLPlaintext()
	overrides["security.protocol"] = []interface{}{"SSL"}
	delete(overrides, "sasl.mechanism")
	delete(overrides, "sasl.username")
	delete(overrides, "sasl.password")
	overrides["sasl_mechanism"] = ""
	overrides["ssl.keystore.location"] = []interface{}{path}

	resolved, err := ResolveConnectionConfig(globalKafkaConfig(), overrides)
	if err != nil {
		t.Fatalf("ResolveConnectionConfig returned error: %v", err)
	}
	if resolved.TLSCertFile != path || resolved.TLSKeyFile != path {
		t.Errorf("keystore location must fill both certificate and key, got %q/%q",
			resolved.TLSCertFile, resolved.TLSKeyFile)
	}
}

// writeTempPEM is a readable file standing in for a PEM keystore; validation only checks that
// the path can be read, so its contents are never parsed here.
func writeTempPEM(t *testing.T) string {
	t.Helper()
	path := t.TempDir() + "/keystore.pem"
	if err := os.WriteFile(path, []byte("-----BEGIN CERTIFICATE-----\n"), 0o600); err != nil {
		t.Fatalf("failed to write the temporary keystore: %v", err)
	}
	return path
}
