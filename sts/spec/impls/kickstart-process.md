# Feature: Kickstart Process

## Overview

Automated bash script that creates an organization, user, and OAuth application in Thunder, simplifying the initial setup process for STS deployments.

## Git Commits

- `3cb021d` - Add kickstart automation script for STS setup

## Motivation

Manual creation of organizations, users, and applications through Thunder's API requires:
- Multiple API calls with proper sequencing
- Understanding of Thunder's data model
- Extracting and correlating IDs across responses
- Manual credential management

The kickstart script automates this entire workflow, providing:
- One-command setup
- Sensible defaults
- Comprehensive output documentation
- Ready-to-use OAuth configuration

## Architecture

```
┌──────────────┐         ┌──────────────────┐         ┌──────────────┐
│              │         │                  │         │              │
│  inputs.yaml │────────▶│  kickstart.sh    │────────▶│ Thunder API  │
│              │         │                  │         │              │
└──────────────┘         └──────────────────┘         └──────────────┘
                                  │
                                  │
                                  ▼
                         ┌──────────────────┐
                         │                  │
                         │ registration.yaml│
                         │                  │
                         └──────────────────┘
```

## Implementation Details

### Script Flow

**File**: `kickstart.sh`

1. **Wait for Thunder** (with retries)
   - Polls `https://localhost:8090/health/liveness`
   - Max retries: 30 (60 seconds total)
   - Retry delay: 2 seconds
   - Fails gracefully with helpful error message

2. **Load Configuration** (from inputs.yaml)
   - Parses YAML using basic grep/sed (no yq dependency)
   - Applies sensible defaults for all fields
   - Supports custom values via inputs.yaml

3. **Create Organization** (API call #1)
   - Endpoint: `POST /organization-units`
   - Payload: name, description, handle
   - Extracts organization ID from response

4. **Create User** (API call #2)
   - Endpoint: `POST /users`
   - Payload: organizationUnit, type, attributes (username, password, email, firstName, lastName)
   - Links user to created organization
   - Extracts user ID from response

5. **Create Application** (API call #3)
   - Endpoint: `POST /applications`
   - Payload: OAuth configuration including:
     - Client ID and secret
     - Redirect URIs
     - Grant types: authorization_code, refresh_token
     - Response types: code
     - Token endpoint auth methods: client_secret_basic, client_secret_post
     - PKCE: disabled (pkce_required: false)
   - Extracts application ID from response

6. **Generate Output** (registration.yaml)
   - Organization details
   - User credentials (KEEP SECURE!)
   - Application OAuth configuration
   - OAuth endpoints
   - Example authorization URL
   - Test commands for token exchange

### Configuration Files

#### inputs.yaml

User-provided configuration with defaults:

```yaml
# Organization Configuration
name: "Acme Corporation"
handle: "acme"
description: "Acme Corporation"

# User Configuration
username: "admin"
password: "Admin@123"
email: "admin@acme.com"
firstName: "Admin"
lastName: "User"
type: "superhuman"

# Application Configuration
application_name: "Management Portal"
application_description: "Management Portal Application"
client_id: "management-portal-client"
# client_secret: auto-generated if not provided

# OAuth Redirect URIs
redirect_uris:
  - "https://localhost:3000/callback"
  - "https://localhost:3001/callback"
```

#### registration.yaml

Generated output with all created resources:

```yaml
# STS Registration Output
# Generated: 2025-10-11T...

organization:
  id: "d713e47e-0a92-4608-b8c6-f069fbd37805"
  name: "Acme Corporation"
  handle: "acme"
  description: "Acme Corporation"
  created_at: "2025-10-11T..."

user:
  id: "1e7e977f-e956-41a8-b8d4-9b6b886ce49c"
  username: "admin"
  email: "admin@acme.com"
  firstName: "Admin"
  lastName: "User"
  organization_unit: "d713e47e-..."
  created_at: "2025-10-11T..."

  # Credentials (KEEP SECURE!)
  password: "Admin@123"

application:
  id: "1f78eac6-e8d2-48f2-b037-e42f2d114e84"
  name: "Management Portal"
  description: "Management Portal Application"
  client_id: "management-portal-client"
  client_secret: "c531589f187567e2c8533bd496b0a5e03c676a0d9364bdde62163966e2eec24d"
  auth_flow_graph_id: "auth_flow_config_basic"
  created_at: "2025-10-11T..."

  redirect_uris:
    - "https://localhost:3000/callback"
    - "https://localhost:3001/callback"

  grant_types:
    - "authorization_code"
    - "refresh_token"

  response_types:
    - "code"

# OAuth 2.0 Endpoints
oauth_endpoints:
  authorize: "https://localhost:8090/oauth2/authorize"
  token: "https://localhost:8090/oauth2/token"
  userinfo: "https://localhost:8090/oauth2/userinfo"

# Example Authorization URL
example_auth_url: "https://localhost:8090/oauth2/authorize?response_type=code&client_id=management-portal-client&redirect_uri=https://localhost:3000/callback&scope=openid&state=random_state_123"

# Quick Test Commands
test_commands:
  authorize: "Open the example_auth_url in a browser and login with user credentials"
  token: |
    curl -k -X POST https://localhost:8090/oauth2/token \
      -u management-portal-client:<client_secret> \
      -d "grant_type=authorization_code" \
      -d "code=<authorization_code>" \
      -d "redirect_uri=https://localhost:3000/callback"
```

### Key Technical Decisions

1. **Standalone Script (Not in Docker)**
   - Runs on host machine after container starts
   - Uses exposed ports (no docker exec needed)
   - No container name dependency
   - Simpler debugging and modification

2. **No External Dependencies**
   - Basic grep/sed for YAML parsing
   - jq optional (falls back to grep if unavailable)
   - Portable across different environments

3. **Sensible Defaults**
   - Every field has a default value
   - Works without inputs.yaml
   - Auto-generates client secret if not provided

4. **Not Idempotent by Design**
   - Fails on second run (creates duplicate resources)
   - User explicitly requested this behavior
   - Prevents accidental re-creation

5. **Colored Output**
   - RED for errors
   - GREEN for success
   - YELLOW for warnings
   - BLUE for info
   - Improves user experience

6. **PKCE Disabled**
   - `pkce_required: false` in application config
   - Simplifies OAuth flow for basic use cases
   - User can modify for production use

## Usage

### Prerequisites

```bash
# Start STS container
docker run -d --name sts-container -p 8090:8090 -p 9090:9090 wso2/api-platform-sts:latest
```

### Run Kickstart

```bash
cd sts
./kickstart.sh [inputs.yaml]
```

### Output

The script generates `registration.yaml` with:
- All resource IDs
- User credentials
- OAuth configuration
- Ready-to-use authorization URL
- Token exchange commands

### Next Steps (from script output)

1. Review registration.yaml for all details
2. Follow the Authorization Code Grant flow:
   - Open the example_auth_url in browser
   - Login with user credentials
   - Use authorization code to exchange for tokens

## OAuth 2.0 Flow Testing

After running kickstart:

### Step 1: Get Authorization Code

Open the `example_auth_url` from `registration.yaml`:

```
https://localhost:8090/oauth2/authorize?response_type=code&client_id=management-portal-client&redirect_uri=https://localhost:3000/callback&scope=openid&state=random_state_123
```

### Step 2: Login

Use credentials from `registration.yaml`:
- Username: admin
- Password: Admin@123

### Step 3: Exchange Code for Token

```bash
curl -k -X POST https://localhost:8090/oauth2/token \
  -u <client_id>:<client_secret> \
  -d "grant_type=authorization_code" \
  -d "code=<authorization_code>" \
  -d "redirect_uri=https://localhost:3000/callback"
```

Response includes:
- access_token
- refresh_token
- id_token
- expires_in

## Challenges & Solutions

### Challenge 1: Dockerfile Modification Rejected
**Problem**: Initially planned to add jq/yq to Docker image
**User feedback**: "Don't do changes to the existing docker file"
**Solution**: Created standalone script that runs on host

### Challenge 2: Container Name Dependency
**Problem**: Script initially used docker exec (needed container name)
**User feedback**: "no need to know about the sts container name"
**Solution**: Use exposed ports with curl from host

### Challenge 3: YAML Parsing Without Dependencies
**Problem**: Can't assume yq/jq availability on host
**Solution**: Implemented basic grep/sed parser for YAML

### Challenge 4: Wait for Thunder Readiness
**Problem**: API calls fail if Thunder not ready
**Solution**: Polling health endpoint with retries and delays

### Challenge 5: Correlating Multiple API Responses
**Problem**: Need IDs from previous responses for subsequent calls
**Solution**: Extract IDs using jq (if available) or grep/sed fallback

## Testing

### Successful Test Results

✅ Thunder health check passed
✅ Organization created with valid ID
✅ User created linked to organization
✅ Application created with OAuth config
✅ registration.yaml generated with all details
✅ Authorization URL works in browser
✅ Login succeeds with generated credentials
✅ Token exchange returns valid tokens

### Test Scenarios

1. **Default configuration**: Run without inputs.yaml
2. **Custom configuration**: Run with inputs.yaml
3. **Duplicate run**: Verify it fails on second execution
4. **Without jq**: Test grep/sed fallback parsing
5. **OAuth flow**: Complete authorization code flow

## Error Handling

The script handles:
- Thunder not ready (retry with timeout)
- API failures (clear error messages with response)
- Missing configuration (use defaults)
- jq not available (fallback to grep/sed)
- Invalid responses (exit with error)

## Security Considerations

⚠️ **IMPORTANT**: The `registration.yaml` file contains sensitive credentials:
- User password
- Client secret
- All resource IDs

**Recommendations**:
- Keep registration.yaml secure
- Don't commit to version control
- Use environment-specific credentials in production
- Enable PKCE for production applications
- Rotate secrets regularly

## Related Features

- [Initial Thunder Setup](./initial-thunder-setup.md) - Provides Thunder APIs used by kickstart
- [Gate App Integration](./gate-app-integration.md) - The runtime environment

## Future Enhancements

- Idempotent mode (update if exists)
- Support for multiple organizations/users/applications
- Automated OAuth flow testing
