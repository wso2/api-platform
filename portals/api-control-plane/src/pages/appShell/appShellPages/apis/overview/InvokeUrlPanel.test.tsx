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
import { buildInvokeUrl } from './InvokeUrlPanel';

describe('buildInvokeUrl', () => {
  it('joins the endpoint and context with a scheme ensured', () => {
    expect(buildInvokeUrl('localhost:8443', '/countries/v1.0')).toBe(
      'https://localhost:8443/countries/v1.0',
    );
  });

  it('resolves a $version placeholder in context against the given version', () => {
    // The exact bug this guards: gateway-controller's ConstructFullPath
    // resolves $version server-side, but the console built the invoke URL
    // straight from the raw context — a real deployed API with context
    // "/countries/$version/graphql" produced a URL with a literal,
    // unresolved "$version" segment that 404s at the gateway.
    expect(buildInvokeUrl('https://localhost:8443', '/countries/$version/graphql', 'v1.0')).toBe(
      'https://localhost:8443/countries/v1.0/graphql',
    );
  });

  it('leaves a context with no $version placeholder unchanged', () => {
    expect(buildInvokeUrl('https://localhost:8443', '/graphqlsampleapi/v1.0.0', '1.0.0')).toBe(
      'https://localhost:8443/graphqlsampleapi/v1.0.0',
    );
  });

  it('substitutes $version as an empty string when no version is given', () => {
    expect(buildInvokeUrl('https://localhost:8443', '/countries/$version/graphql')).toBe(
      'https://localhost:8443/countries//graphql',
    );
  });

  it('returns an empty string for an empty endpoint', () => {
    expect(buildInvokeUrl('', '/countries/$version', 'v1.0')).toBe('');
  });
});
