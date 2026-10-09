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

import { looksLikeSpecUrl } from './looksLikeSpecUrl';

describe('looksLikeSpecUrl', () => {
  it('recognises spec documents by file name, conventional path, or file host', () => {
    for (const url of [
      'https://petstore3.swagger.io/api/v3/openapi.json',
      'https://example.com/specs/orders.yaml',
      'https://example.com/specs/orders.YML?ref=main',
      'https://api.example.com/v3/api-docs',
      'https://api.example.com/swagger',
      'https://raw.githubusercontent.com/acme/apis/main/orders',
      'https://github.com/acme/apis/blob/main/orders.yaml',
    ]) {
      expect(looksLikeSpecUrl(url), url).toBe(true);
    }
  });

  it('treats a running API’s address as not a spec', () => {
    for (const url of [
      'https://apis.bijira.dev/samples/reading-list-api-service/v1.0/books',
      'https://api.example.com/v1',
      'http://billing',
      'https://api.example.com/orders/42',
    ]) {
      expect(looksLikeSpecUrl(url), url).toBe(false);
    }
  });

  it('answers no for anything that is not an http(s) URL', () => {
    for (const value of ['', 'orders', 'ftp://example.com/openapi.json', 'not a url.json']) {
      expect(looksLikeSpecUrl(value), value).toBe(false);
    }
  });
});
