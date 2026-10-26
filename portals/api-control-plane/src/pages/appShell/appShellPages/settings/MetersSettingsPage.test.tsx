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
    { eventKey: 'metadata.apiType', field: 'API type' },
    { eventKey: 'request.verb', field: 'HTTP method' },
  ],
  id: 'llm-input',
  type: 'TOKENS',
  typeKey: 'metadata.aiTokenUsage.promptTokens',
});

let requests: Recorder;

type User = ReturnType<typeof userEvent.setup>;

const valueOf = (input: HTMLElement) => (input as HTMLInputElement).value;

/** The keys of the meter dialog, in order: a Tokens meter's Token key first, then each property's. */
const eventKeys = () =>
  [
    ...screen.queryAllByRole('combobox', { name: /^Token key/ }),
    ...screen.queryAllByRole('textbox', { name: 'Key' }),
  ].map(valueOf);

/** The names of the meter's properties, in order. */
const propertyNames = () => screen.queryAllByRole('textbox', { name: 'Property' }).map(valueOf);

/** A Tokens meter's Token key, under its type: a combo box of our gateway's token keys that takes any other key too. */
const tokenKey = () => screen.getByRole('combobox', { name: /^Token key/ });

/** The name and key inputs of the property at `index`, counting from 0. */
const propertyAt = (index: number) => ({
  key: screen.getAllByRole('textbox', { name: 'Key' })[index],
  name: screen.getAllByRole('textbox', { name: 'Property' })[index],
});

/** The Token key field, which only a Tokens meter has. */
const tokenKeyField = () => screen.queryByRole('combobox', { name: /^Token key/ });

/** What Add filter offers, each our gateway's property by its name and key, then Custom. */
async function offered(user: User) {
  await user.click(screen.getByRole('button', { name: 'Add filter' }));
  const items = (await screen.findAllByRole('menuitem')).map((item) => item.textContent);
  await user.keyboard('{Escape}');
  return items;
}

/** Adds a property to the meter dialog: one of our gateway's, by its name, or Custom. */
async function addProperty(user: User, name: string | RegExp) {
  await user.click(screen.getByRole('button', { name: 'Add filter' }));
  await user.click(await screen.findByRole('menuitem', { name }));
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
  it('lists meters with their type and the names of the properties their prices can filter on', async () => {
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
    expect(within(row).getByText('Requests')).toBeInTheDocument();
    expect(within(row).getByText('Status code, HTTP method, Resource')).toBeInTheDocument();
    const tokens = screen.getByText('LLM input tokens').closest('tr')!;
    // The meter's name says which tokens it adds up.
    expect(within(tokens).getByText('Tokens')).toBeInTheDocument();
    // In the meter's order.
    expect(within(tokens).getByText('API type, HTTP method')).toBeInTheDocument();
    const all = screen.getByText('All calls').closest('tr')!;
    expect(within(all).getByText('None')).toBeInTheDocument();
  });

  it('creates a meter of requests, with no conditions', async () => {
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

  it('adds the Token key with the default key when the type becomes Tokens, removes it for Requests, and sends no key then', async () => {
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
    await switchTo(/^Requests/);
    expect(tokenKeyField()).not.toBeInTheDocument();
    expect(eventKeys()).toEqual([]);
    // Back to Tokens: the Token key returns with the default key.
    await switchTo(/Tokens/);
    expect(tokenKeyField()).toBeInTheDocument();
    expect(eventKeys()).toEqual(['metadata.aiTokenUsage.promptTokens']);
    await switchTo(/^Requests/);
    await user.click(screen.getByRole('button', { name: 'Add Meter' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toEqual({
      displayName: 'All calls',
      filterFields: [],
      type: 'REQUEST_COUNT',
    });
  });

  it("offers our gateway's properties with their names and keys, then Custom, and not again once the meter has one's name or key", async () => {
    server.use(
      collection('/meters', [aMeter()]),
      collection('/subscription-plans', []),
      accepts('post', '/meters', llmInput, { record: requests }),
    );
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Create meter' }));
    await user.type(screen.getByLabelText(/Name/), 'LLM input tokens');
    expect(screen.getByRole('group', { name: 'Filters' })).toBeInTheDocument();
    // Column headings come with the first property; a Tokens meter's Token key is not one.
    expect(screen.queryByText('Key')).not.toBeInTheDocument();
    await user.click(screen.getByRole('combobox', { name: 'Type' }));
    await user.click(await screen.findByRole('option', { name: /Tokens/ }));
    expect(screen.queryByText('Key')).not.toBeInTheDocument();

    expect(await offered(user)).toEqual([
      'Status coderesponse.status',
      'HTTP methodrequest.verb',
      'Resourcemetadata.apiResourceTemplate',
      'API typemetadata.apiType',
      'MCP methodmetadata.mcpAnalytics.jsonRpcMethod',
      'Toolmetadata.mcpAnalytics.capabilityName',
      'Prompt tokensmetadata.aiTokenUsage.promptTokens',
      'Completion tokensmetadata.aiTokenUsage.completionTokens',
      'Total tokensmetadata.aiTokenUsage.totalTokens',
      'Custom',
    ]);
    await addProperty(user, /^API type/);
    expect(screen.getByText('Property')).toBeInTheDocument();
    expect(screen.getByText('Key')).toBeInTheDocument();
    expect(propertyNames()).toEqual(['API type']);
    expect(eventKeys()).toEqual(['metadata.aiTokenUsage.promptTokens', 'metadata.apiType']);
    // A property may read the meter's own Token key, so prices can compare the number it adds up.
    await addProperty(user, /^Prompt tokens/);
    expect(propertyNames()).toEqual(['API type', 'Prompt tokens']);
    expect(eventKeys().slice(1)).toEqual([
      'metadata.apiType',
      'metadata.aiTokenUsage.promptTokens',
    ]);

    // Not offered while the meter has its key, though under another name.
    const apiType = propertyAt(0);
    await user.clear(apiType.name);
    await user.type(apiType.name, 'Kind');
    expect(await offered(user)).not.toContain('API typemetadata.apiType');
    // Nor while it has its name, whatever the case, though with another key.
    await user.clear(apiType.name);
    await user.type(apiType.name, 'api TYPE');
    await user.clear(apiType.key);
    await user.type(apiType.key, 'api.kind');
    expect(await offered(user)).not.toContain('API typemetadata.apiType');
    // Offered again once the meter has neither.
    await user.clear(apiType.name);
    await user.type(apiType.name, 'Kind');
    expect(await offered(user)).toContain('API typemetadata.apiType');
    await user.click(screen.getByRole('button', { name: 'Add Meter' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toEqual({
      displayName: 'LLM input tokens',
      filterFields: [
        { eventKey: 'api.kind', field: 'Kind' },
        { eventKey: 'metadata.aiTokenUsage.promptTokens', field: 'Prompt tokens' },
      ],
      type: 'TOKENS',
      typeKey: 'metadata.aiTokenUsage.promptTokens',
    });
  });

  it('adds a custom property with an empty name and key, each needed, and refuses a name or key the meter repeats', async () => {
    server.use(
      collection('/meters', [aMeter()]),
      collection('/subscription-plans', []),
      accepts('post', '/meters', aMeter({ id: 'regional-calls' }), { record: requests }),
    );
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Create meter' }));
    await user.type(screen.getByLabelText(/Name/), 'Regional calls');
    await addProperty(user, 'Custom');
    expect(propertyNames()).toEqual(['']);
    expect(eventKeys()).toEqual(['']);
    expect(screen.getByRole('button', { name: 'Remove this property' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Add Meter' }));
    expect(
      await screen.findByText('Give each property a name of 1 to 40 characters.'),
    ).toBeInTheDocument();
    expect(screen.getByText('Enter the event key of each property.')).toBeInTheDocument();

    const region = propertyAt(0);
    await user.click(region.name);
    await user.paste('x'.repeat(41));
    await user.type(region.key, 'metadata region');
    expect(
      screen.getByText('Give each property a name of 1 to 40 characters.'),
    ).toBeInTheDocument();
    expect(screen.getByText('Use 1 to 255 characters with no spaces.')).toBeInTheDocument();
    await user.clear(region.name);
    await user.type(region.name, ' Region ');
    await user.clear(region.key);
    await user.type(region.key, 'metadata.region');
    expect(screen.getByRole('button', { name: 'Remove Region' })).toBeInTheDocument();

    // A name is the same whatever its case; a key is the same as written.
    await addProperty(user, 'Custom');
    const zone = propertyAt(1);
    await user.type(zone.name, 'region');
    await user.type(zone.key, 'metadata.region');
    expect(
      screen.getByText('The meter has the property Region more than once.'),
    ).toBeInTheDocument();
    expect(
      screen.getByText('More than one property reads the event key metadata.region.'),
    ).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Add Meter' }));
    expect(requests.count()).toBe(0);

    await user.clear(zone.name);
    await user.type(zone.name, 'Zone');
    await user.clear(zone.key);
    await user.type(zone.key, 'metadata.Region');
    await user.click(screen.getByRole('button', { name: 'Add Meter' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toEqual({
      displayName: 'Regional calls',
      filterFields: [
        { eventKey: 'metadata.region', field: 'Region' },
        { eventKey: 'metadata.Region', field: 'Zone' },
      ],
      type: 'REQUEST_COUNT',
    });
  });

  it('stops adding properties at ten, offering only Custom once every one of gatewayProperties is in', async () => {
    const gatewayProperties = [
      ['Status code', 'response.status'],
      ['HTTP method', 'request.verb'],
      ['Resource', 'metadata.apiResourceTemplate'],
      ['API type', 'metadata.apiType'],
      ['MCP method', 'metadata.mcpAnalytics.jsonRpcMethod'],
      ['Tool', 'metadata.mcpAnalytics.capabilityName'],
      ['Prompt tokens', 'metadata.aiTokenUsage.promptTokens'],
      ['Completion tokens', 'metadata.aiTokenUsage.completionTokens'],
      ['Total tokens', 'metadata.aiTokenUsage.totalTokens'],
    ];
    server.use(
      collection('/meters', [
        aMeter({ filterFields: gatewayProperties.map(([field, eventKey]) => ({ eventKey, field })) }),
      ]),
      collection('/subscription-plans', []),
    );
    renderPage();
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(await screen.findByRole('button', { name: 'Edit Successful calls' }));
    expect(propertyNames()).toEqual(gatewayProperties.map(([field]) => field));
    expect(await offered(user)).toEqual(['Custom']);
    await addProperty(user, 'Custom');
    expect(screen.queryByRole('button', { name: 'Add filter' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Remove Tool' }));
    expect(screen.getByRole('button', { name: 'Add filter' })).toBeInTheDocument();
  });

  it('opens a meter with its stored properties, and saves them back as edited', async () => {
    // The keys come from the meter as the Platform API stores it; this one reads another gateway's
    // fields.
    const meter = aMeter({
      filterFields: [{ eventKey: 'api.kind', field: 'Kind' }],
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
    expect(propertyNames()).toEqual(['Kind']);
    expect(eventKeys()).toEqual(['usage.inputTokens', 'api.kind']);
    await user.clear(tokenKey());
    await user.type(tokenKey(), 'usage.outputTokens');
    await user.keyboard('{Escape}');
    await user.type(propertyAt(0).name, ' of API');
    await user.click(screen.getByRole('button', { name: 'Save changes' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toEqual({
      displayName: 'Successful calls',
      filterFields: [{ eventKey: 'api.kind', field: 'Kind of API' }],
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
    await addProperty(user, /^API type/);
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
    const names = within(dialog).getAllByRole('textbox', { name: 'Property' });
    expect(names.map(valueOf)).toEqual(['Status code', 'HTTP method', 'Resource']);
    for (const name of names) expect(name).toBeDisabled();
    // Its keys too.
    const keys = within(dialog).getAllByRole('textbox', { name: 'Key' });
    expect(keys.map(valueOf)).toEqual([
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
    for (const input of within(dialog).getAllByRole('textbox', { name: /^(Key|Property)$/ }))
      expect(input).toBeDisabled();
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
      { eventKey: 'request.verb', field: 'HTTP method' },
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
