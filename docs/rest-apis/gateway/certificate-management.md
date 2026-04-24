<h1 id="gateway-controller-management-api-certificate-management">Certificate Management</h1>

Manage custom TLS certificates for HTTPS upstream verification

## List all custom certificates

<a id="opIdlistCertificates"></a>

`GET /certificates`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/certificates \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Retrieve all custom TLS certificates currently loaded in the certificate store.
These certificates are used for verifying HTTPS upstream connections.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

> Example responses

> 200 Response

```json
{
  "certificates": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "my-custom-ca",
      "subject": "CN=My CA,O=My Organization,C=US",
      "issuer": "CN=My CA,O=My Organization,C=US",
      "notAfter": "2026-11-26 06:07:26",
      "count": 1,
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
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Upload a new certificate

<a id="opIduploadCertificate"></a>

`POST /certificates`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v0.9/certificates \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Upload a new TLS certificate (PEM format) to the Gateway. The certificate is loaded dynamically without restarting the Gateway.

> Payload

```json
{
  "name": "my-custom-ca",
  "certificate": "-----BEGIN CERTIFICATE-----\nMIIDXTCCAkWgAwIBAgIJAKL0UG+mRKtjMA0GCSqGSIb3DQEBCwUAMEUxCzAJBgNV\n...\n-----END CERTIFICATE-----\n"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="upload-a-new-certificate-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[CertificateUploadRequest](schemas.md#schemacertificateuploadrequest)|true|none|

> Example responses

> 201 Response

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "my-custom-ca",
  "subject": "CN=My CA,O=My Organization,C=US",
  "issuer": "CN=My CA,O=My Organization,C=US",
  "notAfter": "2026-11-26 06:07:26",
  "count": 1,
  "message": "Certificate uploaded and SDS updated successfully",
  "status": "success"
}
```

<h3 id="upload-a-new-certificate-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Certificate uploaded successfully|[CertificateResponse](schemas.md#schemacertificateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid certificate format|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a certificate

<a id="opIddeleteCertificate"></a>

`DELETE /certificates/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v0.9/certificates/{id} \
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

curl -X POST http://localhost:9090/api/management/v0.9/certificates/reload \
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
