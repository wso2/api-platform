# Catalog API — Token based subscription Sample

This sample API demonstrates **token-based subscription access** — the API has Gold and Bronze subscription plans, and callers must present a **subscription token** on every request. When you subscribe to a plan, the portal issues a subscription token; the gateway validates it (alongside your API key) on each call.

## Authentication

| Header | Required | Description |
|--------|----------|-------------|
| `X-API-Key` | Yes | Your API key, generated from the developer portal |
| `X-Subscription-Token` | Yes | The subscription token issued when you subscribe to a plan |

Both headers must be sent on every request.

## Subscription Plans

| Plan | Description |
|------|-------------|
| **Gold** | Higher rate limits and priority throughput |
| **Bronze** | Standard entry-level rate limits |

## How to consume

1. Subscribe to a **Gold** or **Bronze** plan in the developer portal — this issues your **subscription token**.
2. Generate an **API key** from the API's **Manage Keys** page.
3. Call the API with both the `X-API-Key` and `X-Subscription-Token` headers.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/catalog/products` | List all products (supports `categoryId` and `limit` filters) |
| `GET` | `/catalog/products/{productId}` | Get details of a specific product |
| `GET` | `/catalog/categories` | List all product categories |

## Gateway URLs

- **Sandbox:** `http://localhost:8080/catalog`
- **Production:** `http://localhost:8080/catalog`
