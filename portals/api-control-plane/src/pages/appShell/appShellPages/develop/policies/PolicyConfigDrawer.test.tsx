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

import { http, HttpResponse } from 'msw';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { runtimeConfig } from '@/config/runtime';
import { aCustomPolicy, collection } from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { PolicyConfigDrawer } from './PolicyConfigDrawer';

const policy = { displayName: 'Header Auth', name: 'header-auth', version: 'v1' };

const definition = [
  'name: header-auth',
  'version: v1',
  'description: Checks a header',
  'parameters:',
  '  type: object',
  '  required: [headerName]',
  '  properties:',
  '    headerName:',
  '      type: string',
  '      title: Header Name',
  '    caseSensitive:',
  '      type: boolean',
  '      x-wso2-policy-advanced-param: true',
].join('\n');

// The hub is a different origin from the page, so jsdom enforces CORS on the reply.
const cors = { 'Access-Control-Allow-Origin': '*' };

// The saved policy carries only its major ("v1"); the hub serves a definition by full
// version, so the drawer must resolve it to the newest published "v1.x" first.
const hub = `${runtimeConfig.policyHubBaseUrl}/policies/header-auth`;

const serveVersions = () =>
  server.use(
    http.get(`${hub}/versions`, () =>
      HttpResponse.json(
        [{ version: 'v1.2' }, { version: 'v1.10' }, { version: 'v2.0' }].map((entry) => ({
          name: 'header-auth',
          ...entry,
        })),
        { headers: cors },
      ),
    ),
  );

const serveDefinition = (body: string) => {
  serveVersions();
  server.use(
    http.get(`${hub}/versions/v1.10/definition`, () => HttpResponse.text(body, { headers: cors })),
  );
};

beforeEach(() => {
  resetHttpClient();
  // jsdom's AbortSignal is not the one undici's fetch accepts, so the signal React
  // Query hands the hub request is dropped here; MSW still answers the request.
  const mswFetch = globalThis.fetch;
  vi.spyOn(globalThis, 'fetch').mockImplementation((input, init) =>
    mswFetch(input, { ...init, signal: undefined }),
  );
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('PolicyConfigDrawer', () => {
  it('holds the attach button until the required parameter is filled, then confirms the values', async () => {
    serveDefinition(definition);
    const onConfirm = vi.fn();
    const { user } = renderWithProviders(
      <PolicyConfigDrawer onClose={vi.fn()} onConfirm={onConfirm} open policy={policy} />,
    );

    const attach = await screen.findByTestId('policy-param-submit');
    expect(attach).toHaveTextContent('Attach policy');
    expect(attach).toBeDisabled();

    await user.type(screen.getByTestId('field-headerName'), 'X-Api-Key');
    expect(attach).toBeEnabled();
    await user.click(attach);

    expect(onConfirm).toHaveBeenCalledWith({
      name: 'header-auth',
      params: expect.objectContaining({ headerName: 'X-Api-Key' }),
      version: 'v1',
    });
  });

  it('fetches the definition of the newest version of the saved major', async () => {
    const requested: string[] = [];
    serveVersions();
    server.use(
      http.get(`${hub}/versions/:version/definition`, ({ params }) => {
        requested.push(String(params.version));
        return HttpResponse.text(definition, { headers: cors });
      }),
    );
    renderWithProviders(
      <PolicyConfigDrawer onClose={vi.fn()} onConfirm={vi.fn()} open policy={policy} />,
    );

    expect(await screen.findByTestId('policy-param-submit')).toBeInTheDocument();
    expect(requested).toEqual(['v1.10']);
  });

  it('pre-fills an attached policy and offers Save policy when editing', async () => {
    serveDefinition(definition);
    renderWithProviders(
      <PolicyConfigDrawer
        initialValues={{ headerName: 'X-Existing' }}
        mode="edit"
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        open
        policy={policy}
      />,
    );

    expect(await screen.findByTestId('field-headerName')).toHaveValue('X-Existing');
    expect(screen.getByTestId('policy-param-submit')).toHaveTextContent('Save policy');
  });

  it('says so when the policy has no parameters', async () => {
    serveDefinition(['name: header-auth', 'version: v1'].join('\n'));
    renderWithProviders(
      <PolicyConfigDrawer onClose={vi.fn()} onConfirm={vi.fn()} open policy={policy} />,
    );

    await waitFor(() =>
      expect(screen.getByText('This policy has no configurable parameters.')).toBeInTheDocument(),
    );
    expect(screen.queryByTestId('policy-param-submit')).not.toBeInTheDocument();
  });

  it('shows an error when the definition cannot be loaded', async () => {
    serveVersions();
    server.use(
      http.get(
        `${hub}/versions/v1.10/definition`,
        () => new HttpResponse(null, { headers: cors, status: 500 }),
      ),
    );
    renderWithProviders(
      <PolicyConfigDrawer onClose={vi.fn()} onConfirm={vi.fn()} open policy={policy} />,
    );

    expect(await screen.findByText('Unable to load the policy definition.')).toBeInTheDocument();
  });
});

describe('PolicyConfigDrawer — custom policies', () => {
  const hmacSigner = aCustomPolicy({
    name: 'hmac-signer',
    uuid: 'hmac-signer-uuid',
    version: 'v1.2.0',
    policyDefinition: {
      parameters: {
        type: 'object',
        required: ['secretKey'],
        properties: { secretKey: { type: 'string', title: 'Secret Key' } },
      },
    },
  });

  const renderInOrg = (ref: typeof policy, onConfirm = vi.fn()) =>
    renderWithProviders(
      <ApiScopeProvider orgId="acme-org">
        <PolicyConfigDrawer onClose={vi.fn()} onConfirm={onConfirm} open policy={ref} />
      </ApiScopeProvider>,
    );

  it('builds the form from the organization’s custom policy of the same major', async () => {
    const hubDefinitions: string[] = [];
    server.use(
      collection('/gateway-custom-policies', [hmacSigner]),
      http.get(`${runtimeConfig.policyHubBaseUrl}/policies/hmac-signer/versions`, () =>
        HttpResponse.json([], { headers: cors }),
      ),
      http.get(
        `${runtimeConfig.policyHubBaseUrl}/policies/hmac-signer/versions/:version/definition`,
        ({ params }) => {
          hubDefinitions.push(String(params.version));
          return new HttpResponse(null, { headers: cors, status: 404 });
        },
      ),
    );
    const onConfirm = vi.fn();
    const { user } = renderInOrg(
      { displayName: 'hmac-signer', name: 'hmac-signer', version: 'v1' },
      onConfirm,
    );

    await user.type(await screen.findByTestId('field-secretKey'), 'shh');
    expect(screen.getByText('v1.2.0')).toBeInTheDocument();
    await user.click(screen.getByTestId('policy-param-submit'));

    expect(onConfirm).toHaveBeenCalledWith({
      name: 'hmac-signer',
      params: expect.objectContaining({ secretKey: 'shh' }),
      version: 'v1',
    });
    expect(hubDefinitions).toEqual([]);
  });

  it('reads the Policy Hub when the organization holds another major only', async () => {
    serveDefinition(definition);
    server.use(
      collection('/gateway-custom-policies', [
        aCustomPolicy({ name: 'header-auth', uuid: 'header-auth-uuid', version: 'v2.0.0' }),
      ]),
    );
    renderInOrg(policy);

    expect(await screen.findByTestId('field-headerName')).toBeInTheDocument();
  });
});
