# BasicAuth Policy

The **BasicAuth** policy implements HTTP Basic Authentication to protect APIs with username and password credentials. It validates the `Authorization` header and can either block unauthorized requests or allow them to proceed while recording authentication status in request metadata.

## Policy Type

- **Phase**: Request only
- **Execution**: May short-circuit (returns 401) or continues based on configuration
- **Body Processing**: Not required (header-based authentication)

## Use Cases

- Protect administrative APIs with simple authentication
- Add basic security layer to internal services
- Implement optional authentication with metadata tracking
- Test authentication flows during development
- Provide simple API access control without complex infrastructure
- Bridge legacy systems requiring Basic Auth
- Temporary authentication for proof-of-concept APIs

## Configuration Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `username` | string | Yes | - | Expected username for authentication |
| `password` | string | Yes | - | Expected password for authentication |
| `allowUnauthenticated` | boolean | No | `false` | Allow unauthenticated requests to proceed |
| `realm` | string | No | `"Restricted"` | Authentication realm for WWW-Authenticate header |

## Authentication Metadata

The policy sets the following metadata in the request context:

| Metadata Key | Type | Description |
|--------------|------|-------------|
| `auth.success` | boolean | `true` if authentication succeeded, `false` otherwise |
| `auth.username` | string | Authenticated username (only set on successful auth) |
| `auth.method` | string | Authentication method (always `"basic"`) |

## Examples

### Strict Authentication (Default)

Block all unauthenticated requests with 401 response:

```yaml
- name: BasicAuth
  version: v1.0.0
  enabled: true
  parameters:
    username: admin
    password: secret123
```

**Success Response:**
- Request proceeds to upstream
- Metadata: `auth.success = true`, `auth.username = "admin"`, `auth.method = "basic"`

**Failure Response:**
```http
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Basic realm="Restricted"
Content-Type: application/json

{"error": "Unauthorized", "message": "Authentication required"}
```

### Custom Realm

Specify a custom realm for browser authentication prompts:

```yaml
- name: BasicAuth
  version: v1.0.0
  enabled: true
  parameters:
    username: apiuser
    password: apipass
    realm: "API Gateway Admin Portal"
```

### Permissive Authentication

Allow all requests but record authentication status in metadata:

```yaml
- name: BasicAuth
  version: v1.0.0
  enabled: true
  parameters:
    username: admin
    password: secret123
    allowUnauthenticated: true
```

**Authenticated Request:**
- Metadata: `auth.success = true`, `auth.username = "admin"`

**Unauthenticated Request:**
- Request still proceeds to upstream
- Metadata: `auth.success = false`

### Integration with Conditional Policies

Use metadata to conditionally apply downstream policies:

```yaml
- route_key: /api/v1/admin
  policies:
    - name: BasicAuth
      version: v1.0.0
      enabled: true
      parameters:
        username: admin
        password: secret123
        allowUnauthenticated: true

    - name: Respond
      version: v1.0.0
      enabled: true
      condition: 'metadata["auth.success"] == false'
      parameters:
        statusCode: 403
        body: '{"error": "Forbidden", "message": "Admin access required"}'
        headers:
          - name: content-type
            value: application/json
```

In this example:
1. BasicAuth allows all requests through
2. Respond policy only executes if auth failed (returns 403 instead of 401)

### Multi-Tier Access Control

Combine with other policies for sophisticated access control:

```yaml
- route_key: /api/v1/data
  policies:
    - name: BasicAuth
      version: v1.0.0
      enabled: true
      parameters:
        username: datauser
        password: datapass
        allowUnauthenticated: false
        realm: "Data API"

    - name: ModifyHeaders
      version: v1.0.0
      enabled: true
      parameters:
        requestHeaders:
          - action: SET
            name: x-authenticated-user
            value: metadata["auth.username"]
          - action: DELETE
            name: authorization  # Remove after validation
        responseHeaders:
          - action: SET
            name: x-auth-method
            value: basic
```

## How to Use Basic Auth

### From cURL

```bash
# Method 1: Using -u flag (recommended)
curl -u admin:secret123 https://api.example.com/admin

# Method 2: Explicit Authorization header
curl -H "Authorization: Basic YWRtaW46c2VjcmV0MTIz" https://api.example.com/admin

# Method 3: Generate base64 manually
echo -n "admin:secret123" | base64
# Output: YWRtaW46c2VjcmV0MTIz
curl -H "Authorization: Basic YWRtaW46c2VjcmV0MTIz" https://api.example.com/admin
```

### From JavaScript

```javascript
// Browser Fetch API
const username = 'admin';
const password = 'secret123';
const credentials = btoa(`${username}:${password}`);

fetch('https://api.example.com/admin', {
  headers: {
    'Authorization': `Basic ${credentials}`
  }
});

// Axios
const axios = require('axios');

axios.get('https://api.example.com/admin', {
  auth: {
    username: 'admin',
    password: 'secret123'
  }
});
```

### From Python

```python
import requests
from requests.auth import HTTPBasicAuth

# Method 1: Using auth parameter (recommended)
response = requests.get(
    'https://api.example.com/admin',
    auth=HTTPBasicAuth('admin', 'secret123')
)

# Method 2: Manual header
import base64
credentials = base64.b64encode(b'admin:secret123').decode('utf-8')
response = requests.get(
    'https://api.example.com/admin',
    headers={'Authorization': f'Basic {credentials}'}
)
```

## Behavior

1. **Missing Authorization Header**: Returns 401 (or allows if `allowUnauthenticated: true`)
2. **Invalid Scheme**: Returns 401 if not "Basic" auth (e.g., "Bearer" token)
3. **Invalid Base64**: Returns 401 if credentials are not properly base64 encoded
4. **Invalid Format**: Returns 401 if credentials don't follow "username:password" format
5. **Wrong Credentials**: Returns 401 if username or password doesn't match
6. **Successful Auth**: Request proceeds, metadata is set with auth details
7. **Case Sensitivity**: Username and password comparisons are case-sensitive
8. **Header Normalization**: Authorization header name is case-insensitive

## Security Considerations

### ⚠️ Important Security Notes

1. **HTTPS Required**: Basic Auth sends credentials in base64 (NOT encrypted). Always use HTTPS in production.
2. **Not for Production**: Basic Auth is generally not recommended for production APIs. Consider:
   - OAuth 2.0 / JWT for modern APIs
   - API Keys for service-to-service communication
   - mTLS for high-security requirements
3. **Credential Storage**: Store credentials securely (environment variables, secrets management)
4. **Password Strength**: Use strong, randomly generated passwords
5. **Rotation**: Implement credential rotation policies
6. **Rate Limiting**: Combine with rate limiting to prevent brute-force attacks

### When Basic Auth is Appropriate

- Internal APIs behind firewall
- Development and testing environments
- Legacy system integration
- Simple admin interfaces
- Proof-of-concept implementations
- Temporary access during migration

## Performance Considerations

- Minimal overhead (header parsing + base64 decode + string comparison)
- No external dependencies or database lookups
- Fast execution suitable for high-throughput APIs
- Metadata operations are in-memory

## Error Responses

### 401 Unauthorized (Authentication Failed)

```json
{
  "error": "Unauthorized",
  "message": "Authentication required"
}
```

Headers include:
- `WWW-Authenticate: Basic realm="<configured-realm>"`
- `Content-Type: application/json`

## Comparison with Other Auth Methods

| Method | Security | Complexity | Use Case |
|--------|----------|------------|----------|
| Basic Auth | Low (base64) | Very Simple | Internal APIs, dev/test |
| API Key | Medium | Simple | Service-to-service |
| JWT | High | Medium | Modern web APIs |
| OAuth 2.0 | Very High | Complex | Third-party access |
| mTLS | Very High | High | High-security environments |

## Best Practices

1. **Always use HTTPS** in production
2. **Set meaningful realms** to help users identify the resource
3. **Use strong passwords** (minimum 16 characters, random)
4. **Don't hardcode credentials** in configuration files
5. **Combine with rate limiting** to prevent brute-force
6. **Use allowUnauthenticated mode** for optional auth with conditional logic
7. **Remove Authorization header** after validation (use ModifyHeaders policy)
8. **Monitor auth failures** for security incidents
9. **Implement credential rotation** procedures
10. **Consider upgrading** to more secure auth methods for production

## Troubleshooting

### Common Issues

**Issue**: 401 with valid credentials
- Check username/password are correct (case-sensitive)
- Verify base64 encoding is correct
- Ensure no extra whitespace in credentials

**Issue**: Browser keeps prompting for credentials
- Check realm spelling matches expectation
- Verify credentials are being sent correctly
- Clear browser cache/credentials

**Issue**: Metadata not accessible in downstream policies
- Ensure BasicAuth policy runs first in the chain
- Check metadata keys exactly match: `auth.success`, `auth.username`, `auth.method`

**Issue**: CORS preflight failures
- OPTIONS requests don't include Authorization header by default
- Consider allowing OPTIONS without auth or use CORS policy
