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
 * The Invoices page: the invoices of the chosen period (GET /billing/invoices), a page at a time
 * ("Load more" reads the next page with the previous page's cursor), and "Manage payment methods",
 * which opens the payment provider's billing page in this tab for a subscriber who has bought a
 * paid plan.
 * Everything from the responses is written with textContent; links are https only.
 */
(function billingInvoicesPage() {
    const periodSelect = document.getElementById('billingInvoicePeriod');
    const more = document.getElementById('billingInvoicesMore');
    const manage = document.getElementById('billingManage');
    // The manage button is left out for a user who may not change payment details.
    if (!periodSelect || !more) return;
    let cursor = null;
    let loading = 0;

    function el(tag, className, text) {
        const node = document.createElement(tag);
        if (className) node.className = className;
        if (text !== undefined && text !== null) node.textContent = text;
        return node;
    }

    function show(id, visible) {
        const node = document.getElementById(id);
        if (node) node.hidden = !visible;
    }

    function api(path, options) {
        return fetch(apiPortalApi.root(path), Object.assign({
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
        }, options || {}));
    }

    /**
     * Opens the payment provider's billing page in this tab. Its Return link brings the subscriber
     * back here.
     */
    async function openBillingPage() {
        manage.disabled = true;
        show('billingManageError', false);
        try {
            const viewMatch = /\/views\/([^/]+)\//.exec(window.location.pathname);
            const resp = await api('/billing/billing-portal-sessions', {
                method: 'POST',
                body: JSON.stringify(viewMatch ? { viewName: decodeURIComponent(viewMatch[1]) } : {}),
            });
            const body = await resp.json().catch(() => ({}));
            if (!resp.ok || typeof body.url !== 'string' || !body.url.startsWith('https://')) {
                throw new Error(body.message || 'The billing page could not be opened. Try again later.');
            }
            window.location.assign(body.url);
        } catch (err) {
            const error = document.getElementById('billingManageError');
            error.textContent = err.message;
            error.hidden = false;
            manage.disabled = false;
        }
    }

    function formatDate(iso) {
        return iso ? new Date(iso).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric', timeZone: 'UTC' }) : '–';
    }

    // An invoice's status as a pill: paid green, unpaid red, draft and void grey.
    const STATUS_TONES = { paid: 'active', open: 'payment-pending', uncollectible: 'payment-pending' };

    /** An invoice action (View, PDF) as a small button opening the provider's page; https only. */
    function action(href, text, icon, label) {
        if (typeof href !== 'string' || !href.startsWith('https://')) return null;
        const a = el('a', 'billing-action');
        a.href = href;
        a.target = '_blank';
        a.rel = 'noopener noreferrer';
        a.setAttribute('aria-label', label);
        const i = el('i', 'bi ' + icon);
        i.setAttribute('aria-hidden', 'true');
        a.append(i, el('span', null, text));
        return a;
    }

    function statusPill(inv) {
        const pill = el('span', 'sub-status-pill sub-status-pill--' + (STATUS_TONES[inv.status] || 'inactive'));
        pill.append(el('span', 'sub-status-dot'), el('span', null, inv.statusText));
        return pill;
    }

    function row(inv) {
        const name = inv.number ? 'invoice ' + inv.number : 'the invoice';
        const box = el('div', 'billing-links');
        box.append(...[
            action(inv.url, 'View', 'bi-box-arrow-up-right', 'View ' + name),
            action(inv.pdfUrl, 'PDF', 'bi-download', 'Download ' + name + ' as PDF'),
        ].filter(Boolean));
        const actions = el('td');
        actions.append(box);
        const status = el('td');
        status.append(statusPill(inv));
        const tr = el('tr');
        tr.append(
            el('td', null, inv.number || '–'),
            el('td', null, formatDate(inv.date)),
            el('td', null, inv.subscription || '–'),
            el('td', 'billing-num', inv.amountText),
            status,
            actions);
        return tr;
    }

    /**
     * The first read shows a spinner in place of the invoices; another period dims the invoices
     * shown under a spinner until they are replaced; the next page shows a spinner in Load more.
     */
    function busy(on, fresh) {
        const results = document.getElementById('billingInvoicesResults');
        show('billingInvoicesLoading', on && results.hidden);
        results.classList.toggle('is-loading', on && fresh && !results.hidden);
        more.classList.toggle('is-loading', on && !fresh);
        results.setAttribute('aria-busy', String(on));
    }

    /** Reads the first page for the period (fresh), or the next page after the cursor. */
    async function load(fresh) {
        const request = ++loading;
        const body = document.getElementById('billingInvoices');
        if (fresh) cursor = null;
        busy(true, fresh);
        more.disabled = true;
        try {
            const params = new URLSearchParams({ period: periodSelect.value });
            if (cursor) params.set('after', cursor);
            const resp = await api('/billing/invoices?' + params.toString());
            if (!resp.ok) throw new Error('HTTP ' + resp.status);
            const data = await resp.json();
            if (request !== loading) return; // a newer request (another period) took over
            if (manage) manage.hidden = !data.canManageBilling;
            // Another period replaces the invoices shown; the next page adds to them.
            if (fresh) body.replaceChildren();
            body.append(...data.invoices.map(row));
            cursor = data.nextCursor;
            show('billingInvoicesUnavailable', !data.invoicesAvailable);
            show('billingInvoicesTable', body.children.length > 0);
            show('billingNoInvoices', data.invoicesAvailable && body.children.length === 0);
            more.hidden = !cursor;
        } catch (_) {
            if (request !== loading) return;
            show('billingInvoicesUnavailable', true);
        } finally {
            if (request === loading) {
                busy(false, fresh);
                show('billingInvoicesResults', true);
                more.disabled = false;
            }
        }
    }

    periodSelect.addEventListener('change', () => load(true));
    more.addEventListener('click', () => load(false));
    if (manage) manage.addEventListener('click', openBillingPage);
    load(true);
})();
