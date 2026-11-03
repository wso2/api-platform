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
  MAX_COMPARISONS,
  isComparison,
  parseFilterValues,
  propertiesOf,
  sameName,
  takesOneValue,
  type FilterOperator,
} from './meterLabels';

/**
 * The plan dialog's Paid plan section as editable state, and its conversion to
 * the pricing request. A paid plan is either:
 *   - flat rate: one amount every billing period; or
 *   - usage based: up to 20 prices, each charging for what a meter counts,
 *     narrowed by the price's own filters on the fields the meter allows, in one
 *     pricing model (Per unit, Volume or Graduated).
 *
 * A price has a filter row for each of its meter's properties, or two when the
 * property has two comparisons, such as more than 100 and at most 500; a row
 * left empty is no filter. A meter may be priced more than once, as long as no
 * request could be counted by two of its prices.
 */

export type PricingType = SubscriptionPlanPricingBody['pricingType'];
export type PricingModel = SubscriptionPlanPrice['pricingModel'];

export const PRICING_TYPES: PricingType[] = ['FLAT_RATE', 'USAGE_BASED'];
export const PRICING_MODELS: PricingModel[] = ['PER_UNIT', 'VOLUME', 'GRADUATED'];
export const MAX_TIERS = 10;
/** A subscription holds at most 20 items, one per price. */
export const MAX_PRICES = 20;
/** At most this many filters in one price, comparisons included. */
export const MAX_PRICE_FILTERS = 20;
const MAX_UNIT_QUANTITY = 1_000_000_000;
const AMOUNT_PATTERN = /^\d{1,12}(\.\d{1,12})?$/;

export type TierDraft = { key: number; upTo: string; unitAmount: string; flatAmount: string };

/**
 * One filter row of a price being edited: the name of the meter's property it
 * filters on, and its values as typed, comma-separated when there are several.
 * A row without values is no filter.
 */
export type FilterDraft = {
  field: string;
  operator: FilterOperator;
  text: string;
};

/** One usage price being edited. */
export type PriceDraft = {
  key: number;
  /** The meter that counts the units. */
  meterId: string;
  /**
   * Which of the meter's requests the price counts, by its properties; rows without values count
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
  | {
      code: 'FILTER_INVALID' | 'NUMBER_INVALID' | 'FILTERS_CONFLICT' | 'RANGE_EMPTY';
      price: number;
      field: string;
    }
  | { code: 'FILTER_NOT_ALLOWED'; price: number; field: string; meterId: string }
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

/** Whether a filter is on one of a meter's `properties`, by name ignoring case. */
export const isAllowed = (filter: FilterDraft, properties: string[]): boolean =>
  properties.some((name) => sameName(name, filter.field));

/** An empty row on a property, with equals as its operator. */
const emptyFilterRow = (field: string): FilterDraft => ({ field, operator: 'EQUALS', text: '' });

/**
 * The filter rows of a price, for its meter's `properties` in order: the
 * price's filters on each property, named as the meter spells it, or one empty
 * row; then any filter on a property the meter does not have.
 */
export const filterRows = (filters: FilterDraft[], properties: string[]): FilterDraft[] => [
  ...properties.flatMap((name) => {
    const given = filters
      .filter((filter) => sameName(filter.field, name))
      .map((filter) => ({ ...filter, field: name }));
    return given.length > 0 ? given : [emptyFilterRow(name)];
  }),
  ...filters.filter((filter) => !isAllowed(filter, properties)),
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

/** A row's values as typed: the one value, or several comma-separated, each once. */
const valuesOf = ({ operator, text }: FilterDraft): string[] =>
  takesOneValue(operator)
    ? [text.trim()].filter(Boolean)
    : [
        ...new Set(
          text
            .split(',')
            .map((value) => value.trim())
            .filter(Boolean),
        ),
      ];

/** Whether a filter row was left empty, so the price counts every value of its property. */
const isEmpty = (filter: FilterDraft): boolean => valuesOf(filter).length === 0;

/** How a comparison reads in a price name: "Prompt tokens > 333". */
const SIGNS: Partial<Record<FilterOperator, string>> = {
  GREATER_THAN: '>',
  GREATER_THAN_OR_EQUAL: '≥',
  LESS_THAN: '<',
  LESS_THAN_OR_EQUAL: '≤',
};

/**
 * The name a price is shown by, as the API Portal derives it: the meter's name,
 * then each filter in short, "LLM input tokens · LlmProvider", "API calls · not
 * DELETE, OPTIONS", "API calls · /orders…", "Requests · Prompt tokens > 333"
 * (a comparison by its property's name, its sign and its number). It is the
 * same in every language, so the words are not translated here.
 */
export const priceName = (meterName: string, filters: FilterDraft[]): string =>
  [
    meterName,
    ...filters.map((filter) => {
      const values = valuesOf(filter).join(', ');
      if (!values) return '';
      const { field, operator } = filter;
      if (isComparison(operator)) return `${field} ${SIGNS[operator]} ${values}`;
      if (operator === 'NOT_EQUALS' || operator === 'NOT_IN') return `not ${values}`;
      return operator === 'STARTS_WITH' ? `${values}…` : values;
    }),
  ]
    .filter(Boolean)
    .join(' · ');

/**
 * The name a price is shown by in the plan dialog: its meter's name (its id
 * while the meter is not known), then its filters in the order of the meter's
 * properties.
 */
export const priceLabel = (price: PriceDraft, meter: Meter | undefined): string =>
  priceName(meter?.displayName ?? price.meterId, filterRows(price.filters, propertiesOf(meter)));

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

type Range = { low: number; high: number };

/** The whole numbers a property's comparisons let through, `low` to `high`; none when low > high. */
const rangeOf = (comparisons: PriceFilter[]): Range => {
  let low = 0;
  let high = Number.POSITIVE_INFINITY;
  for (const filter of comparisons) {
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
      default:
        high = Math.min(high, value);
    }
  }
  return { high, low };
};

/** A price's filters by the property they are on, its name in lower case, in the order given. */
const byProperty = (filters: PriceFilter[]): Map<string, PriceFilter[]> => {
  const groups = new Map<string, PriceFilter[]>();
  for (const filter of filters) {
    const name = filter.field.trim().toLowerCase();
    groups.set(name, [...(groups.get(name) ?? []), filter]);
  }
  return groups;
};

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
 * filters only on its meter's properties, and on each property has one filter that matches text,
 * or one or two comparisons that together let some number through.
 */
const buildPrice = (
  draft: PriceDraft,
  index: number,
  meters: Meter[],
): { price: Price; problem: null } | { price: null; problem: PricingProblem } => {
  if (!draft.meterId) return { price: null, problem: { code: 'METER_MISSING', price: index } };

  const meter = meters.find((candidate) => candidate.id === draft.meterId);
  const properties = meter ? propertiesOf(meter) : null;
  const rows = properties ? filterRows(draft.filters, properties) : draft.filters;
  const given = rows.filter((filter) => !isEmpty(filter));
  const refused = properties && given.find((filter) => !isAllowed(filter, properties));
  if (refused) {
    return {
      price: null,
      problem: {
        code: 'FILTER_NOT_ALLOWED',
        field: refused.field,
        meterId: draft.meterId,
        price: index,
      },
    };
  }

  const filters: PriceFilter[] = [];
  for (const filter of given) {
    const values = parseFilterValues(filter.operator, filter.text);
    if (!values) {
      const code = isComparison(filter.operator) ? 'NUMBER_INVALID' : 'FILTER_INVALID';
      return { price: null, problem: { code, field: filter.field, price: index } };
    }
    filters.push({ field: filter.field, operator: filter.operator, values });
  }
  if (filters.length > MAX_PRICE_FILTERS)
    return { price: null, problem: { code: 'TOO_MANY_FILTERS', price: index } };
  for (const on of byProperty(filters).values()) {
    const field = on[0].field;
    const comparisons = on.filter((filter) => isComparison(filter.operator));
    if (on.length > 1 && (comparisons.length < on.length || on.length > MAX_COMPARISONS))
      return { price: null, problem: { code: 'FILTERS_CONFLICT', field, price: index } };
    const range = rangeOf(comparisons);
    if (comparisons.length > 0 && range.low > range.high)
      return { price: null, problem: { code: 'RANGE_EMPTY', field, price: index } };
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
 * Equals and is one of match their values; does not equal and is not one of exclude them; starts
 * with matches the values that begin with its text.
 */
const kindOf = (operator: FilterOperator): Kind => {
  if (operator === 'NOT_EQUALS' || operator === 'NOT_IN') return 'exclude';
  return operator === 'STARTS_WITH' ? 'prefix' : 'match';
};

/** Whether two filters that match text, on one property, can match the same value. */
const textFiltersOverlap = (first: PriceFilter, second: PriceFilter): boolean => {
  // Ordered match, then exclude, then prefix, so each pair is handled once.
  const order: Kind[] = ['match', 'exclude', 'prefix'];
  const [a, b] =
    order.indexOf(kindOf(first.operator)) <= order.indexOf(kindOf(second.operator))
      ? [first, second]
      : [second, first];
  switch (`${kindOf(a.operator)}/${kindOf(b.operator)}`) {
    case 'match/match':
      return a.values.some((value) => b.values.includes(value));
    case 'match/exclude':
      return a.values.some((value) => !b.values.includes(value));
    case 'match/prefix':
      return a.values.some((value) => value.startsWith(b.values[0]));
    case 'prefix/prefix':
      return a.values[0].startsWith(b.values[0]) || b.values[0].startsWith(a.values[0]);
    default:
      // Two exclusions, or an exclusion and a prefix, leave values both match.
      return true;
  }
};

/**
 * Whether two prices' filters on one property can match the same value: two
 * ranges share a whole number; a range and equals or is one of overlap when one of
 * those values is a whole number in the range; a range and any other filter
 * always overlap.
 */
const propertyFiltersOverlap = (first: PriceFilter[], second: PriceFilter[]): boolean => {
  const firstRange = isComparison(first[0].operator);
  const secondRange = isComparison(second[0].operator);
  if (firstRange && secondRange) {
    const a = rangeOf(first);
    const b = rangeOf(second);
    return Math.max(a.low, b.low) <= Math.min(a.high, b.high);
  }
  if (!firstRange && !secondRange) return textFiltersOverlap(first[0], second[0]);
  const range = rangeOf(firstRange ? first : second);
  const match = firstRange ? second[0] : first[0];
  if (kindOf(match.operator) !== 'match') return true;
  return match.values.some(
    (value) => /^\d+$/.test(value) && Number(value) >= range.low && Number(value) <= range.high,
  );
};

/**
 * Whether one request could be counted by both prices: they are on the same
 * meter, and on every property both filter on, their filters can match the same
 * value. A property only one of them filters on matches anything for the other.
 */
const pricesOverlap = (first: Price, second: Price): boolean => {
  if (first.meterId !== second.meterId) return false;
  const others = byProperty(second.filters ?? []);
  return [...byProperty(first.filters ?? [])].every(([name, filters]) => {
    const other = others.get(name);
    return !other || propertyFiltersOverlap(filters, other);
  });
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
