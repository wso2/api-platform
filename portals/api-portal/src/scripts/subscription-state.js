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

// A consumer may hold several subscriptions to one API, on the same or different plans, so
// subscriptions are identified by subscriptionId; plan name only groups them for the plan cards.
(function (root) {
    function planOf(sub) {
        return sub.policyName || sub.subscriptionPlanName;
    }

    function findSubscription(subs, subscriptionId) {
        if (!Array.isArray(subs)) return null;
        return subs.find(function (s) { return s.subscriptionId === subscriptionId; }) || null;
    }

    function setSubscriptionStatus(subs, subscriptionId, status) {
        var sub = findSubscription(subs, subscriptionId);
        if (sub) sub.status = status;
        return sub;
    }

    // A plan card is "suspended" only when every subscription held on that plan is inactive.
    function planCardState(planHandle, subs) {
        var onPlan = (Array.isArray(subs) ? subs : []).filter(function (s) { return planOf(s) === planHandle; });
        var suspended = onPlan.length > 0 && onPlan.every(function (s) { return s.status === 'INACTIVE'; });
        return { count: onPlan.length, subscribed: onPlan.length > 0, suspended: suspended };
    }

    var api = { findSubscription: findSubscription, setSubscriptionStatus: setSubscriptionStatus, planCardState: planCardState };
    if (typeof module !== 'undefined' && module.exports) {
        module.exports = api;
    } else {
        root.subscriptionState = api;
    }
}(typeof window !== 'undefined' ? window : this));
