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
  var _cfg = document.getElementById('cfg-page-config') || { dataset: {} };
  var ORG_ID = _cfg.dataset.orgId || '';
  var editWebhookId = null;
  // Whether the subscriber being edited already has a stored secret. A blank secret
  // field means "keep the stored one", so the mandatory check below only applies when
  // there is nothing stored to keep — on create, or on a legacy row saved without one.
  var editHasSecret = false;

  function v(id) { var e=document.getElementById(id); return e?e.value.trim():''; }

  /* build id→webhook lookup from server-rendered data blob */
  var webhookMap = {};
  (function() {
    try {
      var el = document.getElementById('cfg-webhooks-data');
      if (el) {
        var list = JSON.parse(el.textContent || '[]');
        list.forEach(function(wh) { webhookMap[wh.id] = wh; });
      }
    } catch(e) {}
  }());

  /* ── all events / select events segmented control ── */
  var allEventsBtn = document.getElementById('wh-events-all-btn');
  var selectEventsBtn = document.getElementById('wh-events-select-btn');

  function setEventsMode(allEvents) {
    allEventsBtn.classList.toggle('active', allEvents);
    selectEventsBtn.classList.toggle('active', !allEvents);
    document.getElementById('wh-event-list').style.display = allEvents ? 'none' : 'flex';
  }
  allEventsBtn.addEventListener('click', function() { setEventsMode(true); });
  selectEventsBtn.addEventListener('click', function() { setEventsMode(false); });

  function eventChecksIn(group) {
    return document.querySelectorAll('.wh-event-check[data-group="' + group + '"]');
  }

  function allCheckedIn(group) {
    var members = eventChecksIn(group);
    if (members.length === 0) return false;
    return Array.prototype.every.call(members, function(m) { return m.checked; });
  }

  /* Each category's control flips between "Select all" and "Clear" to match its current
     state. The ticks themselves already show a partial selection, so the label only has
     to say what clicking will do. Called after any change so it can't go stale. */
  function syncEventGroupToggles() {
    document.querySelectorAll('.wh-event-group-toggle').forEach(function(btn) {
      btn.textContent = allCheckedIn(btn.dataset.group) ? 'Clear' : 'Select all';
    });
  }

  document.querySelectorAll('.wh-event-group-toggle').forEach(function(btn) {
    btn.addEventListener('click', function() {
      var select = !allCheckedIn(this.dataset.group);
      eventChecksIn(this.dataset.group).forEach(function(m) { m.checked = select; });
      syncEventGroupToggles();
    });
  });

  document.querySelectorAll('.wh-event-check').forEach(function(c) {
    c.addEventListener('change', syncEventGroupToggles);
  });

  function selectedEvents() {
    if (allEventsBtn.classList.contains('active')) return [];
    return Array.prototype.slice.call(document.querySelectorAll('.wh-event-check:checked')).map(function(c){ return c.value; });
  }
  function setSelectedEvents(events) {
    document.querySelectorAll('.wh-event-check').forEach(function(c) { c.checked = false; });
    var everything = !events || events.length === 0;
    setEventsMode(everything);
    if (!everything) {
      events.forEach(function(name) {
        // Stored patterns may be wildcards ("apikey.*"), which match no single checkbox —
        // tick the whole category instead so an existing subscriber renders faithfully.
        if (name.slice(-2) === '.*') {
          eventChecksIn(name.slice(0, -2)).forEach(function(m) { m.checked = true; });
          return;
        }
        var cb = document.querySelector('.wh-event-check[value="'+name+'"]');
        if (cb) cb.checked = true;
      });
    }
    syncEventGroupToggles();
  }

  /* ── list / form view swap (in-panel, not a modal — mirrors the API Workflows panel) ── */
  var listView = document.getElementById('wh-list-view');
  var formView = document.getElementById('wh-form-view');

  function showWebhookForm(mode, data) {
    editWebhookId = mode === 'edit' ? data.id : null;
    editHasSecret = mode === 'edit' && !!data.hasSecret;
    document.getElementById('wh-form-title').textContent = mode === 'edit' ? 'Edit webhook' : 'Add webhook';
    document.getElementById('wh-form-save').textContent  = mode === 'edit' ? 'Save changes' : 'Add webhook';
    document.getElementById('wh-display').value   = mode === 'edit' ? (data.displayName || '') : '';
    document.getElementById('wh-url').value       = mode === 'edit' ? (data.targetUrl || '')       : '';
    document.getElementById('wh-secret').value    = '';
    document.getElementById('wh-timeout').value   = mode === 'edit' ? (data.timeoutMs || 5000) : 5000;
    document.getElementById('wh-enabled').checked = mode === 'edit' ? !!data.enabled : true;
    document.getElementById('wh-secret-hint').style.display = editHasSecret ? 'block' : 'none';
    setSelectedEvents(mode === 'edit' ? data.events : []);
    syncWhSave();
    listView.style.display = 'none';
    formView.style.display = 'block';
    // The panel scrolls, so returning from a long list would otherwise open the form
    // scrolled halfway down it.
    formView.scrollIntoView({ block: 'start' });
    document.getElementById('wh-display').focus();
  }

  function showWebhookList() {
    formView.style.display = 'none';
    listView.style.display = 'block';
    editWebhookId = null;
    editHasSecret = false;
  }

  /* Disable save until Name and Target URL are filled, and (on create, or a legacy row
     with no stored secret) a Secret is entered — mirrors the submit-time validation. */
  var syncWhSave = bindFormValidity(document.getElementById('wh-form-save'),
    ['wh-display', 'wh-url', 'wh-secret'], function() {
      return v('wh-display') !== '' && v('wh-url') !== '' && (editHasSecret || v('wh-secret') !== '');
    });

  /* ── save ── */
  document.getElementById('wh-form-save').addEventListener('click', async function() {
    var saveBtn = this;
    var displayName = v('wh-display');
    var url         = v('wh-url');
    if (!displayName || !url) { await showAlert('Name and target URL are required.', 'error'); return; }

    // The secret both signs deliveries and encrypts sensitive event fields, so a
    // subscriber without one silently loses the API key / token payloads entirely.
    // On edit, a blank field keeps the stored secret — only require one when there
    // is none stored.
    if (!v('wh-secret') && !editHasSecret) {
      await showAlert('A secret is required — it signs each delivery and encrypts the API key and token fields.', 'error');
      return;
    }

    var parsedUrl;
    try { parsedUrl = new URL(url); } catch (e) { parsedUrl = null; }
    if (!parsedUrl || (parsedUrl.protocol !== 'http:' && parsedUrl.protocol !== 'https:')) {
      await showAlert('Target URL must be a valid http:// or https:// URL.', 'error');
      return;
    }

    var timeoutRaw = v('wh-timeout');
    var timeoutMs = parseInt(timeoutRaw, 10);
    if (!Number.isFinite(timeoutMs) || timeoutMs <= 0) {
      await showAlert('Timeout (ms) must be a positive number.', 'error');
      return;
    }

    var events = selectedEvents();
    if (!allEventsBtn.classList.contains('active') && events.length === 0) {
      await showAlert('Select at least one event, or choose "All events".', 'error');
      return;
    }

    var body = {
      displayName: displayName,
      targetUrl: url,
      events: events,
      enabled: document.getElementById('wh-enabled').checked,
      timeoutMs: timeoutMs,
    };
    // The UI never sends an `id`: on create the server generates a UUID handle,
    // and on update the handle is immutable, so there is nothing to send.
    var secret = v('wh-secret');
    if (secret) body.secret = secret;

    var url2   = editWebhookId
      ? window.apiPortalApi.root('/webhook-subscribers/' + encodeURIComponent(editWebhookId))
      : window.apiPortalApi.root('/webhook-subscribers');
    var method = editWebhookId ? 'PUT' : 'POST';

    await withButtonBusy(saveBtn, editWebhookId ? 'Saving…' : 'Adding…', async function() {
      try {
        var res = await fetch(url2, {
          method: method,
          headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
          body: JSON.stringify(body),
        });
        if (res.ok) {
          await showAlert(editWebhookId ? 'Webhook updated.' : 'Webhook created.', 'success');
          window.location.reload();
        } else {
          var err = await res.json().catch(function(){ return {}; });
          await showAlert('Failed: ' + (err.error || err.description || err.message || res.statusText), 'error');
        }
      } catch(e) { await showAlert('Error: ' + e.message, 'error'); }
    });
  });

  document.getElementById('wh-form-cancel').addEventListener('click', showWebhookList);
  document.getElementById('wh-form-back').addEventListener('click', function(e) {
    e.preventDefault();
    showWebhookList();
  });

  document.getElementById('cfg-add-webhook-btn').addEventListener('click', function() { showWebhookForm('add'); });

  /* ── edit / delete via event delegation ── */
  var pendingDelWebhookId = null;
  document.addEventListener('click', function(e) {
    if (e.target.closest('.cfg-webhook-edit-btn')) {
      var btn = e.target.closest('.cfg-webhook-edit-btn');
      var data = webhookMap[btn.dataset.id];
      if (data) showWebhookForm('edit', data);
      return;
    }
    if (e.target.closest('.cfg-webhook-delete-btn')) {
      btn = e.target.closest('.cfg-webhook-delete-btn');
      pendingDelWebhookId = btn.dataset.id;
      document.getElementById('cfg-del-webhook-name-txt').textContent = btn.dataset.name;
      document.getElementById('cfg-delete-webhook-modal').style.display = 'flex';
      return;
    }
  });

  document.getElementById('cfg-del-webhook-cancel').addEventListener('click', function() {
    document.getElementById('cfg-delete-webhook-modal').style.display = 'none';
  });
  document.getElementById('cfg-delete-webhook-modal').addEventListener('click', function(e){ if(e.target===this) this.style.display='none'; });
  document.getElementById('cfg-del-webhook-confirm').addEventListener('click', function() {
    if (!pendingDelWebhookId) return;
    withButtonBusy(this, 'Deleting…', async function() {
      try {
        var res = await fetch(window.apiPortalApi.root('/webhook-subscribers/' + encodeURIComponent(pendingDelWebhookId)), {
          method: 'DELETE',
          headers: { 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
        });
        if (res.ok || res.status === 204) {
          await showAlert('Webhook deleted.', 'success');
          window.location.reload();
          return;
        }
        var err = await res.json().catch(function(){ return {}; });
        await showAlert('Delete failed: ' + (err.error || err.description || err.message || res.statusText), 'error');
      } catch(e) { await showAlert('Error: ' + e.message, 'error'); }
      document.getElementById('cfg-delete-webhook-modal').style.display = 'none';
      pendingDelWebhookId = null;
    });
  });
}());
