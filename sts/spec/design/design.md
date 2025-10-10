# STS Design Specification

## Design Overview

The STS design focuses on integrating [Asgardeo Thunder](https://github.com/asgardeo/thunder) with a custom Gate App (authentication UI) into a single, deployable Docker container.

## Design Principles

1. **Single Container Deployment**: Package both Thunder and Gate App in one Docker image
2. **Minimal Configuration**: Pre-configured for immediate use
3. **Standalone Operation**: No external dependencies required for basic functionality
4. **Docker-First**: Designed to run in Docker mode from the start

## Component Design

### Thunder Integration

- Use Thunder as the base authorization server
- Run Thunder in Docker mode
- Leverage Thunder's OAuth 2.0 / OIDC capabilities
- Utilize Thunder's built-in organization and user management

### Gate App Integration

- Next.js application providing authentication UI
- Serves login and registration pages
- Integrates with Thunder APIs for authentication operations
- Packaged within the same Docker image as Thunder

## Container Structure

The Docker image includes:

1. **Thunder runtime** - The authorization server
2. **Gate App** - The authentication UI application
3. **Shared configuration** - Connect Gate App to Thunder
4. **Pre-configured applications** - Default apps like management-portal

## Design Decisions

### Why Single Container?

- Simplified deployment and distribution
- Ensures version compatibility between Gate App and Thunder
- Reduces operational complexity
- Single artifact to manage

### Why Thunder?

- Modern OAuth 2.0 / OIDC implementation
- Built-in multi-tenancy support
- Active development and maintenance
- Docker-ready architecture

## Reference

Based on [Asgardeo Thunder](https://github.com/asgardeo/thunder/blob/main/README.md)
