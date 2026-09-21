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
    // Editing only. On create the API defaults enabled to true, so the add form
    // neither shows the switch nor sends the field.
    setHidden('km-enabled-field', mode !== 'edit');
    document.getElementById('km-enabled').checked = mode === 'edit' ? !!data.enabled : true;
    fillProvisioning(mode === 'edit' ? data.provisioning : null, mode === 'edit');
    // A key manager's type is fixed when it is created: changing it would leave
    // existing keys addressed by a driver that never issued them. Locked rather
    // than hidden, so an admin can still see which kind this one is.
    el('km-type').disabled = mode === 'edit';
    syncKmSave();
    document.getElementById('cfg-km-modal').style.display = 'flex';
    document.getElementById('km-display').focus();
  }
  function closeKmModal() { document.getElementById('cfg-km-modal').style.display = 'none'; editKmId = null; }

  /* ── key generation (provisioning) section ── */

  function el(id) { return document.getElementById(id); }
  function setHidden(id, hidden) { var e = el(id); if (e) e.hidden = hidden; }

  /* Only one credential block applies at a time, and the other one's values must
     not be submitted — an unused username would otherwise travel with a
     client_credentials payload and be rejected by the schema. */
  /*
   * A provision-type key manager registers nothing, so everything below the type
   * is meaningless for it: no endpoint to register at, no credential to present.
   * Hidden rather than disabled — a disabled form still reads as "fill this in
   * later", and there is no later.
   */
  var PROVISION_TYPE = 'provision';

  function syncKmType() {
    setHidden('km-dcr-only', el('km-type').value === PROVISION_TYPE);
    syncKmSave();
  }

  function syncAuthMethod() {
    var method = el('km-auth-method').value;
    setHidden('km-auth-cc', method !== 'client_credentials');
    setHidden('km-auth-basic', method !== 'basic');
    setHidden('km-auth-apikey', method !== 'api_key');
    syncKmSave();
  }

  /*
   * A stored secret is never sent back to the browser, so the form cannot show
   * it. When one is held the field becomes optional and says so: leaving it
   * blank keeps what is stored, which is the behaviour the API implements.
   *
   * The message sits in the field's own placeholder rather than a line beneath
   * it. It is live state, not a description of the field, and the placeholder is
   * visible exactly when it applies — while the box is empty, which is the case
   * it is talking about.
   */
  function markSecretOptional(reqId, inputId, held) {
    var req = el(reqId), input = el(inputId);
    if (req) req.hidden = held;
    if (input) {
      input.placeholder = held ? 'Leave blank to keep the stored secret' : '';
    }
  }

  function fillProvisioning(p, isEdit) {
    var on = !!p;
    // Left on the placeholder for a new key manager: the driver decides what this
    // key manager can actually do, so it is the admin's choice to make, not one to
    // inherit from whichever option happens to sort first.
    // No provisioning block on an existing key manager means it is provision
    // type — not "unconfigured". Only a brand new form is left on the placeholder.
    el('km-type').value = on && p.type ? p.type : (isEdit ? PROVISION_TYPE : '');
    sv('km-registration-endpoint', on ? p.registrationEndpoint : '');
    sv('km-authorize-endpoint', on ? (p.authorizeEndpoint || '') : '');
    el('km-auth-method').value = on ? (p.authMethod || 'client_credentials') : 'client_credentials';
    sv('km-client-id', on ? (p.clientId || '') : '');
    sv('km-username', on ? (p.username || '') : '');
    sv('km-header-name', on ? (p.headerName || '') : '');
    sv('km-scheme', on ? (p.scheme || '') : '');
    sv('km-scopes', on && p.scopes ? p.scopes.join(' ') : '');
    sv('km-resource', on ? (p.resource || '') : '');
    // Cleared on every open: a secret typed for one key manager must never be
    // carried into the next modal the admin opens.
    sv('km-client-secret', '');
    sv('km-password', '');
    sv('km-api-key', '');
    markSecretOptional('km-client-secret-req', 'km-client-secret', on && !!p.hasClientSecret);
    markSecretOptional('km-password-req', 'km-password', on && !!p.hasPassword);
    markSecretOptional('km-api-key-req', 'km-api-key', on && !!p.hasApiKey);
    syncKmType();
    syncAuthMethod();
  }

  /* Every field the section owns. Used to tell "the admin left this alone" from
     "the admin filled it in", now that no switch says so explicitly. */
  var DCR_FIELDS = [
    'km-type', 'km-registration-endpoint', 'km-authorize-endpoint',
    'km-client-id', 'km-client-secret', 'km-scopes', 'km-resource',
    'km-username', 'km-password',
    'km-header-name', 'km-scheme', 'km-api-key',
  ];

  /*
   * Returns { provisioning } or { error } or {} when the section was left empty.
   *
   * With the toggle gone, intent is read from the form: any value anywhere in the
   * section means "configure this", so a half-filled section is an error rather
   * than a silent drop — the failure the toggle used to prevent. A wholly empty
   * one means "token proxying only", which is still a valid key manager; it just
   * reports canGenerateKeys: false.
   *
   * Note this cannot express "remove the provisioning I saved earlier": clearing
   * the fields sends no provisioning, and the API leaves the stored row alone on
   * omission. That was equally true of the toggle's off position.
   */
  function collectProvisioning() {
    // Provision type is the explicit way to say "this key manager registers
    // nothing". It carries no provisioning block at all, which is exactly how a
    // key manager created before DCR support is already stored.
    if (el('km-type').value === PROVISION_TYPE) return {};
    var touched = DCR_FIELDS.some(function(id) { return v(id) !== ''; });
    if (!touched) return {};
    var method = el('km-auth-method').value;
    var auth = { method: method };
    if (method === 'client_credentials') {
      if (!v('km-client-id')) return { error: 'Client ID is required to let the portal create applications.' };
      auth.clientId = v('km-client-id');
      if (v('km-client-secret')) auth.clientSecret = v('km-client-secret');
      var scopes = v('km-scopes').split(/\s+/).filter(Boolean);
      if (scopes.length) auth.scopes = scopes;
      if (v('km-resource')) auth.resource = v('km-resource');
    } else if (method === 'api_key') {
      // Header name and scheme are both optional: blank means Authorization,
      // and blank scheme means the key is sent on its own.
      if (v('km-header-name')) auth.headerName = v('km-header-name');
      if (v('km-scheme')) auth.scheme = v('km-scheme');
      if (v('km-api-key')) auth.apiKey = v('km-api-key');
    } else {
      if (!v('km-username')) return { error: 'Username is required to let the portal create applications.' };
      auth.username = v('km-username');
      if (v('km-password')) auth.password = v('km-password');
    }
    if (!el('km-type').value) {
      return { error: 'Choose a key manager type to let the portal create applications.' };
    }
    if (!v('km-registration-endpoint')) {
      return { error: 'Registration endpoint is required to let the portal create applications.' };
    }
    var provisioning = {
      type: el('km-type').value,
      registrationEndpoint: v('km-registration-endpoint'),
      auth: auth,
    };
    if (v('km-authorize-endpoint')) provisioning.authorizeEndpoint = v('km-authorize-endpoint');
    return { provisioning: provisioning };
  }

  el('km-auth-method').addEventListener('change', syncAuthMethod);
  el('km-type').addEventListener('change', syncKmType);

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

    var provisioning = collectProvisioning();
    if (provisioning.error) { await showAlert(provisioning.error, 'error'); return; }

    var body = {
      displayName: displayName,
      tokenEndpoint: tokenEndpoint,
    };
    // Sent only when editing — the one case where the admin was shown the switch
    // and could have changed it. Omitted on create, where the API defaults to true.
    if (editKmId) body.enabled = document.getElementById('km-enabled').checked;
    // Omitted entirely when the section is off. Sending an empty object would be
    // a payload the schema rejects, and sending nothing on edit is what leaves an
    // existing configuration untouched.
    if (provisioning.provisioning) body.provisioning = provisioning.provisioning;

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
