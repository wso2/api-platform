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
  // Local fallback for the shared bindFormValidity (defined in alert.js, loaded first
  // on the settings page): keeps save/delete working even if it were ever unavailable,
  // only skipping the disable-until-valid behaviour instead of throwing at init.
  var bindFormValidity = window.bindFormValidity || function () { return function () {}; };

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
    sv('km-token-endpoint', mode === 'edit' ? data.tokenEndpoint  : '');
    document.getElementById('km-enabled').checked = mode === 'edit' ? !!data.enabled : true;
    syncKmSave();
    document.getElementById('cfg-km-modal').style.display = 'flex';
    document.getElementById('km-display').focus();
  }
  function closeKmModal() { document.getElementById('cfg-km-modal').style.display = 'none'; editKmId = null; }

  /* Disable save until Name and Token endpoint are both filled. */
  var syncKmSave = bindFormValidity(document.getElementById('cfg-km-modal-save'), ['km-display', 'km-token-endpoint'], function() {
    return v('km-display') !== '' && v('km-token-endpoint') !== '';
  });

  /* ── save ── */
  document.getElementById('cfg-km-modal-save').addEventListener('click', async function() {
    var displayName   = v('km-display');
    var tokenEndpoint = v('km-token-endpoint');
    if (!displayName || !tokenEndpoint) { await showAlert('Name and token endpoint are required.', 'error'); return; }

    var parsedUrl;
    try { parsedUrl = new URL(tokenEndpoint); } catch (e) { parsedUrl = null; }
    if (!parsedUrl || (parsedUrl.protocol !== 'http:' && parsedUrl.protocol !== 'https:')) {
      await showAlert('Token endpoint must be a valid http:// or https:// URL.', 'error');
      return;
    }

    var body = {
      displayName: displayName,
      tokenEndpoint: tokenEndpoint,
      enabled: document.getElementById('km-enabled').checked,
    };

    var url    = editKmId
      ? window.apiPortalApi.root('/key-managers/' + encodeURIComponent(editKmId))
      : window.apiPortalApi.root('/key-managers');
    var method = editKmId ? 'PUT' : 'POST';

    await withButtonBusy(this, editKmId ? 'Saving…' : 'Adding…', async function() {
      try {
        var res = await fetch(url, {
          method: method,
          headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
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
      btn = e.target.closest('.cfg-km-delete-btn');
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
  document.getElementById('cfg-del-km-confirm').addEventListener('click', function() {
    if (!pendingDelKmId) return;
    withButtonBusy(this, 'Deleting…', async function() {
      try {
        var res = await fetch(window.apiPortalApi.root('/key-managers/' + encodeURIComponent(pendingDelKmId)), {
          method: 'DELETE',
          headers: { 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
        });
        if (res.ok || res.status === 204) {
          await showAlert('Key manager deleted.', 'success');
          window.location.reload();
          return;
        }
        var err = await res.json().catch(function(){ return {}; });
        await showAlert('Delete failed: ' + (err.error || err.description || err.message || res.statusText), 'error');
      } catch(e) { await showAlert('Error: ' + e.message, 'error'); }
      document.getElementById('cfg-delete-km-modal').style.display = 'none';
      pendingDelKmId = null;
    });
  });
}());
