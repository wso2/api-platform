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

      function openManage(subscriptionId) {
        var sub = window.subscriptionState.findSubscription(window.existingSubscriptions, subscriptionId);
        if (!sub) return;
        _sub = sub;
        _revealed = false;

        document.getElementById('apiManagePlanName').textContent = sub.subscriptionPlanName || sub.policyName;
        updateManageStatus(sub.status);
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
            window.subscriptionState.setSubscriptionStatus(window.existingSubscriptions, _sub.subscriptionId, newStatus);
            updateManageStatus(newStatus);
            updateRowStatus(_sub.subscriptionId, newStatus);
            refreshPlanCards();
          } else {
            var data = await resp.json().catch(function () { return {}; });
            showAlert('Failed to update subscription: ' + (data.description || 'Unknown error'), 'error');
          }
        } catch (e) {
          showAlert('Error: ' + e.message, 'error');
        }
      }

      function updateRowStatus(subscriptionId, status) {
        var row = document.querySelector('#apiSubscriptionsTable tr[data-subscription-id="' + subscriptionId + '"] .api-sub-status');
        if (!row) return;
        var isSusp = status === 'INACTIVE';
        row.textContent = isSusp ? 'Suspended' : 'Active';
        row.className = 'mc-manage-status-badge api-sub-status ' + (isSusp ? 'mc-manage-status-badge--suspended' : 'mc-manage-status-badge--active');
      }

      /* A plan card reflects every subscription held on that plan. */
      function refreshPlanCards() {
        document.querySelectorAll('#subscriptionPlans .aov-plan-card').forEach(function (card) {
          var planName = card.dataset.policyName;
          if (!planName) return;
          var state = window.subscriptionState.planCardState(planName, window.existingSubscriptions);
          card.classList.toggle('aov-plan-card--subscribed', state.subscribed);
          card.classList.toggle('aov-plan-card--suspended', state.suspended);
          var label = card.querySelector('.api-ribbon-label');
          if (label) label.textContent = (state.suspended ? 'SUSPENDED' : 'SUBSCRIBED') + (state.count > 1 ? ' ×' + state.count : '');
        });
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

        /* Subscribe always creates a new subscription on this page (see handlePlanSubscription). */
        window.allowMultipleSubscriptions = true;

        /* Manage buttons: one per subscription row */
        document.querySelectorAll('#apiSubscriptionsTable .api-view-sub-btn').forEach(function (btn) {
          btn.addEventListener('click', function () { openManage(btn.dataset.subscriptionId); });
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

        refreshPlanCards();
      }

      if (document.readyState === 'complete') {
        init();
      } else {
        document.addEventListener('DOMContentLoaded', init);
      }
    }());
