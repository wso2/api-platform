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
  aMeter,
  aPlanPricing,
  aSubscriptionPlan,
  collection,
  failure,
  recorder,
  type Recorder,
} from '@/test/msw';
import { server } from '@/test/server';
import { makeConsoleScope } from '@/test/mockScope';
import { renderWithProviders, screen, userEvent, waitFor, within } from '@/test/utils';
import { MetersSettingsPage } from './MetersSettingsPage';

const ORG = 'api-platform-demo';

/** Adds up the input tokens of AI API requests; its prices may filter on the API type and the HTTP method. */
const llmInput = aMeter({
  displayName: 'LLM input tokens',
  filterFields: [
    { eventKey: 'metadata.apiType', field: 'API_TYPE' },
    { eventKey: 'request.verb', field: 'HTTP_METHOD' },
  ],
  id: 'llm-input',
  type: 'TOKENS',
  typeKey: 'metadata.aiTokenUsage.promptTokens',
});

let requests: Recorder;

type User = ReturnType<typeof userEvent.setup>;

const last = <T,>(items: T[]): T => items[items.length - 1];

/** The keys of the meter dialog, in order: a Tokens meter's Token key first, then each filter row's. */
const eventKeys = () =>
  [
    ...screen.queryAllByRole('combobox', { name: /^Token key/ }),
    ...screen.queryAllByRole('combobox', { name: 'Key' }),
    ...screen.queryAllByRole('textbox', { name: 'Key' }),
  ].map((input) => (input as HTMLInputElement).value);

/** A Tokens meter's Token key, under its type: a combo box of our gateway's token keys that takes any other key too. */
const tokenKey = () => screen.getByRole('combobox', { name: /^Token key/ });

/** The key input of the filter row on `field`, in the rows' order. */
const filterKey = (field: string) => {
  const rows = screen.getAllByRole('combobox', { name: 'Property' });
  const keys = screen.getAllByRole('textbox', { name: 'Key' });
  return keys[
    keys.length - rows.length + rows.findIndex((row) => row.textContent?.includes(field))
  ];
};

/** The Token key field, which only a Tokens meter has. */
const tokenKeyField = () => screen.queryByRole('combobox', { name: /^Token key/ });

/** Adds a filter row to the meter dialog and picks its field. */
async function addFilterField(user: User, field: string | RegExp) {
  await user.click(screen.getByRole('button', { name: 'Add filter' }));
  await user.click(last(screen.getAllByRole('combobox', { name: 'Property' })));
  await user.click(await screen.findByRole('option', { name: field }));
}

function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <MetersSettingsPage />
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/settings/meters`,
      scope: makeConsoleScope(),
    },
  );
}

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('MetersSettingsPage', () => {
  it('lists meters with their type and the fields their prices can filter on', async () => {
    server.use(
      collection('/meters', [
        aMeter(),
        llmInput,
        aMeter({ displayName: 'All calls', filterFields: [], id: 'all-calls' }),
      ]),
      collection('/subscription-plans', [
        aSubscriptionPlan({ displayName: 'Gold', pricing: aPlanPricing() }),
        aSubscriptionPlan({ displayName: 'Bronze', id: 'bronze' }),
      ]),
    );
    renderPage();

    const row = (await screen.findByText('Successful calls')).closest('tr')!;
    expect(screen.getAllByRole('columnheader').map((header) => header.textContent)).toEqual([
      'Name',
      'Type',
      'Filters',
      'Actions',
    ]);
    expect(within(row).getByText('API requests')).toBeInTheDocument();
    expect(within(row).getByText('Status code, HTTP method, Resource')).toBeInTheDocument();
    const tokens = screen.getByText('LLM input tokens').closest('tr')!;
    // The meter's name says which tokens it adds up.
    expect(within(tokens).getByText('Tokens')).toBeInTheDocument();
    // In the meter's order.
    expect(within(tokens).getByText('API type, HTTP method')).toBeInTheDocument();
    const all = screen.getByText('All calls').closest('tr')!;
    expect(within(all).getByText('None')).toBeInTheDocument();
  });

  it('creates a meter of API requests, with no conditions', async () => {
    server.use(
      collection('/meters', [aMeter()]),
      collection('/subscription-plans', []),
      accepts('post', '/meters', aMeter({ id: 'all-calls' }), { record: requests }),
    );
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Create meter' }));
    expect(screen.queryByRole('button', { name: 'Add condition' })).not.toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: /^Event/ })).not.toBeInTheDocument();
    await user.type(screen.getByLabelText(/Name/), 'All calls');
    await user.type(screen.getByLabelText('Description'), 'Every call the gateway allows.');
    await user.click(screen.getByRole('button', { name: 'Add Meter' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toEqual({
      description: 'Every call the gateway allows.',
      displayName: 'All calls',
      filterFields: [],
      type: 'REQUEST_COUNT',
    });
  });

  it("gives a token meter a Token key under its type, required, pre-filled and saved as the meter's event key", async () => {
    server.use(
      collection('/meters', [aMeter()]),
      collection('/subscription-plans', []),
      accepts('post', '/meters', aMeter({ id: 'llm-output' }), { record: requests }),
    );
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Create meter' }));
    expect(tokenKeyField()).not.toBeInTheDocument();
    expect(eventKeys()).toEqual([]);
    await user.type(screen.getByLabelText(/Name/), 'LLM output tokens');
    await user.click(screen.getByRole('combobox', { name: 'Type' }));
    await user.click(await screen.findByRole('option', { name: /Tokens/ }));
    // No event to pick: the Token key, under the type, is what the meter adds up.
    expect(screen.queryByRole('combobox', { name: 'Event' })).not.toBeInTheDocument();
    expect(tokenKeyField()).toBeInTheDocument();
    // It is not a filter: the filters stay empty.
    expect(
      within(screen.getByRole('group', { name: 'Filters' })).queryAllByRole('combobox'),
    ).toHaveLength(0);
    expect(screen.queryByRole('button', { name: /^Remove/ })).not.toBeInTheDocument();
    expect(eventKeys()).toEqual(['metadata.aiTokenUsage.promptTokens']);

    // It is required.
    const key = tokenKey();
    await user.clear(key);
    await user.click(screen.getByRole('button', { name: 'Add Meter' }));
    expect(await screen.findByText('Enter the event key to add up.')).toBeInTheDocument();
    // No spaces.
    await user.type(key, 'metadata.ai token usage');
    expect(await screen.findByText('Use 1 to 255 characters with no spaces.')).toBeInTheDocument();
    expect(requests.count()).toBe(0);

    // Any other key is taken as typed, trimmed.
    await user.clear(key);
    await user.type(key, '  usage.output_tokens ');
    await user.keyboard('{Escape}');
    await user.click(screen.getByRole('button', { name: 'Add Meter' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toEqual({
      displayName: 'LLM output tokens',
      filterFields: [],
      type: 'TOKENS',
      typeKey: 'usage.output_tokens',
    });
  });

  it("offers our gateway's three token keys for the Token key, each with its label, and saves the one chosen", async () => {
    server.use(
      collection('/meters', [aMeter()]),
      collection('/subscription-plans', []),
      accepts('post', '/meters', aMeter({ id: 'llm-output' }), { record: requests }),
    );
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Create meter' }));
    await user.type(screen.getByLabelText(/Name/), 'LLM output tokens');
    await user.click(screen.getByRole('combobox', { name: 'Type' }));
    await user.click(await screen.findByRole('option', { name: /Tokens/ }));
    expect(tokenKey()).toHaveValue('metadata.aiTokenUsage.promptTokens');

    // All three are listed, though the field already holds a key.
    await user.click(tokenKey());
    const options = await screen.findAllByRole('option');
    expect(
      options.map((option) =>
        [...option.querySelectorAll('p, span')].map((part) => part.textContent),
      ),
    ).toEqual([
      ['Prompt tokens', 'metadata.aiTokenUsage.promptTokens'],
      ['Completion tokens', 'metadata.aiTokenUsage.completionTokens'],
      ['Total tokens', 'metadata.aiTokenUsage.totalTokens'],
    ]);
    await user.click(screen.getByRole('option', { name: /^Completion tokens/ }));
    expect(tokenKey()).toHaveValue('metadata.aiTokenUsage.completionTokens');
    await user.click(screen.getByRole('button', { name: 'Add Meter' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toEqual({
      displayName: 'LLM output tokens',
      filterFields: [],
      type: 'TOKENS',
      typeKey: 'metadata.aiTokenUsage.completionTokens',
    });
  });

  it('adds the Token key with the default key when the type becomes Tokens, removes it for API requests, and sends no key then', async () => {
    server.use(
      collection('/meters', [aMeter()]),
      collection('/subscription-plans', []),
      accepts('post', '/meters', aMeter({ id: 'all-calls' }), { record: requests }),
    );
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Create meter' }));
    await user.type(screen.getByLabelText(/Name/), 'All calls');
    const switchTo = async (type: RegExp) => {
      await user.click(screen.getByRole('combobox', { name: 'Type' }));
      await user.click(await screen.findByRole('option', { name: type }));
    };
    await switchTo(/Tokens/);
    await user.clear(tokenKey());
    await user.type(tokenKey(), 'usage.tokens');
    await user.keyboard('{Escape}');
    await switchTo(/^API requests/);
    expect(tokenKeyField()).not.toBeInTheDocument();
    expect(eventKeys()).toEqual([]);
    // Back to Tokens: the Token key returns with the default key.
    await switchTo(/Tokens/);
    expect(tokenKeyField()).toBeInTheDocument();
    expect(eventKeys()).toEqual(['metadata.aiTokenUsage.promptTokens']);
    await switchTo(/^API requests/);
    await user.click(screen.getByRole('button', { name: 'Add Meter' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toEqual({
      displayName: 'All calls',
      filterFields: [],
      type: 'REQUEST_COUNT',
    });
  });

  it('lists the fields its prices can filter on, each with its default event key, each field once', async () => {
    server.use(
      collection('/meters', [aMeter()]),
      collection('/subscription-plans', []),
      accepts('post', '/meters', llmInput, { record: requests }),
    );
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Create meter' }));
    await user.type(screen.getByLabelText(/Name/), 'LLM input tokens');
    // The section is titled Fields, with no hint under it.
    expect(screen.getByRole('group', { name: 'Filters' })).toBeInTheDocument();
    expect(screen.queryByText("Filters this meter's prices can use.")).not.toBeInTheDocument();
    // Column headings come with the first filter row; a Tokens meter's Token key is not one.
    expect(screen.queryByText('Key')).not.toBeInTheDocument();
    await user.click(screen.getByRole('combobox', { name: 'Type' }));
    await user.click(await screen.findByRole('option', { name: /Tokens/ }));
    expect(screen.queryByText('Key')).not.toBeInTheDocument();

    await addFilterField(user, /^API type/);
    expect(screen.getByText('Property')).toBeInTheDocument();
    expect(screen.getByText('Key')).toBeInTheDocument();
    expect(eventKeys()).toEqual(['metadata.aiTokenUsage.promptTokens', 'metadata.apiType']);
    await addFilterField(user, 'HTTP method');
    expect(eventKeys()).toEqual([
      'metadata.aiTokenUsage.promptTokens',
      'metadata.apiType',
      'request.verb',
    ]);
    // Every field is offered, then Token; a fixed field already listed is not offered again.
    await user.click(last(screen.getAllByRole('combobox', { name: 'Property' })));
    expect((await screen.findAllByRole('option')).map((option) => option.textContent)).toEqual([
      'Status code',
      'HTTP method',
      'Resource',
      'API type',
      'MCP method',
      'Tool',
      'Token',
    ]);
    expect(screen.getByRole('option', { name: /^API type/ })).toHaveAttribute(
      'aria-disabled',
      'true',
    );
    await user.keyboard('{Escape}');

    await addFilterField(user, 'Status code');
    await addFilterField(user, 'Resource');
    expect(eventKeys().slice(3)).toEqual(['response.status', 'metadata.apiResourceTemplate']);
    // With every fixed field listed, Token is still offered.
    await user.click(last(screen.getAllByRole('combobox', { name: 'Property' })));
    expect(screen.getByRole('option', { name: /^Token/ })).not.toHaveAttribute(
      'aria-disabled',
      'true',
    );
    await user.keyboard('{Escape}');
    await user.click(screen.getByRole('button', { name: 'Remove Status code' }));
    await user.click(screen.getByRole('button', { name: 'Remove Resource' }));
    await user.click(screen.getByRole('button', { name: 'Add Meter' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toEqual({
      displayName: 'LLM input tokens',
      filterFields: [
        { eventKey: 'metadata.apiType', field: 'API_TYPE' },
        { eventKey: 'request.verb', field: 'HTTP_METHOD' },
      ],
      type: 'TOKENS',
      typeKey: 'metadata.aiTokenUsage.promptTokens',
    });
  });

  it('adds Token fields, each needing its own key from the suggestions or typed, and saves them with their keys', async () => {
    server.use(
      collection('/meters', [aMeter()]),
      collection('/subscription-plans', []),
      accepts('post', '/meters', llmInput, { record: requests }),
    );
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Create meter' }));
    await user.type(screen.getByLabelText(/Name/), 'LLM input tokens');
    await user.click(screen.getByRole('combobox', { name: 'Type' }));
    await user.click(await screen.findByRole('option', { name: /Tokens/ }));

    // A Token field has no default key; its key box is the Token key's combo box.
    await addFilterField(user, /^Token/);
    const [first] = screen.getAllByRole('combobox', { name: 'Key' });
    expect(first).toHaveValue('');
    await user.click(screen.getByRole('button', { name: 'Add Meter' }));
    expect(await screen.findByText('A Token property needs an event key.')).toBeInTheDocument();

    // Not the Token key.
    await user.click(first);
    expect((await screen.findAllByRole('option')).map((option) => option.textContent)).toEqual([
      'Prompt tokensmetadata.aiTokenUsage.promptTokens',
      'Completion tokensmetadata.aiTokenUsage.completionTokens',
      'Total tokensmetadata.aiTokenUsage.totalTokens',
    ]);
    await user.click(screen.getByRole('option', { name: /^Prompt tokens/ }));
    expect(
      await screen.findByText(
        "The key metadata.aiTokenUsage.promptTokens is already the meter's Token key.",
      ),
    ).toBeInTheDocument();
    await user.click(first);
    await user.click(await screen.findByRole('option', { name: /^Total tokens/ }));

    // Token can be added again, but not with the same key.
    await addFilterField(user, /^Token/);
    const [, second] = screen.getAllByRole('combobox', { name: 'Key' });
    await user.type(second, 'metadata.aiTokenUsage.totalTokens');
    await user.keyboard('{Escape}');
    expect(
      await screen.findByText(
        'The meter lists the Token key metadata.aiTokenUsage.totalTokens more than once.',
      ),
    ).toBeInTheDocument();
    expect(requests.count()).toBe(0);
    await user.clear(second);
    await user.type(second, 'usage.cachedTokens');
    await user.keyboard('{Escape}');
    expect(
      screen.getByRole('button', { name: 'Remove Token · usage.cachedTokens' }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Add Meter' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toEqual({
      displayName: 'LLM input tokens',
      filterFields: [
        { eventKey: 'metadata.aiTokenUsage.totalTokens', field: 'TOKEN' },
        { eventKey: 'usage.cachedTokens', field: 'TOKEN' },
      ],
      type: 'TOKENS',
      typeKey: 'metadata.aiTokenUsage.promptTokens',
    });
  });

  it('offers Token on a meter of API requests too, and stops adding fields at ten', async () => {
    server.use(collection('/meters', [aMeter()]), collection('/subscription-plans', []));
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Create meter' }));
    // The six fixed fields come first, then Token rows.
    for (let count = 0; count < 10; count++)
      await user.click(screen.getByRole('button', { name: 'Add filter' }));
    expect(
      screen.getAllByRole('combobox', { name: 'Property' }).map((select) => select.textContent),
    ).toEqual([
      'Status code',
      'HTTP method',
      'Resource',
      'API type',
      'MCP method',
      'Tool',
      'Token',
      'Token',
      'Token',
      'Token',
    ]);
    expect(screen.queryByRole('button', { name: 'Add filter' })).not.toBeInTheDocument();
    await user.click(screen.getAllByRole('button', { name: 'Remove Token' })[0]);
    expect(screen.getByRole('button', { name: 'Add filter' })).toBeInTheDocument();
  });

  it("names a meter's Token fields by their keys in the list", async () => {
    server.use(
      collection('/meters', [
        aMeter({
          filterFields: [
            { eventKey: 'request.verb', field: 'HTTP_METHOD' },
            { eventKey: 'metadata.aiTokenUsage.totalTokens', field: 'TOKEN' },
            { eventKey: 'usage.cachedTokens', field: 'TOKEN' },
          ],
        }),
      ]),
      collection('/subscription-plans', []),
    );
    renderPage();

    expect(
      await screen.findByText('HTTP method, Token · Total tokens, Token · usage.cachedTokens'),
    ).toBeInTheDocument();
  });

  it("edits a filter row's event key, refills it with the new field's default when the field changes, and leaves an emptied key to its default", async () => {
    server.use(
      collection('/meters', [aMeter()]),
      collection('/subscription-plans', []),
      accepts('post', '/meters', aMeter({ id: 'other-gateway-calls' }), { record: requests }),
    );
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Create meter' }));
    await user.type(screen.getByLabelText(/Name/), 'Other gateway calls');
    await addFilterField(user, 'Status code');
    await user.clear(filterKey('Status code'));
    await user.type(filterKey('Status code'), 'metadata.statusCode');
    expect(filterKey('Status code')).toHaveValue('metadata.statusCode');

    // Another field reads its own default, whatever the row's key was.
    await user.click(screen.getByRole('combobox', { name: 'Property' }));
    await user.click(await screen.findByRole('option', { name: 'HTTP method' }));
    expect(filterKey('HTTP method')).toHaveValue('request.verb');
    await user.clear(filterKey('HTTP method'));
    await user.type(filterKey('HTTP method'), 'metadata.method');

    // A key with a space is refused here; an emptied one is left to the server's default.
    await addFilterField(user, 'Resource');
    await user.clear(filterKey('Resource'));
    await user.type(filterKey('Resource'), 'metadata.api resource');
    await user.click(screen.getByRole('button', { name: 'Add Meter' }));
    expect(await screen.findByText('Use 1 to 255 characters with no spaces.')).toBeInTheDocument();
    expect(requests.count()).toBe(0);
    await user.clear(filterKey('Resource'));
    expect(filterKey('Resource')).toHaveAttribute('placeholder', 'metadata.apiResourceTemplate');
    await user.click(screen.getByRole('button', { name: 'Add Meter' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toEqual({
      displayName: 'Other gateway calls',
      filterFields: [{ eventKey: 'metadata.method', field: 'HTTP_METHOD' }, { field: 'RESOURCE' }],
      type: 'REQUEST_COUNT',
    });
  });

  it('opens a meter with its stored event keys, and saves them back as edited', async () => {
    // The keys come from the meter as the Platform API stores it; this one reads another gateway's
    // fields.
    const meter = aMeter({
      filterFields: [{ eventKey: 'api.kind', field: 'API_TYPE' }],
      type: 'TOKENS',
      typeKey: 'usage.inputTokens',
    });
    server.use(
      collection('/meters', [meter]),
      collection('/subscription-plans', []),
      accepts('put', '/meters/successful-calls', meter, { record: requests }),
    );
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Edit Successful calls' }));
    expect(screen.getByRole('combobox', { name: 'Type' })).toHaveTextContent('Tokens');
    expect(screen.getByRole('combobox', { name: 'Property' })).toHaveTextContent('API type');
    expect(eventKeys()).toEqual(['usage.inputTokens', 'api.kind']);
    await user.clear(tokenKey());
    await user.type(tokenKey(), 'usage.outputTokens');
    await user.keyboard('{Escape}');
    await user.click(screen.getByRole('button', { name: 'Save changes' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toEqual({
      displayName: 'Successful calls',
      filterFields: [{ eventKey: 'api.kind', field: 'API_TYPE' }],
      type: 'TOKENS',
      typeKey: 'usage.outputTokens',
    });
  });

  it("shows the server's reason for refusing a new meter", async () => {
    const message = 'The event key of API type must be 1 to 255 characters with no spaces';
    server.use(
      collection('/meters', [aMeter()]),
      collection('/subscription-plans', []),
      failure('post', '/meters', 400, 'VALIDATION_FAILED', { message, record: requests }),
    );
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Create meter' }));
    await user.type(screen.getByLabelText(/Name/), 'Typed calls');
    await addFilterField(user, /^API type/);
    await user.click(screen.getByRole('button', { name: 'Add Meter' }));

    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(requests.count()).toBe(1);
    expect(screen.getByRole('button', { name: 'Add Meter' })).toBeInTheDocument();
  });

  it("shows the Platform API's own reason for refusing to delete a meter a plan there still prices", async () => {
    const message = "The meter is in use, so it can't be deleted.";
    server.use(
      collection('/meters', [aMeter()]),
      collection('/subscription-plans', []),
      failure('delete', '/meters/successful-calls', 409, 'METER_IN_USE', {
        message,
        record: requests,
      }),
    );
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Delete Successful calls' }));
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Delete' }));

    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(requests.count()).toBe(1);
  });

  it('says a priced meter is in use instead of deleting it, without reading its plans, and sends no delete', async () => {
    server.use(
      // The Platform API counts the plans that charge for the meter.
      collection('/meters', [aMeter({ planCount: 2 })]),
      accepts('delete', '/meters/successful-calls', null, { record: requests }),
    );
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Delete Successful calls' }));

    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByText('Delete meter')).toBeInTheDocument();
    expect(
      within(dialog).getByText('"Successful calls" is in use and can\'t be deleted.'),
    ).toBeInTheDocument();
    // The usual footer, with Delete disabled.
    expect(within(dialog).getByRole('button', { name: 'Delete' })).toBeDisabled();
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(requests.count()).toBe(0);
  });

  it('opens a meter a plan prices with every field read-only and Save off, saying why', async () => {
    server.use(
      collection('/meters', [
        aMeter({ description: 'Calls the gateway allows.', planCount: 1 }),
        llmInput,
      ]),
    );
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    // Every meter has Edit, whether plans price it or not.
    const edit = await screen.findByRole('button', { name: 'Edit Successful calls' });
    expect(screen.getByRole('button', { name: 'Edit LLM input tokens' })).toBeInTheDocument();
    await user.click(edit);

    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByText('Edit meter')).toBeInTheDocument();
    expect(within(dialog).getByLabelText(/Name/)).toBeDisabled();
    expect(within(dialog).getByLabelText(/Name/)).toHaveValue('Successful calls');
    expect(within(dialog).getByLabelText('Description')).toBeDisabled();
    expect(within(dialog).getByRole('combobox', { name: 'Type' })).toHaveAttribute(
      'aria-disabled',
      'true',
    );
    for (const filter of within(dialog).getAllByRole('combobox', { name: 'Property' })) {
      expect(filter).toHaveAttribute('aria-disabled', 'true');
    }
    // Its keys too.
    const keys = within(dialog).getAllByRole('textbox', { name: 'Key' });
    expect(keys.map((input) => (input as HTMLInputElement).value)).toEqual([
      'response.status',
      'request.verb',
      'metadata.apiResourceTemplate',
    ]);
    for (const key of keys) expect(key).toBeDisabled();
    expect(within(dialog).queryByRole('button', { name: 'Add filter' })).not.toBeInTheDocument();
    expect(within(dialog).queryByRole('button', { name: /^Remove/ })).not.toBeInTheDocument();
    // Save is there but off, and says why.
    const save = within(dialog).getByRole('button', { name: 'Save changes' });
    expect(save).toBeDisabled();
    expect(save).toHaveAccessibleDescription('There are plans using this meter.');
    expect(within(dialog).getByText('There are plans using this meter.')).toBeInTheDocument();

    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('opens a token meter a plan prices read-only, its Token key included', async () => {
    server.use(collection('/meters', [{ ...llmInput, planCount: 1 }]));
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Edit LLM input tokens' }));

    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByRole('combobox', { name: /^Token key/ })).toBeDisabled();
    expect(eventKeys()).toEqual([
      'metadata.aiTokenUsage.promptTokens',
      'metadata.apiType',
      'request.verb',
    ]);
    for (const key of within(dialog).getAllByRole('textbox', { name: 'Key' }))
      expect(key).toBeDisabled();
    expect(within(dialog).queryByRole('button', { name: /^Remove/ })).not.toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Save changes' })).toBeDisabled();
  });

  it("shows the server's reason when a change to a meter a plan prices is refused anyway", async () => {
    const message = "The meter is in use, so it can't be updated.";
    // The plans loaded here don't price it yet; the server knows better.
    server.use(
      collection('/meters', [llmInput]),
      collection('/subscription-plans', []),
      failure('put', '/meters/llm-input', 409, 'METER_IN_USE', { message, record: requests }),
    );
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Edit LLM input tokens' }));
    await user.click(screen.getByRole('button', { name: 'Remove API type' }));
    await user.click(screen.getByRole('button', { name: 'Save changes' }));

    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(JSON.parse(requests.last()!.body).filterFields).toEqual([
      { eventKey: 'request.verb', field: 'HTTP_METHOD' },
    ]);
    // The dialog stays open with the change.
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeInTheDocument();
  });

  it('pages the meters and searches them on the Platform API', async () => {
    const meters = Array.from({ length: 7 }, (_, i) =>
      aMeter({ displayName: `Meter ${i + 1}`, id: `meter-${i + 1}` }),
    );
    server.use(collection('/meters', [...meters, llmInput], { record: requests }));
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    expect(await screen.findByText('Meter 1')).toBeInTheDocument();
    // Five a page by default, with the pager.
    expect(screen.queryByText('Meter 6')).not.toBeInTheDocument();
    expect(screen.getByText('1–5 of 8')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Next page' }));
    expect(await screen.findByText('Meter 6')).toBeInTheDocument();

    await user.type(screen.getByPlaceholderText('Search meters'), 'llm');
    expect(await screen.findByText('LLM input tokens')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByText('Meter 6')).not.toBeInTheDocument());
    expect(requests.last()?.params.get('query')).toBe('llm');

    await user.clear(screen.getByPlaceholderText('Search meters'));
    await user.type(screen.getByPlaceholderText('Search meters'), 'nothing like it');
    expect(await screen.findByText('No matching meters')).toBeInTheDocument();
  });
});
