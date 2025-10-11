# STS Architecture

## High-Level Architecture

The STS is built on the foundation of [Asgardeo Thunder](https://github.com/asgardeo/thunder), an OAuth 2.0 / OIDC authorization server, with an integrated authentication UI (Gate App) packaged together in a single Docker container.

## Architecture Components

```
┌─────────────────────────────────────────┐
│         Docker Container                │
│                                         │
│  ┌────────────────┐  ┌──────────────┐   │
│  │   Gate App     │  │    Thunder   │   │
│  │  (Next.js UI)  │  │    (Core)    │   │
│  │                │  │              │   │
│  │  - Login       │  │ - OAuth 2.0  │   │
│  │  - Register    │  │ - OIDC       │   │
│  │                │  │ - Token Mgmt │   │
│  └────────────────┘  └──────────────┘   │
│                                         │
└─────────────────────────────────────────┘
```

### 1. Thunder Core

- OAuth 2.0 / OpenID Connect authorization server
- Token generation and management
- User and organization management
- Application registration and management
- Runs in Docker mode

### 2. Gate App

- Next.js-based authentication application
- Provides user-facing authentication flows:
  - User login
  - User registration
- Communicates with Thunder Core for authentication operations

### 3. Docker Container

- Single container packaging both Thunder and Gate App
- Simplifies deployment and distribution
- Ensures both components run together with proper configuration

## Deployment Model

**Standalone Docker Container**: Both Thunder and the Gate App run within a single Docker image, eliminating the need for separate deployment of the authentication UI.

## Integration Points

- **Gate App → Thunder**: Authentication requests, user management
- **External Apps → Thunder**: OAuth 2.0 / OIDC flows for token acquisition

## Reference

Based on [Asgardeo Thunder](https://github.com/asgardeo/thunder/blob/main/README.md)
