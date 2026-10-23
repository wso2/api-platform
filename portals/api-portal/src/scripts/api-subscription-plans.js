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
 

    (function () {
      var _plansRoot = document.getElementById('subscriptionPlans');
      window.__subscriptionOrgId = window.__subscriptionOrgId || (_plansRoot && _plansRoot.dataset.orgId) || "";

      var raw = document.getElementById('existing-subs-data').textContent || '[]';
      try {
        var parsed = JSON.parse(raw);
        window.existingSubscriptions = parsed.map(function (s) {
          return { subscriptionId: s.subscriptionId, subscriptionPlanName: s.subscriptionPlanName, policyName: s.policyName, status: s.status };
        });
        window.__tokenMeta = window.__tokenMeta || {};
        parsed.forEach(function (s) {
          window.__tokenMeta[s.subscriptionId] = { maskedToken: s.maskedToken, subscriptionPlanName: s.subscriptionPlanName, policyName: s.policyName, status: s.status };
        });
      } catch (e) {
        window.existingSubscriptions = [];
      }

      /* ── Token modal close ── */
      function closeTokenModal() {
        var modal = document.getElementById('apiTokenModal');
        if (!modal) return;
        modal.style.display = 'none';
        var resolve = modal._resolvePromise;
        if (resolve) { modal._resolvePromise = null; resolve(); }
        if (window.__subscriptionChanged) {
          window.__subscriptionChanged = false;
          window.location.reload();
        }
      }

      /* ── Fetch full token on demand ── */
      async function fetchTokenIfNeeded(subscriptionId) {
        var meta = (window.__tokenMeta || {})[subscriptionId];
        if (meta && meta._fullToken) return meta._fullToken;
        try {
          var resp = await fetch(apiPortalApi.root('/subscriptions/' + encodeURIComponent(subscriptionId)), {
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
          });
          if (!resp.ok) return null;
          var data = await resp.json();
          var token = data.subscriptionToken || null;
          if (token && meta) meta._fullToken = token;
          return token;
        } catch (e) { return null; }
      }

      /* ── Manage modal state ── */
      var _sub = null;
      var _revealed = false;
      var _mCopyTimer = null;
      var _regenerateInFlight = false;

      function openManage(policyName) {
        var subs = window.existingSubscriptions || [];
        var sub = null;
        for (var i = 0; i < subs.length; i++) {
          if ((subs[i].policyName || subs[i].subscriptionPlanName) === policyName) { sub = subs[i]; break; }
        }
        if (!sub) return;
        _sub = sub;
        _revealed = false;

        document.getElementById('apiManagePlanName').textContent = policyName;
        updateManageStatus(sub.status);
        // A paid subscription is stopped by unsubscribing, not suspended (as on the Subscriptions page).
        var card = document.querySelector('#subscriptionPlans .aov-plan-card[data-policy-name="' + policyName + '"]');
        document.getElementById('apiManageSuspendBtn').hidden = !!(card && card.hasAttribute('data-paid'));
        var meta = (window.__tokenMeta || {})[sub.subscriptionId];
        document.getElementById('apiManageTokenVal').textContent = meta && meta.maskedToken ? meta.maskedToken : '•'.repeat(28);
        var revealIcon = document.getElementById('apiManageRevealIcon');
        if (revealIcon) revealIcon.className = 'bi bi-eye';
        resetManageCopyBtn();
        document.getElementById('apiManageModal').style.display = 'flex';
      }

      function closeManage() {
        document.getElementById('apiManageModal').style.display = 'none';
        _sub = null;
        _revealed = false;
      }

      function updateManageStatus(status) {
        var isSusp = status === 'INACTIVE';
        var badge = document.getElementById('apiManageStatusBadge');
        badge.textContent = isSusp ? 'Suspended' : 'Active';
        badge.className = 'mc-manage-status-badge ' + (isSusp ? 'mc-manage-status-badge--suspended' : 'mc-manage-status-badge--active');

        var suspendBtn = document.getElementById('apiManageSuspendBtn');
        suspendBtn.querySelector('i').className = 'bi bi-' + (isSusp ? 'play-circle' : 'pause-circle');
        suspendBtn.querySelector('.api-suspend-label').textContent = isSusp ? 'Resume' : 'Suspend';
        suspendBtn.className = 'dp-btn mc-manage-action-btn ' + (isSusp ? 'mc-manage-action-btn--resume' : 'mc-manage-action-btn--suspend');
      }

      function resetManageCopyBtn() {
        var btn = document.getElementById('apiManageCopyBtn');
        if (btn) btn.classList.remove('copy-btn--copied');
      }

      async function revealToken() {
        if (!_sub) return;
        if (_revealed) {
          _revealed = false;
          var meta = (window.__tokenMeta || {})[_sub.subscriptionId];
          document.getElementById('apiManageTokenVal').textContent = meta && meta.maskedToken ? meta.maskedToken : '•'.repeat(28);
          var ri = document.getElementById('apiManageRevealIcon');
          if (ri) ri.className = 'bi bi-eye';
          return;
        }
        var token = await fetchTokenIfNeeded(_sub.subscriptionId);
        if (!token) return;
        _revealed = true;
        document.getElementById('apiManageTokenVal').textContent = token;
        var ri2 = document.getElementById('apiManageRevealIcon');
        if (ri2) ri2.className = 'bi bi-eye-slash';
      }

      async function copyManageToken() {
        if (!_sub) return;
        var token = await fetchTokenIfNeeded(_sub.subscriptionId);
        if (!token) return;
        try { await navigator.clipboard.writeText(token); } catch (e) {
          var ta = document.createElement('textarea');
          ta.value = token; ta.style.cssText = 'position:fixed;opacity:0';
          document.body.appendChild(ta); ta.select(); document.execCommand('copy'); document.body.removeChild(ta);
        }
        var btn = document.getElementById('apiManageCopyBtn');
        if (btn) btn.classList.add('copy-btn--copied');
        if (_mCopyTimer) clearTimeout(_mCopyTimer);
        _mCopyTimer = setTimeout(resetManageCopyBtn, 1600);
      }

      /* ── Regenerate token ── */
      var _regenTrigger = null;
      function askRegenerate() {
        if (!_sub) return;
        var dialog = document.getElementById('apiRegenerateDialog');
        if (!dialog) return;
        _regenTrigger = document.activeElement; // restore focus here on close
        dialog.style.display = 'flex';
        document.getElementById('apiRegenerateCancelBtn').focus();
      }
      function closeRegenerate() {
        var dialog = document.getElementById('apiRegenerateDialog');
        if (dialog) dialog.style.display = 'none';
        if (_regenTrigger && typeof _regenTrigger.focus === 'function') _regenTrigger.focus();
        _regenTrigger = null;
      }
      async function confirmRegenerate() {
        if (!_sub || _regenerateInFlight) return;
        var subId = _sub.subscriptionId;
        closeRegenerate();
        _regenerateInFlight = true;
        try {
          var resp = await fetch(apiPortalApi.root('/subscriptions/' + encodeURIComponent(subId) + '/regenerate-token'), {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
          });
          if (resp.ok) {
            var data = await resp.json();
            var newToken = data.subscriptionToken;
            var meta = (window.__tokenMeta || {})[subId];
            if (meta) { meta._fullToken = newToken || null; }
            /* Show the new token revealed so the user can copy it before closing. */
            _revealed = true;
            document.getElementById('apiManageTokenVal').textContent = newToken || '•'.repeat(28);
            var ri = document.getElementById('apiManageRevealIcon');
            if (ri) ri.className = 'bi bi-eye-slash';
            resetManageCopyBtn();
            showAlert('Token regenerated. Copy your new token before closing this window.', 'success');
          } else {
            var err = await resp.json().catch(function () { return {}; });
            showAlert('Failed to regenerate token: ' + (err.description || 'Unknown error'), 'error');
          }
        } catch (e) {
          showAlert('Error: ' + e.message, 'error');
        } finally {
          _regenerateInFlight = false;
        }
      }

      async function toggleSuspend() {
        if (!_sub) return;
        var orgId = window.__subscriptionOrgId;
        var newStatus = _sub.status === 'ACTIVE' ? 'INACTIVE' : 'ACTIVE';
        try {
          var resp = await fetch(apiPortalApi.root('/subscriptions/' + encodeURIComponent(_sub.subscriptionId)), {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
            body: JSON.stringify({ status: newStatus }),
          });
          if (resp.ok) {
            _sub.status = newStatus;
            (window.existingSubscriptions || []).forEach(function (s) {
              if (s.subscriptionId === _sub.subscriptionId) s.status = newStatus;
            });
            updateManageStatus(newStatus);
            updateCardSuspendedState(_sub.policyName || _sub.subscriptionPlanName, newStatus === 'INACTIVE');
          } else {
            var data = await resp.json().catch(function () { return {}; });
            showAlert('Failed to update subscription: ' + (data.description || 'Unknown error'), 'error');
          }
        } catch (e) {
          showAlert('Error: ' + e.message, 'error');
        }
      }

      function updateCardSuspendedState(planName, isSuspended) {
        var card = document.querySelector('#subscriptionPlans .aov-plan-card[data-policy-name="' + planName + '"]');
        if (!card) return;
        var label = card.querySelector('.api-ribbon-label');
        if (isSuspended) {
          card.classList.add('aov-plan-card--suspended');
          if (label) label.textContent = 'SUSPENDED';
        } else {
          card.classList.remove('aov-plan-card--suspended');
          if (label) label.textContent = 'SUBSCRIBED';
        }
      }

      /* ── Unsubscribe / plan-switch dialog ── */
      var _unsubAction = 'unsub';
      var _switchArgs = null;

      function showUnsubDialog(titleHtml, desc, confirmLabel) {
        document.getElementById('apiUnsubTitle').innerHTML = titleHtml;
        document.getElementById('apiUnsubDesc').textContent = desc;
        document.getElementById('apiUnsubConfirmBtn').textContent = confirmLabel;
        document.getElementById('apiUnsubDialog').style.display = 'flex';
      }

      function askUnsub() {
        if (!_sub) return;
        _unsubAction = 'unsub';
        _switchArgs = null;
        showUnsubDialog(
          'Unsubscribe from <span id="apiUnsubPlanName">' + _sub.subscriptionPlanName + '</span> plan?',
          'Your applications will lose access to this API and any tokens will stop working. You can re-subscribe at any time.',
          'Unsubscribe'
        );
      }

      function closeUnsub() {
        document.getElementById('apiUnsubDialog').style.display = 'none';
        if (_unsubAction === 'switch') {
          var pendingBtn = (_switchArgs && _switchArgs.btn) || window.__pendingPlanSwitchBtn;
          if (pendingBtn) pendingBtn.classList.remove('aov-btn-loading');
          window.__pendingPlanSwitchBtn = null;
          _activeSubCard = null;
        }
        _unsubAction = 'unsub';
        _switchArgs = null;
      }

      async function confirmUnsub() {
        if (_unsubAction === 'switch' && _switchArgs) {
          var args = _switchArgs;
          closeUnsub();
          try {
            var resp = await fetch(
              apiPortalApi.root('/subscriptions/' + encodeURIComponent(args.subscriptionId) + '/change-plan'),
              {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
                body: JSON.stringify({ planId: args.policyId }),
              }
            );
            if (resp.ok) {
              window.__subscriptionChanged = true;
              window.location.reload();
            } else {
              var errData = await resp.json().catch(function () { return {}; });
              showAlert('Failed to switch plan: ' + (errData.description || 'Unknown error'), 'error');
            }
          } catch (e) {
            showAlert('Error during plan change: ' + e.message, 'error');
          }
          return;
        }
        if (!_sub) return;
        var orgId = window.__subscriptionOrgId;
        try {
          resp = await fetch(apiPortalApi.root('/subscriptions/' + encodeURIComponent(_sub.subscriptionId)), {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
          });
          if (resp.ok) {
            closeUnsub();
            closeManage();
            window.location.reload();
          } else {
            var data = await resp.json().catch(function () { return {}; });
            showAlert('Failed to delete subscription: ' + (data.description || 'Unknown error'), 'error');
          }
        } catch (e) {
          showAlert('Error: ' + e.message, 'error');
        }
      }

      /* ── Active card tracking (for error display) ── */
      var _activeSubCard = null;

      /* ── Wiring (runs after deferred scripts) ── */
      function init() {
        window.refreshLandingPageSubscriptions = function () {
          window.location.reload();
        };

        window.showSubscriptionTokenModal = function (token, planName) {
          return new Promise(function (resolve) {
            var modal = document.getElementById('apiTokenModal');
            if (!modal) { resolve(); return; }
            modal._resolvePromise = resolve;
            modal._token = token;
            document.getElementById('apiTokenPlanName').textContent = planName || '';
            document.getElementById('apiTokenValue').textContent = token || '';
            var copyBtn = document.getElementById('apiTokenCopyBtn');
            if (copyBtn) copyBtn.classList.remove('copy-btn--copied');
            modal.style.display = 'flex';
          });
        };

        window.openWarningModal = function (action, orgId, apiId, planName, displayName, subscriptionId, currentPlan) {
          if (action === 'SwitchSubscriptionPlan') {
            _unsubAction = 'switch';
            var pendingBtn = window.__pendingPlanSwitchBtn;
            _switchArgs = {
              orgId: orgId, apiId: apiId,
              planName: planName, displayName: displayName,
              subscriptionId: subscriptionId,
              policyId: pendingBtn ? pendingBtn.dataset.planId : undefined,
              btn: pendingBtn,
            };
            var label = displayName || planName;
            showUnsubDialog(
              'Switch to <strong>' + label + '</strong> plan?',
              'Switching from ' + (currentPlan || 'your current plan') + ' to ' + label + '. Your subscription token will remain the same.',
              'Switch plan'
            );
          }
        };

        /* Intercept showAlert to show per-card error and suppress toast */
        var _origShowAlert = window.showAlert;
        window.showAlert = function (message, type) {
          if (type === 'error' && _activeSubCard) {
            var card = _activeSubCard;
            _activeSubCard = null;
            var btn = card.querySelector('.subscribe-btn');
            if (btn) {
              btn.classList.remove('aov-btn-loading');
              if (typeof window.resetSubscribeButtonState === 'function') window.resetSubscribeButtonState(btn);
            }
            var errBlock = card.querySelector('.aov-plan-error');
            if (errBlock) {
              errBlock.classList.add('aov-plan-error--visible');
              setTimeout(function () { errBlock.classList.remove('aov-plan-error--visible'); }, 5000);
            }
            return Promise.resolve();
          }
          return _origShowAlert ? _origShowAlert(message, type) : Promise.resolve();
        };

        /* With a subscription, offer only the plan changes the server allows (subscription.js) */
        if (window.applyPlanSwitchRules) window.applyPlanSwitchRules();

        /* Mark initially suspended cards */
        (window.existingSubscriptions || []).forEach(function (sub) {
          if (sub.status === 'INACTIVE') {
            updateCardSuspendedState(sub.policyName || sub.subscriptionPlanName, true);
          }
        });

        /* View subscription buttons */
        document.querySelectorAll('#subscriptionPlans .api-view-sub-btn').forEach(function (btn) {
          btn.addEventListener('click', function () { openManage(btn.dataset.policyName); });
        });

        /* Manage modal */
        var manageModal = document.getElementById('apiManageModal');
        if (manageModal) {
          document.getElementById('apiManageClose').addEventListener('click', closeManage);
          document.getElementById('apiManageRevealBtn').addEventListener('click', revealToken);
          document.getElementById('apiManageCopyBtn').addEventListener('click', copyManageToken);
          var regenBtn = document.getElementById('apiManageRegenerateBtn');
          if (regenBtn) regenBtn.addEventListener('click', askRegenerate);
          document.getElementById('apiManageSuspendBtn').addEventListener('click', toggleSuspend);
          document.getElementById('apiManageUnsubBtn').addEventListener('click', askUnsub);
          manageModal.addEventListener('click', function (e) { if (e.target === manageModal) closeManage(); });
        }

        /* Regenerate confirmation dialog */
        var regenDialog = document.getElementById('apiRegenerateDialog');
        if (regenDialog) {
          document.getElementById('apiRegenerateCancelBtn').addEventListener('click', closeRegenerate);
          document.getElementById('apiRegenerateConfirmBtn').addEventListener('click', confirmRegenerate);
          regenDialog.addEventListener('click', function (e) { if (e.target === regenDialog) closeRegenerate(); });
        }

        /* Unsub dialog */
        var unsubDialog = document.getElementById('apiUnsubDialog');
        if (unsubDialog) {
          document.getElementById('apiUnsubKeepBtn').addEventListener('click', closeUnsub);
          document.getElementById('apiUnsubConfirmBtn').addEventListener('click', confirmUnsub);
          unsubDialog.addEventListener('click', function (e) { if (e.target === unsubDialog) closeUnsub(); });
        }

        /* Token modal */
        var tokenModal = document.getElementById('apiTokenModal');
        if (tokenModal) {
          document.getElementById('apiTokenModalClose').addEventListener('click', closeTokenModal);
          document.getElementById('apiTokenModalDone').addEventListener('click', closeTokenModal);
          document.getElementById('apiTokenCopyBtn').addEventListener('click', function () {
            if (!tokenModal._token) return;
            var btn = document.getElementById('apiTokenCopyBtn');
            try { navigator.clipboard.writeText(tokenModal._token).catch(function () {}); } catch (e) {}
            btn.classList.add('copy-btn--copied');
            if (btn._copyTimer) clearTimeout(btn._copyTimer);
            btn._copyTimer = setTimeout(function () { btn.classList.remove('copy-btn--copied'); }, 1600);
          });
          tokenModal.addEventListener('click', function (e) { if (e.target === tokenModal) closeTokenModal(); });
        }

        /* Subscribe button wiring */
        document.querySelectorAll('#subscriptionPlans .subscribe-btn:not([disabled])').forEach(function (btn) {
          if (btn.dataset.aovWired) return;
          btn.dataset.aovWired = '1';
          btn.addEventListener('click', function () {
            var card = btn.closest('.aov-plan-card');
            if (!card) return;
            var errBlock = card.querySelector('.aov-plan-error');
            if (errBlock) errBlock.classList.remove('aov-plan-error--visible');
            btn.classList.add('aov-btn-loading');
            _activeSubCard = card;
            setTimeout(function () {
              if (_activeSubCard === card) { _activeSubCard = null; btn.classList.remove('aov-btn-loading'); }
            }, 10000);
          });
        });

        /* Update subscribed card states from existingSubscriptions */
        var existing = window.existingSubscriptions || [];
        var subscribedNames = existing.map(function (s) { return s.policyName || s.subscriptionPlanName; });
        document.querySelectorAll('#subscriptionPlans .aov-plan-card').forEach(function (card) {
          var planName = card.dataset.policyName;
          if (!planName) return;
          if (subscribedNames.indexOf(planName) !== -1) {
            card.classList.add('aov-plan-card--subscribed');
          } else {
            card.classList.remove('aov-plan-card--subscribed');
          }
        });
      }

      if (document.readyState === 'complete') {
        init();
      } else {
        document.addEventListener('DOMContentLoaded', init);
      }
    }());

    /*
     * Paid plans: the subscribe dialog, Stripe's checkout form, and the result.
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

      /* The view this page belongs to: /<base>/<org>/views/<view>/api/<handle>. */
      function viewName() {
        var match = /\/views\/([^/]+)\/api\//.exec(window.location.pathname);
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
