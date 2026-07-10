# Getting Started

The Countries GraphQL API lets you query country, continent, and language data in a single request.

## Example query

```graphql
query {
  countries(filter: { continent: { eq: "EU" } }) {
    code
    name
    capital
    currency
    emoji
    languages {
      name
    }
  }
}
```

Send the query to the production endpoint:

```bash
curl -X POST https://countries.trevorblades.com/graphql \
  -H "Content-Type: application/json" \
  -d '{"query":"{ countries(filter:{continent:{eq:\"EU\"}}){ code name capital } }"}'
```

## Filtering

All list fields accept an optional `filter` argument with a `StringQueryOperatorInput`:

| Operator | Description |
|----------|-------------|
| `eq` | Exact match |
| `ne` | Not equal |
| `in` | Match any of the given values |
| `nin` | Exclude all of the given values |
| `regex` | Regular expression match |
| `glob` | Glob pattern match |
