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
import { act } from '@testing-library/react';

import { renderWithProviders, screen } from '../../../../test/utils';
import LLMProxyProviderTab from './LLMProxyProviderTab';
import type { Proxy, ProxyProviderEntry } from '../../../../utils/types';

/* ------------------------------------------------------------------ *
 * The surrounding application, reduced to what this tab reads from it.
 * ------------------------------------------------------------------ */

let proxy: Proxy | null = null;
const setLocalProxy = vi.fn((update: unknown) => {
  proxy = typeof update === 'function' ? (update as (p: Proxy | null) => Proxy | null)(proxy) : (update as Proxy);
});
let providersResponse = { list: [] as unknown[] };
let isProvidersLoading = false;
let templatesResponse = { list: [] as unknown[] };
let transformerPolicies: unknown[] = [];
let transformerPoliciesLoaded = true;
const reloadTransformerPolicies = vi.fn();

vi.mock('../../../../contexts/proxy', () => ({
  useProxy: () => ({ proxy, setLocalProxy }),
}));
vi.mock('../../../../contexts/llmProvider', () => ({
  useLLMProviders: () => ({ providersResponse, isLoading: isProvidersLoading }),
}));
vi.mock('../../../../contexts/llmProvider/providerTemplate', () => ({
  useProviderTemplates: () => ({ templatesResponse }),
}));
vi.mock('../../../../contexts/AppShellContext', () => ({
  useAppShell: () => ({ currentOrganization: { uuid: 'org-1' } }),
}));
vi.mock('../../../../contexts/AppAuthContext', () => ({
  useAppAuth: () => ({ hasPermission: () => true }),
}));
vi.mock('../../../../hooks/useTransformerPolicies', () => ({
  default: () => ({
    policies: transformerPolicies,
    isLoaded: transformerPoliciesLoaded,
    isLoading: false,
    error: null,
    reload: reloadTransformerPolicies,
  }),
}));

/**
 * The two panels are driven from Cypress, not from here. They are stubbed so
 * this test can exercise what the tab does with what they hand back, without
 * standing up either panel's own form.
 */
const savedEntry: { next: ProxyProviderEntry; detail: unknown } = {
  next: { id: 'replacement', isPrimary: false },
  detail: null,
};
const drawerProps: { settings?: Record<string, unknown>; transformer?: Record<string, unknown> } = {};

vi.mock('../../../../Components/Transformer/ProviderSettingsDrawer', () => ({
  default: (props: Record<string, unknown>) => {
    drawerProps.settings = props;
    return props.open ? (
      <button
        type="button"
        data-cyid="stub-settings-save"
        onClick={() =>
          (props.onSave as (n: ProxyProviderEntry, d: unknown) => void)(savedEntry.next, savedEntry.detail)
        }
      >
        save settings
      </button>
    ) : null;
  },
}));
vi.mock('../../../../Components/Transformer/TransformerDrawer', () => ({
  default: (props: Record<string, unknown>) => {
    drawerProps.transformer = props;
    return props.open ? (
      <button
        type="button"
        data-cyid="stub-transformer-apply"
        onClick={() =>
          (props.onApply as (c: unknown) => void)({ type: 'openai-to-anthropic-transformer', version: 'v0' })
        }
      >
        apply transformer
      </button>
    ) : null;
  },
}));

/* ------------------------------------------------------------------ *
 * Fixtures
 * ------------------------------------------------------------------ */

const provider = (id: string, displayName: string, template: string) => ({
  id,
  displayName,
  template,
  vhost: `${id}.example.com`,
  openapi: `openapi-for-${id}`,
  security: { enabled: true, apiKey: { enabled: true, key: 'Authorization', in: 'header' } },
});

const THREE_PROVIDERS = [
  provider('test-openai', 'Test Openai', 'openai'),
  provider('test-anthropic', 'Test Anthropic', 'anthropic'),
  provider('test-mistral', 'Test Mistral', 'mistralai'),
];

const TEMPLATES = [
  { id: 'openai', displayName: 'OpenAI' },
  { id: 'anthropic', displayName: 'Anthropic' },
  { id: 'mistralai', displayName: 'Mistral AI' },
];

const proxyWith = (providers: ProxyProviderEntry[], extra: Partial<Proxy> = {}): Proxy =>
  ({ name: 'test-proxy', providers, ...extra }) as Proxy;

const cy = (c: HTMLElement, id: string) => c.querySelector(`[data-cyid="${id}"]`) as HTMLElement;
const btn = (c: HTMLElement, id: string) => cy(c, id)?.querySelector('button') ?? cy(c, id);

beforeEach(() => {
  vi.clearAllMocks();
  proxy = proxyWith([
    { id: 'test-openai', isPrimary: true },
    { id: 'test-anthropic', isPrimary: false },
  ]);
  providersResponse = { list: THREE_PROVIDERS };
  isProvidersLoading = false;
  templatesResponse = { list: TEMPLATES };
  transformerPolicies = [];
  transformerPoliciesLoaded = true;
  savedEntry.next = { id: 'replacement', isPrimary: false };
  savedEntry.detail = null;
});

const render = (props = {}) => renderWithProviders(<LLMProxyProviderTab {...props} />);

describe('the inbound interface this list is measured against', () => {
  it('names the interface the way the catalogue names it', () => {
    proxy = proxyWith([{ id: 'test-openai', isPrimary: true }], { inboundTemplate: 'anthropic' });

    render();

    expect(screen.getByText('Anthropic')).toBeInTheDocument();
  });

  it('falls back to the primary provider\'s own format when the proxy declares none', () => {
    proxy = proxyWith([
      { id: 'test-anthropic', isPrimary: false },
      { id: 'test-mistral', isPrimary: true },
    ]);

    // The primary by name, not by position: the first row is not necessarily
    // the provider whose format the proxy falls back to.
    render();

    expect(screen.getByText('Mistral AI')).toBeInTheDocument();
  });

  it('still says which interface is in effect when the catalogue no longer lists it', () => {
    proxy = proxyWith([{ id: 'test-openai', isPrimary: true }], { inboundTemplate: 'retired-handle' });
    templatesResponse = { list: [] };

    // The handle is the fallback rather than the label, so the field reports
    // what the proxy runs on instead of appearing unset.
    render();

    expect(screen.getByText('retired-handle')).toBeInTheDocument();
  });

  it('shows a placeholder when there is nothing to fall back to either', () => {
    proxy = proxyWith([]);

    render();

    expect(screen.getByText('—')).toBeInTheDocument();
  });

  it('sends the reader to where the interface is actually editable', async () => {
    const onChangeInDefinition = vi.fn();
    const { user, container } = render({ onChangeInDefinition });

    await user.click(btn(container, 'change-in-definition'));

    // Not editable here: it belongs to the definition, and a second control
    // could disagree with the first.
    expect(onChangeInDefinition).toHaveBeenCalled();
  });
});

describe('the provider list', () => {
  it('counts the providers the proxy routes to', () => {
    render();

    expect(screen.getByRole('heading').textContent).toContain('Providers');
    expect(screen.getByRole('heading').textContent).toContain('2');
  });

  it('names each provider as the catalogue does', () => {
    render();

    expect(screen.getByText('Test Openai')).toBeInTheDocument();
    expect(screen.getByText('Test Anthropic')).toBeInTheDocument();
  });

  it('falls back to the stored id for a provider that has since been deleted', () => {
    proxy = proxyWith([{ id: 'deleted-provider', isPrimary: true }]);

    render();

    // Shown as the name and, with no alias, as the request handle too.
    expect(screen.getAllByText(/deleted-provider/).length).toBeGreaterThan(0);
  });

  it('reads the proxy\'s own list, not the single legacy field', () => {
    proxy = proxyWith(
      [
        { id: 'test-openai', isPrimary: true },
        { id: 'test-anthropic', isPrimary: false },
        { id: 'test-mistral', isPrimary: false },
      ],
      { provider: 'test-openai' } as Partial<Proxy>
    );

    // The legacy field names only the primary, so a proxy with several
    // providers would display as if it had one.
    render();

    expect(screen.getByText('Test Mistral')).toBeInTheDocument();
  });
});

describe('moving the primary marker', () => {
  it('makes the chosen provider primary', async () => {
    const { user, container } = render();

    await user.click(cy(container, 'provider-row-1-primary').querySelector('input') as HTMLElement);

    expect(proxy?.providers).toEqual([
      { id: 'test-openai', isPrimary: false },
      { id: 'test-anthropic', isPrimary: true },
    ]);
  });

  it('writes down the interface that was only implied before moving it', async () => {
    const { user, container } = render();

    await user.click(cy(container, 'provider-row-1-primary').querySelector('input') as HTMLElement);

    // Without this, changing the primary silently changes what clients may
    // send to a proxy already serving them.
    expect(proxy?.inboundTemplate).toBe('openai');
  });

  it('leaves a declared interface exactly as it was', async () => {
    proxy = proxyWith(
      [
        { id: 'test-openai', isPrimary: true },
        { id: 'test-anthropic', isPrimary: false },
      ],
      { inboundTemplate: 'anthropic' }
    );
    const { user, container } = render();

    await user.click(cy(container, 'provider-row-1-primary').querySelector('input') as HTMLElement);

    expect(proxy?.inboundTemplate).toBe('anthropic');
  });

  it('changes nothing on a proxy that cannot be edited', async () => {
    proxy = proxyWith(
      [
        { id: 'test-openai', isPrimary: true },
        { id: 'test-anthropic', isPrimary: false },
      ],
      { readOnly: true }
    );
    const { container } = render();

    expect(cy(container, 'provider-row-1-primary').querySelector('input')).toBeDisabled();
  });
});

describe('detaching a provider', () => {
  it('removes the one asked for', async () => {
    const { user, container } = render();

    await user.click(btn(container, 'provider-row-1-remove'));

    expect(proxy?.providers).toEqual([{ id: 'test-openai', isPrimary: true }]);
  });

  it('hands the primary marker to what remains when the primary is removed', async () => {
    const { user, container } = render();

    await user.click(btn(container, 'provider-row-0-remove'));

    // A proxy always has a primary.
    expect(proxy?.providers).toEqual([{ id: 'test-anthropic', isPrimary: true }]);
  });

  it('withholds removal from the last remaining provider', () => {
    proxy = proxyWith([{ id: 'test-openai', isPrimary: true }]);

    const { container } = render();

    expect(btn(container, 'provider-row-0-remove')).toBeDisabled();
  });

  it('changes nothing on a proxy that cannot be edited', async () => {
    proxy = proxyWith(
      [
        { id: 'test-openai', isPrimary: true },
        { id: 'test-anthropic', isPrimary: false },
      ],
      { readOnly: true }
    );
    const { container } = render();

    expect(btn(container, 'provider-row-1-remove')).toBeDisabled();
  });
});

describe('attaching another provider', () => {
  it('offers the action once the provider catalogue has answered', () => {
    const { container } = render();

    expect(btn(container, 'add-provider-button')).not.toBeDisabled();
  });

  it('withholds it while the catalogue is still loading', () => {
    isProvidersLoading = true;

    const { container } = render();

    expect(btn(container, 'add-provider-button')).toBeDisabled();
  });

  it('withholds it on a proxy that cannot be edited', () => {
    proxy = proxyWith([{ id: 'test-openai', isPrimary: true }], { readOnly: true });

    const { container } = render();

    expect(btn(container, 'add-provider-button')).toBeDisabled();
  });

  it('appends what the panel produced', async () => {
    savedEntry.next = { id: 'test-mistral', isPrimary: false };
    const { user, container } = render();

    await user.click(btn(container, 'add-provider-button'));
    await user.click(btn(container, 'stub-settings-save'));

    expect(proxy?.providers?.map((e) => e.id)).toEqual([
      'test-openai',
      'test-anthropic',
      'test-mistral',
    ]);
  });

  it('makes the first provider on an empty proxy its primary', async () => {
    proxy = proxyWith([]);
    savedEntry.next = { id: 'test-openai', isPrimary: false };
    const { user, container } = render();

    await user.click(btn(container, 'add-provider-button'));
    await user.click(btn(container, 'stub-settings-save'));

    expect(proxy?.providers).toEqual([{ id: 'test-openai', isPrimary: true }]);
  });
});

describe('editing an attached provider', () => {
  it('writes the edit onto the provider the panel was opened on', async () => {
    savedEntry.next = { id: 'test-anthropic', isPrimary: false, alias: 'claude' };
    const { user, container } = render();

    await user.click(btn(container, 'provider-row-1-edit'));
    await user.click(btn(container, 'stub-settings-save'));

    expect(proxy?.providers).toEqual([
      { id: 'test-openai', isPrimary: true },
      { id: 'test-anthropic', isPrimary: false, alias: 'claude' },
    ]);
  });

  it('takes on the new provider\'s identity when the primary becomes a different provider', async () => {
    savedEntry.next = { id: 'test-mistral', isPrimary: true };
    savedEntry.detail = THREE_PROVIDERS[2];
    const { user, container } = render();

    await user.click(btn(container, 'provider-row-0-edit'));
    await user.click(btn(container, 'stub-settings-save'));

    // The proxy's published specification, host, and what it requires of
    // callers all come from the primary.
    expect(proxy?.vhost).toBe('test-mistral.example.com');
    expect(proxy?.openapi).toBe('openapi-for-test-mistral');
    expect(proxy?.security).toEqual({
      enabled: true,
      apiKey: { enabled: true, key: 'Authorization', in: 'header', valuePrefix: undefined },
    });
  });

  it('leaves the proxy\'s identity alone when the primary is the same provider', async () => {
    proxy = proxyWith([{ id: 'test-openai', isPrimary: true }], { vhost: 'kept.example.com' });
    savedEntry.next = { id: 'test-openai', isPrimary: true, alias: 'gpt' };
    savedEntry.detail = THREE_PROVIDERS[0];
    const { user, container } = render();

    await user.click(btn(container, 'provider-row-0-edit'));
    await user.click(btn(container, 'stub-settings-save'));

    // Deriving it again from the same provider would rewrite fields nobody
    // edited, so a panel closed without a change would read as an edit.
    expect(proxy?.vhost).toBe('kept.example.com');
  });

  it('writes down the implied interface when the primary moves to another provider', async () => {
    savedEntry.next = { id: 'test-mistral', isPrimary: true };
    savedEntry.detail = THREE_PROVIDERS[2];
    const { user, container } = render();

    await user.click(btn(container, 'provider-row-0-edit'));
    await user.click(btn(container, 'stub-settings-save'));

    expect(proxy?.inboundTemplate).toBe('openai');
  });

  it('does not resurrect a provider that was removed while the panel was open', async () => {
    savedEntry.next = { id: 'test-anthropic', isPrimary: false, alias: 'claude' };
    const { user, container } = render();

    await user.click(btn(container, 'provider-row-1-edit'));
    // The provider is detached underneath the open panel.
    proxy = proxyWith([{ id: 'test-openai', isPrimary: true }]);
    await user.click(btn(container, 'stub-settings-save'));

    expect(proxy?.providers).toEqual([{ id: 'test-openai', isPrimary: true }]);
  });

  it('refuses the write outright on a proxy that cannot be edited', () => {
    proxy = proxyWith([{ id: 'test-openai', isPrimary: true }], { readOnly: true });
    const before = proxy.providers;
    render();

    // The panel cannot be opened here — the row's action is withheld — so the
    // handler is driven directly to show it refuses on its own account too.
    act(() => {
      (drawerProps.settings?.onSave as (n: ProxyProviderEntry, d: unknown) => void)(
        { id: 'test-mistral', isPrimary: true },
        THREE_PROVIDERS[2]
      );
    });

    expect(proxy?.providers).toBe(before);
  });
});

describe('choosing what translates for a provider', () => {
  it('attaches the chosen translator to the provider the table opened', async () => {
    const { user, container } = render();

    await user.click(btn(container, 'provider-row-1-transformer-add'));
    await user.click(btn(container, 'stub-transformer-apply'));

    expect(proxy?.providers?.[1].transformer).toEqual({
      type: 'openai-to-anthropic-transformer',
      version: 'v0',
    });
  });

  it('detaches the one attached', async () => {
    proxy = proxyWith([
      { id: 'test-openai', isPrimary: true },
      {
        id: 'test-anthropic',
        isPrimary: false,
        transformer: { type: 'openai-to-anthropic-transformer', version: 'v0' },
      },
    ]);
    const { user, container } = render();

    await user.click(screen.getByLabelText('Remove transformer'));

    expect(proxy?.providers?.[1].transformer).toBeNull();
  });

  it('opens on the attached translator rather than on the list', async () => {
    proxy = proxyWith([
      { id: 'test-openai', isPrimary: true },
      {
        id: 'test-anthropic',
        isPrimary: false,
        transformer: { type: 'openai-to-anthropic-transformer', version: 'v0' },
      },
    ]);
    const { user, container } = render();

    await user.click(cy(container, 'provider-row-1-transformer'));

    expect(drawerProps.transformer?.configureOnly).toBe(true);
    expect(drawerProps.transformer?.current).toEqual({
      type: 'openai-to-anthropic-transformer',
      version: 'v0',
    });
  });

  it('says so for a provider that already speaks the inbound format', async () => {
    const { user, container } = render();

    await user.click(btn(container, 'provider-row-0-transformer-add'));

    // The primary speaks openai, which is the interface in effect.
    expect(drawerProps.transformer?.notRequired).toBe(true);
  });

  it('refuses the write outright on a proxy that cannot be edited', () => {
    proxy = proxyWith(
      [
        { id: 'test-openai', isPrimary: true },
        { id: 'test-anthropic', isPrimary: false },
      ],
      { readOnly: true }
    );
    const before = proxy.providers;
    const { container } = render();

    expect(btn(container, 'provider-row-1-transformer-add')).toBeDisabled();
    act(() => {
      (drawerProps.transformer?.onApply as (c: unknown) => void)({
        type: 'openai-to-anthropic-transformer',
        version: 'v0',
      });
    });

    expect(proxy?.providers).toBe(before);
  });
});

describe('what the settings panel is told', () => {
  it('names every other attached provider, so one cannot be attached twice', async () => {
    const { user, container } = render();

    await user.click(btn(container, 'provider-row-0-edit'));

    expect(drawerProps.settings?.attachedProviderIds).toEqual(['test-anthropic']);
  });

  it('names every attached provider while a new one is being described', async () => {
    const { user, container } = render();

    await user.click(btn(container, 'add-provider-button'));

    expect(drawerProps.settings?.attachedProviderIds).toEqual(['test-openai', 'test-anthropic']);
  });
});
