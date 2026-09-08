/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/secrets"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/certificate"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/subscription"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/version"
)

// McpHandler serves the Model Context Protocol endpoint. Built once at startup and reused for every request.
type McpHandler struct {
	// protected is the SDK's Streamable HTTP handler wrapped in the per-tool
	// authorization gate. Everything the mux serves goes through this field.
	protected http.Handler

	restAPIService       *restapi.RestAPIService
	mcpDeploymentService *utils.MCPDeploymentService
	llmDeploymentService *utils.LLMDeploymentService
	secretService        *secrets.SecretService
	apiKeyService        *utils.APIKeyService
	kinds                map[string]*kindOps

	// Certificates and subscriptions are deliberately NOT in the kind registry.
	// They carry no manifest envelope, are identified by a server-generated UUID
	// rather than a metadata.name handle, and have verbs kindOps cannot express
	// (a certificate "reload" with no CRUD analogue). Each gets one
	// action-dispatched tool instead, holding its service directly.
	certificateService  *certificate.CertificateService
	subscriptionService *subscription.SubscriptionService

	// resourceRoles is the management API's route -> roles map, exactly as built
	// by generateAuthConfig in cmd/controller/main.go. MCP calls are authorized
	// against it by relative key ("POST /llm-providers"), so the REST API and
	// the MCP endpoint can never drift apart. See mcp_authz.go.
	resourceRoles map[string][]string

	// roleMapping is auth.idp.role_mapping (local role -> IdP claim values),
	// used ONLY to translate outbound: resourceRoles and every authorization
	// decision stay in local-role vocabulary, while anything a client sees —
	// a WWW-Authenticate `scope` value, a tool-layer scope error — is projected
	// through scopesFor into the IdP's vocabulary. Nil or empty means the two
	// vocabularies are identical, which is the no-role_mapping default.
	roleMapping map[string][]string

	// resourceMetadataURL is the RFC 9728 document URL advertised in
	// WWW-Authenticate challenges. Empty disables the pointer.
	resourceMetadataURL string

	// immutable mirrors ImmutableGateway.Enabled. The MCP route is deliberately
	// NOT wrapped in the immutable middleware (that middleware rejects every
	// POST, which would disable reads too, since every MCP call is a POST), so
	// the write tools enforce the invariant themselves.
	immutable bool

	maxRequestBytes int64
	logger          *slog.Logger

	// pushArtifactUndeploy is the shared control-plane push hook,
	// wired from APIServer so an artifact deleted through an MCP tool takes the
	// same path as one deleted through the REST handlers. Nil when no control
	// plane is configured (e.g. in unit tests).
	pushArtifactUndeploy func(cfg *models.StoredConfig, log *slog.Logger)
}

// McpHandlerParams collects newMcpHandler's dependencies. A struct rather than
// a positional list because the handler needs services, policy data and config
// from three different layers.
type McpHandlerParams struct {
	RestAPIService       *restapi.RestAPIService
	MCPDeploymentService *utils.MCPDeploymentService
	LLMDeploymentService *utils.LLMDeploymentService
	SecretService        *secrets.SecretService
	APIKeyService        *utils.APIKeyService
	CertificateService   *certificate.CertificateService
	SubscriptionService  *subscription.SubscriptionService
	ResourceRoles        map[string][]string
	RoleMapping          map[string][]string
	ResourceMetadataURL  string
	Immutable            bool
	MaxRequestBytes      int64
	Logger               *slog.Logger
}

// newMcpHandler builds the MCP server, registers the six tools, and wraps the
// SDK handler in the authorization gate.
func newMcpHandler(p McpHandlerParams) *McpHandler {
	h := &McpHandler{
		restAPIService:       p.RestAPIService,
		mcpDeploymentService: p.MCPDeploymentService,
		llmDeploymentService: p.LLMDeploymentService,
		secretService:        p.SecretService,
		apiKeyService:        p.APIKeyService,
		certificateService:   p.CertificateService,
		subscriptionService:  p.SubscriptionService,
		resourceRoles:        p.ResourceRoles,
		roleMapping:          p.RoleMapping,
		resourceMetadataURL:  p.ResourceMetadataURL,
		immutable:            p.Immutable,
		maxRequestBytes:      p.MaxRequestBytes,
		logger:               p.Logger,
	}
	if h.maxRequestBytes <= 0 {
		h.maxRequestBytes = mcp.DefaultMaxRequestBodyBytes
	}

	// Built after h exists because each kindOps closes over h's services.
	h.kinds = h.buildKindRegistry()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "wso2-api-platform-gateway-controller",
		Version: version.Version,
	}, nil)
	h.registerTools(server)

	streamHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			// MCP 2026-07-28 removed protocol-level sessions and the GET
			// stream. Stateless mode implements exactly that: no
			// Mcp-Session-Id is read or minted, and GET/DELETE answer 405.
			Stateless: true,
			// The gate reads the body first; keep the SDK's ceiling identical
			// so the two layers reject oversized bodies at the same threshold.
			MaxRequestBodyBytes: h.maxRequestBytes,
			// A closed HTTP request means the response can no longer be
			// delivered, so cancelling the in-flight handler is safe and stops
			// work on an abandoned deploy.
			PropagateRequestCancellation: true,
			Logger:                       p.Logger,
		},
	)

	h.protected = h.ScopeGate(streamHandler)
	return h
}

// ServeHTTP implements http.Handler. Registered on the mux in main.go behind
// the authentication and baseline authorization middleware.
func (h *McpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.protected.ServeHTTP(w, r)
}

// -------------------------------------------------------------------------
// Tool inputs
//
// Typed inputs, not hand-parsed maps: the SDK generates the input schema from
// these structs and validates arguments against it before the handler runs, so
// a malformed call never reaches a service. Fields without `omitempty` are
// required in the generated schema.
// -------------------------------------------------------------------------

type deployInput struct {
	Yaml string `json:"yaml" jsonschema:"complete resource manifest in YAML or JSON; must declare apiVersion, kind, metadata and spec"`
	ID   string `json:"id,omitempty" jsonschema:"handle (metadata.name) of an existing resource; present means update, absent means create"`
}

type deleteInput struct {
	Kind    string `json:"kind" jsonschema:"resource kind"`
	ID      string `json:"id" jsonschema:"resource handle (metadata.name)"`
	Confirm bool   `json:"confirm" jsonschema:"must be true; guards against accidental deletion"`
}

type getInput struct {
	Kind string `json:"kind" jsonschema:"resource kind"`
	ID   string `json:"id" jsonschema:"resource handle (metadata.name)"`
}

type listInput struct {
	Kind string `json:"kind,omitempty" jsonschema:"restrict to one kind; omit for a cross-kind inventory"`
}

// API key inputs.
//
// expiresAt is a plain RFC 3339 string rather than the generated request type's
// ExpiresIn: that field is an anonymous inline struct in generated.go, which a
// tool input struct cannot name. An absolute timestamp is also less ambiguous
// for a model than a duration+unit pair.

type issueKeyInput struct {
	Kind      string `json:"kind" jsonschema:"parent resource kind; one of RestApi, LlmProvider, LlmProxy"`
	ID        string `json:"id" jsonschema:"handle (metadata.name) of the parent resource the key is issued against"`
	KeyName   string `json:"keyName" jsonschema:"human-readable name for the new key"`
	ExpiresAt string `json:"expiresAt,omitempty" jsonschema:"expiry as an RFC 3339 timestamp, e.g. 2027-01-31T23:59:59Z; omit for a key that never expires"`
}

type listKeysInput struct {
	Kind string `json:"kind" jsonschema:"parent resource kind; one of RestApi, LlmProvider, LlmProxy"`
	ID   string `json:"id" jsonschema:"handle (metadata.name) of the parent resource"`
}

type rotateKeyInput struct {
	Kind      string `json:"kind" jsonschema:"parent resource kind; one of RestApi, LlmProvider, LlmProxy"`
	ID        string `json:"id" jsonschema:"handle (metadata.name) of the parent resource"`
	KeyName   string `json:"keyName" jsonschema:"name of the existing key to rotate"`
	ExpiresAt string `json:"expiresAt,omitempty" jsonschema:"new expiry as an RFC 3339 timestamp; omit to keep the key's current expiry"`
	ApiKey    string `json:"apiKey,omitempty" jsonschema:"an externally generated key value to install under this name, at least 36 characters; omit to have the Gateway generate a new value instead"`
}

type revokeKeyInput struct {
	Kind    string `json:"kind" jsonschema:"parent resource kind; one of RestApi, LlmProvider, LlmProxy"`
	ID      string `json:"id" jsonschema:"handle (metadata.name) of the parent resource"`
	KeyName string `json:"keyName" jsonschema:"name of the key to revoke"`
	Confirm bool   `json:"confirm" jsonschema:"must be true; guards against accidental revocation"`
}

// Certificate and subscription inputs.
//
// These two tools dispatch on an "action" argument rather than exposing one
// tool per verb, because their resources are not artifact kinds: they have no
// manifest envelope to read a kind from, and certificates carry a "reload" verb
// with no CRUD counterpart. The action is resolved to the governing REST route
// key by resolveCertAction / resolveSubscriptionAction in mcp_authz.go, which
// the authorization gate and the handler both call.

type manageCertificatesInput struct {
	Action      string `json:"action" jsonschema:"the operation to perform: list, apply, delete or reload"`
	Name        string `json:"name,omitempty" jsonschema:"unique certificate name; required for apply, ignored otherwise"`
	Certificate string `json:"certificate,omitempty" jsonschema:"PEM-encoded X.509 certificate chain; required for apply. Include only CERTIFICATE blocks, never a private key"`
	ID          string `json:"id,omitempty" jsonschema:"certificate id as returned by the list action; required for delete"`
	Confirm     bool   `json:"confirm,omitempty" jsonschema:"must be true for delete and reload"`
}

type manageSubscriptionsInput struct {
	Type    string            `json:"type" jsonschema:"which resource to act on: Subscription or SubscriptionPlan"`
	Action  string            `json:"action" jsonschema:"the operation to perform: list, get, apply or delete"`
	ID      string            `json:"id,omitempty" jsonschema:"resource id; required for get and delete. On apply, present means update and absent means create"`
	Spec    *subscriptionSpec `json:"spec,omitempty" jsonschema:"the fields to write on apply. On the list action, apiId, applicationId and status act as filters instead"`
	Confirm bool              `json:"confirm,omitempty" jsonschema:"must be true for delete"`
}

// subscriptionSpec is the union of the fields a Subscription and a
// SubscriptionPlan accept. Every field is optional, and which ones apply
// depends on "type" — stated per field so the model does not have to infer it.
//
// A typed struct rather than an opaque YAML string (the shape deployInput.Yaml
// uses) for two reasons: the SDK generates a real input schema from it, and
// expiryTime stays a string parsed explicitly here. The generated request type
// declares that field as *time.Time, and a model writing "2027-01-01" would
// otherwise fail deep inside the JSON decoder with nothing actionable to say.
type subscriptionSpec struct {
	// Subscription fields.
	ApiId                 *string `json:"apiId,omitempty" jsonschema:"Subscription only: the API this subscription is against, as a deployment id or a handle (metadata.name). Required when creating. On the list action, narrows the result to one API"`
	ApplicationId         *string `json:"applicationId,omitempty" jsonschema:"Subscription only: application identifier. On the list action, narrows the result to one application"`
	SubscriptionToken     *string `json:"subscriptionToken,omitempty" jsonschema:"Subscription only: the opaque token the client will present. Required when creating. Ask the user for it — never invent one"`
	SubscriptionPlanId    *string `json:"subscriptionPlanId,omitempty" jsonschema:"Subscription only: id of the plan to apply. The plan must exist, be ACTIVE, and be offered by the API"`
	BillingCustomerId     *string `json:"billingCustomerId,omitempty" jsonschema:"Subscription only: billing customer identifier, for analytics"`
	BillingSubscriptionId *string `json:"billingSubscriptionId,omitempty" jsonschema:"Subscription only: billing subscription identifier, for analytics"`

	// SubscriptionPlan fields.
	PlanName           *string `json:"planName,omitempty" jsonschema:"SubscriptionPlan only: the plan's name. Required when creating"`
	BillingPlan        *string `json:"billingPlan,omitempty" jsonschema:"SubscriptionPlan only: associated billing plan identifier"`
	StopOnQuotaReach   *bool   `json:"stopOnQuotaReach,omitempty" jsonschema:"SubscriptionPlan only: whether to stop serving once the quota is reached. Defaults to true"`
	ThrottleLimitCount *int    `json:"throttleLimitCount,omitempty" jsonschema:"SubscriptionPlan only: request allowance per unit. Must be positive and supplied together with throttleLimitUnit"`
	ThrottleLimitUnit  *string `json:"throttleLimitUnit,omitempty" jsonschema:"SubscriptionPlan only: one of Min, Hour, Day, Month. Must be supplied together with throttleLimitCount"`
	ExpiryTime         *string `json:"expiryTime,omitempty" jsonschema:"SubscriptionPlan only: expiry as an RFC 3339 timestamp, e.g. 2027-01-31T23:59:59Z"`

	// Shared, with a different set of values per type.
	Status *string `json:"status,omitempty" jsonschema:"for Subscription one of ACTIVE, INACTIVE, REVOKED; for SubscriptionPlan one of ACTIVE, INACTIVE. On the list action, narrows the result to one status"`
}

// -------------------------------------------------------------------------
// Tool registration
//
// One tool per intent, not per kind. The kind comes from the manifest for the
// create/update tools and from an explicit argument for the rest, so
// supporting a new resource kind means one entry in mcp_kinds.go and no tool
// changes at all.
// -------------------------------------------------------------------------

func (h *McpHandler) registerTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:  "wso2_apip_gw_deploy_api",
		Title: "Deploy or update a routable resource",
		Description: `Create or update any routable resource on this Gateway.

The kind is read from the manifest's "kind" field — do not guess it and do not
pass it separately. Routable kinds: RestApi, Mcp, LlmProxy, LlmProvider.

For supporting configuration (LlmProviderTemplate, Secret) use wso2_apip_gw_apply_config
instead; this tool rejects it.

Omit "id" to create. Pass "id" (the existing resource's handle, i.e. its
metadata.name) to update — read the current state with wso2_apip_gw_get_resource first, then
submit the full updated manifest.

Authorization follows the equivalent management REST operation: LlmProvider
writes require the admin scope, other kinds accept the developer scope. A call
you lack the scope for is refused at the HTTP layer with a step-up challenge,
not by this tool.

The Gateway validates the manifest and returns the validation error when it is
rejected. Read that error, correct the manifest, and retry. Ask the user for any
value you do not have rather than inventing one.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Deploy or update a routable resource",
			DestructiveHint: ptr(false),
			IdempotentHint:  false,
			OpenWorldHint:   ptr(false),
		},
	}, h.deployAPI)

	mcp.AddTool(server, &mcp.Tool{
		Name:  "wso2_apip_gw_undeploy_api",
		Title: "Undeploy a routable resource",
		Description: `Take a routable resource offline and remove it from this Gateway.

Destructive. Confirm the exact kind and id with the user, then call with
confirm=true. Supported kinds: RestApi, Mcp, LlmProxy, LlmProvider.

For supporting configuration (LlmProviderTemplate, Secret) use wso2_apip_gw_delete_config
instead.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Undeploy a routable resource",
			DestructiveHint: ptr(true),
			IdempotentHint:  true,
			OpenWorldHint:   ptr(false),
		},
	}, h.undeployAPI)

	mcp.AddTool(server, &mcp.Tool{
		Name:  "wso2_apip_gw_apply_config",
		Title: "Create or update a supporting configuration",
		Description: `Create or update a supporting configuration on this Gateway.

Supporting configurations are referenced by routable resources but do not take
traffic themselves. Config kinds: LlmProviderTemplate, Secret.

For routable resources (RestApi, Mcp, LlmProxy, LlmProvider) use wso2_apip_gw_deploy_api
instead; this tool rejects them.

The kind is read from the manifest's "kind" field. Omit "id" to create; pass it
to update.

A Secret manifest carries its value in spec.value. The value is encrypted at rest
and can be read back with wso2_apip_gw_get_resource, so treat anything you read
from a Secret as credential material: give it to the user if they asked for it,
and do not repeat it or copy it into other manifests unnecessarily.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Create or update a supporting configuration",
			DestructiveHint: ptr(false),
			OpenWorldHint:   ptr(false),
		},
	}, h.applyConfig)

	mcp.AddTool(server, &mcp.Tool{
		Name:  "wso2_apip_gw_delete_config",
		Title: "Delete a supporting configuration",
		Description: `Delete a supporting configuration from this Gateway.

Destructive. Confirm the exact kind and id with the user, then call with
confirm=true. Config kinds: LlmProviderTemplate, Secret.

Routable resources may still reference the configuration; check with
wso2_apip_gw_list_resources before deleting. Deleting a Secret is irreversible —
its value cannot be recovered, only replaced with a new one.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Delete a supporting configuration",
			DestructiveHint: ptr(true),
			IdempotentHint:  true,
			OpenWorldHint:   ptr(false),
		},
	}, h.deleteConfig)

	mcp.AddTool(server, &mcp.Tool{
		Name:  "wso2_apip_gw_list_resources",
		Title: "List deployed resources",
		Description: `List resources deployed on this Gateway.

With "kind": returns the full manifest of every resource of that kind.
Without "kind": returns a cross-kind inventory — per-kind counts and a compact
summary (id, displayName, version, state) — limited to the kinds your scope
permits reading. Call again with a kind to get full manifests.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "List deployed resources",
			ReadOnlyHint: true,
		},
	}, h.listResources)

	mcp.AddTool(server, &mcp.Tool{
		Name:  "wso2_apip_gw_get_resource",
		Title: "Get one resource",
		Description: `Fetch the full manifest of one resource by kind and handle.

Call this before updating anything with wso2_apip_gw_deploy_api or wso2_apip_gw_apply_config so the
manifest you submit is based on the resource's current state.

Reading a Secret returns its decrypted value in spec.value, exactly as the
management REST API does. Treat that as credential material.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get one resource",
			ReadOnlyHint: true,
		},
	}, h.getResource)

	mcp.AddTool(server, &mcp.Tool{
		Name:  "wso2_apip_gw_issue_api_key",
		Title: "Issue an API key",
		Description: `Create a new API key against a key-bearing resource.

Key-bearing kinds: RestApi, LlmProvider, LlmProxy. Other kinds have no API keys
and are rejected.

IMPORTANT — the response contains the key in PLAIN TEXT, and this is the only
time it is ever available. The Gateway stores just a hash, so the value cannot be
retrieved again by any means. Give it to the user immediately and tell them to
store it somewhere safe. Do not repeat it in later turns, and do not write it
into a file, a manifest or a log.

The key is recorded as created by you, the calling user. Only that user can
rotate it later.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Issue an API key",
			DestructiveHint: ptr(false),
			IdempotentHint:  false,
			OpenWorldHint:   ptr(false),
		},
	}, h.issueAPIKey)

	mcp.AddTool(server, &mcp.Tool{
		Name:  "wso2_apip_gw_list_api_keys",
		Title: "List API keys",
		Description: `List the API keys issued against a key-bearing resource.

Key-bearing kinds: RestApi, LlmProvider, LlmProxy.

Key values come back masked (the last few characters only) and cannot be
unmasked — that is the stored form. Use this to find a key's name before
rotating or revoking it.

Unless you hold the admin scope, the listing shows only the keys you created.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "List API keys",
			ReadOnlyHint: true,
		},
	}, h.listAPIKeys)

	mcp.AddTool(server, &mcp.Tool{
		Name:  "wso2_apip_gw_rotate_api_key",
		Title: "Rotate an API key",
		Description: `Replace the value of an existing API key, optionally changing its expiry.

Either way the old value stops working immediately. Confirm the key name with the
user before calling — list them with wso2_apip_gw_list_api_keys if you are unsure.

Two modes, chosen by whether you pass "apiKey":

  - Omit "apiKey" — the Gateway generates a new value. The response contains it
    in PLAIN TEXT, and that is the only time it is ever available. Hand it to the
    user and do not repeat it.
  - Pass "apiKey" — installs a value you already have (at least 36 characters),
    for adopting a key issued elsewhere. The response shows only the masked form,
    since you supplied the value yourself. Do not invent a key to put here; use
    the generate mode unless the user gave you a specific value.

    This mode only works on a key that was originally issued outside this Gateway.
    A key created by wso2_apip_gw_issue_api_key was generated here, so its value
    cannot be overwritten — regenerate it instead.

Only the user who created a key can change it, in either mode. This is deliberate
and applies even to admins: a refusal here means the key belongs to someone else,
not that the call was malformed. Ask its creator to rotate it, or issue a new key
of your own instead.

Omitting expiresAt keeps the key's current expiry rather than clearing it.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Rotate an API key",
			DestructiveHint: ptr(true),
			IdempotentHint:  false,
			OpenWorldHint:   ptr(false),
		},
	}, h.rotateAPIKey)

	mcp.AddTool(server, &mcp.Tool{
		Name:  "wso2_apip_gw_revoke_api_key",
		Title: "Revoke an API key",
		Description: `Permanently revoke an API key.

Destructive and irreversible. Confirm the exact kind, resource id and key name
with the user, then call with confirm=true.

Note that success does NOT prove the key existed: a key that is missing, already
revoked, or attached to a different resource all report success. This is
deliberate, so the tool cannot be used to probe which key names exist. To check
what is actually present, call wso2_apip_gw_list_api_keys.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Revoke an API key",
			DestructiveHint: ptr(true),
			IdempotentHint:  true,
			OpenWorldHint:   ptr(false),
		},
	}, h.revokeAPIKey)

	// The last two tools are registered only when their service is wired, so a
	// gateway without one simply does not advertise it — the same way secretOps
	// returning nil drops the Secret kind from the registry rather than failing
	// at call time.
	if h.certificateService != nil {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "wso2_apip_gw_manage_certificates",
			Title: "Manage upstream TLS certificates",
			Description: `Manage the custom upstream TLS certificates this Gateway trusts.

One tool, four actions, chosen with "action":

  - list    Every stored certificate: id, name, subject, issuer, expiry, and how
            many certificates the stored chain holds. Start here — delete needs
            an id, and this is the only way to obtain one.
  - apply   Store a new certificate chain and push it to the router. Requires
            "name" and "certificate".
  - delete  Remove one certificate by "id" and push the resulting trust store.
            Requires confirm=true.
  - reload  Re-read every stored certificate and republish it to the router,
            changing nothing stored. Requires confirm=true, because it pushes to
            the running data plane.

"certificate" is PEM text. Include only CERTIFICATE blocks — never a private
key. Blocks of any other type are skipped rather than rejected, so a pasted key
would be stored without a warning. Put a whole chain in one PEM string.

apply only creates. There is no update: names are unique, so applying a name
that already exists is refused. To replace a certificate, delete it and apply
the new one. Do not pass "id" to apply.

reload is also the recovery path. If apply or delete reports that the change was
stored but could not be pushed to the router, the stored state is already
correct — call reload to retry the push rather than repeating the write.

Authorization follows the equivalent management REST operation: list accepts the
developer scope, every other action requires admin. A call you lack the scope
for is refused at the HTTP layer with a step-up challenge, not by this tool.`,
			Annotations: &mcp.ToolAnnotations{
				Title: "Manage upstream TLS certificates",
				// One tool spans read-only and destructive actions, so the hints
				// describe its worst case: a client that gates on
				// DestructiveHint should prompt before any call here.
				DestructiveHint: ptr(true),
				IdempotentHint:  false,
				OpenWorldHint:   ptr(false),
			},
		}, h.manageCertificates)
	}

	if h.subscriptionService != nil {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "wso2_apip_gw_manage_subscriptions",
			Title: "Manage subscriptions and subscription plans",
			Description: `Manage subscriptions and subscription plans on this Gateway.

Pick the resource with "type" and the operation with "action":

  type: Subscription       an application's access to one REST API
  type: SubscriptionPlan   the rate-limit and billing template a subscription uses

  - list    Every resource of that type. For Subscription, apiId, applicationId
            and status inside "spec" narrow the result.
  - get     One resource by "id".
  - apply   Omit "id" to create, pass it to update. Fields go in "spec".
  - delete  Remove one resource by "id". Requires confirm=true.

Only REST APIs bear subscriptions. spec.apiId accepts either a deployment id or
the API's handle (metadata.name), so the handle you already know is enough.

Creating a Subscription requires spec.apiId and spec.subscriptionToken. The
token is the credential the client will present; only its hash is stored. Ask
the user for it — never invent one.

Updating a Subscription changes its status and nothing else. The API,
application, plan and billing identifiers of an existing subscription cannot be
re-pointed; to change any of those, delete it and create a new one. A
SubscriptionPlan, by contrast, updates every field in "spec".

A plan named in spec.subscriptionPlanId must exist, be ACTIVE, and be one of the
plans the target API offers, or the subscription is refused.

Every operation here requires the admin scope. A call you lack the scope for is
refused at the HTTP layer with a step-up challenge, not by this tool.`,
			Annotations: &mcp.ToolAnnotations{
				Title:           "Manage subscriptions and subscription plans",
				DestructiveHint: ptr(true),
				IdempotentHint:  false,
				OpenWorldHint:   ptr(false),
			},
		}, h.manageSubscriptions)
	}
}

// Tool handlers
// A returned error becomes a tool execution error (isError: true) rather than a
// JSON-RPC protocol error, per the SDK's typed-handler contract and the MCP
// specification's error-handling rules: validation and business failures are
// what the model is expected to read and correct.
func (h *McpHandler) deployAPI(ctx context.Context, _ *mcp.CallToolRequest, in deployInput) (*mcp.CallToolResult, any, error) {
	return h.write(ctx, classRoutable, "wso2_apip_gw_deploy_api", in)
}

func (h *McpHandler) applyConfig(ctx context.Context, _ *mcp.CallToolRequest, in deployInput) (*mcp.CallToolResult, any, error) {
	return h.write(ctx, classConfig, "wso2_apip_gw_apply_config", in)
}

// write is the shared create/update path for wso2_apip_gw_deploy_api and wso2_apip_gw_apply_config. The
// two tools differ only in which class of kind they accept, so the body lives
// here once.
func (h *McpHandler) write(ctx context.Context, class kindClass, tool string, in deployInput) (*mcp.CallToolResult, any, error) {
	if h.immutable {
		return nil, nil, fmt.Errorf(
			"this Gateway runs in immutable mode; resources are loaded from disk at startup and cannot be changed at runtime")
	}

	env, err := readManifestEnvelope([]byte(in.Yaml))
	if err != nil {
		return nil, nil, err
	}
	ops, err := h.resolveKind(env.Kind, class)
	if err != nil {
		return nil, nil, err
	}

	method := http.MethodPost
	if in.ID != "" {
		method = http.MethodPut
	}
	if err := h.authorize(ctx, routeKey(method, ops, in.ID != "")); err != nil {
		return nil, nil, err
	}

	correlationID := uuid.NewString()
	log := h.logger.With(
		slog.String("correlation_id", correlationID),
		slog.String("source", "mcp"),
		slog.String("tool", tool),
		slog.String("kind", ops.Kind))

	// A bare manifest is always a create. An explicit id makes it an update and
	// must agree with metadata.name, so a mistyped id cannot overwrite the
	// wrong resource.
	if in.ID != "" {
		if env.Metadata.Name != "" && env.Metadata.Name != in.ID {
			return nil, nil, fmt.Errorf(
				"id %q does not match metadata.name %q in the manifest", in.ID, env.Metadata.Name)
		}
		resource, err := ops.Update(in.ID, []byte(in.Yaml), correlationID, log)
		if err != nil {
			log.Error("MCP update failed", slog.String("id", in.ID), slog.Any("error", err))
			// The validation detail is surfaced deliberately: it is how the
			// model learns to correct the manifest and retry.
			return nil, nil, fmt.Errorf("failed to update %s %q: %w", ops.Kind, in.ID, err)
		}
		return nil, map[string]any{
			"status": "success", "operation": "update",
			"kind": ops.Kind, "id": in.ID, "resource": resource,
		}, nil
	}

	resource, err := ops.Create([]byte(in.Yaml), correlationID, log)
	if err != nil {
		log.Error("MCP create failed", slog.Any("error", err))
		return nil, nil, fmt.Errorf("failed to create %s: %w", ops.Kind, err)
	}
	return nil, map[string]any{
		"status": "success", "operation": "create",
		"kind": ops.Kind, "resource": resource,
	}, nil
}

func (h *McpHandler) undeployAPI(ctx context.Context, _ *mcp.CallToolRequest, in deleteInput) (*mcp.CallToolResult, any, error) {
	return h.remove(ctx, classRoutable, "wso2_apip_gw_undeploy_api", in)
}

func (h *McpHandler) deleteConfig(ctx context.Context, _ *mcp.CallToolRequest, in deleteInput) (*mcp.CallToolResult, any, error) {
	return h.remove(ctx, classConfig, "wso2_apip_gw_delete_config", in)
}

func (h *McpHandler) remove(ctx context.Context, class kindClass, tool string, in deleteInput) (*mcp.CallToolResult, any, error) {
	if h.immutable {
		return nil, nil, fmt.Errorf(
			"this Gateway runs in immutable mode; resources are loaded from disk at startup and cannot be changed at runtime")
	}
	if !in.Confirm {
		return nil, nil, fmt.Errorf(
			"refusing to delete %s %q: confirm the kind and id with the user, then retry with confirm=true",
			in.Kind, in.ID)
	}

	ops, err := h.resolveKind(in.Kind, class)
	if err != nil {
		return nil, nil, err
	}
	if err := h.authorize(ctx, routeKey(http.MethodDelete, ops, true)); err != nil {
		return nil, nil, err
	}

	correlationID := uuid.NewString()
	log := h.logger.With(
		slog.String("correlation_id", correlationID),
		slog.String("source", "mcp"),
		slog.String("tool", tool),
		slog.String("kind", ops.Kind))

	if err := ops.Delete(in.ID, correlationID, log); err != nil {
		log.Error("MCP delete failed", slog.String("id", in.ID), slog.Any("error", err))
		// Deliberately generic: a delete failure is usually "not found", and
		// echoing the storage error would confirm what does and does not exist.
		return nil, nil, fmt.Errorf("failed to delete %s %q", ops.Kind, in.ID)
	}

	return nil, map[string]any{
		"status": "success", "kind": ops.Kind, "id": in.ID,
		"message": fmt.Sprintf("%s %q deleted successfully", ops.Kind, in.ID),
	}, nil
}

func (h *McpHandler) getResource(ctx context.Context, _ *mcp.CallToolRequest, in getInput) (*mcp.CallToolResult, any, error) {
	ops, err := h.resolveKind(in.Kind, classAny)
	if err != nil {
		return nil, nil, err
	}
	if err := h.authorize(ctx, routeKey(http.MethodGet, ops, true)); err != nil {
		return nil, nil, err
	}

	resource, err := ops.Get(in.ID)
	if err != nil {
		h.logger.Warn("MCP wso2_apip_gw_get_resource lookup failed",
			slog.String("kind", ops.Kind), slog.String("id", in.ID), slog.Any("error", err))
		return nil, nil, fmt.Errorf("%s with handle %q not found", ops.Kind, in.ID)
	}
	return nil, map[string]any{"kind": ops.Kind, "id": in.ID, "resource": resource}, nil
}

// listResources returns full manifests for one kind, or a cross-kind inventory.
// The cross-kind form lists only the kinds the caller may read, which the MCP
// specification explicitly permits ("The set MAY vary by the authorization
// presented on the request"). Silently omitting a kind the caller cannot read
// is better than failing the whole call over one of five kinds.
func (h *McpHandler) listResources(ctx context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	if in.Kind != "" {
		ops, err := h.resolveKind(in.Kind, classAny)
		if err != nil {
			return nil, nil, err
		}
		if err := h.authorize(ctx, routeKey(http.MethodGet, ops, false)); err != nil {
			return nil, nil, err
		}
		rows, err := ops.List()
		if err != nil {
			h.logger.Error("MCP wso2_apip_gw_list_resources failed",
				slog.String("kind", ops.Kind), slog.Any("error", err))
			return nil, nil, fmt.Errorf("failed to list resources of kind %s", ops.Kind)
		}
		items := make([]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, row.Resource)
		}
		return nil, map[string]any{"kind": ops.Kind, "count": len(items), "items": items}, nil
	}

	breakdown := make(map[string]any, len(h.kinds))
	total, readable := 0, 0
	for _, kind := range canonicalKinds() {
		ops, ok := h.kinds[kind]
		if !ok {
			continue
		}
		if err := h.authorize(ctx, routeKey(http.MethodGet, ops, false)); err != nil {
			continue // not readable by this caller — omit rather than fail
		}
		readable++
		rows, err := ops.List()
		if err != nil {
			h.logger.Error("MCP wso2_apip_gw_list_resources failed for kind",
				slog.String("kind", kind), slog.Any("error", err))
			breakdown[kind] = map[string]any{"error": "failed to list this kind"}
			continue
		}
		total += len(rows)
		breakdown[kind] = map[string]any{
			"count":     len(rows),
			"resources": rows, // Resource is json:"-" — summaries only here
		}
	}
	if readable == 0 {
		return nil, nil, fmt.Errorf("your credentials do not permit reading any resource kind on this Gateway")
	}

	return nil, map[string]any{
		"total": total,
		"kinds": breakdown,
		"hint": "Call wso2_apip_gw_list_resources again with a kind, or wso2_apip_gw_get_resource with a kind and id, " +
			"for full manifests. Certificates and subscriptions are not kinds and do not appear here - " +
			"use wso2_apip_gw_manage_certificates and wso2_apip_gw_manage_subscriptions for those respectively.",
	}, nil
}

// API key tools
//
// These call utils.APIKeyService directly, exactly as the REST handlers do. The
// service is HTTP-free and takes Kind as a plain string, so all three
// key-bearing kinds share one adapter (keyOps in mcp_kinds.go).

// beginKeyOp is the shared preamble for the four api-key tools: resolve the
// kind, authorize, then resolve the caller's identity.
//
// The order matters and mirrors the CRUD tools: authorization happens before any
// service call. The route key can only be composed after the kind resolves,
// because it is anchored on that kind's collection path — hence method and
// routeSuffix arriving separately rather than as a finished key.
func (h *McpHandler) beginKeyOp(ctx context.Context, rawKind, tool, method, routeSuffix string) (
	*kindOps, *commonmodels.AuthContext, *slog.Logger, string, error) {

	ops, err := h.resolveKeyBearing(rawKind)
	if err != nil {
		return nil, nil, nil, "", err
	}
	if err := h.authorize(ctx, keyRouteKey(method, ops, routeSuffix)); err != nil {
		return nil, nil, nil, "", err
	}
	caller, err := h.callerIdentity(ctx)
	if err != nil {
		return nil, nil, nil, "", err
	}

	correlationID := uuid.NewString()
	log := h.logger.With(
		slog.String("correlation_id", correlationID),
		slog.String("source", "mcp"),
		slog.String("tool", tool),
		slog.String("kind", ops.Kind))

	return ops, caller, log, correlationID, nil
}

// parseTimestamp converts a tool's RFC 3339 string into the pointer the
// generated request types use. An unparseable value is returned as an error
// rather than dropped: silently ignoring it would write a different lifetime
// than the user asked for. field names the argument, so the message points at
// the one the caller actually sent.
func parseTimestamp(field, raw string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf(
			"%s %q is not a valid RFC 3339 timestamp; use a form like 2027-01-31T23:59:59Z", field, raw)
	}
	return &t, nil
}

// immutableKeyWrite mirrors the guard on write/remove. The MCP route is excluded
// from the immutable middleware (it rejects every POST, which would disable
// reads too), but the REST api-key routes are NOT excluded — so
// POST /rest-apis/{id}/api-keys is already refused in immutable mode. Repeating
// the check here is parity, not a new policy.
func (h *McpHandler) immutableKeyWrite() error {
	if h.immutable {
		return fmt.Errorf(
			"this Gateway runs in immutable mode; API keys cannot be issued, rotated or revoked at runtime")
	}
	return nil
}

func (h *McpHandler) issueAPIKey(ctx context.Context, _ *mcp.CallToolRequest, in issueKeyInput) (*mcp.CallToolResult, any, error) {
	if err := h.immutableKeyWrite(); err != nil {
		return nil, nil, err
	}
	expiresAt, err := parseTimestamp("expiresAt", in.ExpiresAt)
	if err != nil {
		return nil, nil, err
	}

	ops, caller, log, correlationID, err := h.beginKeyOp(
		ctx, in.Kind, "wso2_apip_gw_issue_api_key", http.MethodPost, "")
	if err != nil {
		return nil, nil, err
	}

	req := api.APIKeyCreationRequest{Name: &in.KeyName, ExpiresAt: expiresAt}
	resp, err := ops.Keys.Issue(in.ID, req, caller, correlationID, log)
	if err != nil {
		log.Error("MCP API key creation failed",
			slog.String("id", in.ID), slog.Any("error", err))
		return nil, nil, keyOpError("issue an API key for", ops.Kind, in.ID, err)
	}

	return nil, map[string]any{
		"status": "success", "kind": ops.Kind, "id": in.ID,
		"result": resp,
		"notice": "The plaintext key in this response is shown once and cannot be retrieved again. " +
			"Give it to the user now and do not repeat it.",
	}, nil
}

func (h *McpHandler) listAPIKeys(ctx context.Context, _ *mcp.CallToolRequest, in listKeysInput) (*mcp.CallToolResult, any, error) {
	ops, caller, log, correlationID, err := h.beginKeyOp(
		ctx, in.Kind, "wso2_apip_gw_list_api_keys", http.MethodGet, "")
	if err != nil {
		return nil, nil, err
	}

	resp, err := ops.Keys.List(in.ID, caller, correlationID, log)
	if err != nil {
		log.Error("MCP API key listing failed",
			slog.String("id", in.ID), slog.Any("error", err))
		return nil, nil, keyOpError("list API keys for", ops.Kind, in.ID, err)
	}

	return nil, map[string]any{
		"kind": ops.Kind, "id": in.ID, "result": resp,
	}, nil
}

// rotateIsInjection reports whether a rotate call installs a caller-supplied key
// value (PUT .../{apiKeyName}, the external-key-injection operation) rather than
// having the Gateway generate a new one (POST .../{apiKeyName}/regenerate).
//
// Called by BOTH the authorization gate and the tool, so the route key that was
// authorized is always the route key that executes. This is the same
// dispatch-on-argument shape write() uses to split create from update, which is
// why one tool covering two REST operations still maps to exactly one route key
// per call — no conjunctive evaluation is needed.
func rotateIsInjection(in rotateKeyInput) bool {
	return strings.TrimSpace(in.ApiKey) != ""
}

func (h *McpHandler) rotateAPIKey(ctx context.Context, _ *mcp.CallToolRequest, in rotateKeyInput) (*mcp.CallToolResult, any, error) {
	if err := h.immutableKeyWrite(); err != nil {
		return nil, nil, err
	}
	expiresAt, err := parseTimestamp("expiresAt", in.ExpiresAt)
	if err != nil {
		return nil, nil, err
	}

	// An absent apiKey IS the signal to regenerate, so — unlike the REST
	// handler, whose single route cannot tell the two intents apart — an empty
	// value routes to regeneration rather than being an error.
	injecting := rotateIsInjection(in)
	method, suffix := http.MethodPost, "/{apiKeyName}/regenerate"
	if injecting {
		method, suffix = http.MethodPut, "/{apiKeyName}"
	}

	ops, caller, log, correlationID, err := h.beginKeyOp(
		ctx, in.Kind, "wso2_apip_gw_rotate_api_key", method, suffix)
	if err != nil {
		return nil, nil, err
	}

	if injecting {
		req := api.APIKeyCreationRequest{ApiKey: &in.ApiKey, ExpiresAt: expiresAt}
		resp, err := ops.Keys.Update(in.ID, in.KeyName, req, caller, correlationID, log)
		if err != nil {
			log.Error("MCP API key update failed",
				slog.String("id", in.ID), slog.String("key_name", in.KeyName), slog.Any("error", err))
			return nil, nil, keyOpError("update the API key on", ops.Kind, in.ID, err)
		}
		// No plaintext notice here: UpdateAPIKey returns the masked value, and
		// the caller supplied the key in the first place.
		return nil, map[string]any{
			"status": "success", "operation": "update",
			"kind": ops.Kind, "id": in.ID, "keyName": in.KeyName,
			"result": resp,
		}, nil
	}

	req := api.APIKeyRegenerationRequest{ExpiresAt: expiresAt}
	resp, err := ops.Keys.Rotate(in.ID, in.KeyName, req, caller, correlationID, log)
	if err != nil {
		log.Error("MCP API key regeneration failed",
			slog.String("id", in.ID), slog.String("key_name", in.KeyName), slog.Any("error", err))
		return nil, nil, keyOpError("rotate the API key on", ops.Kind, in.ID, err)
	}

	return nil, map[string]any{
		"status": "success", "operation": "regenerate",
		"kind": ops.Kind, "id": in.ID, "keyName": in.KeyName,
		"result": resp,
		"notice": "The previous key value is now invalid. The plaintext key in this response is " +
			"shown once and cannot be retrieved again.",
	}, nil
}

func (h *McpHandler) revokeAPIKey(ctx context.Context, _ *mcp.CallToolRequest, in revokeKeyInput) (*mcp.CallToolResult, any, error) {
	if err := h.immutableKeyWrite(); err != nil {
		return nil, nil, err
	}
	if !in.Confirm {
		return nil, nil, fmt.Errorf(
			"refusing to revoke API key %q on %s %q: confirm the details with the user, then retry with confirm=true",
			in.KeyName, in.Kind, in.ID)
	}

	ops, caller, log, correlationID, err := h.beginKeyOp(
		ctx, in.Kind, "wso2_apip_gw_revoke_api_key", http.MethodDelete, "/{apiKeyName}")
	if err != nil {
		return nil, nil, err
	}

	resp, err := ops.Keys.Revoke(in.ID, in.KeyName, caller, correlationID, log)
	if err != nil {
		log.Error("MCP API key revocation failed",
			slog.String("id", in.ID), slog.String("key_name", in.KeyName), slog.Any("error", err))
		return nil, nil, keyOpError("revoke the API key on", ops.Kind, in.ID, err)
	}

	return nil, map[string]any{
		"status": "success", "kind": ops.Kind, "id": in.ID, "keyName": in.KeyName,
		"result": resp,
	}, nil
}

// keyOpError maps an APIKeyService failure to a message for the model.
//
// It reads the service's sentinels rather than copying any of the three
// different string-matching schemes the REST api-key handlers grew.
// A "not found" is echoed because the caller
// already proved they may read this collection, so confirming the parent exists
// leaks nothing they could not learn from list_resources. Anything else stays
// generic, with the real error in the log only.
func keyOpError(action, kind, id string, err error) error {
	switch {
	case storage.IsNotFoundError(err):
		return fmt.Errorf("cannot %s %s %q: no such resource, or no such key on it", action, kind, id)
	case storage.IsConflictError(err):
		return fmt.Errorf("cannot %s %s %q: a key with that name already exists", action, kind, id)
	case storage.IsOperationNotAllowedError(err):
		// In practice this is only reachable from the injection path, where the
		// target key was generated by this Gateway rather than supplied from
		// outside. Say what to do instead: without an explanation the model has
		// no way to recover, and the retry it needs is one argument away.
		return fmt.Errorf("cannot %s %s %q: a key value can only be installed over a key that was "+
			"originally issued elsewhere. This key was generated by the Gateway — call the same tool "+
			"again without \"apiKey\" to generate a replacement value instead", action, kind, id)
	default:
		// Covers the authorization refusal from canRegenerateAPIKey, which
		// wraps no sentinel.
		return fmt.Errorf("failed to %s %s %q: the Gateway refused the operation, "+
			"which usually means the key belongs to another user or the quota is exhausted", action, kind, id)
	}
}

// Certificate tool
//
// Calls CertificateService directly, exactly as the REST handlers in
// certificates.go do. The action is already resolved to a route key and
// authorized before any of this runs.
func (h *McpHandler) manageCertificates(ctx context.Context, _ *mcp.CallToolRequest, in manageCertificatesInput) (*mcp.CallToolResult, any, error) {
	op, err := resolveCertAction(in.Action)
	if err != nil {
		return nil, nil, err
	}
	// The immutable middleware rejects every POST/PUT/DELETE on the management
	// router, so REST already refuses all three of these — reload included, it
	// being a POST.
	if op.Mutating && h.immutable {
		return nil, nil, fmt.Errorf(
			"this Gateway runs in immutable mode; certificates are loaded at startup and cannot be changed at runtime")
	}
	if op.NeedsConfirm && !in.Confirm {
		return nil, nil, fmt.Errorf(
			"refusing to %s: confirm with the user, then retry with confirm=true", op.Action)
	}
	if err := h.authorize(ctx, op.RouteKey); err != nil {
		return nil, nil, err
	}

	correlationID := uuid.NewString()
	log := h.logger.With(
		slog.String("correlation_id", correlationID),
		slog.String("source", "mcp"),
		slog.String("tool", "wso2_apip_gw_manage_certificates"),
		slog.String("action", op.Action))

	switch op.Action {
	case certActionList:
		result, err := h.certificateService.List()
		if err != nil {
			log.Error("MCP certificate listing failed", slog.Any("error", err))
			return nil, nil, mcpCertError(certActionList, "", err)
		}
		// certificateToResponse, not the stored model: StoredCertificate marshals
		// its PEM bytes under "certificate", so returning rows directly would put
		// every stored chain into the transcript.
		items := make([]CertificateResponse, 0, len(result.Certificates))
		for _, cert := range result.Certificates {
			items = append(items, certificateToResponse(cert))
		}
		return nil, map[string]any{
			"count": len(items), "totalBytes": result.TotalBytes, "items": items,
		}, nil

	case certActionApply:
		// An id on apply means the caller expected an update. There is none, and
		// quietly creating a second certificate instead is the wrong recovery.
		if strings.TrimSpace(in.ID) != "" {
			return nil, nil, fmt.Errorf(
				"certificates cannot be updated, so apply does not take an id; " +
					"delete the existing certificate and apply the new one")
		}
		if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Certificate) == "" {
			return nil, nil, fmt.Errorf(`apply requires both "name" and "certificate"`)
		}
		result, err := h.certificateService.Upload(certificate.UploadParams{
			Name:           in.Name,
			CertificatePEM: []byte(in.Certificate),
			CorrelationID:  correlationID,
			Logger:         log,
		})
		if err != nil {
			log.Error("MCP certificate upload failed", slog.String("name", in.Name), slog.Any("error", err))
			return nil, nil, mcpCertError(certActionApply, in.Name, err)
		}
		return nil, map[string]any{
			"status": "success", "operation": "create",
			"certificate": certificateToResponse(result.Certificate),
			"message":     "Certificate stored and pushed to the router.",
		}, nil

	case certActionDelete:
		if strings.TrimSpace(in.ID) == "" {
			return nil, nil, fmt.Errorf(`delete requires "id"; call this tool with action=list to find it`)
		}
		if _, err := h.certificateService.Delete(certificate.DeleteParams{
			ID:            in.ID,
			CorrelationID: correlationID,
			Logger:        log,
		}); err != nil {
			log.Error("MCP certificate deletion failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpCertError(certActionDelete, in.ID, err)
		}
		return nil, map[string]any{
			"status": "success", "id": in.ID,
			"message": "Certificate deleted and the resulting trust store pushed to the router.",
		}, nil

	default:
		result, err := h.certificateService.Reload(certificate.ReloadParams{
			CorrelationID: correlationID,
			Logger:        log,
		})
		if err != nil {
			log.Error("MCP certificate reload failed", slog.Any("error", err))
			return nil, nil, mcpCertError(certActionReload, "", err)
		}
		return nil, map[string]any{
			"status": "success", "totalBytes": result.TotalBytes,
			"message": "Every stored certificate was re-read and pushed to the router.",
		}, nil
	}
}

// mcpCertError maps a CertificateService failure to a message for the model,
// reading the service's sentinels rather than its error strings.
func mcpCertError(action, subject string, err error) error {
	if errors.Is(err, certificate.ErrCertStoreNotConfigured) {
		return fmt.Errorf(
			"this Gateway was started without a custom certificate store " +
				"(router.upstream.tls.customCertsPath is unset), so certificate operations are unavailable")
	}
	if errors.Is(err, certificate.ErrMissingFields) {
		return fmt.Errorf(`apply requires both "name" and "certificate"`)
	}

	var invalidCert *certificate.InvalidCertificateError
	if errors.As(err, &invalidCert) {
		return fmt.Errorf("the supplied PEM data is not a usable certificate chain: %v. "+
			"Check that it holds at least one complete BEGIN/END CERTIFICATE block", invalidCert.Cause)
	}

	var metadataErr *certificate.MetadataError
	if errors.As(err, &metadataErr) {
		return fmt.Errorf("the certificate decoded but its metadata could not be read: %v", metadataErr.Cause)
	}

	// Checked before PersistError: SaveCertificate's duplicate-name conflict
	// arrives wrapped in one, and "that name is taken" is the actionable half.
	if storage.IsConflictError(err) {
		return fmt.Errorf(
			"a certificate named %q already exists. Names are unique and certificates cannot be updated: "+
				"delete that one first, or choose a different name", subject)
	}

	var syncErr *certificate.SyncError
	if errors.As(err, &syncErr) {
		if syncErr.Persisted {
			// The stored state is already correct and only the push failed.
			// Repeating the write would conflict or duplicate; say so.
			return fmt.Errorf(
				"the change was stored but could not be pushed to the router (it failed at the %s stage). "+
					"Do not repeat this call — retry only the push, with action=reload", syncErr.Stage)
		}
		return fmt.Errorf(
			"nothing changed: the trust store could not be pushed to the router (it failed at the %s stage)",
			syncErr.Stage)
	}

	var persistErr *certificate.PersistError
	if errors.As(err, &persistErr) && persistErr.Op == certificate.OpDelete {
		return fmt.Errorf(
			"cannot delete certificate %q: no certificate with that id, or it could not be removed. "+
				"Call this tool with action=list to see what is stored", subject)
	}

	return fmt.Errorf("the Gateway could not %s the certificate", action)
}

// Subscription tool
// Calls SubscriptionService directly, exactly as subscription_handler.go and
// subscription_plan_handler.go do, and renders results through the same
// response builders those handlers use.
func (h *McpHandler) manageSubscriptions(ctx context.Context, _ *mcp.CallToolRequest, in manageSubscriptionsInput) (*mcp.CallToolResult, any, error) {
	// resolve which REST operation is being requested
	op, err := resolveSubscriptionAction(in.Type, in.Action, in.ID)
	if err != nil {
		return nil, nil, err
	}
	// if in immutable mode, refuse any operation that would change the subscription state
	if op.Mutating && h.immutable {
		return nil, nil, fmt.Errorf(
			"this Gateway runs in immutable mode; subscriptions are loaded at startup and cannot be changed at runtime")
	}
	// refuse unconfirmed deletes
	if op.NeedsConfirm && !in.Confirm {
		return nil, nil, fmt.Errorf(
			"refusing to delete %s %q: confirm the id with the user, then retry with confirm=true",
			op.Type, in.ID)
	}
	// Checks if the caller has the proper scope
	if err := h.authorize(ctx, op.RouteKey); err != nil {
		return nil, nil, err
	}

	// if the operation requires an ID, check if it is provided
	if op.NeedsID && strings.TrimSpace(in.ID) == "" {
		return nil, nil, fmt.Errorf(
			`%s requires "id"; call this tool with action=list to find it`, op.Action)
	}

	correlationID := uuid.NewString()
	log := h.logger.With(
		slog.String("correlation_id", correlationID),
		slog.String("source", "mcp"),
		slog.String("tool", "wso2_apip_gw_manage_subscriptions"),
		slog.String("type", op.Type),
		slog.String("action", op.Action))

	// after the shared checks are done, dispatch to the appropriate handler based on the subscription type
	if op.Type == subTypePlan {
		return h.manageSubscriptionPlan(op, in, correlationID, log)
	}
	return h.manageSubscription(op, in, correlationID, log)
}

func (h *McpHandler) manageSubscription(op subAction, in manageSubscriptionsInput, correlationID string, log *slog.Logger) (*mcp.CallToolResult, any, error) {
	svc := h.subscriptionService

	switch op.Action {
	case subActionList:
		filter := subscription.ListFilter{}
		if spec := in.Spec; spec != nil {
			if spec.ApiId != nil && *spec.ApiId != "" {
				resolved, err := svc.ResolveAPIID(*spec.ApiId)
				if err != nil {
					log.Warn("MCP subscription list filter could not be resolved",
						slog.String("api_id", *spec.ApiId), slog.Any("error", err))
					return nil, nil, mcpSubscriptionError(op, *spec.ApiId, err)
				}
				filter.APIID = resolved
			}
			if spec.ApplicationId != nil && *spec.ApplicationId != "" {
				filter.ApplicationID = spec.ApplicationId
			}
			if status := normalizeSubscriptionStatus(spec.Status); status != nil {
				filter.Status = status
			}
		}

		result, err := svc.List(filter)
		if err != nil {
			log.Error("MCP subscription listing failed", slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, "", err)
		}
		items := make([]api.SubscriptionResponse, 0, len(result.Subscriptions))
		for _, sub := range result.Subscriptions {
			items = append(items, subscriptionToResponse(sub))
		}
		return nil, map[string]any{
			"type": op.Type, "count": len(items), "items": items,
		}, nil

	case subActionGet:
		result, err := svc.Get(in.ID)
		if err != nil {
			log.Warn("MCP subscription lookup failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, in.ID, err)
		}
		return nil, map[string]any{
			"type": op.Type, "id": in.ID, "resource": subscriptionToResponse(result.Subscription),
		}, nil

	case subActionDelete:
		if err := svc.Delete(in.ID, correlationID, log); err != nil {
			log.Error("MCP subscription deletion failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, in.ID, err)
		}
		return nil, map[string]any{
			"status": "success", "type": op.Type, "id": in.ID,
			"message": fmt.Sprintf("Subscription %q deleted successfully", in.ID),
		}, nil
	}

	// apply
	spec := in.Spec
	if spec == nil {
		return nil, nil, fmt.Errorf(`apply requires "spec"`)
	}

	if in.ID != "" {
		req, err := spec.toSubscriptionUpdate()
		if err != nil {
			return nil, nil, err
		}
		result, err := svc.Update(subscription.UpdateParams{
			ID: in.ID, Request: req, CorrelationID: correlationID, Logger: log,
		})
		if err != nil {
			log.Error("MCP subscription update failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, in.ID, err)
		}
		return nil, map[string]any{
			"status": "success", "operation": "update", "type": op.Type, "id": in.ID,
			"resource": subscriptionToResponse(result.Subscription),
			"notice": "Updating a subscription changes its status only. Every other field is unchanged, " +
				"whatever the spec contained.",
		}, nil
	}

	req, err := spec.toSubscriptionCreate()
	if err != nil {
		return nil, nil, err
	}
	result, err := svc.Create(subscription.CreateParams{
		Request: req, CorrelationID: correlationID, Logger: log,
	})
	if err != nil {
		log.Error("MCP subscription creation failed", slog.Any("error", err))
		return nil, nil, mcpSubscriptionError(op, req.ApiId, err)
	}
	return nil, map[string]any{
		"status": "success", "operation": "create", "type": op.Type,
		// Mirrors POST /subscriptions, which echoes the token the caller
		// supplied. Only its hash is stored, so this is the last time it
		// appears anywhere.
		"resource": subscriptionToResponseWithToken(result.Subscription),
		"notice": "The subscription token in this response is stored only as a hash and cannot be read " +
			"back from this Gateway. Treat it as credential material and do not repeat it.",
	}, nil
}

func (h *McpHandler) manageSubscriptionPlan(op subAction, in manageSubscriptionsInput, correlationID string, log *slog.Logger) (*mcp.CallToolResult, any, error) {
	svc := h.subscriptionService

	switch op.Action {
	case subActionList:
		result, err := svc.ListPlans()
		if err != nil {
			log.Error("MCP subscription plan listing failed", slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, "", err)
		}
		items := make([]api.SubscriptionPlanResponse, 0, len(result.Plans))
		for _, plan := range result.Plans {
			items = append(items, subscriptionPlanToResponse(plan))
		}
		return nil, map[string]any{
			"type": op.Type, "count": len(items), "items": items,
		}, nil

	case subActionGet:
		result, err := svc.GetPlan(in.ID)
		if err != nil {
			log.Warn("MCP subscription plan lookup failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, in.ID, err)
		}
		return nil, map[string]any{
			"type": op.Type, "id": in.ID, "resource": subscriptionPlanToResponse(result.Plan),
		}, nil

	case subActionDelete:
		if err := svc.DeletePlan(in.ID, correlationID, log); err != nil {
			log.Error("MCP subscription plan deletion failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, in.ID, err)
		}
		return nil, map[string]any{
			"status": "success", "type": op.Type, "id": in.ID,
			"message": fmt.Sprintf("Subscription plan %q deleted successfully", in.ID),
		}, nil
	}

	// apply
	spec := in.Spec
	if spec == nil {
		return nil, nil, fmt.Errorf(`apply requires "spec"`)
	}

	if in.ID != "" {
		req, err := spec.toPlanUpdate()
		if err != nil {
			return nil, nil, err
		}
		result, err := svc.UpdatePlan(subscription.UpdatePlanParams{
			ID: in.ID, Request: req, CorrelationID: correlationID, Logger: log,
		})
		if err != nil {
			log.Error("MCP subscription plan update failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, in.ID, err)
		}
		return nil, map[string]any{
			"status": "success", "operation": "update", "type": op.Type, "id": in.ID,
			"resource": subscriptionPlanToResponse(result.Plan),
		}, nil
	}

	req, err := spec.toPlanCreate()
	if err != nil {
		return nil, nil, err
	}
	result, err := svc.CreatePlan(subscription.CreatePlanParams{
		Request: req, CorrelationID: correlationID, Logger: log,
	})
	if err != nil {
		log.Error("MCP subscription plan creation failed", slog.Any("error", err))
		return nil, nil, mcpSubscriptionError(op, req.PlanName, err)
	}
	return nil, map[string]any{
		"status": "success", "operation": "create", "type": op.Type,
		"resource": subscriptionPlanToResponse(result.Plan),
	}, nil
}

// Spec mapping
//
// Each mapper builds the generated request type the service already takes, so
// no validation is duplicated here: enum values, throttle-limit pairing and
// plan eligibility all stay in SubscriptionService, which is the single place
// the REST API validates them too. The only work done here is the conversion
// the generated types cannot express — an RFC 3339 string into *time.Time.

func (s *subscriptionSpec) toSubscriptionCreate() (api.SubscriptionCreateRequest, error) {
	req := api.SubscriptionCreateRequest{
		ApplicationId:         s.ApplicationId,
		SubscriptionPlanId:    s.SubscriptionPlanId,
		BillingCustomerId:     s.BillingCustomerId,
		BillingSubscriptionId: s.BillingSubscriptionId,
	}
	if s.ApiId != nil {
		req.ApiId = *s.ApiId
	}
	if s.SubscriptionToken != nil {
		req.SubscriptionToken = *s.SubscriptionToken
	}
	if status := normalizeSubscriptionStatus(s.Status); status != nil {
		req.Status = ptr(api.SubscriptionCreateRequestStatus(*status))
	}
	return req, nil
}

func (s *subscriptionSpec) toSubscriptionUpdate() (api.SubscriptionUpdateRequest, error) {
	var req api.SubscriptionUpdateRequest
	if status := normalizeSubscriptionStatus(s.Status); status != nil {
		req.Status = ptr(api.SubscriptionUpdateRequestStatus(*status))
	}
	// SubscriptionUpdateRequest carries exactly one field, so a spec with no
	// status is a no-op write. Refusing is better than reporting success for a
	// call that changed nothing.
	if req.Status == nil {
		return req, fmt.Errorf(
			"updating a subscription changes its status only, and spec.status was not set; " +
				"set it to ACTIVE, INACTIVE or REVOKED, or delete and recreate the subscription " +
				"to change anything else")
	}
	return req, nil
}

func (s *subscriptionSpec) toPlanCreate() (api.SubscriptionPlanCreateRequest, error) {
	expiry, err := parseTimestamp("spec.expiryTime", derefString(s.ExpiryTime))
	if err != nil {
		return api.SubscriptionPlanCreateRequest{}, err
	}
	req := api.SubscriptionPlanCreateRequest{
		BillingPlan:        s.BillingPlan,
		StopOnQuotaReach:   s.StopOnQuotaReach,
		ThrottleLimitCount: s.ThrottleLimitCount,
		ExpiryTime:         expiry,
	}
	if s.PlanName != nil {
		req.PlanName = *s.PlanName
	}
	if unit := normalizeThrottleUnit(s.ThrottleLimitUnit); unit != nil {
		req.ThrottleLimitUnit = ptr(api.SubscriptionPlanCreateRequestThrottleLimitUnit(*unit))
	}
	if status := normalizeSubscriptionStatus(s.Status); status != nil {
		req.Status = ptr(api.SubscriptionPlanCreateRequestStatus(*status))
	}
	return req, nil
}

func (s *subscriptionSpec) toPlanUpdate() (api.SubscriptionPlanUpdateRequest, error) {
	expiry, err := parseTimestamp("spec.expiryTime", derefString(s.ExpiryTime))
	if err != nil {
		return api.SubscriptionPlanUpdateRequest{}, err
	}
	req := api.SubscriptionPlanUpdateRequest{
		PlanName:           s.PlanName,
		BillingPlan:        s.BillingPlan,
		StopOnQuotaReach:   s.StopOnQuotaReach,
		ThrottleLimitCount: s.ThrottleLimitCount,
		ExpiryTime:         expiry,
	}
	if unit := normalizeThrottleUnit(s.ThrottleLimitUnit); unit != nil {
		req.ThrottleLimitUnit = ptr(api.SubscriptionPlanUpdateRequestThrottleLimitUnit(*unit))
	}
	if status := normalizeSubscriptionStatus(s.Status); status != nil {
		req.Status = ptr(api.SubscriptionPlanUpdateRequestStatus(*status))
	}
	return req, nil
}

// normalizeSubscriptionStatus upper-cases a status so "active" reaches the
// service as ACTIVE. An unrecognised value is passed through untouched, so the
// service stays the one place that decides what is valid and produces the
// message naming the accepted set.
func normalizeSubscriptionStatus(raw *string) *string {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil
	}
	return ptr(strings.ToUpper(strings.TrimSpace(*raw)))
}

// normalizeThrottleUnit resolves a throttle unit to the exact spelling the
// service accepts. As above, an unrecognised value is passed through so the
// service produces the authoritative error.
func normalizeThrottleUnit(raw *string) *string {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	canonical := map[string]string{
		"min": "Min", "minute": "Min", "hour": "Hour",
		"day": "Day", "month": "Month",
	}
	if resolved, ok := canonical[strings.ToLower(trimmed)]; ok {
		return &resolved
	}
	return &trimmed
}

// mcpSubscriptionError maps a SubscriptionService failure to a message for the
// model, following keyOpError's policy: request-shaped validation detail is
// echoed so the model can correct its spec and retry, while anything else stays
// generic with the real error in the log only.
func mcpSubscriptionError(op subAction, subject string, err error) error {
	// ValidationError.Message is written to reach the caller verbatim, and is
	// the whole reason a model can fix a rejected plan or status by itself.
	var validationErr *subscription.ValidationError
	if errors.As(err, &validationErr) {
		return fmt.Errorf("%s", validationErr.Message)
	}

	var notRestAPI *subscription.NotRestAPIError
	if errors.As(err, &notRestAPI) {
		return fmt.Errorf(
			"%q is a %s, not a RestApi. Only REST APIs bear subscriptions",
			notRestAPI.Identifier, notRestAPI.Kind)
	}
	if errors.Is(err, subscription.ErrAPINotFound) {
		return fmt.Errorf(
			"no RestApi with identifier %q on this Gateway; "+
				"call wso2_apip_gw_list_resources with kind=RestApi to see what is deployed", subject)
	}
	if errors.Is(err, subscription.ErrSubscriptionNotFound) || errors.Is(err, subscription.ErrPlanNotFound) {
		return fmt.Errorf(
			"no %s with id %q; call this tool with action=list to see what exists", op.Type, subject)
	}

	if storage.IsConflictError(err) {
		if op.Type == subTypePlan {
			return fmt.Errorf("a subscription plan named %q already exists", subject)
		}
		return fmt.Errorf("that application is already subscribed to API %q", subject)
	}

	return fmt.Errorf("the Gateway could not %s the %s", op.Action, op.Type)
}
