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

import { describe, expect, it } from 'vitest';

import { aGateway } from '@/test/msw';
import { buildInvokeUrl, gatewayEndpoint } from './gatewayDisplay';

describe('gatewayEndpoint', () => {
  it("is the gateway's first address, or empty without one", () => {
    expect(gatewayEndpoint(aGateway({ endpoints: ['https://one.test', 'https://two.test'] }))).toBe('https://one.test');
    expect(gatewayEndpoint(aGateway({ endpoints: [] }))).toBe('');
  });
});

describe('buildInvokeUrl', () => {
  it('joins the endpoint and the context', () => {
    expect(buildInvokeUrl('https://gw.test', '/loans')).toBe('https://gw.test/loans');
  });

  it('drops trailing slashes from the endpoint', () => {
    expect(buildInvokeUrl('https://gw.test//', '/loans')).toBe('https://gw.test/loans');
  });

  it('adds https when the endpoint has no scheme, and keeps http as given', () => {
    expect(buildInvokeUrl('gw.test', '/loans')).toBe('https://gw.test/loans');
    expect(buildInvokeUrl('http://gw.test', '/loans')).toBe('http://gw.test/loans');
  });

  it('adds the leading slash a context lacks, and uses the root without a context', () => {
    expect(buildInvokeUrl('https://gw.test', 'loans')).toBe('https://gw.test/loans');
    expect(buildInvokeUrl('https://gw.test')).toBe('https://gw.test/');
    expect(buildInvokeUrl('https://gw.test', '')).toBe('https://gw.test/');
  });

  it('is empty without an endpoint', () => {
    expect(buildInvokeUrl('  ', '/loans')).toBe('');
  });

  it('resolves a $version placeholder in context against the given version', () => {
    // The exact bug this guards: gateway-controller's ConstructFullPath
    // resolves $version server-side, but the console built the invoke URL
    // straight from the raw context — a real deployed API with context
    // "/countries/$version/graphql" produced a URL with a literal,
    // unresolved "$version" segment that 404s at the gateway.
    expect(buildInvokeUrl('https://gw.test', '/countries/$version/graphql', 'v1.0')).toBe(
      'https://gw.test/countries/v1.0/graphql',
    );
  });

  it('leaves a context with no $version placeholder unchanged', () => {
    expect(buildInvokeUrl('https://gw.test', '/graphqlsampleapi/v1.0.0', '1.0.0')).toBe(
      'https://gw.test/graphqlsampleapi/v1.0.0',
    );
  });

  it('substitutes $version as an empty string when no version is given', () => {
    expect(buildInvokeUrl('https://gw.test', '/countries/$version/graphql')).toBe('https://gw.test/countries//graphql');
  });
});
