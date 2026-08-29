# Getting Started

The Countries API returns country, continent, and language reference data over GraphQL. A single
request returns exactly the fields you ask for, so there is no need to stitch together several
REST calls or discard fields you did not want.

## Endpoint

```
https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-prod.e1-us-east-azure.choreoapis.dev/godzilla/countries-service-nodejs/v1.0
```

## Your first query

Every field below is optional — ask for the ones you need and nothing else.

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

Send it with `curl`:

```bash
curl -X POST "https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-prod.e1-us-east-azure.choreoapis.dev/godzilla/countries-service-nodejs/v1.0" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ countries(filter:{continent:{eq:\"EU\"}}){ code name capital currency } }"}'
```

## What you can query

| Query | Returns |
|---|---|
| `countries(filter:)` | Every country, optionally filtered |
| `country(code:)` | One country by ISO 3166-1 alpha-2 code, e.g. `LK` |
| `continents(filter:)` | Every continent |
| `continent(code:)` | One continent by code, e.g. `EU` |
| `languages(filter:)` | Every language |
| `language(code:)` | One language by ISO 639-1 code, e.g. `en` |

This API is read-only — the schema declares no mutations.

## Filtering

`countries`, `continents`, and `languages` each take a filter whose fields accept `eq`, `ne`,
`in`, `nin`, and `regex`:

```graphql
query {
  countries(filter: { currency: { eq: "EUR" } }) {
    name
    currency
  }
}
```

Combine filters to narrow further — this returns European countries that use the euro:

```graphql
query {
  countries(filter: { continent: { eq: "EU" }, currency: { eq: "EUR" } }) {
    name
    capital
  }
}
```

## Looking up a single record

Use `country`, `continent`, or `language` when you already know the code. They return a single
object rather than a list:

```graphql
query {
  country(code: "LK") {
    name
    native
    capital
    currency
    continent {
      name
    }
    languages {
      code
      name
    }
  }
}
```

## Source

The backing service is published at
[wso2/api-platform-samples/countries-api](https://github.com/wso2/api-platform-samples/tree/main/countries-api).
