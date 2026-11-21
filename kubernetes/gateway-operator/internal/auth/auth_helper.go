/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// DeploymentConfig represents the authentication configuration from the gateway ConfigMap
type DeploymentConfig struct {
	Gateway GatewayConfig `yaml:"gateway"`
}

// GatewayConfig represents the gateway section
type GatewayConfig struct {
	Config Config `yaml:"config"`
}

// Config represents the config section
type Config struct {
	Controller ControllerConfig `yaml:"controller"`
}

// ControllerConfig represents the controller section of the config
type ControllerConfig struct {
	Auth AuthSettings `yaml:"auth"`
}

// AuthSettings holds authentication related configuration
type AuthSettings struct {
	Basic BasicAuthConfig `yaml:"basic"`
	IDP   IDPConfig       `yaml:"idp"`
}

// BasicAuthConfig describes basic authentication configuration
type BasicAuthConfig struct {
	Enabled bool       `yaml:"enabled"`
	Users   []AuthUser `yaml:"users"`
}

// AuthUser describes a locally configured user
type AuthUser struct {
	Username       string   `yaml:"username"`
	Password       string   `yaml:"password"`
	PasswordHashed bool     `yaml:"password_hashed"`
	Roles          []string `yaml:"roles"`

	// PasswordSecretRef is read by the operator only and is stripped from the values before
	// Helm renders the controller's config.toml. It names a Secret key in the gateway's
	// namespace holding this user's plaintext password, which the operator sends to the
	// controller's management API. Pair it with password set to the bcrypt hash of that
	// password and password_hashed: true, so the controller's config never holds the plaintext.
	PasswordSecretRef *SecretKeyRef `yaml:"password_secret_ref,omitempty"`
}

// SecretKeyRef names a key in a Secret in the gateway's namespace.
type SecretKeyRef struct {
	Name string `yaml:"name"`
	Key  string `yaml:"key"`
}

// IDPConfig describes an external identity provider for JWT validation
type IDPConfig struct {
	Enabled bool `yaml:"enabled"`
}

// EncodeBasicAuth encodes username and password for HTTP Basic Authentication
// Returns the value to be used in the Authorization header (without "Basic " prefix)
func EncodeBasicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// CalculateConfigHash calculates a SHA256 hash of the configuration content
// This is used to detect changes in the configuration that should trigger reconciliation
func CalculateConfigHash(configContent string) string {
	hash := sha256.Sum256([]byte(configContent))
	return hex.EncodeToString(hash[:])
}
