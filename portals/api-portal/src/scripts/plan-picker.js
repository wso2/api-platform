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
 * The Subscribe / Switch plan dialog: the API's plans, each with its name and description, to pick
 * one and subscribe. Opened by any [data-plan-picker] element, which carries the API and its plans:
 *   data-api-id, data-api-name, data-org-id, data-plans (JSON: [{ id, displayName, description, paid }]),
 *   data-current (the subscribed plan's id, when there is one), data-detail-url (listing cards).
 * Subscribing reuses the page's own flow: on an API or MCP server page, the chosen plan card's
 * (hidden) button, so plan switches, paid checkout and the token dialog work as before; on a listing
 * page, handlePlanSubscription (subscription.js). A [data-view-subscription] element opens the
 * subscribed plan's "View subscription" dialog the same way.
 */
(function () {
  var PAID_SWITCH_HINT = 'Paid plans can only be changed by unsubscribing first.';
  var overlay = null;

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
  }

  function readPlans(opener) {
    try {
      var plans = JSON.parse(opener.dataset.plans || '[]');
      return Array.isArray(plans) ? plans.filter(function (p) { return p && p.id; }) : [];
    } catch (e) {
      return [];
    }
  }

  /* Which plans can be picked: not the current one, and no paid plan in a switch (the server
     refuses switching to or from a paid plan). */
  function blockedReason(plan, current, currentPaid) {
    if (plan.id === current) return 'Current plan';
    if (current && (currentPaid || plan.paid)) return PAID_SWITCH_HINT;
    return '';
  }

  function close() {
    if (!overlay) return;
    overlay.remove();
    overlay = null;
    document.removeEventListener('keydown', onKey);
  }

  function onKey(e) {
    if (e.key === 'Escape') close();
  }

  function cardButton(planId) {
    var card = document.querySelector('#subscriptionPlans .aov-plan-card[data-policy-name="' + CSS.escape(planId) + '"]');
    return card && card.querySelector('.subscribe-btn, .paid-subscribe-btn');
  }

  async function subscribe(opener, plan) {
    var onPage = cardButton(plan.id);
    if (onPage) { onPage.click(); return; }
    // A listing card: a paid plan is bought from its page, where checkout runs.
    if (plan.paid || typeof window.handlePlanSubscription !== 'function') {
      window.location.href = (opener.dataset.detailUrl || window.location.pathname) + '#subscriptionPlans';
      return;
    }
    var btn = el('button');
    btn.dataset.orgId = opener.dataset.orgId || '';
    btn.dataset.apiId = opener.dataset.apiId || '';
    btn.dataset.planId = plan.id;
    btn.dataset.planName = plan.id;
    btn.dataset.policyId = plan.id;
    btn.dataset.displayName = plan.displayName || plan.id;
    await window.handlePlanSubscription(btn);
    // The card shows "Subscribed" once the page reloads.
    if (window.__subscriptionChanged) window.location.reload();
  }

  function open(opener) {
    close();
    var plans = readPlans(opener);
    var current = opener.dataset.current || '';
    var currentPaid = plans.some(function (p) { return p.id === current && p.paid; });
    var switching = Boolean(current);
    var selected = null;

    overlay = el('div', 'mc-overlay plan-picker-overlay');
    overlay.setAttribute('role', 'dialog');
    overlay.setAttribute('aria-modal', 'true');
    var dialog = el('div', 'mc-unsub-dialog plan-picker');
    overlay.appendChild(dialog);

    var head = el('div', 'plan-picker-head');
    var headIcon = el('span', 'plan-picker-head-icon');
    headIcon.appendChild(el('i', switching ? 'bi bi-arrow-left-right' : 'bi bi-bookmark-plus'));
    var heading = el('div', 'plan-picker-heading');
    var title = el('h2', 'plan-picker-title', switching ? 'Switch plan' : 'Subscribe');
    title.id = 'planPickerTitle';
    overlay.setAttribute('aria-labelledby', title.id);
    var closeBtn = el('button', 'plan-picker-close');
    closeBtn.appendChild(el('i', 'bi bi-x-lg'));
    closeBtn.type = 'button';
    closeBtn.setAttribute('aria-label', 'Close');
    closeBtn.addEventListener('click', close);
    var apiName = opener.dataset.apiName;
    heading.appendChild(title);
    heading.appendChild(el('p', 'plan-picker-sub',
      switching ? 'Choose the plan to move to. Your subscription token stays the same.'
        : 'Choose a plan' + (apiName ? ' for ' + apiName : '') + '.'));
    head.appendChild(headIcon);
    head.appendChild(heading);
    head.appendChild(closeBtn);
    dialog.appendChild(head);

    var list = el('div', 'plan-picker-list');
    list.setAttribute('role', 'radiogroup');
    plans.forEach(function (plan) {
      var reason = blockedReason(plan, current, currentPaid);
      var option = el('label', 'plan-picker-option' + (reason ? ' plan-picker-option--disabled' : ''));
      var radio = el('input', 'plan-picker-radio');
      radio.type = 'radio';
      radio.name = 'planPickerPlan';
      radio.value = plan.id;
      radio.disabled = Boolean(reason);
      radio.addEventListener('change', function () { selected = plan; confirmBtn.disabled = false; });
      var text = el('span', 'plan-picker-text');
      var nameRow = el('span', 'plan-picker-name');
      nameRow.appendChild(el('span', 'plan-picker-name-text', plan.displayName || plan.id));
      if (plan.paid) nameRow.appendChild(el('span', 'aov-plan-badge aov-plan-badge--paid', 'Paid'));
      // The plan the subscriber is on: a small pill by its name; another reason reads under it.
      if (plan.id === current) nameRow.appendChild(el('span', 'plan-picker-pill', reason));
      text.appendChild(nameRow);
      if (plan.description) text.appendChild(el('span', 'plan-picker-desc', plan.description));
      if (reason && plan.id !== current) text.appendChild(el('span', 'plan-picker-note', reason));
      // The plan's icon tile, as on the details page; the choice is a round check on the right.
      var planIcon = el('span', 'plan-picker-icon');
      planIcon.appendChild(el('i', 'bi bi-lightning-charge-fill'));
      option.appendChild(planIcon);
      option.appendChild(text);
      option.appendChild(radio);
      list.appendChild(option);
    });
    dialog.appendChild(list);

    var actions = el('div', 'plan-picker-actions');
    var cancelBtn = el('button', 'dp-btn dp-btn--outline', 'Cancel');
    cancelBtn.type = 'button';
    cancelBtn.addEventListener('click', close);
    var confirmBtn = el('button', 'dp-btn dp-btn--primary plan-picker-confirm', switching ? 'Switch plan' : 'Subscribe');
    confirmBtn.type = 'button';
    confirmBtn.disabled = true;
    confirmBtn.addEventListener('click', function () {
      if (!selected) return;
      var plan = selected;
      close();
      subscribe(opener, plan);
    });
    actions.appendChild(cancelBtn);
    actions.appendChild(confirmBtn);
    dialog.appendChild(actions);

    overlay.addEventListener('click', function (e) { if (e.target === overlay) close(); });
    document.addEventListener('keydown', onKey);
    document.body.appendChild(overlay);
    // Focus the dialog itself, not the first plan: no plan looks chosen until one is clicked.
    dialog.tabIndex = -1;
    dialog.focus();
  }

  if (typeof document !== 'undefined') document.addEventListener('click', function (e) {
    var opener = e.target.closest && e.target.closest('[data-plan-picker]');
    if (opener) {
      e.preventDefault();
      open(opener);
      return;
    }
    var view = e.target.closest && e.target.closest('[data-view-subscription]');
    if (view) {
      e.preventDefault();
      var card = document.querySelector('#subscriptionPlans .aov-plan-card[data-policy-name="' + CSS.escape(view.dataset.viewSubscription) + '"]');
      var btn = card && card.querySelector('.mcp-view-sub-btn, .api-view-sub-btn');
      if (btn) btn.click();
    }
  });

  if (typeof module !== 'undefined' && module.exports) module.exports = { blockedReason: blockedReason };
}());
