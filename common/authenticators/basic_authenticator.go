/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
package authenticators

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/common/models"
	"golang.org/x/crypto/argon2"
	bcrypt "golang.org/x/crypto/bcrypt"
)

// BasicAuthenticator implements HTTP Basic Authentication
type BasicAuthenticator struct {
	authConfig models.AuthConfig
	logger     *slog.Logger
}

// NewBasicAuthenticator creates a new BasicAuthenticator
func NewBasicAuthenticator(authConfig models.AuthConfig, logger *slog.Logger) *BasicAuthenticator {
	return &BasicAuthenticator{
		authConfig: authConfig,
		logger:     logger,
	}
}

// Authenticate verifies basic authentication credentials from context
func (b *BasicAuthenticator) Authenticate(c *gin.Context) (*AuthResult, error) {
	authHeader := c.GetHeader(constants.AuthorizationHeader)
	if authHeader == "" {
		return nil, ErrAuthenticationFailed
	}
	// Extract and decode credentials
	encodedCredentials := strings.TrimPrefix(authHeader, constants.BasicPrefix)
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedCredentials)
	if err != nil {
		return nil, ErrAuthenticationFailed
	}
	credentials := string(decodedBytes)
	parts := strings.SplitN(credentials, ":", 2)
	if len(parts) != 2 {
		return nil, ErrAuthenticationFailed
	}
	username := parts[0]
	password := parts[1]

	// If auth is not configured or no users are defined, skip auth
	if b.authConfig.BasicAuth == nil || len(b.authConfig.BasicAuth.Users) == 0 {
		return nil, errors.New("no users configured for basic authentication")
	}

	// Find user in config
	var matched *models.User
	for i := range b.authConfig.BasicAuth.Users {
		u := &b.authConfig.BasicAuth.Users[i]
		if strings.EqualFold(u.Username, username) {
			matched = u
			break
		}
	}
	if matched == nil {
		return nil, ErrAuthenticationFailed
	}

	// Validate password
	if matched.PasswordHashed {
		if err := verifyPassword(matched.Password, password); err != nil {
			return nil, ErrAuthenticationFailed
		}
	} else {
		// Use constant-time comparison for plaintext passwords
		stored := []byte(matched.Password)
		incoming := []byte(password)
		if len(stored) != len(incoming) ||
			subtle.ConstantTimeCompare(stored, incoming) != 1 {
			return nil, ErrAuthenticationFailed
		}
	}

	return &AuthResult{
		Success: true,
		UserID:  matched.Username,
		Roles:   matched.Roles,
	}, nil
}

// Name returns the authenticator name
func (b *BasicAuthenticator) Name() string {
	return "BasicAuthenticator"
}

// CanHandle checks if credentials in context are BasicCredentials
func (b *BasicAuthenticator) CanHandle(c *gin.Context) bool {

	authHeader := c.GetHeader(constants.AuthorizationHeader)
	if authHeader == "" {
		return false
	}
	// Determine auth type from header
	return strings.HasPrefix(authHeader, constants.BasicPrefix)
}

// verifyPassword verifies a password against a stored hash.
// It supports Argon2id encoded hashes (preferred) and falls back to bcrypt.
func verifyPassword(stored, password string) error {
	if strings.HasPrefix(stored, "$argon2id$") {
		return compareArgon2id(stored, password)
	}

	// Try bcrypt as a fallback if prefix matches bcrypt format
	if strings.HasPrefix(stored, "$2y$") || strings.HasPrefix(stored, "$2a$") || strings.HasPrefix(stored, "$2b$") {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(password))
	}

	// Unknown format — return error
	return errors.New("unsupported password hash format")
}

// compareArgon2id parses an encoded Argon2id hash and compares it to the provided password.
// Expected format: $argon2id$v=19$m=65536,t=3,p=4$<salt_b64>$<hash_b64>
func compareArgon2id(encoded, password string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return fmt.Errorf("invalid argon2id hash format")
	}

	// parts[2] -> v=19
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return err
	}
	if version != argon2.Version {
		return fmt.Errorf("unsupported argon2 version: %d", version)
	}

	// parts[3] -> m=65536,t=3,p=4
	var mem uint32
	var iters uint32
	var threads uint8
	var t, m, p uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return err
	}
	mem = m
	iters = t
	threads = uint8(p)

	// decode salt and hash (try RawStd then Std)
	salt, err := decodeBase64(parts[4])
	if err != nil {
		return err
	}
	hash, err := decodeBase64(parts[5])
	if err != nil {
		return err
	}

	derived := argon2.IDKey([]byte(password), salt, iters, mem, threads, uint32(len(hash)))
	if subtle.ConstantTimeCompare(derived, hash) == 1 {
		return nil
	}
	return errors.New("password mismatch")
}

func decodeBase64(s string) ([]byte, error) {
	b, err := base64.RawStdEncoding.DecodeString(s)
	if err == nil {
		return b, nil
	}
	// try StdEncoding as a fallback
	return base64.StdEncoding.DecodeString(s)
}
