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
     * Paid plans on an API or MCP server page: the subscribe dialog, Stripe's checkout form, and the
     * result (the dialogs are in the plan-checkout partial).
     *
     * Subscribe opens a dialog with the plan's prices (read from its card); Continue starts a
     * checkout and shows Stripe's checkout form in a dialog (Stripe Embedded Checkout). Once it is
     * paid, the portal is asked for the checkout's state until the subscription exists, then its
     * token is shown as for a free plan. A payment method that leaves the page returns the browser
     * here with ?checkout=<id>&checkoutResult=success, which is handled the same way.
     */
    (function () {
      var POLL_MS = 2000;
      var POLL_LIMIT = 15;

      function el(id) { return document.getElementById(id); }

      function text(node) { return node ? node.textContent.replace(/\s+/g, ' ').trim() : ''; }

      function row(label, value) {
        var tr = document.createElement('tr');
        var th = document.createElement('td');
        var td = document.createElement('td');
        th.textContent = label;
        td.textContent = value;
        td.style.textAlign = 'right';
        tr.appendChild(th);
        tr.appendChild(td);
        return tr;
      }

      /* The view this page belongs to: /<base>/<org>/views/<view>/api/<handle> or .../mcp/<handle>. */
      function viewName() {
        var match = /\/views\/([^/]+)\/(?:api|mcp)\//.exec(window.location.pathname);
        return match ? decodeURIComponent(match[1]) : undefined;
      }

      function fillDialog(btn) {
        var section = el('subscriptionPlans');
        var card = btn.closest('.aov-plan-card');
        var name = btn.dataset.displayName || btn.dataset.planId;
        el('paidSubscribeTitle').textContent = 'Subscribe to ' + name + ' plan';
        el('paidSubscribeApi').textContent = [section.dataset.apiName, section.dataset.apiVersion].filter(Boolean).join(' · ');

        var pricing = el('paidSubscribePricing');
        pricing.innerHTML = '';
        var yearly = card.dataset.billingPeriod === 'YEAR';
        var flatFee = card.dataset.flatFee !== undefined;
        var usage = Number(card.dataset.usagePrices) > 0;
        var amount = text(card.querySelector('.aov-plan-price-amount'));
        var unit = text(card.querySelector('.aov-plan-price-unit'));
        // The headline is the flat rate or the one usage price; "From …" and "Pay per use" only
        // sum up the prices listed below it.
        var summary = card.querySelector('.aov-plan-price-prefix') !== null;
        if (flatFee) pricing.appendChild(row(yearly ? 'Yearly fee' : 'Monthly fee', amount));
        else if (unit && !summary) pricing.appendChild(row('Price', amount + ' ' + unit));
        // The prices under the card's Pricing details, one row per price and per tier.
        card.querySelectorAll('.aov-plan-price-item').forEach(function (item) {
          // A price is named only beside others; alone, its model heads its tiers, or it is the usage
          // charged beside a flat fee.
          if (item.dataset.name) pricing.appendChild(row(item.dataset.name, item.dataset.text));
          else if (item.querySelector('.aov-plan-tier')) pricing.appendChild(row(item.dataset.text, ''));
          else pricing.appendChild(row('Usage', item.dataset.text));
          item.querySelectorAll('.aov-plan-tier').forEach(function (tier) {
            pricing.appendChild(row(tier.dataset.label, tier.dataset.amounts));
          });
        });
        var limit = text(card.querySelector('.aov-plan-limit'));
        if (limit) pricing.appendChild(row('Rate limit', limit));

        // A flat rate is charged up front; usage is invoiced afterwards. The amounts carry their currency.
        el('paidSubscribeTerms').textContent = [
          flatFee ? 'The ' + (yearly ? 'yearly' : 'monthly') + ' fee is charged when the subscription starts.' : '',
          usage ? 'Usage is invoiced at the end of each billing period.' : '',
        ].filter(Boolean).join(' ');
      }

      var activeBtn = null;

      function openDialog(btn) {
        activeBtn = btn;
        fillDialog(btn);
        el('paidSubscribeModal').style.display = 'flex';
        el('paidSubscribeContinue').focus();
      }

      function closeDialog() {
        el('paidSubscribeModal').style.display = 'none';
        activeBtn = null;
      }

      /* ── payment: Stripe's checkout form in a dialog (Stripe Embedded Checkout) ── */
      // Stripe.js must come from js.stripe.com; it is loaded the first time a payment starts.
      var STRIPE_JS = 'https://js.stripe.com/v3/';
      var stripeLoading = null;
      var embedded = null; // Stripe allows one mounted checkout at a time

      function loadStripe() {
        if (window.Stripe) return Promise.resolve();
        if (!stripeLoading) {
          stripeLoading = new Promise(function (resolve, reject) {
            var script = document.createElement('script');
            script.src = STRIPE_JS;
            script.onload = resolve;
            script.onerror = function () { stripeLoading = null; reject(new Error('Stripe.js could not be loaded')); };
            document.head.appendChild(script);
          });
        }
        return stripeLoading;
      }

      function closePayment() {
        if (embedded) {
          embedded.destroy();
          embedded = null;
        }
        el('checkoutPayModal').style.display = 'none';
      }

      /* Mounts the started checkout; once paid, the subscription is activated as on a return from Stripe. */
      async function openPayment(started, title, subtitle) {
        el('checkoutPayTitle').textContent = title;
        el('checkoutPayApi').textContent = subtitle;
        el('checkoutPayLoading').hidden = false;
        el('checkoutPayModal').style.display = 'flex';
        await loadStripe();
        if (embedded) embedded.destroy();
        embedded = await window.Stripe(started.publishableKey).initEmbeddedCheckout({
          fetchClientSecret: function () { return Promise.resolve(started.clientSecret); },
          onComplete: function () {
            closePayment();
            handleReturn(started.id, 'success');
          },
        });
        el('checkoutPayLoading').hidden = true;
        embedded.mount('#checkoutPayMount');
      }

      async function startCheckout() {
        if (!activeBtn) return;
        var continueBtn = el('paidSubscribeContinue');
        continueBtn.disabled = true;
        continueBtn.textContent = 'Opening payment…';
        try {
          var resp = await fetch(apiPortalApi.root('/checkout-sessions'), {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
            body: JSON.stringify({ artifactId: activeBtn.dataset.apiId, subscriptionPlanId: activeBtn.dataset.planId, viewName: viewName() }),
          });
          var data = await resp.json().catch(function () { return {}; });
          if (resp.ok && data.clientSecret && data.publishableKey) {
            var title = el('paidSubscribeTitle').textContent;
            var subtitle = el('paidSubscribeApi').textContent;
            closeDialog();
            await openPayment(data, title, subtitle);
            return;
          }
          closeDialog();
          showAlert('Checkout could not be started: ' + (data.message || resp.statusText), 'error');
        } catch (e) {
          closeDialog();
          closePayment();
          showAlert('Checkout could not be started. Check your connection and try again.', 'error');
        } finally {
          continueBtn.disabled = false;
          continueBtn.textContent = 'Continue to payment';
        }
      }

      /* ── return from Stripe ── */
      function showStatus(title, message, hint, retryPlanId) {
        el('checkoutStatusTitle').textContent = title;
        el('checkoutStatusText').textContent = message;
        el('checkoutStatusHint').textContent = hint || '';
        var retry = el('checkoutStatusRetry');
        retry.style.display = retryPlanId ? '' : 'none';
        retry.dataset.planId = retryPlanId || '';
        el('checkoutStatusModal').style.display = 'flex';
      }

      function closeStatus() {
        el('checkoutStatusModal').style.display = 'none';
      }

      /* Drops ?checkout=… so a reload does not repeat the return handling. */
      function cleanUrl() {
        var url = new URL(window.location.href);
        url.searchParams.delete('checkout');
        url.searchParams.delete('checkoutResult');
        window.history.replaceState(null, '', url.pathname + url.search + url.hash);
      }

      async function readCheckout(id) {
        var resp = await fetch(apiPortalApi.root('/checkout-sessions/' + encodeURIComponent(id)), {
          headers: { 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
        });
        if (!resp.ok) throw new Error(resp.statusText);
        return resp.json();
      }

      async function showCreated(checkout, planName) {
        closeStatus();
        var token = '';
        try {
          var resp = await fetch(apiPortalApi.root('/subscriptions/' + encodeURIComponent(checkout.subscriptionId)), {
            headers: { 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
          });
          if (resp.ok) token = (await resp.json()).subscriptionToken || '';
        } catch (e) { /* the token can be revealed later from the subscription */ }
        if (window.showSubscriptionTokenModal) await window.showSubscriptionTokenModal(token, planName);
        window.location.reload();
      }

      async function handleReturn(id, result) {
        cleanUrl();
        var planName = '';
        if (result !== 'success') {
          var notDone;
          try { notDone = await readCheckout(id); } catch (e) { notDone = null; }
          if (notDone && notDone.status === 'COMPLETED') { showCreated(notDone, ''); return; }
          showStatus('Payment not completed', 'You left the payment page before completing it. No subscription was created and you have not been charged.',
            '', notDone && notDone.subscriptionPlanId);
          return;
        }
        showStatus('Activating subscription', 'Waiting for Stripe to confirm the payment. This usually takes a few seconds.',
          'You can close this window. The subscription appears in Subscriptions once the payment is confirmed.');
        for (var attempt = 0; attempt < POLL_LIMIT; attempt++) {
          var checkout;
          try { checkout = await readCheckout(id); } catch (e) { checkout = null; }
          if (checkout) {
            var btn = document.querySelector('.paid-subscribe-btn[data-plan-id="' + CSS.escape(checkout.subscriptionPlanId || '') + '"]');
            planName = btn ? (btn.dataset.displayName || checkout.subscriptionPlanId) : (checkout.subscriptionPlanId || '');
            if (checkout.status === 'COMPLETED' && checkout.subscriptionId) { showCreated(checkout, planName); return; }
            if (checkout.status === 'EXPIRED') {
              showStatus('Payment failed', 'Stripe couldn\'t process the payment for the ' + planName + ' plan. Your subscription is not active.',
                'Try again with a different payment method.', checkout.subscriptionPlanId);
              return;
            }
          }
          await new Promise(function (resolve) { setTimeout(resolve, POLL_MS); });
        }
        // Still open: Stripe has not confirmed yet. The subscription appears once it does.
      }

      function init() {
        if (!el('paidSubscribeModal')) return;
        document.querySelectorAll('.paid-subscribe-btn').forEach(function (btn) {
          btn.addEventListener('click', function () { openDialog(btn); });
        });
        document.querySelectorAll('.paid-modal-close').forEach(function (b) { b.addEventListener('click', closeDialog); });
        el('paidSubscribeModal').addEventListener('click', function (e) { if (e.target === this) closeDialog(); });
        el('paidSubscribeContinue').addEventListener('click', startCheckout);
        document.querySelectorAll('.checkout-status-close').forEach(function (b) { b.addEventListener('click', closeStatus); });
        // Closing the payment leaves the checkout open: choosing the plan again picks it up.
        document.querySelectorAll('.checkout-pay-close').forEach(function (b) { b.addEventListener('click', closePayment); });
        el('checkoutStatusRetry').addEventListener('click', function () {
          var btn = document.querySelector('.paid-subscribe-btn[data-plan-id="' + CSS.escape(this.dataset.planId) + '"]');
          closeStatus();
          if (btn) openDialog(btn);
        });

        var params = new URLSearchParams(window.location.search);
        var id = params.get('checkout');
        if (id && /^[0-9a-f-]{36}$/.test(id)) handleReturn(id, params.get('checkoutResult'));
      }

      if (document.readyState === 'complete') {
        init();
      } else {
        document.addEventListener('DOMContentLoaded', init);
      }
    }());
