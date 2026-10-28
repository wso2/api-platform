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
 *
 */

/*
 * Tag: Billing
 */
const billingConfigurationService = require('../../../services/billing/billingConfigurationService');
const checkoutService = require('../../../services/billing/checkoutService');
const meterService = require('../../../services/billing/meterService');
const planPricingService = require('../../../services/billing/planPricingService');
const subscriptionBillingService = require('../../../services/billing/subscriptionBillingService');
const usageService = require('../../../services/billing/usageService');
const revenueService = require('../../../services/billing/revenueService');
const billingAccountService = require('../../../services/billing/billingAccountService');

module.exports = {
    getBillingConfiguration: billingConfigurationService.getBillingConfiguration,
    createBillingConfiguration: billingConfigurationService.createBillingConfiguration,
    deleteBillingConfiguration: billingConfigurationService.deleteBillingConfiguration,
    updateBillingWebhookEndpoint: billingConfigurationService.updateBillingWebhookEndpoint,
    createCheckoutSession: checkoutService.createCheckoutSession,
    getCheckoutSession: checkoutService.getCheckoutSession,
    getSubscriptionBilling: subscriptionBillingService.getSubscriptionBilling,
    getUsage: usageService.getUsage,
    listBillingInvoices: billingAccountService.listBillingInvoices,
    createAccountBillingPortalSession: billingAccountService.createAccountBillingPortalSession,
    getRevenueReport: revenueService.getRevenueReport,
    listRevenueInvoices: revenueService.listRevenueInvoices,
    listMeters: meterService.listMeters,
    createMeter: meterService.createMeter,
    getMeter: meterService.getMeter,
    updateMeter: meterService.updateMeter,
    deleteMeter: meterService.deleteMeter,
    getPlanPricing: planPricingService.getPlanPricing,
};
