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

/**
 * The API Platform feature taxonomy (schema
 * https://wso2.com/schemas/feature-taxonomy/v1), as far as the console emits
 * it. Each key is a feature or sub-feature `code`, sent verbatim as the Moesif
 * action name; the other fields are copied from the same taxonomy entry and
 * ride along as metadata. Only codes listed here can be tracked.
 *
 * Every entry is one UI action the console actually offers (checked against
 * the pages, not only the taxonomy): a code is
 * `<product>.<category>.<feature-action>`, and a dimension only describes a
 * variant of that same action (REST or GraphQL, the gateway type). A dimension lists only the values the
 * console can produce, which may be fewer than the taxonomy allows for other
 * surfaces such as the CLI.
 *
 * Dimensions are enums only, so a call site cannot forward user input (an API
 * name, a URL, an identifier) into analytics.
 */

type DimensionSpec = { readonly values: readonly string[]; readonly required: boolean };

type FeatureSpec = {
  /** The taxonomy entry's `name`, sent as `metadata.action`. */
  name: string;
  /** The taxonomy entry's `category`, sent as `metadata.category`. */
  category: string;
  tier: 'core' | 'premium' | 'enterprise';
  eventCategory: 'discovery' | 'engagement' | 'adoption' | 'abandonment' | 'lifecycle';
  dimensions: Readonly<Record<string, DimensionSpec>>;
};

/** A dimension every emission of the feature carries. */
const required = <const V extends readonly string[]>(values: V) =>
  ({ required: true, values }) as const;

/** A dimension sent only when it applies (e.g. the contract source of a contract import). */
const optional = <const V extends readonly string[]>(values: V) =>
  ({ required: false, values }) as const;

/** `product.id` of the API Platform taxonomy. */
export const API_PLATFORM_PRODUCT_ID = 'apip';

const API_TYPES = ['rest-api', 'graphql-api'] as const;
const GATEWAY_TYPES = ['regular', 'ai', 'event'] as const;
const GATEWAY_HOSTING = ['managed', 'self-hosted'] as const;
const CREATION_METHODS = ['scratch', 'contract'] as const;
const CONTRACT_SOURCES = ['url', 'file'] as const;
/** Where an attached policy applies: the whole API, or one operation (resource). */
const POLICY_SCOPES = ['api', 'operation'] as const;

const DOCUMENT_TYPES = ['how-to', 'samples', 'public-forum', 'support-forum', 'other'] as const;
/** A flag dimension; an enum rather than a boolean, so every value is a string. */
const YES_NO = ['yes', 'no'] as const;

export const API_PLATFORM_FEATURES = {
  // ── Manage APIs ─────────────────────────────────────────────────────────────
  'apip.manage-apis.api-creation-wizard-open': {
    name: 'API Creation Wizard Open',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'discovery',
    dimensions: {
      entry_point: required([
        'api-list',
        'api-list-empty-state',
        'org-home',
        'project-quick-actions',
        'direct',
      ]),
    },
  },
  'apip.manage-apis.api-creation-wizard-api-type-select': {
    name: 'API Type Selection',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      api_type: required(API_TYPES),
    },
  },
  'apip.manage-apis.api-creation-wizard-creation-method-select': {
    name: 'API Creation Method Selection',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      api_type: required(API_TYPES),
      creation_method: required(CREATION_METHODS),
    },
  },
  'apip.manage-apis.rest-api-create-click': {
    name: 'REST API Creation Click',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      contract_source: optional(CONTRACT_SOURCES),
      creation_method: required(CREATION_METHODS),
    },
  },
  'apip.manage-apis.graphql-api-create-click': {
    name: 'GraphQL API Creation Click',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      contract_source: optional(CONTRACT_SOURCES),
      creation_method: required(CREATION_METHODS),
    },
  },
  'apip.manage-apis.api-update-click': {
    name: 'API Details Update Click',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      api_type: required(API_TYPES),
    },
  },
  'apip.manage-apis.api-endpoint-update-click': {
    name: 'API Endpoint Update Click',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      api_type: required(API_TYPES),
    },
  },
  'apip.manage-apis.api-definition-update-click': {
    name: 'OpenAPI Definition Update Click',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {},
  },
  'apip.manage-apis.api-delete-click': {
    name: 'API Deletion Click',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      api_type: required(API_TYPES),
    },
  },
  'apip.manage-apis.api-thumbnail-upload-click': {
    name: 'API Thumbnail Upload Click',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      api_type: optional(API_TYPES),
    },
  },
  'apip.manage-apis.api-thumbnail-remove-click': {
    name: 'API Thumbnail Removal Click',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      api_type: optional(API_TYPES),
    },
  },
  'apip.manage-apis.api-document-create-click': {
    name: 'API Document Creation Click',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      api_type: optional(API_TYPES),
      document_type: optional(DOCUMENT_TYPES),
    },
  },
  'apip.manage-apis.api-document-update-click': {
    name: 'API Document Update Click',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      api_type: optional(API_TYPES),
      document_type: optional(DOCUMENT_TYPES),
    },
  },
  'apip.manage-apis.api-document-delete-click': {
    name: 'API Document Deletion Click',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      api_type: optional(API_TYPES),
      document_type: optional(DOCUMENT_TYPES),
    },
  },
  'apip.manage-apis.test-console-execute': {
    name: 'Test Console Execution',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      api_type: required(API_TYPES),
      call_mode: optional(['proxy', 'direct']),
      outcome: required(['success', 'failure']),
    },
  },
  'apip.manage-apis.test-console-curl-copy': {
    name: 'cURL Command Copy',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      api_type: required(['rest-api']),
    },
  },
  // ── Gateway & Runtime ───────────────────────────────────────────────────────
  'apip.gateway.gateway-register-click': {
    name: 'Gateway Registration Click',
    category: 'gateway',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      gateway_hosting: optional(GATEWAY_HOSTING),
      gateway_type: required(GATEWAY_TYPES),
    },
  },
  'apip.gateway.gateway-update-click': {
    name: 'Gateway Update Click',
    category: 'gateway',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      gateway_type: optional(GATEWAY_TYPES),
    },
  },
  'apip.gateway.gateway-token-regenerate-click': {
    name: 'Gateway Token Regeneration Click',
    category: 'gateway',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {},
  },
  'apip.gateway.gateway-setup-command-copy': {
    name: 'Gateway Setup Command Copy',
    category: 'gateway',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      install_method: required(['quick-start', 'virtual-machine', 'docker', 'kubernetes']),
      setup_step: required(['check-runtime', 'download', 'setup', 'configure', 'start', 'install']),
    },
  },
  // ── API Policies & Traffic Management ───────────────────────────────────────
  'apip.policies.client-authentication-attach': {
    name: 'Client Authentication',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      artifact_kind: optional(API_TYPES),
      policy_name: required([
        'api-key-auth',
        'basic-auth',
        'jwt-auth',
        'mtls-auth',
        'opaque-token-auth',
      ]),
      policy_scope: optional(POLICY_SCOPES),
    },
  },
  'apip.policies.backend-authentication-attach': {
    name: 'Backend Authentication',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      artifact_kind: optional(API_TYPES),
      policy_name: required([
        'backend-jwt',
        'aws-authentication',
        'oauth2-generator',
        'oauth2-token-exchange',
      ]),
      policy_scope: optional(POLICY_SCOPES),
    },
  },
  'apip.policies.access-control-attach': {
    name: 'Subscription Validation',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      artifact_kind: optional(API_TYPES),
      policy_scope: optional(POLICY_SCOPES),
    },
  },
  'apip.policies.rate-limiting-attach': {
    name: 'Rate Limiting',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      artifact_kind: optional(API_TYPES),
      policy_name: required(['basic-ratelimit', 'advanced-ratelimit']),
      policy_scope: optional(POLICY_SCOPES),
    },
  },
  'apip.policies.traffic-routing-attach': {
    name: 'Traffic Routing',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      artifact_kind: optional(API_TYPES),
      policy_name: required(['dynamic-endpoint', 'redirect']),
      policy_scope: optional(POLICY_SCOPES),
    },
  },
  'apip.policies.transformation-attach': {
    name: 'Transformation',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      artifact_kind: optional(API_TYPES),
      policy_name: required([
        'set-headers',
        'remove-headers',
        'request-rewrite',
        'host-rewrite',
        'json-xml-mediator',
        'respond',
      ]),
      policy_scope: optional(POLICY_SCOPES),
    },
  },
  'apip.policies.content-validation-attach': {
    name: 'Content Validation',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      artifact_kind: optional(API_TYPES),
      policy_name: required([
        'json-schema-guardrail',
        'regex-guardrail',
        'content-length-guardrail',
        'url-guardrail',
        'word-count-guardrail',
        'sentence-count-guardrail',
      ]),
      policy_scope: optional(POLICY_SCOPES),
    },
  },
  'apip.policies.cors-attach': {
    name: 'CORS',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      artifact_kind: optional(API_TYPES),
      policy_scope: optional(POLICY_SCOPES),
    },
  },
  'apip.policies.interception-attach': {
    name: 'Interception & Logging',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      artifact_kind: optional(API_TYPES),
      policy_name: required(['interceptor-service', 'log-message']),
      policy_scope: optional(POLICY_SCOPES),
    },
  },
  'apip.policies.analytics-filtering-attach': {
    name: 'Analytics Filtering',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      artifact_kind: optional(API_TYPES),
      policy_scope: optional(POLICY_SCOPES),
    },
  },
  'apip.policies.policy-hub-browse': {
    name: 'Policy Hub Browsing',
    category: 'policies',
    tier: 'core',
    eventCategory: 'discovery',
    dimensions: {
      artifact_kind: optional(API_TYPES),
    },
  },
  'apip.policies.gateway-policies-view': {
    name: 'Gateway Policies View',
    category: 'policies',
    tier: 'core',
    eventCategory: 'discovery',
    dimensions: {},
  },
  // ── API Security ────────────────────────────────────────────────────────────
  'apip.security.api-key-create-click': {
    name: 'API Key Creation Click',
    category: 'security',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      artifact_kind: required(API_TYPES),
    },
  },
  'apip.security.test-api-key-generate-click': {
    name: 'Test API Key Generation Click',
    category: 'security',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      artifact_kind: required(API_TYPES),
    },
  },
  'apip.security.api-key-revoke-click': {
    name: 'API Key Revocation Click',
    category: 'security',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      artifact_kind: required(API_TYPES),
    },
  },
  'apip.security.service-account-create-click': {
    name: 'Service Account Creation Click',
    category: 'security',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {},
  },
  'apip.security.service-account-update-click': {
    name: 'Service Account Update Click',
    category: 'security',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {},
  },
  'apip.security.service-account-enable-click': {
    name: 'Service Account Enabling Click',
    category: 'security',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {},
  },
  'apip.security.service-account-disable-click': {
    name: 'Service Account Disabling Click',
    category: 'security',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {},
  },
  'apip.security.service-account-delete-click': {
    name: 'Service Account Deletion Click',
    category: 'security',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {},
  },
  'apip.security.service-account-secret-regenerate-click': {
    name: 'Service Account Secret Regeneration Click',
    category: 'security',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {},
  },
  'apip.security.service-account-test-token-get-click': {
    name: 'Service Account Test Token Click',
    category: 'security',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {},
  },
  // ── API Lifecycle Management ────────────────────────────────────────────────
  'apip.lifecycle.api-deploy-click': {
    name: 'API Deployment Click',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      artifact_kind: required(API_TYPES),
      gateway_hosting: optional(GATEWAY_HOSTING),
      gateway_type: required(GATEWAY_TYPES),
    },
  },
  'apip.lifecycle.deployment-undeploy-click': {
    name: 'Deployment Stop Click',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      artifact_kind: required(API_TYPES),
    },
  },
  'apip.lifecycle.deployment-redeploy-click': {
    name: 'Deployment Redeploy Click',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      artifact_kind: required(API_TYPES),
    },
  },
  'apip.lifecycle.deployment-restore-click': {
    name: 'Deployment Restore Click',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      artifact_kind: required(API_TYPES),
    },
  },
  'apip.lifecycle.deployment-delete-click': {
    name: 'Deployment Deletion Click',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      artifact_kind: required(API_TYPES),
    },
  },
  'apip.lifecycle.deployment-history-view': {
    name: 'Deployment History View',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'discovery',
    dimensions: {
      artifact_kind: optional(API_TYPES),
    },
  },
  'apip.lifecycle.api-publish-click': {
    name: 'API Publication Click',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      api_type: required(API_TYPES),
    },
  },
  'apip.lifecycle.api-unpublish-click': {
    name: 'API Unpublication Click',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      api_type: required(API_TYPES),
    },
  },
  'apip.lifecycle.api-deprecate-click': {
    name: 'API Deprecation Click',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      api_type: required(API_TYPES),
    },
  },
  'apip.lifecycle.publication-draft-save-click': {
    name: 'Publication Draft Save Click',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      api_type: optional(API_TYPES),
      definition_saved: required(YES_NO),
      landing_page_changed: required(YES_NO),
      thumbnail_changed: required(YES_NO),
    },
  },
  'apip.lifecycle.api-portal-open': {
    name: 'API Portal Open',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      api_type: optional(API_TYPES),
    },
  },
  // ── API Monetization ────────────────────────────────────────────────────────
  'apip.monetization.subscription-plan-create-click': {
    name: 'Subscription Plan Creation Click',
    category: 'monetization',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {},
  },
  'apip.monetization.subscription-plan-update-click': {
    name: 'Subscription Plan Update Click',
    category: 'monetization',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {},
  },
  'apip.monetization.subscription-plan-status-change-click': {
    name: 'Subscription Plan Status Change Click',
    category: 'monetization',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      status: required(['active', 'inactive']),
    },
  },
  'apip.monetization.subscription-plan-delete-click': {
    name: 'Subscription Plan Deletion Click',
    category: 'monetization',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {},
  },
  'apip.monetization.meter-create-click': {
    name: 'Meter Creation Click',
    category: 'monetization',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {},
  },
  'apip.monetization.meter-update-click': {
    name: 'Meter Update Click',
    category: 'monetization',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {},
  },
  'apip.monetization.meter-delete-click': {
    name: 'Meter Deletion Click',
    category: 'monetization',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {},
  },
  // ── Administration & Configuration ──────────────────────────────────────────
  'apip.admin.project-create-click': {
    name: 'Project Creation Click',
    category: 'admin',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {},
  },
  'apip.admin.project-delete-click': {
    name: 'Project Deletion Click',
    category: 'admin',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {},
  },
} as const satisfies Record<string, FeatureSpec>;

export type ApiPlatformFeatureCode = keyof typeof API_PLATFORM_FEATURES;

type DimensionsOf<C extends ApiPlatformFeatureCode> =
  (typeof API_PLATFORM_FEATURES)[C]['dimensions'];
type ValueOf<D> = D extends { readonly values: readonly (infer V)[] } ? V : never;
type RequiredKeys<C extends ApiPlatformFeatureCode> = {
  [K in keyof DimensionsOf<C>]: DimensionsOf<C>[K] extends { readonly required: true } ? K : never;
}[keyof DimensionsOf<C>];

/**
 * The dimensions a call passes for feature `C`: every required key and any of
 * the optional ones, each as its enum's literal union.
 */
export type FeatureDimensions<C extends ApiPlatformFeatureCode> = {
  [K in RequiredKeys<C>]: ValueOf<DimensionsOf<C>[K]>;
} & {
  [K in Exclude<keyof DimensionsOf<C>, RequiredKeys<C>>]?: ValueOf<DimensionsOf<C>[K]>;
};

/**
 * The rest arguments after the code: the dimensions, which may be left out
 * when the feature has no required one.
 */
export type FeatureDimensionArgs<C extends ApiPlatformFeatureCode> = [RequiredKeys<C>] extends [
  never,
]
  ? [dimensions?: FeatureDimensions<C>]
  : [dimensions: FeatureDimensions<C>];

/** The values the console may send for dimension `key` of feature `code`. */
export const dimensionValues = <C extends ApiPlatformFeatureCode>(
  code: C,
  key: keyof DimensionsOf<C> & string,
): readonly string[] =>
  (API_PLATFORM_FEATURES[code].dimensions as Readonly<Record<string, DimensionSpec>>)[key].values;

/**
 * The taxonomy's `api_type` for a value already spelled the same way, such as
 * the `{apiType}` path segment of the API Portal and thumbnail endpoints; any
 * other value gives undefined, so it is left out.
 */
export const asApiType = (value: unknown): (typeof API_TYPES)[number] | undefined =>
  typeof value === 'string' && (API_TYPES as readonly string[]).includes(value)
    ? (value as (typeof API_TYPES)[number])
    : undefined;

/** Whether `code` is a feature code in the registry. */
export const isFeatureCode = (code: unknown): code is ApiPlatformFeatureCode =>
  typeof code === 'string' && Object.prototype.hasOwnProperty.call(API_PLATFORM_FEATURES, code);

/** A policy-capability feature: one of the `apip.policies.*` codes a policy maps to. */
export type PolicyFeatureCode = Extract<
  ApiPlatformFeatureCode,
  | 'apip.policies.client-authentication-attach'
  | 'apip.policies.backend-authentication-attach'
  | 'apip.policies.access-control-attach'
  | 'apip.policies.rate-limiting-attach'
  | 'apip.policies.traffic-routing-attach'
  | 'apip.policies.transformation-attach'
  | 'apip.policies.content-validation-attach'
  | 'apip.policies.cors-attach'
  | 'apip.policies.interception-attach'
  | 'apip.policies.analytics-filtering-attach'
>;

/** Policies whose capability has a single policy, so the feature carries no `policy_name`. */
const SINGLE_POLICY_FEATURES: Readonly<Record<string, PolicyFeatureCode>> = {
  'analytics-header-filter': 'apip.policies.analytics-filtering-attach',
  cors: 'apip.policies.cors-attach',
  'subscription-validation': 'apip.policies.access-control-attach',
};

const NAMED_POLICY_FEATURES: readonly PolicyFeatureCode[] = [
  'apip.policies.client-authentication-attach',
  'apip.policies.backend-authentication-attach',
  'apip.policies.rate-limiting-attach',
  'apip.policies.traffic-routing-attach',
  'apip.policies.transformation-attach',
  'apip.policies.content-validation-attach',
  'apip.policies.interception-attach',
];

/** Policy name → the capability feature it belongs to, built from the registry. */
const POLICY_FEATURES: ReadonlyMap<string, PolicyFeatureCode> = new Map([
  ...Object.entries(SINGLE_POLICY_FEATURES),
  ...NAMED_POLICY_FEATURES.flatMap((code) =>
    (
      API_PLATFORM_FEATURES[code].dimensions as Readonly<Record<string, DimensionSpec>>
    ).policy_name.values.map((name) => [name, code] as const),
  ),
]);

/**
 * The capability feature an attached policy reports, or undefined for a policy
 * the taxonomy doesn't classify (a custom policy, or an AI or MCP policy).
 * Matches the policy's `name` handle as stored on the API (e.g. `api-key-auth`).
 */
export const policyFeatureOf = (policyName: string): PolicyFeatureCode | undefined =>
  POLICY_FEATURES.get(policyName);

/**
 * The metadata an action carries: the feature's declared dimensions whose
 * values are in their enum, plus the taxonomy fields from the registry. Used
 * both to build an action and, by the PII filter, to rebuild one, so nothing a
 * caller (or the SDK) adds can survive.
 */
export function featureMetadata(
  code: ApiPlatformFeatureCode,
  dimensions: Readonly<Record<string, unknown>> | undefined,
): Record<string, string> {
  const spec: FeatureSpec = API_PLATFORM_FEATURES[code];
  const metadata: Record<string, string> = {};
  for (const [key, { values }] of Object.entries(spec.dimensions)) {
    const value =
      dimensions && Object.prototype.hasOwnProperty.call(dimensions, key)
        ? dimensions[key]
        : undefined;
    if (typeof value === 'string' && values.includes(value)) {
      metadata[key] = value;
    }
  }
  return {
    ...metadata,
    action: spec.name,
    category: spec.category,
    tier: spec.tier,
    event_category: spec.eventCategory,
    initiating_product: API_PLATFORM_PRODUCT_ID,
    platform: 'web',
  };
}
