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

import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  buildBasicIframeSrc,
  buildBasicProjectIframeSrc,
  resolveInsightsScopeLevel,
  resolveMoesifEmbeddingOrigin,
} from './moesifEmbed';

describe('moesifEmbed helpers', () => {
  it('resolves insights scope level', () => {
    expect(resolveInsightsScopeLevel({})).toBe('organization');
    expect(resolveInsightsScopeLevel({ projectHandle: 'p' })).toBe('project');
  });

  it('builds wrap/basic iframe src like choreo-console main', () => {
    expect(buildBasicIframeSrc('https://web-dev.moesif.com')).toBe(
      'https://web-dev.moesif.com/wrap/basic#auth=post'
    );
  });

  it('adds project_id for project-level wrap/basic', () => {
    expect(
      buildBasicProjectIframeSrc('https://web-dev.moesif.com', 'proj-1')
    ).toBe(
      'https://web-dev.moesif.com/wrap/basic?project_id=proj-1#auth=post'
    );
  });

  it('falls back to org iframe when project id is empty', () => {
    expect(buildBasicProjectIframeSrc('https://web-dev.moesif.com', '  ')).toBe(
      'https://web-dev.moesif.com/wrap/basic#auth=post'
    );
  });

  it('normalizes moesif app url to serialized origin for postMessage', () => {
    expect(
      resolveMoesifEmbeddingOrigin('https://www.moesif.com/')
    ).toBe('https://www.moesif.com');
    expect(
      resolveMoesifEmbeddingOrigin('https://web-dev.moesif.com')
    ).toBe('https://web-dev.moesif.com');
  });
});

describe('analyticsApi', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('fetchViewerToken reads token from cloud analytics endpoint', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({
        ok: true,
        json: async () => ({ token: 'viewer-token-123' }),
      }))
    );

    const { fetchViewerToken } = await import('../api/analyticsApi');
    await expect(fetchViewerToken()).resolves.toBe('viewer-token-123');
    expect(fetch).toHaveBeenCalledWith(
      '/proxy/cloud/analytics/id-token',
      expect.objectContaining({ credentials: 'include' })
    );
  });

  it('resolveProjectScope matches platform-api project handle in id field', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) => ({
        ok: true,
        json: async () => {
          if (url.includes('/projects/new-project')) {
            return {
              id: 'new-project',
              displayName: 'New Project',
              organizationId: 'default',
            };
          }
          return { list: [] };
        },
      }))
    );

    const { resolveProjectScope } = await import('../api/analyticsApi');
    await expect(resolveProjectScope('default', 'new-project')).resolves.toEqual({
      projectId: 'new-project',
      projectName: 'New Project',
    });
  });

  it('resolveProjectScope prefers uuid when present', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({
        ok: true,
        json: async () => ({
          id: 'new-project',
          uuid: '019feb1e-63d9-71f7-a693-1956ac197303',
          displayName: 'New Project',
        }),
      }))
    );

    const { resolveProjectScope } = await import('../api/analyticsApi');
    await expect(resolveProjectScope('default', 'new-project')).resolves.toEqual({
      projectId: '019feb1e-63d9-71f7-a693-1956ac197303',
      projectName: 'New Project',
    });
  });
});
