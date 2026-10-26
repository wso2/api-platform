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
    FILTER_RULES, HTTP_METHODS, MAX_TOKEN_VALUE, MCP_TOOL_CALL, MeterValidationError, normalizeValues, underPath,
} = require('./meterValidation');
const { isOfferedCurrency } = require('../../utils/currencies');
const constants = require('../../utils/constants');

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
// The conditions a price may put on one Token key: enough for a range ("more than 100", "at most 500").
const MAX_TOKEN_CONDITIONS = 2;

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

/** A Token condition's value: one whole number from 0 to MAX_TOKEN_VALUE, as written in digits. */
function tokenValue(values, where) {
    const raw = Array.isArray(values) && values.length === 1 ? values[0] : null;
    const text = typeof raw === 'number' ? String(raw) : typeof raw === 'string' ? raw.trim() : '';
    if (!/^\d+$/.test(text) || Number(text) > MAX_TOKEN_VALUE) {
        throw new PricingValidationError(`${where}: the Token value must be a whole number from 0 to ${MAX_TOKEN_VALUE}`);
    }
    return String(Number(text));
}

/**
 * The whole numbers a price's conditions on one Token key allow, { low, high } (both included);
 * empty when low > high. A price with no condition on the key allows every count.
 */
function tokenRange(filters, key) {
    let low = 0;
    let high = Infinity;
    for (const f of filters) {
        if (f.field !== 'TOKEN' || f.eventKey !== key) continue;
        const n = Number(f.values[0]);
        if (f.operator === 'GREATER_THAN') low = Math.max(low, n + 1);
        else if (f.operator === 'GREATER_THAN_OR_EQUAL') low = Math.max(low, n);
        else if (f.operator === 'LESS_THAN') high = Math.min(high, n - 1);
        else if (f.operator === 'LESS_THAN_OR_EQUAL') high = Math.min(high, n);
        else if (f.operator === 'EQUALS') { low = Math.max(low, n); high = Math.min(high, n); }
    }
    return { low, high };
}

/** The Token keys a price's filters compare, in order, each once. */
function tokenKeysOf(filters) {
    return [...new Set(filters.filter((f) => f.field === 'TOKEN').map((f) => f.eventKey))];
}

/** A Token condition ({ field: TOKEN, eventKey, operator, values: [n] }), normalized. */
function validateTokenCondition(entry, operator, where) {
    const key = typeof entry.eventKey === 'string' ? entry.eventKey.trim() : '';
    if (!key) throw new PricingValidationError(`${where}: a Token filter needs an eventKey`);
    if (!FILTER_RULES.TOKEN.operators.includes(operator)) {
        throw new PricingValidationError(`The TOKEN operator of ${where} must be one of ${FILTER_RULES.TOKEN.operators.join(', ')}`);
    }
    return { field: 'TOKEN', eventKey: key, operator, values: [tokenValue(entry.values, where)] };
}

/**
 * The filters of a price: at most MAX_FILTERS. A fixed field ([{ field, operator, values }]) at most
 * once, with an operator it accepts and the values that operator takes, normalized. A Token condition
 * ({ field: TOKEN, eventKey, operator, values }) compares the number at one of the meter's Token keys;
 * a price puts at most MAX_TOKEN_CONDITIONS on a key, which must be able to all be true together.
 */
function validateFilters(raw, where) {
    if (raw === undefined || raw === null) return [];
    if (!Array.isArray(raw)) throw new PricingValidationError(`filters of ${where} must be an array`);
    if (raw.length > MAX_FILTERS) throw new PricingValidationError(`${where}: a price has at most ${MAX_FILTERS} filters`);
    const seen = new Set();
    const filters = raw.map((entry, k) => {
        if (!entry || typeof entry !== 'object' || Array.isArray(entry)) {
            throw new PricingValidationError(`Each filter of ${where} needs a field, an operator and values`);
        }
        const field = String(entry.field || '').toUpperCase();
        const operator = String(entry.operator || '').toUpperCase();
        const rule = FILTER_RULES[field];
        if (!rule) {
            throw new PricingValidationError(`field of filter ${k + 1} of ${where} must be one of ${Object.keys(FILTER_RULES).join(', ')}`);
        }
        if (field === 'TOKEN') return validateTokenCondition(entry, operator, where);
        if (entry.eventKey !== undefined && entry.eventKey !== null
            && (typeof entry.eventKey !== 'string' || entry.eventKey.trim() !== '')) {
            throw new PricingValidationError(`${where}: only a Token filter takes an eventKey`);
        }
        if (seen.has(field)) throw new PricingValidationError(`${where} filters on ${field} more than once`);
        seen.add(field);
        if (!rule.operators.includes(operator)) {
            throw new PricingValidationError(`The ${field} operator of ${where} must be one of ${rule.operators.join(', ')}`);
        }
        try {
            return { field, operator, values: normalizeValues(field, operator, entry.values) };
        } catch (err) {
            if (err instanceof MeterValidationError) throw new PricingValidationError(`${where}: ${err.message}`);
            throw err;
        }
    });
    for (const key of tokenKeysOf(filters)) {
        if (filters.filter((f) => f.field === 'TOKEN' && f.eventKey === key).length > MAX_TOKEN_CONDITIONS) {
            throw new PricingValidationError(`${where}: a price puts at most ${MAX_TOKEN_CONDITIONS} Token conditions on ${key}`);
        }
        const range = tokenRange(filters, key);
        if (range.low > range.high) throw new PricingValidationError(`${where}: the Token conditions on ${key} can't all be true`);
    }
    if (seen.has('MCP_TOOL') && seen.has('MCP_METHOD')) {
        throw new PricingValidationError(`${where}: a Tool filter counts only tool calls, so it can't be used with an MCP method filter`);
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

/**
 * Whether two filters on one field can match the same value. Each is { operator, values }; a value
 * list holds one value for EQUALS, NOT_EQUALS and STARTS_WITH. A STARTS_WITH filter matches its path
 * and the resources under it (underPath).
 */
function filtersOverlap(field, a, b) {
    const isMatch = (f) => f.operator === 'EQUALS' || f.operator === 'IN';
    const isExclusion = (f) => f.operator === 'NOT_EQUALS' || f.operator === 'NOT_IN';
    const isPrefix = (f) => f.operator === 'STARTS_WITH';
    if (isMatch(a) && isMatch(b)) return a.values.some((v) => b.values.includes(v));
    if (isMatch(b) && !isMatch(a)) return filtersOverlap(field, b, a);
    if (isMatch(a) && isExclusion(b)) return a.values.some((v) => !b.values.includes(v));
    if (isMatch(a) && isPrefix(b)) return a.values.some((v) => underPath(v, b.values[0]));
    if (isPrefix(a) && isPrefix(b)) return underPath(a.values[0], b.values[0]) || underPath(b.values[0], a.values[0]);
    // Negative against negative or a prefix: some other value matches both, unless the field's
    // values are known and the two exclusions cover them all (HTTP methods).
    if (field === 'HTTP_METHOD' && isExclusion(a) && isExclusion(b)) {
        return HTTP_METHODS.some((m) => !a.values.includes(m) && !b.values.includes(m));
    }
    return true;
}

/**
 * A price's filters with the condition a Tool filter implies: a Tool filter counts only tool calls,
 * so it comes with the MCP method tools/call (a price never filters on both).
 */
function withToolCall(filters) {
    if (!(filters || []).some((f) => f.field === 'MCP_TOOL')) return filters || [];
    return [...filters, { field: 'MCP_METHOD', operator: 'EQUALS', values: [MCP_TOOL_CALL] }];
}

/**
 * What a price counts, as the billing provider is given it: its meter and filters (withToolCall). The
 * MCP method a Tool filter implies is read at the meter's MCP method key or, when the meter does not
 * list that field, at our gateway's.
 */
function countedFilters(meter, filters) {
    const counted = withToolCall(filters);
    const fields = (meter && meter.filter_fields) || [];
    if (!(filters || []).some((f) => f.field === 'MCP_TOOL') || fields.some((f) => f.field === 'MCP_METHOD')) {
        return { meter, filters: counted };
    }
    const method = { field: 'MCP_METHOD', eventKey: FILTER_RULES.MCP_METHOD.eventKey };
    return { meter: { ...meter, filter_fields: [...fields, method] }, filters: counted };
}

/**
 * Whether one request can be counted by both prices (on one meter): for every fixed field either
 * price filters on, the two can match the same value, and for every Token key either compares, the
 * two allow a common count. A field or key only one of them filters on matches anything for the
 * other. A Tool filter's tool call counts as an MCP method filter (withToolCall).
 */
function pricesOverlap(a, b) {
    const fixed = (filters) => withToolCall(filters).filter((f) => f.field !== 'TOKEN');
    const filters = new Map(fixed(b.filters).map((f) => [f.field, f]));
    if (!fixed(a.filters).every((f) => !filters.has(f.field) || filtersOverlap(f.field, f, filters.get(f.field)))) return false;
    return tokenKeysOf([...a.filters, ...b.filters]).every((key) => {
        const ra = tokenRange(a.filters, key);
        const rb = tokenRange(b.filters, key);
        return Math.max(ra.low, rb.low) <= Math.min(ra.high, rb.high);
    });
}

/** The Token keys a meter's prices can compare: its Token row's, then its Token fields'. */
function tokenKeysOfMeter(meter) {
    return [meter.type_key, ...(meter.filter_fields || []).filter((f) => f.field === 'TOKEN').map((f) => f.eventKey)]
        .filter(Boolean);
}

/**
 * Checks each price of validated pricing against its meter (`meters`: Map of handle to meter row)
 * and returns the prices with `meter` set. Refuses an unknown meter, a filter on a field the meter
 * does not let its prices filter on, and two prices on one meter that can count the same request.
 */
function resolvePrices(prices, meters) {
    const resolved = prices.map((price, i) => {
        const meter = meters.get(price.meterId);
        if (!meter) throw new PricingValidationError(`Unknown meter: ${price.meterId}`);
        const unknownKey = price.filters.find((f) => f.field === 'TOKEN' && !tokenKeysOfMeter(meter).includes(f.eventKey));
        if (unknownKey) {
            throw new PricingValidationError(`price ${i + 1}: the meter has no Token field with the key ${unknownKey.eventKey}`);
        }
        const refused = price.filters.find((f) => f.field !== 'TOKEN'
            && !(meter.filter_fields || []).some((ff) => ff.field === f.field));
        if (refused) {
            throw new PricingValidationError(`price ${i + 1}: the meter ${meter.handle} does not allow filtering on `
                + `${refused.field}`);
        }
        return { ...price, meter };
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

/**
 * The API type our gateway writes in usage events (metadata.apiType) for an api_metadata.type. The
 * names are the same (RestApi, Mcp, WebSubApi) except for GraphQL and WebSocket APIs; a SOAP API has
 * no counterpart, so it gives undefined.
 */
const GATEWAY_API_TYPES = {
    [constants.API_TYPE.GRAPHQL]: 'GraphQLApi',
    [constants.API_TYPE.WS]: 'WebSocketApi',
    [constants.API_TYPE.SOAP]: undefined,
};
const gatewayApiType = (apiType) => (Object.prototype.hasOwnProperty.call(GATEWAY_API_TYPES, apiType)
    ? GATEWAY_API_TYPES[apiType] : apiType);

/**
 * Whether a usage price can count calls to an API of `apiType` (api_metadata.type, such as RestApi,
 * Mcp or GRAPHQL). Two kinds of filter can rule an API out. An API_TYPE filter: EQUALS and IN count
 * the types they list, NOT_EQUALS and NOT_IN the others, compared with the type our gateway writes for
 * the API (gatewayApiType). An MCP method or Tool filter: only calls to MCP servers have those fields,
 * and every operator needs the field. Each check holds only when the price's meter reads the field at
 * the default event key, which our gateway fills; with any other key, or for a type the gateway has no
 * counterpart for, the API's value is unknown, so the price applies. `filters` are the price's filters
 * and `meter` its meter row (with filter_fields).
 */
function priceAppliesToApiType(filters, meter, apiType) {
    const gatewayType = gatewayApiType(apiType);
    if (!gatewayType) return true;
    const readsDefault = (field) => ((meter && meter.filter_fields) || [])
        .some((f) => f.field === field && f.eventKey === FILTER_RULES[field].eventKey);
    const mcpOnly = (filters || []).some((f) => (f.field === 'MCP_METHOD' || f.field === 'MCP_TOOL') && readsDefault(f.field));
    if (mcpOnly && gatewayType !== 'Mcp') return false;
    const filter = (filters || []).find((f) => f.field === 'API_TYPE');
    if (!filter || !readsDefault('API_TYPE')) return true;
    const listed = filter.values.includes(gatewayType);
    return filter.operator === 'EQUALS' || filter.operator === 'IN' ? listed : !listed;
}

module.exports = {
    BILLING_PERIODS,
    PRICING_TYPES,
    PRICING_MODELS,
    MAX_PRICES,
    MAX_FILTERS,
    MAX_TOKEN_CONDITIONS,
    PricingValidationError,
    validatePricing,
    resolvePrices,
    countedFilters,
    gatewayApiType,
    priceAppliesToApiType,
};
