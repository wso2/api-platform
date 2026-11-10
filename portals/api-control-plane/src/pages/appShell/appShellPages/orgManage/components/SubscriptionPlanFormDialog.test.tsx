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
  aMeter,
  aPlanPricing,
  aSubscriptionPlan,
  collection,
  failure,
  recorder,
  resource,
  type MeterFixture,
  type Recorder,
  type SubscriptionPlanFixture,
} from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, userEvent, waitFor, within } from '@/test/utils';
import { SubscriptionPlanFormDialog } from './SubscriptionPlanFormDialog';

const ORG = 'api-platform-demo';
const PLANS = '/subscription-plans';

/**
 * Adds up the input tokens of AI API requests; its prices may filter on the API type and the HTTP
 * method.
 */
const llmInput = aMeter({
  displayName: 'LLM input tokens',
  filterFields: [
    { eventKey: 'metadata.apiType', field: 'API type' },
    { eventKey: 'request.verb', field: 'HTTP method' },
  ],
  id: 'llm-input',
  type: 'TOKENS',
  typeKey: 'metadata.aiTokenUsage.promptTokens',
});

let requests: Recorder;

/** The usage of a plan nothing offers or uses. */
const UNUSED = { drafts: 0, inUse: false, publications: 0, restApis: 0, subscriptions: 0 };

/** Both mutations resolve their organization from `ApiScopeContext`, so the
 * provider has to be mounted for a request to be allowed out at all. */
function setup(
  overrides: {
    plan?: SubscriptionPlanFixture | null;
    meters?: MeterFixture[];
    usage?: NonNullable<SubscriptionPlanFixture['usage']>;
  } = {},
) {
  const onClose = vi.fn();
  // The Paid plan section lists the organization's meters, and reads each priced meter by id.
  const meters = overrides.meters ?? [aMeter()];
  server.use(
    collection('/meters', meters),
    ...meters.map((meter) => resource(`/meters/${meter.id}`, meter)),
  );
  // An existing plan is read again for its usage, which locks its name and conditions while it is used.
  if (overrides.plan) {
    server.use(
      resource(`${PLANS}/${overrides.plan.id}`, {
        ...overrides.plan,
        usage: overrides.usage ?? UNUSED,
      }),
    );
  }
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

type User = ReturnType<typeof userEvent.setup>;

/** Turns the plan paid and usage based, ready for its prices. */
async function chooseUsageBased(user: User) {
  await user.click(screen.getByRole('switch', { name: 'Paid plan' }));
  await user.click(screen.getByRole('combobox', { name: 'Pricing' }));
  await user.click(await screen.findByRole('option', { name: 'Usage based' }));
}

/**
 * Fills the filter row of a price on one property: picks its operator, then
 * types its values. `row` counts the property's rows from 0.
 */
async function setFilter(
  user: User,
  price: HTMLElement,
  field: string,
  operator: string,
  values: string,
  row = 0,
) {
  const filter = within(price).getAllByRole('group', { name: field })[row];
  await user.click(within(filter).getByRole('combobox', { name: 'Filter operator' }));
  await user.click(await screen.findByRole('option', { name: operator }));
  const input = within(filter).getByRole('textbox', { name: 'Filter values' });
  await user.clear(input);
  if (values) await user.type(input, values);
}

/** The operators a filter row offers, in order. */
async function operatorsOf(user: User, row: HTMLElement) {
  await user.click(within(row).getByRole('combobox', { name: 'Filter operator' }));
  const operators = (await screen.findAllByRole('option')).map((option) => option.textContent);
  await user.keyboard('{Escape}');
  return operators;
}

/** The price shown at `position`, counting from 1. */
const priceAt = (position: number) => screen.getByRole('group', { name: `Price ${position}` });

/** The names of a price's filter rows, in order. */
const filterNames = (price: HTMLElement) =>
  within(price)
    .queryAllByRole('group')
    .map((row) => row.firstElementChild?.textContent);

beforeEach(() => {
  disabled.names = [];
  requests = recorder();
  resetHttpClient();
});

const disabled = vi.hoisted(() => ({ names: [] as string[] }));

vi.mock('@/config/features', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/config/features')>()),
  isFeatureDisabled: (name: string) => disabled.names.includes(name),
}));

describe('SubscriptionPlanFormDialog', () => {
  it('creates a plan with a handle slugified from the name', async () => {
    server.use(accepts('post', PLANS, aSubscriptionPlan(), { record: requests }));
    const { user, onClose } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold Plan');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toMatchObject({
      id: 'gold-plan',
      displayName: 'Gold Plan',
      status: 'ACTIVE',
    });
    expect(onClose).toHaveBeenCalled();
  });

  it('keeps the handle of a long name to the 40 characters the Platform API takes, without a trailing hyphen', async () => {
    server.use(accepts('post', PLANS, aSubscriptionPlan(), { record: requests }));
    const { user } = setup();

    await user.type(
      screen.getByLabelText(/Plan name/),
      'Premium annual plan for enterprise team and partners',
    );
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body).id).toBe('premium-annual-plan-for-enterprise-team');
  });

  it('disables Add plan, staying quiet about the empty name until it is actually rejected', async () => {
    const { user } = setup();
    const submit = screen.getByRole('button', { name: 'Add plan' });
    const name = screen.getByLabelText(/Plan name/);

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

    await user.type(screen.getByLabelText(/Plan name/), '!!!');
    await user.tab();

    expect(screen.getByText('Use letters or numbers in the name')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add plan' })).toBeDisabled();
    expect(requests.count()).toBe(0);
  });

  it('shows the subtitle, the plan name field and the optional expiry label', () => {
    setup();

    expect(
      screen.getByText('Define the conditions, pricing and availability of this plan.'),
    ).toBeInTheDocument();
    expect(screen.getByPlaceholderText('e.g. Gold')).toBeInTheDocument();
    expect(screen.getByLabelText(/Expiry date \(optional\)/)).toBeInTheDocument();
  });

  it('shows Paid plan as coming soon, with no switch, when paidplan is disabled', () => {
    disabled.names = ['paidplan'];
    setup();

    expect(screen.getByText('Coming soon')).toBeInTheDocument();
    expect(screen.queryByRole('switch', { name: 'Paid plan' })).not.toBeInTheDocument();
  });

  it('shows no coming soon label by default', () => {
    setup();

    expect(screen.queryByText('Coming soon')).not.toBeInTheDocument();
  });

  it('offers a Paid plan switch, off for a new plan', () => {
    setup();

    expect(screen.getByRole('switch', { name: 'Paid plan' })).not.toBeChecked();
  });

  it('sends the picked expiry date as a full RFC3339 timestamp', async () => {
    server.use(accepts('post', PLANS, aSubscriptionPlan(), { record: requests }));
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold');
    // The picker is a segmented field: focus the first section and type the
    // whole date, which advances through month, day and year.
    await user.click(screen.getByRole('spinbutton', { name: 'Month' }));
    await user.keyboard('12312026');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body).expiryTime).toBe('2026-12-31T00:00:00.000Z');
  });

  it('sends an explicit null when the expiry date of an existing plan is cleared', async () => {
    const plan = aSubscriptionPlan({
      displayName: 'Gold',
      expiryTime: '2026-12-31T00:00:00Z',
      id: 'gold',
    });
    server.use(accepts('put', `${PLANS}/gold`, plan, { record: requests }));
    const { user } = setup({ plan });

    await user.click(screen.getByRole('button', { name: 'Clear' }));
    await user.click(screen.getByRole('button', { name: 'Save changes' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    // null, not absent: an omitted expiryTime means "keep the current value".
    expect(JSON.parse(requests.last()!.body)).toHaveProperty('expiryTime', null);
  });

  it('shows a taken name as an inline field error, not a generic toast', async () => {
    server.use(failure('post', PLANS, 409, 'SUBSCRIPTION_PLAN_EXISTS'));
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    expect(await screen.findByText('Plan not created')).toBeInTheDocument();
    expect(screen.getByText('A plan with this name already exists')).toBeInTheDocument();
  });

  it('closes without saving on Cancel', async () => {
    const { user, onClose } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold');
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(onClose).toHaveBeenCalled();
    expect(requests.count()).toBe(0);
  });

  it('creates a flat rate plan with its pricing in the one request', async () => {
    server.use(
      accepts('post', PLANS, aSubscriptionPlan({ id: 'gold-plan' }), { record: requests }),
    );
    const { user, onClose } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold Plan');
    await user.click(screen.getByRole('switch', { name: 'Paid plan' }));
    expect(screen.getByRole('combobox', { name: 'Pricing' })).toHaveTextContent('Flat rate');
    expect(screen.queryByRole('button', { name: 'Add price' })).not.toBeInTheDocument();
    await user.type(screen.getByLabelText('Amount'), '99.00');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toMatchObject({
      displayName: 'Gold Plan',
      id: 'gold-plan',
      pricing: {
        billingPeriod: 'MONTH',
        currency: 'USD',
        flatAmount: '99.00',
        pricingType: 'FLAT_RATE',
      },
    });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it('says, once a new plan is made paid, that its pricing is final', async () => {
    const { user } = setup();

    expect(
      screen.queryByText("Pricing can't be updated once the plan is created."),
    ).not.toBeInTheDocument();
    await user.click(screen.getByRole('switch', { name: 'Paid plan' }));
    expect(
      screen.getByText("Pricing can't be updated once the plan is created."),
    ).toBeInTheDocument();
    await user.click(screen.getByRole('switch', { name: 'Paid plan' }));
    expect(
      screen.queryByText("Pricing can't be updated once the plan is created."),
    ).not.toBeInTheDocument();
  });

  it('creates a free plan without pricing', async () => {
    server.use(
      accepts('post', PLANS, aSubscriptionPlan({ id: 'gold-plan' }), { record: requests }),
    );
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold Plan');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).not.toHaveProperty('pricing');
  });

  it('starts a usage based plan with one price, which cannot be removed while it is the only one', async () => {
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold Plan');
    await chooseUsageBased(user);
    const first = screen.getByRole('group', { name: 'Price 1' });
    expect(within(first).queryByRole('button', { name: 'Remove price 1' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Add price' }));
    expect(
      within(screen.getByRole('group', { name: 'Price 1' })).getByRole('button', {
        name: 'Remove price 1',
      }),
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole('group', { name: 'Price 2' })).getByRole('button', {
        name: 'Remove price 2',
      }),
    ).toBeInTheDocument();
  });

  it('prices a usage based plan per unit, counted by the chosen meter and narrowed by its filters', async () => {
    server.use(
      accepts('post', PLANS, aSubscriptionPlan({ id: 'gold-plan' }), { record: requests }),
    );
    const { user, onClose } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold Plan');
    await chooseUsageBased(user);
    expect(screen.queryByLabelText('Amount')).not.toBeInTheDocument();
    const price = screen.getByRole('group', { name: 'Price 1' });
    await user.click(within(price).getByRole('combobox', { name: 'Meter' }));
    await user.click(await screen.findByRole('option', { name: 'Successful calls' }));
    await setFilter(user, price, 'Status code', 'is one of', '200, 201');
    await user.type(within(price).getByLabelText('Unit price'), '0.01');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body).pricing).toEqual({
      billingPeriod: 'MONTH',
      currency: 'USD',
      prices: [
        {
          filters: [{ field: 'Status code', operator: 'IN', values: ['200', '201'] }],
          meterId: 'successful-calls',
          pricingModel: 'PER_UNIT',
          tiers: null,
          unitAmount: '0.01',
          unitQuantity: 1,
        },
      ],
      pricingType: 'USAGE_BASED',
    });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it('prices the same meter twice with different filters, each price named by its meter and filters', { timeout: 15_000 }, async () => {
    server.use(
      accepts('post', PLANS, aSubscriptionPlan({ id: 'gold-plan' }), { record: requests }),
    );
    const { user, onClose } = setup({ meters: [aMeter(), llmInput] });

    await user.type(screen.getByLabelText(/Plan name/), 'Gold Plan');
    await chooseUsageBased(user);

    const large = screen.getByRole('group', { name: 'Price 1' });
    await user.click(within(large).getByRole('combobox', { name: 'Meter' }));
    await user.click(await screen.findByRole('option', { name: 'LLM input tokens' }));
    await setFilter(user, large, 'API type', 'equals', 'LlmProvider');
    expect(within(large).getByText('LLM input tokens · LlmProvider')).toBeInTheDocument();
    await user.type(within(large).getByLabelText('Unit price'), '2.50');
    await user.clear(within(large).getByLabelText('Units per price'));
    await user.type(within(large).getByLabelText('Units per price'), '1000000');

    await user.click(screen.getByRole('button', { name: 'Add price' }));
    const small = screen.getByRole('group', { name: 'Price 2' });
    // Every meter is offered, also one another price uses.
    await user.click(within(small).getByRole('combobox', { name: 'Meter' }));
    expect((await screen.findAllByRole('option')).map((option) => option.textContent)).toEqual([
      'Successful calls',
      'LLM input tokens',
    ]);
    await user.click(screen.getByRole('option', { name: 'LLM input tokens' }));
    await setFilter(user, small, 'API type', 'is one of', 'LlmProxy, Mcp');
    expect(within(small).getByText('LLM input tokens · LlmProxy, Mcp')).toBeInTheDocument();
    await user.click(within(small).getByRole('combobox', { name: 'Pricing model' }));
    await user.click(await screen.findByRole('option', { name: 'Graduated tiers' }));
    await user.type(within(small).getAllByLabelText('Up to')[0], '10000');
    const [firstTier, lastTier] = within(small).getAllByLabelText('Per unit');
    await user.type(firstTier, '0');
    await user.type(lastTier, '0.001');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body).pricing).toEqual({
      billingPeriod: 'MONTH',
      currency: 'USD',
      prices: [
        {
          filters: [{ field: 'API type', operator: 'EQUALS', values: ['LlmProvider'] }],
          meterId: 'llm-input',
          pricingModel: 'PER_UNIT',
          tiers: null,
          unitAmount: '2.50',
          unitQuantity: 1000000,
        },
        {
          filters: [{ field: 'API type', operator: 'IN', values: ['LlmProxy', 'Mcp'] }],
          meterId: 'llm-input',
          pricingModel: 'GRADUATED',
          tiers: [
            { flatAmount: null, unitAmount: '0', upTo: 10000 },
            { flatAmount: null, unitAmount: '0.001', upTo: null },
          ],
          unitAmount: null,
          unitQuantity: null,
        },
      ],
      pricingType: 'USAGE_BASED',
    });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("takes one value or several for a filter, as its operator reads, sent in the meter's order", async () => {
    server.use(
      accepts('post', PLANS, aSubscriptionPlan({ id: 'gold-plan' }), { record: requests }),
    );
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold Plan');
    await chooseUsageBased(user);
    const price = screen.getByRole('group', { name: 'Price 1' });
    await user.click(within(price).getByRole('combobox', { name: 'Meter' }));
    await user.click(await screen.findByRole('option', { name: 'Successful calls' }));
    await setFilter(user, price, 'Resource', 'starts with', '/orders');
    await setFilter(user, price, 'HTTP method', 'is not one of', 'DELETE, PATCH');
    expect(
      within(price).getByText('Successful calls · not DELETE, PATCH · /orders…'),
    ).toBeInTheDocument();
    await user.type(within(price).getByLabelText('Unit price'), '0.01');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body).pricing.prices[0].filters).toEqual([
      { field: 'HTTP method', operator: 'NOT_IN', values: ['DELETE', 'PATCH'] },
      { field: 'Resource', operator: 'STARTS_WITH', values: ['/orders'] },
    ]);
  });

  it('does not save two prices of one meter that count the same requests', async () => {
    server.use(accepts('post', PLANS, aSubscriptionPlan(), { record: requests }));
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold Plan');
    await chooseUsageBased(user);
    for (const [position, operator, codes] of [
      [1, 'equals', '200'],
      [2, 'is one of', '200, 404'],
    ] as const) {
      if (position > 1) await user.click(screen.getByRole('button', { name: 'Add price' }));
      const price = screen.getByRole('group', { name: `Price ${position}` });
      await user.click(within(price).getByRole('combobox', { name: 'Meter' }));
      await user.click(await screen.findByRole('option', { name: 'Successful calls' }));
      await setFilter(user, price, 'Status code', operator, codes);
      await user.type(within(price).getByLabelText('Unit price'), '0.01');
    }
    // A meter can be priced again, so adding a price stays open.
    expect(screen.getByRole('button', { name: 'Add price' })).toBeEnabled();
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    expect(
      await screen.findByText(
        'Prices 1 and 2 both count the same requests of the meter successful-calls; give them different filters.',
      ),
    ).toBeInTheDocument();
    expect(requests.count()).toBe(0);
  });

  it('counts every value of a filter left empty, and does not send it', async () => {
    server.use(
      accepts('post', PLANS, aSubscriptionPlan({ id: 'gold-plan' }), { record: requests }),
    );
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold Plan');
    await chooseUsageBased(user);
    const price = screen.getByRole('group', { name: 'Price 1' });
    await user.click(within(price).getByRole('combobox', { name: 'Meter' }));
    await user.click(await screen.findByRole('option', { name: 'Successful calls' }));
    expect(
      within(price).getByText('Leave a filter empty to count every value.'),
    ).toBeInTheDocument();
    await setFilter(user, price, 'HTTP method', 'equals', 'GET');
    expect(within(price).getByText('Successful calls · GET')).toBeInTheDocument();
    await setFilter(user, price, 'HTTP method', 'is one of', '');
    // Several values are typed with commas between them.
    expect(
      within(within(price).getByRole('group', { name: 'HTTP method' })).getByRole('textbox', {
        name: 'Filter values',
      }),
    ).toHaveAttribute('placeholder', 'Separate values with commas');
    expect(within(price).getByText('Successful calls')).toBeInTheDocument();
    await user.type(within(price).getByLabelText('Unit price'), '0.01');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body).pricing.prices[0].filters).toEqual([]);
  });

  it('shows what the chosen meter counts, in its units, and a row for each of its filters, keeping a filter the new meter also has', async () => {
    server.use(
      accepts('post', PLANS, aSubscriptionPlan({ id: 'gold-plan' }), { record: requests }),
    );
    const { user } = setup({ meters: [aMeter(), llmInput] });

    await user.type(screen.getByLabelText(/Plan name/), 'Gold Plan');
    await chooseUsageBased(user);
    const price = screen.getByRole('group', { name: 'Price 1' });
    expect(within(price).getByRole('textbox', { name: 'Event' })).toHaveValue('');
    expect(within(price).getByText('unit')).toBeInTheDocument();
    await user.click(within(price).getByRole('combobox', { name: 'Meter' }));
    await user.click(await screen.findByRole('option', { name: 'Successful calls' }));
    expect(within(price).getByRole('textbox', { name: 'Event' })).toHaveValue('Requests');
    expect(within(price).getByRole('textbox', { name: 'Event' })).toHaveAttribute('readonly');
    // The price per unit reads in what the meter counts.
    expect(within(price).getByText('request')).toBeInTheDocument();
    expect(filterNames(price)).toEqual(['Status code', 'HTTP method', 'Resource']);
    // The meter decides the filters: none are added or removed here.
    expect(within(price).queryByRole('button', { name: /filter/i })).not.toBeInTheDocument();
    await setFilter(user, price, 'Status code', 'equals', '200');
    await setFilter(user, price, 'HTTP method', 'equals', 'GET');

    await user.click(within(price).getByRole('combobox', { name: 'Meter' }));
    await user.click(await screen.findByRole('option', { name: 'LLM input tokens' }));
    // A token meter reads "Tokens", whichever tokens it adds up; its name says which.
    expect(within(price).getByRole('textbox', { name: 'Event' })).toHaveValue('Tokens');
    expect(within(price).getByText('token')).toBeInTheDocument();
    // LLM input tokens filters on the API type and the HTTP method, so the status code and the
    // resource go.
    expect(filterNames(price)).toEqual(['API type', 'HTTP method']);
    expect(within(price).getByText('LLM input tokens · GET')).toBeInTheDocument();
    const apiType = within(price).getByRole('group', { name: 'API type' });
    expect(within(apiType).getByRole('combobox', { name: 'Filter operator' })).toHaveTextContent(
      'equals',
    );
    expect(within(apiType).getByRole('textbox', { name: 'Filter values' })).toHaveValue('');
    await user.type(within(price).getByLabelText('Unit price'), '0.01');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body).pricing.prices[0]).toMatchObject({
      filters: [{ field: 'HTTP method', operator: 'EQUALS', values: ['GET'] }],
      meterId: 'llm-input',
    });
  });

  it('offers every operator on any property, with typed values', async () => {
    server.use(
      accepts('post', PLANS, aSubscriptionPlan({ id: 'gold-plan' }), { record: requests }),
    );
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold Plan');
    await chooseUsageBased(user);
    const price = screen.getByRole('group', { name: 'Price 1' });
    await user.click(within(price).getByRole('combobox', { name: 'Meter' }));
    await user.click(await screen.findByRole('option', { name: 'Successful calls' }));
    expect(await operatorsOf(user, within(price).getByRole('group', { name: 'Resource' }))).toEqual(
      [
        'equals',
        'does not equal',
        'is one of',
        'is not one of',
        'starts with',
        'more than',
        'at least',
        'less than',
        'at most',
      ],
    );
    await setFilter(user, price, 'Status code', 'at least', '500');
    await setFilter(user, price, 'HTTP method', 'starts with', 'P');
    await setFilter(user, price, 'Resource', 'is not one of', '/health, /ping');
    expect(
      within(price).getByText('Successful calls · Status code ≥ 500 · P… · not /health, /ping'),
    ).toBeInTheDocument();
    await user.type(within(price).getByLabelText('Unit price'), '0.01');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body).pricing.prices[0].filters).toEqual([
      { field: 'Status code', operator: 'GREATER_THAN_OR_EQUAL', values: ['500'] },
      { field: 'HTTP method', operator: 'STARTS_WITH', values: ['P'] },
      { field: 'Resource', operator: 'NOT_IN', values: ['/health', '/ping'] },
    ]);
  });

  it('sets up to two comparisons on a property, each a whole number, and sends both', async () => {
    const tokenMeter = aMeter({
      displayName: 'LLM input tokens',
      filterFields: [{ eventKey: 'metadata.aiTokenUsage.promptTokens', field: 'Prompt tokens' }],
      id: 'llm-input',
      type: 'TOKENS',
      typeKey: 'metadata.aiTokenUsage.promptTokens',
    });
    server.use(
      accepts('post', PLANS, aSubscriptionPlan({ id: 'gold-plan' }), { record: requests }),
    );
    const { user } = setup({ meters: [tokenMeter] });

    await user.type(screen.getByLabelText(/Plan name/), 'Gold Plan');
    await chooseUsageBased(user);
    const price = screen.getByRole('group', { name: 'Price 1' });
    await user.click(within(price).getByRole('combobox', { name: 'Meter' }));
    await user.click(await screen.findByRole('option', { name: 'LLM input tokens' }));
    expect(filterNames(price)).toEqual(['Prompt tokens']);
    const addCondition = () =>
      within(price).queryByRole('button', { name: 'Add a condition on Prompt tokens' });
    // Only a comparison takes a second one.
    expect(addCondition()).not.toBeInTheDocument();

    await setFilter(user, price, 'Prompt tokens', 'more than', '1.5');
    await user.type(within(price).getByLabelText('Unit price'), '0.01');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));
    expect(
      await screen.findByText(
        'Price 1: the value of Prompt tokens must be a whole number from 0 to 1,000,000,000,000.',
      ),
    ).toBeInTheDocument();

    // A second comparison bounds the other side; no third.
    await setFilter(user, price, 'Prompt tokens', 'more than', '500');
    await user.click(addCondition()!);
    expect(addCondition()).not.toBeInTheDocument();
    const [first, second] = within(price).getAllByRole('group', { name: 'Prompt tokens' });
    expect(within(second).getByRole('combobox', { name: 'Filter operator' })).toHaveTextContent(
      'at most',
    );
    // Two comparisons make a range, so each offers the comparisons only.
    expect(await operatorsOf(user, first)).toEqual([
      'more than',
      'at least',
      'less than',
      'at most',
    ]);
    await user.type(within(second).getByRole('textbox', { name: 'Filter values' }), '100');
    expect(
      await screen.findByText("Price 1: the conditions on Prompt tokens can't all be true."),
    ).toBeInTheDocument();
    expect(requests.count()).toBe(0);

    await setFilter(user, price, 'Prompt tokens', 'more than', '10');
    expect(
      within(price).getByText('LLM input tokens · Prompt tokens > 10 · Prompt tokens ≤ 100'),
    ).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body).pricing.prices[0].filters).toEqual([
      { field: 'Prompt tokens', operator: 'GREATER_THAN', values: ['10'] },
      { field: 'Prompt tokens', operator: 'LESS_THAN_OR_EQUAL', values: ['100'] },
    ]);
  });

  it('removes the second comparison on a property, offering it again', async () => {
    const tokenMeter = aMeter({
      displayName: 'LLM input tokens',
      filterFields: [{ eventKey: 'metadata.aiTokenUsage.totalTokens', field: 'Total tokens' }],
      id: 'llm-input',
      type: 'TOKENS',
      typeKey: 'metadata.aiTokenUsage.promptTokens',
    });
    const { user } = setup({ meters: [tokenMeter] });

    await chooseUsageBased(user);
    const price = screen.getByRole('group', { name: 'Price 1' });
    await user.click(within(price).getByRole('combobox', { name: 'Meter' }));
    await user.click(await screen.findByRole('option', { name: 'LLM input tokens' }));
    await setFilter(user, price, 'Total tokens', 'at most', '');
    await user.click(
      within(price).getByRole('button', { name: 'Add a condition on Total tokens' }),
    );
    const rows = within(price).getAllByRole('group', { name: 'Total tokens' });
    expect(rows).toHaveLength(2);
    expect(within(rows[1]).getByRole('combobox', { name: 'Filter operator' })).toHaveTextContent(
      'more than',
    );

    await user.click(
      within(price).getByRole('button', { name: 'Remove this condition on Total tokens' }),
    );
    expect(within(price).getAllByRole('group', { name: 'Total tokens' })).toHaveLength(1);
    expect(
      within(price).getByRole('button', { name: 'Add a condition on Total tokens' }),
    ).toBeInTheDocument();
    // The one left offers every operator again.
    expect(
      await operatorsOf(user, within(price).getByRole('group', { name: 'Total tokens' })),
    ).toHaveLength(9);
  });

  it('offers no filters on a meter that has none', async () => {
    const plain = aMeter({ displayName: 'All calls', filterFields: [], id: 'all-calls' });
    const { user } = setup({ meters: [plain] });

    await chooseUsageBased(user);
    const price = screen.getByRole('group', { name: 'Price 1' });
    // Filters come with the meter.
    expect(within(price).queryByText('Filters')).not.toBeInTheDocument();
    await user.click(within(price).getByRole('combobox', { name: 'Meter' }));
    await user.click(await screen.findByRole('option', { name: 'All calls' }));

    expect(within(price).getByText('This meter has no filters.')).toBeInTheDocument();
    expect(
      within(price).queryByText('Leave a filter empty to count every value.'),
    ).not.toBeInTheDocument();
    expect(filterNames(price)).toEqual([]);
  });

  it("shows a saved price's rows from its filters, the meter's other filters empty", async () => {
    const { user } = setup({ plan: aSubscriptionPlan({ pricing: aPlanPricing() }) });

    const price = screen.getByRole('group', { name: 'Price 1' });
    await within(price).findByRole('group', { name: 'Resource' });
    expect(filterNames(price)).toEqual(['Status code', 'HTTP method', 'Resource']);
    const status = within(price).getByRole('group', { name: 'Status code' });
    expect(within(status).getByRole('combobox', { name: 'Filter operator' })).toHaveTextContent(
      'is one of',
    );
    expect(within(status).getByRole('textbox', { name: 'Filter values' })).toHaveValue('200');
    const method = within(price).getByRole('group', { name: 'HTTP method' });
    expect(within(method).getByRole('combobox', { name: 'Filter operator' })).toHaveTextContent(
      'equals',
    );
    expect(within(method).getByRole('textbox', { name: 'Filter values' })).toHaveValue('');
    await user.click(screen.getByRole('button', { name: 'Cancel' }));
  });

  it("shows an existing plan's pricing read-only, its name and conditions still editable", async () => {
    setup({ plan: aSubscriptionPlan({ pricing: aPlanPricing() }) });

    expect(
      screen.getByText("Pricing can't be updated once a plan is created."),
    ).toBeInTheDocument();
    expect(screen.getByRole('switch', { name: 'Paid plan' })).toBeDisabled();
    for (const name of ['Currency', 'Charge every', 'Pricing']) {
      const field = screen.getByRole('combobox', { name });
      expect(field.hasAttribute('disabled') || field.getAttribute('aria-disabled') === 'true').toBe(
        true,
      );
    }
    const price = screen.getByRole('group', { name: 'Price 1' });
    expect(within(price).getByRole('combobox', { name: 'Meter' })).toBeDisabled();
    expect(within(price).getByLabelText('Unit price')).toBeDisabled();
    expect(within(price).getByLabelText('Units per price')).toBeDisabled();
    expect(within(price).getAllByRole('textbox', { name: 'Filter values' })[0]).toBeDisabled();
    // No price can be added or removed.
    expect(screen.queryByRole('button', { name: 'Add price' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Remove price 1' })).not.toBeInTheDocument();
    expect(
      screen.queryByText('Create a meter in Manage › Meters to charge for usage.'),
    ).not.toBeInTheDocument();
    // The rest of the plan stays editable.
    expect(screen.getByLabelText(/Plan name/)).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Add condition' })).toBeEnabled();
  });

  it("lists an existing plan's prices by name, as the API Portal does, whatever order they come in", async () => {
    const onMethod = (verb: string) => ({
      filters: [{ field: 'HTTP method', operator: 'EQUALS' as const, values: [verb] }],
      meterId: 'successful-calls',
      pricingModel: 'PER_UNIT' as const,
      tiers: null,
      unitAmount: '0.01',
      unitQuantity: 1,
    });
    const pricing = aPlanPricing({ prices: [onMethod('POST'), onMethod('GET')] });
    setup({ plan: aSubscriptionPlan({ pricing }) });

    // Named once the meters are in.
    await screen.findByText('Successful calls · GET');
    expect(within(priceAt(1)).getByText('Successful calls · GET')).toBeInTheDocument();
    expect(within(priceAt(2)).getByText('Successful calls · POST')).toBeInTheDocument();
  });

  it('keeps the prices of a new plan in the order they are added', async () => {
    const { user } = setup();

    await chooseUsageBased(user);
    const first = screen.getByRole('group', { name: 'Price 1' });
    await user.click(within(first).getByRole('combobox', { name: 'Meter' }));
    await user.click(await screen.findByRole('option', { name: 'Successful calls' }));
    await setFilter(user, first, 'HTTP method', 'equals', 'POST');
    await user.click(screen.getByRole('button', { name: 'Add price' }));
    const second = screen.getByRole('group', { name: 'Price 2' });
    await user.click(within(second).getByRole('combobox', { name: 'Meter' }));
    await user.click(await screen.findByRole('option', { name: 'Successful calls' }));
    await setFilter(user, second, 'HTTP method', 'equals', 'GET');

    expect(within(priceAt(1)).getByText('Successful calls · POST')).toBeInTheDocument();
    expect(within(priceAt(2)).getByText('Successful calls · GET')).toBeInTheDocument();
  });

  it("keeps a free plan free: its Paid plan switch can't be turned on", async () => {
    setup({ plan: aSubscriptionPlan({ displayName: 'Bronze', id: 'bronze' }) });

    expect(screen.getByRole('switch', { name: 'Paid plan' })).not.toBeChecked();
    expect(screen.getByRole('switch', { name: 'Paid plan' })).toBeDisabled();
    expect(
      screen.getByText("Pricing can't be updated once a plan is created."),
    ).toBeInTheDocument();
  });

  it('saves an edited plan without its pricing, even when the stored pricing no longer checks out', async () => {
    server.use(accepts('put', `${PLANS}/gold`, aSubscriptionPlan(), { record: requests }));
    // The stored price filters on the status code, which the meter no longer lists.
    const { user, onClose } = setup({
      meters: [aMeter({ filterFields: [{ eventKey: 'request.verb', field: 'HTTP method' }] })],
      plan: aSubscriptionPlan({ displayName: 'Gold', id: 'gold', pricing: aPlanPricing() }),
    });

    const price = screen.getByRole('group', { name: 'Price 1' });
    await within(price).findByRole('group', { name: 'HTTP method' });
    const name = screen.getByLabelText(/Plan name/);
    await user.clear(name);
    await user.type(name, 'Gold Plus');
    await user.click(screen.getByRole('button', { name: 'Save changes' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    const body = JSON.parse(requests.last()!.body);
    expect(body).toMatchObject({ displayName: 'Gold Plus', id: 'gold' });
    // A body with pricing is refused; the plan keeps the pricing it has.
    expect(body).not.toHaveProperty('pricing');
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it('finds a meter by typing part of its name, the Platform API searching', async () => {
    const requests = recorder();
    const meters = [
      aMeter(),
      ...Array.from({ length: 30 }, (_, i) =>
        aMeter({ displayName: `Meter ${i + 1}`, id: `meter-${i + 1}` }),
      ),
    ];
    const { user } = setup({ meters });
    // Registered after setup's, so the searches are recorded.
    server.use(collection('/meters', meters, { record: requests }));

    await chooseUsageBased(user);
    const meter = screen.getByRole('combobox', { name: 'Meter' });
    await user.type(meter, 'meter 27');
    expect(await screen.findByRole('option', { name: 'Meter 27' })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'Successful calls' })).not.toBeInTheDocument();
    await waitFor(() =>
      expect(requests.calls.some((call) => call.params.get('query') === 'meter 27')).toBe(true),
    );

    await user.click(screen.getByRole('option', { name: 'Meter 27' }));
    expect(meter).toHaveValue('Meter 27');
  });

  it('does not save a usage price until a meter is chosen', async () => {
    server.use(accepts('post', PLANS, aSubscriptionPlan(), { record: requests }));
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold Plan');
    await chooseUsageBased(user);
    expect(screen.getByRole('combobox', { name: 'Meter' })).toHaveValue('');
    await user.type(screen.getByLabelText('Unit price'), '0.01');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    expect(
      await screen.findByText('Pick the meter that counts the units of price 1.'),
    ).toBeInTheDocument();
    expect(requests.count()).toBe(0);
  });

  it('finds a currency by typing part of its name', async () => {
    server.use(
      accepts('post', PLANS, aSubscriptionPlan({ id: 'gold-plan' }), { record: requests }),
    );
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold Plan');
    await user.click(screen.getByRole('switch', { name: 'Paid plan' }));
    const currency = screen.getByRole('combobox', { name: 'Currency' });
    await user.clear(currency);
    await user.type(currency, 'sri lanka');
    expect(screen.getAllByRole('option').map((option) => option.textContent)).toEqual([
      'LKR — Sri Lanka Rupee',
    ]);
    await user.click(screen.getByRole('option', { name: 'LKR — Sri Lanka Rupee' }));
    await user.type(screen.getByLabelText('Amount'), '99.00');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body).pricing).toMatchObject({
      currency: 'LKR',
      flatAmount: '99.00',
    });
  });

  it('does not save a flat rate plan without an amount, nor a usage based plan whose price has no meter', async () => {
    server.use(accepts('post', PLANS, aSubscriptionPlan(), { record: requests }));
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold Plan');
    await user.click(screen.getByRole('switch', { name: 'Paid plan' }));
    await user.click(screen.getByRole('button', { name: 'Add plan' }));
    expect(
      await screen.findByText('Enter the flat rate as an amount above zero, such as 20.00.'),
    ).toBeInTheDocument();

    await user.click(screen.getByRole('combobox', { name: 'Pricing' }));
    await user.click(await screen.findByRole('option', { name: 'Usage based' }));
    // A usage based plan starts with one price, still to be filled in.
    expect(
      await screen.findByText('Pick the meter that counts the units of price 1.'),
    ).toBeInTheDocument();
    expect(requests.count()).toBe(0);
  });

  it('shows the reason a new plan with pricing is refused, and sends nothing else', async () => {
    const message = 'Each tier of price 1 needs an upTo greater than the tier before it';
    server.use(failure('post', PLANS, 400, 'VALIDATION_FAILED', { message, record: requests }));
    const { user, onClose } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold Plan');
    await user.click(screen.getByRole('switch', { name: 'Paid plan' }));
    await user.type(screen.getByLabelText('Amount'), '99.00');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(requests.count()).toBe(1);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("shows the server's reason for refusing an edit, and stays open", async () => {
    const message = 'The specified subscription plan could not be found.';
    server.use(
      failure('put', `${PLANS}/gold`, 404, 'SUBSCRIPTION_PLAN_NOT_FOUND', {
        message,
        record: requests,
      }),
    );
    const { user, onClose } = setup({
      plan: aSubscriptionPlan({ displayName: 'Gold', id: 'gold' }),
    });

    await user.click(screen.getByRole('button', { name: 'Save changes' }));

    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(requests.count()).toBe(1);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("shows a 409 on an edit as the server's reason, not as a taken name", async () => {
    const message = 'The subscription plan could not be updated.';
    server.use(failure('put', `${PLANS}/gold`, 409, 'CONFLICT', { message }));
    const { user, onClose } = setup({
      plan: aSubscriptionPlan({ displayName: 'Gold', id: 'gold' }),
    });

    await user.click(screen.getByRole('button', { name: 'Save changes' }));

    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(screen.queryByText('Plan not created')).not.toBeInTheDocument();
    expect(screen.queryByText('A plan with this name already exists')).not.toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });
});

/** Picks a condition type from the Add condition menu. */
async function addCondition(user: User, type: string) {
  await user.click(screen.getByRole('button', { name: 'Add condition' }));
  await user.click(await screen.findByRole('menuitem', { name: new RegExp(`^${type}`) }));
  await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument());
}

/** Picks an option of a select. */
async function choose(user: User, select: HTMLElement, option: string) {
  await user.click(select);
  await user.click(await screen.findByRole('option', { name: option }));
}

/** The card of a condition, by its type's name. */
const card = (name: string) => screen.getByRole('group', { name });

const requestsCondition = {
  id: 'requests',
  params: {
    limits: [
      { count: 100, timeAmount: 1, timeUnit: 'MINUTE' },
      { count: 10000, timeAmount: 1, timeUnit: 'DAY' },
    ],
    onExceed: 'allow',
  },
  type: 'request-count' as const,
};

const customCondition = {
  id: 'tier',
  params: { code: '00123', level: 'gold', max: 5 },
  type: 'custom' as const,
};

describe('SubscriptionPlanFormDialog conditions', () => {
  it('offers the built-in condition types not yet used, and Custom', async () => {
    const { user } = setup();

    expect(screen.getByText('No conditions: this plan is unlimited.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Add condition' }));
    expect((await screen.findAllByRole('menuitem')).map((item) => item.textContent)).toEqual([
      'Request countREST APIs only. Counts every request.',
      'Event countWebSocket and WebSub APIs only. Counts every event.',
      'Tool-call countMCP proxies only. Counts tools/call.',
      'Token countLLM proxies only. Counts model tokens.',
      'CustomAn ID and key-value pairs, read by a policy your team writes.',
    ]);
    await user.click(screen.getByRole('menuitem', { name: /^Request count/ }));
    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument());

    expect(screen.queryByText('No conditions: this plan is unlimited.')).not.toBeInTheDocument();
    expect(card('Request count')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Add condition' }));
    const names = (await screen.findAllByRole('menuitem')).map((item) => item.textContent ?? '');
    expect(names).toHaveLength(4);
    expect(names.some((name) => name.startsWith('Request count'))).toBe(false);
  });

  it('creates a plan with its conditions and description, never the legacy limits', async () => {
    server.use(accepts('post', PLANS, aSubscriptionPlan(), { record: requests }));
    const { user, onClose } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold');
    await user.type(screen.getByLabelText(/Description/), 'For partners');

    await addCondition(user, 'Request count');
    const requestCount = card('Request count');
    await user.type(within(requestCount).getByLabelText('Count'), '100');
    await user.click(within(requestCount).getByRole('button', { name: 'Add limit' }));
    await user.type(within(requestCount).getAllByLabelText('Count')[1], '10000');
    await choose(
      user,
      within(requestCount).getAllByRole('combobox', { name: 'Window unit' })[1],
      'Day',
    );
    await choose(
      user,
      within(requestCount).getByRole('combobox', { name: 'When a limit is reached' }),
      'Allow when reached',
    );
    expect(
      within(requestCount).getByText(
        '100 requests per minute · 10,000 requests per day · overage allowed',
      ),
    ).toBeInTheDocument();

    await addCondition(user, 'Token count');
    const tokenCount = card('Token count');
    await choose(
      user,
      within(tokenCount).getByRole('combobox', { name: 'Tokens counted' }),
      'Prompt tokens',
    );
    await user.type(within(tokenCount).getByLabelText('Count'), '5000');
    await user.clear(within(tokenCount).getByLabelText('Window length'));
    await user.type(within(tokenCount).getByLabelText('Window length'), '2');
    await choose(user, within(tokenCount).getByRole('combobox', { name: 'Window unit' }), 'Hour');

    await addCondition(user, 'Custom');
    const custom = card('Custom');
    await user.type(within(custom).getByLabelText(/^ID/), 'business-hours');
    await user.type(within(custom).getByLabelText('Key'), 'start');
    await user.type(within(custom).getByLabelText('Value'), '9');
    await user.click(within(custom).getByRole('button', { name: 'Add value' }));
    await user.type(within(custom).getAllByLabelText('Key')[1], 'zone');
    await user.type(within(custom).getAllByLabelText('Value')[1], 'UTC');

    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    const body = JSON.parse(requests.last()!.body);
    expect(body).toMatchObject({ description: 'For partners', displayName: 'Gold', id: 'gold' });
    expect(body).not.toHaveProperty('limits');
    expect(body.conditions).toEqual([
      {
        id: 'requests',
        params: {
          limits: [
            { count: 100, timeAmount: 1, timeUnit: 'MINUTE' },
            { count: 10000, timeAmount: 1, timeUnit: 'DAY' },
          ],
          onExceed: 'allow',
        },
        type: 'request-count',
      },
      {
        id: 'tokens',
        params: {
          limits: [{ count: 5000, timeAmount: 2, timeUnit: 'HOUR' }],
          onExceed: 'reject',
          tokenType: 'prompt',
        },
        type: 'token-count',
      },
      { id: 'business-hours', params: { start: 9, zone: 'UTC' }, type: 'custom' },
    ]);
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it('sends an empty conditions list, and no description, for an unlimited plan', async () => {
    server.use(accepts('post', PLANS, aSubscriptionPlan(), { record: requests }));
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Free');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    const body = JSON.parse(requests.last()!.body);
    expect(body.conditions).toEqual([]);
    expect(body).not.toHaveProperty('description');
    expect(body).not.toHaveProperty('limits');
  });

  it('shows a bad count, or a window another limit already has, by its limit', async () => {
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold');
    await addCondition(user, 'Request count');
    const requestCount = card('Request count');
    // The new limit's count has the focus, and is not flagged before it is filled in.
    expect(within(requestCount).getByLabelText('Count')).toHaveFocus();
    expect(
      within(requestCount).queryByText('Enter a whole number of at least 1.'),
    ).not.toBeInTheDocument();
    await user.type(within(requestCount).getByLabelText('Count'), '0');
    await user.tab();

    expect(
      within(requestCount).getByText('Enter a whole number of at least 1.'),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add plan' })).toBeDisabled();

    await user.clear(within(requestCount).getByLabelText('Count'));
    await user.type(within(requestCount).getByLabelText('Count'), '100');
    expect(
      within(requestCount).queryByText('Enter a whole number of at least 1.'),
    ).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add plan' })).toBeEnabled();

    // 60 minutes is the same window as 1 hour.
    await user.click(within(requestCount).getByRole('button', { name: 'Add limit' }));
    await user.type(within(requestCount).getAllByLabelText('Count')[1], '5000');
    await user.clear(within(requestCount).getAllByLabelText('Window length')[1]);
    await user.type(within(requestCount).getAllByLabelText('Window length')[1], '60');
    await user.tab();
    await choose(
      user,
      within(requestCount).getAllByRole('combobox', { name: 'Window unit' })[0],
      'Hour',
    );

    expect(
      within(requestCount).getByText(
        'Another limit already covers this window; each limit needs a window of its own length.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add plan' })).toBeDisabled();
    expect(requests.count()).toBe(0);
  });

  it('refuses a count past what a number holds exactly, rather than rounding it', async () => {
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold');
    await addCondition(user, 'Request count');
    const requestCount = card('Request count');
    await user.type(within(requestCount).getByLabelText('Count'), '9007199254740993');
    await user.tab();

    expect(
      within(requestCount).getByText('Enter a whole number of at most 9,007,199,254,740,991.'),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add plan' })).toBeDisabled();
    expect(requests.count()).toBe(0);
  });

  it('sends a custom key named __proto__ as a key of its own', async () => {
    server.use(accepts('post', PLANS, aSubscriptionPlan(), { record: requests }));
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold');
    await addCondition(user, 'Custom');
    const custom = card('Custom');
    await user.type(within(custom).getByLabelText(/^ID/), 'tier');
    await user.type(within(custom).getByLabelText('Key'), '__proto__');
    await user.type(within(custom).getByLabelText('Value'), 'gold');
    await user.click(screen.getByRole('button', { name: 'Add plan' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    const raw = requests.last()!.body;
    expect(raw).toContain('{"id":"tier","params":{"__proto__":"gold"},"type":"custom"}');
    const params = JSON.parse(raw).conditions[0].params as Record<string, unknown>;
    expect(Object.keys(params)).toEqual(['__proto__']);
  });

  it("checks a custom condition's ID and keys as the Platform API does", async () => {
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold');
    await addCondition(user, 'Custom');
    const custom = card('Custom');
    const id = within(custom).getByLabelText(/^ID/);

    await user.type(id, 'Business Hours');
    await user.tab();
    expect(
      within(custom).getByText(
        'Use lowercase letters, digits and hyphens, starting with a letter (at most 63 characters).',
      ),
    ).toBeInTheDocument();

    await user.clear(id);
    await user.type(id, 'requests');
    expect(
      within(custom).getByText('requests is the ID of a built-in condition; choose another.'),
    ).toBeInTheDocument();

    await user.clear(id);
    await user.type(id, 'tier');
    await user.type(within(custom).getByLabelText('Value'), 'gold');
    await user.tab();
    expect(within(custom).getByText('Each value needs a key.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add plan' })).toBeDisabled();

    await user.type(within(custom).getByLabelText('Key'), 'level');
    expect(within(custom).queryByText('Each value needs a key.')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add plan' })).toBeEnabled();
  });

  it('refuses a description longer than the Platform API keeps', async () => {
    const { user } = setup();

    await user.type(screen.getByLabelText(/Plan name/), 'Gold');
    await user.click(screen.getByLabelText(/Description/));
    await user.paste('x'.repeat(1024));

    expect(screen.getByText('Use at most 1,023 characters (1,024 now).')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add plan' })).toBeDisabled();
  });

  it("opens a plan's conditions collapsed, with their summary, and saves them back as they were", async () => {
    const plan = aSubscriptionPlan({
      conditions: [requestsCondition, customCondition],
      description: 'For partners',
      displayName: 'Gold',
      id: 'gold',
      status: 'INACTIVE',
    });
    server.use(accepts('put', `${PLANS}/gold`, plan, { record: requests }));
    const { user } = setup({ plan });

    expect(screen.getByLabelText(/Plan name/)).toHaveValue('Gold');
    expect(screen.getByLabelText(/Description/)).toHaveValue('For partners');
    expect(
      screen.getByText('100 requests per minute · 10,000 requests per day · overage allowed'),
    ).toBeInTheDocument();
    expect(screen.getByText('code = 00123 · level = gold · max = 5')).toBeInTheDocument();
    // Collapsed: the windows are not shown until the condition is opened.
    expect(screen.queryByLabelText('Count')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Show Request count settings' }));
    expect(
      within(card('Request count'))
        .getAllByLabelText('Count')
        .map((input) => (input as HTMLInputElement).value),
    ).toEqual(['100', '10000']);

    await user.click(screen.getByRole('button', { name: 'Save changes' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    const body = JSON.parse(requests.last()!.body);
    // Status isn't editable in this dialog — the plan's own status must round-trip.
    expect(body.status).toBe('INACTIVE');
    // A saved "00123" stays a string, and 5 stays a number.
    expect(body.conditions).toEqual([requestsCondition, customCondition]);
    expect(body).not.toHaveProperty('limits');
    // The description did not change, so it is not sent.
    expect(body).not.toHaveProperty('description');
  });

  it("edits a plan's conditions: removes a limit and a condition", async () => {
    const plan = aSubscriptionPlan({
      conditions: [requestsCondition, customCondition],
      displayName: 'Gold',
      id: 'gold',
    });
    server.use(accepts('put', `${PLANS}/gold`, plan, { record: requests }));
    const { user } = setup({ plan });

    await user.click(screen.getByRole('button', { name: 'Show Request count settings' }));
    await user.click(
      within(card('Request count')).getAllByRole('button', { name: 'Remove limit' })[1],
    );
    await user.click(screen.getByRole('button', { name: 'Remove Custom condition' }));
    await user.type(screen.getByLabelText(/Description/), 'Now with support');
    await user.click(screen.getByRole('button', { name: 'Save changes' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    const body = JSON.parse(requests.last()!.body);
    expect(body.description).toBe('Now with support');
    expect(body.conditions).toEqual([
      {
        ...requestsCondition,
        params: { ...requestsCondition.params, limits: [requestsCondition.params.limits[0]] },
      },
    ]);
  });

  it('keeps the last limit of a condition: remove the condition instead', async () => {
    const { user } = setup();

    await addCondition(user, 'Event count');
    expect(within(card('Event count')).getByRole('button', { name: 'Remove limit' })).toBeDisabled();
    await user.click(screen.getByRole('button', { name: 'Remove Event count condition' }));
    expect(screen.getByText('No conditions: this plan is unlimited.')).toBeInTheDocument();
  });

  it('stops adding conditions at 20, and limits at 10', async () => {
    const conditions = [
      {
        id: 'requests',
        params: {
          limits: Array.from({ length: 10 }, (_, index) => ({
            count: 10,
            timeAmount: index + 1,
            timeUnit: 'MINUTE' as const,
          })),
          onExceed: 'reject',
        },
        type: 'request-count' as const,
      },
      ...Array.from({ length: 19 }, (_, index) => ({
        id: `custom-${index}`,
        params: {},
        type: 'custom' as const,
      })),
    ];
    const { user } = setup({ plan: aSubscriptionPlan({ conditions }) });

    expect(screen.getByRole('button', { name: 'Add condition' })).toBeDisabled();
    await user.click(screen.getByRole('button', { name: 'Show Request count settings' }));
    expect(within(card('Request count')).getByRole('button', { name: 'Add limit' })).toBeDisabled();
  });

  it('locks the name and conditions of a plan in use, and still saves its description and expiry', async () => {
    const plan = aSubscriptionPlan({
      conditions: [requestsCondition],
      displayName: 'Gold',
      id: 'gold',
    });
    server.use(accepts('put', `${PLANS}/gold`, plan, { record: requests }));
    const { user } = setup({
      plan,
      usage: { ...UNUSED, inUse: true, restApis: 1, subscriptions: 2 },
    });

    expect(
      await screen.findByText(
        'This plan is in use. To change its name or conditions, create a new plan.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText(/Plan name/)).toBeDisabled();
    expect(screen.queryByRole('button', { name: 'Add condition' })).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Remove Request count condition' }),
    ).not.toBeInTheDocument();
    // Its conditions can still be looked at.
    await user.click(screen.getByRole('button', { name: 'Show Request count settings' }));
    for (const input of within(card('Request count')).getAllByLabelText('Count')) {
      expect(input).toBeDisabled();
    }
    expect(screen.queryByRole('button', { name: 'Remove limit' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Add limit' })).not.toBeInTheDocument();

    await user.type(screen.getByLabelText(/Description/), 'Now with support');
    await user.click(screen.getByRole('button', { name: 'Save changes' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    const body = JSON.parse(requests.last()!.body);
    expect(body).toMatchObject({ description: 'Now with support', displayName: 'Gold' });
    // Omitted means "keep": the plan keeps its conditions.
    expect(body).not.toHaveProperty('conditions');
    expect(body).not.toHaveProperty('limits');
  });
});
