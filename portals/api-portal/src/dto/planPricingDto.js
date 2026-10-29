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
const { applyAudit } = require('./dtoUtils');

class PlanPricing {
    /**
     * @param {object} pricing a planPricingDao row, with its prices
     * @param {object} [audit]
     */
    constructor(pricing, audit) {
        this.pricingType = pricing.pricing_type;
        this.currency = pricing.currency;
        this.billingPeriod = pricing.billing_period;
        this.flatAmount = pricing.flat_amount ?? null;
        this.prices = (pricing.prices || []).map((price) => ({
            meterId: price.meter_handle || null,
            filters: (price.filters || []).map((f) => ({ field: f.field, operator: f.operator, values: f.values || [] })),
            pricingModel: price.pricing_model,
            unitAmount: price.unit_amount ?? null,
            unitQuantity: price.unit_quantity ?? null,
            tiers: price.tiers || null,
        }));
        applyAudit(this, audit);
    }
}

module.exports = PlanPricing;
