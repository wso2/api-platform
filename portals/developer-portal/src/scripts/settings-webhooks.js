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
        var cb = document.querySelector('.wh-event-check[value="'+name+'"]');
        if (cb) cb.checked = true;
      });
    }
  }

  /* ── open modal ── */
  function openWebhookModal(mode, data) {
    editWebhookId = mode === 'edit' ? data.id : null;
    document.getElementById('cfg-webhook-modal-title').textContent = mode === 'edit' ? 'Edit webhook' : 'Add webhook';
    document.getElementById('cfg-webhook-modal-save').textContent  = mode === 'edit' ? 'Save changes' : 'Add webhook';
    document.getElementById('wh-display').value   = mode === 'edit' ? (data.displayName || '') : '';
    document.getElementById('wh-handle').value    = mode === 'edit' ? (data.id || '')          : '';
    document.getElementById('wh-url').value       = mode === 'edit' ? (data.targetUrl || '')       : '';
    document.getElementById('wh-secret').value    = '';
    document.getElementById('wh-publickey').value = '';
    document.getElementById('wh-timeout').value   = mode === 'edit' ? (data.timeoutMs || 5000) : 5000;
    document.getElementById('wh-enabled').checked = mode === 'edit' ? !!data.enabled : true;
    document.getElementById('wh-secret-hint').style.display    = mode === 'edit' && data.hasSecret ? 'block' : 'none';
    document.getElementById('wh-publickey-hint').style.display = mode === 'edit' && data.hasPublicKey ? 'block' : 'none';
    setSelectedEvents(mode === 'edit' ? data.events : []);
    document.getElementById('cfg-webhook-modal').style.display = 'flex';
    document.getElementById('wh-display').focus();
  }
  function closeWebhookModal() { document.getElementById('cfg-webhook-modal').style.display = 'none'; editWebhookId = null; }

  /* ── auto-slug display → handle ── */
  document.getElementById('wh-display').addEventListener('input', function() {
    if (editWebhookId) return;
    document.getElementById('wh-handle').value = this.value.toLowerCase().trim().replace(/[^a-z0-9]+/g,'-').replace(/^-|-$/g,'');
  });

  /* ── save ── */
  document.getElementById('cfg-webhook-modal-save').addEventListener('click', async function() {
    var displayName = v('wh-display');
    var handle      = v('wh-handle');
    var url         = v('wh-url');
    if (!displayName || !handle || !url) { await showAlert('Display name, handle and target URL are required.', 'error'); return; }

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
      id: handle,
      displayName: displayName,
      targetUrl: url,
      events: events,
      enabled: document.getElementById('wh-enabled').checked,
      timeoutMs: timeoutMs,
    };
    var secret = v('wh-secret');
    if (secret) body.secret = secret;
    var publicKey = v('wh-publickey');
    if (publicKey) body.publicKey = publicKey;

    var url2   = editWebhookId
      ? window.devportalApi.root('/webhook-subscribers/' + encodeURIComponent(editWebhookId))
      : window.devportalApi.root('/webhook-subscribers');
    var method = editWebhookId ? 'PUT' : 'POST';

    try {
      var res = await fetch(url2, {
        method: method,
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.devportalApi.csrfToken() },
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

  document.getElementById('cfg-webhook-modal-close').addEventListener('click', closeWebhookModal);
  document.getElementById('cfg-webhook-modal-cancel').addEventListener('click', closeWebhookModal);
  document.getElementById('cfg-webhook-modal').addEventListener('click', function(e){ if(e.target===this) closeWebhookModal(); });

  document.getElementById('cfg-add-webhook-btn').addEventListener('click', function() { openWebhookModal('add'); });

  /* ── edit / delete via event delegation ── */
  var pendingDelWebhookId = null;
  document.addEventListener('click', function(e) {
    if (e.target.closest('.cfg-webhook-edit-btn')) {
      var btn = e.target.closest('.cfg-webhook-edit-btn');
      var data = webhookMap[btn.dataset.id];
      if (data) openWebhookModal('edit', data);
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
  document.getElementById('cfg-del-webhook-confirm').addEventListener('click', async function() {
    if (!pendingDelWebhookId) return;
    document.getElementById('cfg-delete-webhook-modal').style.display = 'none';
    try {
      var res = await fetch(window.devportalApi.root('/webhook-subscribers/' + encodeURIComponent(pendingDelWebhookId)), {
        method: 'DELETE',
        headers: { 'X-CSRF-Token': window.devportalApi.csrfToken() },
      });
      if (res.ok || res.status === 204) {
        await showAlert('Webhook deleted.', 'success');
        window.location.reload();
      } else {
        var err = await res.json().catch(function(){ return {}; });
        await showAlert('Delete failed: ' + (err.error || err.description || err.message || res.statusText), 'error');
      }
    } catch(e) { await showAlert('Error: ' + e.message, 'error'); }
    pendingDelWebhookId = null;
  });
}());
