<h1 id="gateway-controller-management-api-system">System</h1>

Health and readiness of the gateway controller

## Liveness check

<a id="opIdgetManagementHealth"></a>

`GET /health`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/health \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Returns the health status of the gateway controller, including whether its
database is reachable. Each dependency is reported under `checks`; a failing
check carries only a generic reason. The answer comes from the same cached
evaluation as `/ready` and as the admin server's `/api/admin/v1/health`.
It is served on whichever management listener is active (the plaintext port,
or the TLS port when `controller.server.tls` is enabled). The listener opens
only once startup has finished, so the connection is refused until then; the
admin server's health endpoints answer during startup.
This endpoint is exempt from authentication so that health probes and load
balancers can reach it. Only clients in `controller.health.allowed_clients`
get an answer; any other client gets 403. It has no legacy unprefixed alias.

> Example responses
>
> 200 Response

```json
{
  "status": "healthy",
  "timestamp": "2019-08-24T14:15:22Z",
  "checks": {
    "property1": {
      "status": "up",
      "error": "string",
      "since": "2019-08-24T14:15:22Z",
      "warning": "string"
    },
    "property2": {
      "status": "up",
      "error": "string",
      "since": "2019-08-24T14:15:22Z",
      "warning": "string"
    }
  }
}
```

<h3 id="liveness-check-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Gateway controller is healthy|[HealthResponse](schemas.md#schemahealthresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|The client is not in `controller.health.allowed_clients`|[HealthForbiddenResponse](schemas.md#schemahealthforbiddenresponse)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Gateway controller is unhealthy; at least one check is down|[HealthResponse](schemas.md#schemahealthresponse)|

## Readiness check

<a id="opIdgetManagementReady"></a>

`GET /ready`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/ready \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Returns whether the gateway controller is ready to serve, with the same checks
and answer as `/health`; both paths, and the admin server's
`/api/admin/v1/health` and `/api/admin/v1/ready`, are served from one cached
evaluation. It is served on whichever management listener is active and
refuses connections until startup has finished.
This endpoint is exempt from authentication so that health probes and load
balancers can reach it. Only clients in `controller.health.allowed_clients`
get an answer; any other client gets 403. It has no legacy unprefixed alias.

> Example responses
>
> 200 Response

```json
{
  "status": "healthy",
  "timestamp": "2019-08-24T14:15:22Z",
  "checks": {
    "property1": {
      "status": "up",
      "error": "string",
      "since": "2019-08-24T14:15:22Z",
      "warning": "string"
    },
    "property2": {
      "status": "up",
      "error": "string",
      "since": "2019-08-24T14:15:22Z",
      "warning": "string"
    }
  }
}
```

<h3 id="readiness-check-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Gateway controller is ready|[HealthResponse](schemas.md#schemahealthresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|The client is not in `controller.health.allowed_clients`|[HealthForbiddenResponse](schemas.md#schemahealthforbiddenresponse)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Gateway controller is not ready; at least one check is down|[HealthResponse](schemas.md#schemahealthresponse)|
