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
import type { Meter } from '@/api/resources/meters';
import type {
  PriceFilter,
  SubscriptionPlanPrice,
  SubscriptionPlanPricing,
  SubscriptionPlanPricingBody,
} from '@/api/resources/subscriptionPlans';
import {
  FILTER_RULES,
  HTTP_METHODS,
  TOKEN_KEYS,
  parseFilterValues,
  priceFieldsOf,
  type FilterField,
  type FilterOperator,
  type PriceField,
} from './meterLabels';

/**
 * The plan dialog's Paid plan section as editable state, and its conversion to
 * the pricing request. A paid plan is either:
 *   - flat rate: one amount every billing period; or
 *   - usage based: up to 20 prices, each charging for what a meter counts,
 *     narrowed by the price's own filters on the fields the meter allows, in one
 *     pricing model (Per unit, Volume or Graduated).
 *
 * A price has a filter row for each of its meter's fields, and a condition row
 * for each of its Token keys (up to two per key, such as more than 100 and at
 * most 500); a row left empty is no filter. A meter may be priced more than
 * once, as long as no request could be counted by two of its prices. The checks
 * mirror the API Portal's rules, which the Platform API applies too, so most
 * mistakes are caught before a request is sent.
 */

export type PricingType = SubscriptionPlanPricingBody['pricingType'];
export type PricingModel = SubscriptionPlanPrice['pricingModel'];

export const PRICING_TYPES: PricingType[] = ['FLAT_RATE', 'USAGE_BASED'];
export const PRICING_MODELS: PricingModel[] = ['PER_UNIT', 'VOLUME', 'GRADUATED'];
export const MAX_TIERS = 10;
/** A subscription holds at most 20 items, one per price. */
export const MAX_PRICES = 20;
/** At most this many filters in one price, Token conditions included. */
export const MAX_PRICE_FILTERS = 20;
const MAX_UNIT_QUANTITY = 1_000_000_000;
const AMOUNT_PATTERN = /^\d{1,12}(\.\d{1,12})?$/;

export type TierDraft = { key: number; upTo: string; unitAmount: string; flatAmount: string };

/**
 * One filter row of a price being edited; its values as typed or picked,
 * comma-separated when there are several. A row without values is no filter.
 * A Token condition names the key it reads in `eventKey` and holds one whole
 * number.
 */
export type FilterDraft = {
  field: FilterField;
  operator: FilterOperator;
  text: string;
  eventKey?: string;
};

/** One usage price being edited. */
export type PriceDraft = {
  key: number;
  /** The meter that counts the units. */
  meterId: string;
  /**
   * Which of the meter's requests the price counts, one row per field; rows without values count
   * them all.
   */
  filters: FilterDraft[];
  pricingModel: PricingModel;
  /** Per unit: the price per `unitQuantity` units. */
  unitAmount: string;
  unitQuantity: string;
  /** Volume and Graduated: the tiers, the last without an upper bound. */
  tiers: TierDraft[];
};

export type PricingDraft = {
  paid: boolean;
  pricingType: PricingType;
  currency: string;
  billingPeriod: 'MONTH' | 'YEAR';
  /** Flat rate: the amount every billing period. */
  flatAmount: string;
  /** Usage based: the prices. */
  prices: PriceDraft[];
};

/**
 * What is wrong with the draft, for one message in the dialog. `price` is the
 * index of the price at fault; an overlap names both prices and their meter.
 */
export type PricingProblem =
  | { code: 'FLAT_AMOUNT_INVALID' | 'NO_PRICES' }
  | {
      code:
        | 'METER_MISSING'
        | 'AMOUNT_INVALID'
        | 'QUANTITY_INVALID'
        | 'TIERS_INVALID'
        | 'TOO_MANY_FILTERS';
      price: number;
    }
  | { code: 'FILTER_INVALID'; price: number; field: FilterField }
  | { code: 'FILTER_NOT_ALLOWED'; price: number; field: FilterField; meterId: string }
  | {
      code: 'TOKEN_KEY_UNKNOWN' | 'TOKEN_VALUE_INVALID' | 'TOKEN_RANGE_EMPTY';
      price: number;
      eventKey: string;
    }
  | { code: 'OVERLAP'; price: number; other: number; meterId: string };

type Price = NonNullable<SubscriptionPlanPricingBody['prices']>[number];
type Tier = NonNullable<Price['tiers']>[number];

let nextKey = 0;
export const draftKey = (): number => nextKey++;

export const newTier = (): TierDraft => ({
  flatAmount: '',
  key: draftKey(),
  unitAmount: '',
  upTo: '',
});

export const newPrice = (): PriceDraft => ({
  filters: [],
  key: draftKey(),
  meterId: '',
  pricingModel: 'PER_UNIT',
  tiers: [newTier(), newTier()],
  unitAmount: '',
  unitQuantity: '1',
});

/** Whether a filter is on one of `allowed`: its field, and for Token its key too. */
export const isAllowed = (filter: FilterDraft, allowed: PriceField[]): boolean =>
  allowed.some(
    (entry) =>
      entry.field === filter.field &&
      (filter.field !== 'TOKEN' || entry.eventKey === filter.eventKey),
  );

/** An empty row on a field, with the field's first operator. */
export const emptyFilterRow = ({ eventKey, field }: PriceField): FilterDraft => ({
  field,
  operator: FILTER_RULES[field].operators[0],
  text: '',
  ...(eventKey === undefined ? {} : { eventKey }),
});

/**
 * The filter rows of a price, for its meter's fields (`allowed`) in order: one
 * for each fixed field, with the price's filter on it or empty; the price's
 * conditions on each Token key, or one empty; then any filter on a field or
 * Token key the meter does not allow, so it can be seen and emptied.
 */
export const filterRows = (filters: FilterDraft[], allowed: PriceField[]): FilterDraft[] => [
  ...allowed.flatMap((entry) => {
    if (entry.field !== 'TOKEN') {
      return [filters.find((filter) => filter.field === entry.field) ?? emptyFilterRow(entry)];
    }
    const conditions = filters.filter(
      (filter) => filter.field === 'TOKEN' && filter.eventKey === entry.eventKey,
    );
    return conditions.length > 0 ? conditions : [emptyFilterRow(entry)];
  }),
  ...filters.filter((filter) => !isAllowed(filter, allowed)),
];

/** Whether the model prices units in tiers. */
export const isTiered = (model: PricingModel): boolean =>
  model === 'VOLUME' || model === 'GRADUATED';

export const freeDraft = (): PricingDraft => ({
  billingPeriod: 'MONTH',
  currency: 'USD',
  flatAmount: '',
  paid: false,
  prices: [],
  pricingType: 'FLAT_RATE',
});

const draftFromPrice = (price: SubscriptionPlanPrice): PriceDraft => {
  const tiers = (price.tiers ?? []).map((tier) => ({
    flatAmount: tier.flatAmount ?? '',
    key: draftKey(),
    unitAmount: tier.unitAmount ?? '',
    upTo: tier.upTo === null || tier.upTo === undefined ? '' : String(tier.upTo),
  }));
  return {
    filters: (price.filters ?? []).map((filter) => ({
      field: filter.field,
      operator: filter.operator,
      text: filter.values.join(', '),
      ...(filter.eventKey ? { eventKey: filter.eventKey } : {}),
    })),
    key: draftKey(),
    meterId: price.meterId,
    pricingModel: price.pricingModel,
    tiers: tiers.length > 0 ? tiers : [newTier(), newTier()],
    unitAmount: price.unitAmount ?? '',
    unitQuantity: String(price.unitQuantity ?? 1),
  };
};

/** The draft of a plan's stored pricing; a free plan starts unpaid. */
export const draftFromPricing = (pricing: SubscriptionPlanPricing | undefined): PricingDraft => {
  if (!pricing) return freeDraft();
  return {
    billingPeriod: pricing.billingPeriod === 'YEAR' ? 'YEAR' : 'MONTH',
    currency: pricing.currency ?? 'USD',
    flatAmount: pricing.flatAmount ?? '',
    paid: true,
    prices: (pricing.prices ?? []).map(draftFromPrice),
    pricingType: pricing.pricingType === 'USAGE_BASED' ? 'USAGE_BASED' : 'FLAT_RATE',
  };
};

const splitValues = (text: string): string[] => [
  ...new Set(
    text
      .split(',')
      .map((value) => value.trim())
      .filter(Boolean),
  ),
];

/** Whether a filter row was left empty, so the price counts every value of its field. */
const isEmpty = (filter: FilterDraft): boolean => splitValues(filter.text).length === 0;

/** How a Token condition's operator reads in a price name: "prompt tokens > 333". */
const COMPARISONS: Partial<Record<FilterOperator, string>> = {
  EQUALS: '=',
  GREATER_THAN: '>',
  GREATER_THAN_OR_EQUAL: '≥',
  LESS_THAN: '<',
  LESS_THAN_OR_EQUAL: '≤',
};

/**
 * The name a price is shown by, as the API Portal derives it: the meter's name,
 * then each filter in order, "LLM input tokens · LlmProvider", "API calls · not
 * DELETE, OPTIONS", "API calls · /orders…", "Input tokens · prompt tokens > 333"
 * (a Token key by its lower-case label, or the key itself). It is the same in
 * every language, so the words are not translated here.
 */
export const priceName = (meterName: string, filters: FilterDraft[]): string =>
  [
    meterName,
    ...filters.map(({ eventKey, field, operator, text }) => {
      const values = splitValues(text).join(', ');
      if (!values) return '';
      if (field === 'TOKEN') {
        const key = TOKEN_KEYS.find((candidate) => candidate.key === eventKey)?.name ?? eventKey;
        return `${key} ${COMPARISONS[operator] ?? operator} ${values}`;
      }
      if (operator === 'NOT_EQUALS' || operator === 'NOT_IN') return `not ${values}`;
      return operator === 'STARTS_WITH' ? `${values}…` : values;
    }),
  ]
    .filter(Boolean)
    .join(' · ');

/**
 * The name a price is shown by in the plan dialog: its meter's name (its id
 * while the meter is not known), then its filters in the order of the meter's
 * fields.
 */
export const priceLabel = (price: PriceDraft, meter: Meter | undefined): string =>
  priceName(meter?.displayName ?? price.meterId, filterRows(price.filters, priceFieldsOf(meter)));

/**
 * A saved plan's prices in the order they are shown, the API Portal's order
 * too: by name, ignoring case, then by meter id (two meters may share a name;
 * one meter's prices never do, as they would count the same requests). A plan
 * stores no order for its prices. Prices being entered are not sorted; they
 * keep the order they were added in.
 */
export const pricesByName = (prices: PriceDraft[], meters: Meter[]): PriceDraft[] => {
  const meterOf = (price: PriceDraft) => meters.find((meter) => meter.id === price.meterId);
  return prices
    .map((price) => ({ name: priceLabel(price, meterOf(price)), price }))
    .sort(
      (a, b) =>
        a.name.localeCompare(b.name, 'en', { sensitivity: 'base' }) ||
        (a.price.meterId < b.price.meterId ? -1 : Number(a.price.meterId > b.price.meterId)),
    )
    .map(({ price }) => price);
};

/**
 * The whole numbers a price's Token conditions on `key` let through, `low` to `high`; none when
 * low > high.
 */
const tokenRange = (filters: PriceFilter[], key: string): { low: number; high: number } => {
  let low = 0;
  let high = Number.POSITIVE_INFINITY;
  for (const filter of filters) {
    if (filter.field !== 'TOKEN' || filter.eventKey !== key) continue;
    const value = Number(filter.values[0]);
    switch (filter.operator) {
      case 'GREATER_THAN':
        low = Math.max(low, value + 1);
        break;
      case 'GREATER_THAN_OR_EQUAL':
        low = Math.max(low, value);
        break;
      case 'LESS_THAN':
        high = Math.min(high, value - 1);
        break;
      case 'LESS_THAN_OR_EQUAL':
        high = Math.min(high, value);
        break;
      default:
        low = Math.max(low, value);
        high = Math.min(high, value);
    }
  }
  return { high, low };
};

/** The Token keys a price sets conditions on, each once. */
const tokenKeysIn = (filters: PriceFilter[]): string[] => [
  ...new Set(
    filters.flatMap((filter) => (filter.field === 'TOKEN' ? [filter.eventKey ?? ''] : [])),
  ),
];

const isAmount = (value: string): boolean => AMOUNT_PATTERN.test(value.trim());
const isPositive = (value: string): boolean => /[1-9]/.test(value);

/** The tiers of a draft, or null when any of them is invalid. */
const buildTiers = (drafts: TierDraft[]): Tier[] | null => {
  if (drafts.length === 0 || drafts.length > MAX_TIERS) return null;
  let previous = 0;
  const tiers: Tier[] = [];
  for (const [index, tier] of drafts.entries()) {
    const last = index === drafts.length - 1;
    const unitAmount = tier.unitAmount.trim();
    const flatAmount = tier.flatAmount.trim();
    if (
      (unitAmount && !isAmount(unitAmount)) ||
      (flatAmount && !isAmount(flatAmount)) ||
      (!unitAmount && !flatAmount)
    ) {
      return null;
    }
    let upTo: number | null = null;
    if (!last) {
      upTo = Number(tier.upTo.trim());
      if (!tier.upTo.trim() || !Number.isSafeInteger(upTo) || upTo <= previous) return null;
      previous = upTo;
    }
    tiers.push({ flatAmount: flatAmount || null, unitAmount: unitAmount || null, upTo });
  }
  return tiers;
};

/**
 * One price of the request, or the first problem with it. Only the chosen model's fields are given.
 * Filter rows left empty are not sent; the others go in the order the rows are shown. A price
 * filters only on the fields its meter allows, and sets Token conditions only on its meter's Token
 * keys, which together must let some number through; a meter not in `meters` is left to the server
 * to check.
 */
const buildPrice = (
  draft: PriceDraft,
  index: number,
  meters: Meter[],
): { price: Price; problem: null } | { price: null; problem: PricingProblem } => {
  if (!draft.meterId) return { price: null, problem: { code: 'METER_MISSING', price: index } };

  const meter = meters.find((candidate) => candidate.id === draft.meterId);
  const allowed = meter ? priceFieldsOf(meter) : null;
  const rows = allowed ? filterRows(draft.filters, allowed) : draft.filters;
  const given = rows.filter((filter) => !isEmpty(filter));
  const refused = allowed && given.find((filter) => !isAllowed(filter, allowed));
  if (refused) {
    return {
      price: null,
      problem:
        refused.field === 'TOKEN'
          ? { code: 'TOKEN_KEY_UNKNOWN', eventKey: refused.eventKey ?? '', price: index }
          : {
              code: 'FILTER_NOT_ALLOWED',
              field: refused.field,
              meterId: draft.meterId,
              price: index,
            },
    };
  }

  const filters: PriceFilter[] = [];
  for (const filter of given) {
    const values = parseFilterValues(filter.field, filter.operator, filter.text);
    if (filter.field === 'TOKEN') {
      const eventKey = filter.eventKey ?? '';
      if (!values)
        return { price: null, problem: { code: 'TOKEN_VALUE_INVALID', eventKey, price: index } };
      // One whole number, written without leading zeros.
      filters.push({
        eventKey,
        field: 'TOKEN',
        operator: filter.operator,
        values: [String(Number(values[0]))],
      });
      continue;
    }
    if (!values)
      return {
        price: null,
        problem: { code: 'FILTER_INVALID', field: filter.field, price: index },
      };
    // Repeated values count once, in the order given, as the API Portal keeps them.
    filters.push({ field: filter.field, operator: filter.operator, values: [...new Set(values)] });
  }
  if (filters.length > MAX_PRICE_FILTERS)
    return { price: null, problem: { code: 'TOO_MANY_FILTERS', price: index } };
  const impossible = tokenKeysIn(filters).find((eventKey) => {
    const { high, low } = tokenRange(filters, eventKey);
    return low > high;
  });
  if (impossible !== undefined) {
    return {
      price: null,
      problem: { code: 'TOKEN_RANGE_EMPTY', eventKey: impossible, price: index },
    };
  }
  const base = { filters, meterId: draft.meterId, pricingModel: draft.pricingModel };

  if (draft.pricingModel === 'PER_UNIT') {
    if (!isAmount(draft.unitAmount))
      return { price: null, problem: { code: 'AMOUNT_INVALID', price: index } };
    const quantity = Number(draft.unitQuantity.trim());
    if (!Number.isSafeInteger(quantity) || quantity < 1 || quantity > MAX_UNIT_QUANTITY) {
      return { price: null, problem: { code: 'QUANTITY_INVALID', price: index } };
    }
    return {
      price: { ...base, tiers: null, unitAmount: draft.unitAmount.trim(), unitQuantity: quantity },
      problem: null,
    };
  }

  const tiers = buildTiers(draft.tiers);
  if (!tiers) return { price: null, problem: { code: 'TIERS_INVALID', price: index } };
  return { price: { ...base, tiers, unitAmount: null, unitQuantity: null }, problem: null };
};

type Kind = 'match' | 'exclude' | 'prefix';

/**
 * Whether a resource is the path or under it, by whole path segments, as the servers count it:
 * "/orders" covers "/orders" and "/orders/{id}", not "/orders-archive"; "/" covers every resource.
 * A trailing "/" is ignored.
 */
const underPath = (resource: string, path: string): boolean => {
  const base = path.replace(/\/+$/, '') || '/';
  return base === '/' || resource === base || resource.startsWith(`${base}/`);
};

/**
 * Equals and is one of match their values; does not equal and is not one of exclude them; starts
 * with matches a path and what is under it.
 */
const kindOf = (operator: FilterOperator): Kind => {
  if (operator === 'NOT_EQUALS' || operator === 'NOT_IN') return 'exclude';
  return operator === 'STARTS_WITH' ? 'prefix' : 'match';
};

/** Whether two filters on one field can match the same value. */
const filtersOverlap = (field: FilterField, first: PriceFilter, second: PriceFilter): boolean => {
  // Ordered match, then exclude, then prefix, so each pair is handled once.
  const order: Kind[] = ['match', 'exclude', 'prefix'];
  const [a, b] =
    order.indexOf(kindOf(first.operator)) <= order.indexOf(kindOf(second.operator))
      ? [first, second]
      : [second, first];
  const pair = `${kindOf(a.operator)}/${kindOf(b.operator)}`;
  switch (pair) {
    case 'match/match':
      return a.values.some((value) => b.values.includes(value));
    case 'match/exclude':
      return a.values.some((value) => !b.values.includes(value));
    case 'match/prefix':
      return a.values.some((value) => underPath(value, b.values[0]));
    case 'prefix/prefix':
      return underPath(a.values[0], b.values[0]) || underPath(b.values[0], a.values[0]);
    default:
      // Two exclusions, or an exclusion and a prefix, leave values both match; HTTP methods are
      // the seven known ones, so two exclusions of methods leave none when together they cover all
      // seven.
      return (
        field !== 'HTTP_METHOD' ||
        !HTTP_METHODS.every((method) => [...a.values, ...b.values].includes(method))
      );
  }
};

/**
 * Whether one request could be counted by both prices: they are on the same
 * meter; on every fixed field either filters on, the two filters can match the
 * same value (a field only one of them filters on matches anything for the
 * other); and on every Token key, their conditions let some number through for
 * both (a key only one of them sets conditions on lets every number through for
 * the other).
 */
const pricesOverlap = (first: Price, second: Price): boolean => {
  if (first.meterId !== second.meterId) return false;
  const firstFilters = first.filters ?? [];
  const secondFilters = second.filters ?? [];
  const fieldsMeet = firstFilters
    .filter((filter) => filter.field !== 'TOKEN')
    .every((filter) => {
      const other = secondFilters.find((candidate) => candidate.field === filter.field);
      return !other || filtersOverlap(filter.field, filter, other);
    });
  return (
    fieldsMeet &&
    tokenKeysIn([...firstFilters, ...secondFilters]).every((key) => {
      const a = tokenRange(firstFilters, key);
      const b = tokenRange(secondFilters, key);
      return Math.max(a.low, b.low) <= Math.min(a.high, b.high);
    })
  );
};

/**
 * The pricing request for a paid draft, or the first problem with it. An unpaid
 * draft has no body: the plan is free. A flat rate plan sends its amount; a
 * usage based plan its prices, checked against the organization's `meters`.
 */
export const buildPricingBody = (
  draft: PricingDraft,
  meters: Meter[],
): { body: SubscriptionPlanPricingBody | null; problem: PricingProblem | null } => {
  if (!draft.paid) return { body: null, problem: null };
  const base = {
    billingPeriod: draft.billingPeriod,
    currency: draft.currency,
    pricingType: draft.pricingType,
  };

  if (draft.pricingType === 'FLAT_RATE') {
    const flatAmount = draft.flatAmount.trim();
    if (!isAmount(flatAmount) || !isPositive(flatAmount)) {
      return { body: null, problem: { code: 'FLAT_AMOUNT_INVALID' } };
    }
    return { body: { ...base, flatAmount }, problem: null };
  }

  if (draft.prices.length === 0) return { body: null, problem: { code: 'NO_PRICES' } };
  const prices: Price[] = [];
  for (const [index, entry] of draft.prices.entries()) {
    const result = buildPrice(entry, index, meters);
    if (result.problem) return { body: null, problem: result.problem };
    prices.push(result.price);
  }
  for (const [index, price] of prices.entries()) {
    const other = prices.findIndex(
      (candidate, at) => at > index && pricesOverlap(price, candidate),
    );
    if (other >= 0) {
      return {
        body: null,
        problem: { code: 'OVERLAP', meterId: price.meterId, other, price: index },
      };
    }
  }
  return { body: { ...base, prices }, problem: null };
};
