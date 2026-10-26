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
 * How paid plans read on an API's page: the price on each plan card, and the details under its
 * Pricing details control.
 */

const meterDao = require('../../dao/meterDao');
const { priceAppliesToApiType } = require('./planPricingValidation');
const { formatAmount, nameOf, periodLabel, priceLabel, unitOf } = require('../../utils/pricingLabels');

const PERIOD_ADVERBS = { MONTH: 'monthly', YEAR: 'yearly' };
const TIERED_TITLES = { VOLUME: 'Volume pricing', GRADUATED: 'Graduated pricing' };
// How each tiered model charges, under its table; `unit` is what the meter counts.
const TIERED_HINTS = {
    VOLUME: (unit) => `All ${unit}s in the period are charged at the tier their total reaches.`,
    GRADUATED: (unit) => `${capitalize(unit)}s in each tier are charged at that tier's price.`,
};

const present = (value) => value !== null && value !== undefined && value !== '';
const capitalize = (text) => text.charAt(0).toUpperCase() + text.slice(1);

/**
 * A tiered price as a table: headers ("Requests", "Per request", "Flat fee", with only the amount columns
 * some tier uses) and one row per tier with a cell per header ("1–10", "$0.05", "–"). Each row also
 * reads as one line for the subscribe dialog: label "1–10 requests", amounts "$0.05 per request".
 */
function tierTable(price, currency, unit) {
    let from = 1;
    const tiers = (price.tiers || []).map((tier) => {
        const open = !present(tier.upTo);
        const range = open
            ? `${from.toLocaleString('en-US')}+`
            : `${from.toLocaleString('en-US')}–${Number(tier.upTo).toLocaleString('en-US')}`;
        if (!open) from = Number(tier.upTo) + 1;
        return {
            range,
            unitAmount: present(tier.unitAmount) ? formatAmount(tier.unitAmount, currency) : null,
            flatAmount: present(tier.flatAmount) ? formatAmount(tier.flatAmount, currency) : null,
        };
    });
    const perUnit = tiers.some((t) => t.unitAmount);
    const flat = tiers.some((t) => t.flatAmount);
    return {
        headers: [capitalize(`${unit}s`), perUnit && `Per ${unit}`, flat && 'Flat fee'].filter(Boolean),
        rows: tiers.map((t) => ({
            cells: [t.range, perUnit && (t.unitAmount || '–'), flat && (t.flatAmount || '–')].filter(Boolean),
            label: `${t.range} ${unit}s`,
            amounts: [t.unitAmount && `${t.unitAmount} per ${unit}`, t.flatAmount && `${t.flatAmount} flat`]
                .filter(Boolean).join(' + '),
        })),
    };
}

/**
 * The least a usage price charges for one unit it counts, to compare prices by: { value, amount,
 * per } with amount and per as the headline shows them ("$0.01", "request"; "$1.00", "1K requests"). A
 * per-unit price's amount over its quantity; a tiered price's lowest tier amount above zero. Null
 * when it charges nothing per unit (only flat amounts, or zero).
 */
function leastPerUnit(price, currency, unit) {
    if (price.pricingModel === 'PER_UNIT') {
        const amount = Number(price.unitAmount);
        if (!(amount > 0)) return null;
        const [shown, ...per] = priceLabel(price, currency, unit).split(' per ');
        return { value: amount / (Number(price.unitQuantity) || 1), amount: shown, per: per.join(' per ') };
    }
    const amounts = (price.tiers || []).map((tier) => tier.unitAmount)
        .filter((amount) => present(amount) && Number(amount) > 0)
        .sort((a, b) => Number(a) - Number(b));
    return amounts.length ? { value: Number(amounts[0]), amount: formatAmount(amounts[0], currency), per: unit } : null;
}

/**
 * The headline of a plan with several usage prices or tiers: the least it charges per unit ("From
 * $0.01 / request"), when every price counts the same unit and one charges per unit; else null.
 */
function fromPrice(prices, currency, meters) {
    const units = new Set(prices.map((price) => unitOf(price, meters)));
    if (units.size !== 1) return null;
    const least = prices.map((price) => leastPerUnit(price, currency, unitOf(price, meters)))
        .filter(Boolean).sort((a, b) => a.value - b.value);
    return least[0] || null;
}

/** A usage price under Pricing details: { name, text, tiers, hint } (tiers and hint for a tiered price). */
function priceItem(price, currency, unit, name) {
    if (price.pricingModel === 'PER_UNIT') {
        return { name, text: priceLabel(price, currency, unit), tiers: null, hint: null };
    }
    const hint = TIERED_HINTS[price.pricingModel];
    return {
        name,
        text: TIERED_TITLES[price.pricingModel] || 'Usage-based',
        tiers: tierTable(price, currency, unit),
        hint: hint ? hint(unit) : null,
    };
}

/**
 * Adds to each paid plan (as appendSubscriptionPlanDetails returns them) of an API of `apiType` the
 * text its card shows, from the usage prices that apply to that type (priceAppliesToApiType; the
 * others are not sold on the API):
 *   priceAmount / priceUnit  the headline: a flat rate plan's amount ("$20.00" "/ month"), else the
 *                            one per-unit price ("$0.01" "/ request"), else the least per unit with
 *   pricePrefix              "From" ("From $0.01 / request"), else "Pay per use" (prices that count
 *                            different units, or none per unit);
 *   billingNote              "Billed monthly" under a usage based headline (a flat rate's unit
 *                            already names the period);
 *   priceItems               one { name, text, tiers, hint } per usage price not in the headline,
 *                            shown under Pricing details; named only when there are several, since
 *                            the meter's name means nothing to a subscriber beside its unit.
 * The amounts carry the currency's symbol, such as $, A$ or €, or its code.
 * A price is named after its meter and values ("Requests · GET") and says what is counted
 * ("request").
 */
async function decoratePlans(orgId, plans, apiType) {
    const paid = (plans || []).filter((p) => p.paid && p.pricing);
    if (paid.length === 0) return;
    const meters = new Map((await meterDao.list(orgId)).map((m) => [m.handle, m]));

    for (const plan of paid) {
        const pricing = plan.pricing;
        const prices = (pricing.prices || [])
            .filter((price) => priceAppliesToApiType(price.filters, meters.get(price.meterId), apiType));
        let listed = prices;
        plan.pricePrefix = null;
        if (pricing.pricingType === 'FLAT_RATE') {
            plan.priceAmount = formatAmount(pricing.flatAmount, pricing.currency);
            plan.priceUnit = `/ ${periodLabel(pricing.billingPeriod)}`;
        } else if (prices.length === 1 && prices[0].pricingModel === 'PER_UNIT') {
            const [amount, ...per] = priceLabel(prices[0], pricing.currency, unitOf(prices[0], meters)).split(' per ');
            plan.priceAmount = amount;
            plan.priceUnit = `/ ${per.join(' per ')}`;
            listed = [];
        } else {
            const from = fromPrice(prices, pricing.currency, meters);
            plan.pricePrefix = from ? 'From' : null;
            plan.priceAmount = from ? from.amount : 'Pay per use';
            plan.priceUnit = from ? `/ ${from.per}` : '';
        }
        const named = listed.length > 1;
        plan.priceItems = listed.map((price) => priceItem(price, pricing.currency, unitOf(price, meters),
            named ? nameOf(price, meters) : null));
        plan.billingNote = pricing.pricingType === 'FLAT_RATE' ? null
            : `Billed ${PERIOD_ADVERBS[pricing.billingPeriod] || String(pricing.billingPeriod).toLowerCase()}`;
    }
}

module.exports = { decoratePlans };
