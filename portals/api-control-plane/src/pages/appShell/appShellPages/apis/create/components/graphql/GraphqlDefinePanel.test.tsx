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

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { accepts, recorder, type Recorder } from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { GraphqlDefinePanel } from './GraphqlDefinePanel';

const SAMPLE_SDL = 'type Query { hello: String }';
const ORG = 'api-platform-demo';

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

const renderPanel = (onDraftChange = vi.fn()) => {
  const rendered = renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <GraphqlDefinePanel onDraftChange={onDraftChange} />
    </ApiScopeProvider>,
  );
  return { onDraftChange, ...rendered };
};

/**
 * `GraphqlDefinePanel` used to hand the wizard's configure step a draft with
 * every field name from `GraphqlResolvedSchema` present as an *own key*, even
 * ones the resolved schema had no value for (`endpointUrl` on a URL-sourced
 * resolve, say) — set explicitly to `undefined`. The configure step fills its
 * defaults via `{...DEFAULT, ...draft}`, so an own `undefined` there clobbers
 * the default instead of falling through to it, and crashed the form the
 * moment it read `state.endpointUrl.trim()`. These tests are the regression
 * guard for that fix: an absent field must be an *absent key*, not one with an
 * explicit `undefined` value.
 */
describe('GraphqlDefinePanel — draft field presence', () => {
  it('omits `endpointUrl` from the draft when the schema came from a URL', async () => {
    server.use(
      accepts('post', '/graphql-apis/validate-schema', { resolved: true, sdl: SAMPLE_SDL }, {
        record: requests,
      }),
    );
    const { onDraftChange, user } = renderPanel();

    await user.type(screen.getByLabelText(/Schema URL/), 'https://raw.example.com/schema.graphql');
    await user.tab();

    await screen.findByText('Queries');

    const lastCall = onDraftChange.mock.calls.at(-1)?.[0];
    expect(lastCall).toMatchObject({
      schemaSource: 'url',
      sdl: SAMPLE_SDL,
      sdlUrl: 'https://raw.example.com/schema.graphql',
    });
    expect(lastCall).not.toHaveProperty('endpointUrl');
    expect(lastCall).not.toHaveProperty('sdlFile');
  });

  it('carries `endpointUrl` (and no `sdlUrl`) when the "Start from scratch" endpoint resolves', async () => {
    server.use(
      accepts(
        'post',
        '/graphql-apis/validate-schema',
        { resolved: true, sdl: SAMPLE_SDL },
        { record: requests },
      ),
    );
    const { onDraftChange, user } = renderPanel();

    await user.click(screen.getByRole('button', { name: /Start from scratch/ }));
    await user.type(screen.getByLabelText(/Endpoint URL/i), 'https://backend.example.com/graphql');

    // The endpoint is reported at once; the schema joins it when the
    // background check comes back.
    await waitFor(() =>
      expect(onDraftChange.mock.calls.at(-1)?.[0]).toMatchObject({
        schemaSource: 'introspection',
        endpointUrl: 'https://backend.example.com/graphql',
        sdl: SAMPLE_SDL,
      }),
    );
    const lastCall = onDraftChange.mock.calls.at(-1)?.[0];
    expect(lastCall).not.toHaveProperty('sdlUrl');
    expect(lastCall).not.toHaveProperty('sdlFile');
  });

  it('lets "Start from scratch" continue with just the endpoint when introspection is disabled', async () => {
    server.use(
      accepts('post', '/graphql-apis/validate-schema', {
        message: 'introspection is disabled on this endpoint',
        resolved: false,
      }),
    );
    const { onDraftChange, user } = renderPanel();

    await user.click(screen.getByRole('button', { name: /Start from scratch/ }));
    await user.type(screen.getByLabelText(/Endpoint URL/i), 'https://backend.example.com/graphql');

    // The status line under the field explains it; the explorer keeps its
    // initial empty state rather than showing an error for a usable endpoint …
    expect(await screen.findByText(/introspection may be disabled/)).toBeInTheDocument();
    expect(screen.getByText('Schema will show here')).toBeInTheDocument();
    expect(screen.queryByText('introspection is disabled on this endpoint')).not.toBeInTheDocument();
    // … and the wizard gets a draft, so Continue is enabled.
    const lastCall = onDraftChange.mock.calls.at(-1)?.[0];
    expect(lastCall).toMatchObject({
      endpointUrl: 'https://backend.example.com/graphql',
      schemaSource: 'introspection',
    });
    expect(lastCall).not.toHaveProperty('sdl');
    expect(screen.queryByText(/Fetched by introspection/)).not.toBeInTheDocument();
  });

  it('reports null once a resolved schema is cleared by switching approach', async () => {
    server.use(accepts('post', '/graphql-apis/validate-schema', { resolved: true, sdl: SAMPLE_SDL }));
    const { onDraftChange, user } = renderPanel();

    await user.type(screen.getByLabelText(/Schema URL/), 'https://raw.example.com/schema.graphql');
    await user.tab();
    await screen.findByText('Queries');

    await user.click(screen.getByRole('button', { name: /Start from scratch/ }));

    expect(onDraftChange).toHaveBeenLastCalledWith(null);
  });

  it('keeps the Explorer/SDL toggle usable both ways after fetching via URL upload', async () => {
    server.use(accepts('post', '/graphql-apis/validate-schema', { resolved: true, sdl: SAMPLE_SDL }));
    const { user } = renderPanel();

    await user.type(screen.getByLabelText(/Schema URL/), 'https://raw.example.com/schema.graphql');
    await user.tab();
    await screen.findByText('Queries');

    await user.click(screen.getByRole('button', { name: 'SDL' }));
    expect(screen.getByRole('button', { name: 'Explorer' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Explorer' }));
    expect(screen.getByRole('button', { name: 'SDL' })).toBeInTheDocument();
  });
});

/**
 * GraphQL SDL/introspection carries no `info.title` the way an OpenAPI
 * document does, so there's nothing structured to read a name from — this is
 * a best-effort guess at a starting name (file name, or the endpoint/SDL
 * URL's hostname) rather than anything the schema itself asserts. It's always
 * just a suggestion: the configure step's existing `identifierEdited` guard
 * (same one REST's spec-derived name already relies on) lets the user
 * override it, and the identifier it derives, by hand.
 */
describe('GraphqlDefinePanel — display name suggestion', () => {
  it('suggests a name from the URL’s hostname when a schema is fetched by URL', async () => {
    server.use(accepts('post', '/graphql-apis/validate-schema', { resolved: true, sdl: SAMPLE_SDL }));
    const { onDraftChange, user } = renderPanel();

    await user.type(screen.getByLabelText(/Schema URL/), 'https://raw.example.com/schema.graphql');
    await user.tab();
    await screen.findByText('Queries');

    expect(onDraftChange).toHaveBeenLastCalledWith(expect.objectContaining({ displayName: 'Raw' }));
  });

  it('suggests a name from the file name when a schema is fetched by upload', async () => {
    server.use(accepts('post', '/graphql-apis/validate-schema', { resolved: true, sdl: SAMPLE_SDL }));
    const { container, onDraftChange, user } = renderPanel();

    await user.click(screen.getByRole('button', { name: 'Upload' }));
    const file = new File([SAMPLE_SDL], 'countries-schema.graphql');
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    await user.upload(fileInput, file);
    await screen.findByText('Queries');

    expect(onDraftChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ displayName: 'Countries Schema' }),
    );
  });

  it('suggests a name from the endpoint’s hostname for "Start from scratch"', async () => {
    server.use(accepts('post', '/graphql-apis/validate-schema', { resolved: true, sdl: SAMPLE_SDL }));
    const { onDraftChange, user } = renderPanel();

    await user.click(screen.getByRole('button', { name: /Start from scratch/ }));
    await user.type(screen.getByLabelText(/Endpoint URL/i), 'https://backend.example.com/graphql');
    await screen.findByText('Queries');

    expect(onDraftChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ displayName: 'Backend' }),
    );
  });
});

/**
 * The schema-import side reports a rejected SDL to the explorer — unlike
 * "Start from scratch", where an endpoint without introspection is still a
 * usable source, an SDL that doesn't parse is something the user must fix.
 */
describe('GraphqlDefinePanel — a schema that fails validation', () => {
  it('shows each SDL error with its line and column, and hands the wizard no draft', async () => {
    server.use(
      accepts('post', '/graphql-apis/validate-schema', {
        message: 'The supplied SDL could not be parsed.',
        resolved: false,
        sdlErrors: [{ column: 12, line: 3, message: 'Unexpected Name "this"' }],
      }),
    );
    const { onDraftChange, user } = renderPanel();

    await user.type(screen.getByLabelText(/Schema URL/), 'https://raw.example.com/broken.graphql');
    await user.tab();

    expect(await screen.findByText(/Line 3, column 12/)).toBeInTheDocument();
    expect(screen.getByText(/Unexpected Name "this"/)).toBeInTheDocument();
    expect(screen.queryByText('Schema will show here')).not.toBeInTheDocument();
    // No draft, so the wizard's Continue stays disabled until it's fixed.
    expect(onDraftChange).toHaveBeenLastCalledWith(null);
  });

  it('shows the server’s reason when a schema URL cannot be fetched', async () => {
    server.use(
      accepts('post', '/graphql-apis/validate-schema', {
        message: 'The schema URL could not be fetched.',
        resolved: false,
      }),
    );
    const { onDraftChange, user } = renderPanel();

    await user.type(screen.getByLabelText(/Schema URL/), 'https://raw.example.com/missing.graphql');
    await user.tab();

    expect(await screen.findByText('The schema URL could not be fetched.')).toBeInTheDocument();
    expect(onDraftChange).toHaveBeenLastCalledWith(null);
  });
});
