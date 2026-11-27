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
import { matchPath } from 'react-router-dom';
import { NOT_FOUND_PATTERN, outsideReferrerOrigin, pageViewEvent, routePattern, sortPatterns } from './pageView';

const matches = (pattern: string, path: string) => matchPath({ path: pattern, end: true }, path) !== null;
const patterns = sortPatterns([
  '/organizations/:orgHandle/gateways/:gatewayId',
  '/organizations/:orgHandle/gateways/new',
  '/organizations/:orgHandle/projects/:projectHandler/apis/:apiHandler/deploy',
  '/organizations/:orgHandle/home',
]);
const of = (path: string) => routePattern(path, patterns, matches);

describe('routePattern', () => {
  it('replaces handles with the route parameters', () => {
    expect(of('/organizations/acme/projects/payroll/apis/orders/deploy')).toBe(
      '/organizations/:orgHandle/projects/:projectHandler/apis/:apiHandler/deploy'
    );
    expect(of('/organizations/acme/home')).toBe('/organizations/:orgHandle/home');
  });
  it('prefers the literal route over a parameterised one', () => {
    expect(of('/organizations/acme/gateways/new')).toBe('/organizations/:orgHandle/gateways/new');
    expect(of('/organizations/acme/gateways/g-1')).toBe('/organizations/:orgHandle/gateways/:gatewayId');
  });
  it('reports unmatched paths as not-found without echoing them', () => {
    expect(of('/organizations/acme/secret-customer-thing')).toBe(NOT_FOUND_PATTERN);
  });
});

describe('pageViewEvent', () => {
  it('carries no path, query, hash or customer title', () => {
    const e = pageViewEvent('/organizations/:orgHandle/home', 'https://www.google.com/search?q=x', 'https://console.example');
    expect(e).toEqual({
      event: 'virtual_page_view',
      page_location: 'https://console.example/organizations/:orgHandle/home',
      page_path: '/organizations/:orgHandle/home',
      page_referrer: 'https://www.google.com',
      page_title: 'API Platform Console',
      product: 'api-platform',
    });
  });
  it('drops a same-site or unparsable referrer', () => {
    expect(outsideReferrerOrigin('https://console.example/a?b', 'https://console.example')).toBe('');
    expect(outsideReferrerOrigin('not a url', 'https://console.example')).toBe('');
    expect(outsideReferrerOrigin('', 'https://console.example')).toBe('');
  });
});
