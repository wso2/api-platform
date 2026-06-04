# Ping API — API Key Sample

This sample API proxies an HTTP echo/probe service through the Platform Gateway and demonstrates **API key authentication** with no subscription plans.

Pass your API key in the `X-API-Key` header on every request.

## Authentication

| Header | Required | Description |
|--------|----------|-------------|
| `X-API-Key` | Yes | Your API key, obtained from the developer portal |

No subscription plan is needed — just generate an API key and start calling.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/ping/get` | Echoes query parameters and request headers |
| `POST` | `/ping/post` | Echoes the JSON request body |
| `GET` | `/ping/status/{code}` | Returns the specified HTTP status code |

## Gateway URLs

- **Sandbox:** `http://localhost:8080/ping`
- **Production:** `http://localhost:8080/ping`
