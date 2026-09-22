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
import { fireEvent, renderWithProviders, screen, waitFor } from '@/test/utils';
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
  it('requires a URL once the field is left empty', async () => {
    const { onResolved, user } = renderForm();

    await user.click(screen.getByLabelText(/Schema URL/));
    await user.tab();

    expect(await screen.findByText('Enter the URL of the SDL file.')).toBeInTheDocument();
    expect(onResolved).not.toHaveBeenCalled();
    expect(requests.count()).toBe(0);
  });

  it('rejects a non-URL value once the field is touched', async () => {
    const { user } = renderForm();

    await user.type(screen.getByLabelText(/Schema URL/), 'not-a-url');
    await user.tab();

    expect(await screen.findByText('Enter a valid HTTP or HTTPS URL.')).toBeInTheDocument();
  });

  it('checks a valid URL on its own once the field is left, with no fetch button', async () => {
    server.use(
      accepts(
        'post',
        '/graphql-apis/validate-schema',
        { resolved: true, sdl: 'type Query { hello: String }' },
        { record: requests },
      ),
    );
    const { onResolved, user } = renderForm();

    expect(screen.queryByRole('button', { name: 'Fetch Schema' })).not.toBeInTheDocument();

    await user.type(screen.getByLabelText(/Schema URL/), 'https://raw.example.com/schema.graphql');
    await user.tab();

    await waitFor(() =>
      expect(onResolved).toHaveBeenCalledWith({
        schemaSource: 'url',
        sdl: 'type Query { hello: String }',
        sdlUrl: 'https://raw.example.com/schema.graphql',
      }),
    );
    expect(requests.count()).toBe(1);
  });

  it('does not re-check an unchanged URL on a second blur, but does after an edit', async () => {
    server.use(
      accepts(
        'post',
        '/graphql-apis/validate-schema',
        { resolved: true, sdl: 'type Query { hello: String }' },
        { record: requests },
      ),
    );
    const { onResolved, user } = renderForm();
    const field = screen.getByLabelText(/Schema URL/);

    await user.type(field, 'https://raw.example.com/schema.graphql');
    await user.tab();
    await waitFor(() => expect(requests.count()).toBe(1));

    // Leaving the field again with nothing edited must not re-check it.
    await user.click(field);
    await user.tab();
    expect(requests.count()).toBe(1);

    // Editing it invalidates the cached check, so leaving it again does re-check.
    await user.type(field, '2');
    await user.tab();
    await waitFor(() => expect(requests.count()).toBe(2));
    expect(onResolved).toHaveBeenLastCalledWith({
      schemaSource: 'url',
      sdl: 'type Query { hello: String }',
      sdlUrl: 'https://raw.example.com/schema.graphql2',
    });
  });

  it('shows the unresolved message and reports null when the server could not resolve the schema', async () => {
    server.use(accepts('post', '/graphql-apis/validate-schema', { resolved: false }));
    const { onResolved, user } = renderForm();

    await user.type(screen.getByLabelText(/Schema URL/), 'https://raw.example.com/schema.graphql');
    await user.tab();

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
    await user.tab();

    await waitFor(() => expect(onResolved).toHaveBeenLastCalledWith(null));
    expect(
      screen.queryByText('That schema could not be resolved. Check it is valid GraphQL SDL.'),
    ).not.toBeInTheDocument();
  });

  it('fills the URL field from the sample-endpoint link and checks it immediately', async () => {
    server.use(
      accepts(
        'post',
        '/graphql-apis/validate-schema',
        { resolved: true, sdl: 'type Query { a: String }' },
        { record: requests },
      ),
    );
    const { onResolved, user } = renderForm();

    await user.type(screen.getByLabelText(/Schema URL/), 'https://raw.example.com/schema.graphql');
    await user.tab();
    await waitFor(() =>
      expect(onResolved).toHaveBeenLastCalledWith({
        schemaSource: 'url',
        sdl: 'type Query { a: String }',
        sdlUrl: 'https://raw.example.com/schema.graphql',
      }),
    );

    await user.click(screen.getByRole('button', { name: 'Try with Sample Schema' }));

    expect(screen.getByLabelText(/Schema URL/)).toHaveValue(
      'https://raw.githubusercontent.com/graphql/swapi-graphql/master/schema.graphql',
    );
    // No second, stale request for the URL it replaced — the click resets and
    // checks the sample in one go rather than committing the old value first.
    await waitFor(() => expect(requests.count()).toBe(2));
  });
});

describe('GraphqlUrlUploadForm — Upload tab', () => {
  /** Unambiguous only before switching: the dropzone's own "Upload" button doesn't exist yet. */
  const switchToUploadTab = async (user: ReturnType<typeof renderForm>['user']) => {
    await user.click(screen.getByRole('button', { name: 'Upload' }));
  };

  it('has no fetch button and does nothing until a file is chosen', async () => {
    const { user } = renderForm();

    await switchToUploadTab(user);

    expect(screen.queryByRole('button', { name: 'Fetch Schema' })).not.toBeInTheDocument();
    expect(requests.count()).toBe(0);
  });

  it('checks a chosen file immediately, with no fetch button', async () => {
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

    await waitFor(() =>
      expect(onResolved).toHaveBeenCalledWith({
        schemaSource: 'file',
        sdl: 'type Query { hello: String }',
        sdlFile: file,
      }),
    );
    expect(requests.count()).toBe(1);
  });

  it('lets the chosen file be removed, clearing whatever it had resolved', async () => {
    server.use(
      accepts('post', '/graphql-apis/validate-schema', {
        resolved: true,
        sdl: 'type Query { hello: String }',
      }),
    );
    const { container, onResolved, user } = renderForm();

    await switchToUploadTab(user);
    const file = new File(['type Query { hello: String }'], 'schema.graphql');
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    await user.upload(fileInput, file);
    await waitFor(() =>
      expect(onResolved).toHaveBeenLastCalledWith({
        schemaSource: 'file',
        sdl: 'type Query { hello: String }',
        sdlFile: file,
      }),
    );

    await user.click(screen.getByRole('button', { name: 'Remove schema.graphql' }));

    expect(screen.queryByText('schema.graphql')).not.toBeInTheDocument();
    expect(onResolved).toHaveBeenLastCalledWith(null);
  });

  it('rejects a file extension the shared dropzone does not accept, without checking it', async () => {
    const { container, onResolved, user } = renderForm();

    await switchToUploadTab(user);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    // `user.upload` honours the input's `accept` attribute and would refuse to
    // hand the file over at all, so the change is fired directly — what a
    // real drop, or an OS picker that ignores the filter, would still send.
    fireEvent.change(fileInput, {
      target: { files: [new File(['not a schema'], 'notes.txt', { type: 'text/plain' })] },
    });

    expect(
      await screen.findByText(/That file type is not supported\. Accepted types:/),
    ).toBeInTheDocument();
    // Switching to the tab, and the rejection itself, both report null —
    // neither reaches the network, which is the behavior under test.
    expect(onResolved).toHaveBeenLastCalledWith(null);
    expect(requests.count()).toBe(0);
  });
});
