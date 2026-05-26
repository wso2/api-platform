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
 * Some are protected with csrfProtection middleware 
 */
const billingController = require('../../controllers/billingController');
const { ensureBillingAuth, verifyRequestOrigin } = require('../../middlewares/billingAuth');
const { compose } = require('./_compose');

module.exports = {
    // Read-only
    getUsageData: compose(ensureBillingAuth, billingController.getUsageData),
    getPaymentMethods: compose(ensureBillingAuth, billingController.getPaymentMethods),
    getBillingInfo: compose(ensureBillingAuth, billingController.getBillingInfo),
    getActiveSubscriptions: compose(ensureBillingAuth, billingController.getActiveSubscriptions),
    getSubscriptionBillingStatus: compose(ensureBillingAuth, billingController.getSubscriptionBillingStatus),

    // Billing engine keys (admin config; OAuth scope handles auth, origin verification on writes)
    addBillingEngineKeys: compose(verifyRequestOrigin, billingController.addBillingEngineKeys),
    updateBillingEngineKeys: compose(verifyRequestOrigin, billingController.updateBillingEngineKeys),
    deleteBillingEngineKeys: compose(verifyRequestOrigin, billingController.deleteBillingEngineKeys),
    getBillingEngineKeys: billingController.getBillingEngineKeys,

    // Mutating Stripe / billing portal flows
    createCheckoutSessionForSubscription: compose(verifyRequestOrigin, ensureBillingAuth, billingController.createCheckoutSessionForSubscription),
    registerStripeCheckoutSession: compose(verifyRequestOrigin, ensureBillingAuth, billingController.registerStripeCheckoutSession),
    cancelSubscription: compose(verifyRequestOrigin, ensureBillingAuth, billingController.cancelSubscription),
    createBillingPortalByOrg: compose(verifyRequestOrigin, ensureBillingAuth, billingController.createBillingPortalByOrg),
    createBillingPortal: compose(verifyRequestOrigin, ensureBillingAuth, billingController.createBillingPortal),
};
