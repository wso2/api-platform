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

import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import {
  accepts,
  aSubscriptionPlan,
  collection,
  noContent,
  recorder,
  type Recorder,
  type SubscriptionPlanFixture,
} from '@/test/msw';
import { server } from '@/test/server';
import { makeConsoleScope } from '@/test/mockScope';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { SubscriptionPlansSettingsPage } from './SubscriptionPlansSettingsPage';

const ORG = 'api-platform-demo';
const PLANS = '/subscription-plans';

const plans: SubscriptionPlanFixture[] = [
  aSubscriptionPlan({
    displayName: 'Bronze',
    id: 'bronze',
    limits: [
      {
        limitCount: 1000,
        limitType: 'REQUEST_COUNT',
        stopOnQuotaReach: true,
        timeAmount: 1,
        timeUnit: 'HOUR',
      },
    ],
  }),
  aSubscriptionPlan({ displayName: 'Gold', id: 'gold', status: 'INACTIVE' }),
];

let requests: Recorder;

/**
 * The page's hooks read `ApiScopeContext`, not the console scope, so the
 * provider has to be mounted here — without it the query stays
 * `enabled: false` and the page renders its loading state forever.
 */
function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <SubscriptionPlansSettingsPage />
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/settings/subscription-plans`,
      scope: makeConsoleScope(),
    },
  );
}

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('SubscriptionPlansSettingsPage', () => {
  it('lists plans with their limit chips', async () => {
    server.use(collection(PLANS, plans, { record: requests }));
    renderPage();

    expect(await screen.findByText('Bronze')).toBeInTheDocument();
    expect(screen.getByText('Gold')).toBeInTheDocument();
    expect(screen.getByText('req · 1000 / hr')).toBeInTheDocument();
    // Gold has no limits configured — the unlimited chip, not an empty cell.
    expect(screen.getByText('req · ∞')).toBeInTheDocument();
  });

  it('filters the loaded list by name without asking the server again', async () => {
    server.use(collection(PLANS, plans, { record: requests }));
    const { user } = renderPage();

    await screen.findByText('Bronze');
    const requestsBefore = requests.count();
    await user.type(screen.getByPlaceholderText('Search plans'), 'gold');

    expect(screen.getByText('Gold')).toBeInTheDocument();
    expect(screen.queryByText('Bronze')).not.toBeInTheDocument();
    expect(requests.count()).toBe(requestsBefore);
  });

  it('shows the create prompt when the organization has no plans', async () => {
    server.use(collection(PLANS, []));
    renderPage();

    expect(await screen.findByText('Create your first subscription plan')).toBeInTheDocument();
    // The prompt is the whole page: no search over a collection that doesn't exist.
    expect(screen.queryByPlaceholderText('Search plans')).not.toBeInTheDocument();
  });

  it('shows "No matching plans" for a search with no hits, not the create prompt', async () => {
    server.use(collection(PLANS, plans));
    const { user } = renderPage();

    await screen.findByText('Bronze');
    await user.type(screen.getByPlaceholderText('Search plans'), 'platinum');

    expect(await screen.findByText('No matching plans')).toBeInTheDocument();
    expect(screen.queryByText('Create your first subscription plan')).not.toBeInTheDocument();
  });

  it("toggles a plan's status from the row switch, without resending its limits", async () => {
    server.use(
      collection(PLANS, plans, { record: requests }),
      accepts(
        'put',
        `${PLANS}/bronze`,
        aSubscriptionPlan({ displayName: 'Bronze', id: 'bronze', status: 'INACTIVE' }),
        { record: requests },
      ),
    );
    const { user } = renderPage();

    await screen.findByText('Bronze');
    await user.click(screen.getByRole('checkbox', { name: 'Bronze, active' }));

    // Not `.last()`: the mutation's own success invalidation refetches the
    // list, and that GET can land after the PUT.
    await waitFor(() => expect(requests.calls.some((call) => call.method === 'PUT')).toBe(true));
    const putCall = requests.calls.find((call) => call.method === 'PUT');
    // Bronze has a limit configured; a stale cached copy of it must never be
    // resent, since the server would persist it verbatim, undoing whatever
    // the limit's actual current value is.
    expect(JSON.parse(putCall!.body)).toEqual({
      displayName: 'Bronze',
      id: 'bronze',
      status: 'INACTIVE',
    });
  });

  it('opens the edit dialog only from the pencil icon, not the row itself', async () => {
    server.use(collection(PLANS, plans, { record: requests }));
    const { user } = renderPage();

    await screen.findByText('Bronze');
    await user.click(screen.getByText('Bronze'));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Edit Bronze' }));

    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText('Edit subscription plan')).toBeInTheDocument();
  });

  it('deletes a plan once its name is typed to confirm', async () => {
    server.use(
      collection(PLANS, plans, { record: requests }),
      noContent('delete', `${PLANS}/bronze`, { record: requests }),
    );
    const { user } = renderPage();

    await screen.findByText('Bronze');
    await user.click(screen.getByRole('button', { name: 'Delete Bronze' }));

    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByRole('button', { name: 'Delete' })).toBeDisabled();

    await user.type(within(dialog).getByRole('textbox'), 'Bronze');
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    // Not `.last()`: a successful delete also invalidates the list, and that
    // refetch's GET can land after the DELETE.
    await waitFor(() => expect(requests.calls.some((call) => call.method === 'DELETE')).toBe(true));
    // The dialog closes from the mutation's own onSuccess, a tick after the
    // request itself resolves — not necessarily settled by the line above.
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });
});
