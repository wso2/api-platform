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
import type { FilterField, FilterOperator } from './meterLabels';

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
const filter = (field: FilterField, operator: FilterOperator, text: string): FilterDraft => ({
  field,
  operator,
  text,
});
/** A Token condition on `eventKey`. */
const token = (eventKey: string, operator: FilterOperator, text: string): FilterDraft => ({
  eventKey,
  field: 'TOKEN',
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
 * Without meters, no price's meter is known, so the fields it filters on are left to the server.
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
      perUnit('llm-input', [filter('API_TYPE', 'EQUALS', 'LlmProvider')], {
        unitAmount: '2.50',
        unitQuantity: '1000000',
      }),
      perUnit('llm-input', [filter('API_TYPE', 'IN', 'Mcp, LlmProxy, Mcp')], {
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
            filters: [{ field: 'API_TYPE', operator: 'EQUALS', values: ['LlmProvider'] }],
            meterId: 'llm-input',
            pricingModel: 'PER_UNIT',
            tiers: null,
            unitAmount: '2.50',
            unitQuantity: 1000000,
          },
          {
            filters: [{ field: 'API_TYPE', operator: 'IN', values: ['Mcp', 'LlmProxy'] }],
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
    const invalid = (field: FilterField, operator: FilterOperator, text: string) =>
      problem(perUnit('calls', [filter(field, operator, text)]));
    expect(invalid('API_TYPE', 'EQUALS', 'LlmProvider, Agent')).toEqual({
      code: 'FILTER_INVALID',
      field: 'API_TYPE',
      price: 0,
    });
    // Equals, does not equal and starts with take one value; status codes are exact; a resource
    // starts with /.
    expect(invalid('HTTP_METHOD', 'NOT_EQUALS', 'GET, POST')).toMatchObject({
      code: 'FILTER_INVALID',
    });
    expect(invalid('HTTP_STATUS_CODE', 'IN', '200, 2xx')).toMatchObject({ code: 'FILTER_INVALID' });
    expect(invalid('RESOURCE', 'STARTS_WITH', 'orders')).toMatchObject({ code: 'FILTER_INVALID' });
    expect(invalid('API_TYPE', 'EQUALS', 'x'.repeat(256))).toMatchObject({
      code: 'FILTER_INVALID',
    });
    expect(problem(perUnit('calls', [], { unitAmount: 'x' }))).toEqual({
      code: 'AMOUNT_INVALID',
      price: 0,
    });
    expect(problem(perUnit('calls', [], { unitQuantity: '0' }))).toEqual({
      code: 'QUANTITY_INVALID',
      price: 0,
    });
  });

  it("sends no filter for a row left empty, and the others in the order of the meter's rows", () => {
    // The meter's rows are status code, HTTP method and resource.
    const draft = usageBased([
      perUnit('successful-calls', [
        filter('RESOURCE', 'STARTS_WITH', ' '),
        filter('HTTP_METHOD', 'NOT_EQUALS', 'GET'),
        filter('API_TYPE', 'IN', ' , '),
        filter('HTTP_STATUS_CODE', 'IN', '200'),
      ]),
    ]);
    expect(build(draft, [aMeter()]).body?.prices?.[0].filters).toEqual([
      { field: 'HTTP_STATUS_CODE', operator: 'IN', values: ['200'] },
      { field: 'HTTP_METHOD', operator: 'NOT_EQUALS', values: ['GET'] },
    ]);
    // Without the meter, the rows go in the order given.
    expect(build(draft).body?.prices?.[0].filters).toEqual([
      { field: 'HTTP_METHOD', operator: 'NOT_EQUALS', values: ['GET'] },
      { field: 'HTTP_STATUS_CODE', operator: 'IN', values: ['200'] },
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

describe('buildPricingBody filter field check', () => {
  const llmInput = aMeter({
    filterFields: [{ eventKey: 'metadata.apiType', field: 'API_TYPE' }],
    id: 'llm-input',
    type: 'TOKENS',
    typeKey: 'metadata.aiTokenUsage.promptTokens',
  });
  const plain = aMeter({ filterFields: [], id: 'plain-calls' });

  it('names the first price that filters on a field its meter does not allow, before its values; an empty row is no filter', () => {
    const draft = usageBased([
      perUnit('llm-input', [filter('API_TYPE', 'EQUALS', 'LlmProvider')]),
      perUnit('llm-input', [
        filter('API_TYPE', 'EQUALS', 'Agent'),
        filter('HTTP_METHOD', 'EQUALS', ''),
      ]),
      perUnit('llm-input', [
        filter('API_TYPE', 'EQUALS', 'WebSubApi'),
        filter('HTTP_METHOD', 'EQUALS', 'GET'),
      ]),
    ]);
    expect(build(draft, [llmInput]).problem).toEqual({
      code: 'FILTER_NOT_ALLOWED',
      field: 'HTTP_METHOD',
      meterId: 'llm-input',
      price: 2,
    });
    expect(
      build(usageBased([perUnit('plain-calls', [filter('HTTP_STATUS_CODE', 'EQUALS', '200')])]), [
        plain,
      ]).problem,
    ).toEqual({
      code: 'FILTER_NOT_ALLOWED',
      field: 'HTTP_STATUS_CODE',
      meterId: 'plain-calls',
      price: 0,
    });
  });

  it('accepts filters on the fields the meter allows, and leaves a meter it does not know to the server', () => {
    expect(
      build(usageBased([perUnit('llm-input', [filter('API_TYPE', 'EQUALS', 'LlmProvider')])]), [
        llmInput,
      ]).problem,
    ).toBeNull();
    expect(
      build(usageBased([perUnit('other', [filter('HTTP_METHOD', 'EQUALS', 'GET')])]), [llmInput])
        .problem,
    ).toBeNull();
  });
});

describe('buildPricingBody overlap check', () => {
  it('names the first two prices on one meter that count the same requests, by position', () => {
    const draft = usageBased([
      perUnit('calls', [filter('API_TYPE', 'IN', 'LlmProvider, Agent')]),
      perUnit('other', [filter('API_TYPE', 'EQUALS', 'LlmProvider')]),
      perUnit('calls', [filter('API_TYPE', 'EQUALS', 'LlmProxy')]),
      perUnit('calls', [filter('API_TYPE', 'EQUALS', 'Agent')]),
    ]);
    // Prices on different meters are never compared.
    expect(build(draft).problem).toEqual({ code: 'OVERLAP', meterId: 'calls', other: 3, price: 0 });
  });

  it('prices without filters, or filtering on fields the other leaves open, count the same requests', () => {
    expect(pair([], [])).toEqual(OVERLAP);
    expect(pair([filter('API_TYPE', 'EQUALS', 'LlmProvider')], [])).toEqual(OVERLAP);
    expect(
      pair(
        [filter('API_TYPE', 'EQUALS', 'LlmProvider')],
        [filter('HTTP_METHOD', 'EQUALS', 'POST')],
      ),
    ).toEqual(OVERLAP);
  });

  it('overlaps only when every field both filter on overlaps', () => {
    expect(
      pair(
        [filter('API_TYPE', 'EQUALS', 'LlmProvider'), filter('HTTP_STATUS_CODE', 'EQUALS', '200')],
        [filter('API_TYPE', 'EQUALS', 'LlmProvider'), filter('HTTP_STATUS_CODE', 'EQUALS', '201')],
      ),
    ).toBeNull();
  });

  it('equals or is one of against equals or is one of: when they share a value', () => {
    expect(
      pair(
        [filter('API_TYPE', 'IN', 'LlmProvider, Agent')],
        [filter('API_TYPE', 'EQUALS', 'Agent')],
      ),
    ).toEqual(OVERLAP);
    expect(
      pair(
        [filter('API_TYPE', 'IN', 'LlmProvider, Agent')],
        [filter('API_TYPE', 'IN', 'LlmProxy, WebSubApi')],
      ),
    ).toBeNull();
  });

  it('equals or is one of against does not equal or is not one of: when a value is not excluded', () => {
    expect(
      pair(
        [filter('API_TYPE', 'IN', 'LlmProvider, Agent')],
        [filter('API_TYPE', 'NOT_EQUALS', 'Agent')],
      ),
    ).toEqual(OVERLAP);
    expect(
      pair(
        [filter('API_TYPE', 'NOT_IN', 'LlmProvider, Agent')],
        [filter('API_TYPE', 'IN', 'Agent, LlmProvider')],
      ),
    ).toBeNull();
  });

  it('equals or is one of against starts with: when a value is the path or under it, by whole segments', () => {
    expect(
      pair(
        [filter('RESOURCE', 'EQUALS', '/orders/{id}')],
        [filter('RESOURCE', 'STARTS_WITH', '/orders')],
      ),
    ).toEqual(OVERLAP);
    expect(
      pair(
        [filter('RESOURCE', 'EQUALS', '/orders')],
        [filter('RESOURCE', 'STARTS_WITH', '/orders/')],
      ),
    ).toEqual(OVERLAP);
    expect(
      pair(
        [filter('RESOURCE', 'STARTS_WITH', '/orders')],
        [filter('RESOURCE', 'EQUALS', '/payments')],
      ),
    ).toBeNull();
    expect(
      pair(
        [filter('RESOURCE', 'EQUALS', '/orders-archive')],
        [filter('RESOURCE', 'STARTS_WITH', '/orders')],
      ),
    ).toBeNull();
    expect(
      pair([filter('RESOURCE', 'EQUALS', '/payments')], [filter('RESOURCE', 'STARTS_WITH', '/')]),
    ).toEqual(OVERLAP);
  });

  it('starts with against starts with: when one path is the other or under it', () => {
    expect(
      pair(
        [filter('RESOURCE', 'STARTS_WITH', '/orders/')],
        [filter('RESOURCE', 'STARTS_WITH', '/orders')],
      ),
    ).toEqual(OVERLAP);
    expect(
      pair(
        [filter('RESOURCE', 'STARTS_WITH', '/orders')],
        [filter('RESOURCE', 'STARTS_WITH', '/orders/items')],
      ),
    ).toEqual(OVERLAP);
    expect(
      pair(
        [filter('RESOURCE', 'STARTS_WITH', '/orders')],
        [filter('RESOURCE', 'STARTS_WITH', '/payments')],
      ),
    ).toBeNull();
    expect(
      pair(
        [filter('RESOURCE', 'STARTS_WITH', '/order')],
        [filter('RESOURCE', 'STARTS_WITH', '/orders')],
      ),
    ).toBeNull();
  });

  it('two exclusions, or an exclusion and a prefix: always', () => {
    expect(
      pair(
        [filter('API_TYPE', 'NOT_IN', 'LlmProvider, Agent')],
        [filter('API_TYPE', 'NOT_EQUALS', 'WebSubApi')],
      ),
    ).toEqual(OVERLAP);
    expect(
      pair(
        [filter('RESOURCE', 'NOT_EQUALS', '/orders')],
        [filter('RESOURCE', 'STARTS_WITH', '/orders')],
      ),
    ).toEqual(OVERLAP);
  });

  it('two exclusions of HTTP methods: unless together they exclude all seven', () => {
    expect(
      pair(
        [filter('HTTP_METHOD', 'NOT_IN', 'GET, POST')],
        [filter('HTTP_METHOD', 'NOT_EQUALS', 'PUT')],
      ),
    ).toEqual(OVERLAP);
    expect(
      pair(
        [filter('HTTP_METHOD', 'NOT_IN', 'GET, POST, PUT')],
        [filter('HTTP_METHOD', 'NOT_IN', 'PATCH, DELETE, HEAD, OPTIONS')],
      ),
    ).toBeNull();
  });
});

describe('API type filters', () => {
  it('takes one value for equals and does not equal, up to 20 for is one of, each up to 255 characters, as written', () => {
    const problem = (operator: FilterOperator, text: string) =>
      build(usageBased([perUnit('calls', [filter('API_TYPE', operator, text)])])).problem;
    expect(problem('EQUALS', 'RestApi, Mcp')).toEqual({
      code: 'FILTER_INVALID',
      field: 'API_TYPE',
      price: 0,
    });
    expect(problem('NOT_EQUALS', 'x'.repeat(256))).toEqual({
      code: 'FILTER_INVALID',
      field: 'API_TYPE',
      price: 0,
    });
    expect(
      problem('IN', Array.from({ length: 21 }, (_, index) => `Api${index}`).join(', ')),
    ).toMatchObject({
      code: 'FILTER_INVALID',
    });
    // Any text is accepted, not only our gateway's API types.
    const { body } = build(
      usageBased([perUnit('calls', [filter('API_TYPE', 'IN', 'RestApi, CustomApi, RestApi')])]),
    );
    expect(body?.prices?.[0].filters).toEqual([
      { field: 'API_TYPE', operator: 'IN', values: ['RestApi', 'CustomApi'] },
    ]);
  });

  it('lets two prices on one meter count different API types, matched exactly as written', () => {
    expect(
      pair(
        [filter('API_TYPE', 'EQUALS', 'RestApi')],
        [filter('API_TYPE', 'EQUALS', 'LlmProvider')],
      ),
    ).toBeNull();
    expect(
      pair([filter('API_TYPE', 'EQUALS', 'RestApi')], [filter('API_TYPE', 'EQUALS', 'restapi')]),
    ).toBeNull();
    expect(
      pair(
        [filter('API_TYPE', 'IN', 'RestApi, Mcp')],
        [filter('API_TYPE', 'NOT_EQUALS', 'RestApi')],
      ),
    ).toEqual(OVERLAP);
    expect(
      pair(
        [filter('API_TYPE', 'EQUALS', 'RestApi')],
        [filter('API_TYPE', 'NOT_IN', 'RestApi, Mcp')],
      ),
    ).toBeNull();
  });

  it('names a price by its API types', () => {
    expect(priceName('API requests', [filter('API_TYPE', 'EQUALS', 'RestApi')])).toBe(
      'API requests · RestApi',
    );
    expect(priceName('API requests', [filter('API_TYPE', 'NOT_EQUALS', 'LlmProvider')])).toBe(
      'API requests · not LlmProvider',
    );
  });
});

describe('MCP method and Tool filters', () => {
  it('take free text, kept as written, and a price with a Tool filter has no MCP method filter', () => {
    const { body } = build(
      usageBased([
        perUnit('calls', [filter('MCP_METHOD', 'IN', 'tools/call, resources/read, tools/call')]),
        perUnit('tools', [filter('MCP_TOOL', 'NOT_EQUALS', 'Search')]),
      ]),
    );
    expect(body?.prices?.map((p) => p.filters)).toEqual([
      [{ field: 'MCP_METHOD', operator: 'IN', values: ['tools/call', 'resources/read'] }],
      [{ field: 'MCP_TOOL', operator: 'NOT_EQUALS', values: ['Search'] }],
    ]);
    expect(
      build(
        usageBased([
          perUnit('calls', [
            filter('MCP_METHOD', 'EQUALS', 'tools/call'),
            filter('MCP_TOOL', 'EQUALS', 'search'),
          ]),
        ]),
      ).problem,
    ).toEqual({ code: 'TOOL_WITH_METHOD', price: 0 });
  });

  it('compare a Tool filter as one on the MCP method tools/call', () => {
    const tool = (operator: FilterOperator, text: string) => [filter('MCP_TOOL', operator, text)];
    const method = (operator: FilterOperator, text: string) => [
      filter('MCP_METHOD', operator, text),
    ];
    expect(pair(method('EQUALS', 'tools/call'), method('NOT_EQUALS', 'tools/call'))).toBeNull();
    expect(pair(tool('EQUALS', 'search'), tool('NOT_IN', 'search'))).toBeNull();
    expect(pair(tool('EQUALS', 'search'), method('EQUALS', 'prompts/get'))).toBeNull();
    expect(pair(tool('EQUALS', 'search'), method('NOT_EQUALS', 'tools/call'))).toBeNull();
    expect(pair(tool('EQUALS', 'search'), method('EQUALS', 'tools/call'))).toEqual(OVERLAP);
    expect(pair(tool('IN', 'search, fetch'), [])).toEqual(OVERLAP);
  });

  it('name a price by the methods or tools it counts', () => {
    expect(priceName('MCP calls', [filter('MCP_METHOD', 'EQUALS', 'tools/call')])).toBe(
      'MCP calls · tools/call',
    );
    expect(priceName('MCP calls', [filter('MCP_TOOL', 'NOT_IN', 'search, fetch')])).toBe(
      'MCP calls · not search, fetch',
    );
  });
});

describe('Token conditions', () => {
  const prompt = 'metadata.aiTokenUsage.promptTokens';
  const total = 'metadata.aiTokenUsage.totalTokens';
  /**
   * A Tokens meter adding up prompt tokens, with a Token field on total tokens and the API type.
   */
  const llm = aMeter({
    filterFields: [
      { eventKey: 'metadata.apiType', field: 'API_TYPE' },
      { eventKey: total, field: 'TOKEN' },
    ],
    id: 'llm',
    type: 'TOKENS',
    typeKey: prompt,
  });
  const onLlm = (filters: FilterDraft[]) => build(usageBased([perUnit('llm', filters)]), [llm]);

  it("sends a condition on one of the meter's Token keys, its Token row included, with its key and one whole number", () => {
    const { body } = onLlm([
      token(prompt, 'GREATER_THAN', ' 0333 '),
      token(total, 'LESS_THAN_OR_EQUAL', '5000'),
    ]);
    expect(body?.prices?.[0].filters).toEqual([
      { eventKey: prompt, field: 'TOKEN', operator: 'GREATER_THAN', values: ['333'] },
      { eventKey: total, field: 'TOKEN', operator: 'LESS_THAN_OR_EQUAL', values: ['5000'] },
    ]);
  });

  it("refuses a key the meter doesn't have, and a value that isn't a whole number from 0 to 1000000000000", () => {
    expect(onLlm([token('metadata.aiTokenUsage.completionTokens', 'EQUALS', '1')]).problem).toEqual(
      {
        code: 'TOKEN_KEY_UNKNOWN',
        eventKey: 'metadata.aiTokenUsage.completionTokens',
        price: 0,
      },
    );
    for (const value of ['1.5', '-1', 'ten', '1000000000001', '1, 2']) {
      expect(onLlm([token(prompt, 'GREATER_THAN', value)]).problem).toEqual({
        code: 'TOKEN_VALUE_INVALID',
        eventKey: prompt,
        price: 0,
      });
    }
    expect(onLlm([token(prompt, 'LESS_THAN', '1000000000000')]).problem).toBeNull();
    expect(onLlm([token(prompt, 'EQUALS', '0')]).problem).toBeNull();
  });

  it("refuses two conditions on one key that can't both be true", () => {
    expect(
      onLlm([token(prompt, 'GREATER_THAN', '500'), token(prompt, 'LESS_THAN_OR_EQUAL', '100')])
        .problem,
    ).toEqual({
      code: 'TOKEN_RANGE_EMPTY',
      eventKey: prompt,
      price: 0,
    });
    expect(
      onLlm([token(prompt, 'GREATER_THAN', '100'), token(prompt, 'LESS_THAN', '101')]).problem,
    ).toMatchObject({
      code: 'TOKEN_RANGE_EMPTY',
    });
    expect(
      onLlm([token(prompt, 'GREATER_THAN', '100'), token(prompt, 'LESS_THAN_OR_EQUAL', '500')])
        .problem,
    ).toBeNull();
    expect(
      onLlm([token(prompt, 'EQUALS', '7'), token(prompt, 'GREATER_THAN_OR_EQUAL', '7')]).problem,
    ).toBeNull();
  });

  it('keeps two prices apart when their conditions on a key share no number, mixed with other fields as before', () => {
    const tokenPair = (first: FilterDraft[], second: FilterDraft[]) =>
      build(usageBased([perUnit('calls', first), perUnit('calls', second)])).problem;
    expect(
      tokenPair(
        [token(prompt, 'LESS_THAN_OR_EQUAL', '333')],
        [token(prompt, 'GREATER_THAN', '333')],
      ),
    ).toBeNull();
    expect(
      tokenPair([token(prompt, 'GREATER_THAN', '100')], [token(prompt, 'GREATER_THAN', '333')]),
    ).toEqual(OVERLAP);
    // A price without a condition on the key counts every number of it.
    expect(tokenPair([token(prompt, 'GREATER_THAN', '100')], [])).toEqual(OVERLAP);
    // Conditions on different keys leave both ranges open for the other key.
    expect(
      tokenPair([token(prompt, 'GREATER_THAN', '100')], [token(total, 'LESS_THAN', '10')]),
    ).toEqual(OVERLAP);
    expect(
      tokenPair(
        [filter('API_TYPE', 'EQUALS', 'LlmProvider'), token(prompt, 'GREATER_THAN', '333')],
        [filter('API_TYPE', 'EQUALS', 'Agent'), token(prompt, 'GREATER_THAN', '333')],
      ),
    ).toBeNull();
    expect(
      tokenPair(
        [filter('API_TYPE', 'EQUALS', 'LlmProvider'), token(prompt, 'GREATER_THAN', '100')],
        [
          filter('API_TYPE', 'EQUALS', 'LlmProvider'),
          token(prompt, 'GREATER_THAN_OR_EQUAL', '333'),
        ],
      ),
    ).toEqual(OVERLAP);
  });

  it("names a condition by its key's lower-case label, or the key, and a sign", () => {
    expect(
      priceName('Input tokens', [
        filter('API_TYPE', 'EQUALS', 'LlmProvider'),
        token(prompt, 'GREATER_THAN', '333'),
      ]),
    ).toBe('Input tokens · LlmProvider · prompt tokens > 333');
    expect(
      priceName('Calls', [
        token(total, 'GREATER_THAN_OR_EQUAL', '1'),
        token('metadata.aiTokenUsage.completionTokens', 'LESS_THAN', '10'),
        token(prompt, 'LESS_THAN_OR_EQUAL', '500'),
        token('usage.cached', 'EQUALS', '0'),
        token(prompt, 'GREATER_THAN', ''),
      ]),
    ).toBe(
      'Calls · total tokens ≥ 1 · completion tokens < 10 · prompt tokens ≤ 500 · usage.cached = 0',
    );
  });

  it('refuses a price with more than 20 filters, Token conditions included', () => {
    const keys = Array.from({ length: 10 }, (_, index) => `usage.count${index}`);
    const wide = aMeter({
      filterFields: keys.map((eventKey) => ({ eventKey, field: 'TOKEN' as const })),
      id: 'wide',
    });
    const conditions = (count: number) =>
      keys
        .flatMap((key) => [token(key, 'GREATER_THAN', '1'), token(key, 'LESS_THAN', '100')])
        .slice(0, count);
    expect(build(usageBased([perUnit('wide', conditions(20))]), [wide]).problem).toBeNull();
    expect(
      build(
        usageBased([perUnit('wide', [...conditions(20), filter('HTTP_METHOD', 'EQUALS', 'GET')])]),
        [
          aMeter({
            ...wide,
            filterFields: [
              ...(wide.filterFields ?? []),
              { eventKey: 'request.verb', field: 'HTTP_METHOD' },
            ],
          }),
        ],
      ).problem,
    ).toEqual({ code: 'TOO_MANY_FILTERS', price: 0 });
  });

  it('round-trips stored Token conditions with their keys', () => {
    const stored = {
      billingPeriod: 'MONTH' as const,
      currency: 'USD',
      flatAmount: null,
      prices: [
        {
          filters: [
            {
              eventKey: prompt,
              field: 'TOKEN' as const,
              operator: 'GREATER_THAN' as const,
              values: ['333'],
            },
          ],
          meterId: 'llm',
          pricingModel: 'PER_UNIT' as const,
          tiers: null,
          unitAmount: '0.01',
          unitQuantity: 1,
        },
      ],
      pricingType: 'USAGE_BASED' as const,
      subscriptionPlanId: 'gold',
    };
    const draft = draftFromPricing(stored);
    expect(draft.prices[0].filters).toEqual([token(prompt, 'GREATER_THAN', '333')]);
    expect(build(draft, [llm]).body?.prices).toEqual(stored.prices);
  });
});

describe('filterRows', () => {
  it("gives a row for each of the meter's fields, in its order, then the filters on fields it does not allow", () => {
    const given = [
      filter('API_TYPE', 'IN', 'LlmProvider, Agent'),
      filter('RESOURCE', 'STARTS_WITH', '/orders'),
    ];
    expect(filterRows(given, [{ field: 'HTTP_METHOD' }, { field: 'API_TYPE' }])).toEqual([
      // A field without a filter gets an empty row with its first operator.
      { field: 'HTTP_METHOD', operator: 'EQUALS', text: '' },
      { field: 'API_TYPE', operator: 'IN', text: 'LlmProvider, Agent' },
      { field: 'RESOURCE', operator: 'STARTS_WITH', text: '/orders' },
    ]);
    expect(filterRows([], [])).toEqual([]);
  });

  it("gives each Token key its conditions, or one empty row, then conditions on keys the meter doesn't have", () => {
    const prompt = 'metadata.aiTokenUsage.promptTokens';
    const given = [
      token(prompt, 'GREATER_THAN', '100'),
      token(prompt, 'LESS_THAN_OR_EQUAL', '500'),
      token('usage.gone', 'EQUALS', '1'),
    ];
    expect(
      filterRows(given, [
        { field: 'API_TYPE' },
        { eventKey: prompt, field: 'TOKEN' },
        { eventKey: 'metadata.aiTokenUsage.totalTokens', field: 'TOKEN' },
      ]),
    ).toEqual([
      { field: 'API_TYPE', operator: 'EQUALS', text: '' },
      token(prompt, 'GREATER_THAN', '100'),
      token(prompt, 'LESS_THAN_OR_EQUAL', '500'),
      // A Token key without a condition gets an empty one, "more than".
      {
        eventKey: 'metadata.aiTokenUsage.totalTokens',
        field: 'TOKEN',
        operator: 'GREATER_THAN',
        text: '',
      },
      token('usage.gone', 'EQUALS', '1'),
    ]);
  });
});

describe('priceName', () => {
  it('names a price after its meter and filters', () => {
    expect(priceName('LLM input tokens', [filter('API_TYPE', 'EQUALS', 'LlmProvider')])).toBe(
      'LLM input tokens · LlmProvider',
    );
    expect(priceName('API calls', [filter('HTTP_METHOD', 'NOT_IN', 'DELETE, OPTIONS')])).toBe(
      'API calls · not DELETE, OPTIONS',
    );
    expect(priceName('API calls', [filter('RESOURCE', 'STARTS_WITH', '/orders')])).toBe(
      'API calls · /orders…',
    );
    expect(
      priceName('API calls', [
        filter('HTTP_STATUS_CODE', 'IN', '200, 201'),
        filter('HTTP_METHOD', 'NOT_EQUALS', 'GET'),
      ]),
    ).toBe('API calls · 200, 201 · not GET');
    expect(priceName('API calls', [])).toBe('API calls');
  });
});

describe('priceLabel', () => {
  it("names a price after its meter and its filters in the meter's field order, or after the meter's id while it is not known", () => {
    const calls = aMeter({
      displayName: 'API calls',
      filterFields: [
        { eventKey: 'request.verb', field: 'HTTP_METHOD' },
        { eventKey: 'response.status', field: 'HTTP_STATUS_CODE' },
      ],
      id: 'calls',
    });
    const given = perUnit('calls', [
      filter('HTTP_STATUS_CODE', 'EQUALS', '200'),
      filter('HTTP_METHOD', 'EQUALS', 'GET'),
    ]);
    expect(priceLabel(given, calls)).toBe('API calls · GET · 200');
    expect(priceLabel(given, undefined)).toBe('calls · 200 · GET');
  });
});

describe('pricesByName', () => {
  const calls = aMeter({
    displayName: 'API calls',
    filterFields: [{ eventKey: 'request.verb', field: 'HTTP_METHOD' }],
    id: 'calls',
  });
  const meters = [
    calls,
    aMeter({ displayName: 'batch jobs', filterFields: [], id: 'batch' }),
    aMeter({ displayName: 'Webhooks', filterFields: [], id: 'webhooks' }),
  ];
  const method = (verb: string) => perUnit('calls', [filter('HTTP_METHOD', 'EQUALS', verb)]);

  it('lists saved prices by name, so API calls · POST given before API calls · GET is shown after it', () => {
    const post = method('POST');
    const get = method('GET');
    expect(pricesByName([post, get], [calls])).toEqual([get, post]);
  });

  it('ignores case, keeps prices of the same name in their given order, and leaves the given list as it is', () => {
    const webhooks = perUnit('webhooks');
    const moreWebhooks = perUnit('webhooks', [], { unitAmount: '0.02' });
    const batch = perUnit('batch');
    const post = method('POST');
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

  it('round-trips stored usage based pricing', () => {
    const stored = {
      billingPeriod: 'YEAR' as const,
      currency: 'EUR',
      prices: [
        {
          filters: [
            { field: 'API_TYPE' as const, operator: 'IN' as const, values: ['LlmProvider', 'Mcp'] },
            { field: 'HTTP_METHOD' as const, operator: 'NOT_EQUALS' as const, values: ['GET'] },
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
