# Authentication Middleware

This package provides flexible authentication middleware for Gin applications, supporting both Basic Authentication and Bearer Token (JWT) authentication.

## Features

- **Basic Authentication**: Username/password authentication with support for both plaintext and bcrypt-hashed passwords
- **Bearer Token Authentication**: JWT token validation with JWKS support
- **Flexible Configuration**: Use either authentication method independently or together
- **Path Skipping**: Configure paths that bypass authentication
- **Context Integration**: Authenticated user information is automatically stored in Gin context

## Usage

### 1. Basic Authentication Only

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/wso2/api-platform/common/authenticators"
    "github.com/wso2/api-platform/common/models"
)

func main() {
    r := gin.Default()

    // Configure basic auth
    basicAuth := &models.BasicAuth{
        Enabled: true,
        Users: []models.User{
            {
                UserID:   "admin",
                Password: "secret123",
                PasswordHashed: false,
                Roles:    []string{"admin", "user"},
            },
            {
                UserID:   "user1",
                Password: "$2a$10$...", // bcrypt hashed password
                PasswordHashed: true,
                Roles:    []string{"user"},
            },
        },
    }

    // Apply middleware
    r.Use(authenticators.BasicAuthMiddleware(basicAuth, []string{"/health", "/metrics"}))

    r.GET("/protected", func(c *gin.Context) {
        userID, _ := c.Get("user_id")
        c.JSON(200, gin.H{"message": "Hello " + userID.(string)})
    })

    r.Run(":8080")
}
```

**Request Example:**
```bash
curl -u admin:secret123 http://localhost:8080/protected
# or
curl -H "Authorization: Basic YWRtaW46c2VjcmV0MTIz" http://localhost:8080/protected
```

### 2. Bearer Token Authentication Only

```go
func main() {
    r := gin.Default()

    // Configure JWT auth
    jwtConfig := authenticators.JWTAuthConfig{
        IDPConfig: &models.IDPConfig{
            IssuerURL: "https://idp.example.com",
            JWKSUrl:   "https://idp.example.com/.well-known/jwks.json",
        },
        AllowedIssuers: []string{"https://idp.example.com"},
        SkipPaths:      []string{"/health", "/metrics"},
    }

    // Apply middleware
    r.Use(authenticators.BearerAuthMiddleware(jwtConfig, []string{"/health"}))

    r.GET("/protected", func(c *gin.Context) {
        claims, _ := authenticators.GetClaimsFromContext(c)
        c.JSON(200, gin.H{"message": "Hello " + claims.Subject})
    })

    r.Run(":8080")
}
```

**Request Example:**
```bash
curl -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..." http://localhost:8080/protected
```

### 3. Combined Authentication (Basic OR Bearer)

```go
func main() {
    r := gin.Default()

    // Configure both auth methods
    authConfig := authenticators.AuthConfig{
        BasicAuth: &models.BasicAuth{
            Enabled: true,
            Users: []models.User{
                {
                    UserID:   "admin",
                    Password: "secret123",
                    Roles:    []string{"admin"},
                },
            },
        },
        JWTConfig: &authenticators.JWTAuthConfig{
            IDPConfig: &models.IDPConfig{
                JWKSUrl: "https://idp.example.com/.well-known/jwks.json",
            },
        },
        SkipPaths:   []string{"/health", "/public"},
        AllowEither: true, // Accept either basic OR bearer auth
    }

    // Apply middleware
    r.Use(authenticators.AuthMiddleware(authConfig))

    r.GET("/protected", func(c *gin.Context) {
        userID, _ := authenticators.GetUserIDFromContext(c)
        authType, _ := authenticators.GetAuthTypeFromContext(c)
        
        c.JSON(200, gin.H{
            "user_id":   userID,
            "auth_type": authType, // "basic" or "bearer"
        })
    })

    r.Run(":8080")
}
```

## Context Values

After successful authentication, the following values are available in the Gin context:

### For Basic Authentication:
- `user_id`: User identifier
- `username`: Username (same as user_id)
- `roles`: Array of user roles
- `auth_type`: "basic"

### For Bearer Token Authentication:
- `user_id`: Subject from JWT
- `username`: Username from claims
- `claims`: Full JWT claims object
- `email`: User email (if present)
- `first_name`: User's first name (if present)
- `last_name`: User's last name (if present)
- `organization`: Organization (if present)
- `roles`: User roles (if present)
- `scope`: Token scope (if present)
- `audience`: Token audience (if present)
- `auth_type`: "bearer" (implicit)

## Helper Functions

### Retrieve User Information

```go
// Get user ID
userID, exists := authenticators.GetUserIDFromContext(c)

// Get username
username, exists := authenticators.GetUsernameFromContext(c)

// Get organization
org, exists := authenticators.GetOrganizationFromContext(c)

// Get roles
roles, exists := authenticators.GetRolesFromContext(c)

// Get full JWT claims
claims, exists := authenticators.GetClaimsFromContext(c)

// Get authentication type
authType, exists := authenticators.GetAuthTypeFromContext(c)
```

## Password Hashing

For secure password storage, use bcrypt to hash passwords:

```go
import "golang.org/x/crypto/bcrypt"

// Generate hashed password
hashedPassword, err := bcrypt.GenerateFromPassword([]byte("plaintext-password"), bcrypt.DefaultCost)
if err != nil {
    log.Fatal(err)
}

// Use in configuration
user := models.User{
    UserID:   "admin",
    Password: string(hashedPassword),
    PasswordHashed: true,
}
```

## Configuration Options

### AuthConfig

- `BasicAuth`: Basic authentication configuration (optional)
- `JWTConfig`: JWT/Bearer token configuration (optional)
- `SkipPaths`: Paths that bypass authentication
- `AllowEither`: If true, accept either basic or bearer auth; if false and both configured, require both
- `UnauthorizedHandler`: Custom handler for unauthorized requests

### JWTAuthConfig

- `IDPConfig`: Identity provider configuration
- `SkipPaths`: Paths to skip authentication
- `SkipValidation`: Skip token validation (development only)
- `SecretKey`: Secret key for HS256 (optional)
- `AllowedIssuers`: List of allowed token issuers
- `JWKSCacheTTL`: JWKS cache duration (default: 5 minutes)
- `HTTPTimeout`: HTTP request timeout (default: 10 seconds)

## Error Handling

The middleware returns `401 Unauthorized` with a JSON error message for authentication failures:

```json
{
  "error": "Authorization header is required"
}
```

You can customize error responses using a custom unauthorized handler:

```go
authConfig := authenticators.AuthConfig{
    // ... other config
    UnauthorizedHandler: func(c *gin.Context) {
        c.JSON(401, gin.H{
            "status": "error",
            "message": "Authentication required",
        })
        c.Abort()
    },
}
```

## Security Best Practices

1. **Always use HTTPS** in production to protect credentials and tokens
2. **Use hashed passwords** (bcrypt) instead of plaintext
3. **Implement rate limiting** to prevent brute force attacks
4. **Rotate secrets regularly** for JWT signing keys
5. **Use short token expiration times** and implement refresh tokens
6. **Validate token issuers** to prevent token substitution attacks
7. **Keep JWKS cache TTL reasonable** to allow for key rotation
