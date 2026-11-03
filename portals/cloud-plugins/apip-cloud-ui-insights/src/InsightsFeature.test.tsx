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
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { InsightsEmbedScope } from './types';

const { mockResolveProjectScope, embedScopes, projectFilter, comingSoon } = vi.hoisted(
  () => ({
    mockResolveProjectScope: vi.fn(),
    embedScopes: [] as InsightsEmbedScope[],
    projectFilter: { enabled: false },
    comingSoon: { enabled: false },
  })
);

vi.mock('./api/analyticsApi', () => ({
  resolveProjectScope: (...args: unknown[]) => mockResolveProjectScope(...args),
}));

vi.mock('./config/acpProjectFilter', () => ({
  get ENABLE_ACP_PROJECT_FILTER() {
    return projectFilter.enabled;
  },
}));

vi.mock('./config/comingSoon', () => ({
  get SHOW_INSIGHTS_COMING_SOON() {
    return comingSoon.enabled;
  },
}));

vi.mock('./components/ComingSoon', () => ({
  default: () => <div data-testid="coming-soon" />,
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

vi.mock('@wso2/oxygen-ui', () => ({
  PageContent: ({ children }: { children?: ReactNode }) => (
    <div data-testid="page-content">{children}</div>
  ),
}));

vi.mock('./InsightsEmbed', () => ({
  default: ({ scope }: { scope: InsightsEmbedScope }) => {
    embedScopes.push(scope);
    return (
      <div data-testid="insights-embed">
        {scope.level}:{scope.projectId ?? 'none'}
      </div>
    );
  },
}));

import InsightsFeature from './InsightsFeature';
import type { InsightsHostPort } from './hostPort';

const basePort: InsightsHostPort = {
  orgHandle: 'acme',
  navigate: vi.fn(),
  notify: vi.fn(),
  apiFetch: vi.fn(),
};

describe('InsightsFeature', () => {
  beforeEach(() => {
    embedScopes.length = 0;
    mockResolveProjectScope.mockReset();
    projectFilter.enabled = false;
    comingSoon.enabled = false;
  });

  describe('when SHOW_INSIGHTS_COMING_SOON is true', () => {
    beforeEach(() => {
      comingSoon.enabled = true;
    });

    it('shows Coming Soon instead of the embed for ACP', async () => {
      render(
        <InsightsFeature
          port={basePort}
          forcedScopeLevel="organization"
          embedProfile="api-control-plane"
        />
      );

      expect(await screen.findByTestId('coming-soon')).toBeInTheDocument();
      expect(screen.queryByTestId('insights-embed')).not.toBeInTheDocument();
      expect(screen.queryByTestId('page-content')).not.toBeInTheDocument();
    });

    it('keeps AI Workspace page chrome around Coming Soon', async () => {
      render(<InsightsFeature port={basePort} embedProfile="ai-workspace" />);

      expect(await screen.findByTestId('page-content')).toContainElement(
        screen.getByTestId('coming-soon')
      );
    });

    it('still resolves the project scope before showing Coming Soon', async () => {
      projectFilter.enabled = true;
      mockResolveProjectScope.mockResolvedValue({
        projectId: 'p-1',
        projectName: 'Project A',
      });
      render(
        <InsightsFeature
          port={{ ...basePort, projectHandle: 'project-a' }}
          forcedScopeLevel="project"
          embedProfile="api-control-plane"
        />
      );

      expect(await screen.findByTestId('coming-soon')).toBeInTheDocument();
      expect(mockResolveProjectScope).toHaveBeenCalledTimes(1);
      expect(screen.queryByTestId('insights-embed')).not.toBeInTheDocument();
    });

    it('still reports a missing organization instead of Coming Soon', () => {
      render(
        <InsightsFeature
          port={{ ...basePort, orgHandle: '' }}
          embedProfile="api-control-plane"
        />
      );

      expect(screen.getByTestId('error-state')).toBeInTheDocument();
      expect(screen.queryByTestId('coming-soon')).not.toBeInTheDocument();
    });
  });

  describe('when ENABLE_ACP_PROJECT_FILTER is false', () => {
    it('uses organization embed for ACP project scope', async () => {
      render(
        <InsightsFeature
          port={{ ...basePort, projectHandle: 'project-a' }}
          forcedScopeLevel="project"
          embedProfile="api-control-plane"
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('insights-embed')).toHaveTextContent(
          'organization:none'
        );
      });
      expect(mockResolveProjectScope).not.toHaveBeenCalled();
      expect(embedScopes.at(-1)).toEqual({
        level: 'organization',
        projectId: null,
        projectName: null,
      });
    });

    it('uses organization embed for ACP organization scope', async () => {
      render(
        <InsightsFeature
          port={basePort}
          forcedScopeLevel="organization"
          embedProfile="api-control-plane"
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('insights-embed')).toHaveTextContent(
          'organization:none'
        );
      });
      expect(mockResolveProjectScope).not.toHaveBeenCalled();
    });
  });

  describe('when ENABLE_ACP_PROJECT_FILTER is true', () => {
    beforeEach(() => {
      projectFilter.enabled = true;
    });

    it('embeds project Insights when project scope resolves', async () => {
      mockResolveProjectScope.mockResolvedValue({
        projectId: 'id-a',
        projectName: 'Project A',
      });

      render(
        <InsightsFeature
          port={{ ...basePort, projectHandle: 'project-a' }}
          forcedScopeLevel="project"
          embedProfile="api-control-plane"
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('insights-embed')).toHaveTextContent(
          'project:id-a'
        );
      });
      expect(mockResolveProjectScope).toHaveBeenCalledWith(
        basePort.apiFetch,
        'project-a'
      );
      expect(embedScopes.at(-1)).toEqual({
        level: 'project',
        projectId: 'id-a',
        projectName: 'Project A',
      });
    });

    it('falls back to organization Insights when project scope resolve fails', async () => {
      mockResolveProjectScope.mockRejectedValue(
        new Error('Project "missing" was not found')
      );

      render(
        <InsightsFeature
          port={{ ...basePort, projectHandle: 'missing' }}
          forcedScopeLevel="project"
          embedProfile="api-control-plane"
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('insights-embed')).toHaveTextContent(
          'organization:none'
        );
      });
      expect(screen.queryByTestId('error-state')).not.toBeInTheDocument();
      expect(embedScopes.at(-1)).toEqual({
        level: 'organization',
        projectId: null,
        projectName: null,
      });
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

      mockResolveProjectScope.mockImplementation((_apiFetch, handle) => {
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
          embedProfile="api-control-plane"
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('insights-embed')).toHaveTextContent(
          'project:id-a'
        );
      });
      const embedRenderCountAfterA = embedScopes.length;

      rerender(
        <InsightsFeature
          port={{ ...basePort, projectHandle: 'project-b' }}
          forcedScopeLevel="project"
          embedProfile="api-control-plane"
        />
      );

      expect(screen.queryByTestId('insights-embed')).not.toBeInTheDocument();
      expect(screen.getByTestId('loading-state')).toHaveTextContent(
        'Preparing Insights'
      );
      expect(embedScopes.length).toBe(embedRenderCountAfterA);

      resolveProjectB({ projectId: 'id-b', projectName: 'Project B' });

      await waitFor(() => {
        expect(screen.getByTestId('insights-embed')).toHaveTextContent(
          'project:id-b'
        );
      });
      expect(embedScopes.at(-1)).toEqual({
        level: 'project',
        projectId: 'id-b',
        projectName: 'Project B',
      });
    });
  });

  it('uses ai-overview embed for AI Workspace without resolving project_id', async () => {
    projectFilter.enabled = true;

    render(
      <InsightsFeature
        port={{ ...basePort, projectHandle: 'orders' }}
        forcedScopeLevel="project"
        embedProfile="ai-workspace"
      />
    );

    await waitFor(() => {
      expect(screen.getByTestId('insights-embed')).toHaveTextContent(
        'organization:none'
      );
    });
    expect(mockResolveProjectScope).not.toHaveBeenCalled();
  });

  it('wraps AI Workspace Insights in PageContent for shell padding', async () => {
    render(
      <InsightsFeature
        port={basePort}
        forcedScopeLevel="organization"
        embedProfile="ai-workspace"
      />
    );

    await waitFor(() => {
      expect(screen.getByTestId('insights-embed')).toBeInTheDocument();
    });
    expect(screen.getByTestId('page-content')).toContainElement(
      screen.getByTestId('insights-embed')
    );
  });

  it('does not nest PageContent for API Control Plane', async () => {
    render(
      <InsightsFeature
        port={basePort}
        forcedScopeLevel="organization"
        embedProfile="api-control-plane"
      />
    );

    await waitFor(() => {
      expect(screen.getByTestId('insights-embed')).toBeInTheDocument();
    });
    expect(screen.queryByTestId('page-content')).not.toBeInTheDocument();
  });
});
