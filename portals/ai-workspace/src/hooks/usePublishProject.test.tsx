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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { renderHook } from '@testing-library/react';
import type { ReactNode } from 'react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { useAppShell } from '../contexts/AppShellContext';
import { usePublishProject } from './usePublishProject';

vi.mock('../contexts/AppShellContext', () => ({ useAppShell: vi.fn() }));

const org = { uuid: 'org-uuid', handle: 'acme', name: 'Acme' };
const alpha = { id: 'alpha', displayName: 'Alpha' };
const beta = { id: 'beta', displayName: 'Beta' };

const shell = (overrides: Record<string, unknown> = {}) =>
  vi.mocked(useAppShell).mockReturnValue({
    currentOrganization: org,
    currentProject: alpha,
    projectsForCurrentOrganization: [alpha, beta],
    ...overrides,
  } as never);

/** Renders the hook at `route`, with `projectSlug` read from the URL when the route has one. */
const renderAt = (route: string, path: string) =>
  renderHook(() => usePublishProject(), {
    wrapper: ({ children }: { children: ReactNode }) => (
      <MemoryRouter initialEntries={[route]}>
        <Routes>
          <Route element={<>{children}</>} path={path} />
        </Routes>
      </MemoryRouter>
    ),
  });

beforeEach(() => {
  vi.mocked(useAppShell).mockReset();
});

describe('usePublishProject', () => {
  it('builds its links inside the project named in the URL', () => {
    shell();

    const { result } = renderAt('/organizations/acme/projects/beta/x', '/organizations/acme/projects/:projectSlug/x');

    expect(result.current.publishPath('/travel')).toBe('/organizations/acme/projects/beta/publish/travel');
    expect(result.current.overviewPath('travel')).toBe('/organizations/acme/projects/beta/mcp-proxy/travel');
  });

  it('uses the current project when the URL names none', () => {
    shell();

    const { result } = renderAt('/organizations/acme/x', '/organizations/acme/x');

    expect(result.current.publishPath()).toBe('/organizations/acme/projects/alpha/publish');
  });

  it('stays at the organization when there is no project at all', () => {
    shell({ currentProject: null, projectsForCurrentOrganization: [] });

    const { result } = renderAt('/organizations/acme/x', '/organizations/acme/x');

    expect(result.current.publishPath('/travel')).toBe('/organizations/acme/publish/travel');
  });

  it('encodes an MCP proxy id in the overview link', () => {
    shell();

    const { result } = renderAt('/organizations/acme/x', '/organizations/acme/x');

    expect(result.current.overviewPath('a/b c')).toBe('/organizations/acme/projects/alpha/mcp-proxy/a%2Fb%20c');
  });

  it('reports the organization it acts in', () => {
    shell();

    const { result } = renderAt('/organizations/acme/x', '/organizations/acme/x');

    expect(result.current.organizationId).toBe('org-uuid');
    expect(result.current.currentOrganization).toBe(org);
  });

  it('has an empty organization id before one is chosen', () => {
    shell({ currentOrganization: null });

    const { result } = renderAt('/x', '/x');

    expect(result.current.organizationId).toBe('');
  });
});
