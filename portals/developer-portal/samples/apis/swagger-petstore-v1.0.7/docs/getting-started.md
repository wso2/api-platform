# Getting Started with the Petstore API

## Authentication

The API supports API keys and OAuth2:

- `api_key` header for simple authentication
- `petstore_auth` OAuth2 flow for pet operations

## Example requests

### Find pets by status

`GET /pet/findByStatus?status=available`

### Create a new pet

`POST /pet`

Body: `Pet` object

### Place an order

`POST /store/order`

### Get order by ID

`GET /store/order/{orderId}`
