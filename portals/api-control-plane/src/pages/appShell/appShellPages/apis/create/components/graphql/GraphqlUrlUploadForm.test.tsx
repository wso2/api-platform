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
import { GraphqlUrlUploadForm } from './GraphqlUrlUploadForm';

const ORG = 'api-platform-demo';

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

const renderForm = (onResolved = vi.fn()) => {
  const rendered = renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <GraphqlUrlUploadForm onResolved={onResolved} />
    </ApiScopeProvider>,
  );
  return { onResolved, ...rendered };
};

describe('GraphqlUrlUploadForm — URL tab', () => {
  it('requires a URL before submitting', async () => {
    const { onResolved, user } = renderForm();

    await user.click(screen.getByRole('button', { name: 'Fetch Schema' }));

    expect(await screen.findByText('Enter the URL of the SDL file.')).toBeInTheDocument();
    expect(onResolved).not.toHaveBeenCalled();
  });

  it('rejects a non-URL value once the field is touched', async () => {
    const { user } = renderForm();

    await user.type(screen.getByLabelText(/Schema URL/), 'not-a-url');
    await user.tab();

    expect(await screen.findByText('Enter a valid HTTP or HTTPS URL.')).toBeInTheDocument();
  });

  it('resolves a valid URL and reports the resolved schema', async () => {
    server.use(
      accepts(
        'post',
        '/graphql-apis/validate-schema',
        { resolved: true, sdl: 'type Query { hello: String }' },
        { record: requests },
      ),
    );
    const { onResolved, user } = renderForm();

    await user.type(screen.getByLabelText(/Schema URL/), 'https://raw.example.com/schema.graphql');
    await user.click(screen.getByRole('button', { name: 'Fetch Schema' }));

    await waitFor(() =>
      expect(onResolved).toHaveBeenCalledWith({
        schemaSource: 'url',
        sdl: 'type Query { hello: String }',
        sdlUrl: 'https://raw.example.com/schema.graphql',
      }),
    );
    expect(requests.count()).toBe(1);
  });

  it('disables Fetch Schema once resolved, and re-enables it once the URL is edited', async () => {
    server.use(
      accepts('post', '/graphql-apis/validate-schema', {
        resolved: true,
        sdl: 'type Query { hello: String }',
      }),
    );
    const { onResolved, user } = renderForm();

    await user.type(screen.getByLabelText(/Schema URL/), 'https://raw.example.com/schema.graphql');
    await user.click(screen.getByRole('button', { name: 'Fetch Schema' }));

    await waitFor(() => expect(onResolved).toHaveBeenCalled());
    expect(screen.getByRole('button', { name: 'Fetch Schema' })).toBeDisabled();

    await user.type(screen.getByLabelText(/Schema URL/), '2');

    expect(screen.getByRole('button', { name: 'Fetch Schema' })).toBeEnabled();
  });

  it('shows the unresolved message and reports null when the server could not resolve the schema', async () => {
    server.use(accepts('post', '/graphql-apis/validate-schema', { resolved: false }));
    const { onResolved, user } = renderForm();

    await user.type(screen.getByLabelText(/Schema URL/), 'https://raw.example.com/schema.graphql');
    await user.click(screen.getByRole('button', { name: 'Fetch Schema' }));

    expect(
      await screen.findByText('That schema could not be resolved. Check it is valid GraphQL SDL.'),
    ).toBeInTheDocument();
    expect(onResolved).toHaveBeenLastCalledWith(null);
  });

  it('reports null and shows nothing extra when the request itself fails', async () => {
    server.use(
      accepts('post', '/graphql-apis/validate-schema', { status: 'error' }, { status: 500 }),
    );
    const { onResolved, user } = renderForm();

    await user.type(screen.getByLabelText(/Schema URL/), 'https://raw.example.com/schema.graphql');
    await user.click(screen.getByRole('button', { name: 'Fetch Schema' }));

    await waitFor(() => expect(onResolved).toHaveBeenLastCalledWith(null));
    expect(
      screen.queryByText('That schema could not be resolved. Check it is valid GraphQL SDL.'),
    ).not.toBeInTheDocument();
  });

  it('fills the URL field from the sample-endpoint link and resets any prior resolution', async () => {
    server.use(
      accepts('post', '/graphql-apis/validate-schema', {
        resolved: true,
        sdl: 'type Query { a: String }',
      }),
    );
    const { onResolved, user } = renderForm();

    await user.type(screen.getByLabelText(/Schema URL/), 'https://raw.example.com/schema.graphql');
    await user.click(screen.getByRole('button', { name: 'Fetch Schema' }));
    await waitFor(() =>
      expect(onResolved).toHaveBeenLastCalledWith({
        schemaSource: 'url',
        sdl: 'type Query { a: String }',
        sdlUrl: 'https://raw.example.com/schema.graphql',
      }),
    );

    await user.click(screen.getByRole('button', { name: 'Try with Sample Endpoint' }));

    // Filling the sample URL resets the prior resolution rather than leaving
    // a stale "resolved" result attached to a URL the user never fetched.
    expect(onResolved).toHaveBeenLastCalledWith(null);
    expect(screen.getByLabelText(/Schema URL/)).toHaveValue(
      'https://raw.githubusercontent.com/graphql/swapi-graphql/master/schema.graphql',
    );
  });
});

describe('GraphqlUrlUploadForm — Upload tab', () => {
  /** Unambiguous only before switching: the dropzone's own "Upload" button doesn't exist yet. */
  const switchToUploadTab = async (user: ReturnType<typeof renderForm>['user']) => {
    await user.click(screen.getByRole('button', { name: 'Upload' }));
  };

  it('requires a file before submitting', async () => {
    const { user } = renderForm();

    // Switching tabs itself reports null (clearing whatever the other tab had
    // resolved) — the assertion that matters here is that submitting with no
    // file never reaches the network, not that onResolved was never called.
    await switchToUploadTab(user);
    await user.click(screen.getByRole('button', { name: 'Fetch Schema' }));

    expect(await screen.findByText('Select a schema file to continue.')).toBeInTheDocument();
    expect(requests.count()).toBe(0);
  });

  it('resolves an uploaded file and reports the resolved schema', async () => {
    server.use(
      accepts(
        'post',
        '/graphql-apis/validate-schema',
        { resolved: true, sdl: 'type Query { hello: String }' },
        { record: requests },
      ),
    );
    const { container, onResolved, user } = renderForm();

    await switchToUploadTab(user);

    const file = new File(['type Query { hello: String }'], 'schema.graphql', {
      type: 'application/octet-stream',
    });
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    await user.upload(fileInput, file);

    // Selecting a file shows its name and swaps the picker for a remove button.
    expect(screen.getByText('schema.graphql')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Fetch Schema' }));

    await waitFor(() =>
      expect(onResolved).toHaveBeenCalledWith({
        schemaSource: 'file',
        sdl: 'type Query { hello: String }',
        sdlFile: file,
      }),
    );
    expect(requests.count()).toBe(1);
  });

  it('lets the chosen file be removed and re-requires a file before submitting again', async () => {
    const { container, user } = renderForm();

    await switchToUploadTab(user);
    const file = new File(['type Query { hello: String }'], 'schema.graphql');
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    await user.upload(fileInput, file);

    await user.click(screen.getByRole('button', { name: 'Remove schema.graphql' }));

    expect(screen.queryByText('schema.graphql')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Fetch Schema' }));
    expect(await screen.findByText('Select a schema file to continue.')).toBeInTheDocument();
    expect(requests.count()).toBe(0);
  });
});
