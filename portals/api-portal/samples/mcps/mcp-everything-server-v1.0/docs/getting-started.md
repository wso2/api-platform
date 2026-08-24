# Getting Started

The Everything MCP Server implements as much of the Model Context Protocol as possible in one
place — tools, prompts, resources, and sampling. It is not meant to be useful in production; it
exists so you can point an MCP client at something that exercises every part of the protocol.

## Endpoint

```
https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-prod.e1-us-east-azure.choreoapis.dev/godzilla/mcp-everything-server/v1.0/mcp
```

This is a streamable HTTP MCP endpoint — note the trailing `/mcp`, which is part of the address.

## Tools

| Tool | Arguments | Does |
|---|---|---|
| `echo` | `message` | Returns the message unchanged |
| `add` | `a`, `b` | Returns the sum |
| `viewPizzaMenu` | — | Returns the available pizzas as JSON |
| `orderPizza` | `pizzaType`, `quantity`, `customerName`, `deliveryAddress`, `creditCardNumber` | Places an order and returns the order details, including an `orderId` |

`echo` and `add` are the quickest way to confirm a client is connected and passing arguments
correctly. `viewPizzaMenu` and `orderPizza` form a two-step flow: list the menu, then order from
it — a realistic shape for testing how an agent chains one tool's output into the next.

> The pizza order takes a credit card number because it mimics a real checkout. It is a sample
> server with no payment processing behind it, so use an obviously fake number such as
> `4111 1111 1111 1111` — never a real one.

## Prompts

| Prompt | Arguments | Returns |
|---|---|---|
| `simple_prompt` | — | A single message exchange |
| `complex_prompt` | `temperature` (required), `style` | A multi-turn conversation including images |
| `resource_prompt` | `resourceId` (required, 1-100) | A conversation with an embedded resource reference |

## Resources

The server exposes 100 numbered test resources under `test://static/resource/{n}`:

- **Even** numbers return plain text.
- **Odd** numbers return base64-encoded binary.

They page 10 at a time, support subscriptions, and any resource you subscribe to updates itself
every 5 seconds — which is what makes this server useful for testing a client's resource
subscription handling.

## Logging

The server emits a log message at a random level every 15 seconds, so a client that surfaces
`notifications/message` will show activity without you doing anything.

## Trying it from the portal

Open the MCP server's page and use the **MCP Playground** to connect and call a tool. `echo` is the
simplest round trip to start with.

## Source

The server is published at
[wso2/api-platform-samples/mcp-everything-server](https://github.com/wso2/api-platform-samples/tree/main/mcp-everything-server).
