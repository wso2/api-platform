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

import { renderWithProviders } from '../../../../test/utils';
import LLMProxyOverview from './LLMProxyOverview';
import type { LLMProvider, Proxy, ProxyProviderEntry } from '../../../../utils/types';

const listed = (id: string, displayName: string) =>
  ({ id, displayName, template: 'openai' }) as LLMProvider;

const PROVIDERS = [
  listed('test-openai', 'Test Openai'),
  listed('test-anthropic', 'Test Anthropic'),
  listed('test-mistral', 'Test Mistral'),
  listed('test-gemini', 'Test Gemini'),
  listed('test-bedrock', 'Test Bedrock'),
];

let proxy: Proxy | null = null;

vi.mock('../../../../contexts/proxy', () => ({
  ProxyProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useProxy: () => ({
    proxy,
    savedProxy: proxy,
    setLocalProxy: vi.fn(),
    isLoading: false,
    error: null,
    updateProxy: vi.fn(),
    deleteProxy: vi.fn(),
    refetch: vi.fn(),
  }),
  useProxies: () => ({ refreshProxies: vi.fn() }),
}));
vi.mock('../../../../contexts/llmProvider', () => ({
  useLLMProviders: () => ({ providersResponse: { list: PROVIDERS }, isLoading: false }),
}));
vi.mock('../../../../contexts/llmProvider/providerTemplate', () => ({
  ProviderTemplatesProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useProviderTemplates: () => ({
    templatesResponse: { list: [] },
    isLoading: false,
    error: null,
  }),
}));
vi.mock('../../../../contexts/AppShellContext', () => ({
  useAppShell: () => ({
    currentOrganization: { uuid: 'org-1' },
    currentProject: { id: 'proj-1', name: 'test', handle: 'test' },
  }),
}));
vi.mock('../../../../contexts/AppAuthContext', () => ({
  useAppAuth: () => ({ hasPermission: () => true }),
}));
vi.mock('../../../../hooks/aiWorkspaceSnackbar', () => ({
  default: () => vi.fn(),
  useAIWorkspaceSnackbar: () => vi.fn(),
}));

const entry = (id: string, isPrimary = false): ProxyProviderEntry => ({ id, isPrimary });

const proxyWith = (providers: ProxyProviderEntry[], extra: Partial<Proxy> = {}): Proxy =>
  ({
    id: 'demo-mpp',
    displayName: 'demo mpp',
    version: 'v1.0',
    context: '/default/demo-mpp',
    providers,
    ...extra,
  }) as Proxy;

const cy = (c: HTMLElement, id: string) => c.querySelector(`[data-cyid="${id}"]`) as HTMLElement;

beforeEach(() => {
  vi.clearAllMocks();
  proxy = proxyWith([entry('test-openai', true)]);
});

const render = () =>
  renderWithProviders(<LLMProxyOverview />, {
    route: '/proxies/demo-mpp',
    path: '/proxies/:proxyId',
  });

describe('the providers the proxy routes to', () => {
  it('names the single provider it has', () => {
    const { container } = render();

    expect(cy(container, 'proxy-providers').textContent).toBe('Test Openai');
  });

  it('names every attached provider, primary first', () => {
    proxy = proxyWith([
      entry('test-anthropic'),
      entry('test-mistral', true),
    ]);

    const { container } = render();

    // The stored order need not lead with the primary; the line does.
    expect(cy(container, 'proxy-providers').textContent).toBe(
      'Test Mistral, Test Anthropic'
    );
  });

  it('stops after the third and says it has stopped', () => {
    proxy = proxyWith([
      entry('test-openai', true),
      entry('test-anthropic'),
      entry('test-mistral'),
      entry('test-gemini'),
      entry('test-bedrock'),
    ]);

    const { container } = render();

    expect(cy(container, 'proxy-providers').textContent).toBe(
      'Test Openai, Test Anthropic, Test Mistral, …'
    );
  });

  it('shows three without trimming them', () => {
    proxy = proxyWith([
      entry('test-openai', true),
      entry('test-anthropic'),
      entry('test-mistral'),
    ]);

    const { container } = render();

    expect(cy(container, 'proxy-providers').textContent).not.toContain('…');
  });

  it('falls back to the stored id for a provider the catalogue no longer lists', () => {
    proxy = proxyWith([entry('deleted-provider', true)]);

    const { container } = render();

    expect(cy(container, 'proxy-providers').textContent).toBe('deleted-provider');
  });

  it('reads the single legacy field on a proxy that carries no list', () => {
    proxy = proxyWith([], { provider: 'test-anthropic' } as Partial<Proxy>);

    const { container } = render();

    // An older response carries only the primary; the line still names it.
    expect(cy(container, 'proxy-providers').textContent).toBe('Test Anthropic');
  });
});
