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

const Handlebars = require('handlebars');
const constants = require('../utils/constants');

const helpers = {
    // Array helpers
    filterByStatus: (array, status) => {
        if (!Array.isArray(array)) return [];
        if (!status || status === 'ALL') return array;
        const s = status.toLowerCase();
        return array.filter(item => item.status && item.status.toLowerCase() === s);
    },
    isEmpty: (arr) => !arr || arr.length === 0,
    filter: (array, property, value, include) => {
        if (!Array.isArray(array)) return [];
        const shouldInclude = typeof include === 'boolean' ? include : true;
        return array.filter(item => item && (shouldInclude ? item[property] === value : item[property] !== value));
    },
    contains: (array, value) => array && array.includes(value),
    getSubIDs: (subAPIs) => JSON.stringify(subAPIs.map(api => api.subID)),
    every: function (array, key, options) {
        if (!Array.isArray(array)) return options.inverse(this);
        return array.every(item => item[key]);
    },
    some: function (array, key, options) {
        if (!Array.isArray(array)) return options.inverse(this);
        return array.some(item => item[key]);
    },

    // JSON helpers
    json: (context) => JSON.stringify(context ?? null),
    jsonBeautify: (context) => typeof context === 'string' ? context : JSON.stringify(context ?? {}, null, 2),
    jsonSafeSubscriptions: function (context) {
        try {
            if (!Array.isArray(context)) return JSON.stringify([]);
            const maskToken = (t) => (!t || String(t).length <= 4) ? '****' : '****' + String(t).slice(-4);
            const safe = context.map((s) => ({
                subscriptionId: s.subscriptionId,
                subscriptionPlanName: s.subscriptionPlanName,
                status: s.status,
                customerName: s.customerName || s.customer || null,
                maskedToken: maskToken(s.subscriptionToken),
            }));
            return JSON.stringify(safe);
        } catch (e) {
            return JSON.stringify([]);
        }
    },

    // String helpers
    lowercase: (str) => typeof str === 'string' ? str.toLowerCase() : str,
    firstTwoLetters: (text) => text ? text.substring(0, 2).toUpperCase() : '',
    beforeSeparator: (value, separator) => typeof value === 'string' && typeof separator === 'string' ? value.split(separator)[0] : value,
    stripMdExtension: (value) => typeof value === 'string' && value.endsWith('.md') ? value.slice(0, -3) : value,
    startsWith: function (str, includeStr, options) {
        return str && str.startsWith(includeStr) ? options.fn(this) : options.inverse(this);
    },

    // Comparison / logic helpers
    eq: (a, b) => a === b || (a != null && b != null && (a === b.toString() || a.toString() === b)),
    compare: function (a, operator, b, options) {
        if (arguments.length < 4) throw new Error('Handlebars Helper "compare" needs 3 parameters');
        const ops = { '===': a === b, '!==': a !== b, '<': a < b, '>': a > b, '<=': a <= b, '>=': a >= b };
        if (!(operator in ops)) throw new Error(`Handlebars Helper "compare" doesn't know the operator ${operator}`);
        return ops[operator] ? options.fn(this) : options.inverse(this);
    },
    conditionalIf: (condition, value1, value2) => condition ? value1 : value2,
    and: function () {
        const args = Array.prototype.slice.call(arguments);
        const options = args.pop();
        return args.every(Boolean) ? options.fn(this) : options.inverse(this);
    },
    or: function (...args) {
        const vals = args.slice(0, -1);
        return vals.find(v => v) || vals[vals.length - 1];
    },
    in: function (value, options) {
        const rawValues = Array.isArray(options.hash.values) ? options.hash.values : options.hash.values.split(',');
        const match = rawValues.map(v => v.trim()).some(valid => value?.trim()?.includes(valid));
        return match ? options.fn(this) : options.inverse(this);
    },
    let: function (name, value, options) {
        const data = Handlebars.createFrame(options.data);
        data[name] = value;
        return options.fn({ ...options.hash, ...data });
    },

    // Object helpers
    getValue: (obj, key) => obj[key],

    // Display / formatting helpers
    isMiddle: (index, length) => index === Math.floor(length / 2),
    isFederatedAPI: (gatewayVendor) => typeof gatewayVendor === 'string' && constants.FEDERATED_GATEWAY_VENDORS.includes(gatewayVendor),
    maskToken: (token) => (!token || token.length <= 4) ? '****' : '****' + token.slice(-4),
    isCurrentPlan: (policyName, subs) => Array.isArray(subs) && !!policyName && subs.some(s => s.subscriptionPlanName === policyName),
    currentYear: () => new Date().getFullYear(),
    formatExpiresAt:  function (value) {
        // Accepts ISO-8601 strings, Unix seconds, or Unix milliseconds
        if (value == null || value === '') return '';
        const s = String(value).trim();
        if (/^\d+$/.test(s)) {
            const n = parseInt(s, 10);
            if (Number.isNaN(n)) return s;
            const d = String(n).length <= 10 ? new Date(n * 1000) : new Date(n);
            return Number.isNaN(d.getTime()) ? s : d.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
        }
        const d = new Date(s);
        return Number.isNaN(d.getTime()) ? s : d.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
    },
    formatPrice: (price) => price ? parseFloat(price).toString() : '0',
    formatBillingPeriod: (period) => ({ day: 'daily', week: 'weekly', month: 'monthly', year: 'yearly' })[String(period || '').toLowerCase()] || (String(period || '').toLowerCase() + 'ly'),
    formatTierRange: (startUnit, endUnit) => {
        const start = startUnit != null ? Number(startUnit).toLocaleString() : '0';
        return (endUnit == null || endUnit === '' || endUnit === Infinity) ? start + ' +' : start + ' \u2013 ' + Number(endUnit).toLocaleString();
    },
};

function registerHelpers() {
    Object.entries(helpers).forEach(([name, fn]) => Handlebars.registerHelper(name, fn));
}

module.exports = { registerHelpers };
