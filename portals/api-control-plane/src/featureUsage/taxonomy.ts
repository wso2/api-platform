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
 * Dimensions are enums only, so a call site cannot forward user input (an API
 * name, a URL, an identifier) into analytics.
 */

type FeatureSpec = {
  /** The taxonomy entry's `name`, sent as `metadata.action`. */
  name: string;
  /** The taxonomy entry's `category`, sent as `metadata.category`. */
  category: string;
  tier: 'core' | 'premium' | 'enterprise';
  eventCategory: 'discovery' | 'engagement' | 'adoption' | 'abandonment' | 'lifecycle';
  dimensions: Readonly<Record<string, readonly string[]>>;
};

/** `product.id` of the API Platform taxonomy. */
export const API_PLATFORM_PRODUCT_ID = 'apip';

export const API_PLATFORM_FEATURES = {
  'apip.manage-apis.api-creation-wizard': {
    name: 'API Creation Wizard',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'discovery',
    dimensions: {
      entry_point: [
        'api-list',
        'api-list-empty-state',
        'org-home',
        'project-quick-actions',
        'direct',
      ],
    },
  },
  'apip.manage-apis.api-creation-wizard.select-api-type': {
    name: 'API Type Selection',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: { api_type: ['rest-api', 'graphql-api'] },
  },
  'apip.manage-apis.create-rest-api.select-creation-method': {
    name: 'REST API Creation Method Selection',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'engagement',
    dimensions: { creation_method: ['scratch', 'contract'] },
  },
  'apip.manage-apis.create-rest-api': {
    name: 'REST API Creation',
    category: 'manage-apis',
    tier: 'core',
    eventCategory: 'adoption',
    dimensions: { creation_method: ['scratch', 'contract'] },
  },
} as const satisfies Record<string, FeatureSpec>;

export type ApiPlatformFeatureCode = keyof typeof API_PLATFORM_FEATURES;

/** The dimensions a call passes for feature `C`: every declared key, as its enum's literal union. */
export type FeatureDimensions<C extends ApiPlatformFeatureCode> = {
  [
    K in keyof (typeof API_PLATFORM_FEATURES)[C]['dimensions']
  ]: (typeof API_PLATFORM_FEATURES)[C]['dimensions'][K] extends readonly (infer V)[] ? V : never;
};

/** Whether `code` is a feature code in the registry. */
export const isFeatureCode = (code: unknown): code is ApiPlatformFeatureCode =>
  typeof code === 'string' && Object.prototype.hasOwnProperty.call(API_PLATFORM_FEATURES, code);

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
  const spec = API_PLATFORM_FEATURES[code];
  const metadata: Record<string, string> = {};
  for (const [key, values] of Object.entries(spec.dimensions)) {
    const value =
      dimensions && Object.prototype.hasOwnProperty.call(dimensions, key)
        ? dimensions[key]
        : undefined;
    if (typeof value === 'string' && (values as readonly string[]).includes(value)) {
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
