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
 * Settings › Usage & Revenue.
 *   - When the panel is first opened, GET /billing-reports/revenue gives the paid subscriptions and
 *     their payment issues.
 *   - The period's invoices are then read from GET /billing-reports/invoices a page at a time,
 *     newest first, and the cards and the chart are drawn once all are read (revenue-totals.js
 *     adds them up). The first read shows a spinner in place of the report; a later one dims the
 *     report under a spinner until it is replaced.
 *   - Choosing a longer period reads only the months not read yet; a shorter one reads nothing.
 *   - The current month is shown on its own first. Selecting a month on the chart shows that month;
 *     selecting it again shows the whole period.
 *   - The search keeps only the invoices of a matching subscriber, API or plan.
 * Everything from the responses is written with textContent.
 */
(function () {
  var panel = document.getElementById('cfg-revenue');
  if (!panel) return;

  var goto = panel.querySelector('.cfg-revenue-goto-billing');
  if (goto) {
    goto.addEventListener('click', function (e) {
      e.preventDefault();
      if (window.cfgActivatePanel) window.cfgActivatePanel('cfg-billing');
    });
  }

  var periodSelect = document.getElementById('cfgRevenueMonths');
  if (!periodSelect) return;
  var searchInput = document.getElementById('cfgRevenueSearch');
  var totals = window.revenueTotals;

  var started = false;
  var summary = null;
  // The invoices read so far, newest first, and the key of each (so a month read twice counts once).
  var invoices = [];
  var seen = {};
  // The start of the months read in full; null until a read has finished.
  var readFrom = null;
  // The read in progress or last finished: { done, failed }.
  var reading = null;
  // A new read stops an older one still going.
  var generation = 0;
  // The month shown on its own (YYYY-MM), or null for the whole period.
  var selected = new Date().toISOString().slice(0, 7);
  var searchTimer = null;

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined && text !== null) node.textContent = text;
    return node;
  }
  function show(id, visible) {
    var node = document.getElementById(id);
    if (node) node.hidden = !visible;
  }
  function number(n) { return Number(n || 0).toLocaleString('en-US'); }
  function currentMonth() { return new Date().toISOString().slice(0, 7); }

  function card(label, value, note, noteClass) {
    var node = el('div', 'cfg-revenue-card');
    node.append(el('span', 'cfg-revenue-card-label', label), el('span', 'cfg-revenue-card-value', value));
    if (note) node.append(el('span', 'cfg-revenue-card-note' + (noteClass ? ' ' + noteClass : ''), note));
    return node;
  }

  /** A month as people read it: "October", or "October 2025" outside the current year. */
  function monthName(key) {
    var date = new Date(key + '-01T00:00:00Z');
    var name = date.toLocaleDateString('en-US', { month: 'long', timeZone: 'UTC' });
    return key.slice(0, 4) === currentMonth().slice(0, 4) ? name : name + ' ' + key.slice(0, 4);
  }

  /** The first day of the period's first month, not before the month billing was set up in. */
  function periodStart() {
    var now = new Date();
    var since = new Date(summary.billingSince);
    var first = new Date(Date.UTC(since.getUTCFullYear(), since.getUTCMonth(), 1));
    if (periodSelect.value === 'ALL') return first;
    var start = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - (Number(periodSelect.value) - 1), 1));
    return start > first ? start : first;
  }

  /** What the cards are about: the selected month, or the whole period. */
  function selectionText() {
    if (selected) return 'in ' + monthName(selected) + (selected === currentMonth() ? ' so far' : '');
    if (periodSelect.value === 'ALL') return 'since ' + monthName(periodStart().toISOString().slice(0, 7));
    return 'in the last ' + periodSelect.value + ' months';
  }

  function notice(text) {
    var node = document.getElementById('cfgRevenueNotice');
    if (!text) {
      node.hidden = true;
      return;
    }
    node.replaceChildren(el('i', 'bi bi-exclamation-circle'), el('span', null, text));
    node.firstChild.setAttribute('aria-hidden', 'true');
    node.hidden = false;
  }

  function renderCards(sums, currency, months) {
    var nodes = [];
    if (currency) {
      var previous = selected ? months[months.indexOf(selected) - 1] : null;
      nodes.push(card('Invoiced ' + selectionText(), totals.formatMoney(sums.invoiced, sums.digits, currency),
        previous ? monthName(previous) + ': ' + totals.formatMoney(sums.months[previous] || 0, sums.digits, currency) : null));
    }
    var apis = summary.paidApis === 1 ? 'across 1 API' : 'across ' + summary.paidApis + ' APIs';
    nodes.push(card('Paid subscriptions', number(summary.paidSubscriptions), apis));
    if (currency) {
      nodes.push(card('Billable requests ' + selectionText(), number(sums.billableRequests), 'Request-count meters'));
    }
    var issues = summary.paymentIssues.pastDue + summary.paymentIssues.paymentPending;
    nodes.push(card('Payment issues', number(issues),
      summary.paymentIssues.pastDue + ' past due · ' + summary.paymentIssues.paymentPending + ' payment pending',
      issues > 0 ? 'cfg-revenue-warn' : null));
    var holder = document.getElementById('cfgRevenueCards');
    holder.replaceChildren.apply(holder, nodes);
  }

  function renderChart(sums, currency, months) {
    var holder = document.getElementById('cfgRevenueChart');
    if (!currency) {
      holder.replaceChildren(el('p', 'cfg-revenue-muted', 'No invoices in this period.'));
      return;
    }
    var values = months.map(function (m) { return Math.max(0, sums.months[m] || 0); });
    var max = Math.max.apply(null, values.concat([1]));
    var chart = el('div', 'cfg-revenue-chart' + (selected ? ' has-selection' : ''));
    chart.setAttribute('role', 'group');
    chart.setAttribute('aria-label', 'Invoiced revenue by month');
    var axis = el('div', 'cfg-revenue-axis');
    axis.setAttribute('aria-hidden', 'true');
    var long = months.length > 12;
    months.forEach(function (m, i) {
      var text = monthName(m) + (m === currentMonth() ? ' so far' : '') + ': ' + totals.formatMoney(sums.months[m] || 0, sums.digits, currency);
      var slot = el('button', 'cfg-revenue-slot' + (m === selected ? ' is-selected' : ''));
      slot.type = 'button';
      slot.title = text;
      slot.setAttribute('aria-label', text);
      slot.setAttribute('aria-pressed', String(m === selected));
      var bar = el('span', 'cfg-revenue-bar');
      bar.style.height = ((values[i] / max) * 100).toFixed(1) + '%';
      slot.append(bar);
      slot.addEventListener('click', function () {
        selected = selected === m ? null : m;
        render();
      });
      chart.append(slot);
      var date = new Date(m + '-01T00:00:00Z');
      axis.append(el('span', null, date.toLocaleDateString('en-US', long
        ? { month: 'short', year: '2-digit', timeZone: 'UTC' } : { month: 'short', timeZone: 'UTC' })));
    });
    holder.replaceChildren(chart, axis);
  }

  /** Draws the report from the invoices read. */
  function render() {
    var months = totals.monthsBetween(periodStart(), new Date());
    if (selected && months.indexOf(selected) === -1) selected = null;
    var search = searchInput ? searchInput.value : '';
    var shown = invoices.filter(function (inv) { return totals.matches(inv, search); });
    // The currency is chosen from all invoices read, not only the ones the search keeps, so a search
    // does not change it.
    var ranked = totals.currencies(invoices);
    var currency = ranked[0] || null;
    var range = {};
    (selected ? [selected] : months).forEach(function (m) { range[m] = true; });
    var sums = currency ? totals.summarize(shown, currency, range) : null;

    if (reading && reading.failed) notice('Invoices could not be loaded from Stripe, so these numbers may be incomplete. Try again later.');
    else if (ranked.length > 1) notice('Amounts are in ' + currency + '. Invoices in ' + ranked.slice(1).join(', ') + ' are not included.');
    else notice(null);

    renderCards(sums, currency, months);
    renderChart(sums, currency, months);
  }

  /**
   * The first read shows a spinner in place of the report; a later one dims the report under a
   * spinner until it is replaced.
   */
  function busy(on) {
    var body = document.getElementById('cfgRevenueBody');
    show('cfgRevenueLoading', on && body.hidden);
    body.classList.toggle('is-loading', on && !body.hidden);
    body.setAttribute('aria-busy', String(on));
  }

  function invoiceKey(inv) {
    return inv.number || [inv.created, inv.total, inv.subscriptionId].join('|');
  }

  /** Reads the invoices created from `from` up to `to`, a page at a time, then draws the report. */
  async function read(from, to) {
    var mine = ++generation;
    reading = { done: false, failed: false };
    busy(true);
    var after = null;
    try {
      do {
        var query = '?from=' + encodeURIComponent(from.toISOString()) + '&to=' + encodeURIComponent(to.toISOString())
          + (after ? '&after=' + encodeURIComponent(after) : '');
        var res = await fetch(window.apiPortalApi.root('/billing-reports/invoices' + query), {
          headers: { 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
        });
        if (mine !== generation) return;
        if (!res.ok) throw new Error('HTTP ' + res.status);
        var page = await res.json();
        if (mine !== generation) return;
        if (!page.invoicesAvailable) throw new Error('Stripe could not be read');
        page.invoices.forEach(function (inv) {
          var key = invoiceKey(inv);
          if (seen[key]) return;
          seen[key] = true;
          invoices.push(inv);
        });
        after = page.nextCursor;
      } while (after);
      readFrom = from;
    } catch (_) {
      if (mine !== generation) return;
      reading.failed = true;
    }
    reading.done = true;
    busy(false);
    show('cfgRevenueBody', true);
    render();
  }

  /** Shows the selected period: read from scratch, only its months not read yet, or nothing. */
  function loadPeriod() {
    var from = periodStart();
    var complete = reading && reading.done && !reading.failed && readFrom;
    if (complete && from >= readFrom) {
      render();
      return;
    }
    if (complete) {
      read(from, readFrom);
      return;
    }
    invoices = [];
    seen = {};
    readFrom = null;
    read(from, new Date());
  }

  async function start() {
    started = true;
    try {
      var res = await fetch(window.apiPortalApi.root('/billing-reports/revenue'), {
        headers: { 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
      });
      if (!res.ok) throw new Error('HTTP ' + res.status);
      summary = await res.json();
    } catch (_) {
      show('cfgRevenueLoading', false);
      notice('The report could not be loaded. Try again later.');
      started = false;
      return;
    }
    if (!summary.configured) {
      show('cfgRevenueLoading', false);
      notice('Billing is not set up.');
      return;
    }
    loadPeriod();
  }

  document.addEventListener('cfg:panel-shown', function (e) {
    if (e.detail && e.detail.id === 'cfg-revenue' && !started) start();
  });
  if (panel.style.display === 'flex' && !started) start();
  periodSelect.addEventListener('change', function () {
    if (summary && summary.configured) loadPeriod();
  });
  if (searchInput) {
    searchInput.addEventListener('input', function () {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(function () {
        if (reading && reading.done) render();
      }, 200);
    });
  }
}());
