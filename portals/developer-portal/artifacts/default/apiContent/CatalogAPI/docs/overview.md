# Catalog API — Direct Subscription Sample

This sample API demonstrates **direct subscription access** — the API has Gold and Bronze subscription plans, but callers don't need to include any auth token in their requests. Access is controlled entirely by the gateway based on your subscription, with no additional headers required.

## Authentication

| Header | Required | Description |
|--------|----------|-------------|
| `X-API-Key` | Yes | Your API key, obtained from the developer portal |

No subscription token header is needed — the gateway validates your subscription plan via the API key automatically.

## Subscription Plans

| Plan | Description |
|------|-------------|
| **Gold** | Higher rate limits and priority throughput |
| **Bronze** | Standard entry-level rate limits |

Once your subscription is active, call the API directly — no API key or token header needed.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/catalog/products` | List all products (supports `categoryId` and `limit` filters) |
| `GET` | `/catalog/products/{productId}` | Get details of a specific product |
| `GET` | `/catalog/categories` | List all product categories |

## Gateway URLs

- **Sandbox:** `http://localhost:8080/catalog`
- **Production:** `http://localhost:8080/catalog`
