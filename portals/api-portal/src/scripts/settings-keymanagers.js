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
  /* Headers for every state-changing call from this page. The CSRF token is read
     at call time rather than captured once, so a token refreshed since page load
     is the one that travels. */
  function mutationHeaders() {
    return { 'Content-Type': 'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() };
  }

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
    fillProvisioning(mode === 'edit' ? data.provisioning : null, mode === 'edit');
    resetDiscovery(mode === 'edit');
    /*
     * Fixed at creation, so editing shows it disabled rather than hiding it — an
     * admin should still be able to see which environment they are looking at.
     * The value is still submitted on edit; the API accepts it unchanged and
     * answers 409 only to an actual change.
     */
    var keyType = mode === 'edit' ? (data.keyType || 'PRODUCTION') : 'PRODUCTION';
    el('km-keytype-production').checked = keyType !== 'SANDBOX';
    el('km-keytype-sandbox').checked = keyType === 'SANDBOX';
    ['km-keytype-production', 'km-keytype-sandbox'].forEach(function (id) {
      el(id).disabled = mode === 'edit';
    });
    setHidden('km-keytype-hint', mode !== 'edit');
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
   * A key manager that imports registers nothing, so everything the registration
   * section asks for is meaningless for it: no endpoint to register at, no
   * credential to present. Hidden rather than disabled — a disabled form still
   * reads as "fill this in later", and there is no later.
   *
   * There is no constant for the driver name here any more. The mode decides the
   * payload, and the absence of a provisioning block is what the API reads as
   * "this one imports" — so the browser never has to name the driver.
   */
  /*
   * What a fresh Add form starts on. Every option in the dropdown registers
   * applications now, so a default no longer risks picking a driver that cannot —
   * the guard the old empty placeholder existed for. Falls back to the first
   * option when a build does not ship this driver, so the form is never left
   * submitting an empty type.
   */
  var DEFAULT_TYPE = 'thunderid';

  function isImportMode() {
    var r = el('km-mode-import');
    return !!(r && r.checked);
  }

  /*
   * The mode decides the shape of the form, and the type only exists inside one
   * branch of it. Both are driven from here so there is one place that knows the
   * two are related.
   */
  function syncKmMode() {
    var importing = isImportMode();
    // editKmId is assigned before fillProvisioning runs, so this is already set
    // by the time the first sync happens.
    var editing = editKmId !== null;
    // Settled at creation, so editing states it rather than asking again.
    setHidden('km-mode-field', editing);
    // Only where the answer would otherwise show as the driver name "Provision"
    // in a field labelled "Key manager type".
    setHidden('km-creation-field', !(editing && importing));
    setHidden('km-type-field', importing);
    setHidden('km-dcr-only', importing);
    syncKmSave();
  }

  /*
   * Rebuild the grant-type checkboxes for whichever driver is selected.
   *
   * The options come off the selected <option>'s data-grants, which the server
   * filled from that driver's own metadata — so this can never offer a grant the
   * key manager does not support. `keep` re-ticks what was already chosen, which
   * is what makes switching type and switching back non-destructive.
   *
   * A driver that declares no grant types (provision) hides the whole group
   * rather than showing an empty one.
   */
  function renderGrantTypes(keep) {
    var host = el('km-grants');
    var sel = el('km-type');
    if (!host || !sel) return;
    var opt = sel.options[sel.selectedIndex];
    var grants = [];
    try { grants = JSON.parse((opt && opt.getAttribute('data-grants')) || '[]') || []; } catch (e) { grants = []; }
    var chosen = {};
    (keep || []).forEach(function (g) { chosen[g] = true; });

    host.textContent = '';
    setHidden('km-grants-field', !grants.length);
    grants.forEach(function (g) {
      var label = document.createElement('label');
      label.className = 'cfg-check';
      var box = document.createElement('input');
      box.type = 'checkbox';
      box.className = 'cfg-km-grant';
      box.value = g.value;
      box.checked = !!chosen[g.value];
      var text = document.createElement('span');
      text.textContent = g.label || g.value;
      label.appendChild(box);
      label.appendChild(text);
      host.appendChild(label);
    });
  }

  function selectedGrantTypes() {
    return Array.prototype.slice.call(document.querySelectorAll('.cfg-km-grant:checked'))
      .map(function (b) { return b.value; });
  }

  function selectDefaultType() {
    var sel = el('km-type');
    if (!sel) return;
    sel.value = DEFAULT_TYPE;
    // Not shipped in this build — take whatever the registry did register rather
    // than leaving the value empty.
    if (!sel.value && sel.options.length) sel.value = sel.options[0].value;
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
    /*
     * The mode is the fact; the type is a detail inside one branch of it. A stored
     * key manager with no provisioning block imports — that is what the absence
     * means, not "unconfigured".
     */
    var importing = isEdit ? !on : false;
    el('km-mode-import').checked = importing;
    el('km-mode-register').checked = !importing;
    if (on && p.type) {
        el('km-type').value = p.type;
    } else if (!isEdit) {
        selectDefaultType();
    }
    renderGrantTypes(on && Array.isArray(p.supportedGrantTypes) ? p.supportedGrantTypes : []);
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
    syncKmMode();
    syncAuthMethod();
  }

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
    // "They already exist" is the explicit way to say "this key manager registers
    // nothing". It sends no provisioning block at all, which is exactly how a key
    // manager created before DCR support is already stored — so the payload is
    // unchanged from when this was a type in the dropdown.
    if (isImportMode()) return {};
    /*
     * Register mode with every field blank is a mistake, not a request to import.
     * Omitting the block here would save a key manager that registers nothing —
     * the opposite of what the admin selected — and silently, because the API
     * reads an absent block as "this one imports".
     *
     * This was near-unreachable before: the type dropdown opened on a placeholder,
     * so a half-filled form failed on the missing type first. Now that the form
     * opens on Register, an admin who fills in only the name and token endpoint
     * lands here directly. Fall through to the per-method checks below, which name
     * whichever credential field is missing.
     */
    if (!v('km-registration-endpoint')) {
      return { error: 'Registration endpoint is required when the portal creates the applications. '
        + 'Choose "They already exist" if the applications are created in the identity server instead.' };
    }
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
    // Sent only when it restricts something. An empty array would be indistinguishable
    // from "no grant type permitted" on the wire, and the API reads absent as "no
    // restriction" — which is what no boxes ticked means.
    var grants = selectedGrantTypes();
    if (grants.length) provisioning.supportedGrantTypes = grants;
    return { provisioning: provisioning };
  }

  // Switching driver changes which grants exist, so the group is rebuilt — keeping
  // any still-valid choice rather than silently clearing the admin's selection.
  el('km-type').addEventListener('change', function () { renderGrantTypes(selectedGrantTypes()); });

  el('km-auth-method').addEventListener('change', syncAuthMethod);
  el('km-mode-register').addEventListener('change', syncKmMode);
  el('km-mode-import').addEventListener('change', syncKmMode);

  /* Disable save until Name and Token endpoint are both filled. */
  var syncKmSave = bindFormValidity(document.getElementById('cfg-km-modal-save'), ['km-display', 'km-token-endpoint'], function() {
    return v('km-display') !== '' && v('km-token-endpoint') !== '';
  });

  /* ── discovery ──────────────────────────────────────────────
   *
   * Fills the endpoint fields from the identity server's own metadata document,
   * so three URLs need not be transcribed out of another browser tab.
   *
   * The portal does the fetch, not this page, for three reasons a browser cannot
   * work around. Some identity servers send no Access-Control-Allow-Origin on
   * their discovery document (Asgardeo does not; Keycloak does), so a fetch from
   * here would work for one key manager type and fail for another. A portal
   * served over https cannot fetch an http:// discovery URL at all — mixed
   * content is blocked outright, and a key manager on plain http is a supported
   * configuration. And an identity server reachable from the portal but not from
   * the admin's own network would be unreachable from here.
   *
   * Going through the server also means the document is read under exactly the
   * address policy that will judge these endpoints when the form is saved, so
   * discovery cannot fill the form with values the save would then reject.
   *
   * Nothing about the discovery URL is submitted. It is a way of typing the
   * endpoints, and the key manager records only the endpoints themselves.
   */

  // Captured once, as the markup wrote it: this line doubles as the control's
  // result, so resetting it has to put the original explanation back.
  var discoveryHint = el('km-discovery-status') ? el('km-discovery-status').textContent : '';

  function setDiscoveryStatus(text, state) {
    var p = el('km-discovery-status');
    if (!p) return;
    p.textContent = text;
    p.classList.remove('is-error', 'is-success');
    if (state) p.classList.add(state);
  }

  /* Edit hides it: the endpoints are settled there, and re-reading them would
     overwrite a value an admin may have corrected by hand since. */
  function resetDiscovery(isEdit) {
    setHidden('km-discovery-field', isEdit);
    sv('km-discovery-url', '');
    setDiscoveryStatus(discoveryHint, null);
  }

  async function fetchDiscovery(btn) {
    var url = v('km-discovery-url');
    if (!url) { setDiscoveryStatus('Enter the discovery document URL first.', 'is-error'); return; }

    // The button carries an icon, not a word, so there is nothing to swap for
    // "Fetching…" — the spin and the status line below are the busy signal.
    btn.disabled = true;
    btn.classList.add('is-busy');
    setDiscoveryStatus('Reading the discovery document…', null);
    try {
      var res = await fetch(window.apiPortalApi.root('/key-managers/discovery'), {
        method: 'POST',
        headers: mutationHeaders(),
        body: JSON.stringify({ url: url }),
      });
      var data = await res.json().catch(function () { return {}; });
      if (!res.ok) {
        setDiscoveryStatus(
          data.message || data.description || data.error
            || 'The discovery document could not be read from that URL.',
          'is-error');
        return;
      }
      /*
       * Only what the document declared is written. An endpoint it omits leaves
       * whatever is in the field alone rather than blanking it — otherwise a
       * second fetch against a thinner document would quietly erase a correct
       * value the admin had already typed.
       */
      var filled = [];
      if (data.tokenEndpoint) { sv('km-token-endpoint', data.tokenEndpoint); filled.push('token'); }
      if (data.authorizeEndpoint) { sv('km-authorize-endpoint', data.authorizeEndpoint); filled.push('authorization'); }
      if (data.registrationEndpoint) { sv('km-registration-endpoint', data.registrationEndpoint); filled.push('registration'); }
      // The token endpoint is one of the two fields Save waits on, and setting a
      // value from script fires no input event — so the button is re-evaluated here.
      syncKmSave();
      setDiscoveryStatus(
        'Filled in the ' + filled.join(', ')
          + (filled.length === 1 ? ' endpoint. Check it' : ' endpoints. Check them')
          + ' before adding the key manager.',
        'is-success');
    } catch (e) {
      setDiscoveryStatus('The discovery document could not be read from that URL.', 'is-error');
    } finally {
      btn.disabled = false;
      btn.classList.remove('is-busy');
    }
  }

  el('km-discovery-fetch').addEventListener('click', function () { fetchDiscovery(this); });

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

    var checkedKeyType = document.querySelector('input[name="km-keytype"]:checked');
    var body = {
      displayName: displayName,
      tokenEndpoint: tokenEndpoint,
      // Sent in both modes. On edit the control is disabled and carries the stored
      // value, which the API accepts as an unchanged round-trip; only a different
      // value is refused.
      keyType: checkedKeyType ? checkedKeyType.value : 'PRODUCTION',
    };
    // Sent only when editing — the one case where the admin was shown the switch
    // and could have changed it. Omitted on create, where the API defaults to true.
    // `enabled` is deliberately absent: the Status control in the list owns it, and
    // sending it from here would let a stale modal value overwrite a toggle made
    // since the form was opened.
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
          headers: mutationHeaders(),
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
  /*
   * Flip one key manager's enabled flag from the list.
   *
   * Driven by the switch's own `change`, so `checked` already holds the state
   * being asked for and the browser handles the interaction (click, Space, a tap
   * on the label). The control is then re-rendered from what the server
   * confirmed rather than from the DOM — which is what puts a refused change back
   * where it was, instead of leaving the page claiming something that did not
   * happen.
   */
  async function toggleKmStatus(input) {
    if (input.disabled || input.dataset.busy === '1') return;
    var id = input.dataset.id;
    var previous = kmMap[id] ? !!kmMap[id].enabled : !input.checked;
    var wanted = !!input.checked;

    input.dataset.busy = '1';
    try {
      var resp = await fetch(window.apiPortalApi.root('/key-managers/' + encodeURIComponent(id)), {
        method: 'PUT', headers: mutationHeaders(),
        body: JSON.stringify({ enabled: wanted }),
      });
      if (!resp.ok) {
        var msg = 'Could not change the status of this key manager.';
        try { var d = await resp.json(); if (d && d.message) msg = d.message; } catch (err) { /* not JSON */ }
        await showAlert(msg, 'error');
        return;
      }
      var saved = await resp.json();
      var on = !!saved.enabled;
      if (kmMap[id]) kmMap[id].enabled = on;
      // The switch carries no visible text, so its tooltip is what states the
      // state in words — it has to follow the change like the label used to.
      input.title = on ? 'Enabled' : 'Disabled';
    } catch (err) {
      await showAlert('Could not reach the portal to change the status.', 'error');
    } finally {
      input.dataset.busy = '';
      // Whatever happened, the control shows what the server last confirmed —
      // on a failure that is the state before the switch was touched.
      input.checked = kmMap[id] ? !!kmMap[id].enabled : previous;
    }
  }

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

  // `change`, not the delegated click above: a switch is also toggled by Space
  // and by clicking its label, neither of which is a click on the input itself.
  document.addEventListener('change', function(e) {
    var sw = e.target.closest && e.target.closest('.cfg-km-status-switch');
    if (sw) toggleKmStatus(sw);
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
