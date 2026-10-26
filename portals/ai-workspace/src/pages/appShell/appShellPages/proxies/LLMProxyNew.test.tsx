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
import { waitFor } from '@testing-library/react';

import { renderWithProviders, screen } from '../../../../test/utils';
import LLMProxyNew from './LLMProxyNew';
import type { LLMProvider } from '../../../../utils/types';

/* ---------------------------------------------------------------- *
 * The application around the form, reduced to what it reads.
 * ---------------------------------------------------------------- */

const provider = (id: string, displayName: string, needsKey = true): LLMProvider =>
  ({
    id,
    displayName,
    template: 'openai',
    updatedAt: '2026-10-01T00:00:00Z',
    openapi: '',
    upstream: { main: { url: 'https://api.example.invalid', auth: { type: 'none' } } },
    security: needsKey
      ? { enabled: true, apiKey: { enabled: true, key: 'Authorization', in: 'header' } }
      : { enabled: false },
  }) as LLMProvider;

const OPENAI = provider('test-openai', 'Test Openai');
const ANTHROPIC = provider('test-anthropic', 'Test Anthropic');

let providersResponse: { list: LLMProvider[] } = { list: [OPENAI, ANTHROPIC] };
let isProvidersLoading = false;
/** The primary's own detail, as the single-provider context answers for it. */
let selectedProvider: LLMProvider | null = OPENAI;
let selectedProviderError: Error | null = null;
const refetchSelectedProvider = vi.fn();
const getLLMProvider = vi.fn();

vi.mock('../../../../contexts/proxy', () => ({
  useProxies: () => ({ createProxy: vi.fn() }),
}));
vi.mock('../../../../contexts/llmProvider', () => ({
  LLMProviderProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useLLMProviders: () => ({ providersResponse, isLoading: isProvidersLoading }),
  useLLMProvider: () => ({
    provider: selectedProvider,
    isLoading: false,
    error: selectedProviderError,
    refetch: refetchSelectedProvider,
  }),
}));
vi.mock('../../../../contexts/llmProvider/providerTemplate', () => ({
  ProviderTemplatesProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useProviderTemplates: () => ({
    templatesResponse: { list: [{ id: 'openai', displayName: 'OpenAI', enabled: true, isLatest: true }] },
    isLoading: false,
    error: null,
  }),
}));
vi.mock('../../../../contexts/AppShellContext', () => ({
  useAppShell: () => ({
    currentOrganization: { uuid: 'org-1' },
    currentProject: { id: 'proj-1', name: 'test', handle: 'test' },
    projectsForCurrentOrganization: [{ id: 'proj-1', name: 'test', handle: 'test' }],
    setCurrentProject: vi.fn(),
  }),
}));
vi.mock('../../../../contexts/AppAuthContext', () => ({
  useAppAuth: () => ({ hasPermission: () => true }),
}));
vi.mock('../../../../hooks/aiWorkspaceSnackbar', () => ({
  useAIWorkspaceSnackbar: () => vi.fn(),
}));
vi.mock('../../../../hooks/useTransformerPolicies', () => ({
  TRANSFORMER_POLICY_CATEGORY: 'LLM Transformation',
  FALLBACK_POLICY_CATEGORY: 'AI',
  default: () => ({
    policies: [],
    isLoaded: true,
    isLoading: false,
    error: null,
    reload: vi.fn(),
  }),
}));
vi.mock('../../../../apis/llmProviderApis', () => ({
  getLLMProvider: (...args: unknown[]) => getLLMProvider(...args),
  createLLMProviderAPIKey: vi.fn(),
}));

const cy = (c: HTMLElement, id: string) => c.querySelector(`[data-cyid="${id}"]`) as HTMLElement;

beforeEach(() => {
  vi.clearAllMocks();
  providersResponse = { list: [OPENAI, ANTHROPIC] };
  isProvidersLoading = false;
  selectedProvider = OPENAI;
  selectedProviderError = null;
  getLLMProvider.mockResolvedValue(ANTHROPIC);
});

const render = () => renderWithProviders(<LLMProxyNew />);

describe('while the selected provider is still being read', () => {
  it('stands in for the transformer and credential sections together', () => {
    // Nothing has answered for the selected provider yet.
    selectedProvider = null;

    const { container } = render();

    expect(cy(container, 'transformer-status-skeleton')).toBeInTheDocument();
    expect(cy(container, 'api-key-skeleton')).toBeInTheDocument();
  });

  it('stands in while the provider list itself is still arriving', () => {
    isProvidersLoading = true;
    selectedProvider = null;

    const { container } = render();

    expect(cy(container, 'transformer-status-skeleton')).toBeInTheDocument();
  });

  it('draws the sections once the provider has answered', () => {
    const { container } = render();

    expect(cy(container, 'transformer-status-skeleton')).toBeNull();
    expect(cy(container, 'api-key-skeleton')).toBeNull();
    expect(screen.getByText('API Keys')).toBeInTheDocument();
  });

  it('leaves both sections out where the organization holds no providers', () => {
    providersResponse = { list: [] };
    selectedProvider = null;

    const { container } = render();

    // Nothing will ever be selected, so nothing is pending.
    expect(cy(container, 'transformer-status-skeleton')).toBeNull();
    expect(cy(container, 'api-key-skeleton')).toBeNull();
  });
});

describe('when the selected provider cannot be read', () => {
  beforeEach(() => {
    selectedProvider = null;
    selectedProviderError = new Error('offline');
  });

  it('says so instead of standing in forever', () => {
    const { container } = render();

    // A placeholder that never resolves claims a request is still coming back.
    expect(cy(container, 'transformer-status-skeleton')).toBeNull();
    expect(cy(container, 'provider-detail-error')).toBeInTheDocument();
  });

  it('names the credential as what is lost, not the translator', () => {
    render();

    // What translates is decided from the list, which arrived.
    expect(
      screen.getByText(/Could not load this provider's configuration, so its credential cannot be set./)
    ).toBeInTheDocument();
  });

  it('offers the read again', async () => {
    const { user } = render();

    await user.click(screen.getByRole('button', { name: 'Retry' }));

    expect(refetchSelectedProvider).toHaveBeenCalled();
  });

  it('refuses to create, and says the read failed rather than that it is loading', async () => {
    const { user, container } = render();
    // The reason names the first unmet condition, so the earlier ones are met
    // before asking about this one.
    const name = cy(container, 'proxy-name-input').querySelector('input');
    await user.type(name as HTMLElement, 'demo');

    const create = cy(container, 'create-proxy-button');
    expect(create.querySelector('button') ?? create).toBeDisabled();

    await user.hover(create.parentElement ?? create);

    // "Loading the selected provider details" would be a lie: nothing is
    // loading, and waiting will not help.
    expect(
      await screen.findByText('Could not load the selected provider. Retry before creating.')
    ).toBeInTheDocument();
  });
});

describe('an additional provider whose own detail is being read', () => {
  const openAdditionalProviderForm = async (
    user: ReturnType<typeof renderWithProviders>['user'],
    container: HTMLElement
  ) => {
    const add = cy(container, 'add-provider-button');
    await user.click(add.querySelector('button') ?? add);
  };

  it('stands in for both of its sections', async () => {
    let settle: (value: LLMProvider) => void = () => {};
    getLLMProvider.mockImplementation(
      () => new Promise<LLMProvider>((resolve) => { settle = resolve; })
    );
    const { user, container } = render();

    await openAdditionalProviderForm(user, container);

    await waitFor(() =>
      expect(cy(container, 'transformer-status-skeleton')).toBeInTheDocument()
    );
    settle(ANTHROPIC);
  });

  it('reports a failed read and offers it again', async () => {
    getLLMProvider.mockRejectedValue(new Error('offline'));
    const { user, container } = render();

    await openAdditionalProviderForm(user, container);

    await waitFor(() =>
      expect(cy(container, 'provider-detail-error')).toBeInTheDocument()
    );
    // The skeleton gives way: the read is not coming back on its own.
    expect(cy(container, 'transformer-status-skeleton')).toBeNull();

    getLLMProvider.mockResolvedValue(ANTHROPIC);
    await user.click(screen.getByRole('button', { name: 'Retry' }));

    await waitFor(() => expect(getLLMProvider).toHaveBeenCalledTimes(2));
  });
});
