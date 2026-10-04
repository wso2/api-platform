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
import {
  accepts,
  aSubscriptionPlan,
  failure,
  recorder,
  type Recorder,
  type SubscriptionPlanFixture,
} from '@/test/msw';
import { server } from '@/test/server';
import { fireEvent, renderWithProviders, screen, userEvent, waitFor } from '@/test/utils';
import { SubscriptionPlanFormDialog } from './SubscriptionPlanFormDialog';

const ORG = 'api-platform-demo';
const PLANS = '/subscription-plans';

let requests: Recorder;

/** Both mutations resolve their organization from `ApiScopeContext`, so the
 * provider has to be mounted for a request to be allowed out at all. */
function setup(overrides: { plan?: SubscriptionPlanFixture | null } = {}) {
  const onClose = vi.fn();
  const utils = renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <SubscriptionPlanFormDialog onClose={onClose} open plan={overrides.plan ?? null} />
    </ApiScopeProvider>,
  );
  return {
    ...utils,
    onClose,
    // jsdom never fires the Dialog's fade-in `transitionend`, so its backdrop
    // can still report `pointer-events: none` after render; this is a known
    // false positive, not a real accessibility gap — see e.g.
    // testing-library/user-event#1147.
    user: userEvent.setup({ pointerEventsCheck: 0 }),
  };
}

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('SubscriptionPlanFormDialog', () => {
  it('creates a plan with a handle slugified from the name', async () => {
    server.use(accepts('post', PLANS, aSubscriptionPlan(), { record: requests }));
    const { user, onClose } = setup();

    await user.type(screen.getByLabelText(/Name/), 'Gold Plan');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toMatchObject({
      id: 'gold-plan',
      displayName: 'Gold Plan',
      status: 'ACTIVE',
    });
    expect(onClose).toHaveBeenCalled();
  });

  it('disables Add plan, staying quiet about the empty name until it is actually rejected', async () => {
    const { user } = setup();
    const submit = screen.getByRole('button', { name: 'Add plan' });
    const name = screen.getByLabelText(/Name/);

    expect(submit).toBeDisabled();
    expect(screen.queryByText('Enter a display name')).not.toBeInTheDocument();

    // The name field is autofocused, so moving on without ever typing (e.g.
    // the dialog's own mount/focus handling shifting focus) must not be read
    // as "the user rejected this field".
    await user.tab();
    expect(screen.queryByText('Enter a display name')).not.toBeInTheDocument();

    // Typing then clearing it is a rejection, and does get flagged.
    await user.type(name, 'Gold');
    await user.clear(name);
    await user.tab();

    expect(screen.getByText('Enter a display name')).toBeInTheDocument();
    expect(submit).toBeDisabled();
    expect(requests.count()).toBe(0);
  });

  it('rejects a name with no letters or numbers to slug', async () => {
    const { user } = setup();

    await user.type(screen.getByLabelText(/Name/), '!!!');
    await user.tab();

    expect(screen.getByText('Use letters or numbers in the name')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add plan' })).toBeDisabled();
    expect(requests.count()).toBe(0);
  });

  it('disables Add plan until a newly added limit has a valid count', async () => {
    const { user } = setup();

    await user.type(screen.getByLabelText(/Name/), 'Gold');
    await user.click(screen.getByRole('button', { name: 'Add limit' }));
    await user.type(screen.getByLabelText('Request count'), '0');
    await user.tab();

    expect(screen.getByText('Enter a whole number of at least 1.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add plan' })).toBeDisabled();
    expect(requests.count()).toBe(0);
  });

  it('caps limits at one: "Add limit" hides after a row is added, and returns once it is removed', async () => {
    const { user } = setup();

    await user.click(screen.getByRole('button', { name: 'Add limit' }));
    expect(screen.queryByRole('button', { name: 'Add limit' })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Remove limit' }));
    expect(screen.getByRole('button', { name: 'Add limit' })).toBeInTheDocument();
  });

  it('sends the expiry date as a full RFC3339 timestamp, not the bare date the input holds', async () => {
    server.use(accepts('post', PLANS, aSubscriptionPlan(), { record: requests }));
    const { user } = setup();

    await user.type(screen.getByLabelText(/Name/), 'Gold');
    // A native date input's value is always a bare `yyyy-mm-dd`; userEvent
    // can't type into it segment-by-segment reliably, so set it directly.
    fireEvent.change(screen.getByLabelText(/Expiry date/), { target: { value: '2026-12-31' } });
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body).expiryTime).toBe('2026-12-31T00:00:00Z');
  });

  it('opens pre-filled with an existing plan, and preserves its status on save', async () => {
    const plan = aSubscriptionPlan({
      displayName: 'Gold',
      id: 'gold',
      limits: [
        {
          limitCount: 1000,
          limitType: 'REQUEST_COUNT',
          stopOnQuotaReach: true,
          timeAmount: 1,
          timeUnit: 'HOUR',
        },
      ],
      status: 'INACTIVE',
    });
    server.use(accepts('put', `${PLANS}/gold`, plan, { record: requests }));
    const { user } = setup({ plan });

    expect(screen.getByLabelText(/Name/)).toHaveValue('Gold');
    expect(screen.getByLabelText('Request count')).toHaveValue(1000);

    await user.click(screen.getByRole('button', { name: 'Save changes' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    // Status isn't editable in this dialog — the plan's own status must round-trip.
    expect(JSON.parse(requests.last()!.body).status).toBe('INACTIVE');
  });

  it('shows a 409 as an inline field error, not a generic toast', async () => {
    server.use(failure('post', PLANS, 409, 'CONFLICT'));
    const { user } = setup();

    await user.type(screen.getByLabelText(/Name/), 'Gold');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    expect(await screen.findByText('Plan not created')).toBeInTheDocument();
    expect(screen.getByText('A plan with this name already exists')).toBeInTheDocument();
  });

  it('closes without saving on Cancel', async () => {
    const { user, onClose } = setup();

    await user.type(screen.getByLabelText(/Name/), 'Gold');
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(onClose).toHaveBeenCalled();
    expect(requests.count()).toBe(0);
  });
});
