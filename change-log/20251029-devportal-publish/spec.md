# Feature Specification: API Publishing to Developer Portal

**Feature Branch**: `005-devportal-publish`
**Created**: 2025-10-29
**Status**: Draft
**Input**: User description: "Create a feature to publish APIs to developer portal from platform-api"

## Clarifications

### Session 2025-10-29

- Q: How should organization creation handle developer portal synchronization failures? → A: Block organization creation in platform-api until developer portal confirms success (synchronous) - only if developer portal is configured and enabled
- Q: What retry strategy should be used when developer portal is unreachable during API publishing? → A: Platform-api should retry exactly 3 times to publish the API to developer portal before completely blocking and failing
- Q: What timeout value should be used for developer portal HTTP requests? → A: 15 seconds per HTTP request (with 3 retries, total maximum 60 seconds)
- Q: What additional setup is needed during organization creation in developer portal? → A: Create a default "unlimited" subscription policy with 1000000 request count per minute for each new organization
- Q: What subscription policies should be used when publishing APIs to developer portal? → A: Hardcode "unlimited" subscription policy for all APIs published to developer portal

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Configure Developer Portal Connection (Priority: P1)

As a platform administrator, I want to configure the developer portal connection settings so that platform-api can communicate with the developer portal securely.

**Why this priority**: This is a prerequisite for all publishing functionality. Without proper configuration, no APIs can be published.

**Independent Test**: Can be tested by providing developer portal URL and API key in configuration, then verifying connectivity by attempting to retrieve organizations from the developer portal.

**Acceptance Scenarios**:

1. **Given** administrator provides valid developer portal URL and API key, **When** configuration is loaded, **Then** platform-api can successfully authenticate and communicate with the developer portal
2. **Given** developer portal is not configured, **When** administrator attempts to publish an API, **Then** the system provides a clear error message indicating developer portal is not configured
3. **Given** invalid API key is provided, **When** platform-api attempts to communicate with developer portal, **Then** authentication fails with a clear error message

---

### User Story 2 - Automatic Organization Synchronization (Priority: P1)

As a platform administrator, I want organizations created in platform-api to be automatically created in the developer portal with a default subscription policy so that API publishing happens within the correct organizational context.

**Why this priority**: Organizations provide multi-tenancy and access control boundaries. Without synchronized organizations and their default subscription policies, APIs cannot be properly scoped, isolated, or published between different tenants.

**Independent Test**: Can be tested by creating a new organization in platform-api and verifying that (1) the same organization with matching UUID exists in the developer portal, and (2) a default "unlimited" subscription policy has been created for that organization, enabling immediate API publishing without additional configuration.

**Acceptance Scenarios**:

1. **Given** a new organization is created in platform-api and developer portal is enabled, **When** the organization creation request is processed, **Then** platform-api blocks until developer portal confirms successful creation with matching UUID and default "unlimited" subscription policy
2. **Given** developer portal is enabled and organization creation fails in the developer portal, **When** the failure occurs, **Then** the entire organization creation in platform-api fails with a clear error message
3. **Given** developer portal is disabled or not configured, **When** a new organization is created in platform-api, **Then** organization creation completes immediately without attempting developer portal synchronization
4. **Given** an organization with the same UUID already exists in the developer portal, **When** platform-api attempts to create it, **Then** the organization creation fails with a duplicate error message

---

### User Story 3 - Publish API to Developer Portal (Priority: P1)

As an API administrator, I want to publish an API from the platform API to the developer portal so that developers can discover and subscribe to the API.

**Why this priority**: This is the core functionality that enables the entire API discovery and consumption workflow. Without this, APIs cannot be made available to developers.

**Independent Test**: Can be fully tested by creating an API in platform-api, invoking the publish endpoint with valid developer portal configuration, and verifying the API appears in the developer portal with correct metadata and delivers immediate value by making the API discoverable.

**Acceptance Scenarios**:

1. **Given** an API exists in platform-api and developer portal is configured, **When** administrator publishes the API to a specific developer portal, **Then** the API metadata and OpenAPI definition are created in the developer portal with matching organization context
2. **Given** a published API, **When** the API is retrieved from the developer portal, **Then** all metadata (name, version, description, endpoints, subscription policies) matches the source API in platform-api
3. **Given** multiple APIs in platform-api, **When** administrator publishes each API to the developer portal, **Then** each API is independently accessible in the developer portal with unique identifiers

---

### User Story 4 - Update Published API (Priority: P2)

As an API administrator, I want to update an already published API in the developer portal so that changes to API metadata, endpoints, or definitions are reflected for developers.

**Why this priority**: APIs evolve over time with updated documentation, endpoints, and policies. This ensures the developer portal stays synchronized with the latest API state.

**Independent Test**: Can be tested by first publishing an API, then modifying its metadata or definition in platform-api, re-publishing to the developer portal, and verifying the updates are reflected without creating duplicate entries.

**Acceptance Scenarios**:

1. **Given** an API is already published to the developer portal, **When** administrator updates the API metadata and re-publishes, **Then** the existing API in the developer portal is updated (not duplicated) with the new metadata
2. **Given** an API's OpenAPI definition has changed, **When** administrator publishes the updated API, **Then** the developer portal reflects the new API definition
3. **Given** an API's subscription policies have changed, **When** administrator re-publishes the API, **Then** the developer portal shows the updated subscription policies

---

### Edge Cases

- When the developer portal is unreachable during API publishing, system retries exactly 3 times before failing with error message
- Network timeouts are set to 15 seconds per HTTP request; with 3 retries, total maximum wait time is 60 seconds
- All APIs are published with the hardcoded "unlimited" subscription policy created during organization setup
- How does the system handle duplicate API handles in the developer portal?
- What happens when the OpenAPI definition file is malformed or exceeds size limits?
- What happens when an API is deleted from platform-api but still exists in the developer portal?
- What happens when developer portal is temporarily unavailable during organization creation?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow administrators to configure developer portal API URL, authentication credentials, and HTTP timeout (15 seconds) in platform-api configuration
- **FR-002**: System MUST automatically create organizations in the developer portal when organizations are created in platform-api, using the same UUID; if developer portal is configured and enabled, organization creation MUST block until developer portal confirms success; if developer portal is disabled or not configured, organization creation proceeds without blocking
- **FR-016**: System MUST create a default "unlimited" subscription policy for each new organization in the developer portal with policy name "unlimited", display name "Unlimited Tier", billing plan "FREE", request count 1000000 per minute
- **FR-003**: System MUST provide an endpoint to publish a specific API to a designated developer portal
- **FR-004**: System MUST transmit complete API metadata including name, version, description, endpoints, visibility, owners, and subscription policies when publishing
- **FR-005**: System MUST transmit the OpenAPI definition file when publishing an API to the developer portal
- **FR-006**: System MUST detect whether an API already exists in the developer portal and update it rather than create a duplicate
- **FR-007**: System MUST validate developer portal connectivity before attempting to publish APIs
- **FR-008**: System MUST provide clear error messages when developer portal is not configured or unreachable
- **FR-009**: System MUST use the developer portal's multipart form-data API format for creating/updating APIs
- **FR-010**: System MUST include organization context when publishing APIs to ensure proper multi-tenancy isolation
- **FR-011**: System MUST map platform-api API identifiers to developer portal reference IDs for tracking purposes
- **FR-012**: System MUST handle publishing failures gracefully without corrupting platform-api data; when developer portal is unreachable during API publishing, system MUST retry exactly 3 times before failing the publish operation with a clear error message
- **FR-013**: System MUST support publishing APIs with different visibility settings (PUBLIC, PRIVATE, RESTRICTED)
- **FR-014**: System MUST preserve API ownership information when publishing to the developer portal
- **FR-015**: System MUST assign the "unlimited" subscription policy to all APIs published to developer portal regardless of policies configured in platform-api

### Key Entities

- **Developer Portal Configuration**: Configuration settings including API base URL, API key, HTTP request timeout (15 seconds), and enabled/disabled status
- **Published API Mapping**: Relationship tracking between platform-api API identifiers and developer portal API identifiers for synchronization
- **Organization Synchronization Record**: Tracking which organizations have been synchronized between platform-api and developer portal

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Administrators can successfully publish an API to the developer portal in under 10 seconds for typical API definitions
- **SC-002**: Published APIs are immediately visible and searchable in the developer portal
- **SC-003**: 100% of API metadata fields (name, version, description, endpoints, policies) are accurately transferred to the developer portal
- **SC-004**: Organizations created in platform-api appear in the developer portal within 5 seconds
- **SC-005**: API updates published from platform-api are reflected in the developer portal without creating duplicate entries
- **SC-006**: System provides clear error feedback for 100% of publishing failures (network errors, authentication failures, validation errors)
- **SC-007**: Administrators can configure developer portal connection in under 2 minutes
