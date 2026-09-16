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
import { renderWithProviders, screen } from '@/test/utils';
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
    await user.click(screen.getByRole('button', { name: 'Fetch Schema' }));

    await screen.findByText('Query', { exact: false });

    const lastCall = onDraftChange.mock.calls.at(-1)?.[0];
    expect(lastCall).toMatchObject({
      schemaSource: 'url',
      sdl: SAMPLE_SDL,
      sdlUrl: 'https://raw.example.com/schema.graphql',
    });
    expect(lastCall).not.toHaveProperty('endpointUrl');
    expect(lastCall).not.toHaveProperty('sdlFile');
  });

  it('carries `endpointUrl` (and no `sdlUrl`) when the "Design from scratch" endpoint resolves', async () => {
    server.use(
      accepts(
        'post',
        '/graphql-apis/validate-schema',
        { resolved: true, sdl: SAMPLE_SDL },
        { record: requests },
      ),
    );
    const { onDraftChange, user } = renderPanel();

    await user.click(screen.getByRole('button', { name: /Design from scratch/ }));
    await user.type(screen.getByLabelText(/Backend endpoint/i), 'https://backend.example.com/graphql');
    await user.click(screen.getByRole('button', { name: 'Check' }));

    await screen.findByText('Query', { exact: false });

    const lastCall = onDraftChange.mock.calls.at(-1)?.[0];
    expect(lastCall).toMatchObject({
      schemaSource: 'introspection',
      endpointUrl: 'https://backend.example.com/graphql',
      sdl: SAMPLE_SDL,
    });
    expect(lastCall).not.toHaveProperty('sdlUrl');
    expect(lastCall).not.toHaveProperty('sdlFile');
  });

  it('reports null once a resolved schema is cleared by switching approach', async () => {
    server.use(accepts('post', '/graphql-apis/validate-schema', { resolved: true, sdl: SAMPLE_SDL }));
    const { onDraftChange, user } = renderPanel();

    await user.type(screen.getByLabelText(/Schema URL/), 'https://raw.example.com/schema.graphql');
    await user.click(screen.getByRole('button', { name: 'Fetch Schema' }));
    await screen.findByText('Query', { exact: false });

    await user.click(screen.getByRole('button', { name: /Design from scratch/ }));

    expect(onDraftChange).toHaveBeenLastCalledWith(null);
  });

  it('keeps the Explorer/SDL toggle usable both ways after fetching via URL upload', async () => {
    server.use(accepts('post', '/graphql-apis/validate-schema', { resolved: true, sdl: SAMPLE_SDL }));
    const { user } = renderPanel();

    await user.type(screen.getByLabelText(/Schema URL/), 'https://raw.example.com/schema.graphql');
    await user.click(screen.getByRole('button', { name: 'Fetch Schema' }));
    await screen.findByText('Query', { exact: false });

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
    await user.click(screen.getByRole('button', { name: 'Fetch Schema' }));
    await screen.findByText('Query', { exact: false });

    expect(onDraftChange).toHaveBeenLastCalledWith(expect.objectContaining({ displayName: 'Raw' }));
  });

  it('suggests a name from the file name when a schema is fetched by upload', async () => {
    server.use(accepts('post', '/graphql-apis/validate-schema', { resolved: true, sdl: SAMPLE_SDL }));
    const { container, onDraftChange, user } = renderPanel();

    await user.click(screen.getByRole('button', { name: 'Upload' }));
    const file = new File([SAMPLE_SDL], 'countries-schema.graphql');
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    await user.upload(fileInput, file);
    await user.click(screen.getByRole('button', { name: 'Fetch Schema' }));
    await screen.findByText('Query', { exact: false });

    expect(onDraftChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ displayName: 'Countries Schema' }),
    );
  });

  it('suggests a name from the endpoint’s hostname for "Design from scratch"', async () => {
    server.use(accepts('post', '/graphql-apis/validate-schema', { resolved: true, sdl: SAMPLE_SDL }));
    const { onDraftChange, user } = renderPanel();

    await user.click(screen.getByRole('button', { name: /Design from scratch/ }));
    await user.type(screen.getByLabelText(/Backend endpoint/i), 'https://backend.example.com/graphql');
    await user.click(screen.getByRole('button', { name: 'Check' }));
    await screen.findByText('Query', { exact: false });

    expect(onDraftChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ displayName: 'Backend' }),
    );
  });
});
