# Data Model: API Publishing to Developer Portal

**Feature**: 005-devportal-publish
**Date**: 2025-10-29
**Phase**: 1 (Design)

## Overview

This document defines the data structures for API publishing to the developer portal. **Note**: Database persistence is deferred to a future task. This phase focuses on configuration and in-memory data structures for HTTP communication.

## Configuration Models

### DevPortal Config

Configuration for developer portal connection, loaded via environment variables.

```go
// Package: config
// File: config/config.go

type DevPortal struct {
    Enabled    bool   `envconfig:"ENABLED" default:"false"`
    BaseURL    string `envconfig:"BASE_URL" default:"172.17.0.1:3001"`
    APIKey     string `envconfig:"API_KEY" default:""`
    Timeout    int    `envconfig:"TIMEOUT" default:"15"` // seconds
}
```

**Fields**:
- `Enabled`: Flag to enable/disable devportal integration (default: false)
- `BaseURL`: Developer portal API base URL (default: 172.17.0.1:3001 for Docker host network)
- `APIKey`: Authentication key for developer portal API
- `Timeout`: HTTP request timeout in seconds (default: 15)

**Usage**: Loaded automatically via `envconfig` in server initialization

---

## Platform API DTOs

Request/response models for platform-api REST endpoints.

### PublishAPIRequest

Request body for publishing an API to developer portal.

```go
// Package: dto
// File: internal/dto/api_dto.go

type PublishAPIRequest struct {
    DevPortalID string `json:"devportalId" binding:"required"`
}
```

**Fields**:
- `DevPortalID`: Identifier of the target developer portal (query parameter in spec, but can be body field)

**Validation**:
- `devportalId` is required

### PublishAPIResponse

Response body after successful API publication.

```go
// Package: dto
// File: internal/dto/api_dto.go

type PublishAPIResponse struct {
    Message        string `json:"message"`
    APIID          string `json:"apiId"`
    DevPortalRefID string `json:"devPortalRefId"`
    PublishedAt    string `json:"publishedAt"` // ISO 8601 timestamp
}
```

**Fields**:
- `message`: Human-readable success message
- `apiId`: Platform-api API identifier
- `devPortalRefId`: Developer portal reference ID for the published API
- `publishedAt`: Timestamp of publication (ISO 8601 format)

---

## Developer Portal Client DTOs

Data structures for communicating with developer portal API (multipart and JSON).

### OrganizationCreateRequest

Request for creating an organization in developer portal.

```go
// Package: devportal/dto
// File: internal/client/devportal/dto/organization_request.go

type OrganizationCreateRequest struct {
    ID          string `json:"id"`          // UUID from platform-api
    Name        string `json:"name"`
    DisplayName string `json:"displayName"`
    Description string `json:"description"`
}
```

**Fields**:
- `id`: Organization UUID (must match platform-api organization UUID)
- `name`: Organization name
- `displayName`: Human-readable display name
- `description`: Organization description

**HTTP Method**: `POST /devportal/organizations`
**Content-Type**: `application/json`

### OrganizationCreateResponse

Response from developer portal after organization creation.

```go
// Package: devportal/dto
// File: internal/client/devportal/dto/organization_request.go

type OrganizationCreateResponse struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    DisplayName string `json:"displayName"`
    CreatedAt   string `json:"createdAt"`
}
```

**Fields**:
- `id`: Created organization UUID
- `name`: Organization name
- `displayName`: Display name
- `createdAt`: Timestamp of creation

---

### SubscriptionPolicyCreateRequest

Request for creating a subscription policy in developer portal.

```go
// Package: devportal/dto
// File: internal/client/devportal/dto/subscription_policy_request.go

type SubscriptionPolicyCreateRequest struct {
    PolicyName   string `json:"policyName"`
    DisplayName  string `json:"displayName"`
    BillingPlan  string `json:"billingPlan"`
    Description  string `json:"description"`
    Type         string `json:"type"`
    TimeUnit     int    `json:"timeUnit"`
    UnitTime     string `json:"unitTime"`
    RequestCount int    `json:"requestCount"`
}
```

**Fields**:
- `policyName`: Policy identifier (e.g., "unlimited")
- `displayName`: Human-readable name (e.g., "Unlimited Tier")
- `billingPlan`: Billing plan type (e.g., "FREE")
- `description`: Policy description
- `type`: Policy type (e.g., "requestCount")
- `timeUnit`: Time unit value (e.g., 60)
- `unitTime`: Time unit string (e.g., "min")
- `requestCount`: Maximum requests allowed (e.g., 1000000)

**HTTP Method**: `POST /devportal/organizations/{orgId}/subscription-policies`
**Content-Type**: `application/json`

**Example**:
```json
{
  "policyName": "unlimited",
  "displayName": "Unlimited Tier",
  "billingPlan": "FREE",
  "description": "Allows unlimited requests per minute",
  "type": "requestCount",
  "timeUnit": 60,
  "unitTime": "min",
  "requestCount": 1000000
}
```

### SubscriptionPolicyCreateResponse

Response from developer portal after policy creation.

```go
// Package: devportal/dto
// File: internal/client/devportal/dto/subscription_policy_request.go

type SubscriptionPolicyCreateResponse struct {
    ID           string `json:"id"`
    PolicyName   string `json:"policyName"`
    DisplayName  string `json:"displayName"`
    CreatedAt    string `json:"createdAt"`
}
```

**Fields**:
- `id`: Created policy UUID
- `policyName`: Policy identifier
- `displayName`: Display name
- `createdAt`: Timestamp of creation

---

### APIPublishRequest

Multipart form-data request for publishing an API to developer portal.

```go
// Package: devportal/dto
// File: internal/client/devportal/dto/api_publish_request.go

type APIPublishRequest struct {
    // Form fields
    Name              string `form:"name"`
    Handle            string `form:"handle"`
    Version           string `form:"version"`
    Description       string `form:"description"`
    Visibility        string `form:"visibility"`         // PUBLIC, PRIVATE, RESTRICTED
    SubscriptionTier  string `form:"subscriptionTier"`   // "unlimited"
    OrganizationID    string `form:"organizationId"`
    ReferenceID       string `form:"referenceId"`        // Platform-API API ID

    // File field (multipart)
    OpenAPIDefinition []byte // Transmitted as file named "openapiDefinition"
}
```

**Fields**:
- `name`: API name
- `handle`: API handle (URL-friendly identifier)
- `version`: API version
- `description`: API description
- `visibility`: Visibility setting (PUBLIC, PRIVATE, RESTRICTED)
- `subscriptionTier`: Subscription policy name (hardcoded to "unlimited")
- `organizationId`: Organization UUID
- `referenceId`: Platform-API API identifier for tracking
- `OpenAPIDefinition`: OpenAPI specification file content (YAML/JSON)

**HTTP Method**: `POST /devportal/organizations/{orgId}/apis`
**Content-Type**: `multipart/form-data`

**Form Structure**:
```
--boundary
Content-Disposition: form-data; name="name"

Petstore API
--boundary
Content-Disposition: form-data; name="openapiDefinition"; filename="openapi.yaml"
Content-Type: application/x-yaml

<OpenAPI YAML content>
--boundary--
```

### APIPublishResponse

Response from developer portal after API publication.

```go
// Package: devportal/dto
// File: internal/client/devportal/dto/api_publish_request.go

type APIPublishResponse struct {
    ID             string `json:"id"`             // DevPortal API ID
    Name           string `json:"name"`
    Handle         string `json:"handle"`
    Version        string `json:"version"`
    ReferenceID    string `json:"referenceId"`    // Platform-API API ID
    OrganizationID string `json:"organizationId"`
    CreatedAt      string `json:"createdAt"`
}
```

**Fields**:
- `id`: Developer portal API identifier
- `name`: API name
- `handle`: API handle
- `version`: API version
- `referenceId`: Platform-API API identifier (for bi-directional tracking)
- `organizationId`: Organization UUID
- `createdAt`: Timestamp of creation

---

## Error Models

Structured error types for developer portal client operations.

### DevPortalError

Custom error type for developer portal client failures.

```go
// Package: devportal
// File: internal/client/devportal/devportal_client.go

type DevPortalError struct {
    Code       int    // HTTP status code
    Message    string // Error message
    Retryable  bool   // Whether error is retryable
    Underlying error  // Underlying error if any
}

func (e *DevPortalError) Error() string {
    return fmt.Sprintf("devportal error (%d): %s", e.Code, e.Message)
}
```

**Fields**:
- `Code`: HTTP status code from developer portal
- `Message`: Human-readable error message
- `Retryable`: Whether the error should trigger a retry
- `Underlying`: Wrapped underlying error

**Usage**: Returned by all developer portal client methods for consistent error handling

---

## Data Flows

### Organization Creation Flow

```
Platform-API Handler
    ↓
Platform-API Service (OrganizationService)
    ↓ [if devportal enabled]
DevPortal Client
    ↓ POST /devportal/organizations
    → OrganizationCreateRequest (JSON)
    ← OrganizationCreateResponse (JSON)
    ↓ POST /devportal/organizations/{orgId}/subscription-policies
    → SubscriptionPolicyCreateRequest (JSON)
    ← SubscriptionPolicyCreateResponse (JSON)
    ↓
Platform-API Service [return success]
    ↓
Platform-API Handler [return 201 Created]
```

### API Publishing Flow

```
Platform-API Handler (/apis/{id}/publish-to-devportal)
    ↓ PublishAPIRequest
Platform-API Service (APIService)
    ↓ [fetch API from repository]
    ↓ [build multipart request]
DevPortal Client
    ↓ POST /devportal/organizations/{orgId}/apis
    → APIPublishRequest (multipart/form-data)
    ← APIPublishResponse (JSON)
    ↓ [retry up to 3 times on failure]
Platform-API Service [return success]
    ↓ PublishAPIResponse
Platform-API Handler [return 200 OK]
```

---

## Validation Rules

### Organization Creation
- Organization UUID must be valid UUID format
- Name and DisplayName are required
- Description is optional

### Subscription Policy Creation
- PolicyName must be "unlimited" (hardcoded)
- RequestCount must be 1000000 (spec requirement)
- TimeUnit and UnitTime define rate limiting window

### API Publishing
- API must exist in platform-api before publishing
- OrganizationID must match existing organization
- Visibility must be one of: PUBLIC, PRIVATE, RESTRICTED
- SubscriptionTier must be "unlimited" (hardcoded)
- OpenAPI definition must be valid YAML/JSON

---

## Future Enhancements (Deferred)

The following data models will be added in future tasks:

### Database Persistence
- `devportal_config` table: Store developer portal configuration per organization
- `published_api_mappings` table: Track platform-api API ID ↔ devportal API ID mappings
- Foreign key constraints to ensure referential integrity

### Extended Models
- Support for multiple subscription tiers
- API lifecycle state tracking (published, updated, unpublished)
- Sync status tracking for organization creation