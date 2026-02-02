# LLM Provider Integration Test Plan

## Overview
This test plan provides comprehensive testing coverage for the LLM Provider Management API, ensuring all CRUD operations, access control mechanisms, upstream configurations, and invocation flows work correctly.

## Test Coverage Matrix

### 1. CRUD Operations (Happy Path)

| Test ID | Test Name | Description | Expected Result |
|---------|-----------|-------------|-----------------|
| CRUD-01 | Create LLM Provider | Create a new LLM provider with valid configuration | 201 Created, provider ID returned |
| CRUD-02 | Retrieve LLM Provider | Get an existing provider by ID | 200 OK, full provider configuration returned |
| CRUD-03 | Update LLM Provider | Update an existing provider's configuration | 200 OK, provider updated successfully |
| CRUD-04 | Delete LLM Provider | Remove an existing provider | 200 OK, provider deleted successfully |
| CRUD-05 | Verify Deletion | Attempt to retrieve deleted provider | 404 Not Found |

### 2. List and Filter Operations

| Test ID | Test Name | Description | Expected Result |
|---------|-----------|-------------|-----------------|
| LIST-01 | List All Providers | Retrieve all LLM providers | 200 OK, array of providers with count |
| LIST-02 | Filter by Display Name | Filter providers by displayName | 200 OK, filtered results only |
| LIST-03 | Filter by Version | Filter providers by version | 200 OK, filtered results only |
| LIST-04 | Filter by Context | Filter providers by context path | 200 OK, filtered results only |
| LIST-05 | Filter by VHost | Filter providers by virtual host | 200 OK, filtered results only |
| LIST-06 | Filter by Status | Filter providers by deployment status | 200 OK, filtered results only |

### 3. Access Control Testing

| Test ID | Test Name | Description | Expected Result |
|---------|-----------|-------------|-----------------|
| AC-01 | Allow All Mode | Create provider with allow_all access control | All paths accessible |
| AC-02 | Deny All Mode | Create provider with deny_all access control | All paths blocked by default |
| AC-03 | Deny with Exceptions | Create deny_all with path exceptions | Only exception paths accessible |
| AC-04 | Multiple Exceptions | Test multiple path/method exceptions | Each exception works independently |
| AC-05 | Method-specific Access | Test method-specific exceptions (GET vs POST) | Only specified methods allowed |

### 4. Upstream Configuration

| Test ID | Test Name | Description | Expected Result |
|---------|-----------|-------------|-----------------|
| UP-01 | API Key Auth | Configure upstream with api-key authentication | Requests include API key header |
| UP-02 | Bearer Token Auth | Configure upstream with bearer token | Requests include bearer token |
| UP-03 | Custom Header Auth | Configure custom authorization header | Requests include custom header |
| UP-04 | HTTPS Upstream | Configure HTTPS upstream URL | Provider accepts HTTPS URLs |
| UP-05 | HTTP Upstream | Configure HTTP upstream URL | Provider accepts HTTP URLs |

### 5. Virtual Host and Context Path

| Test ID | Test Name | Description | Expected Result |
|---------|-----------|-------------|-----------------|
| VH-01 | Custom Context Path | Set custom context path (e.g., /openai/v1) | Requests routed to correct context |
| VH-02 | Root Context | Use default root context (/) | Requests work at root level |
| VH-03 | Nested Context | Use nested path (e.g., /api/llm/openai) | Deep paths work correctly |
| VH-04 | VHost Configuration | Set virtual host (e.g., api.openai.local) | VHost stored and retrievable |
| VH-05 | VHost + Context | Combine vhost and context path | Both configurations applied |

### 6. Template Integration

| Test ID | Test Name | Description | Expected Result |
|---------|-----------|-------------|-----------------|
| TMP-01 | OpenAI Template | Use openai template | Template applied correctly |
| TMP-02 | Anthropic Template | Use anthropic template | Template applied correctly |
| TMP-03 | Azure OpenAI Template | Use azure-openai template | Template applied correctly |
| TMP-04 | Custom Template | Reference custom template | Provider uses custom template |
| TMP-05 | Template Validation | Use non-existent template | Validation error returned |

### 7. Policy Attachment

| Test ID | Test Name | Description | Expected Result |
|---------|-----------|-------------|-----------------|
| POL-01 | Single Policy | Attach one policy to provider | Policy applied to requests |
| POL-02 | Multiple Policies | Attach multiple policies | All policies applied in order |
| POL-03 | Path-specific Policy | Apply policy to specific paths | Policy only applies to specified paths |
| POL-04 | Method-specific Policy | Apply policy to specific methods | Policy only applies to specified methods |
| POL-05 | Policy Parameters | Configure policy with custom parameters | Parameters passed correctly |

### 8. Error Scenarios

| Test ID | Test Name | Description | Expected Result |
|---------|-----------|-------------|-----------------|
| ERR-01 | Missing Required Fields | Create provider without required fields | 400 Bad Request with validation errors |
| ERR-02 | Invalid API Version | Use incorrect apiVersion | 400 Bad Request |
| ERR-03 | Invalid Kind | Use incorrect kind value | 400 Bad Request |
| ERR-04 | Duplicate Provider | Create provider with existing name/version | 409 Conflict |
| ERR-05 | Invalid URL Format | Provide malformed upstream URL | 400 Bad Request |
| ERR-06 | Invalid Context Path | Use invalid context path (no leading /) | 400 Bad Request |
| ERR-07 | Invalid VHost Pattern | Use invalid virtual host format | 400 Bad Request |
| ERR-08 | Retrieve Non-existent | Get provider that doesn't exist | 404 Not Found |
| ERR-09 | Update Non-existent | Update provider that doesn't exist | 404 Not Found |
| ERR-10 | Delete Non-existent | Delete provider that doesn't exist | 404 Not Found |

### 9. End-to-End Invocation Tests

| Test ID | Test Name | Description | Expected Result |
|---------|-----------|-------------|-----------------|
| E2E-01 | Chat Completions | Create provider and invoke /chat/completions | 200 OK with chat response |
| E2E-02 | Embeddings | Create provider and invoke /embeddings | 200 OK with embeddings |
| E2E-03 | Models List | Create provider and invoke /models | 200 OK with models list |
| E2E-04 | Model Details | Create provider and invoke /models/{id} | 200 OK with model details |
| E2E-05 | Response Validation | Verify response structure matches OpenAI spec | Valid JSON with expected fields |
| E2E-06 | Token Tracking | Verify usage tokens in response | Token counts present and accurate |
| E2E-07 | Error Propagation | Send invalid request to upstream | Error properly propagated |
| E2E-08 | Auth Header Pass-through | Verify auth headers sent to upstream | Upstream receives correct headers |

### 10. Validation and Schema Tests

| Test ID | Test Name | Description | Expected Result |
|---------|-----------|-------------|-----------------|
| VAL-01 | Display Name Format | Test displayName pattern validation | Only valid characters accepted |
| VAL-02 | Version Format | Test version pattern (v\d+\.\d+) | Only valid versions accepted |
| VAL-03 | Context Path Format | Test context path validation | Paths must start with / |
| VAL-04 | Empty Access Control | Create without accessControl | 400 Bad Request |
| VAL-05 | Empty Upstream | Create without upstream configuration | 400 Bad Request |

## Component Definitions Testing

Based on the LLMProviderConfiguration schema, the following components are tested:

### Core Components
- ✅ **apiVersion**: `gateway.api-platform.wso2.com/v1alpha1`
- ✅ **kind**: `LlmProvider`
- ✅ **metadata**: name (RFC-1123 compliant)
- ✅ **spec**: All sub-components below

### Spec Components
- ✅ **displayName**: Human-readable name (pattern: `^[a-zA-Z0-9\-_\. ]+$`)
- ✅ **version**: Semantic version (pattern: `^v\d+\.\d+$`)
- ✅ **template**: Template reference
- ✅ **context**: Base path (pattern: `^\/([a-zA-Z0-9_\-\/]*[^\/])?$`)
- ✅ **vhost**: Virtual host (RFC-compliant hostname)
- ✅ **upstream**: URL and authentication
  - ✅ **url**: Upstream endpoint URL
  - ✅ **auth**: Authentication configuration
    - ✅ **type**: api-key | bearer
    - ✅ **header**: Header name
    - ✅ **value**: Authentication value
- ✅ **accessControl**: Access control configuration
  - ✅ **mode**: allow_all | deny_all
  - ✅ **exceptions**: Array of route exceptions
    - ✅ **path**: Path pattern
    - ✅ **methods**: Array of HTTP methods
- ✅ **policies**: Array of policy configurations (optional)
  - ✅ **name**: Policy name
  - ✅ **version**: Policy version
  - ✅ **paths**: Array of policy paths
    - ✅ **path**: Path to apply policy
    - ✅ **methods**: HTTP methods
    - ✅ **params**: Policy parameters (JSON Schema)

## Test Execution Strategy

### Pre-requisites
- Gateway services running (gateway-controller, router, policy-engine)
- Mock OpenAPI server available at http://mock-openapi:4010
- Admin authentication configured

### Test Groups Execution Order
1. **Setup**: Verify services are healthy
2. **CRUD Operations**: Test basic lifecycle
3. **List/Filter**: Test querying capabilities
4. **Access Control**: Test security mechanisms
5. **Upstream**: Test backend configurations
6. **VHost/Context**: Test routing
7. **Templates**: Test template integration
8. **Policies**: Test policy attachment
9. **E2E Invocation**: Test actual LLM requests
10. **Error Scenarios**: Test validation and error handling
11. **Cleanup**: Remove all test data

### Success Criteria
- All happy path scenarios pass
- All error scenarios return appropriate status codes
- No memory leaks or resource exhaustion
- Response times within acceptable limits (<2s for CRUD, <5s for invocation)
- All created resources are properly cleaned up

## Notes on Implementation

### Mock Backend
The tests use the Stoplight Prism mock server (`mock-openapi:4010`) which:
- Implements OpenAI-compatible endpoints
- Returns realistic responses based on OpenAPI spec
- Supports /chat/completions and /embeddings endpoints
- No actual LLM calls (deterministic responses)

### Wait Times
Some tests include `I wait for 2 seconds` after creating providers to allow:
- xDS configuration propagation to router
- Policy engine synchronization
- Route table updates

### Authentication
All management API calls use basic auth (`admin` user) while LLM invocations use the configured upstream authentication.

### Context Paths
Tests use unique context paths to avoid conflicts between concurrent test scenarios.

## Feature File Location
`/Users/nimsara/wso2/api-platform/gateway/it/features/llm-provider.feature`

## Step Definitions Location
`/Users/nimsara/wso2/api-platform/gateway/it/steps_llm.go`
