# STS Implementation Guide

## Implementation Overview

This guide outlines the steps to create the STS Docker image that packages Thunder and the Gate App together.

## Implementation Steps

### Step 1: Base Thunder Image

Start with the Thunder Docker image as the foundation.

**Reference**: [Thunder Documentation](https://github.com/asgardeo/thunder/blob/main/README.md)

**Tasks**:
- Use official Thunder Docker image or build from source
- Configure Thunder to run in Docker mode
- Verify Thunder core functionality

### Step 2: Include Gate App

Integrate the Gate App (authentication UI) into the Docker image.

**Tasks**:
- Build the Gate App Next.js application
- Copy Gate App build artifacts into the Docker image
- Configure Gate App to communicate with Thunder
- Set up proper networking between Gate App and Thunder

### Step 3: Configuration

Configure both components to work together.

**Tasks**:
- Configure Thunder endpoints for Gate App
- Set up Gate App environment variables to point to Thunder
- Configure pre-registered applications (e.g., management-portal)
- Set up default organization and user configurations (if needed)

### Step 4: Container Orchestration

Ensure both Thunder and Gate App run together in the container.

**Tasks**:
- Create startup script to launch both services
- Configure port mappings
- Set up health checks
- Configure logging

### Step 5: Testing

Verify the integrated system works correctly.

**Tasks**:
- Test Thunder OAuth 2.0 / OIDC flows
- Test Gate App login flow
- Test Gate App registration flow
- Test organization and user management
- Verify token generation includes organization and user IDs

## Technical Requirements

### Gate App Requirements

- Node.js 20+
- PNPM 10+
- Next.js build output

### Docker Requirements

- Multi-stage build for optimization
- Proper layer caching
- Minimal final image size
- Clear documentation for configuration

## Deliverables

1. **Dockerfile** - Complete Docker build configuration
2. **Startup Scripts** - Scripts to launch both Thunder and Gate App
3. **Configuration Files** - Default configurations for both components
4. **Documentation** - Deployment and usage instructions

## Reference

Based on [Thunder Docker Setup](https://github.com/asgardeo/thunder/blob/main/README.md)
