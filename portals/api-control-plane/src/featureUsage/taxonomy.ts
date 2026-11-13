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
 * it. The taxonomy itself is `api-platform-taxonomy.yaml`, next to this file;
 * `taxonomy.generated.ts` is generated from it (`npm run taxonomy:codegen`)
 * and holds both the whole taxonomy and the console's part of it.
 *
 * A feature is the console's when its `portal` dimension names
 * "api-control-plane". Each key is that feature's `code`, sent verbatim as the
 * Moesif action name; the other fields come from the same taxonomy entry and
 * ride along as metadata. Only codes listed here can be tracked.
 *
 * `portal` is not among a feature's dimensions here: the console adds it to
 * every event itself (`featureMetadata`), so no call site can leave it out.
 *
 * Dimensions are enums only, so a call site cannot forward user input (an API
 * name, a URL, an identifier) into analytics.
 */

import { CONSOLE_FEATURES, CONSOLE_PORTAL } from './taxonomy.generated';

export { API_PLATFORM_TAXONOMY, CONSOLE_PORTAL } from './taxonomy.generated';

type DimensionSpec = { readonly values: readonly string[]; readonly required: boolean };

type FeatureSpec = {
  /** The taxonomy entry's `name`, sent as `metadata.action`. */
  readonly name: string;
  /** The taxonomy entry's `category`, sent as `metadata.category`. */
  readonly category: string;
  readonly tier: 'core' | 'premium' | 'enterprise';
  readonly eventCategory: 'discovery' | 'engagement' | 'adoption' | 'abandonment' | 'lifecycle';
  readonly dimensions: Readonly<Record<string, DimensionSpec>>;
};

/** `product.id` of the API Platform taxonomy. */
export const API_PLATFORM_PRODUCT_ID = 'apip';

/** The API types the console manages, spelled as the taxonomy's `api_type`. */
const API_TYPES = ['rest-api', 'graphql-api'] as const;

/** The features the console emits, from the taxonomy. */
export const API_PLATFORM_FEATURES = CONSOLE_FEATURES satisfies Readonly<
  Record<string, FeatureSpec>
>;

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

/**
 * A policy-capability feature: an `apip.policies.<capability>-attach` code a
 * policy maps to. Derived from the taxonomy, so a capability added there needs
 * no change here.
 */
export type PolicyFeatureCode = Extract<ApiPlatformFeatureCode, `apip.policies.${string}-attach`>;

const isPolicyFeatureCode = (code: string): code is PolicyFeatureCode =>
  /^apip\.policies\.[a-z0-9-]+-attach$/.test(code);

/**
 * Policies whose capability has a single policy, so the feature carries no
 * `policy_name` to read the policy from. The one mapping the taxonomy can't
 * supply: add a capability here only if it has exactly one policy.
 */
const SINGLE_POLICY_FEATURES: Readonly<Record<string, PolicyFeatureCode>> = {
  'analytics-header-filter': 'apip.policies.analytics-filtering-attach',
  cors: 'apip.policies.cors-attach',
  'subscription-validation': 'apip.policies.access-control-attach',
};

/**
 * Policy name → the capability feature it belongs to. Every capability feature
 * whose taxonomy entry declares `policy_name` maps each of its values, so a
 * policy or capability added to the taxonomy is classified without a change
 * here.
 */
const POLICY_FEATURES: ReadonlyMap<string, PolicyFeatureCode> = new Map([
  ...Object.entries(SINGLE_POLICY_FEATURES),
  ...Object.entries(API_PLATFORM_FEATURES as Readonly<Record<string, FeatureSpec>>).flatMap(
    ([code, spec]) =>
      isPolicyFeatureCode(code) && spec.dimensions.policy_name
        ? spec.dimensions.policy_name.values.map((name) => [name, code] as const)
        : [],
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
    portal: CONSOLE_PORTAL,
  };
}
