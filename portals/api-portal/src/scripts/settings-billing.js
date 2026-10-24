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
  var bindFormValidity = window.bindFormValidity || function () { return function () {}; };
  var panel = document.getElementById('cfg-billing');
  if (!panel) return;

  function v(id) { var e = document.getElementById(id); return e ? e.value.trim() : ''; }

  function api(path, method, body) {
    var headers = { 'X-CSRF-Token': window.apiPortalApi.csrfToken() };
    if (body) headers['Content-Type'] = 'application/json';
    return fetch(window.apiPortalApi.root(path), { method: method, headers: headers, body: body ? JSON.stringify(body) : undefined });
  }

  /*
   * What to tell the administrator when a billing request fails. The server's own wording is
   * shown only for what the administrator can act on in the form (a refused key, an invalid
   * entry); everything else gets a fixed message.
   *   failed     such as "Billing could not be saved.", used for errors the administrator cannot fix
   *   returns    { message, fieldErrors: [{ field, message }] }
   */
  var SESSION_EXPIRED = 'Your session has expired. Reload the page and sign in again.';

  async function failureOf(res, failed) {
    var body = await res.json().catch(function () { return {}; });
    var serverText = body.message || body.description || '';
    if (res.status === 401 || (res.status === 403 && /csrf/i.test(serverText))) {
      // The CSRF token belongs to the session, so a missing one means the session is gone too.
      return { message: SESSION_EXPIRED, fieldErrors: [] };
    }
    if (res.status === 403) {
      return { message: 'You do not have permission to manage billing.', fieldErrors: [] };
    }
    if (res.status === 404) {
      return { message: 'Billing is no longer set up. Reload the page.', fieldErrors: [] };
    }
    var fieldErrors = (body.errors || []).filter(function (e) { return e.field && e.message; });
    if (['BILLING_KEY_INVALID', 'BILLING_PROVIDER_UNAVAILABLE', 'BILLING_INPUT_INVALID'].indexOf(body.code) >= 0
        && fieldErrors.length) {
      return { message: '', fieldErrors: fieldErrors };
    }
    if ((res.status === 400 || res.status === 409) && serverText) {
      return { message: serverText, fieldErrors: [] };
    }
    return { message: failed + ' Try again later.', fieldErrors: [] };
  }

  function unreachable(failed) {
    return { message: failed + ' Check your connection and try again.', fieldErrors: [] };
  }

  /* "Last webhook event" as a relative time: "2 minutes ago". */
  document.querySelectorAll('.cfg-billing-time[data-time]').forEach(function (el) {
    var at = Date.parse(el.dataset.time);
    if (isNaN(at)) return;
    var seconds = Math.max(0, Math.round((Date.now() - at) / 1000));
    var steps = [[60, 'second'], [60, 'minute'], [24, 'hour'], [30, 'day'], [12, 'month']];
    var value = seconds; var unit = 'second';
    for (var i = 0; i < steps.length && value >= steps[i][0]; i++) {
      value = Math.floor(value / steps[i][0]);
      unit = (steps[i + 1] || ['', 'year'])[1];
    }
    el.textContent = value < 1 && unit === 'second' ? 'Just now' : value + ' ' + unit + (value === 1 ? '' : 's') + ' ago';
    el.title = new Date(at).toLocaleString();
  });

  /* ── set up ── */
  /* Two steps: the billing engine's keys (Next), then the payment provider's (Set up billing).
     Both are sent, and checked by the server, together. */
  var formView = document.getElementById('cfg-billing-modal');
  var saveBtn = document.getElementById('bil-form-save');
  var nextBtn = document.getElementById('bil-form-next');
  var backBtn = document.getElementById('bil-form-back');
  var cancelBtn = document.getElementById('bil-form-cancel');
  // Every key input of every provider; the chosen engine and payment provider decide which are needed.
  var KEY_FIELDS = Array.prototype.map.call(formView.querySelectorAll('input.cfg-form-input'), function (el) { return el.id; });

  /* The chosen value: a checked radio when the form offers a choice, else the single pair's
     hidden field. */
  function checked(name) {
    var input = formView.querySelector('input[name="' + name + '"]:checked')
      || formView.querySelector('input[type="hidden"][name="' + name + '"]');
    return input ? input.value : '';
  }
  function selectedEngine() { return checked('bil-engine'); }
  function selectedPayment() { return checked('bil-payment-' + selectedEngine()); }

  /* Show the keys of the chosen billing engine and of the payment provider it bills through. */
  function syncSections() {
    var engine = selectedEngine();
    formView.querySelectorAll('.bil-engine-section').forEach(function (el) {
      el.hidden = el.dataset.engine !== engine;
    });
    var payment = selectedPayment();
    formView.querySelectorAll('.bil-payment-section').forEach(function (el) {
      el.hidden = el.dataset.payment !== payment;
    });
    syncNext();
    syncSave();
  }

  /* A step's keys: the chosen engine's in step 1, the chosen payment provider's in step 2. */
  function stepSection(step) {
    var engine = formView.querySelector('.bil-step[data-step="' + step + '"] .bil-engine-section[data-engine="' + selectedEngine() + '"]');
    if (!engine || step === 1) return engine;
    return engine.querySelector('.bil-payment-section[data-payment="' + selectedPayment() + '"]');
  }

  /* A step is complete when its section is there and every key in it is entered. */
  function complete(step) {
    var section = stepSection(step);
    return !!section && Array.prototype.every.call(section.querySelectorAll('input.cfg-form-input'),
      function (el) { return el.value.trim() !== ''; });
  }
  var syncNext = bindFormValidity(nextBtn, KEY_FIELDS, function () { return complete(1); });
  var syncSave = bindFormValidity(saveBtn, KEY_FIELDS, function () { return complete(1) && complete(2); });

  /* Shows one step: its keys, where it is in the stepper, and its buttons. */
  function showStep(step) {
    formView.querySelectorAll('.bil-step').forEach(function (el) { el.hidden = Number(el.dataset.step) !== step; });
    formView.querySelectorAll('#bil-stepper .cfg-step').forEach(function (el) {
      var at = Number(el.dataset.step);
      el.classList.toggle('cfg-step--active', at === step);
      el.classList.toggle('cfg-step--done', at < step);
      var circle = el.querySelector('.cfg-step-circle');
      if (at < step) {
        var check = document.createElement('i');
        check.className = 'bi bi-check';
        check.setAttribute('aria-hidden', 'true');
        circle.replaceChildren(check);
      } else {
        circle.textContent = String(at);
      }
    });
    formView.querySelector('#bil-stepper .cfg-step-connector').classList.toggle('cfg-step-connector--done', step > 1);
    cancelBtn.style.display = step === 1 ? '' : 'none';
    nextBtn.style.display = step === 1 ? '' : 'none';
    backBtn.style.display = step === 1 ? 'none' : '';
    saveBtn.style.display = step === 1 ? 'none' : '';
    var section = stepSection(step);
    var inputs = section ? Array.prototype.slice.call(section.querySelectorAll('input.cfg-form-input')) : [];
    var first = inputs.filter(function (el) { return el.value.trim() === ''; })[0] || inputs[0];
    if (first) first.focus();
  }

  function showForm() {
    KEY_FIELDS.forEach(function (id) { document.getElementById(id).value = ''; });
    syncSections();
    clearErrors();
    revealButtons.forEach(function (button) { setRevealed(button, false); });
    formView.style.display = 'flex';
    showStep(1);
  }

  function closeForm() { formView.style.display = 'none'; }

  var setupBtn = document.getElementById('cfg-billing-setup-btn');
  if (setupBtn) setupBtn.addEventListener('click', showForm);
  formView.querySelectorAll('input[type="radio"]').forEach(function (r) { r.addEventListener('change', syncSections); });
  nextBtn.addEventListener('click', function () { showStep(2); });
  backBtn.addEventListener('click', function () { showStep(1); });
  // The finished first step in the stepper goes back to it, like Back.
  formView.querySelector('#bil-stepper .cfg-step[data-step="1"]').addEventListener('click', function () {
    if (this.classList.contains('cfg-step--done')) showStep(1);
  });
  document.getElementById('bil-form-close').addEventListener('click', closeForm);
  cancelBtn.addEventListener('click', closeForm);
  formView.addEventListener('click', function (e) { if (e.target === formView) closeForm(); });

  /* Errors are shown in the modal: a provider's refusal under the key it is about, anything
     else at the top. They clear when the modal opens and when the field is edited. */
  var formError = document.getElementById('bil-form-error');

  function setFieldError(field, message) {
    var input = formView.querySelector('input[data-field="' + field + '"]');
    var text = formView.querySelector('[data-error-for="' + field + '"]');
    if (!input || !text) return false;
    input.classList.toggle('cfg-form-input--error', !!message);
    text.textContent = message || '';
    text.hidden = !message;
    return true;
  }

  function setFormError(message) {
    document.getElementById('bil-form-error-text').textContent = message || '';
    formError.hidden = !message;
  }

  function clearErrors() {
    setFormError('');
    formView.querySelectorAll('input[data-field]').forEach(function (el) { setFieldError(el.dataset.field, ''); });
  }

  /* Shows a failed save: each field error under its field, the rest at the top. A refused key
     of the first step takes the administrator back to it. */
  function showSaveFailure(failure) {
    var unplaced = failure.fieldErrors.filter(function (e) { return !setFieldError(e.field, e.message); });
    var message = [failure.message].concat(unplaced.map(function (e) { return e.message; })).filter(Boolean).join(' ');
    setFormError(message);
    var first = formView.querySelector('.cfg-form-input--error');
    if (!first) return;
    showStep(Number(first.closest('.bil-step').dataset.step));
    first.focus();
  }

  formView.querySelectorAll('input[data-field]').forEach(function (el) {
    el.addEventListener('input', function () { setFieldError(el.dataset.field, ''); });
  });

  /* Show / hide a key. Every key is hidden again when the modal opens. */
  function setRevealed(button, show) {
    var input = document.getElementById(button.dataset.reveal);
    input.type = show ? 'text' : 'password';
    button.setAttribute('aria-pressed', show ? 'true' : 'false');
    button.setAttribute('aria-label', show ? 'Hide the key' : 'Show the key');
    button.querySelector('i').className = show ? 'bi bi-eye-slash' : 'bi bi-eye';
  }
  var revealButtons = formView.querySelectorAll('[data-reveal]');
  revealButtons.forEach(function (button) {
    button.addEventListener('click', function () {
      setRevealed(button, document.getElementById(button.dataset.reveal).type === 'password');
    });
  });

  saveBtn.addEventListener('click', async function () {
    clearErrors();
    var body = {
      provider: selectedEngine(),
      paymentProvider: selectedPayment(),
      moesifCollectorApplicationId: v('bil-moesif-app') || undefined,
      moesifManagementKey: v('bil-moesif-key') || undefined,
      stripeApiKey: v('bil-stripe-key') || undefined,
      stripePublishableKey: v('bil-stripe-pk') || undefined,
    };
    if (body.stripeApiKey && !/^(sk|rk)_(test|live)_/.test(body.stripeApiKey)) {
      setFieldError('stripeApiKey', 'Enter a restricted key (rk_…) or secret key (sk_…), not a publishable key.');
      return;
    }
    if (body.stripePublishableKey && !/^pk_(test|live)_/.test(body.stripePublishableKey)) {
      setFieldError('stripePublishableKey', 'Enter a publishable key (pk_test_… or pk_live_…).');
      return;
    }
    await withButtonBusy(this, 'Checking keys…', async function () {
      try {
        var res = await api('/billing-configuration', 'POST', body);
        if (res.ok) {
          await showAlert('Billing is set up.', 'success');
          window.location.reload();
          return;
        }
        showSaveFailure(await failureOf(res, 'Billing could not be set up.'));
      } catch (e) { showSaveFailure(unreachable('Billing could not be set up.')); }
    });
  });

  /* ── webhook host ── */
  /* Moves the portal's endpoint in Stripe to another public address. Save is enabled once the
     address differs from the current one; Stripe's refusal is shown under the field. */
  var webhookModal = document.getElementById('cfg-billing-webhook-modal');
  var webhookEditBtn = document.getElementById('cfg-billing-webhook-edit-btn');
  if (webhookModal && webhookEditBtn) {
    var webhookInput = document.getElementById('bil-webhook-edit-base');
    var webhookSave = document.getElementById('bil-webhook-save');
    var webhookFieldError = webhookModal.querySelector('[data-error-for="webhookBaseUrl"]');

    var setWebhookFieldError = function (message) {
      webhookInput.classList.toggle('cfg-form-input--error', !!message);
      webhookFieldError.textContent = message || '';
      webhookFieldError.hidden = !message;
    };
    var setWebhookError = function (message) {
      document.getElementById('bil-webhook-error-text').textContent = message || '';
      document.getElementById('bil-webhook-error').hidden = !message;
    };
    var syncWebhookSave = bindFormValidity(webhookSave, ['bil-webhook-edit-base'], function () {
      var value = webhookInput.value.trim();
      return value !== '' && value !== webhookEditBtn.dataset.baseUrl;
    });
    var closeWebhook = function () { webhookModal.style.display = 'none'; };

    webhookEditBtn.addEventListener('click', function () {
      webhookInput.value = webhookEditBtn.dataset.baseUrl || '';
      setWebhookFieldError('');
      setWebhookError('');
      syncWebhookSave();
      webhookModal.style.display = 'flex';
      webhookInput.focus();
    });
    webhookInput.addEventListener('input', function () { setWebhookFieldError(''); });
    document.getElementById('bil-webhook-close').addEventListener('click', closeWebhook);
    document.getElementById('bil-webhook-cancel').addEventListener('click', closeWebhook);
    webhookModal.addEventListener('click', function (e) { if (e.target === webhookModal) closeWebhook(); });

    webhookSave.addEventListener('click', async function () {
      setWebhookFieldError('');
      setWebhookError('');
      var failed = 'The webhook host could not be changed.';
      await withButtonBusy(this, 'Saving…', async function () {
        try {
          var res = await api('/billing-configuration/webhook-endpoint', 'PUT', { webhookBaseUrl: webhookInput.value.trim() });
          if (res.ok) {
            await showAlert('Webhook host changed.', 'success');
            window.location.reload();
            return;
          }
          var failure = await failureOf(res, failed);
          var fieldError = failure.fieldErrors.filter(function (e) { return e.field === 'webhookBaseUrl'; })[0];
          if (fieldError) {
            setWebhookFieldError(fieldError.message);
            webhookInput.focus();
          } else {
            setWebhookError(failure.message || failure.fieldErrors.map(function (e) { return e.message; }).join(' '));
          }
        } catch (e) { setWebhookError(unreachable(failed).message); }
      });
    });
  }

  /* ── remove ── */
  var removeModal = document.getElementById('cfg-billing-remove-modal');
  var removeBtn = document.getElementById('cfg-billing-remove-btn');
  /* While paid subscriptions are active, the dialog says billing can't be removed yet and Remove is disabled. */
  if (removeBtn) removeBtn.addEventListener('click', function () {
    var active = Number(removeBtn.dataset.activeSubscriptions) || 0;
    var blocked = active > 0;
    document.getElementById('cfg-billing-remove-ask').style.display = blocked ? 'none' : '';
    document.getElementById('cfg-billing-remove-blocked').style.display = blocked ? '' : 'none';
    document.getElementById('cfg-billing-remove-confirm').disabled = blocked;
    removeModal.style.display = 'flex';
  });
  document.getElementById('cfg-billing-remove-cancel').addEventListener('click', function () { removeModal.style.display = 'none'; });
  removeModal.addEventListener('click', function (e) { if (e.target === removeModal) removeModal.style.display = 'none'; });
  document.getElementById('cfg-billing-remove-confirm').addEventListener('click', function () {
    withButtonBusy(this, 'Removing…', async function () {
      try {
        var res = await api('/billing-configuration', 'DELETE');
        if (res.ok || res.status === 204) {
          await showAlert('Billing removed.', 'success');
          window.location.reload();
          return;
        }
        removeModal.style.display = 'none';
        await showAlert((await failureOf(res, 'Billing could not be removed.')).message, 'error');
      } catch (e) {
        removeModal.style.display = 'none';
        await showAlert(unreachable('Billing could not be removed.').message, 'error');
      }
    });
  });
}());
