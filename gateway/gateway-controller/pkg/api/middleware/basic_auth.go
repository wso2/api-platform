package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"go.uber.org/zap"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// BasicAuthMiddleware implements HTTP Basic auth against locally configured users.
// It supports plain-text passwords and bcrypt-hashed passwords (when PasswordHashed is true).
func BasicAuthMiddleware(cfg *config.Config, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// If auth is not configured or no users are defined, skip auth
		if len(cfg.GatewayController.Auth.Users) == 0 {
			c.Next()
			return
		}

		// Extract basic auth credentials
		username, password, ok := c.Request.BasicAuth()
		if !ok {
			logger.Debug("missing basic auth header")
			c.Header("WWW-Authenticate", `Basic realm="Restricted"`)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// Find user in config
		var matched *config.AuthUser
		for i := range cfg.GatewayController.Auth.Users {
			u := &cfg.GatewayController.Auth.Users[i]
			if strings.EqualFold(u.Username, username) {
				matched = u
				break
			}
		}

		if matched == nil {
			logger.Debug("unknown user", zap.String("user", username))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// Validate password
		if matched.PasswordHashed {
			if err := verifyPassword(matched.Password, password); err != nil {
				logger.Debug("password mismatch", zap.String("user", username), zap.Error(err))
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
		} else {
			if matched.Password != password {
				logger.Debug("password mismatch", zap.String("user", username))
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
		}

		// Attach user info to context
		c.Set(AuthUserKey, matched.Username)
		c.Set(AuthRolesKey, matched.Roles)
		logger.Info("Auth Roles", zap.String("user", username), zap.Strings("roles", matched.Roles))
		// Add to logger for downstream handlers
		log := GetLogger(c, logger).With(zap.String("auth_user", matched.Username))
		c.Set(LoggerKey, log)

		c.Next()
	}
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
