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

import { rawSpecUrl } from './rawSpecUrl';

describe('rawSpecUrl', () => {
  it('turns a GitHub page link into the raw file', () => {
    expect(
      rawSpecUrl('https://github.com/wso2/bijira-samples/blob/main/reading-list-api/openapi.yaml'),
    ).toEqual({
      host: 'GitHub',
      url: 'https://raw.githubusercontent.com/wso2/bijira-samples/main/reading-list-api/openapi.yaml',
    });
  });

  it('keeps a branch name that contains slashes intact', () => {
    expect(rawSpecUrl('https://github.com/acme/api/blob/release/2.x/spec/openapi.json')?.url).toBe(
      'https://raw.githubusercontent.com/acme/api/release/2.x/spec/openapi.json',
    );
  });

  it('turns GitLab and Bitbucket page links into raw files', () => {
    expect(rawSpecUrl('https://gitlab.com/acme/platform/api/-/blob/main/openapi.yaml')).toEqual({
      host: 'GitLab',
      url: 'https://gitlab.com/acme/platform/api/-/raw/main/openapi.yaml',
    });
    expect(rawSpecUrl('https://bitbucket.org/acme/api/src/main/openapi.yaml')).toEqual({
      host: 'Bitbucket',
      url: 'https://bitbucket.org/acme/api/raw/main/openapi.yaml',
    });
  });

  it('leaves raw links, other hosts and non-URLs alone', () => {
    for (const untouched of [
      'https://raw.githubusercontent.com/acme/api/main/openapi.yaml',
      'https://github.com/acme/api',
      'https://gitlab.example.com/acme/api/-/blob/main/openapi.yaml',
      'https://petstore3.swagger.io/api/v3/openapi.json',
      'not a url',
    ]) {
      expect(rawSpecUrl(untouched)).toBeUndefined();
    }
  });
});
