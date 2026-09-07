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

import { parseGitHubRepoUrl, resolveGitHubRef } from './github';

describe('parseGitHubRepoUrl', () => {
  it('reads owner and repo from the plain and .git forms', () => {
    expect(parseGitHubRepoUrl('https://github.com/wso2/api-platform')).toEqual({
      owner: 'wso2',
      repo: 'api-platform',
    });
    expect(parseGitHubRepoUrl('https://github.com/wso2/api-platform.git')).toEqual({
      owner: 'wso2',
      repo: 'api-platform',
    });
  });

  it('keeps a tree deep link whole rather than guessing where the branch ends', () => {
    // The URL format gives no delimiter between the ref and the path, so the
    // split is deferred to resolveGitHubRef, which has the branch list.
    expect(parseGitHubRepoUrl('https://github.com/wso2/api-platform/tree/feature/x/apis')).toEqual({
      owner: 'wso2',
      refSegments: ['feature', 'x', 'apis'],
      repo: 'api-platform',
    });
  });

  it('ignores a link that names something other than a tree', () => {
    expect(
      parseGitHubRepoUrl('https://github.com/wso2/api-platform/blob/main/openapi.yaml'),
    ).toEqual({ owner: 'wso2', repo: 'api-platform' });
  });

  it('rejects anything that is not github.com', () => {
    expect(parseGitHubRepoUrl('https://github.com.evil.example/wso2/api-platform')).toBeNull();
    expect(parseGitHubRepoUrl('not a url')).toBeNull();
  });
});

describe('resolveGitHubRef', () => {
  it('prefers the longest branch name that actually exists', () => {
    // `feature/x` is a real branch, so `apis` is the directory — not branch
    // `feature` with directory `x/apis`, which is what the string alone says.
    expect(resolveGitHubRef(['feature', 'x', 'apis'], ['main', 'feature/x'])).toEqual({
      branch: 'feature/x',
      path: 'apis',
    });
  });

  it('takes the single-segment branch when that is the one that exists', () => {
    expect(resolveGitHubRef(['feature', 'x', 'apis'], ['main', 'feature'])).toEqual({
      branch: 'feature',
      path: 'x/apis',
    });
  });

  it('reads a branch with no directory after it', () => {
    expect(resolveGitHubRef(['release', '1.0'], ['main', 'release/1.0'])).toEqual({
      branch: 'release/1.0',
      path: '',
    });
  });

  it('returns null when no prefix names a branch, so the caller keeps the default', () => {
    expect(resolveGitHubRef(['gone', 'apis'], ['main'])).toBeNull();
    expect(resolveGitHubRef(undefined, ['main'])).toBeNull();
    expect(resolveGitHubRef([], ['main'])).toBeNull();
  });
});
