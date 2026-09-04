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
  buildAiWorkspaceIframeSrc,
  buildBasicIframeSrc,
  buildBasicProjectIframeSrc,
  resolveInsightsScopeLevel,
  resolveMoesifEmbeddingOrigin,
  resolveTrustedMoesifAppUrl,
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

  it('builds the AI Workspace ai-overview iframe src', () => {
    expect(buildAiWorkspaceIframeSrc('https://web-dev.moesif.com')).toBe(
      'https://web-dev.moesif.com/wrap/basic/ai-overview?embedded_ui=true&isolated_section=true#auth=post'
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

  it('rejects untrusted moesif hosts and uses allowlisted fallback', () => {
    expect(
      resolveTrustedMoesifAppUrl(
        'https://evil.example.com',
        'https://web-dev.moesif.com'
      )
    ).toBe('https://web-dev.moesif.com');
  });

  it('returns undefined when no allowlisted host is configured', () => {
    expect(
      resolveTrustedMoesifAppUrl(
        'https://evil.example.com',
        'https://also-evil.example.com'
      )
    ).toBeUndefined();
  });

  it('rejects non-https moesif hosts', () => {
    expect(
      resolveTrustedMoesifAppUrl(
        'http://www.moesif.com',
        'https://www.moesif.com'
      )
    ).toBe('https://www.moesif.com');
  });

  it('accepts allowlisted https moesif hosts', () => {
    expect(
      resolveTrustedMoesifAppUrl(
        'https://www.moesif.com/wrap',
        'https://web-dev.moesif.com'
      )
    ).toBe('https://www.moesif.com');
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

  it('fetchViewerToken maps 404 to a user-facing org message', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({
        ok: false,
        status: 404,
        json: async () => ({ error: 'not found' }),
      }))
    );

    const { fetchViewerToken } = await import('../api/analyticsApi');
    await expect(fetchViewerToken()).rejects.toThrow(
      /not available for this organization/i
    );
  });

  it('fetchViewerToken maps 502 without exposing the status code', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({
        ok: false,
        status: 502,
        json: async () => ({ error: 'bad gateway' }),
      }))
    );

    const { fetchViewerToken } = await import('../api/analyticsApi');
    await expect(fetchViewerToken()).rejects.toThrow(
      /temporarily unavailable/i
    );
    await expect(fetchViewerToken()).rejects.not.toThrow(/502/);
  });

  it('resolveProjectScope does not reuse the org-mapping 404 copy', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({
        ok: false,
        status: 404,
        json: async () => ({ error: 'missing' }),
      }))
    );

    const { resolveProjectScope } = await import('../api/analyticsApi');
    await expect(resolveProjectScope('default', 'missing-proj')).rejects.toThrow(
      /Project "missing-proj" was not found/
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

  it('resolveProjectScope matches project id when handler differs', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({
        ok: true,
        json: async () => ({
          id: 'proj-uuid',
          handler: 'orders',
          uuid: 'proj-uuid',
          displayName: 'Orders',
        }),
      }))
    );

    const { resolveProjectScope } = await import('../api/analyticsApi');
    await expect(resolveProjectScope('default', 'proj-uuid')).resolves.toEqual({
      projectId: 'proj-uuid',
      projectName: 'Orders',
    });
  });

  it('resolveProjectScope sends Accept and X-Org-Id headers', async () => {
    const fetchMock = vi.fn(async () => ({
      ok: true,
      json: async () => ({
        id: 'proj-uuid',
        displayName: 'Orders',
      }),
    }));
    vi.stubGlobal('fetch', fetchMock);

    const { resolveProjectScope } = await import('../api/analyticsApi');
    await resolveProjectScope('default', 'proj-uuid');

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/projects/proj-uuid'),
      expect.objectContaining({
        headers: expect.objectContaining({
          accept: 'application/json',
          'X-Org-Id': 'default',
        }),
      })
    );
  });
});
