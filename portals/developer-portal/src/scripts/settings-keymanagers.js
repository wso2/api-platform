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
/* eslint-disable no-undef */

(function () {
  var editKmId = null;

  function v(id) { var e=document.getElementById(id); return e?e.value.trim():''; }
  function sv(id,val) { var e=document.getElementById(id); if(e) e.value=val||''; }

  /* build id→key manager lookup from server-rendered data blob */
  var kmMap = {};
  (function() {
    try {
      var el = document.getElementById('cfg-keymanagers-data');
      if (el) {
        var list = JSON.parse(el.textContent || '[]');
        list.forEach(function(km) { kmMap[km.id] = km; });
      }
    } catch(e) {}
  }());

  /* ── open modal ── */
  function openKmModal(mode, data) {
    editKmId = mode === 'edit' ? data.id : null;
    document.getElementById('cfg-km-modal-title').textContent = mode === 'edit' ? 'Edit key manager' : 'Add key manager';
    document.getElementById('cfg-km-modal-save').textContent  = mode === 'edit' ? 'Save changes' : 'Add key manager';
    sv('km-display',        mode === 'edit' ? data.displayName    : '');
    sv('km-handle',         mode === 'edit' ? data.id             : '');
    sv('km-token-endpoint', mode === 'edit' ? data.tokenEndpoint  : '');
    document.getElementById('km-enabled').checked = mode === 'edit' ? !!data.enabled : true;
    document.getElementById('cfg-km-modal').style.display = 'flex';
    document.getElementById('km-display').focus();
  }
  function closeKmModal() { document.getElementById('cfg-km-modal').style.display = 'none'; editKmId = null; }

  /* ── auto-slug display → handle ── */
  document.getElementById('km-display').addEventListener('input', function() {
    if (editKmId) return;
    document.getElementById('km-handle').value = this.value.toLowerCase().trim().replace(/[^a-z0-9]+/g,'-').replace(/^-|-$/g,'');
  });

  /* ── save ── */
  document.getElementById('cfg-km-modal-save').addEventListener('click', async function() {
    var displayName   = v('km-display');
    var handle        = v('km-handle');
    var tokenEndpoint = v('km-token-endpoint');
    if (!displayName || !handle || !tokenEndpoint) { await showAlert('Display name, handle and token endpoint are required.', 'error'); return; }

    var parsedUrl;
    try { parsedUrl = new URL(tokenEndpoint); } catch (e) { parsedUrl = null; }
    if (!parsedUrl || (parsedUrl.protocol !== 'http:' && parsedUrl.protocol !== 'https:')) {
      await showAlert('Token endpoint must be a valid http:// or https:// URL.', 'error');
      return;
    }

    var body = {
      id: handle,
      displayName: displayName,
      tokenEndpoint: tokenEndpoint,
      enabled: document.getElementById('km-enabled').checked,
    };

    var url    = editKmId
      ? window.devportalApi.root('/key-managers/' + encodeURIComponent(editKmId))
      : window.devportalApi.root('/key-managers');
    var method = editKmId ? 'PUT' : 'POST';

    try {
      var res = await fetch(url, {
        method: method,
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.devportalApi.csrfToken() },
        body: JSON.stringify(body),
      });
      if (res.ok) {
        await showAlert(editKmId ? 'Key manager updated.' : 'Key manager created.', 'success');
        window.location.reload();
      } else {
        var err = await res.json().catch(function(){ return {}; });
        await showAlert('Failed: ' + (err.error || err.description || err.message || res.statusText), 'error');
      }
    } catch(e) { await showAlert('Error: ' + e.message, 'error'); }
  });

  document.getElementById('cfg-km-modal-close').addEventListener('click', closeKmModal);
  document.getElementById('cfg-km-modal-cancel').addEventListener('click', closeKmModal);
  document.getElementById('cfg-km-modal').addEventListener('click', function(e){ if(e.target===this) closeKmModal(); });

  document.getElementById('cfg-add-km-btn').addEventListener('click', function() { openKmModal('add'); });

  /* ── edit / delete via event delegation ── */
  var pendingDelKmId = null;
  document.addEventListener('click', function(e) {
    if (e.target.closest('.cfg-km-edit-btn')) {
      var btn = e.target.closest('.cfg-km-edit-btn');
      var data = kmMap[btn.dataset.id];
      if (data) openKmModal('edit', data);
      return;
    }
    if (e.target.closest('.cfg-km-delete-btn')) {
      var btn = e.target.closest('.cfg-km-delete-btn');
      pendingDelKmId = btn.dataset.id;
      document.getElementById('cfg-del-km-name-txt').textContent = btn.dataset.name;
      document.getElementById('cfg-delete-km-modal').style.display = 'flex';
      return;
    }
  });

  document.getElementById('cfg-del-km-cancel').addEventListener('click', function() {
    document.getElementById('cfg-delete-km-modal').style.display = 'none';
  });
  document.getElementById('cfg-delete-km-modal').addEventListener('click', function(e){ if(e.target===this) this.style.display='none'; });
  document.getElementById('cfg-del-km-confirm').addEventListener('click', async function() {
    if (!pendingDelKmId) return;
    document.getElementById('cfg-delete-km-modal').style.display = 'none';
    try {
      var res = await fetch(window.devportalApi.root('/key-managers/' + encodeURIComponent(pendingDelKmId)), {
        method: 'DELETE',
        headers: { 'X-CSRF-Token': window.devportalApi.csrfToken() },
      });
      if (res.ok || res.status === 204) {
        await showAlert('Key manager deleted.', 'success');
        window.location.reload();
      } else {
        var err = await res.json().catch(function(){ return {}; });
        await showAlert('Delete failed: ' + (err.error || err.description || err.message || res.statusText), 'error');
      }
    } catch(e) { await showAlert('Error: ' + e.message, 'error'); }
    pendingDelKmId = null;
  });
}());
