# STS Product Requirements Document (PRD)

## Product Overview

The STS (Security Token Service) is a containerized authentication and authorization service that provides OAuth 2.0 / OIDC capabilities with an integrated authentication UI.

## Requirements Status

> **Note**: These are preliminary requirements as Thunder is still under active development.

## Functional Requirements

### FR1: Standalone Operation

**Requirement**: Thunder must run as a standalone application within a Docker container.

**Acceptance Criteria**:
- Thunder runs independently without external service dependencies
- All core OAuth 2.0 / OIDC functionality is operational
- Container can be started with minimal configuration

### FR2: Pre-configured Applications

**Requirement**: The system must include pre-registered applications such as management-portal.

**Acceptance Criteria**:
- Management portal application is registered by default
- Application credentials are available for immediate use
- Documentation provided for accessing pre-configured applications

### FR3: Organization Registration

**Requirement**: The system must support organization registration and management.

**Acceptance Criteria**:
- Ability to register new organizations
- Organizations can be managed (view, update, delete)
- Organization data persists across container restarts

### FR4: User Registration

**Requirement**: The system must allow user registration under a given organization.

**Acceptance Criteria**:
- Users can be registered and associated with an organization
- User credentials are stored securely
- Users can be managed within their organization context

### FR5: User Authentication and Token Generation

**Requirement**: Users must be able to obtain access tokens by logging in through the Gate App.

**Acceptance Criteria**:
- Users can log in via the Gate App UI
- Successful login returns a valid access token
- Access token contains:
  - Organization ID
  - User ID
  - Standard OAuth 2.0 / OIDC claims

### FR6: Authentication UI

**Requirement**: The system must provide a user-friendly authentication interface via the Gate App.

**Acceptance Criteria**:
- Login page functional and accessible
- Registration page functional and accessible
- Proper error handling and user feedback
- Responsive UI design

## Non-Functional Requirements

### NFR1: Deployment Simplicity

- Single Docker image for easy deployment
- Minimal configuration required to get started
- Clear documentation for deployment

## Success Metrics

1. Successful deployment with single `docker run` command
2. Successful user login and token acquisition
3. Token contains required organization and user information
4. Pre-configured applications accessible immediately
