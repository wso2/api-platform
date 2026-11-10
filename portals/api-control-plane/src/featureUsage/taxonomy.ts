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
 * Every entry is a feature the console's UI actually offers (checked against
 * the pages, not only the taxonomy). A dimension lists only the values the
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

/**
 * The policies dimensions shared by the policy-capability features below.
 * `policy_name` lists the WSO2 policies of that capability that ship with the
 * gateway (gateway/build-manifest.yaml); a custom policy's name is the
 * customer's own text, so it is never sent.
 */
const policyDimensions = <const N extends readonly string[]>(names: N) => ({
  artifact_kind: optional(API_TYPES),
  policy_name: required(names),
  policy_scope: optional(POLICY_SCOPES),
});
const unnamedPolicyDimensions = {
  artifact_kind: optional(API_TYPES),
  policy_scope: optional(POLICY_SCOPES),
};

export const API_PLATFORM_FEATURES = {
  // ── Manage APIs ───────────────────────────────────────────────────────────
  'apip.manage-apis.api-creation-wizard': {
    name: 'API Creation Wizard',
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
  'apip.manage-apis.api-creation-wizard.select-api-type': {
    name: 'API Type Selection',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: { api_type: required(API_TYPES) },
  },
  'apip.manage-apis.api-creation-wizard.select-creation-method': {
    name: 'API Creation Method Selection',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: { api_type: required(API_TYPES), creation_method: required(CREATION_METHODS) },
  },
  'apip.manage-apis.create-rest-api': {
    name: 'REST API Creation',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'adoption',
    dimensions: {
      contract_source: optional(CONTRACT_SOURCES),
      creation_method: required(CREATION_METHODS),
    },
  },
  'apip.manage-apis.create-graphql-api': {
    name: 'GraphQL API Creation',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'adoption',
    dimensions: {
      contract_source: optional(CONTRACT_SOURCES),
      creation_method: required(CREATION_METHODS),
    },
  },
  'apip.manage-apis.update-api': {
    name: 'API Update',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      api_type: required(API_TYPES),
      part: required(['metadata', 'endpoint', 'definition']),
    },
  },
  'apip.manage-apis.delete-api': {
    name: 'API Deletion',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: { api_type: required(API_TYPES) },
  },
  'apip.manage-apis.api-documentation': {
    name: 'API Documentation',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      api_type: optional(API_TYPES),
      document_type: optional(['how-to', 'samples', 'public-forum', 'support-forum', 'other']),
      operation: required(['create', 'update', 'delete']),
    },
  },
  'apip.manage-apis.api-thumbnail': {
    name: 'API Thumbnail',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: { api_type: optional(API_TYPES), operation: required(['upsert', 'delete']) },
  },
  'apip.manage-apis.test-console': {
    name: 'Test Console',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      api_type: required(API_TYPES),
      call_mode: optional(['proxy', 'direct']),
      outcome: required(['success', 'failure']),
    },
  },
  'apip.manage-apis.test-console.copy-curl': {
    name: 'cURL Command Copy',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: { api_type: required(['rest-api']) },
  },

  // ── Gateway & Runtime ─────────────────────────────────────────────────────
  'apip.gateway.register-gateway': {
    name: 'Gateway Registration',
    category: 'gateway',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      gateway_hosting: optional(GATEWAY_HOSTING),
      gateway_type: required(GATEWAY_TYPES),
    },
  },
  'apip.gateway.manage-gateway': {
    name: 'Gateway Update & Deletion',
    category: 'gateway',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      gateway_type: optional(GATEWAY_TYPES),
      operation: required(['update', 'delete']),
    },
  },
  'apip.gateway.gateway-token': {
    name: 'Gateway Registration Token',
    category: 'gateway',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: { operation: required(['rotate', 'revoke']) },
  },
  'apip.gateway.copy-setup-command': {
    name: 'Gateway Setup Command Copy',
    category: 'gateway',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: {
      install_method: required(['quick-start', 'virtual-machine', 'docker', 'kubernetes']),
      setup_step: required(['check-runtime', 'download', 'setup', 'configure', 'start', 'install']),
    },
  },

  // ── API Policies & Traffic Management ─────────────────────────────────────
  'apip.policies.client-authentication': {
    name: 'Client Authentication',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: policyDimensions([
      'api-key-auth',
      'basic-auth',
      'jwt-auth',
      'mtls-auth',
      'opaque-token-auth',
    ]),
  },
  'apip.policies.backend-authentication': {
    name: 'Backend Authentication',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: policyDimensions([
      'backend-jwt',
      'aws-authentication',
      'oauth2-generator',
      'oauth2-token-exchange',
    ]),
  },
  'apip.policies.access-control': {
    name: 'Subscription Validation',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: unnamedPolicyDimensions,
  },
  'apip.policies.rate-limiting': {
    name: 'Rate Limiting',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: policyDimensions(['basic-ratelimit', 'advanced-ratelimit']),
  },
  'apip.policies.traffic-routing': {
    name: 'Traffic Routing',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: policyDimensions(['dynamic-endpoint', 'redirect']),
  },
  'apip.policies.transformation': {
    name: 'Request & Response Transformation',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: policyDimensions([
      'set-headers',
      'remove-headers',
      'request-rewrite',
      'host-rewrite',
      'json-xml-mediator',
      'respond',
    ]),
  },
  'apip.policies.content-validation': {
    name: 'Content Validation',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: policyDimensions([
      'json-schema-guardrail',
      'regex-guardrail',
      'content-length-guardrail',
      'url-guardrail',
      'word-count-guardrail',
      'sentence-count-guardrail',
    ]),
  },
  'apip.policies.cors': {
    name: 'CORS',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: unnamedPolicyDimensions,
  },
  'apip.policies.interception': {
    name: 'Interception & Logging',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: policyDimensions(['interceptor-service', 'log-message']),
  },
  'apip.policies.analytics-filtering': {
    name: 'Analytics Header Filtering',
    category: 'policies',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: unnamedPolicyDimensions,
  },
  'apip.policies.browse-policies': {
    name: 'Policy Hub Browsing',
    category: 'policies',
    tier: 'core',
    eventCategory: 'discovery',
    dimensions: { artifact_kind: optional(API_TYPES) },
  },
  'apip.policies.view-gateway-policies': {
    name: 'Gateway Policies View',
    category: 'policies',
    tier: 'core',
    eventCategory: 'discovery',
    dimensions: {},
  },

  // ── API Security ──────────────────────────────────────────────────────────
  'apip.security.api-key-issuance': {
    name: 'API Key Issuance',
    category: 'security',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      artifact_kind: required(API_TYPES),
      key_purpose: required(['production', 'test']),
    },
  },
  'apip.security.api-key-management': {
    name: 'API Key Update & Revocation',
    category: 'security',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: { artifact_kind: required(API_TYPES), operation: required(['update', 'revoke']) },
  },
  'apip.security.service-accounts': {
    name: 'Service Accounts',
    category: 'security',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      operation: required([
        'create',
        'update',
        'enable',
        'disable',
        'delete',
        'regenerate-secret',
        'issue-test-token',
      ]),
    },
  },

  // ── API Lifecycle Management ──────────────────────────────────────────────
  'apip.lifecycle.deploy-api': {
    name: 'Artifact Deployment',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'adoption',
    dimensions: {
      artifact_kind: required(API_TYPES),
      gateway_hosting: optional(GATEWAY_HOSTING),
      gateway_type: required(GATEWAY_TYPES),
    },
  },
  'apip.lifecycle.manage-deployment': {
    name: 'Deployment Management',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      artifact_kind: required(API_TYPES),
      operation: required(['undeploy', 'restore', 'delete']),
    },
  },
  'apip.lifecycle.view-deployment-history': {
    name: 'Deployment History View',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'discovery',
    dimensions: { artifact_kind: optional(API_TYPES) },
  },
  'apip.lifecycle.publish-api': {
    name: 'API Publication',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'adoption',
    dimensions: { api_type: required(API_TYPES) },
  },
  'apip.lifecycle.manage-publication': {
    name: 'Publication Management',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: { api_type: required(API_TYPES), operation: required(['unpublish', 'deprecate']) },
  },
  'apip.lifecycle.publication-draft': {
    name: 'Publication Draft',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      api_type: optional(API_TYPES),
      draft_part: required(['metadata', 'definition', 'landing-page', 'thumbnail']),
    },
  },
  'apip.lifecycle.open-api-portal': {
    name: 'API Portal Opened',
    category: 'lifecycle',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: { api_type: optional(API_TYPES) },
  },

  // ── API Monetization ──────────────────────────────────────────────────────
  'apip.monetization.subscription-plan-management': {
    name: 'Subscription Plan Management',
    category: 'monetization',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: {
      operation: required(['create', 'update', 'activate', 'deactivate', 'delete']),
    },
  },
  'apip.monetization.meter-management': {
    name: 'Meter Management',
    category: 'monetization',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: { operation: required(['create', 'update', 'delete']) },
  },

  // ── Administration & Configuration ────────────────────────────────────────
  'apip.admin.manage-projects': {
    name: 'Project Management',
    category: 'admin',
    tier: 'core',
    eventCategory: 'lifecycle',
    dimensions: { operation: required(['create', 'update', 'delete']) },
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
  | 'apip.policies.client-authentication'
  | 'apip.policies.backend-authentication'
  | 'apip.policies.access-control'
  | 'apip.policies.rate-limiting'
  | 'apip.policies.traffic-routing'
  | 'apip.policies.transformation'
  | 'apip.policies.content-validation'
  | 'apip.policies.cors'
  | 'apip.policies.interception'
  | 'apip.policies.analytics-filtering'
>;

/** Policies whose capability has a single policy, so the feature carries no `policy_name`. */
const SINGLE_POLICY_FEATURES: Readonly<Record<string, PolicyFeatureCode>> = {
  'analytics-header-filter': 'apip.policies.analytics-filtering',
  cors: 'apip.policies.cors',
  'subscription-validation': 'apip.policies.access-control',
};

const NAMED_POLICY_FEATURES: readonly PolicyFeatureCode[] = [
  'apip.policies.client-authentication',
  'apip.policies.backend-authentication',
  'apip.policies.rate-limiting',
  'apip.policies.traffic-routing',
  'apip.policies.transformation',
  'apip.policies.content-validation',
  'apip.policies.interception',
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
