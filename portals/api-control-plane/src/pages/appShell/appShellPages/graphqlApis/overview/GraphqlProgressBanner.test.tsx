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
    </Routes>,
    { route: BASE },
  );
}

describe('GraphqlProgressBanner — before anything is deployed', () => {
  it('marks Create complete and Deploy as the active next step', () => {
    renderBanner(false);

    expect(screen.getByRole('button', { name: 'Create' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Deploy' })).toBeEnabled();
    // Test/Publish have no honest "done" signal for a GraphQL API, so they
    // stay disabled next-next steps rather than clickable pills before deploy.
    expect(screen.getByRole('button', { name: 'Test' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Publish to Devportal' })).toBeDisabled();
    expect(screen.getByText('1 of 4 completed')).toBeInTheDocument();
    // "Next: " and the step name land in separate nodes (the step name is
    // wrapped in its own bolded span) — matched via a substring text function
    // rather than an exact string, and via a prefix (not "Deploy" alone,
    // which would also match the stepper pill's own label).
    expect(screen.getByText((_, element) => element?.textContent === 'Next: Deploy')).toBeInTheDocument();
  });

  it('navigates to the Deploy page when the Deploy pill is clicked', async () => {
    const { user } = renderBanner(false);

    await user.click(screen.getByRole('button', { name: 'Deploy' }));

    expect(await screen.findByText('DEPLOY PAGE')).toBeInTheDocument();
  });
});

describe('GraphqlProgressBanner — once deployed', () => {
  it('marks Deploy complete, offers Test as a next action, and keeps Publish disabled', () => {
    renderBanner(true);

    expect(screen.getByRole('button', { name: 'Deploy' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Test' })).toBeEnabled();
    // Publish leads to a `ComingSoon` stub with nothing behind it yet, so —
    // unlike Test — it never becomes a real next action, deployed or not.
    expect(screen.getByRole('button', { name: 'Publish to Devportal' })).toBeDisabled();
    // Caps at "2 of 4" and stays there: Test/Publish never mark complete, so
    // there is no further real signal to advance the counter with.
    expect(screen.getByText('2 of 4 completed')).toBeInTheDocument();
    expect(screen.getByText((_, element) => element?.textContent === 'Next: Test')).toBeInTheDocument();
  });

  it('navigates to the Test console when the Test pill is clicked', async () => {
    const { user } = renderBanner(true);

    await user.click(screen.getByRole('button', { name: 'Test' }));

    expect(await screen.findByText('TEST CONSOLE PAGE')).toBeInTheDocument();
  });
});
