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

import { beforeEach, describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen, waitFor } from '@/test/utils';
import TestConsoleSpecViewer from './TestConsoleSpecViewer';
import type { KeyValueRow } from '../utils/types';

/**
 * Regression tests for the stale-closure bug that froze this page.
 *
 * `swagger-ui-react` builds its system in an effect with `[]` deps, so it reads
 * `plugins` and `requestInterceptor` **once, at mount** and ignores every later
 * value. The first version of this component passed freshly-built closures each
 * render, so swagger kept the mount-time ones — capturing a `baseUrl` of `''`
 * (gateways had not loaded) and no test key. The stale values were reported
 * upward, the page corrected them, and the two looped until React gave up:
 * clicks stopped landing and operations would not expand.
 *
 * The mock below reproduces exactly that capture-once behaviour, which is the
 * only way to catch this in a test. Mounting the real swagger-ui would not:
 * it is slow in jsdom, and a test that re-read the props on every render would
 * pass against the broken implementation.
 */

/** The props swagger captured at mount, as the real wrapper would. */
let captured: {
  plugins?: unknown[];
  requestInterceptor?: (request: Record<string, unknown>) => Record<string, unknown>;
  spec?: unknown;
} = {};

/** How many distinct `spec` references swagger was handed. */
let specIdentities: unknown[] = [];

vi.mock('swagger-ui-react', () => ({
  default: (props: Record<string, unknown>) => {
    specIdentities.push(props.spec);
    // Capture-once, exactly like the real wrapper's mount-time effect.
    if (!captured.requestInterceptor) {
      captured = {
        plugins: props.plugins as unknown[],
        requestInterceptor: props.requestInterceptor as typeof captured.requestInterceptor,
        spec: props.spec,
      };
    }
    return null;
  },
}));

vi.mock('swagger-ui-react/swagger-ui.css', () => ({}));

const spec = {
  openapi: '3.0.1',
  paths: { '/books': { get: { responses: {} } } },
};

/** A document with two resources and three verbs, for the filter controls. */
const richSpec = {
  openapi: '3.0.1',
  paths: {
    '/books': {
      get: { operationId: 'listBooks', summary: 'List all the reading list books' },
      post: { operationId: 'createBook', summary: 'Add a new book' },
    },
    '/authors': { get: { operationId: 'listAuthors', summary: 'List contributors' } },
  },
};

/** Paths in the document swagger was most recently handed. */
const renderedPaths = (): string[] => {
  const latest = specIdentities.at(-1) as { paths?: Record<string, unknown> } | undefined;
  return Object.keys(latest?.paths ?? {});
};

/**
 * Relay identifiers every render needs. The console does not call the gateway
 * itself — it posts to the BFF, which resolves the target from these. They are
 * inert in these tests (swagger is mocked, so nothing is ever sent), but the
 * component requires them, and defaulting them here keeps each case about the
 * behaviour it is actually asserting.
 */
const relayProps = { gatewayId: 'gw-prod', orgHandle: 'acme', restApiId: 'api-1' };

const keyRow = (value: string): KeyValueRow[] => [
  { auto: true, enabled: true, id: 'test-key', name: 'Test-Key', secret: true, value },
];

beforeEach(() => {
  captured = {};
  specIdentities = [];
});

describe('TestConsoleSpecViewer — resource search and method filter', () => {
  const renderViewer = () =>
    renderWithProviders(
      <TestConsoleSpecViewer
        {...relayProps}
        baseUrl="https://gw.example.com"
        extraHeaders={[]}
        spec={richSpec}
      />,
    );

  it('renders both controls', () => {
    renderViewer();

    expect(screen.getByPlaceholderText(/Search resources by path or description/i)).toBeVisible();
    expect(screen.getByRole('combobox', { name: /Filter by method/i })).toBeVisible();
  });

  it('narrows the document swagger is given as the search is typed', async () => {
    const { user } = renderViewer();

    expect(renderedPaths()).toEqual(['/books', '/authors']);

    await user.type(screen.getByPlaceholderText(/Search resources/i), 'authors');

    // Debounced, so the assertion waits rather than expecting it per keystroke.
    await waitFor(() => expect(renderedPaths()).toEqual(['/authors']));
  });

  it('matches an operation summary, not only a path', async () => {
    const { user } = renderViewer();

    await user.type(screen.getByPlaceholderText(/Search resources/i), 'contributors');

    await waitFor(() => expect(renderedPaths()).toEqual(['/authors']));
  });

  it('narrows by method', async () => {
    const { user } = renderViewer();

    await user.click(screen.getByRole('combobox', { name: /Filter by method/i }));
    await user.click(await screen.findByRole('option', { name: 'POST' }));

    // Only /books has a POST.
    await waitFor(() => expect(renderedPaths()).toEqual(['/books']));
  });

  it('does not re-parse the document while the filters are untouched', async () => {
    const { rerender } = renderViewer();

    rerender(
      <TestConsoleSpecViewer
        {...relayProps}
        baseUrl="https://gw.example.com"
        extraHeaders={keyRow('k')}
        spec={richSpec}
      />,
    );

    // One reference throughout: a new one makes swagger forget which
    // operations are open and discard anything typed into a try-out form.
    expect(new Set(specIdentities).size).toBe(1);
  });

  it('reports no matches without unmounting the viewer', async () => {
    const { user } = renderViewer();

    await user.type(screen.getByPlaceholderText(/Search resources/i), 'no-such-resource');

    expect(await screen.findByText(/No resources match your search/i)).toBeInTheDocument();
    // Hidden, not unmounted: tearing down swagger's store would lose every
    // expanded operation and try-out value as soon as the search was cleared.
    expect(captured.requestInterceptor).toBeDefined();
  });

  it('restores the full list when the search is cleared', async () => {
    const { user } = renderViewer();

    const field = screen.getByPlaceholderText(/Search resources/i);
    await user.type(field, 'authors');
    await waitFor(() => expect(renderedPaths()).toEqual(['/authors']));

    await user.clear(field);

    await waitFor(() => expect(renderedPaths()).toEqual(['/books', '/authors']));
  });
});

describe('TestConsoleSpecViewer', () => {
  it('gives the request interceptor the current base url, not the mount-time one', () => {
    const { rerender } = renderWithProviders(
      <TestConsoleSpecViewer {...relayProps} baseUrl="" extraHeaders={[]} spec={spec} />,
    );

    // Mounted before any gateway resolved — the exact condition that broke it.
    rerender(
      <TestConsoleSpecViewer
        {...relayProps}
        baseUrl="https://gw.example.com/payments/v1"
        extraHeaders={[]}
        spec={spec}
      />,
    );

    const onRequestChange = vi.fn();
    rerender(
      <TestConsoleSpecViewer
        {...relayProps}
        baseUrl="https://gw.example.com/payments/v1"
        extraHeaders={[]}
        onRequestChange={onRequestChange}
        spec={spec}
      />,
    );

    captured.requestInterceptor?.({
      headers: {},
      method: 'get',
      url: 'https://gw.example.com/payments/v1/books',
    });

    // A stale closure would have compared against '' and rejected this URL as
    // off-base, reporting nothing at all.
    expect(onRequestChange).toHaveBeenCalledTimes(1);
    expect(onRequestChange.mock.calls[0][0]).toMatchObject({
      baseUrl: 'https://gw.example.com/payments/v1',
      path: '/books',
    });
  });

  it('injects the test key that arrived after mount', () => {
    const { rerender } = renderWithProviders(
      <TestConsoleSpecViewer
        {...relayProps}
        baseUrl="https://gw.example.com"
        extraHeaders={[]}
        spec={spec}
      />,
    );

    // The key is minted asynchronously, so it is never present at mount.
    rerender(
      <TestConsoleSpecViewer
        {...relayProps}
        baseUrl="https://gw.example.com"
        extraHeaders={keyRow('live-credential')}
        spec={spec}
      />,
    );

    const result = captured.requestInterceptor?.({
      headers: {},
      method: 'get',
      url: 'https://gw.example.com/books',
    });

    expect((result?.headers as Record<string, string>)['Test-Key']).toBe('live-credential');
  });

  it('injects a regenerated key rather than the one it first saw', () => {
    const { rerender } = renderWithProviders(
      <TestConsoleSpecViewer
        {...relayProps}
        baseUrl="https://gw.example.com"
        extraHeaders={keyRow('first-key')}
        spec={spec}
      />,
    );

    rerender(
      <TestConsoleSpecViewer
        {...relayProps}
        baseUrl="https://gw.example.com"
        extraHeaders={keyRow('second-key')}
        spec={spec}
      />,
    );

    const result = captured.requestInterceptor?.({
      headers: {},
      method: 'get',
      url: 'https://gw.example.com/books',
    });

    expect((result?.headers as Record<string, string>)['Test-Key']).toBe('second-key');
  });

  it('omits an excluded auto header', () => {
    renderWithProviders(
      <TestConsoleSpecViewer
        {...relayProps}
        baseUrl="https://gw.example.com"
        extraHeaders={[{ ...keyRow('k')[0], enabled: false }]}
        spec={spec}
      />,
    );

    const result = captured.requestInterceptor?.({
      headers: {},
      method: 'get',
      url: 'https://gw.example.com/books',
    });

    expect(result?.headers).toEqual({});
  });

  it('keeps one spec reference across re-renders, so expansion state survives', () => {
    const { rerender } = renderWithProviders(
      <TestConsoleSpecViewer
        {...relayProps}
        baseUrl="https://gw.example.com"
        extraHeaders={[]}
        spec={spec}
      />,
    );

    rerender(
      <TestConsoleSpecViewer
        {...relayProps}
        baseUrl="https://gw.example.com"
        extraHeaders={keyRow('k')}
        spec={spec}
      />,
    );
    rerender(
      <TestConsoleSpecViewer
        {...relayProps}
        baseUrl="https://gw.example.com"
        extraHeaders={keyRow('k2')}
        spec={spec}
      />,
    );

    // swagger-ui-react re-parses the document whenever this prop's *reference*
    // changes, discarding which operations are open and everything typed into
    // them. A new object per render is what makes an operation refuse to stay
    // expanded.
    expect(new Set(specIdentities).size).toBe(1);
  });

  it('builds a new spec reference when the gateway actually changes', () => {
    const { rerender } = renderWithProviders(
      <TestConsoleSpecViewer
        {...relayProps}
        baseUrl="https://gw-a.example.com"
        extraHeaders={[]}
        spec={spec}
      />,
    );

    rerender(
      <TestConsoleSpecViewer
        {...relayProps}
        baseUrl="https://gw-b.example.com"
        extraHeaders={[]}
        spec={spec}
      />,
    );

    // Re-parsing is correct here: the document's server has genuinely changed.
    expect(new Set(specIdentities).size).toBe(2);
  });

  it('wraps none of swagger’s own components', () => {
    renderWithProviders(
      <TestConsoleSpecViewer
        {...relayProps}
        baseUrl="https://gw.example.com"
        extraHeaders={[]}
        spec={spec}
      />,
    );

    // The live sync reads swagger's store instead of wrapping its rendering.
    // An earlier version wrapped `OperationContainer` — the component that owns
    // expand/collapse — to inject a reporter beside it, which put speculative
    // code directly on the path the user reported broken.
    //
    // The one plugin that is passed installs the relay transport, and this is
    // what keeps it from drifting into the same mistake: it may wrap the
    // `spec.executeRequest` *action* and nothing else. An action wrapper never
    // participates in rendering, so swagger still renders exactly as it ships.
    const plugins = (captured.plugins ?? []) as Array<() => Record<string, unknown>>;
    expect(plugins).toHaveLength(1);

    const built = plugins.map((plugin) => plugin());
    built.forEach((definition) => {
      expect(definition.wrapComponents).toBeUndefined();
      expect(definition.components).toBeUndefined();
      expect(Object.keys(definition)).toEqual(['statePlugins']);
    });

    const specPlugin = (built[0].statePlugins as { spec: Record<string, unknown> }).spec;
    expect(Object.keys(specPlugin)).toEqual(['wrapActions']);
    expect(Object.keys(specPlugin.wrapActions as object)).toEqual(['executeRequest']);
  });
});
