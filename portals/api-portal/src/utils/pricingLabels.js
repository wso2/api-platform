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
 * Display text for plan pricing, shared by the settings page and the API pages: "$99.00 / month",
 * "$0.01 per request, billed monthly", "3 usage prices, billed monthly".
 *
 * Amounts are decimal strings. They are formatted for display only, never computed with. They show
 * the currency's own decimals, and more only where the administrator gave digits that matter: 0.008
 * shows as $0.008, not $0.01; 12.00 in yen as ¥12. The currency's symbol names it ($ is only the US
 * dollar; others have their own, such as A$, CA$ or €, or the code, such as LKR), so no code is
 * added beside an amount.
 */

const { meterUnit, priceName } = require('./meterLabels');

const PERIOD_LABELS = { MONTH: 'month', YEAR: 'year' };

/**
 * "99" in USD -> "$99.00"; "0.008" in USD -> "$0.008"; "12.00" in JPY -> "¥12"; "12" in KWD ->
 * "KWD 12.000"; an unknown currency -> "99.00 XYZ".
 */
function formatAmount(amount, currency) {
    const text = String(amount);
    // Trailing zeros beyond the currency's own decimals say nothing.
    const decimals = text.includes('.') ? text.split('.')[1].replace(/0+$/, '').length : 0;
    try {
        const format = new Intl.NumberFormat('en-US', { style: 'currency', currency, maximumFractionDigits: 12 });
        const minimum = format.resolvedOptions().minimumFractionDigits;
        return new Intl.NumberFormat('en-US', {
            style: 'currency',
            currency,
            minimumFractionDigits: Math.max(minimum, decimals),
            maximumFractionDigits: Math.max(minimum, decimals),
        }).format(Number(text));
    } catch (_err) {
        return `${text} ${currency}`;
    }
}

function periodLabel(billingPeriod) {
    return PERIOD_LABELS[billingPeriod] || String(billingPeriod).toLowerCase();
}

const BILLED = { MONTH: 'billed monthly', YEAR: 'billed yearly' };
const MODEL_NAMES = { VOLUME: 'Volume tiers', GRADUATED: 'Graduated tiers' };

function billedLabel(billingPeriod) {
    return BILLED[billingPeriod] || `billed every ${periodLabel(billingPeriod)}`;
}

/** "1000000" -> "1M", "1000" -> "1K", "250" -> "250": the size of a priced batch of units. */
function quantityLabel(quantity) {
    const n = Number(quantity) || 1;
    if (n >= 1000000 && n % 1000000 === 0) return `${n / 1000000}M`;
    if (n >= 1000 && n % 1000 === 0) return `${n / 1000}K`;
    return n.toLocaleString('en-US');
}

/** "$20.00 / month": a flat rate plan's amount per billing period. */
function feeLabel(amount, currency, billingPeriod) {
    return `${formatAmount(amount, currency)} / ${periodLabel(billingPeriod)}`;
}

/**
 * What a usage price (a PlanPricing DTO's price) charges, in the plan's currency: "$0.50 per 1M
 * tokens", "volume: up to 1,000 at $0.01 per request, then $0.005 per request".
 *
 * @param {object} price    a price of a PlanPricing DTO
 * @param {string} currency the plan's currency
 * @param {string} [unit]   what the price's meter counts, singular ("request", "token")
 */
function priceLabel(price, currency, unit = 'unit') {
    if (price.pricingModel === 'PER_UNIT') {
        const quantity = Number(price.unitQuantity) || 1;
        const per = quantity > 1 ? `${quantityLabel(quantity)} ${unit}s` : unit;
        return `${formatAmount(price.unitAmount, currency)} per ${per}`;
    }
    const tiers = (price.tiers || []).map((tier, i) => {
        const amounts = [
            tier.unitAmount !== null && tier.unitAmount !== undefined ? `${formatAmount(tier.unitAmount, currency)} per ${unit}` : null,
            tier.flatAmount !== null && tier.flatAmount !== undefined ? `${formatAmount(tier.flatAmount, currency)} flat` : null,
        ].filter(Boolean).join(' + ');
        if (tier.upTo === null || tier.upTo === undefined) return i === 0 ? amounts : `then ${amounts}`;
        const from = i === 0 ? 'up to' : 'then up to';
        return `${from} ${Number(tier.upTo).toLocaleString('en-US')} at ${amounts}`;
    });
    const model = price.pricingModel === 'VOLUME' ? 'volume' : 'graduated';
    return `${model}: ${tiers.join(', ')}`;
}

/** The unit a price counts in, from the meters by handle; "unit" when its meter is unknown. */
function unitOf(price, meters) {
    const meter = meters && meters.get(price.meterId);
    return meter ? meterUnit(meter) : 'unit';
}

/** The name of a price (a PlanPricing DTO's price): its meter's name, then its filters. */
function nameOf(price, meters) {
    const meter = meters && meters.get(price.meterId);
    return priceName(meter ? meter.display_name || meter.displayName : price.meterId, price.filters);
}

/**
 * The one-line summary of a plan's pricing (a PlanPricing DTO, or null for a free plan):
 * "$20.00 / month" for a flat rate plan; for a usage based plan "$0.01 per request, billed monthly",
 * "Graduated tiers, billed monthly" or "3 usage prices, billed monthly".
 *
 * @param {object} pricing              a PlanPricing DTO
 * @param {Map<string, object>} [meters] meter handle to a meter row, for the unit
 */
function pricingSummary(pricing, meters) {
    if (!pricing) return 'Free';
    if (pricing.pricingType === 'FLAT_RATE') return feeLabel(pricing.flatAmount, pricing.currency, pricing.billingPeriod);
    const prices = pricing.prices || [];
    const billed = billedLabel(pricing.billingPeriod);
    if (prices.length === 1) {
        const [price] = prices;
        if (price.pricingModel === 'PER_UNIT') return `${priceLabel(price, pricing.currency, unitOf(price, meters))}, ${billed}`;
        return `${MODEL_NAMES[price.pricingModel] || 'Usage'}, ${billed}`;
    }
    return `${prices.length} usage prices, ${billed}`;
}

/**
 * The price lines of a paid plan: a flat rate plan's amount, or each usage price after its name
 * and then how often it is billed: ["LLM input tokens · LlmProvider · $2.50 per 1M tokens", "Billed monthly"].
 *
 * @param {object} pricing              a PlanPricing DTO
 * @param {Map<string, object>} meters  meter handle to a meter row
 */
function pricingLines(pricing, meters) {
    if (!pricing) return [];
    if (pricing.pricingType === 'FLAT_RATE') return [feeLabel(pricing.flatAmount, pricing.currency, pricing.billingPeriod)];
    const lines = (pricing.prices || [])
        .map((price) => `${nameOf(price, meters)} · ${priceLabel(price, pricing.currency, unitOf(price, meters))}`);
    const billed = billedLabel(pricing.billingPeriod);
    return [...lines, billed.charAt(0).toUpperCase() + billed.slice(1)];
}

module.exports = {
    formatAmount, periodLabel, billedLabel, quantityLabel, feeLabel, priceLabel, unitOf, nameOf, pricingSummary,
    pricingLines,
};
