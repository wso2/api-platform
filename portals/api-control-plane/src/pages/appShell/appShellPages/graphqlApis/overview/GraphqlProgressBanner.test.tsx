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

import { Route, Routes } from 'react-router-dom';
import { describe, expect, it } from 'vitest';

import { renderWithProviders, screen } from '@/test/utils';
import { GraphqlProgressBanner } from './GraphqlProgressBanner';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'countries-graphql-api';
const BASE = `/organizations/${ORG}/projects/${PROJECT}/graphql-apis/${API}`;

/** Reads `graphqlApiHandler` etc. off `useParams()`, so the route has to actually match. */
function renderBanner(deployed: boolean) {
  return renderWithProviders(
    <Routes>
      <Route
        element={<GraphqlProgressBanner deployed={deployed} />}
        path="/organizations/:orgHandle/projects/:projectHandler/graphql-apis/:graphqlApiHandler"
      />
      <Route element={<div>DEPLOY PAGE</div>} path={`${BASE}/deploy`} />
      <Route element={<div>TEST CONSOLE PAGE</div>} path={`${BASE}/test/console`} />
      <Route element={<div>PUBLISH PAGE</div>} path={`${BASE}/publish`} />
    </Routes>,
    { route: BASE },
  );
}

describe('GraphqlProgressBanner — before anything is deployed', () => {
  it('marks Create complete and Deploy as the active next step', () => {
    renderBanner(false);

    expect(screen.getByRole('button', { name: 'Create' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Deploy' })).toBeEnabled();
    // Test needs a deployed endpoint; Publish, as on REST's stepper, is
    // always reachable — its Portals list is where publishing starts.
    expect(screen.getByRole('button', { name: 'Test' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Publish' })).toBeEnabled();
  });

  it('navigates to the Deploy page when the Deploy pill is clicked', async () => {
    const { user } = renderBanner(false);

    await user.click(screen.getByRole('button', { name: 'Deploy' }));

    expect(await screen.findByText('DEPLOY PAGE')).toBeInTheDocument();
  });
});

describe('GraphqlProgressBanner — once deployed', () => {
  it('marks Deploy complete and offers Test and Publish as next actions', () => {
    renderBanner(true);

    expect(screen.getByRole('button', { name: 'Deploy' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Test' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Publish' })).toBeEnabled();
  });

  it('navigates to the Test console when the Test pill is clicked', async () => {
    const { user } = renderBanner(true);

    await user.click(screen.getByRole('button', { name: 'Test' }));

    expect(await screen.findByText('TEST CONSOLE PAGE')).toBeInTheDocument();
  });
});

describe('GraphqlProgressBanner — publishing', () => {
  it('opens the API’s Portals list when the Publish pill is clicked', async () => {
    const { user } = renderBanner(true);

    await user.click(screen.getByRole('button', { name: 'Publish' }));

    expect(await screen.findByText('PUBLISH PAGE')).toBeInTheDocument();
  });
});
