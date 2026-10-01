<h1 id="gateway-controller-management-api-certificate-management">Certificate Management</h1>

Manage custom TLS certificates for HTTPS upstream verification

## List all custom certificates

<a id="opIdlistCertificates"></a>

`GET /certificates`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/certificates \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Retrieve custom TLS certificates currently loaded in the certificate store.
Without a filter, every certificate is listed. With usage: upstream, the
certificates used to verify HTTPS backends; with usage: downstream, the
pooled client certificate authorities that authenticate mutual-TLS
callers; with usage: identity, the gateway identities presented to
backends.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="list-all-custom-certificates-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|usage|query|string|false|Filter the list to a single usage. Omit to list all certificates regardless of usage.|

#### Enumerated Values

|Parameter|Value|
|---|---|
|usage|upstream|
|usage|downstream|
|usage|identity|

> Example responses
>
> 200 Response

```json
{
  "certificates": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "partner-a-root",
      "subject": "CN=Partner A Root,O=Partner A,C=US",
      "issuer": "CN=Partner A Root,O=Partner A,C=US",
      "notAfter": "2026-11-26 06:07:26",
      "count": 1,
      "usage": "downstream",
      "role": "client",
      "isLeaf": false,
      "referencedByApis": 0,
      "message": "Certificate uploaded and SDS updated successfully",
      "status": "success"
    }
  ],
  "totalCount": 3,
  "totalBytes": 221599,
  "status": "success"
}
```

<h3 id="list-all-custom-certificates-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of certificates|[CertificateListResponse](schemas.md#schemacertificatelistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid usage filter|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Upload a new certificate

<a id="opIduploadCertificate"></a>

`POST /certificates`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/certificates \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Upload a new TLS certificate (PEM format) to the Gateway. The certificate is
loaded dynamically without restarting the Gateway. With usage: upstream (the
default), it is used for verifying HTTPS upstream connections. With usage:
downstream, it becomes a pooled client certificate authority used to
authenticate mutual-TLS callers. With usage: identity, it is a gateway
identity presented to backends. Each usage has its own trust bundle or
secret; they are never mixed.

> Payload

```json
{
  "name": "partner-a-root",
  "usage": "downstream",
  "certificate": "-----BEGIN CERTIFICATE-----\nMIIDXTCCAkWgAwIBAgIJAKL0UG+mRKtjMA0GCSqGSIb3DQEBCwUAMEUxCzAJBgNV\n...\n-----END CERTIFICATE-----\n"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="upload-a-new-certificate-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[CertificateUploadRequest](schemas.md#schemacertificateuploadrequest)|true|none|

> Example responses
>
> 201 Response

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "partner-a-root",
  "subject": "CN=Partner A Root,O=Partner A,C=US",
  "issuer": "CN=Partner A Root,O=Partner A,C=US",
  "notAfter": "2026-11-26 06:07:26",
  "count": 1,
  "usage": "downstream",
  "role": "client",
  "isLeaf": false,
  "referencedByApis": 0,
  "message": "Certificate uploaded and SDS updated successfully",
  "status": "success"
}
```

<h3 id="upload-a-new-certificate-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Certificate uploaded successfully|[CertificateResponse](schemas.md#schemacertificateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid certificate format|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|A certificate with this name already exists (names are one namespace across usages), or a role default identity is uploaded while another identity already has role default|[ErrorResponse](schemas.md#schemaerrorresponse)|
|413|[Payload Too Large](https://tools.ietf.org/html/rfc7231#section-6.5.11)|Request body exceeds the maximum allowed size|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Rotate a gateway identity's certificate and key

<a id="opIdupdateCertificate"></a>

`PUT /certificates/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v1/certificates/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Replace a usage: identity certificate's chain and private key in place, keeping its name and references intact. Only `certificate` and `privateKey` are read from the body; `name`, `usage`, `role` and `match` stay as stored. Refused for any other usage. Triggers an SDS snapshot update so newly-dialed upstream connections use the new identity.

> Payload

```json
{
  "name": "partner-a-root",
  "usage": "downstream",
  "certificate": "-----BEGIN CERTIFICATE-----\nMIIDXTCCAkWgAwIBAgIJAKL0UG+mRKtjMA0GCSqGSIb3DQEBCwUAMEUxCzAJBgNV\n...\n-----END CERTIFICATE-----\n"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="rotate-a-gateway-identity's-certificate-and-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|ID of the certificate to replace|
|body|body|[CertificateUploadRequest](schemas.md#schemacertificateuploadrequest)|true|none|

> Example responses
>
> 200 Response

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "partner-a-root",
  "subject": "CN=Partner A Root,O=Partner A,C=US",
  "issuer": "CN=Partner A Root,O=Partner A,C=US",
  "notAfter": "2026-11-26 06:07:26",
  "count": 1,
  "usage": "downstream",
  "role": "client",
  "isLeaf": false,
  "referencedByApis": 0,
  "message": "Certificate uploaded and SDS updated successfully",
  "status": "success"
}
```

<h3 id="rotate-a-gateway-identity's-certificate-and-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Certificate rotated successfully|[CertificateResponse](schemas.md#schemacertificateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid update (bad certificate/key, or the certificate is not usage identity)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Certificate not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a certificate

<a id="opIddeleteCertificate"></a>

`DELETE /certificates/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v1/certificates/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete a certificate from the Gateway. The change is applied dynamically without restarting the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="delete-a-certificate-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|ID of the certificate to delete|

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "Certificate deleted and SDS updated successfully",
  "id": "550e8400-e29b-41d4-a716-446655440000"
}
```

<h3 id="delete-a-certificate-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Certificate deleted successfully|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Certificate not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The certificate is still referenced by a deployed API and cannot
be removed: a client authority named in an mtls-auth accept list
(or the pool's last non-relay authority while a deployed API still
attaches mtls-auth), an upstream certificate named in an upstream
definition's tls.trustedCAs, or a gateway identity named in an
upstream definition's tls.identity.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-a-certificate-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|

## Manually reload certificates

<a id="opIdreloadCertificates"></a>

`POST /certificates/reload`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/certificates/reload \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Manually trigger a reload of all certificates from the filesystem into the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "Certificates reloaded and SDS updated successfully",
  "totalBytes": 221599
}
```

<h3 id="manually-reload-certificates-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Certificates reloaded successfully|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="manually-reload-certificates-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» totalBytes|integer|false|none|Total bytes of all loaded certificates|
