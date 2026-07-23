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
  var editLabelName = null;

  function esc(s) { return String(s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;'); }
  function v(id) { var e=document.getElementById(id); return e?e.value.trim():''; }

  /* ── open modal ── */
  function openLabelModal(mode, data) {
    editLabelName = mode === 'edit' ? data.id : null;
    document.getElementById('cfg-label-modal-title').textContent = mode === 'edit' ? 'Edit label' : 'Add label';
    document.getElementById('cfg-label-modal-save').textContent  = mode === 'edit' ? 'Save changes' : 'Add label';
    document.getElementById('lbl-display').value = mode === 'edit' ? data.displayName : '';
    document.getElementById('lbl-name').value    = mode === 'edit' ? data.id          : '';
    document.getElementById('cfg-label-modal').style.display = 'flex';
    document.getElementById('lbl-display').focus();
  }
  function closeLabelModal() { document.getElementById('cfg-label-modal').style.display = 'none'; editLabelName = null; }

  /* ── auto-slug display → name ── */
  document.getElementById('lbl-display').addEventListener('input', function() {
    if (editLabelName) return;
    document.getElementById('lbl-name').value = this.value.toLowerCase().trim().replace(/[^a-z0-9]+/g,'-').replace(/^-|-$/g,'');
  });

  /* ── save label ── */
  document.getElementById('cfg-label-modal-save').addEventListener('click', async function() {
    var displayName = v('lbl-display');
    var name        = v('lbl-name');
    if (!displayName || !name) { await showAlert('Display name and name are required.', 'error'); return; }

    try {
      var res;
      if (editLabelName && editLabelName !== name) {
        /* handle changed — handle is immutable, so delete old and create new */
        await fetch(window.devportalApi.root('/labels/'+encodeURIComponent(editLabelName)), {
          method: 'DELETE',
          headers: { 'X-CSRF-Token': window.devportalApi.csrfToken() },
        });
        res = await fetch(window.devportalApi.root('/labels'), {
          method: 'POST',
          headers: { 'Content-Type':'application/json', 'X-CSRF-Token': window.devportalApi.csrfToken() },
          body: JSON.stringify({ id: name, displayName: displayName }),
        });
      } else if (editLabelName) {
        res = await fetch(window.devportalApi.root('/labels/'+encodeURIComponent(name)), {
          method: 'PUT',
          headers: { 'Content-Type':'application/json', 'X-CSRF-Token': window.devportalApi.csrfToken() },
          body: JSON.stringify({ id: name, displayName: displayName }),
        });
      } else {
        res = await fetch(window.devportalApi.root('/labels'), {
          method: 'POST',
          headers: { 'Content-Type':'application/json', 'X-CSRF-Token': window.devportalApi.csrfToken() },
          body: JSON.stringify({ id: name, displayName: displayName }),
        });
      }
      if (res.ok) {
        await showAlert(editLabelName ? 'Label updated.' : 'Label created.', 'success');
        window.location.reload();
      } else {
        var err = await res.json().catch(function(){ return {}; });
        await showAlert('Failed: '+(err.description||err.message||res.statusText), 'error');
      }
    } catch(e) { await showAlert('Error: '+e.message, 'error'); }
  });

  document.getElementById('cfg-label-modal-close').addEventListener('click', closeLabelModal);
  document.getElementById('cfg-label-modal-cancel').addEventListener('click', closeLabelModal);
  document.getElementById('cfg-label-modal').addEventListener('click', function(e){ if(e.target===this) closeLabelModal(); });

  /* ── Add btn ── */
  document.getElementById('cfg-add-label-btn').addEventListener('click', function() { openLabelModal('add'); });

  /* ── Edit / Delete via event delegation ── */
  var pendingDelName = null;
  document.addEventListener('click', function(e) {
    if (e.target.closest('.cfg-label-edit-btn')) {
      var btn = e.target.closest('.cfg-label-edit-btn');
      openLabelModal('edit', { id: btn.dataset.name, displayName: btn.dataset.display });
      return;
    }
    if (e.target.closest('.cfg-label-delete-btn')) {
      btn = e.target.closest('.cfg-label-delete-btn');
      pendingDelName = btn.dataset.name;
      document.getElementById('cfg-del-label-name-txt').textContent = btn.dataset.display || btn.dataset.name;
      document.getElementById('cfg-delete-label-modal').style.display = 'flex';
      return;
    }
  });

  document.getElementById('cfg-del-label-cancel').addEventListener('click', function() {
    document.getElementById('cfg-delete-label-modal').style.display = 'none';
  });
  document.getElementById('cfg-delete-label-modal').addEventListener('click', function(e){ if(e.target===this) this.style.display='none'; });
  document.getElementById('cfg-del-label-confirm').addEventListener('click', async function() {
    if (!pendingDelName) return;
    document.getElementById('cfg-delete-label-modal').style.display = 'none';
    try {
      var res = await fetch(window.devportalApi.root('/labels/'+encodeURIComponent(pendingDelName)), {
        method: 'DELETE',
        headers: { 'X-CSRF-Token': window.devportalApi.csrfToken() },
      });
      if (res.ok || res.status===204) {
        await showAlert('Label deleted.', 'success');
        window.location.reload();
      } else {
        var err = await res.json().catch(function(){ return {}; });
        await showAlert('Delete failed: '+(err.description||err.message||res.statusText), 'error');
      }
    } catch(e) { await showAlert('Error: '+e.message, 'error'); }
    pendingDelName = null;
  });
}());
