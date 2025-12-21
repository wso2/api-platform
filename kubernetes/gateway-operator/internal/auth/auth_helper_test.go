package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
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
