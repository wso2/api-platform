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

import { render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { InsightsEmbedScope } from './types';

const { mockResolveProjectScope, embedScopes } = vi.hoisted(() => ({
  mockResolveProjectScope: vi.fn(),
  embedScopes: [] as InsightsEmbedScope[],
}));

vi.mock('./api/analyticsApi', () => ({
  resolveProjectScope: (...args: unknown[]) => mockResolveProjectScope(...args),
}));

vi.mock('./components/StateViews', () => ({
  LoadingState: ({ label }: { label?: string }) => (
    <div data-testid="loading-state">{label}</div>
  ),
  ErrorState: ({ title, message }: { title: string; message: string }) => (
    <div data-testid="error-state">
      {title}: {message}
    </div>
  ),
}));

vi.mock('./InsightsEmbed', () => ({
  default: ({ scope }: { scope: InsightsEmbedScope }) => {
    embedScopes.push(scope);
    return (
      <div data-testid="insights-embed">{scope.projectId ?? 'organization'}</div>
    );
  },
}));

import InsightsFeature from './InsightsFeature';
import type { InsightsHostPort } from './hostPort';

const basePort: InsightsHostPort = {
  orgHandle: 'acme',
  navigate: vi.fn(),
  notify: vi.fn(),
};

describe('InsightsFeature', () => {
  beforeEach(() => {
    embedScopes.length = 0;
    mockResolveProjectScope.mockReset();
  });

  it('shows loading instead of stale project metadata when switching projects', async () => {
    let resolveProjectB: (value: {
      projectId: string;
      projectName: string;
    }) => void = () => {};
    const projectBPromise = new Promise<{
      projectId: string;
      projectName: string;
    }>((resolve) => {
      resolveProjectB = resolve;
    });

    mockResolveProjectScope.mockImplementation((_org, handle) => {
      if (handle === 'project-a') {
        return Promise.resolve({
          projectId: 'id-a',
          projectName: 'Project A',
        });
      }
      if (handle === 'project-b') {
        return projectBPromise;
      }
      return Promise.reject(new Error(`unexpected handle: ${handle}`));
    });

    const { rerender } = render(
      <InsightsFeature
        port={{ ...basePort, projectHandle: 'project-a' }}
        forcedScopeLevel="project"
      />
    );

    await waitFor(() => {
      expect(screen.getByTestId('insights-embed')).toHaveTextContent('id-a');
    });
    const embedRenderCountAfterA = embedScopes.length;

    rerender(
      <InsightsFeature
        port={{ ...basePort, projectHandle: 'project-b' }}
        forcedScopeLevel="project"
      />
    );

    expect(screen.queryByTestId('insights-embed')).not.toBeInTheDocument();
    expect(screen.getByTestId('loading-state')).toHaveTextContent(
      'Preparing Insights'
    );
    expect(embedScopes.length).toBe(embedRenderCountAfterA);

    resolveProjectB({ projectId: 'id-b', projectName: 'Project B' });

    await waitFor(() => {
      expect(screen.getByTestId('insights-embed')).toHaveTextContent('id-b');
    });
    expect(embedScopes.at(-1)).toEqual({
      level: 'project',
      projectId: 'id-b',
      projectName: 'Project B',
    });
  });
});
