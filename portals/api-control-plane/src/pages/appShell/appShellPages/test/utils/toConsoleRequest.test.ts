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

import type { ConsoleSendRequest } from '@/components/ApiTestConsole';
import { toCurl } from '../curl/utils/toCurl';
import { toConsoleRequest } from './toConsoleRequest';

const request = (overrides: Partial<ConsoleSendRequest> = {}): ConsoleSendRequest => ({
  baseUrl: 'https://gw.example.com/p',
  headers: [],
  method: 'POST',
  path: '/orders',
  query: [],
  ...overrides,
});

describe('toConsoleRequest', () => {
  it('leaves out headers and query parameters the console adds on its own', () => {
    const mirrored = toConsoleRequest(
      request({
        headers: [
          ['X-Api-Key', 'secret'],
          ['X-Other', '1'],
        ],
        query: [
          ['api_key', 'secret'],
          ['size', 'L'],
        ],
      }),
      [{ name: 'x-api-key', secret: true, value: 'secret' }],
      [{ name: 'api_key', secret: true, value: 'secret' }],
    );

    expect(mirrored.headers.map((h) => h.name)).toEqual(['X-Other']);
    expect(mirrored.queryParams.map((q) => q.name)).toEqual(['size']);
  });

  it('mirrors a text body as raw, and drops a Content-Type the cURL model would infer', () => {
    const mirrored = toConsoleRequest(
      request({
        body: { kind: 'text', mediaType: 'application/json', text: '{"a":1}' },
        headers: [['Content-Type', 'application/json; charset=utf-8']],
      }),
      [],
      [],
    );

    expect(mirrored).toMatchObject({ body: '{"a":1}', bodyMode: 'raw', rawFormat: 'json' });
    expect(mirrored.headers).toEqual([]);
  });

  it('mirrors a form as URL-encoded fields', () => {
    const mirrored = toConsoleRequest(
      request({ body: { fields: [['a', '1']], kind: 'form' } }),
      [],
      [],
    );
    expect(mirrored.bodyMode).toBe('url-encoded');
    expect(mirrored.formFields.map((f) => [f.name, f.value])).toEqual([['a', '1']]);
  });

  it('mirrors multipart parts, carrying a file as a reference to it', () => {
    const mirrored = toConsoleRequest(
      request({
        body: {
          kind: 'multipart',
          parts: [
            { name: 'note', value: 'hi' },
            { file: new File(['x'], 'a.csv'), name: 'upload' },
          ],
        },
      }),
      [],
      [],
    );

    expect(mirrored.bodyMode).toBe('form-data');
    expect(mirrored.formFields.map((f) => [f.name, f.value, f.file ?? false])).toEqual([
      ['note', 'hi', false],
      ['upload', 'a.csv', true],
    ]);
  });

  it('mirrors a binary upload in its own mode, so choosing None removes it', () => {
    const mirrored = toConsoleRequest(
      request({
        body: { file: new File(['x'], 'photo.png'), kind: 'binary', mediaType: 'image/png' },
      }),
      [],
      [],
    );

    expect(mirrored).toMatchObject({ binaryFile: 'photo.png', bodyMode: 'binary' });
    expect(toCurl(mirrored, { revealSecrets: true })).toContain("--data-binary '@photo.png'");
    expect(toCurl({ ...mirrored, bodyMode: 'none' }, { revealSecrets: true })).not.toContain(
      '--data-binary',
    );
  });

  it('mirrors a request with no body as none', () => {
    expect(toConsoleRequest(request({ method: 'GET' }), [], []).bodyMode).toBe('none');
  });
});
