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
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
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
	GatewayController GatewayControllerConfig `yaml:"gateway_controller"`
}

// GatewayControllerConfig represents the gateway_controller section of the config
type GatewayControllerConfig struct {
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
}

// IDPConfig describes an external identity provider for JWT validation
type IDPConfig struct {
	Enabled bool `yaml:"enabled"`
}

// GetDeploymentConfigFromGateway retrieves authentication configuration from a Gateway's ConfigRef
// Returns nil if no ConfigRef is specified or if auth config is not found in the ConfigMap
func GetDeploymentConfigFromGateway(ctx context.Context, k8sClient client.Client, gateway *apiv1.Gateway) (*AuthSettings, error) {
	// If no ConfigRef, return nil (will use default auth)
	if gateway.Spec.ConfigRef == nil {
		return nil, nil
	}

	// Get the ConfigMap
	configMap := &corev1.ConfigMap{}
	if err := k8sClient.Get(ctx, client.ObjectKey{
		Name:      gateway.Spec.ConfigRef.Name,
		Namespace: gateway.Namespace,
	}, configMap); err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap %s/%s: %w", gateway.Namespace, gateway.Spec.ConfigRef.Name, err)
	}

	// Look for values.yaml key
	valuesYAML, ok := configMap.Data["values.yaml"]
	if !ok {
		// No values.yaml key, return nil (will use default auth)
		return nil, nil
	}

	// Parse the YAML
	var deploymentConfig DeploymentConfig
	if err := yaml.Unmarshal([]byte(valuesYAML), &deploymentConfig); err != nil {
		return nil, fmt.Errorf("failed to parse auth config from ConfigMap: %w", err)
	}

	return &deploymentConfig.Gateway.Config.GatewayController.Auth, nil
}

// GetAuthConfigFromSecret retrieves authentication configuration from a Gateway's AuthSecretRef.
// It parses the 'users.yaml' key from the Secret into a list of AuthUser.
// Returns nil if no AuthSecretRef is specified, or if the Secret/key is missing.
func GetAuthConfigFromSecret(ctx context.Context, k8sClient client.Client, gateway *apiv1.Gateway) (*AuthSettings, error) {
	// If no AuthSecretRef, return nil
	if gateway.Spec.AuthSecretRef == nil {
		return nil, nil
	}

	// Get the Secret
	secret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, client.ObjectKey{
		Name:      gateway.Spec.AuthSecretRef.Name,
		Namespace: gateway.Namespace,
	}, secret); err != nil {
		return nil, fmt.Errorf("failed to get Auth Secret %s/%s: %w", gateway.Namespace, gateway.Spec.AuthSecretRef.Name, err)
	}

	// Look for users.yaml key
	usersYAML, ok := secret.Data["users.yaml"]
	if !ok {
		// Fallback to stringData if not in Data (though client normally consolidates them)
		// But in controller-runtime Struct Data contains byte slices
		return nil, fmt.Errorf("secret %s/%s does not contain 'users.yaml' key", gateway.Namespace, gateway.Spec.AuthSecretRef.Name)
	}

	// Parse the YAML
	var users []AuthUser
	if err := yaml.Unmarshal(usersYAML, &users); err != nil {
		return nil, fmt.Errorf("failed to parse users from Secret: %w", err)
	}

	// Construct AuthSettings
	// We assume basic auth is enabled if users are provided via secret
	authSettings := &AuthSettings{
		Basic: BasicAuthConfig{
			Enabled: true,
			Users:   users,
		},
	}

	return authSettings, nil
}

// GetBasicAuthCredentials extracts basic auth credentials from the auth config
// Returns username, password, and ok=true if basic auth is enabled and has at least one user
// Returns empty strings and ok=false if basic auth is not configured or disabled
func GetBasicAuthCredentials(authConfig *AuthSettings) (username, password string, ok bool) {
	if authConfig == nil {
		return "", "", false
	}

	// Check if basic auth is enabled
	if !authConfig.Basic.Enabled {
		return "", "", false
	}

	// Get the first user (if any)
	if len(authConfig.Basic.Users) == 0 {
		return "", "", false
	}

	user := authConfig.Basic.Users[0]

	// For now, we only support plain passwords (not hashed)
	// The operator needs the plain password to send in the Authorization header
	if user.PasswordHashed {
		// Cannot use hashed password for outgoing requests
		return "", "", false
	}

	return user.Username, user.Password, true
}

// GetDefaultBasicAuthCredentials returns the default basic auth credentials
// Default: username="admin", password="admin"
func GetDefaultBasicAuthCredentials() (username, password string) {
	return "admin", "admin"
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
