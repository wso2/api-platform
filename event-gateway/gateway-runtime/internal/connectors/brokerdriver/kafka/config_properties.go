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
	"fmt"
	"strings"
)

// Broker properties reach this driver as a free-form map, and two different writers fill it.
// The control plane's own clients use this driver's option names — sasl_password and the rest.
// The Choreo console writes the names a Kafka client library uses — sasl.password, and it sends
// every value as a one-element list of strings rather than a scalar.
//
// Nothing between the two translates, so a connection configured in the console reached this
// driver with not one key it recognised, and it fell back to the gateway's own global Kafka
// settings. canonicalizeOverrides is that translation: it maps the client property names onto
// this driver's options and unwraps the one-element lists, leaving a map the rest of this file
// reads exactly as it always has.
//
// This driver's own names win where both spellings are present, so an existing binding keeps
// behaving as it did.

// consoleScalarAliases maps a Kafka client property name to this driver's option name. Only
// properties this driver can act on are listed; see unsupportedConsoleProperties for the ones
// that are rejected rather than ignored.
var consoleScalarAliases = map[string]string{
	"sasl.mechanism":                 "sasl_mechanism",
	"sasl.username":                  "sasl_username",
	"sasl.password":                  "sasl_password",
	"ssl.truststore.location":        "tls_ca_file",
	"ssl.truststore.certificates":    "tls_ca_pem",
	"ssl.keystore.certificate.chain": "tls_cert_pem",
	"ssl.keystore.key":               "tls_key_pem",
}

// unsupportedConsoleProperties are properties the console can write that this driver has no way
// to honour. They are rejected with a named error rather than ignored: silently dropping a
// Kerberos keytab or an encrypted key's passphrase would leave a connection that looks
// configured and cannot authenticate, which is the failure this whole translation exists to end.
var unsupportedConsoleProperties = map[string]string{
	"ssl.key.password":                    "an encrypted private key is not supported; supply an unencrypted PEM key",
	"sasl.kerberos.service.name":          "Kerberos (GSSAPI) is not supported by this driver",
	"sasl.kerberos.principal":             "Kerberos (GSSAPI) is not supported by this driver",
	"sasl.kerberos.keytab":                "Kerberos (GSSAPI) is not supported by this driver",
	"sasl.oauthbearer.token.endpoint.url": "OAUTHBEARER is not supported by this driver",
}

// canonicalizeOverrides returns the overrides with Kafka client property names translated to
// this driver's option names. The input map is not modified.
func canonicalizeOverrides(overrides map[string]interface{}) (map[string]interface{}, error) {
	if overrides == nil {
		return nil, nil
	}

	canonical := make(map[string]interface{}, len(overrides))
	for key, value := range overrides {
		canonical[key] = value
	}

	for property, reason := range unsupportedConsoleProperties {
		if present, err := hasValue(overrides[property]); err != nil {
			return nil, err
		} else if present {
			return nil, fmt.Errorf("kafka broker property %q is set but %s", property, reason)
		}
	}

	for property, option := range consoleScalarAliases {
		value, present, err := scalarValue(overrides[property], property)
		if err != nil {
			return nil, err
		}
		// This driver's own option name wins, so a binding that already carries one is
		// untouched by whatever the console wrote alongside it.
		if !present || canonical[option] != nil {
			continue
		}
		canonical[option] = value
	}

	if err := applySecurityProtocol(overrides, canonical); err != nil {
		return nil, err
	}

	// ssl.keystore.location names a PEM file holding the client certificate, and its key when
	// the key is not supplied inline. Go reads both out of the same file, so the one path fills
	// both options.
	location, present, err := scalarValue(overrides["ssl.keystore.location"], "ssl.keystore.location")
	if err != nil {
		return nil, err
	}
	if present && location != "" {
		if canonical["tls_cert_file"] == nil && canonical["tls_cert_pem"] == nil {
			canonical["tls_cert_file"] = location
		}
		if canonical["tls_key_file"] == nil && canonical["tls_key_pem"] == nil {
			canonical["tls_key_file"] = location
		}
	}

	// Unwrap any remaining one-element list on this driver's own scalar options, so a client
	// that sends every value the console's way is read the same either way.
	for _, option := range []string{"tls_ca_file", "tls_ca_pem", "tls_cert_file", "tls_cert_pem",
		"tls_key_file", "tls_key_pem", "tls_server_name", "sasl_mechanism", "sasl_username",
		"sasl_password"} {
		value, present, err := scalarValue(canonical[option], option)
		if err != nil {
			return nil, err
		}
		if present {
			canonical[option] = value
		}
	}

	return canonical, nil
}

// applySecurityProtocol turns the console's security.protocol into this driver's tls option.
// A protocol naming SSL turns TLS on; one naming PLAINTEXT turns it off, which is what a user
// choosing an unencrypted connection in the console asked for even when the gateway's global
// Kafka settings enable TLS.
func applySecurityProtocol(overrides, canonical map[string]interface{}) error {
	protocol, present, err := scalarValue(overrides["security.protocol"], "security.protocol")
	if err != nil {
		return err
	}
	if !present || canonical["tls"] != nil {
		return nil
	}

	switch strings.ToUpper(strings.TrimSpace(protocol)) {
	case "":
		return nil
	case "PLAINTEXT", "SASL_PLAINTEXT":
		canonical["tls"] = false
	case "SSL", "SASL_SSL":
		canonical["tls"] = true
	default:
		return fmt.Errorf("unsupported kafka security.protocol %q", protocol)
	}
	return nil
}

// scalarValue reads a property the console writes as a one-element list of strings, and accepts
// a plain string for the clients that send one. An empty list reads as absent; a longer one is
// an error rather than a silent choice between its entries.
func scalarValue(value interface{}, property string) (string, bool, error) {
	switch v := value.(type) {
	case nil:
		return "", false, nil
	case string:
		return v, true, nil
	case []string:
		if len(v) == 0 {
			return "", false, nil
		}
		if len(v) > 1 {
			return "", false, fmt.Errorf("kafka broker property %q takes a single value, got %d", property, len(v))
		}
		return v[0], true, nil
	case []interface{}:
		if len(v) == 0 {
			return "", false, nil
		}
		if len(v) > 1 {
			return "", false, fmt.Errorf("kafka broker property %q takes a single value, got %d", property, len(v))
		}
		str, ok := v[0].(string)
		if !ok {
			return "", false, fmt.Errorf("expected string for kafka broker property %q, got %T", property, v[0])
		}
		return str, true, nil
	default:
		return "", false, fmt.Errorf("expected string for kafka broker property %q, got %T", property, value)
	}
}

// hasValue reports whether a property carries anything, for the properties that are rejected on
// sight. A key present with an empty value is not a configured credential: the control plane
// blanks a credential it will not return, and that blank must not fail the connection.
func hasValue(value interface{}) (bool, error) {
	str, present, err := scalarValue(value, "")
	if err != nil {
		// A shape this cannot read is still a value, and the property is unsupported either way.
		return true, nil
	}
	return present && strings.TrimSpace(str) != "", nil
}
