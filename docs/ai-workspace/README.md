# AI Workspace Documentation

Documentation for self-hosted deployments of the AI Workspace control plane.

## Getting Started

- [Overview](overview.md) — What AI Workspace is and what it manages
- [Quick Start](../QUICKSTART.md) — Up and running in minutes with file-based auth
- [Configuration Reference](configuration.md) — All `config.toml` keys and environment variables

## Authentication & IDP Setup

- [Authentication Overview](authentication/README.md) — Choose between file-based and OIDC auth
- [File-Based Auth](authentication/file-based-auth.md) — Local users for demos and development
- [OIDC Auth](authentication/oidc-auth.md) — Connect any OIDC-compliant IDP
- [Asgardeo Setup](authentication/asgardeo-setup.md) — Step-by-step guide for WSO2 Asgardeo

## Features

- [AI Gateways](features/ai-gateways.md) — Register and manage gateway runtimes
- [LLM Provider Templates](features/llm-provider-templates.md) — Reusable blueprints for connecting LLM providers
- [LLM Providers](features/llm-providers.md) — Connect upstream LLM services
- [LLM Proxies](features/llm-proxies.md) — Application-facing proxy endpoints
- [MCP Proxies](features/mcp-proxies.md) — Model Context Protocol server proxies
- [GenAI Applications](features/genai-applications.md) — Group consumers under named applications
- [Policies](features/policies.md) — Guardrails, rate limits, and traffic controls
- [Global Policies for LLM Providers and Proxies](features/global-policies-llm-providers-proxies.md) — Policies shared across every operation
- [Insights](features/insights.md) — Usage analytics and monitoring
- [Data Plane → Control Plane Sync](bottom-up-ai-artifact-deployment-guide.md) - Syncing gateway-created AI artifacts up to the AI Workspace

## Production Setup

See [`production/README.md`](../production/README.md) for a complete Asgardeo + Platform API + AI Workspace production configuration walkthrough.
