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
 * Validation of plan pricing requests. Kept free of database and configuration dependencies so
 * the rules can be tested on their own.
 *
 * A paid plan is either flat rate (FLAT_RATE: flatAmount every billing period) or usage based
 * (USAGE_BASED: up to MAX_PRICES prices, all on one invoice per period). Each usage price charges
 * for what one meter (meterId) counts, narrowed by its optional filters, in one pricing model:
 * PER_UNIT charges unitAmount per unitQuantity units; VOLUME prices all the units at the tier their
 * total falls in; GRADUATED prices the units in each tier at that tier's price. Fields of another
 * model are refused. A plan may price one meter several times, as long as no request can be counted
 * by two of its prices. A price has no name of its own: it is named after its meter and filters
 * (meterLabels.priceName).
 *
 * validatePricing checks the request on its own; resolvePrices then checks the prices against the
 * meters they name, which the caller reads.
 */

const {
    COMPARE_OPERATORS, MAX_NAME_LENGTH, MeterValidationError, OPERATORS, normalizeValues, propertyName, sameName,
} = require('./meterValidation');
const { isOfferedCurrency } = require('../../utils/currencies');

const BILLING_PERIODS = ['MONTH', 'YEAR'];
const PRICING_MODELS = ['PER_UNIT', 'VOLUME', 'GRADUATED'];
const AMOUNT_PATTERN = /^\d{1,12}(\.\d{1,12})?$/;
const MAX_TIERS = 10;
const MAX_UNIT_QUANTITY = 1000000000;
const PRICING_TYPES = ['FLAT_RATE', 'USAGE_BASED'];
// A payment provider subscription holds at most 20 items (Stripe), one per usage price.
const MAX_PRICES = 20;
const MAX_FILTERS = 20;
// A meter's id (handle) is at most 40 characters (meterValidation).
const MAX_METER_ID_CHARS = 40;
// The comparisons a price may put on one property: enough for a range ("more than 100", "at most 500").
const MAX_COMPARISONS = 2;

// The fields each model takes, besides meterId and filters.
const MODEL_FIELDS = Object.freeze({
    PER_UNIT: ['unitAmount', 'unitQuantity'],
    VOLUME: ['tiers'],
    GRADUATED: ['tiers'],
});
const PRICE_FIELDS = ['unitAmount', 'unitQuantity', 'tiers'];

class PricingValidationError extends Error {}

function isPositiveAmount(value) {
    return /[1-9]/.test(value);
}

function present(value) {
    return value !== undefined && value !== null && value !== '';
}

function validateAmount(value, name, { required }) {
    if (!present(value)) {
        if (required) throw new PricingValidationError(`${name} is required`);
        return null;
    }
    const text = typeof value === 'number' ? String(value) : value;
    if (typeof text !== 'string' || !AMOUNT_PATTERN.test(text)) {
        throw new PricingValidationError(`${name} must be a decimal amount such as 9.99`);
    }
    return text;
}

function validateTiers(tiers, where) {
    if (!Array.isArray(tiers) || tiers.length === 0 || tiers.length > MAX_TIERS) {
        throw new PricingValidationError(`${where} needs 1 to ${MAX_TIERS} tiers`);
    }
    let previous = 0;
    return tiers.map((tier, i) => {
        const last = i === tiers.length - 1;
        const upTo = tier?.upTo ?? null;
        if (last) {
            if (upTo !== null) throw new PricingValidationError(`The last tier of ${where} must have no upper bound (upTo null)`);
        } else if (!Number.isSafeInteger(upTo) || upTo <= previous) {
            throw new PricingValidationError(`Each tier of ${where} needs an upTo greater than the tier before it`);
        }
        const unitAmount = validateAmount(tier.unitAmount, `unitAmount of tier ${i + 1} of ${where}`, { required: false });
        const flatAmount = validateAmount(tier.flatAmount, `flatAmount of tier ${i + 1} of ${where}`, { required: false });
        if (unitAmount === null && flatAmount === null) {
            throw new PricingValidationError(`Tier ${i + 1} of ${where} needs a unitAmount, a flatAmount or both`);
        }
        if (!last) previous = upTo;
        return { upTo, unitAmount, flatAmount };
    });
}

const isComparison = (f) => COMPARE_OPERATORS.includes(f.operator);

/**
 * The whole numbers a price's comparisons on one property allow, { low, high } (both included); empty
 * when low > high. A price with no comparison on the property allows every number.
 */
function compareRange(filters, name) {
    let low = 0;
    let high = Infinity;
    for (const f of filters) {
        if (!isComparison(f) || !sameName(f.field, name)) continue;
        const n = Number(f.values[0]);
        if (f.operator === 'GREATER_THAN') low = Math.max(low, n + 1);
        else if (f.operator === 'GREATER_THAN_OR_EQUAL') low = Math.max(low, n);
        else if (f.operator === 'LESS_THAN') high = Math.min(high, n - 1);
        else if (f.operator === 'LESS_THAN_OR_EQUAL') high = Math.min(high, n);
    }
    return { low, high };
}

/** The properties a price's filters name, in order, each once (ignoring case). */
function propertiesOf(filters) {
    const names = [];
    for (const f of filters) {
        if (!names.some((n) => sameName(n, f.field))) names.push(f.field);
    }
    return names;
}

/**
 * The filters of a price ([{ field, operator, values }]): at most MAX_FILTERS, each naming a property of
 * the price's meter, with any operator and the values that operator takes, normalized. On one property
 * a price has either one filter that matches text (EQUALS, NOT_EQUALS, IN, NOT_IN, STARTS_WITH) or up
 * to MAX_COMPARISONS comparisons, which must be able to all be true together.
 */
function validateFilters(raw, where) {
    if (raw === undefined || raw === null) return [];
    if (!Array.isArray(raw)) throw new PricingValidationError(`filters of ${where} must be an array`);
    if (raw.length > MAX_FILTERS) throw new PricingValidationError(`${where}: a price has at most ${MAX_FILTERS} filters`);
    const filters = raw.map((entry, k) => {
        if (!entry || typeof entry !== 'object' || Array.isArray(entry)
            || Object.keys(entry).some((key) => !['field', 'operator', 'values'].includes(key))) {
            throw new PricingValidationError(`Each filter of ${where} has only a field, an operator and values`);
        }
        const name = propertyName(entry.field);
        if (!name) {
            throw new PricingValidationError(`field of filter ${k + 1} of ${where} must be 1 to ${MAX_NAME_LENGTH} characters`);
        }
        const operator = String(entry.operator || '').toUpperCase();
        if (!OPERATORS[operator]) {
            throw new PricingValidationError(`The operator of filter ${k + 1} of ${where} must be one of ${Object.keys(OPERATORS).join(', ')}`);
        }
        try {
            return { field: name, operator, values: normalizeValues(name, operator, entry.values) };
        } catch (err) {
            if (err instanceof MeterValidationError) throw new PricingValidationError(`${where}: ${err.message}`);
            throw err;
        }
    });
    for (const name of propertiesOf(filters)) {
        const on = filters.filter((f) => sameName(f.field, name));
        if (on.some((f) => !isComparison(f))) {
            if (on.length > 1) throw new PricingValidationError(`${where} filters on ${name} more than once`);
            continue;
        }
        if (on.length > MAX_COMPARISONS) {
            throw new PricingValidationError(`${where}: a price puts at most ${MAX_COMPARISONS} comparisons on ${name}`);
        }
        const range = compareRange(filters, name);
        if (range.low > range.high) throw new PricingValidationError(`${where}: the comparisons on ${name} can't all be true`);
    }
    return filters;
}

function validatePrice(body, index) {
    const where = `price ${index + 1}`;
    if (!body || typeof body !== 'object' || Array.isArray(body)) {
        throw new PricingValidationError(`${where} is missing or invalid`);
    }
    const pricingModel = String(body.pricingModel || '').toUpperCase();
    if (!PRICING_MODELS.includes(pricingModel)) {
        throw new PricingValidationError(`pricingModel of ${where} must be one of ${PRICING_MODELS.join(', ')}`);
    }
    const allowed = MODEL_FIELDS[pricingModel];
    const extra = PRICE_FIELDS.filter((f) => !allowed.includes(f) && present(body[f]));
    if (extra.length > 0) {
        throw new PricingValidationError(`A ${pricingModel} price (${where}) does not take ${extra.join(', ')}`);
    }
    const meterId = typeof body.meterId === 'string' ? body.meterId.trim() : '';
    if (!meterId) throw new PricingValidationError(`meterId of ${where} is required`);
    if ([...meterId].length > MAX_METER_ID_CHARS) {
        throw new PricingValidationError(`meterId of ${where} must be at most ${MAX_METER_ID_CHARS} characters`);
    }
    const price = {
        meterId, filters: validateFilters(body.filters, where),
        pricingModel, unitAmount: null, unitQuantity: null, tiers: null,
    };
    if (pricingModel === 'PER_UNIT') {
        const unitQuantity = body.unitQuantity ?? 1;
        if (!Number.isSafeInteger(unitQuantity) || unitQuantity < 1 || unitQuantity > MAX_UNIT_QUANTITY) {
            throw new PricingValidationError(`unitQuantity of ${where} must be a whole number from 1 to ${MAX_UNIT_QUANTITY}`);
        }
        price.unitAmount = validateAmount(body.unitAmount, `unitAmount of ${where}`, { required: true });
        price.unitQuantity = unitQuantity;
        return price;
    }
    price.tiers = validateTiers(body.tiers, where);
    return price;
}

/**
 * Validates a pricing request body and returns the normalized pricing, with each price's meter
 * still named by its handle (meterId); see resolvePrices. Throws PricingValidationError with a
 * message fit for the caller.
 */
function validatePricing(body) {
    if (!body || typeof body !== 'object' || Array.isArray(body)) {
        throw new PricingValidationError('Request body is missing or invalid');
    }
    const currency = String(body.currency || '').toUpperCase();
    if (!isOfferedCurrency(currency)) {
        throw new PricingValidationError('currency must be the code of a currency with two decimal places, '
            + 'such as USD or EUR. Currencies with no or three decimal places cannot be used.');
    }
    const billingPeriod = String(body.billingPeriod || 'MONTH').toUpperCase();
    if (!BILLING_PERIODS.includes(billingPeriod)) {
        throw new PricingValidationError(`billingPeriod must be one of ${BILLING_PERIODS.join(', ')}`);
    }
    const pricingType = String(body.pricingType || '').toUpperCase();
    if (!PRICING_TYPES.includes(pricingType)) {
        throw new PricingValidationError(`pricingType must be one of ${PRICING_TYPES.join(', ')}`);
    }
    const rawPrices = body.prices === undefined || body.prices === null ? [] : body.prices;
    if (!Array.isArray(rawPrices)) throw new PricingValidationError('prices must be an array');
    if (pricingType === 'FLAT_RATE') {
        if (rawPrices.length > 0) throw new PricingValidationError('A flat rate plan has no usage prices');
        const flatAmount = validateAmount(body.flatAmount, 'flatAmount', { required: true });
        if (!isPositiveAmount(flatAmount)) {
            throw new PricingValidationError('flatAmount must be more than 0. Create a free plan instead.');
        }
        return { pricingType, currency, billingPeriod, flatAmount, prices: [] };
    }
    if (present(body.flatAmount)) throw new PricingValidationError('A usage based plan has no flat amount');
    if (rawPrices.length === 0 || rawPrices.length > MAX_PRICES) {
        throw new PricingValidationError(`A usage based plan needs 1 to ${MAX_PRICES} prices`);
    }
    return { pricingType, currency, billingPeriod, flatAmount: null, prices: rawPrices.map(validatePrice) };
}

const isMatch = (f) => f.operator === 'EQUALS' || f.operator === 'IN';
const isExclusion = (f) => f.operator === 'NOT_EQUALS' || f.operator === 'NOT_IN';
const isPrefix = (f) => f.operator === 'STARTS_WITH';

/**
 * Whether the text filters two prices put on one property can match the same value. Each is
 * { operator, values }; a value list holds one value for EQUALS, NOT_EQUALS and STARTS_WITH.
 */
function textFiltersOverlap(a, b) {
    if (isMatch(a) && isMatch(b)) return a.values.some((v) => b.values.includes(v));
    if (isMatch(b) && !isMatch(a)) return textFiltersOverlap(b, a);
    if (isMatch(a) && isExclusion(b)) return a.values.some((v) => !b.values.includes(v));
    if (isMatch(a) && isPrefix(b)) return a.values.some((v) => v.startsWith(b.values[0]));
    if (isPrefix(a) && isPrefix(b)) return a.values[0].startsWith(b.values[0]) || b.values[0].startsWith(a.values[0]);
    return true;
}

/**
 * Whether one request can be counted by both prices (on one meter): for every property both filter
 * on, their filters on it can match one value. A property only one of them filters on matches anything
 * for the other.
 */
function pricesOverlap(a, b) {
    return propertiesOf(a.filters).every((name) => {
        const onA = a.filters.filter((f) => sameName(f.field, name));
        const onB = b.filters.filter((f) => sameName(f.field, name));
        if (onB.length === 0) return true;
        const comparesA = onA.every(isComparison);
        const comparesB = onB.every(isComparison);
        if (!comparesA && !comparesB) return textFiltersOverlap(onA[0], onB[0]);
        if (comparesA && comparesB) {
            const ra = compareRange(a.filters, name);
            const rb = compareRange(b.filters, name);
            return Math.max(ra.low, rb.low) <= Math.min(ra.high, rb.high);
        }
        const range = compareRange((comparesA ? a : b).filters, name);
        const text = (comparesA ? onB : onA)[0];
        if (!isMatch(text)) return true;
        return text.values.some((v) => /^\d+$/.test(v) && Number(v) >= range.low && Number(v) <= range.high);
    });
}

/**
 * Checks each price of validated pricing against its meter (`meters`: Map of handle to meter row)
 * and returns the prices with `meter` set, each filter naming its property as the meter spells it.
 * Refuses an unknown meter, a filter on a property the meter does not have, and two prices on one
 * meter that can count the same request.
 */
function resolvePrices(prices, meters) {
    const resolved = prices.map((price, i) => {
        const meter = meters.get(price.meterId);
        if (!meter) throw new PricingValidationError(`Unknown meter: ${price.meterId}`);
        const filters = price.filters.map((f) => {
            const property = (meter.filter_fields || []).find((ff) => sameName(ff.field, f.field));
            if (!property) {
                throw new PricingValidationError(`price ${i + 1}: the meter ${meter.handle} has no property ${f.field}`);
            }
            return { ...f, field: property.field };
        });
        return { ...price, filters, meter };
    });
    resolved.forEach((a, i) => {
        resolved.slice(i + 1).forEach((b, j) => {
            if (a.meter.handle === b.meter.handle && pricesOverlap(a, b)) {
                throw new PricingValidationError(`Prices ${i + 1} and ${i + j + 2} both count the same requests of the meter `
                    + `${a.meter.handle}; give them different filters.`);
            }
        });
    });
    return resolved;
}

module.exports = {
    BILLING_PERIODS,
    PRICING_TYPES,
    PRICING_MODELS,
    MAX_PRICES,
    MAX_FILTERS,
    MAX_COMPARISONS,
    PricingValidationError,
    validatePricing,
    resolvePrices,
};
