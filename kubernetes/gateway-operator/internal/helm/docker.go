// Add this method to your helm client to support using existing imagePullSecrets

package helm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DockerConfigJSON represents the structure of a Docker config.json
type DockerConfigJSON struct {
	Auths map[string]DockerAuthConfig `json:"auths"`
}

// DockerAuthConfig contains auth info for a registry
type DockerAuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}

// GetCredentialsFromImagePullSecret extracts Docker Hub credentials from an imagePullSecret
func GetCredentialsFromImagePullSecret(ctx context.Context, k8sClient client.Client, secretName, namespace, registryURL string) (string, string, error) {
	secret := &corev1.Secret{}
	err := k8sClient.Get(ctx, client.ObjectKey{
		Name:      secretName,
		Namespace: namespace,
	}, secret)
	if err != nil {
		return "", "", fmt.Errorf("failed to get secret: %w", err)
	}

	// Check if it's a docker-registry type secret
	if secret.Type == corev1.SecretTypeDockerConfigJson {
		dockerConfigJSON := secret.Data[corev1.DockerConfigJsonKey]
		if dockerConfigJSON == nil {
			return "", "", fmt.Errorf("secret does not contain .dockerconfigjson")
		}

		var config DockerConfigJSON
		if err := json.Unmarshal(dockerConfigJSON, &config); err != nil {
			return "", "", fmt.Errorf("failed to parse docker config: %w", err)
		}

		// Look for the specific registry
		for registry, authConfig := range config.Auths {
			if registry == registryURL || registry == "https://"+registryURL {
				// If auth is base64 encoded "username:password"
				if authConfig.Auth != "" {
					decoded, err := base64.StdEncoding.DecodeString(authConfig.Auth)
					if err != nil {
						return "", "", fmt.Errorf("failed to decode auth: %w", err)
					}
					// Parse "username:password"
					creds := string(decoded)
					for i, c := range creds {
						if c == ':' {
							return creds[:i], creds[i+1:], nil
						}
					}
				}
				// Otherwise use username and password directly
				return authConfig.Username, authConfig.Password, nil
			}
		}
		return "", "", fmt.Errorf("registry %s not found in secret", registryURL)
	}

	// Handle generic secret with username/password keys
	username := string(secret.Data["username"])
	password := string(secret.Data["password"])

	if username == "" || password == "" {
		return "", "", fmt.Errorf("username or password is empty in secret")
	}

	return username, password, nil
}
