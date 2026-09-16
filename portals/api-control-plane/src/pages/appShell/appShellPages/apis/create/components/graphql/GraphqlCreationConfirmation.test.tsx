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

import { describe, expect, it, vi } from 'vitest';

import { aGraphQLApi } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { renderWithProviders, screen } from '@/test/utils';
import { GraphqlCreationConfirmation } from './GraphqlCreationConfirmation';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';

const api = aGraphQLApi({
  context: '/countries-api/v1.0.0',
  displayName: 'Countries API',
  id: 'countries-api',
  upstream: { main: { url: 'https://backend.example.com/graphql' } },
});

const scope = makeConsoleScope({ params: { orgHandle: ORG, projectHandler: PROJECT } });

describe('GraphqlCreationConfirmation', () => {
  it('announces the created API by name and its context/endpoint', () => {
    renderWithProviders(<GraphqlCreationConfirmation api={api} />, { scope });

    expect(screen.getByText('Countries API created')).toBeInTheDocument();
    expect(screen.getByText('/countries-api/v1.0.0')).toBeInTheDocument();
    expect(screen.getByText('https://backend.example.com/graphql')).toBeInTheDocument();
    expect(screen.getByText('GraphQL API')).toBeInTheDocument();
  });

  it('links "Go to API" to the new API’s own overview page, not the general APIs list', () => {
    renderWithProviders(<GraphqlCreationConfirmation api={api} />, { scope });

    expect(screen.getByRole('link', { name: 'Go to API' })).toHaveAttribute(
      'href',
      `/organizations/${ORG}/projects/${PROJECT}/graphql-apis/countries-api`,
    );
  });

  it('copies the context to the clipboard', async () => {
    const { user } = renderWithProviders(<GraphqlCreationConfirmation api={api} />, { scope });
    // `userEvent.setup()` (inside `renderWithProviders`) installs its own
    // jsdom clipboard stub — spy on it rather than replacing
    // `navigator.clipboard` outright, which would just be clobbered by that
    // same setup call.
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);

    await user.click(screen.getByRole('button', { name: 'Copy Context' }));

    expect(writeText).toHaveBeenCalledWith('/countries-api/v1.0.0');
    expect(await screen.findByText('Copied to clipboard.')).toBeInTheDocument();
  });
});
