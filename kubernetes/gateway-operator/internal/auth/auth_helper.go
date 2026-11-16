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
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/helm"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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
	// Enabled is nil when the values do not set it; the gateway chart's default is true.
	Enabled *bool      `yaml:"enabled"`
	Users   []AuthUser `yaml:"users"`
	// Generate makes the gateway chart create the "admin" user with a random password
	// when Users is empty and ExistingSecret is not set. nil means the chart default, true.
	Generate *bool `yaml:"generate"`
	// ExistingSecret names a Secret in the gateway's namespace, with keys "password" and
	// "password-hash", that holds the "admin" user's credential when Users is empty.
	ExistingSecret string `yaml:"existingSecret"`
}

// IsEnabled reports whether basic auth is on, applying the gateway chart's default.
func (b BasicAuthConfig) IsEnabled() bool { return b.Enabled == nil || *b.Enabled }

// GenerateEnabled reports whether the gateway chart generates the admin credential,
// applying its default.
func (b BasicAuthConfig) GenerateEnabled() bool { return b.Generate == nil || *b.Generate }

// AuthUser describes a locally configured user
type AuthUser struct {
	Username       string   `yaml:"username"`
	Password       string   `yaml:"password"`
	PasswordHashed bool     `yaml:"password_hashed"`
	Roles          []string `yaml:"roles"`
	// PasswordSecretRef names a Secret, in the gateway's namespace, holding this
	// user's plaintext password. The gateway-controller never reads it: it is how the
	// operator learns the password when the controller config holds only its hash
	// (password_hashed: true).
	PasswordSecretRef *SecretKeyRef `yaml:"passwordSecretRef"`
}

// SecretKeyRef selects one key of a Secret in the gateway's namespace.
type SecretKeyRef struct {
	Name string `yaml:"name"`
	// Key within the Secret's data. Defaults to DefaultPasswordSecretKey.
	Key string `yaml:"key"`
}

// DefaultPasswordSecretKey is the Secret key read when passwordSecretRef.key is empty.
const DefaultPasswordSecretKey = "password"

// The admin user the gateway chart configures when no users are listed, and the
// labels of the Secret it generates for it (<fullname>-controller-admin).
const (
	AdminUsername             = "admin"
	AdminSecretPassword       = "password"
	adminSecretInstanceLabel  = "app.kubernetes.io/instance"
	adminSecretComponentLabel = "app.kubernetes.io/component"
	adminSecretComponent      = "controller-admin"
)

// ErrNoControllerCredentials reports that the gateway's values configure no
// gateway-controller user whose password the operator can send.
var ErrNoControllerCredentials = errors.New("no usable gateway-controller basic auth credentials")

// IDPConfig describes an external identity provider for JWT validation
type IDPConfig struct {
	Enabled bool `yaml:"enabled"`
}

// GetDeploymentConfigFromGateway retrieves authentication configuration from an APIGateway's ConfigRef
// Returns nil if no ConfigRef is specified or if auth config is not found in the ConfigMap
func GetDeploymentConfigFromGateway(ctx context.Context, k8sClient client.Reader, gateway *apiv1.APIGateway) (*AuthSettings, error) {
	if gateway.Spec.ConfigRef == nil {
		return nil, nil
	}
	return GetDeploymentAuthFromConfigMap(ctx, k8sClient, gateway.Namespace, gateway.Spec.ConfigRef.Name)
}

// GetDeploymentAuthFromConfigMap reads auth settings from a ConfigMap values.yaml key.
func GetDeploymentAuthFromConfigMap(ctx context.Context, k8sClient client.Reader, namespace, configMapName string) (*AuthSettings, error) {
	valuesYAML, found, err := configMapValues(ctx, k8sClient, namespace, configMapName)
	if err != nil || !found {
		return nil, err
	}
	return parseAuthSettings(valuesYAML)
}

// GetAuthSettingsForRegistryGateway resolves controller REST auth settings for a registered gateway.
// Prefer Helm values ConfigMap when HelmValuesConfigMapName is set; otherwise load from APIGateway CR.
func GetAuthSettingsForRegistryGateway(ctx context.Context, k8sClient client.Reader, info *registry.GatewayInfo) (*AuthSettings, error) {
	valuesYAML, found, err := overlayValuesForRegistryGateway(ctx, k8sClient, info)
	if err != nil || !found {
		return nil, err
	}
	return parseAuthSettings(valuesYAML)
}

// ResolveControllerCredentials returns the username and password the operator sends to
// the gateway-controller REST API of a registered gateway.
//
// The auth settings are read from the same Helm values the gateway was installed with:
// the operator's base values file (baseValuesFile, may be empty) overlaid with the
// gateway's own values ConfigMap. There is no built-in fallback credential: when no
// usable user is configured, an error wrapping ErrNoControllerCredentials is returned.
func ResolveControllerCredentials(ctx context.Context, k8sClient client.Reader, info *registry.GatewayInfo, baseValuesFile string) (username, password string, err error) {
	if info == nil {
		return "", "", fmt.Errorf("%w: gateway is not known", ErrNoControllerCredentials)
	}
	settings, err := effectiveAuthSettings(ctx, k8sClient, info, baseValuesFile)
	if err != nil {
		return "", "", err
	}
	return ResolveBasicAuthCredentials(ctx, k8sClient, info.Namespace, helm.GetReleaseName(info.Name), settings)
}

// ResolveBasicAuthCredentials returns the credential the operator sends to a gateway
// installed as Helm release releaseName in namespace.
//
// With no users listed (the gateway chart's default), it is the "admin" user's
// password from basic.existingSecret, or from the Secret the chart generated for the
// release (found by its labels). Otherwise it is the first listed user whose password
// the operator can send: one with a passwordSecretRef (read from that Secret in
// namespace), or one with a plaintext password. Users with only a hashed password, or
// with a config interpolation token as their password, are skipped, because the
// operator cannot recover the plaintext from them. nil settings mean the chart defaults.
func ResolveBasicAuthCredentials(ctx context.Context, k8sClient client.Reader, namespace, releaseName string, settings *AuthSettings) (username, password string, err error) {
	if settings == nil {
		settings = &AuthSettings{}
	}
	basic := settings.Basic
	if !basic.IsEnabled() {
		return "", "", fmt.Errorf("%w: gateway.config.controller.auth.basic is not enabled", ErrNoControllerCredentials)
	}
	if len(basic.Users) == 0 {
		return resolveAdminSecret(ctx, k8sClient, namespace, releaseName, basic)
	}
	var skipped []string
	for _, user := range settings.Basic.Users {
		if user.Username == "" {
			continue
		}
		if ref := user.PasswordSecretRef; ref != nil && ref.Name != "" {
			if k8sClient == nil {
				return "", "", fmt.Errorf("read password of gateway-controller user %q: no Kubernetes client", user.Username)
			}
			password, err := readSecretKey(ctx, k8sClient, namespace, ref)
			if err != nil {
				return "", "", fmt.Errorf("read password of gateway-controller user %q: %w", user.Username, err)
			}
			return user.Username, password, nil
		}
		switch {
		case user.PasswordHashed:
			skipped = append(skipped, fmt.Sprintf("user %q has a hashed password and no passwordSecretRef", user.Username))
		case strings.Contains(user.Password, "{{"):
			skipped = append(skipped, fmt.Sprintf("user %q has an interpolation token as its password, which only the gateway can resolve; set passwordSecretRef", user.Username))
		case user.Password == "":
			skipped = append(skipped, fmt.Sprintf("user %q has no password", user.Username))
		default:
			return user.Username, user.Password, nil
		}
	}
	if len(skipped) == 0 {
		return "", "", fmt.Errorf("%w: no basic auth user has a username", ErrNoControllerCredentials)
	}
	return "", "", fmt.Errorf("%w: %s", ErrNoControllerCredentials, strings.Join(skipped, "; "))
}

// resolveAdminSecret reads the password of the chart-configured "admin" user from
// basic.existingSecret or from the generated Secret of the release.
func resolveAdminSecret(ctx context.Context, k8sClient client.Reader, namespace, releaseName string, basic BasicAuthConfig) (string, string, error) {
	if basic.ExistingSecret == "" && !basic.GenerateEnabled() {
		return "", "", fmt.Errorf("%w: gateway.config.controller.auth.basic has no users, no existingSecret and generate is false", ErrNoControllerCredentials)
	}
	if k8sClient == nil {
		return "", "", fmt.Errorf("read gateway-controller admin password: no Kubernetes client")
	}
	name := basic.ExistingSecret
	if name == "" {
		secrets := &corev1.SecretList{}
		selector := client.MatchingLabels{adminSecretInstanceLabel: releaseName, adminSecretComponentLabel: adminSecretComponent}
		if err := k8sClient.List(ctx, secrets, client.InNamespace(namespace), selector); err != nil {
			return "", "", fmt.Errorf("list gateway-controller admin Secrets in %s: %w", namespace, err)
		}
		if len(secrets.Items) == 0 {
			return "", "", fmt.Errorf("%w: no Secret labelled %s=%s,%s=%s in namespace %s yet; the gateway may not be installed yet, "+
				"or its chart version does not generate the controller admin credential (then list gateway.config.controller.auth.basic.users)",
				ErrNoControllerCredentials, adminSecretInstanceLabel, releaseName, adminSecretComponentLabel, adminSecretComponent, namespace)
		}
		sort.Slice(secrets.Items, func(i, j int) bool { return secrets.Items[i].Name < secrets.Items[j].Name })
		name = secrets.Items[0].Name
	}
	password, err := readSecretKey(ctx, k8sClient, namespace, &SecretKeyRef{Name: name, Key: AdminSecretPassword})
	if err != nil {
		return "", "", fmt.Errorf("read gateway-controller admin password: %w", err)
	}
	return AdminUsername, password, nil
}

// effectiveAuthSettings merges the operator's base values file with the gateway's
// values overlay, the same way the gateway's Helm install does, and returns the
// resulting controller auth settings.
func effectiveAuthSettings(ctx context.Context, k8sClient client.Reader, info *registry.GatewayInfo, baseValuesFile string) (*AuthSettings, error) {
	base := ""
	if baseValuesFile != "" {
		data, err := os.ReadFile(baseValuesFile)
		switch {
		case err == nil:
			base = string(data)
		case !errors.Is(err, os.ErrNotExist):
			return nil, fmt.Errorf("read base Helm values file %s: %w", baseValuesFile, err)
		}
	}
	overlay, _, err := overlayValuesForRegistryGateway(ctx, k8sClient, info)
	if err != nil {
		return nil, err
	}
	merged, err := helm.MergeValuesYAML(base, overlay)
	if err != nil {
		return nil, fmt.Errorf("merge gateway Helm values: %w", err)
	}
	return parseAuthSettings(merged)
}

// overlayValuesForRegistryGateway returns the values.yaml overlay of a registered
// gateway: its Helm values ConfigMap when HelmValuesConfigMapName is set, otherwise the
// ConfigMap its APIGateway CR references. found is false when there is none.
func overlayValuesForRegistryGateway(ctx context.Context, k8sClient client.Reader, info *registry.GatewayInfo) (string, bool, error) {
	if info == nil {
		return "", false, nil
	}
	if info.HelmValuesConfigMapName != "" {
		return configMapValues(ctx, k8sClient, info.Namespace, info.HelmValuesConfigMapName)
	}
	gateway := &apiv1.APIGateway{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: info.Namespace, Name: info.Name}, gateway); err != nil {
		if apierrors.IsNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}
	if gateway.Spec.ConfigRef == nil {
		return "", false, nil
	}
	return configMapValues(ctx, k8sClient, gateway.Namespace, gateway.Spec.ConfigRef.Name)
}

func configMapValues(ctx context.Context, k8sClient client.Reader, namespace, configMapName string) (string, bool, error) {
	if configMapName == "" {
		return "", false, nil
	}
	configMap := &corev1.ConfigMap{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: configMapName}, configMap); err != nil {
		return "", false, fmt.Errorf("get ConfigMap %s/%s: %w", namespace, configMapName, err)
	}
	valuesYAML, ok := configMap.Data["values.yaml"]
	if !ok {
		return "", false, nil
	}
	return valuesYAML, true, nil
}

func parseAuthSettings(valuesYAML string) (*AuthSettings, error) {
	var deploymentConfig DeploymentConfig
	if err := yaml.Unmarshal([]byte(valuesYAML), &deploymentConfig); err != nil {
		return nil, fmt.Errorf("parse auth config from Helm values: %w", err)
	}
	return &deploymentConfig.Gateway.Config.Controller.Auth, nil
}

// readSecretKey reads one key of a Secret in namespace. Only the gateway's own
// namespace is searched, so a values document cannot point the operator at another
// namespace's Secrets.
func readSecretKey(ctx context.Context, k8sClient client.Reader, namespace string, ref *SecretKeyRef) (string, error) {
	key := ref.Key
	if key == "" {
		key = DefaultPasswordSecretKey
	}
	secret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ref.Name}, secret); err != nil {
		return "", fmt.Errorf("get Secret %s/%s: %w", namespace, ref.Name, err)
	}
	value, ok := secret.Data[key]
	if !ok || len(value) == 0 {
		return "", fmt.Errorf("secret %s/%s has no value under key %q", namespace, ref.Name, key)
	}
	return string(value), nil
}

// GetBasicAuthCredentials extracts the plaintext credentials of the first basic auth
// user from the auth config. It returns ok=false if basic auth is disabled, has no
// users, or the first user's password is hashed. It does not read Secrets; use
// ResolveBasicAuthCredentials for users with a passwordSecretRef.
func GetBasicAuthCredentials(authConfig *AuthSettings) (username, password string, ok bool) {
	if authConfig == nil {
		return "", "", false
	}

	// Check if basic auth is enabled
	if !authConfig.Basic.IsEnabled() {
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
