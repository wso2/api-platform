# Reading-List-API — Open Access Sample

This sample API tracks a personal reading list. Each book has a title, an author, and a reading status (`to_read`, `reading`, or `read`).

It is the simplest of the sample APIs: **no API key and no subscription token** are required, so you can call it from the try-out console straight away.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/reading-list/v1.0/books` | List every book on the reading list |
| `POST` | `/reading-list/v1.0/books` | Add a book |
| `GET` | `/reading-list/v1.0/books/{id}` | Get a single book by id |
| `PUT` | `/reading-list/v1.0/books/{id}` | Update a book — e.g. move it to `read` |
| `DELETE` | `/reading-list/v1.0/books/{id}` | Remove a book |

## Example

```bash
curl http://localhost:8080/reading-list/v1.0/books
```

```json
{
  "books": [
    {
      "id": "1d4c9647-5e62-4f1d-9c30-e1f25c6d0e73",
      "title": "The Great Gatsby",
      "author": "F. Scott Fitzgerald",
      "status": "read"
    }
  ]
}
```

A request for an unknown id returns `404` with an error body:

```json
{ "error": "UUID does not exist" }
```

## Gateway URLs

- **Sandbox:** `http://localhost:8080/reading-list/v1.0`
- **Production:** `http://localhost:8080/reading-list/v1.0`

## Registering the API on the gateway

The portal entry above describes an API that the gateway fronts. Register it with the gateway's management API:

```bash
curl -X POST http://localhost:9090/api/management/v1/rest-apis \
  -u admin:admin \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: reading-list-api-v1.0
spec:
  displayName: Reading-List-API
  version: v1.0
  context: /reading-list/$version
  upstream:
    main:
      url: https://apis.bijira.dev/samples/reading-list-api-service/v1.0
  policies:
    - name: set-headers
      version: v1
      params:
        request:
          headers:
            - name: x-wso2-apip-gateway-version
              value: v1.0.0
        response:
          headers:
            - name: x-environment
              value: development
  operations:
    - method: GET
      path: /books
    - method: POST
      path: /books
    - method: GET
      path: /books/{id}
    - method: PUT
      path: /books/{id}
    - method: DELETE
      path: /books/{id}
EOF
```

The `set-headers` policy adds `x-wso2-apip-gateway-version: v1.0.0` to every upstream request and `x-environment: development` to every response, so you can confirm a call actually traversed the gateway.
