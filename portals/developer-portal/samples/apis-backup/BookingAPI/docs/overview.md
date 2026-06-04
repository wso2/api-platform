# Booking API — Token-Based Subscription Sample

This sample API demonstrates **subscription-based access with a per-request subscription token**. It models a hotel and event booking service with tiered subscription plans.

Every request requires two headers: an API key (for identity) and a subscription token (issued at subscription time).

## Authentication

| Header | Required | Description |
|--------|----------|-------------|
| `X-API-Key` | Yes | Your API key, obtained from the developer portal |
| `X-Subscription-Token` | Yes | Token issued when you subscribed to a plan |

## Subscription Plans

| Plan | Description |
|------|-------------|
| **Gold** | Highest rate limits, priority support |
| **Silver** | Standard rate limits |
| **Bronze** | Entry-level rate limits |

Subscribe to a plan in the portal to receive your `X-Subscription-Token`. Both the API key and subscription token must be present on every request, or the gateway will reject the call with `401 Unauthorized`.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/booking/bookings` | List all bookings for the account |
| `POST` | `/booking/bookings` | Create a new booking |
| `GET` | `/booking/bookings/{bookingId}` | Get details of a specific booking |
| `DELETE` | `/booking/bookings/{bookingId}` | Cancel a booking |

## Gateway URLs

- **Sandbox:** `http://localhost:8080/booking`
- **Production:** `http://localhost:8080/booking`
