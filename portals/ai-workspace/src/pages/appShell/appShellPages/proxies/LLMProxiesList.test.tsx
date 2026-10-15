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

import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { renderWithProviders, screen } from '../../../../test/utils';
import LLMProxiesList from './LLMProxiesList';
import type { LLMProvider, Proxy } from '../../../../utils/types';

const PROVIDERS = [
  { id: 'test-openai', displayName: 'Test Openai' } as LLMProvider,
  { id: 'test-anthropic', displayName: 'Test Anthropic' } as LLMProvider,
];

let proxies: Proxy[] = [];
let isProxiesLoading = false;

vi.mock('../../../../contexts/proxy', () => ({
  useProxies: () => ({
    proxiesResponse: { list: proxies, pagination: { total: proxies.length } },
    isLoading: isProxiesLoading,
    error: null,
    deleteProxy: vi.fn(),
    refreshProxies: vi.fn(),
  }),
}));
vi.mock('../../../../contexts/llmProvider', () => ({
  useLLMProviders: () => ({ providersResponse: { list: PROVIDERS }, isLoading: false }),
}));
vi.mock('../../../../contexts/AppShellContext', () => ({
  useAppShell: () => ({
    currentOrganization: { uuid: 'org-1', handle: 'default' },
    currentProject: { id: 'proj-1', name: 'test', handle: 'test' },
    projectsForCurrentOrganization: [{ id: 'proj-1', name: 'test', handle: 'test' }],
    setCurrentProject: vi.fn(),
    isProjectsLoading: false,
  }),
}));
vi.mock('../../../../contexts/AppAuthContext', () => ({
  useAppAuth: () => ({ hasPermission: () => true }),
}));
vi.mock('../../../../hooks/aiWorkspaceSnackbar', () => ({
  default: () => vi.fn(),
  useAIWorkspaceSnackbar: () => vi.fn(),
}));

const proxy = (id: string, provider: string): Proxy =>
  ({
    id,
    displayName: id,
    version: 'v1.0',
    provider,
    updatedAt: '2026-10-01T00:00:00Z',
  }) as Proxy;

beforeEach(() => {
  vi.clearAllMocks();
  proxies = [proxy('demo-mpp', 'test-openai')];
  isProxiesLoading = false;
});

describe('the proxy table', () => {
  it('names the provider column for the one it shows', () => {
    renderWithProviders(<LLMProxiesList />);

    // The cell renders the proxy's primary, so the heading says which one.
    expect(screen.getByText('Primary Provider')).toBeInTheDocument();
    expect(screen.queryByText('Service Provider')).not.toBeInTheDocument();
  });

  it('shows the primary provider against each proxy', () => {
    renderWithProviders(<LLMProxiesList />);

    expect(screen.getByText('Test Openai')).toBeInTheDocument();
  });
});
