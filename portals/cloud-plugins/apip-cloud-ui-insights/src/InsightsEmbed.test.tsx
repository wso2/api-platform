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

import { act, render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { MOESIF_EMBEDDED_POST_MESSAGE_TYPES } from './utils/moesifEmbed';

const mockFetchViewerToken = vi.fn();

vi.mock('./api/analyticsApi', () => ({
  fetchViewerToken: () => mockFetchViewerToken(),
}));

vi.mock('./config/runtimeConfig', () => ({
  insightsRuntimeConfig: {
    moesifAppUrl: 'https://web-dev.moesif.com',
    platformApiBaseUrl: '/proxy',
    platformApiVersion: 'v0.9',
  },
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
      Header: ({ children }: { children?: ReactNode }) => <h1>{children}</h1>,
      SubHeader: ({ children }: { children?: ReactNode }) => <p>{children}</p>,
    }
  ),
}));

import InsightsEmbed from './InsightsEmbed';

describe('InsightsEmbed', () => {
  beforeEach(() => {
    mockFetchViewerToken.mockReset();
    mockFetchViewerToken.mockResolvedValue('viewer-token');
  });

  it('keeps the Moesif iframe hidden until the embed handshake completes', async () => {
    render(<InsightsEmbed scope={{ level: 'organization' }} />);

    const iframe = await waitFor(() => {
      const element = screen.getByTitle('Moesif Insights') as HTMLIFrameElement;
      expect(element).toBeInTheDocument();
      return element;
    });

    expect(iframe.style.display).toBe('none');
    expect(screen.getByTestId('loading-state')).toBeInTheDocument();

    act(() => {
      window.dispatchEvent(
        new MessageEvent('message', {
          data: {
            type: MOESIF_EMBEDDED_POST_MESSAGE_TYPES.SCHEMA_GEN_FINISHED,
          },
          origin: 'https://web-dev.moesif.com',
          // wrap/basic can post from nested frames.
          source: null,
        })
      );
    });

    await waitFor(() => {
      expect(iframe.style.display).toBe('block');
    });
    expect(screen.queryByTestId('loading-state')).not.toBeInTheDocument();
  });
});
