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
  // Same fallback as settings-plans.js: alert.js normally defines it.
  var bindFormValidity = window.bindFormValidity || function () { return function () {}; };

  var editMeterId = null;
  // Whether the open meter is one that plans price, which can't change.
  var lockedMeter = false;

  function meterType() { return document.getElementById('mtr-type').value; }

  function v(id) { var e = document.getElementById(id); return e ? e.value.trim() : ''; }

  // The event keys a new meter starts with, from our gateway's events: the number a Tokens meter adds
  // up, and the field each filter reads. The administrator can change them.
  var registry = (function () {
    try { return JSON.parse(document.getElementById('cfg-meter-modal').dataset.registry || '{}') || {}; } catch (e) { return {}; }
  }());
  var TOKEN_EVENT_KEY = registry.tokenEventKey || '';
  var FILTER_FIELDS = registry.filterFields || [];
  var MAX_FILTER_FIELDS = registry.maxFilterFields || FILTER_FIELDS.length;
  // Our gateway's token keys, suggested for a Token field's key, read as "Prompt tokens, metadata…".
  var TOKEN_KEY_CHOICES = (registry.tokenEventKeys || []).map(function (k) { return { value: k.key, label: k.label }; });

  function defaultKey(field) {
    return (FILTER_FIELDS.filter(function (f) { return f.value === field; })[0] || {}).eventKey || '';
  }

  /* The Token key: a text box whose list offers our gateway's token keys (the shared
     suggestion combo). Choosing one fills in its key; any typed key is kept. */
  var tokenKeyPicker = window.cfgSuggestCombo.attach(document.getElementById('mtr-token-key-combo'));

  /* A Tokens meter adds up the number at its Token key, shown under its type; a Requests meter
     counts 1 each and has no such key. */
  function syncMeterType() {
    tokenKeyPicker.close(false);
    document.getElementById('mtr-token-row').style.display = meterType() === 'TOKENS' ? '' : 'none';
    syncFilterFields();
  }
  // Switching to Tokens fills in the default Token key.
  document.getElementById('mtr-type').addEventListener('change', function () {
    document.getElementById('mtr-type-key').value = TOKEN_EVENT_KEY;
    syncMeterType();
    tokenKeyPicker.showStart();
  });

  /* ── filters: the fields the meter's prices may filter on, with the event key each reads: each
     fixed field once, and Token once per key ── */
  function filterRows() {
    return Array.prototype.slice.call(document.querySelectorAll('#mtr-filters-list .cfg-filter-row'));
  }

  /* Each row offers the fixed fields the other rows leave, and Token always; Add filter is hidden
     once the meter has as many fields as it may. */
  function syncFilterFields() {
    var rows = filterRows();
    rows.forEach(function (row) {
      var used = rows.filter(function (other) { return other !== row; })
        .map(function (other) { return other.querySelector('.mtr-filter-field').value; });
      Array.prototype.forEach.call(row.querySelector('.mtr-filter-field').options, function (o) {
        o.hidden = o.disabled = o.value !== 'TOKEN' && used.indexOf(o.value) !== -1;
      });
    });
    document.getElementById('mtr-add-filter-btn').style.display = rows.length >= MAX_FILTER_FIELDS ? 'none' : '';
    document.getElementById('mtr-filters-head').style.display = rows.length ? '' : 'none';
  }

  /* A filter row: the field and the event key it reads. A fixed field's key starts as its default (or
     the saved key) and goes back to the new field's default when the field changes. A Token field has
     no default: its key is typed or picked from our gateway's token keys, like the Token key. */
  function makeFilterRow(field, eventKey) {
    var row = document.createElement('div');
    row.className = 'cfg-filter-row cfg-key-grid';
    var select = document.createElement('select');
    select.className = 'cfg-form-input mtr-filter-field';
    select.setAttribute('aria-label', 'Property');
    var used = filterRows().map(function (r) { return r.querySelector('.mtr-filter-field').value; });
    var chosen = field || (FILTER_FIELDS.filter(function (f) { return f.value !== 'TOKEN' && used.indexOf(f.value) === -1; })[0]
      || { value: 'TOKEN' }).value;
    FILTER_FIELDS.forEach(function (f) {
      var o = document.createElement('option');
      o.value = f.value; o.textContent = f.label; o.selected = f.value === chosen;
      select.appendChild(o);
    });
    var remove = document.createElement('button');
    remove.type = 'button'; remove.className = 'cfg-icon-btn cfg-icon-btn--danger';
    remove.innerHTML = '<i class="bi bi-x-lg" aria-hidden="true"></i>';
    // The key box, and what holds it in the row: the box, or for a Token field its suggestion combo.
    var holder = null;

    function makeKey(value) {
      var key = document.createElement('input');
      key.type = 'text'; key.maxLength = 255; key.autocomplete = 'off'; key.spellcheck = false;
      key.className = 'cfg-form-input cfg-form-input--mono cfg-key-input mtr-filter-key';
      key.setAttribute('aria-label', 'Key');
      if (select.value === 'TOKEN') {
        key.placeholder = 'Choose or type a key';
        key.setAttribute('aria-required', 'true');
        key.value = value || '';
      } else {
        key.placeholder = defaultKey(select.value);
        key.value = value || defaultKey(select.value);
      }
      var shown = key;
      if (select.value === 'TOKEN') {
        shown = window.cfgSuggestCombo.build(key, TOKEN_KEY_CHOICES, 'Token keys');
        window.cfgSuggestCombo.attach(shown);
      }
      if (holder) holder.replaceWith(shown); else row.insertBefore(shown, remove);
      holder = shown;
      // A key filled in shows its start: "metadata.aiMetadata…", not its end.
      key.scrollLeft = 0;
    }
    // The remove button is named after the row's field: "Remove API type".
    function nameRemove() {
      var name = 'Remove ' + (select.selectedOptions[0] ? select.selectedOptions[0].textContent : '');
      remove.title = name;
      remove.setAttribute('aria-label', name);
    }
    row.appendChild(select); row.appendChild(remove);
    makeKey(eventKey);
    nameRemove();
    select.addEventListener('change', function () { makeKey(null); nameRemove(); syncFilterFields(); });
    remove.addEventListener('click', function () { row.remove(); syncFilterFields(); });
    return row;
  }

  document.getElementById('mtr-add-filter-btn').addEventListener('click', function () {
    var row = makeFilterRow(null);
    document.getElementById('mtr-filters-list').appendChild(row);
    syncFilterFields();
    row.querySelector('.mtr-filter-field').focus();
  });

  /* ── open / close modal ── */
  function openMeterModal(meter) {
    editMeterId = meter ? meter.id : null;
    // A meter that a plan prices can't change: its fields are read-only and Save is off, saying why.
    var locked = !!(meter && meter.planCount > 0);
    lockedMeter = locked;
    document.getElementById('mtr-fields').disabled = locked;
    document.getElementById('mtr-locked-note').hidden = !locked;
    document.getElementById('cfg-meter-modal-title').textContent = meter ? 'Edit meter' : 'Add Meter';
    document.getElementById('cfg-meter-modal-save').textContent = meter ? 'Save changes' : 'Add Meter';
    document.getElementById('mtr-display').value = meter ? (meter.displayName || '') : '';
    document.getElementById('mtr-desc').value = meter ? (meter.description || '') : '';
    document.getElementById('mtr-type').value = meter ? meter.type : 'REQUEST_COUNT';
    document.getElementById('mtr-type-key').value = meter && meter.typeKey ? meter.typeKey : TOKEN_EVENT_KEY;
    var list = document.getElementById('mtr-filters-list');
    list.innerHTML = '';
    (meter && meter.filterFields ? meter.filterFields : []).forEach(function (f) { list.appendChild(makeFilterRow(f.field, f.eventKey)); });
    syncMeterType();

    syncMeterSave();
    document.getElementById('cfg-meter-modal').style.display = 'flex';
    // Once shown, every key reads from its start.
    tokenKeyPicker.showStart();
    filterRows().forEach(function (row) { row.querySelector('.mtr-filter-key').scrollLeft = 0; });
    document.getElementById(locked ? 'cfg-meter-modal-cancel' : 'mtr-display').focus();
  }
  function closeMeterModal() {
    tokenKeyPicker.close(false);
    document.getElementById('cfg-meter-modal').style.display = 'none';
    editMeterId = null;
  }

  /* Disable save until the meter Name is filled. */
  var syncMeterSave = bindFormValidity(document.getElementById('cfg-meter-modal-save'), ['mtr-display'], function () {
    return !lockedMeter && document.getElementById('mtr-display').value.trim() !== '';
  });

  /* ── save ── */
  document.getElementById('cfg-meter-modal-save').addEventListener('click', async function () {
    var displayName = v('mtr-display');
    if (!displayName) { await showAlert('Name is required.', 'error'); return; }
    var typeKey = meterType() === 'TOKENS' ? v('mtr-type-key') : undefined;
    if (typeKey === '') { await showAlert('Enter an event key.', 'error'); return; }

    var body = {
      displayName: displayName,
      description: v('mtr-desc') || undefined,
      type: meterType(),
      typeKey: typeKey,
      // A filter whose event key is left empty reads the field's default.
      filterFields: filterRows().map(function (row) {
        return { field: row.querySelector('.mtr-filter-field').value, eventKey: row.querySelector('.mtr-filter-key').value.trim() || undefined };
      }),
    };
    // No `id` on create: the server derives it from the name. On edit the id stays.
    var url = editMeterId
      ? window.apiPortalApi.root('/meters/' + encodeURIComponent(editMeterId))
      : window.apiPortalApi.root('/meters');

    await withButtonBusy(this, editMeterId ? 'Saving…' : 'Adding…', async function () {
      try {
        var res = await fetch(url, {
          method: editMeterId ? 'PUT' : 'POST',
          headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
          body: JSON.stringify(body),
        });
        if (res.ok) {
          await showAlert(editMeterId ? 'Meter updated.' : 'Meter created.', 'success');
          window.location.reload();
        } else {
          var err = await res.json().catch(function () { return {}; });
          await showAlert('Meter could not be saved: ' + (err.message || err.description || res.statusText), 'error');
        }
      } catch (e) { await showAlert('The portal could not be reached. Check your connection and try again.', 'error'); }
    });
  });

  document.getElementById('cfg-meter-modal-close').addEventListener('click', closeMeterModal);
  document.getElementById('cfg-meter-modal-cancel').addEventListener('click', closeMeterModal);
  document.getElementById('cfg-meter-modal').addEventListener('click', function (e) { if (e.target === this) closeMeterModal(); });
  document.getElementById('cfg-add-meter-btn').addEventListener('click', function () { openMeterModal(null); });

  /* ── edit / delete delegation ── */
  var pendingDelMeterId = null;
  document.addEventListener('click', async function (e) {
    var btn = e.target.closest('.cfg-meter-edit-btn');
    if (btn) {
      var meter = null;
      try { meter = JSON.parse(btn.dataset.meter); } catch (err) { meter = null; }
      if (meter) openMeterModal(meter);
      return;
    }
    btn = e.target.closest('.cfg-meter-delete-btn');
    if (btn) {
      // A meter that plans price can't be deleted: the dialog says it is in use, and Delete is disabled.
      var plans = Number(btn.dataset.plans) || 0;
      var blocked = plans > 0;
      var name = btn.dataset.display || btn.dataset.id;
      document.getElementById('cfg-del-meter-ask').style.display = blocked ? 'none' : '';
      document.getElementById('cfg-del-meter-blocked').style.display = blocked ? '' : 'none';
      document.getElementById('cfg-del-meter-confirm').disabled = blocked;
      if (blocked) {
        pendingDelMeterId = null;
        var strong = document.createElement('strong');
        strong.textContent = name;
        document.getElementById('cfg-del-meter-blocked')
          .replaceChildren(strong, document.createTextNode(' is in use and can\'t be deleted.'));
      } else {
        pendingDelMeterId = btn.dataset.id;
        document.getElementById('cfg-del-meter-name-txt').textContent = name;
      }
      document.getElementById('cfg-delete-meter-modal').style.display = 'flex';
    }
  });

  document.getElementById('cfg-del-meter-cancel').addEventListener('click', function () {
    document.getElementById('cfg-delete-meter-modal').style.display = 'none';
  });
  document.getElementById('cfg-delete-meter-modal').addEventListener('click', function (e) { if (e.target === this) this.style.display = 'none'; });
  document.getElementById('cfg-del-meter-confirm').addEventListener('click', function () {
    if (!pendingDelMeterId) return;
    withButtonBusy(this, 'Deleting…', async function () {
      try {
        var res = await fetch(window.apiPortalApi.root('/meters/' + encodeURIComponent(pendingDelMeterId)), {
          method: 'DELETE',
          headers: { 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
        });
        if (res.ok || res.status === 204) {
          await showAlert('Meter deleted.', 'success');
          window.location.reload();
          return;
        }
        var err = await res.json().catch(function () { return {}; });
        await showAlert('Meter could not be deleted: ' + (err.message || err.description || res.statusText), 'error');
      } catch (e) { await showAlert('The portal could not be reached. Check your connection and try again.', 'error'); }
      document.getElementById('cfg-delete-meter-modal').style.display = 'none';
      pendingDelMeterId = null;
    });
  });
}());
