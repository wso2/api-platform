/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
'use strict';

/*
 * The Moesif billing catalog objects for one paid plan on one API. Moesif links one billing meter
 * to a catalog plan, so each part of the plan gets its own catalog plan (a Stripe product): one with
 * a flat price for a flat rate plan, and one with a metered price and a billing meter for each usage
 * price. The meter counts the subscription's calls and reports them to Stripe against that price.
 *
 * Only payloads are built here, so their shapes are tested in one place. The shapes follow
 * Moesif's management API reference, checked against Moesif: amounts as decimal strings,
 * `period_units` M or y, tiers with `up_to` "inf" on the last tier.
 *
 * The billing meter's es_query is what Moesif searches its API call events with. The event fields
 * a meter reads are stored on the meter, as full paths in the Moesif event: the number a TOKENS
 * meter adds up (type_key, for our gateway `metadata.aiTokenUsage.promptTokens`,
 * `completionTokens` or `totalTokens`) and the field each filter matches (filter_fields[].eventKey,
 * for our gateway `response.status`, `request.verb`, `metadata.apiResourceTemplate` and
 * `metadata.apiType`). Status codes are numbers in the events and are matched on the
 * key itself; other values are matched exactly on the key's `.raw` keyword field. A Token condition
 * compares the number at the key it names with a `range` condition (EQUALS is gte and lte).
 *
 * Every event must also carry billing metadata, `metadata.billingCustomerId` and
 * `metadata.billingSubscriptionId`, matched against Moesif's {{company_id}} and {{subscription_id}}
 * templates, so each meter counts only one Stripe subscription's calls. These keys are fixed: a
 * gateway that does not send them can be metered, but not billed.
 *
 * A meter counts calls, or adds up a number of each call. A price narrows what its meter counts with
 * its own filters (priceMeter).
 */

const FIELDS = Object.freeze({
    customer: 'metadata.billingCustomerId.raw',
    subscription: 'metadata.billingSubscriptionId.raw',
    weight: 'weight',
    time: 'request.time',
});

const REPORTING_PERIOD = '15m';
const PERIOD_UNITS = { MONTH: 'M', YEAR: 'y' };
// A price's pricing model, as Moesif names it.
const PRICING_MODELS = { PER_UNIT: 'per_unit', VOLUME: 'volume', GRADUATED: 'tiered' };

/** A sync step the portal refuses before calling Moesif; the message is shown to administrators. */
class CatalogInputError extends Error {}

/**
 * The event field a filter matches and how its values are written: a status code as a number on the
 * meter's key, any other value as text on the key's keyword field.
 */
function filterTarget(meter, field) {
    const entry = (meter.filter_fields || []).find((f) => f.field === field);
    if (!entry || !entry.eventKey) {
        throw new CatalogInputError(`The meter ${meter.handle} has no event key for the ${field} filter.`);
    }
    return field === 'HTTP_STATUS_CODE'
        ? { field: entry.eventKey, value: Number }
        : { field: `${entry.eventKey}.raw`, value: String };
}

// The range bounds of a Token condition's operator.
const RANGE_BOUNDS = Object.freeze({
    GREATER_THAN: ['gt'],
    GREATER_THAN_OR_EQUAL: ['gte'],
    LESS_THAN: ['lt'],
    LESS_THAN_OR_EQUAL: ['lte'],
    EQUALS: ['gte', 'lte'],
});

/** A Token condition as a range condition on the number at its key: { range: { key: { gt: 333 } } }. */
function tokenClause(f) {
    const bounds = RANGE_BOUNDS[f.operator];
    if (!bounds) throw new CatalogInputError(`The ${f.operator} operator is not supported for a Token condition.`);
    const n = Number(f.values[0]);
    return { range: { [f.eventKey]: Object.fromEntries(bounds.map((b) => [b, n])) } };
}

/**
 * A STARTS_WITH condition: the path itself or a resource under it, by whole path segments, so
 * "/orders" matches "/orders" and "/orders/{id}" but not "/orders-archive" (meterValidation.underPath).
 * The path is stored without a trailing "/"; "/" matches every resource.
 */
function underPathClause(field, path) {
    if (path === '/') return { prefix: { [field]: '/' } };
    return { bool: { should: [{ terms: { [field]: [path] } }, { prefix: { [field]: `${path}/` } }], minimum_should_match: 1 } };
}

/**
 * The clauses a price's filters add on its meter (from priceMeter): { must: [...], mustNot: [...] }.
 * EQUALS and IN match the values; NOT_EQUALS and NOT_IN match calls that have the field with another
 * value (a call without the field, such as one from a gateway that does not send it, is not
 * counted); STARTS_WITH matches a path and what is under it; and each Token condition is a range on
 * its key.
 */
function filterClauses(meter) {
    const must = [];
    const mustNot = [];
    for (const f of meter.filters || []) {
        if (f.field === 'TOKEN') {
            must.push(tokenClause(f));
            continue;
        }
        const target = filterTarget(meter, f.field);
        const values = f.values.map(target.value);
        switch (f.operator) {
            case 'EQUALS':
            case 'IN':
                must.push({ terms: { [target.field]: values } });
                break;
            case 'NOT_EQUALS':
            case 'NOT_IN':
                must.push({ exists: { field: target.field } });
                mustNot.push({ terms: { [target.field]: values } });
                break;
            case 'STARTS_WITH':
                must.push(underPathClause(target.field, values[0]));
                break;
            default:
                throw new CatalogInputError(`The ${f.operator} operator is not supported.`);
        }
    }
    return { must, mustNot };
}

/** What a price counts with: its meter, narrowed by the price's filters ([{ field, operator, values }]). */
function priceMeter(meter, filters) {
    return { ...meter, filters: filters || [] };
}

/** What a meter adds up per counted call: each call (its weight), or the number at its event key. */
function usageSum(meter) {
    if (meter.type === 'REQUEST_COUNT') return { sum: { field: FIELDS.weight, missing: 1 } };
    if (!meter.type_key) {
        throw new CatalogInputError(`The meter ${meter.handle} does not say which event key it adds up.`);
    }
    return { sum: { field: meter.type_key, missing: 0 } };
}

/**
 * The calls a price's meter (from priceMeter) counts for one subscription: those carrying its
 * billing customer and subscription ids that match the price's filters. `customerId` and
 * `subscriptionId` are values, or Moesif's {{company_id}} and {{subscription_id}} templates in a
 * billing meter.
 */
function meterFilter(meter, { customerId, subscriptionId }) {
    const { must, mustNot } = filterClauses(meter);
    const bool = {
        must: [
            { terms: { [FIELDS.customer]: [customerId] } },
            { terms: { [FIELDS.subscription]: [subscriptionId] } },
            ...must,
        ],
    };
    if (mustNot.length > 0) bool.must_not = mustNot;
    return { bool };
}

/** The search a billing meter runs for one subscription: its matching calls, summed. */
function meterQuery(meter) {
    return {
        query: meterFilter(meter, { customerId: '{{company_id}}', subscriptionId: '{{subscription_id}}' }),
        aggs: { seg: { filter: { match_all: {} }, aggs: { usage_value: usageSum(meter) } } },
        size: 0,
    };
}

// Moesif's bucket size for each interval of a usage series.
const SERIES_INTERVALS = Object.freeze({ HOUR: '1h', DAY: '1d', WEEK: '1w' });

/**
 * The same count as the billing meter, for one subscription, per hour, day or week (`interval`):
 * the usage a subscriber sees. Used with Moesif's event search, whose time field is `request.time`.
 */
function usageSearch(meter, ids, interval = 'DAY', timeZone = 'UTC') {
    return {
        query: meterFilter(meter, ids),
        aggs: {
            series: {
                date_histogram: {
                    field: FIELDS.time, calendar_interval: SERIES_INTERVALS[interval], time_zone: timeZone, min_doc_count: 0,
                },
                aggs: { usage: usageSum(meter) },
            },
        },
        size: 0,
    };
}

/** [{ start, value }] from a usageSearch response; start is the bucket's start (ISO 8601). */
function usageBuckets(response) {
    const buckets = response && response.aggregations && response.aggregations.series
        && Array.isArray(response.aggregations.series.buckets) ? response.aggregations.series.buckets : [];
    return buckets.map((b) => {
        const start = Number.isFinite(Number(b.key)) ? new Date(Number(b.key)) : new Date(b.key_as_string);
        const value = b.usage && Number.isFinite(Number(b.usage.value)) ? Number(b.usage.value) : Number(b.doc_count) || 0;
        return { start: start.toISOString(), value };
    });
}

/**
 * `provider` in the builders below is the billing provider Moesif syncs the catalog to, by the
 * name Moesif knows it ("stripe"); the payment sub-connector supplies it.
 */
function catalogPlanPayload(provider, name) {
    return {
        name,
        provider,
        status: 'active',
        billing_type: 'continuous_reporting',
        reporting_period: REPORTING_PERIOD,
    };
}

function basePrice(provider, planId, pricing, name) {
    return {
        name,
        plan_id: planId,
        status: 'active',
        provider,
        currency: pricing.currency.toLowerCase(),
        period: 1,
        period_units: PERIOD_UNITS[pricing.billing_period] || 'M',
    };
}

/** A flat rate plan's price (a planPricingDao row with flat_amount), charged every period. */
function flatPricePayload(provider, planId, name, pricing) {
    return {
        ...basePrice(provider, planId, pricing, name),
        pricing_model: 'flat',
        price_in_decimal: pricing.flat_amount,
        // Moesif requires it on every price; a flat price still becomes a fixed (licensed) one.
        usage_aggregator: 'sum',
    };
}

/**
 * A usage price of a plan (one of a planPricingDao row's prices), whose quantity the price's
 * billing meter reports, summed over the period.
 */
function usagePricePayload(provider, planId, name, pricing, price) {
    const payload = {
        ...basePrice(provider, planId, pricing, name),
        pricing_model: PRICING_MODELS[price.pricing_model],
        usage_aggregator: 'sum',
    };
    if (price.pricing_model === 'PER_UNIT') {
        payload.price_in_decimal = price.unit_amount;
        if (Number(price.unit_quantity) > 1) {
            payload.transform_quantity = { divide_by: Number(price.unit_quantity), round: 'up' };
        }
    } else {
        payload.tiers = (price.tiers || []).map((tier) => {
            const out = { up_to: tier.upTo === null || tier.upTo === undefined ? 'inf' : tier.upTo };
            if (tier.unitAmount !== null && tier.unitAmount !== undefined) out.unit_price_in_decimal = tier.unitAmount;
            if (tier.flatAmount !== null && tier.flatAmount !== undefined) out.flat_price_in_decimal = tier.flatAmount;
            return out;
        });
    }
    return payload;
}

// Moesif accepts only its own billing meter slugs (another slug is refused as an invalid meter);
// every meter its dashboard creates uses this one. A meter is known by its id, not its slug.
const BILLING_METER_SLUG = 'hourly_usage';

// The view of a meter's usage in the Moesif dashboard, which a billing meter must have: the sum of
// events per company.
const BILLING_METER_URL_QUERY = '?seg[metrics][0][metricName]=sum(events)&seg[groups][0][field]=company_id.raw'
    + '&seg[groups][0][func]=terms&seg[groups][0][buckets]=25&seg[chartType]=table';

/**
 * A billing meter: counts one subscription's calls that match a price's meter (`meter`, from
 * priceMeter) in its es_query and reports them to the billing provider. `providerParams` is the
 * provider's part of the meter's billing plan (for Stripe, the product and price to report to),
 * which the payment sub-connector builds.
 */
function billingMeterPayload({ provider, providerParams, name, meter }) {
    return {
        name,
        slug: BILLING_METER_SLUG,
        url_query: BILLING_METER_URL_QUERY,
        status: 'active',
        zero_balance_behavior: 'bill_customer',
        es_query: meterQuery(meter),
        billing_plan: {
            provider_slug: provider,
            params: {
                usage_multiplier: '1.0',
                usage_rounding_mode: 'up',
                ...providerParams,
            },
        },
    };
}

module.exports = {
    FIELDS,
    REPORTING_PERIOD,
    CatalogInputError,
    priceMeter,
    meterQuery,
    SERIES_INTERVALS,
    usageSearch,
    usageBuckets,
    catalogPlanPayload,
    flatPricePayload,
    usagePricePayload,
    billingMeterPayload,
};
