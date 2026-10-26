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

/*
 * Adds up the invoices that GET /billing-reports/invoices returns, for Settings › Usage & Revenue:
 * the total invoiced each month and the requests billed. Amounts are added as whole minor units
 * ("28.12" is 2812 cents). Months are calendar months in UTC (YYYY-MM).
 */
(function (root, factory) {
  var api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.revenueTotals = api;
}(typeof self !== 'undefined' ? self : this, function () {
  'use strict';

  /** An amount ("28.12", "-0.50") as { minor: 2812, digits: 2 }. */
  function minorOf(amount) {
    var text = String(amount || '0');
    var negative = text.charAt(0) === '-';
    var parts = text.replace('-', '').split('.');
    var fraction = parts[1] || '';
    var minor = Number(parts[0] + fraction);
    return { minor: negative ? -minor : minor, digits: fraction.length };
  }

  /** The month an invoice was created in (YYYY-MM, UTC), or null. */
  function monthOf(invoice) {
    return invoice.created ? invoice.created.slice(0, 7) : null;
  }

  /** The months from the one `from` is in to the one `to` is in, oldest first (YYYY-MM). */
  function monthsBetween(from, to) {
    var months = [];
    var year = from.getUTCFullYear();
    var month = from.getUTCMonth();
    var last = to.getUTCFullYear() * 12 + to.getUTCMonth();
    while (year * 12 + month <= last) {
      months.push(year + '-' + String(month + 1).padStart(2, '0'));
      month += 1;
      if (month === 12) { month = 0; year += 1; }
    }
    return months;
  }

  /**
   * The currencies of the invoices, the one used by the most subscriptions first. Amounts in
   * different currencies cannot be added up, so the report uses that first currency and names the
   * others.
   */
  function currencies(invoices) {
    var subscriptions = {};
    invoices.forEach(function (invoice) {
      var set = subscriptions[invoice.currency] || (subscriptions[invoice.currency] = {});
      set[invoice.subscriptionId || invoice.number || invoice.created] = true;
    });
    return Object.keys(subscriptions).sort(function (a, b) {
      return Object.keys(subscriptions[b]).length - Object.keys(subscriptions[a]).length || a.localeCompare(b);
    });
  }

  /** Whether the invoice's subscriber (name or email), API or plan contains `text`, ignoring case. */
  function matches(invoice, text) {
    var needle = String(text || '').trim().toLowerCase();
    if (!needle) return true;
    var subscriber = invoice.subscriber || {};
    return [subscriber.name, subscriber.email, invoice.api, invoice.plan].some(function (value) {
      return typeof value === 'string' && value.toLowerCase().indexOf(needle) !== -1;
    });
  }

  /**
   * What the invoices in `currency` add up to:
   *   - months: the invoice totals of every month ({ YYYY-MM: minor });
   *   - over the months in `range` (a set of YYYY-MM):
   *       - invoiced: the invoices' total;
   *       - billableRequests: the requests billed by request-count prices;
   *   - digits: the currency's number of decimals.
   */
  function summarize(invoices, currency, range) {
    var result = { months: {}, invoiced: 0, billableRequests: 0, digits: 2 };
    invoices.forEach(function (invoice) {
      if (invoice.currency !== currency) return;
      var total = minorOf(invoice.total);
      result.digits = total.digits;
      var month = monthOf(invoice);
      result.months[month] = (result.months[month] || 0) + total.minor;
      if (!range[month]) return;
      result.invoiced += total.minor;
      (invoice.items || []).forEach(function (item) {
        if (item.countsRequests && item.quantity) result.billableRequests += item.quantity;
      });
    });
    return result;
  }

  /** A minor amount in the currency, as people read it ("$28.12"). */
  function formatMoney(minor, digits, currency) {
    var value = minor / Math.pow(10, digits);
    try {
      return new Intl.NumberFormat('en-US', { style: 'currency', currency: currency }).format(value);
    } catch (_) {
      return currency + ' ' + value.toFixed(digits);
    }
  }

  return {
    minorOf: minorOf,
    monthOf: monthOf,
    monthsBetween: monthsBetween,
    currencies: currencies,
    matches: matches,
    summarize: summarize,
    formatMoney: formatMoney,
  };
}));
