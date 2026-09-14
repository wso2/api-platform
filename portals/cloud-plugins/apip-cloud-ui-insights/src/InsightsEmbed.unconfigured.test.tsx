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

import { render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('./api/analyticsApi', () => ({
  fetchViewerToken: vi.fn(),
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
  Box: ({
    children,
    ...props
  }: {
    children?: ReactNode;
    sx?: unknown;
  }) => <div {...props}>{children}</div>,
  PageTitle: Object.assign(
    ({ children }: { children?: ReactNode }) => <div>{children}</div>,
    {
      Header: ({ children }: { children?: ReactNode }) => (
        <h1>{children}</h1>
      ),
      SubHeader: ({ children }: { children?: ReactNode }) => (
        <p>{children}</p>
      ),
    }
  ),
}));

describe('InsightsEmbed without Moesif runtime config', () => {
  afterEach(() => {
    delete window.config;
    delete window.__RUNTIME_CONFIG__;
    vi.resetModules();
    vi.unstubAllEnvs();
  });

  beforeEach(() => {
    delete window.config;
    delete window.__RUNTIME_CONFIG__;
    vi.unstubAllEnvs();
  });

  it('renders not-configured error and never mounts a Moesif iframe', async () => {
    const { default: InsightsEmbed } = await import('./InsightsEmbed');

    render(<InsightsEmbed scope={{ level: 'organization' }} />);

    expect(screen.getByTestId('error-state')).toHaveTextContent(
      'Insights is not configured for this deployment.'
    );
    expect(screen.queryByTitle('Moesif Insights')).not.toBeInTheDocument();
  });
});
