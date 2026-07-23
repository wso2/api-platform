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
  var views = [];
  try { views = JSON.parse(document.getElementById('cfg-views-data').textContent) || []; } catch (e) { /* ignore */ }
  var editHandle = null;
  var modal = document.getElementById('cfg-view-modal');
  if (!modal) return;
  function g(id) { return document.getElementById(id); }

  // Labels are chosen via toggle chips (no cmd+click multi-select).
  var labelPicker = g('view-labels');
  if (labelPicker) {
    labelPicker.addEventListener('click', function (e) {
      var chip = e.target.closest('.cfg-label-toggle');
      if (!chip) return;
      var on = chip.getAttribute('aria-pressed') === 'true';
      chip.setAttribute('aria-pressed', on ? 'false' : 'true');
      chip.classList.toggle('selected', !on);
    });
  }
  function setSelectedLabels(labels) {
    if (!labelPicker) return;
    var set = {}; (labels || []).forEach(function (l) { set[l] = true; });
    labelPicker.querySelectorAll('.cfg-label-toggle').forEach(function (chip) {
      var on = !!set[chip.dataset.value];
      chip.setAttribute('aria-pressed', on ? 'true' : 'false');
      chip.classList.toggle('selected', on);
    });
  }
  function getSelectedLabels() {
    if (!labelPicker) return [];
    return Array.prototype.map.call(
      labelPicker.querySelectorAll('.cfg-label-toggle[aria-pressed="true"]'),
      function (chip) { return chip.dataset.value; }
    );
  }

  function openModal(mode, view) {
    editHandle = mode === 'edit' ? view.id : null;
    g('cfg-view-modal-title').textContent = mode === 'edit' ? 'Edit view' : 'Add view';
    g('cfg-view-modal-save').textContent  = mode === 'edit' ? 'Save changes' : 'Add view';
    g('view-handle').value    = view ? (view.id || '') : '';
    g('view-handle').readOnly = mode === 'edit';
    g('view-display').value   = view ? (view.displayName || '') : '';
    setSelectedLabels(view ? view.labels : []);
    modal.style.display = 'flex';
    g('view-handle').focus();
  }
  function closeModal() { modal.style.display = 'none'; editHandle = null; }

  var addBtn = g('cfg-add-view-btn');
  if (addBtn) addBtn.addEventListener('click', function () { openModal('add'); });
  g('cfg-view-modal-close').addEventListener('click', closeModal);
  g('cfg-view-modal-cancel').addEventListener('click', closeModal);
  modal.addEventListener('click', function (e) { if (e.target === modal) closeModal(); });

  /* ── auto-slug display → handle ── */
  var viewHandleRe = /^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$/;
  g('view-display').addEventListener('input', function () {
    if (editHandle) return;
    g('view-handle').value = this.value.toLowerCase().trim().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
  });

  g('cfg-view-modal-save').addEventListener('click', async function () {
    var handle  = g('view-handle').value.trim();
    var display = g('view-display').value.trim();
    var labels  = getSelectedLabels();
    if (!handle) { await showAlert('Handle is required.', 'error'); return; }
    if (!viewHandleRe.test(handle)) {
      await showAlert('Handle must be lowercase letters, numbers and hyphens only.', 'error');
      return;
    }
    try {
      var res;
      if (editHandle) {
        res = await fetch(window.devportalApi.root('/views/' + encodeURIComponent(editHandle)), {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.devportalApi.csrfToken() },
          credentials: 'same-origin', body: JSON.stringify({ displayName: display, labels: labels }),
        });
      } else {
        var body = { id: handle, labels: labels };
        if (display) body.displayName = display;
        res = await fetch(window.devportalApi.root('/views'), {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.devportalApi.csrfToken() },
          credentials: 'same-origin', body: JSON.stringify(body),
        });
      }
      if (res.ok) {
        await showAlert(editHandle ? 'View updated.' : 'View created.', 'success');
        window.location.reload();
      } else {
        var err = await res.json().catch(function () { return {}; });
        await showAlert('Failed: ' + (err.description || err.message || res.statusText), 'error');
      }
    } catch (e) { await showAlert('Error: ' + e.message, 'error'); }
  });

  var pendingDel = null;
  document.addEventListener('click', function (e) {
    var editBtn = e.target.closest('.cfg-view-edit-btn');
    if (editBtn) {
      var view = views.filter(function (vw) { return vw.id === editBtn.dataset.id; })[0];
      if (view) openModal('edit', view);
      return;
    }
    var delBtn = e.target.closest('.cfg-view-delete-btn');
    if (delBtn) {
      pendingDel = delBtn.dataset.id;
      g('cfg-del-view-name-txt').textContent = delBtn.dataset.name || delBtn.dataset.id;
      g('cfg-delete-view-modal').style.display = 'flex';
      return;
    }
  });
  g('cfg-del-view-cancel').addEventListener('click', function () { g('cfg-delete-view-modal').style.display = 'none'; });
  g('cfg-delete-view-modal').addEventListener('click', function (e) { if (e.target === this) this.style.display = 'none'; });
  g('cfg-del-view-confirm').addEventListener('click', async function () {
    if (!pendingDel) return;
    g('cfg-delete-view-modal').style.display = 'none';
    try {
      var res = await fetch(window.devportalApi.root('/views/' + encodeURIComponent(pendingDel)), {
        method: 'DELETE',
        headers: { 'X-CSRF-Token': window.devportalApi.csrfToken() }, credentials: 'same-origin',
      });
      if (res.ok || res.status === 204) { await showAlert('View deleted.', 'success'); window.location.reload(); }
      else { var err = await res.json().catch(function () { return {}; }); await showAlert('Delete failed: ' + (err.description || err.message || res.statusText), 'error'); }
    } catch (e) { await showAlert('Error: ' + e.message, 'error'); }
    pendingDel = null;
  });
}());
