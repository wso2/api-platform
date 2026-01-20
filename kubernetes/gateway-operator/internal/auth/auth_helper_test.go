package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAuthConfigParsing(t *testing.T) {
	yamlContent := `
gateway:
  config:
    gateway_controller:
      auth:
        basic:
          enabled: true
          users:
            - username: "admin"
              password: "password123"
              password_hashed: false
              roles: ["admin"]
`

	var authConfig DeploymentConfig
	err := yaml.Unmarshal([]byte(yamlContent), &authConfig)
	assert.NoError(t, err)

	// Verify structure traversal
	basicAuth := authConfig.Gateway.Config.GatewayController.Auth.Basic
	assert.True(t, basicAuth.Enabled)
	assert.Len(t, basicAuth.Users, 1)
	assert.Equal(t, "admin", basicAuth.Users[0].Username)
	assert.Equal(t, "password123", basicAuth.Users[0].Password)
}

func TestGetBasicAuthCredentials(t *testing.T) {
	yamlContent := `
gateway:
  config:
    gateway_controller:
      auth:
        basic:
          enabled: true
          users:
            - username: "testuser"
              password: "testpassword"
              password_hashed: false
              roles: ["admin"]
`
	var deploymentConfig DeploymentConfig
	_ = yaml.Unmarshal([]byte(yamlContent), &deploymentConfig)

	username, password, ok := GetBasicAuthCredentials(&deploymentConfig.Gateway.Config.GatewayController.Auth)
	assert.True(t, ok)
	assert.Equal(t, "testuser", username)
	assert.Equal(t, "testpassword", password)
}

func TestCalculateConfigHash(t *testing.T) {
	content1 := "some content"
	content2 := "some content"
	content3 := "different content"

	hash1 := CalculateConfigHash(content1)
	hash2 := CalculateConfigHash(content2)
	hash3 := CalculateConfigHash(content3)

	assert.Equal(t, hash1, hash2, "Same content should produce same hash")
	assert.NotEqual(t, hash1, hash3, "Different content should produce different hash")
	assert.NotEmpty(t, hash1)
}

func TestGetAuthConfigFromSecret(t *testing.T) {
	// Setup scheme
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = apiv1.AddToScheme(scheme)

	// Define test data
	secretName := "auth-secret"
	namespace := "default"

	usersYaml := `
- username: "secretadmin"
  password: "secretpassword"
  roles: ["admin"]
`

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"users.yaml": []byte(usersYaml),
		},
	}

	gateway := &apiv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-gateway",
			Namespace: namespace,
		},
		Spec: apiv1.GatewaySpec{
			AuthSecretRef: &corev1.LocalObjectReference{
				Name: secretName,
			},
		},
	}

	// Create fake client
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	// Test
	authSettings, err := GetAuthConfigFromSecret(context.Background(), client, gateway)
	assert.NoError(t, err)
	assert.NotNil(t, authSettings)
	assert.True(t, authSettings.Basic.Enabled)
	assert.Len(t, authSettings.Basic.Users, 1)
	assert.Equal(t, "secretadmin", authSettings.Basic.Users[0].Username)
	assert.Equal(t, "secretpassword", authSettings.Basic.Users[0].Password)
}
