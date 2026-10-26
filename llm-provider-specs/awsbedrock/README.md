# AWS Bedrock built-in provider

The built-in `awsbedrock` provider fronts the native Amazon Bedrock Runtime
**Converse** (`POST /model/{modelId}/converse`) and **ConverseStream**
(`POST /model/{modelId}/converse-stream`) APIs.

| Artifact | Role |
|---|---|
| `llm-provider-specs/awsbedrock/openapi.yaml` | API description of Converse and ConverseStream, used by the portal for provider resources. |
| `gateway/gateway-controller/default-llm-provider-templates/awsbedrock-template.yaml` | Gateway template. Drives runtime extraction of tokens, cache usage, service tier, and model. |
| `platform-api/resources/default-llm-provider-templates/awsbedrock-template.yaml` | Platform API template. Drives the portal (auth form, logo, spec URL) and names the gateway template a deployment uses. |
| `gateway/examples/bedrock-provider.yaml` | Gateway provider using a Bedrock API key. |
| `gateway/examples/bedrock-provider-sigv4.yaml` | Gateway provider using SigV4 and the default AWS credential chain. |

## Upstream authentication

Bedrock Runtime accepts two credentials. Both work with the built-in provider.

### Amazon Bedrock API key (bearer)

The gateway sends `Authorization: Bearer <key>`. This is the template default:
the portal asks for the key and prepends `Bearer `. In gateway YAML, include
the prefix yourself:

```yaml
upstream:
  url: https://bedrock-runtime.us-east-1.amazonaws.com
  auth:
    type: api-key
    header: Authorization
    value: Bearer <bedrock-api-key>   # prefer a managed secret reference
```

- A **short-term** key lasts up to 12 hours, or until the session that
  generated it ends, whichever comes first.
- A **long-term** key lasts until its configured expiry and works only in the
  Region where it was generated.
- The calling identity needs `bedrock:CallWithBearerToken`, plus the inference
  actions listed below.

### AWS Signature Version 4

The `aws-authentication` policy, which ships in the default gateway build,
signs each request with the signing name `bedrock`. Use
`authenticationType: default-credential-chain` so the policy-engine process
resolves credentials from its own environment: environment variables, shared
config, an EC2 instance profile, an ECS task role, or EKS Pod Identity. The AWS
SDK caches and refreshes these credentials. **Never put `awsAccessKeyID`,
`awsSecretAccessKey`, or `awsSessionToken` in provider configuration.**

**Gateway YAML.** Use the provider's upstream-auth slot:

```yaml
upstream:
  url: https://bedrock-runtime.us-east-1.amazonaws.com
  auth:
    type: other
    policyName: aws-authentication
    policyVersion: v0
    policyParams:
      service: bedrock
      region: us-east-1               # must match the Region in upstream.url
      authenticationType: default-credential-chain
```

The gateway appends the upstream-auth policy after every other policy on each
route. SigV4 needs this because the signature covers the request body, so it
must be computed after any policy that rewrites the body.

**Platform API / portal.** The Platform API upstream-auth model stores only a
type, header, and value, so it cannot carry `policyName` or `policyParams`.
Configure SigV4 like this instead:

1. Set the upstream auth type to `none`.
2. Attach `aws-authentication` (version `v0`) as an **operation policy** on
   `POST /model/{modelId}/converse` and `POST /model/{modelId}/converse-stream`,
   with the parameters shown above.
3. List it **after** every other operation policy on those routes that can
   modify the request body.

The portal's policy picker lists policies from the Policy Hub. If
`aws-authentication` isn't listed there, create the provider through the
Platform API.

**IAM.** The resolved identity needs `bedrock:InvokeModel` for Converse and
`bedrock:InvokeModelWithResponseStream` for ConverseStream, on the model or
inference profile being called. A prompt resource also needs
`bedrock:RenderPrompt`.

## Token exchange and key refresh

Bedrock short-term API keys are **not** OAuth tokens. AWS's token-generator
libraries (`aws-bedrock-token-generator`, `@aws/bedrock-token-generator`)
create them from the caller's own AWS credentials. AWS documents no token
endpoint, no grant, and no refresh token for them, so there is nothing for an
RFC 8693 token exchange to call.

- The generic `oauth-token-exchange` policy needs a `tokenEndpoint` and an
  authorization server that accepts RFC 8693 or RFC 7523. Bedrock offers
  neither, so the Bedrock template deliberately leaves that policy and its
  parameters out.
- **The gateway does not generate or refresh Bedrock API keys.** It sends the
  configured value as-is. A short-term key therefore stops working after at
  most 12 hours, and a long-term key at its expiry, until someone replaces
  the stored value.
- **For long-running deployments, use SigV4 with
  `default-credential-chain`.** The credentials then rotate with the
  environment's role, and nothing in the provider configuration expires.

## Model extraction

Bedrock carries the model only in the request path. Both templates use the
`pathParam` pattern `model/([A-Za-z0-9._:%-]+)/`, and the extractors
percent-decode the capture. It matches:

- base model IDs, such as `anthropic.claude-3-sonnet-20240229-v1:0`
- cross-Region and global inference profile IDs, such as
  `us.anthropic.claude-3-5-sonnet-20240620-v1:0`
- ARNs of foundation models, inference profiles, application inference
  profiles, provisioned or custom model deployments, imported models, and
  prompt versions, **sent percent-encoded as one path segment** (`:` as
  `%3A`, `/` as `%2F`), as the AWS SDKs send them

Limitations:

- An ARN sent with a raw `/` splits the path segment. It neither matches the
  `/model/{modelId}/converse` route nor extracts.
- Encoded slashes reach the route only with the router's default
  `path_with_escaped_slashes_action = KEEP_UNCHANGED`. `REJECT_REQUEST` or
  `UNESCAPE_AND_*` breaks ARN model IDs that contain `/`.
- For an application inference profile or prompt router, the extracted model
  is the ARN in the path, not the underlying model that served the request.

## Usage, cache, and service-tier mapping

The gateway template reads a Converse response body:

| Gateway field | Bedrock field |
|---|---|
| `promptTokens` | `usage.inputTokens`: non-cached input only |
| `completionTokens` | `usage.outputTokens` |
| `totalTokens` | `usage.totalTokens` |
| `cachedTokens` | `usage.cacheReadInputTokens` |
| `cacheWriteTokens` | `usage.cacheWriteInputTokens` |
| `providerFields.cacheDetails` | `usage.cacheDetails`: cache writes per TTL (`5m`, `1h`) |
| `serviceTier` | `serviceTier.type`: `priority` and `flex` map to their own rates; `default` and `reserved` map to standard |
| `cacheAccounting` | `additive`: total input = `inputTokens + cacheReadInputTokens + cacheWriteInputTokens` |

The fallback identifiers (`$.usage.input_tokens`, `$.usage.prompt_tokens`, and
so on) cover InvokeModel and OpenAI-transformed response shapes on the same
provider.

Live responses also carry fields the AWS API reference does not list:
`usage.cacheReadInputTokenCount` and `usage.cacheWriteInputTokenCount`
(duplicates of the documented cache fields), `usage.serverToolUsage`, a `p`
padding field on every ConverseStream event, and `Message` (capital M) on
403 errors from the authentication layer. The OpenAPI document follows the API
reference and leaves them out, and the templates read only documented fields.

### Intentional differences between the two templates

The Platform API template schema models only `promptTokens`,
`completionTokens`, `totalTokens`, `remainingTokens`, `requestModel`, and
`responseModel`, so the cache, service-tier, fallback, and provider-field
entries exist only in the gateway template. That causes no runtime gap: a
Platform API deployment names the gateway template by handle (`awsbedrock`),
and the gateway's copy drives extraction. The fields both templates declare
must stay identical; `TestAWSBedrockBuiltInTemplate_ConsistentWithPlatformAPI`
enforces this.

## Features the built-in integration does not cover

- **ConverseStream token, cache, and cost metering.** ConverseStream responds
  with AWS event-stream binary frames (`application/vnd.amazon.eventstream`),
  not server-sent events. Usage arrives in the final `metadata` frame. Neither
  the analytics system policy nor `sdk/ai/llmusage` decodes that framing, so
  streamed requests record the model (from the path) but no token counts, cache
  usage, service tier, or cost. Token- and cost-based rate limits read the same
  JSON body paths, so they do not count streamed usage either. (The
  `openai-to-bedrock-transformer` policy decodes event-stream frames, but only
  to convert responses for OpenAI-style proxies, not for native Converse
  routes.)
- **Other Bedrock Runtime operations.** InvokeModel, InvokeModelWithResponseStream,
  CountTokens, ApplyGuardrail, async invoke, and the OpenAI-compatible
  endpoints are not described in the OpenAPI document. The default access
  control allows only the two Converse routes.
- **Automatic generation or refresh of short-term Bedrock API keys.** See
  [Token exchange and key refresh](#token-exchange-and-key-refresh).
- **Nested shapes described by reference.** Citations, search results, the
  guardrail trace assessment, and audio sources are documented as open
  objects that link to the AWS API reference, rather than being restated
  field by field.
