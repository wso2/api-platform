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

import { parseInsightsRouteParams } from './routeParams';

describe('parseInsightsRouteParams', () => {
  it('reads org and project handles at the origin root', () => {
    expect(parseInsightsRouteParams('/organizations/acme/projects/p1/insights')).toEqual({
      orgHandle: 'acme',
      projectHandler: 'p1',
    });
  });

  // The console can be served under a base path on a shared origin; the
  // handles must still resolve from the full browser pathname.
  it('reads org and project handles under a base path', () => {
    expect(parseInsightsRouteParams('/api-platform/organizations/acme/projects/p1/insights')).toEqual({
      orgHandle: 'acme',
      projectHandler: 'p1',
    });
  });

  it('returns undefined handles when the path carries none', () => {
    expect(parseInsightsRouteParams('/api-platform/')).toEqual({
      orgHandle: undefined,
      projectHandler: undefined,
    });
  });
});
