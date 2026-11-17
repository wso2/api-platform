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

import { describe, expect, it, vi } from 'vitest';

import { trackPolicyAttachment } from './policies';
import {
  API_PLATFORM_FEATURES,
  API_PLATFORM_PRODUCT_ID,
  dimensionValues,
  type ApiPlatformFeatureCode,
  featureMetadata,
  policyFeatureOf,
} from './taxonomy';

describe('the feature registry', () => {
  it.each(Object.entries(API_PLATFORM_FEATURES))(
    '%s follows the taxonomy code convention',
    (code, spec) => {
      // <product>.<category>.<action>[.<sub-action>], the category being the second segment.
      expect(code).toMatch(/^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*){2,3}$/);
      const [product, category] = code.split('.');
      expect(product).toBe(API_PLATFORM_PRODUCT_ID);
      expect(category).toBe(spec.category);
    },
  );

  it('prefixes every sub-feature with a registered parent code', () => {
    const codes = Object.keys(API_PLATFORM_FEATURES);
    for (const code of codes.filter((c) => c.split('.').length > 3)) {
      expect(codes).toContain(code.split('.').slice(0, 3).join('.'));
    }
  });

  it('reads the values of a dimension', () => {
    expect(dimensionValues('apip.manage-apis.api-delete-click', 'api_type')).toEqual([
      'rest-api',
      'graphql-api',
    ]);
  });
});

describe('featureMetadata', () => {
  it('sends an optional dimension only when it is given', () => {
    expect(
      featureMetadata('apip.manage-apis.rest-api-create-click', { creation_method: 'scratch' }),
    ).not.toHaveProperty('contract_source');
    expect(
      featureMetadata('apip.manage-apis.rest-api-create-click', {
        contract_source: 'file',
        creation_method: 'contract',
      }),
    ).toMatchObject({ contract_source: 'file', creation_method: 'contract' });
  });

  it('carries only the taxonomy fields for a feature without dimensions', () => {
    expect(featureMetadata('apip.policies.gateway-policies-view', undefined)).toEqual({
      action: 'Gateway Policies View',
      category: 'policies',
      event_category: 'discovery',
      growth_action: 'apip.policies.gateway-policies-view',
      platform: 'API Platform',
      portal: 'api-control-plane',
      source: 'console',
      tier: 'core',
    });
  });

  it('names the product and source as Platform API and Agent Manager do', () => {
    const metadata = featureMetadata('apip.manage-apis.rest-api-create-click', {
      creation_method: 'scratch',
    });

    expect(metadata).toMatchObject({
      growth_action: 'apip.manage-apis.rest-api-create-click',
      platform: 'API Platform',
      source: 'console',
    });
    expect(metadata).not.toHaveProperty('initiating_product');
  });

  it('never lets a caller replace a field every action carries', () => {
    const metadata = featureMetadata('apip.policies.gateway-policies-view', {
      growth_action: 'apip.other',
      platform: 'web',
      source: 'api',
    });

    expect(metadata).toMatchObject({
      growth_action: 'apip.policies.gateway-policies-view',
      platform: 'API Platform',
      source: 'console',
    });
  });

  it.each(Object.keys(API_PLATFORM_FEATURES))('marks %s as coming from the console', (code) => {
    expect(featureMetadata(code as ApiPlatformFeatureCode, undefined).portal).toBe(
      'api-control-plane',
    );
  });

  it('never lets a caller choose the portal', () => {
    const metadata = featureMetadata('apip.policies.gateway-policies-view', {
      portal: 'api-portal',
    });

    expect(metadata.portal).toBe('api-control-plane');
  });
});

describe('policy capabilities', () => {
  it.each([
    ['api-key-auth', 'apip.policies.client-authentication-attach'],
    ['backend-jwt', 'apip.policies.backend-authentication-attach'],
    ['subscription-validation', 'apip.policies.access-control-attach'],
    ['advanced-ratelimit', 'apip.policies.rate-limiting-attach'],
    ['redirect', 'apip.policies.traffic-routing-attach'],
    ['json-xml-mediator', 'apip.policies.transformation-attach'],
    ['regex-guardrail', 'apip.policies.content-validation-attach'],
    ['cors', 'apip.policies.cors-attach'],
    ['log-message', 'apip.policies.interception-attach'],
    ['analytics-header-filter', 'apip.policies.analytics-filtering-attach'],
  ])('maps %s to %s', (policy, code) => {
    expect(policyFeatureOf(policy)).toBe(code);
  });

  it('classifies no custom, AI or MCP policy', () => {
    expect(policyFeatureOf('acme-fraud-check')).toBeUndefined();
    expect(policyFeatureOf('semantic-cache')).toBeUndefined();
    expect(policyFeatureOf('mcp-auth')).toBeUndefined();
  });

  it('reports an attachment under its capability, with the policy name where declared', () => {
    const trackFeature = vi.fn();

    trackPolicyAttachment(trackFeature, 'basic-ratelimit', {
      artifact_kind: 'rest-api',
      policy_scope: 'operation',
    });
    trackPolicyAttachment(trackFeature, 'cors', {
      artifact_kind: 'graphql-api',
      policy_scope: 'api',
    });
    trackPolicyAttachment(trackFeature, 'acme-fraud-check', { artifact_kind: 'rest-api' });

    expect(trackFeature.mock.calls).toEqual([
      [
        'apip.policies.rate-limiting-attach',
        { artifact_kind: 'rest-api', policy_name: 'basic-ratelimit', policy_scope: 'operation' },
      ],
      [
        'apip.policies.cors-attach',
        { artifact_kind: 'graphql-api', policy_name: 'cors', policy_scope: 'api' },
      ],
    ]);
    // The single-policy capabilities carry no policy_name to Moesif.
    expect(
      featureMetadata('apip.policies.cors-attach', trackFeature.mock.calls[1][1]),
    ).not.toHaveProperty('policy_name');
  });
});
