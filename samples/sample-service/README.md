# Sample Service

A simple HTTP service that returns request details (method, path, query, headers, body) as JSON. Used as a backend for testing the API Gateway.

## Endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/health` | GET | Health check, returns `{"status": "healthy"}` |
| `/sandbox/whoami` | GET | Sandbox identity endpoint, returns deterministic sandbox payload for routing tests |
| `/` | Any | Echoes request info (method, path, query, headers, body) and stores it as the last captured request |
| `/captured-request` | GET | Returns the last request captured by `/`. Returns `204 No Content` if no request has been received yet |

### Query Parameters

| Parameter | Description |
|---|---|
| `statusCode` | If set to a valid integer, the service responds with that HTTP status code (e.g. `?statusCode=500`). Defaults to 200 if omitted or invalid. |

### Example Response

```
curl http://localhost:8080/pets?id=1
```

```json
{
  "method": "GET",
  "path": "/pets",
  "query": "id=1",
  "headers": {
    "Accept": ["*/*"],
    "User-Agent": ["curl/8.7.1"]
  }
}
```

### Custom Status Code Example

```
curl -v http://localhost:8080/pets?statusCode=503
```

Returns HTTP 503 with the usual request info JSON body.

### Captured Request Example

Send a request to any path first:

```
curl -X POST http://localhost:8080/api/v1/echo \
  -H "Content-Type: application/json" \
  -d '{"user": "alice"}'
```

Then retrieve the captured request:

```
curl http://localhost:8080/captured-request
```

```json
{
  "method": "POST",
  "path": "/api/v1/echo",
  "headers": {
    "Content-Type": ["application/json"],
    "User-Agent": ["curl/8.7.1"]
  },
  "body": "{\"user\": \"alice\"}"
}
```

Returns `204 No Content` if no request has been captured yet.

## Flags

| Flag | Default | Description |
|---|---|---|
| `-addr` | `:8080` | Server listen address |
| `-pretty` | `false` | Pretty print JSON responses |

## Build

```bash
make build
```

## Run

```bash
make run

# With flags
make run ARGS="-pretty -addr :9080"
```

## Test

```bash
make test
```

## Release

Build and push multi-arch image (amd64, arm64) to registry:

```bash
make release
```
