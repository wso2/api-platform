# Reading-List-API — Open Access Sample

This sample API tracks a personal reading list. Each book has a title, an author, and a reading status (`to_read`, `reading`, or `read`).

It is the simplest of the sample APIs: **no API key and no subscription token** are required, so you can call it from the try-out console straight away.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/books` | List every book on the reading list |
| `POST` | `/books` | Add a book |
| `GET` | `/books/{id}` | Get a single book by id |
| `PUT` | `/books/{id}` | Update a book — e.g. move it to `read` |
| `DELETE` | `/books/{id}` | Remove a book |

## Example

```bash
curl https://apis.bijira.dev/samples/reading-list-api-service/v1.0/books
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

## Endpoints

- **Sandbox:** `https://apis.bijira.dev/samples/reading-list-api-service/v1.0`
- **Production:** `https://apis.bijira.dev/samples/reading-list-api-service/v1.0`

