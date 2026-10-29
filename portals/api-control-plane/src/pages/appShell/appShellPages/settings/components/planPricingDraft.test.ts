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
import { describe, expect, it } from 'vitest';

import type { Meter } from '@/api/resources/meters';
import { aMeter } from '@/test/msw';
import {
  buildPricingBody,
  draftFromPricing,
  filterRows,
  freeDraft,
  newPrice,
  newTier,
  priceLabel,
  priceName,
  pricesByName,
  type FilterDraft,
  type PriceDraft,
  type PricingDraft,
} from './planPricingDraft';
import type { FilterOperator } from './meterLabels';

const flatRate = (flatAmount: string, patch: Partial<PricingDraft> = {}): PricingDraft => ({
  ...freeDraft(),
  flatAmount,
  paid: true,
  ...patch,
});
const usageBased = (prices: PriceDraft[]): PricingDraft => ({
  ...freeDraft(),
  paid: true,
  prices,
  pricingType: 'USAGE_BASED',
});
const filter = (field: string, operator: FilterOperator, text: string): FilterDraft => ({
  field,
  operator,
  text,
});
const price = (patch: Partial<PriceDraft> = {}): PriceDraft => ({ ...newPrice(), ...patch });
const perUnit = (meterId: string, filters: FilterDraft[] = [], patch: Partial<PriceDraft> = {}) =>
  price({ filters, meterId, unitAmount: '0.01', ...patch });
const tier = (upTo: string, unitAmount: string, flatAmount = '') => ({
  ...newTier(),
  flatAmount,
  unitAmount,
  upTo,
});
/**
 * Without meters, no price's meter is known, so the properties it filters on are left to the
 * server.
 */
const build = (draft: PricingDraft, meters: Meter[] = []) => buildPricingBody(draft, meters);
/** The problem with two prices on the meter "calls", given each price's filters. */
const pair = (first: FilterDraft[], second: FilterDraft[]) =>
  build(usageBased([perUnit('calls', first), perUnit('calls', second)])).problem;
const OVERLAP = { code: 'OVERLAP', meterId: 'calls', other: 1, price: 0 };

describe('buildPricingBody', () => {
  it('sends nothing for a free plan', () => {
    expect(build(freeDraft())).toEqual({ body: null, problem: null });
  });

  it('builds a flat rate plan with its amount and no prices', () => {
    // Prices left from choosing usage based first are not sent.
    expect(
      build(flatRate(' 20.00 ', { billingPeriod: 'YEAR', prices: [perUnit('calls')] })),
    ).toEqual({
      body: {
        billingPeriod: 'YEAR',
        currency: 'USD',
        flatAmount: '20.00',
        pricingType: 'FLAT_RATE',
      },
      problem: null,
    });
  });

  it('needs a flat rate above zero', () => {
    expect(build(flatRate('')).problem).toEqual({ code: 'FLAT_AMOUNT_INVALID' });
    expect(build(flatRate('9.9.9')).problem).toEqual({ code: 'FLAT_AMOUNT_INVALID' });
    expect(build(flatRate('0.00')).problem).toEqual({ code: 'FLAT_AMOUNT_INVALID' });
  });

  it('builds a usage based plan whose prices carry their filters, the same meter more than once', () => {
    const draft = usageBased([
      perUnit('llm-input', [filter('API type', 'EQUALS', 'LlmProvider')], {
        unitAmount: '2.50',
        unitQuantity: '1000000',
      }),
      perUnit('llm-input', [filter('API type', 'IN', 'Mcp, LlmProxy, Mcp')], {
        unitAmount: '0.15',
      }),
      price({
        meterId: 'api-calls',
        pricingModel: 'GRADUATED',
        tiers: [tier('1000', '0'), tier('ignored', '0.001')],
      }),
    ]);
    expect(build(draft)).toEqual({
      body: {
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
            filters: [{ field: 'API type', operator: 'IN', values: ['Mcp', 'LlmProxy'] }],
            meterId: 'llm-input',
            pricingModel: 'PER_UNIT',
            tiers: null,
            unitAmount: '0.15',
            unitQuantity: 1,
          },
          {
            filters: [],
            meterId: 'api-calls',
            pricingModel: 'GRADUATED',
            tiers: [
              { flatAmount: null, unitAmount: '0', upTo: 1000 },
              { flatAmount: null, unitAmount: '0.001', upTo: null },
            ],
            unitAmount: null,
            unitQuantity: null,
          },
        ],
        pricingType: 'USAGE_BASED',
      },
      problem: null,
    });
  });

  it('needs at least one price', () => {
    expect(build(usageBased([])).problem).toEqual({ code: 'NO_PRICES' });
  });

  it('needs a meter, valid filter values, then a valid amount and batch size', () => {
    const problem = (entry: PriceDraft) => build(usageBased([entry])).problem;
    expect(problem(price({ unitAmount: '0.01' }))).toEqual({ code: 'METER_MISSING', price: 0 });
    const regionProblem = (operator: FilterOperator, text: string) =>
      problem(perUnit('calls', [filter('Region', operator, text)]));
    // Is one of and is not one of take up to 20 values; every text value is up to 255 characters.
    expect(
      regionProblem('IN', Array.from({ length: 21 }, (_, index) => `eu-${index}`).join(', ')),
    ).toEqual({ code: 'FILTER_INVALID', field: 'Region', price: 0 });
    expect(regionProblem('NOT_IN', `eu-west, ${'x'.repeat(256)}`)).toMatchObject({
      code: 'FILTER_INVALID',
    });
    expect(regionProblem('STARTS_WITH', 'x'.repeat(256))).toMatchObject({ code: 'FILTER_INVALID' });
    expect(problem(perUnit('calls', [], { unitAmount: 'x' }))).toEqual({
      code: 'AMOUNT_INVALID',
      price: 0,
    });
    expect(problem(perUnit('calls', [], { unitQuantity: '0' }))).toEqual({
      code: 'QUANTITY_INVALID',
      price: 0,
    });
  });

  it('takes any text as the one value of equals, does not equal and starts with, as written', () => {
    const { body } = build(
      usageBased([
        perUnit('calls', [
          filter('Resource', 'STARTS_WITH', ' /orders/ '),
          filter('Region', 'EQUALS', 'eu-west, eu-north'),
          filter('Status code', 'NOT_EQUALS', '2xx'),
        ]),
      ]),
    );
    expect(body?.prices?.[0].filters).toEqual([
      // A trailing "/" is kept.
      { field: 'Resource', operator: 'STARTS_WITH', values: ['/orders/'] },
      { field: 'Region', operator: 'EQUALS', values: ['eu-west, eu-north'] },
      { field: 'Status code', operator: 'NOT_EQUALS', values: ['2xx'] },
    ]);
  });

  it("sends no filter for a row left empty, and the others in the order of the meter's properties, as the meter spells them", () => {
    // The meter's properties are Status code, HTTP method and Resource.
    const draft = usageBased([
      perUnit('successful-calls', [
        filter('Resource', 'STARTS_WITH', ' '),
        filter('http METHOD', 'NOT_EQUALS', 'GET'),
        filter('status code', 'IN', '200'),
      ]),
    ]);
    expect(build(draft, [aMeter()]).body?.prices?.[0].filters).toEqual([
      { field: 'Status code', operator: 'IN', values: ['200'] },
      { field: 'HTTP method', operator: 'NOT_EQUALS', values: ['GET'] },
    ]);
    // Without the meter, the rows go in the order given, as given.
    expect(build(draft).body?.prices?.[0].filters).toEqual([
      { field: 'http METHOD', operator: 'NOT_EQUALS', values: ['GET'] },
      { field: 'status code', operator: 'IN', values: ['200'] },
    ]);
  });

  it('refuses tiers whose bounds do not increase or that charge nothing', () => {
    const volume = (tiers: PriceDraft['tiers']) =>
      usageBased([price({ meterId: 'calls', pricingModel: 'VOLUME', tiers })]);
    const problem = { code: 'TIERS_INVALID', price: 0 };
    expect(
      build(volume([tier('100', '0.01'), tier('50', '0.02'), tier('', '0.03')])).problem,
    ).toEqual(problem);
    expect(build(volume([tier('', '0.01'), tier('', '0.02')])).problem).toEqual(problem);
    expect(build(volume([tier('100', '0.01'), tier('', '')])).problem).toEqual(problem);
    expect(build(volume([])).problem).toEqual(problem);
  });
});

describe('buildPricingBody property check', () => {
  const llmInput = aMeter({
    filterFields: [{ eventKey: 'metadata.apiType', field: 'API type' }],
    id: 'llm-input',
    type: 'TOKENS',
    typeKey: 'metadata.aiTokenUsage.promptTokens',
  });
  const plain = aMeter({ filterFields: [], id: 'plain-calls' });

  it("names the first price that filters on a property its meter doesn't have, before its values; an empty row is no filter", () => {
    const draft = usageBased([
      perUnit('llm-input', [filter('API type', 'EQUALS', 'LlmProvider')]),
      perUnit('llm-input', [
        filter('API type', 'EQUALS', 'Agent'),
        filter('HTTP method', 'EQUALS', ''),
      ]),
      perUnit('llm-input', [
        filter('API type', 'EQUALS', 'WebSubApi'),
        filter('HTTP method', 'EQUALS', 'GET'),
      ]),
    ]);
    expect(build(draft, [llmInput]).problem).toEqual({
      code: 'FILTER_NOT_ALLOWED',
      field: 'HTTP method',
      meterId: 'llm-input',
      price: 2,
    });
    expect(
      build(usageBased([perUnit('plain-calls', [filter('Status code', 'EQUALS', '200')])]), [plain])
        .problem,
    ).toEqual({
      code: 'FILTER_NOT_ALLOWED',
      field: 'Status code',
      meterId: 'plain-calls',
      price: 0,
    });
  });

  it("accepts filters on the meter's properties, ignoring case, and leaves a meter it does not know to the server", () => {
    expect(
      build(usageBased([perUnit('llm-input', [filter('api TYPE', 'EQUALS', 'LlmProvider')])]), [
        llmInput,
      ]).problem,
    ).toBeNull();
    expect(
      build(usageBased([perUnit('other', [filter('HTTP method', 'EQUALS', 'GET')])]), [llmInput])
        .problem,
    ).toBeNull();
  });
});

describe('comparisons', () => {
  const promptFilter = (operator: FilterOperator, text: string) =>
    filter('Prompt tokens', operator, text);
  const buildOnePrice = (filters: FilterDraft[]) => build(usageBased([perUnit('llm', filters)]));

  it('take one whole number from 0 to 1000000000000 on any property, sent without leading zeros', () => {
    expect(
      buildOnePrice([promptFilter('GREATER_THAN', ' 0333 '), filter('Status code', 'GREATER_THAN_OR_EQUAL', '500')])
        .body?.prices?.[0].filters,
    ).toEqual([
      { field: 'Prompt tokens', operator: 'GREATER_THAN', values: ['333'] },
      { field: 'Status code', operator: 'GREATER_THAN_OR_EQUAL', values: ['500'] },
    ]);
    for (const value of ['1.5', '-1', 'ten', '1000000000001', '1, 2']) {
      expect(buildOnePrice([promptFilter('LESS_THAN', value)]).problem).toEqual({
        code: 'NUMBER_INVALID',
        field: 'Prompt tokens',
        price: 0,
      });
    }
    expect(buildOnePrice([promptFilter('LESS_THAN_OR_EQUAL', '1000000000000')]).problem).toBeNull();
    expect(buildOnePrice([promptFilter('GREATER_THAN_OR_EQUAL', '0')]).problem).toBeNull();
  });

  it('take a second comparison on one property, and refuse two that leave no whole number', () => {
    expect(
      buildOnePrice([promptFilter('GREATER_THAN', '100'), promptFilter('LESS_THAN_OR_EQUAL', '500')]).problem,
    ).toBeNull();
    expect(
      buildOnePrice([promptFilter('GREATER_THAN', '7'), promptFilter('GREATER_THAN_OR_EQUAL', '100')]).problem,
    ).toBeNull();
    expect(
      buildOnePrice([promptFilter('GREATER_THAN', '500'), promptFilter('LESS_THAN_OR_EQUAL', '100')]).problem,
    ).toEqual({
      code: 'RANGE_EMPTY',
      field: 'Prompt tokens',
      price: 0,
    });
    expect(buildOnePrice([promptFilter('GREATER_THAN', '100'), promptFilter('LESS_THAN', '101')]).problem).toMatchObject({
      code: 'RANGE_EMPTY',
    });
  });

  it('refuse a match with a comparison on one property, two matches, or three comparisons', () => {
    const conflict = { code: 'FILTERS_CONFLICT', field: 'Prompt tokens', price: 0 };
    expect(buildOnePrice([promptFilter('EQUALS', '7'), promptFilter('GREATER_THAN', '1')]).problem).toEqual(conflict);
    expect(buildOnePrice([promptFilter('EQUALS', '7'), promptFilter('NOT_EQUALS', '8')]).problem).toEqual(conflict);
    expect(
      buildOnePrice([promptFilter('GREATER_THAN', '1'), promptFilter('LESS_THAN', '100'), promptFilter('LESS_THAN', '50')])
        .problem,
    ).toEqual(conflict);
    // The same property whatever its case.
    expect(buildOnePrice([promptFilter('EQUALS', '7'), filter('prompt TOKENS', 'IN', '8')]).problem).toEqual(
      conflict,
    );
  });

  it('refuse a price with more than 20 filters', () => {
    const filters = (count: number) =>
      Array.from({ length: count }, (_, index) => filter(`Count ${index}`, 'GREATER_THAN', '1'));
    expect(buildOnePrice(filters(20)).problem).toBeNull();
    expect(buildOnePrice(filters(21)).problem).toEqual({ code: 'TOO_MANY_FILTERS', price: 0 });
  });
});

describe('buildPricingBody overlap check', () => {
  it('names the first two prices on one meter that count the same requests, by position', () => {
    const draft = usageBased([
      perUnit('calls', [filter('API type', 'IN', 'LlmProvider, Agent')]),
      perUnit('other', [filter('API type', 'EQUALS', 'LlmProvider')]),
      perUnit('calls', [filter('API type', 'EQUALS', 'LlmProxy')]),
      perUnit('calls', [filter('API type', 'EQUALS', 'Agent')]),
    ]);
    // Prices on different meters are never compared.
    expect(build(draft).problem).toEqual({ code: 'OVERLAP', meterId: 'calls', other: 3, price: 0 });
  });

  it('prices without filters, or filtering on properties the other leaves open, count the same requests', () => {
    expect(pair([], [])).toEqual(OVERLAP);
    expect(pair([filter('API type', 'EQUALS', 'LlmProvider')], [])).toEqual(OVERLAP);
    expect(
      pair(
        [filter('API type', 'EQUALS', 'LlmProvider')],
        [filter('HTTP method', 'EQUALS', 'POST')],
      ),
    ).toEqual(OVERLAP);
  });

  it('overlaps only when every property both filter on overlaps, the same property whatever its case', () => {
    expect(
      pair(
        [filter('API type', 'EQUALS', 'LlmProvider'), filter('Status code', 'EQUALS', '200')],
        [filter('api type', 'EQUALS', 'LlmProvider'), filter('status code', 'EQUALS', '201')],
      ),
    ).toBeNull();
  });

  it('equals or is one of against equals or is one of: when they share a value, matched as written', () => {
    expect(
      pair(
        [filter('API type', 'IN', 'LlmProvider, Agent')],
        [filter('API type', 'EQUALS', 'Agent')],
      ),
    ).toEqual(OVERLAP);
    expect(
      pair(
        [filter('API type', 'IN', 'LlmProvider, Agent')],
        [filter('API type', 'IN', 'LlmProxy, WebSubApi')],
      ),
    ).toBeNull();
    expect(
      pair([filter('API type', 'EQUALS', 'RestApi')], [filter('API type', 'EQUALS', 'restapi')]),
    ).toBeNull();
  });

  it('equals or is one of against does not equal or is not one of: when a value is not excluded', () => {
    expect(
      pair(
        [filter('API type', 'IN', 'LlmProvider, Agent')],
        [filter('API type', 'NOT_EQUALS', 'Agent')],
      ),
    ).toEqual(OVERLAP);
    expect(
      pair(
        [filter('API type', 'NOT_IN', 'LlmProvider, Agent')],
        [filter('API type', 'IN', 'Agent, LlmProvider')],
      ),
    ).toBeNull();
  });

  it('equals or is one of against starts with: when a value starts with the text', () => {
    expect(
      pair(
        [filter('Resource', 'EQUALS', '/orders/{id}')],
        [filter('Resource', 'STARTS_WITH', '/orders')],
      ),
    ).toEqual(OVERLAP);
    expect(
      pair(
        [filter('Resource', 'EQUALS', '/orders-archive')],
        [filter('Resource', 'STARTS_WITH', '/orders')],
      ),
    ).toEqual(OVERLAP);
    expect(
      pair(
        [filter('Resource', 'EQUALS', '/orders')],
        [filter('Resource', 'STARTS_WITH', '/orders/')],
      ),
    ).toBeNull();
    expect(
      pair(
        [filter('Resource', 'STARTS_WITH', '/orders')],
        [filter('Resource', 'IN', '/payments, /refunds')],
      ),
    ).toBeNull();
  });

  it('starts with against starts with: when one text starts with the other', () => {
    expect(
      pair(
        [filter('Resource', 'STARTS_WITH', '/order')],
        [filter('Resource', 'STARTS_WITH', '/orders')],
      ),
    ).toEqual(OVERLAP);
    expect(
      pair(
        [filter('Resource', 'STARTS_WITH', '/orders/items')],
        [filter('Resource', 'STARTS_WITH', '/orders')],
      ),
    ).toEqual(OVERLAP);
    expect(
      pair(
        [filter('Resource', 'STARTS_WITH', '/orders')],
        [filter('Resource', 'STARTS_WITH', '/payments')],
      ),
    ).toBeNull();
  });

  it('two exclusions, or an exclusion and starts with: always', () => {
    expect(
      pair(
        [filter('HTTP method', 'NOT_IN', 'GET, POST, PUT')],
        [filter('HTTP method', 'NOT_IN', 'PATCH, DELETE, HEAD, OPTIONS')],
      ),
    ).toEqual(OVERLAP);
    expect(
      pair(
        [filter('Resource', 'NOT_EQUALS', '/orders')],
        [filter('Resource', 'STARTS_WITH', '/orders')],
      ),
    ).toEqual(OVERLAP);
  });

  it('comparisons against comparisons: when their whole-number ranges share a number', () => {
    const promptFilter = (operator: FilterOperator, text: string) =>
      filter('Prompt tokens', operator, text);
    expect(pair([promptFilter('LESS_THAN_OR_EQUAL', '333')], [promptFilter('GREATER_THAN', '333')])).toBeNull();
    expect(
      pair(
        [promptFilter('GREATER_THAN', '100'), promptFilter('LESS_THAN', '200')],
        [promptFilter('GREATER_THAN_OR_EQUAL', '200')],
      ),
    ).toBeNull();
    expect(pair([promptFilter('GREATER_THAN', '100')], [promptFilter('GREATER_THAN', '333')])).toEqual(OVERLAP);
    expect(
      pair(
        [promptFilter('GREATER_THAN', '100'), promptFilter('LESS_THAN_OR_EQUAL', '200')],
        [promptFilter('GREATER_THAN_OR_EQUAL', '200')],
      ),
    ).toEqual(OVERLAP);
  });

  it('comparisons against equals or is one of: when one of the values is a whole number in the range', () => {
    const statusFilter = (operator: FilterOperator, text: string) =>
      filter('Status code', operator, text);
    expect(pair([statusFilter('GREATER_THAN_OR_EQUAL', '500')], [statusFilter('IN', '200, 404')])).toBeNull();
    expect(pair([statusFilter('IN', '200, 503')], [statusFilter('GREATER_THAN_OR_EQUAL', '500')])).toEqual(
      OVERLAP,
    );
    expect(pair([statusFilter('GREATER_THAN_OR_EQUAL', '500')], [statusFilter('EQUALS', '5xx')])).toBeNull();
  });

  it('comparisons against does not equal, is not one of or starts with: always', () => {
    const statusFilter = (operator: FilterOperator, text: string) =>
      filter('Status code', operator, text);
    expect(pair([statusFilter('GREATER_THAN_OR_EQUAL', '500')], [statusFilter('NOT_IN', '500, 503')])).toEqual(
      OVERLAP,
    );
    expect(pair([statusFilter('LESS_THAN', '300')], [statusFilter('STARTS_WITH', '5')])).toEqual(OVERLAP);
  });
});

describe('filterRows', () => {
  it("gives a row for each of the meter's properties, in its order and spelling, then the filters on properties it doesn't have", () => {
    const given = [
      filter('api type', 'IN', 'LlmProvider, Agent'),
      filter('Resource', 'STARTS_WITH', '/orders'),
    ];
    expect(filterRows(given, ['HTTP method', 'API type'])).toEqual([
      // A property without a filter gets an empty row that equals.
      { field: 'HTTP method', operator: 'EQUALS', text: '' },
      { field: 'API type', operator: 'IN', text: 'LlmProvider, Agent' },
      { field: 'Resource', operator: 'STARTS_WITH', text: '/orders' },
    ]);
    expect(filterRows([], [])).toEqual([]);
  });

  it('gives a property both of its comparisons', () => {
    const given = [
      filter('Prompt tokens', 'GREATER_THAN', '100'),
      filter('Prompt tokens', 'LESS_THAN_OR_EQUAL', '500'),
    ];
    expect(filterRows(given, ['API type', 'Prompt tokens'])).toEqual([
      { field: 'API type', operator: 'EQUALS', text: '' },
      ...given,
    ]);
  });
});

describe('priceName', () => {
  it('names a price after its meter and each filter in short', () => {
    expect(priceName('LLM input tokens', [filter('API type', 'IN', 'LlmProvider, LlmProxy')])).toBe(
      'LLM input tokens · LlmProvider, LlmProxy',
    );
    expect(priceName('API calls', [filter('HTTP method', 'NOT_IN', 'DELETE, OPTIONS')])).toBe(
      'API calls · not DELETE, OPTIONS',
    );
    expect(priceName('API calls', [filter('Resource', 'STARTS_WITH', '/orders')])).toBe(
      'API calls · /orders…',
    );
    expect(
      priceName('Requests', [
        filter('HTTP method', 'EQUALS', 'GET'),
        filter('Prompt tokens', 'GREATER_THAN', '333'),
      ]),
    ).toBe('Requests · GET · Prompt tokens > 333');
    expect(
      priceName('Calls', [
        filter('Total tokens', 'GREATER_THAN_OR_EQUAL', '1'),
        filter('Total tokens', 'LESS_THAN', '10'),
        filter('Status code', 'LESS_THAN_OR_EQUAL', '499'),
        filter('Prompt tokens', 'GREATER_THAN', ''),
      ]),
    ).toBe('Calls · Total tokens ≥ 1 · Total tokens < 10 · Status code ≤ 499');
    expect(priceName('API calls', [])).toBe('API calls');
  });
});

describe('priceLabel', () => {
  it("names a price after its meter and its filters in the meter's property order, or after the meter's id while it is not known", () => {
    const calls = aMeter({
      displayName: 'API calls',
      filterFields: [
        { eventKey: 'request.verb', field: 'HTTP method' },
        { eventKey: 'response.status', field: 'Status code' },
      ],
      id: 'calls',
    });
    const given = perUnit('calls', [
      filter('Status code', 'EQUALS', '200'),
      filter('HTTP method', 'EQUALS', 'GET'),
    ]);
    expect(priceLabel(given, calls)).toBe('API calls · GET · 200');
    expect(priceLabel(given, undefined)).toBe('calls · 200 · GET');
  });
});

describe('pricesByName', () => {
  const calls = aMeter({
    displayName: 'API calls',
    filterFields: [{ eventKey: 'request.verb', field: 'HTTP method' }],
    id: 'calls',
  });
  const meters = [
    calls,
    aMeter({ displayName: 'batch jobs', filterFields: [], id: 'batch' }),
    aMeter({ displayName: 'Webhooks', filterFields: [], id: 'webhooks' }),
  ];
  const methodPrice = (verb: string) => perUnit('calls', [filter('HTTP method', 'EQUALS', verb)]);

  it('lists saved prices by name, so API calls · POST given before API calls · GET is shown after it', () => {
    const post = methodPrice('POST');
    const get = methodPrice('GET');
    expect(pricesByName([post, get], [calls])).toEqual([get, post]);
  });

  it('ignores case, keeps prices of the same name in their given order, and leaves the given list as it is', () => {
    const webhooks = perUnit('webhooks');
    const moreWebhooks = perUnit('webhooks', [], { unitAmount: '0.02' });
    const batch = perUnit('batch');
    const post = methodPrice('POST');
    const given = [webhooks, batch, moreWebhooks, post];
    // "batch jobs" comes between "API calls" and "Webhooks", not after both as it would by
    // character codes.
    expect(pricesByName(given, meters)).toEqual([post, batch, webhooks, moreWebhooks]);
    expect(given).toEqual([webhooks, batch, moreWebhooks, post]);
  });

  it('lists prices of two meters with the same name by meter id, as the API Portal does', () => {
    const twins = [
      aMeter({ displayName: 'Calls', filterFields: [], id: 'calls-b' }),
      aMeter({ displayName: 'Calls', filterFields: [], id: 'calls-a' }),
    ];
    const b = perUnit('calls-b');
    const a = perUnit('calls-a');
    expect(pricesByName([b, a], twins)).toEqual([a, b]);
  });
});

describe('draftFromPricing', () => {
  it('starts a plan without pricing as free and flat rate', () => {
    expect(draftFromPricing(undefined)).toMatchObject({
      paid: false,
      prices: [],
      pricingType: 'FLAT_RATE',
    });
  });

  it('round-trips stored flat rate pricing', () => {
    const draft = draftFromPricing({
      billingPeriod: 'MONTH',
      currency: 'USD',
      flatAmount: '20.00',
      prices: [],
      pricingType: 'FLAT_RATE',
    });
    expect(build(draft).body).toEqual({
      billingPeriod: 'MONTH',
      currency: 'USD',
      flatAmount: '20.00',
      pricingType: 'FLAT_RATE',
    });
  });

  it('round-trips stored usage based pricing, comparisons included', () => {
    const stored = {
      billingPeriod: 'YEAR' as const,
      currency: 'EUR',
      prices: [
        {
          filters: [
            { field: 'API type', operator: 'IN' as const, values: ['LlmProvider', 'Mcp'] },
            { field: 'HTTP method', operator: 'NOT_EQUALS' as const, values: ['GET'] },
            { field: 'Prompt tokens', operator: 'GREATER_THAN' as const, values: ['333'] },
            { field: 'Prompt tokens', operator: 'LESS_THAN_OR_EQUAL' as const, values: ['5000'] },
          ],
          meterId: 'llm-input',
          pricingModel: 'VOLUME' as const,
          tiers: [
            { flatAmount: null, unitAmount: '0.01', upTo: 1000 },
            { flatAmount: '5', unitAmount: null, upTo: null },
          ],
          unitAmount: null,
          unitQuantity: null,
        },
      ],
      pricingType: 'USAGE_BASED' as const,
    };
    const draft = draftFromPricing({ ...stored, flatAmount: null });
    expect(draft).toMatchObject({
      billingPeriod: 'YEAR',
      currency: 'EUR',
      paid: true,
      pricingType: 'USAGE_BASED',
    });
    expect(build(draft).body).toEqual(stored);
  });
});
