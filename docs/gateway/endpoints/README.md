# Global endpoints

An **Endpoint** is a backend that belongs to the gateway, not to one API. You create it once, and any API on the gateway can route requests to it through the `dynamic-endpoint` policy. Adding, changing, or removing an Endpoint never requires an API to be edited or redeployed.

This page is for the **platform administrator** who creates and maintains Endpoints, and the **API developer** who routes an API to them.

## When to use an Endpoint

Without Endpoints, a backend is part of an API. To make one more backend reachable, you add an entry to that API's `upstreamDefinitions`, point the `dynamic-endpoint` policy's `targetUpstream` at it, and redeploy the API. Use that when a backend is used by one API only.

Use an Endpoint when:

- Several APIs share a backend, and you want its URL, TLS settings, and timeout defined in one place.
- You add backends over time, such as one per tenant, per region, or for a canary, and you don't want to touch the APIs each time.
- A request should choose its own backend, from a set you approve.
- You need to move a backend or change its weights for every API that uses it at once.

| | `upstreamDefinitions` entry | Endpoint |
|---|---|---|
| Belongs to | One API | The gateway |
| Created by | The API developer, in the API | The platform administrator |
| Adding one | Edit and redeploy the API | Create the Endpoint; no API changes |
| Selected by | `targetUpstream` in the policy | `targetEndpoint`, or a request header |

The two can be mixed in one API.

## How it works

Each Endpoint becomes one cluster in the router, named `endpoint_<name>`. The name is stable for the life of the Endpoint, so:

- Changing an Endpoint's URLs, weights, TLS settings, or connect timeout replaces only that cluster.
- Changing its `basePath` or labels updates only the copy the policy engine holds.
- In both cases no API's stored configuration changes, and no route changes.

The policy engine knows the name, base path, and labels of every Endpoint. The `dynamic-endpoint` policy tells it which Endpoint a request goes to, and the gateway sends the request to that Endpoint's cluster with the Endpoint's base path prefixed.

A route only lets a policy choose its backend when its own policy chain routes to Endpoints. An API that doesn't use them behaves exactly as before.

## Create an Endpoint

An Endpoint has the same backend fields as an `upstreamDefinitions` entry.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: Endpoint
metadata:
  name: tenant-a
  labels:
    team: orders
spec:
  displayName: Orders backend for tenant A
  basePath: /v1
  upstreams:
    - url: http://orders-tenant-a:9080
  timeout:
    connect: 2s
```

```bash
curl -u admin:<password> -H 'Content-Type: application/yaml' \
  --data-binary @tenant-a.yaml \
  http://localhost:9090/api/management/v1/endpoints
```

You can also apply the file with the CLI:

```bash
ap gateway apply -f tenant-a.yaml
```

### Fields

| Field | Required | Description |
|---|---|---|
| `metadata.name` | Yes | The Endpoint's name. Lowercase letters and digits in hyphen-separated segments, 3 to 40 characters (`^[a-z0-9]+(-[a-z0-9]+)*$`). It can't be changed after the Endpoint is created. |
| `metadata.labels` | No | Key-value pairs. A policy's `endpointSelector` matches against them to decide which Endpoints an API may reach. Keys must not contain whitespace. |
| `spec.displayName` | No | A human-readable name, up to 253 characters. |
| `spec.upstreams[].url` | Yes | A backend URL with scheme, host, and optional port. It must not include a path, query string, or fragment. All targets of one Endpoint must use the same scheme. |
| `spec.upstreams[].weight` | No | Relative load-balancing weight, 0 to 100, across the Endpoint's targets. |
| `spec.basePath` | No | A path prefixed to every request sent to this Endpoint. It must start with `/` and must not end with `/`. Omit it to forward from the root. |
| `spec.timeout.connect` | No | Connection timeout, such as `2s` or `500ms`. It must be positive. |
| `spec.tls` | No | Mutual TLS presentation and trust for this backend: `identity`, `trustedCAs`, and `verifyHostName`. Every target must be `https://`. See [Connect to backends with mutual TLS](../mutual-tls/connect-to-backends-with-mtls.md). |

### Who can do what

| Operation | Roles |
|---|---|
| `POST /endpoints`, `PUT /endpoints/{id}`, `DELETE /endpoints/{id}` | `admin` |
| `GET /endpoints`, `GET /endpoints/{id}` | `admin`, `developer` |

An Endpoint is shared by every API, so only an administrator can change one. A developer can list Endpoints to see the names and labels available to route to.

## Route an API to Endpoints

Routing uses the `dynamic-endpoint` policy, version 1.1 or later. The policy is released separately from the gateway, in the [gateway-controllers](https://github.com/wso2/gateway-controllers) repository; check the version your gateway build includes with `GET /policies`. A gateway that has the Endpoint kind but an older `dynamic-endpoint` policy rejects the parameters below when you deploy the API.

**A fixed Endpoint.** Every request goes to one named Endpoint.

```yaml
policies:
  - name: dynamic-endpoint
    version: v1
    params:
      targetEndpoint: tenant-default
```

**Chosen per request.** The request names the Endpoint in a header. `endpointSelector` limits the choice to Endpoints carrying the given labels.

```yaml
policies:
  - name: dynamic-endpoint
    version: v1
    params:
      endpointHeader: x-endpoint
      endpointSelector:
        matchLabels:
          team: orders
      targetEndpoint: tenant-default   # optional: used when the header is absent
```

```bash
curl -H 'x-endpoint: tenant-a' http://localhost:8080/orders/v1.0/items
```

Attach the policy at the API level to apply it to every operation, or to individual operations. Place it last in a chain: if several policies in a chain set a routing target, the last one wins.

### Restrict which Endpoints an API can reach

A request header is chosen by the caller, so treat `endpointSelector` as a security boundary. Label Endpoints by who may use them, and select on those labels in each API. An empty selector (`{}`) lets the API reach every Endpoint on the gateway; write it only when that is what you mean.

## Change an Endpoint

Send the complete Endpoint to `PUT /endpoints/{name}`, or apply the file again with the CLI. Every API that routes to the Endpoint follows the change. The name can't be changed; create a new Endpoint instead.

A new or changed backend takes a moment to become reachable while the router resolves and connects to it. Requests that arrive in that window can receive a `503`.

## Delete an Endpoint

```bash
curl -u admin:<password> -X DELETE http://localhost:9090/api/management/v1/endpoints/tenant-a
```

The gateway refuses to delete an Endpoint that a deployed API names in `targetEndpoint`, and tells you which APIs:

```
409 {"status":"error","message":"Endpoint 'tenant-default' is the fixed target of deployed APIs: orders-v1.0"}
```

Remove the reference from those APIs first.

The gateway can't check the same thing for Endpoints chosen by header, because that choice is made per request. After you delete such an Endpoint, a request that names it is rejected. It is never sent to another backend.

## Order doesn't matter

You can create Endpoints and deploy APIs in any order, including from files in immutable mode. An API that names an Endpoint that doesn't exist yet is accepted, and the gateway logs a warning. Requests on those routes receive a `503` with error code `905008` until the Endpoint is created, and then succeed with no redeploy.

## Certificates

An Endpoint's `tls` block names certificates from the gateway's certificate pool, the same way an upstream definition does. The gateway checks that each name refers to an entry of the right usage when you create or update the Endpoint, and refuses to delete a certificate or a gateway identity while an Endpoint still names it.

## Run more than one controller

When several gateway controller replicas share a database, an Endpoint created, changed, or deleted through one replica takes effect on all of them.

## Troubleshooting

| Symptom | Cause |
|---|---|
| `400` when creating an Endpoint | The name doesn't follow the name rule, a URL includes a path, or targets mix `http` and `https`. The message lists each problem. |
| `409` when creating an Endpoint | An Endpoint with that name already exists. Use `PUT` to change it. |
| `409` when deleting an Endpoint | A deployed API names it in `targetEndpoint`. |
| `503` with error code `905008` | A policy routes to an Endpoint that doesn't exist. Create it. |
| `503` shortly after creating or changing an Endpoint | The router hasn't finished connecting to the new backend. |
| The API deploy is rejected with an unknown parameter | The gateway build includes a `dynamic-endpoint` policy older than 1.1. |
