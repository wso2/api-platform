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

