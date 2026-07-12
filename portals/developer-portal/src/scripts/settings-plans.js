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
  var editPlanId = null;

  var LIMIT_TYPES = ['REQUEST_COUNT','EVENT_COUNT','BANDWIDTH','TOTAL_TOKEN_COUNT'];
  var TIME_UNITS  = ['MINUTE','HOUR','DAY','MONTH'];

  function v(id) { var e=document.getElementById(id); return e?e.value.trim():''; }

  /* ── limit row builder ── */
  function makeLimitRow(limit) {
    var row = document.createElement('div');
    row.className = 'cfg-limit-row';
    row.style.cssText = 'display:flex;gap:8px;align-items:center;margin-bottom:8px;';

    var typeOpts = LIMIT_TYPES.map(function(t){
      return '<option value="'+t+'"'+(limit&&limit.limitType===t?' selected':'')+'>'+t+'</option>';
    }).join('');
    var unitOpts = '<option value="">— no window —</option>'+TIME_UNITS.map(function(u){
      return '<option value="'+u+'"'+(limit&&limit.timeUnit===u?' selected':'')+'>'+u+'</option>';
    }).join('');

    row.innerHTML =
      '<select class="cfg-form-input cfg-limit-type" style="flex:2;">'+typeOpts+'</select>'+
      '<input type="number" class="cfg-form-input cfg-form-input--mono cfg-limit-count" style="flex:1.5;" '+
        'placeholder="count" value="'+(limit?limit.limitCount:'')+'" />'+
      '<span style="color:#637282;font-size:.8rem;white-space:nowrap;">/ per</span>'+
      '<input type="number" class="cfg-form-input cfg-form-input--mono cfg-limit-amount" style="flex:.7;" '+
        'placeholder="1" value="'+(limit&&limit.timeAmount!=null?limit.timeAmount:1)+'" min="1" />'+
      '<select class="cfg-form-input cfg-limit-unit" style="flex:1.2;">'+unitOpts+'</select>'+
      '<button type="button" class="cfg-icon-btn cfg-icon-btn--danger cfg-limit-remove-btn" title="Remove">'+
        '<i class="bi bi-x-lg"></i></button>';

    row.querySelector('.cfg-limit-remove-btn').addEventListener('click', function(){ row.remove(); });
    return row;
  }

  document.getElementById('pol-add-limit-btn').addEventListener('click', function(){
    document.getElementById('pol-limits-list').appendChild(makeLimitRow(null));
  });

  function readLimits() {
    var rows = document.querySelectorAll('#pol-limits-list .cfg-limit-row');
    var limits = [];
    rows.forEach(function(row){
      var rawCount = row.querySelector('.cfg-limit-count').value.trim();
      var count = rawCount === '-1' ? -1 : parseInt(rawCount, 10);
      var rawAmount = row.querySelector('.cfg-limit-amount').value.trim();
      var amount = rawAmount === '' ? 1 : parseInt(rawAmount, 10);
      if (isNaN(count) || (count !== -1 && count <= 0)) {
        throw new Error('Limit count must be -1 (unlimited) or a positive number.');
      }
      if (isNaN(amount) || amount <= 0) {
        throw new Error('Limit time amount must be a positive number.');
      }
      limits.push({
        limitType:  row.querySelector('.cfg-limit-type').value,
        limitCount: count,
        timeUnit:   row.querySelector('.cfg-limit-unit').value || null,
        timeAmount: amount,
      });
    });
    return limits;
  }

  /* ── open / close modal ── */
  function openPlanModal(mode, data) {
    editPlanId = mode === 'edit' ? data.planId : null;
    document.getElementById('cfg-plan-modal-title').textContent = mode === 'edit' ? 'Edit subscription plan' : 'Add subscription plan';
    document.getElementById('cfg-plan-modal-save').textContent  = mode === 'edit' ? 'Save changes' : 'Add plan';
    document.getElementById('pol-display').value = mode === 'edit' ? (data.displayName||'') : '';
    document.getElementById('pol-name').value    = mode === 'edit' ? (data.planName||'')    : '';
    document.getElementById('pol-desc').value    = mode === 'edit' ? (data.description||'') : '';
    document.getElementById('pol-refid').value   = mode === 'edit' ? (data.refId||'')       : '';

    var list = document.getElementById('pol-limits-list');
    list.innerHTML = '';
    var existing = [];
    if (mode === 'edit' && data.limits) {
      try { existing = typeof data.limits === 'string' ? JSON.parse(data.limits) : data.limits; } catch(e){}
    }
    existing.forEach(function(l){ list.appendChild(makeLimitRow(l)); });

    document.getElementById('cfg-plan-modal').style.display = 'flex';
    document.getElementById('pol-display').focus();
  }
  function closePlanModal() { document.getElementById('cfg-plan-modal').style.display='none'; editPlanId=null; }

  /* ── save ── */
  document.getElementById('cfg-plan-modal-save').addEventListener('click', async function() {
    var displayName = v('pol-display');
    var planName    = v('pol-name');
    if (!displayName || !planName) { await showAlert('Display name and name are required.', 'error'); return; }

    var limits;
    try {
      limits = readLimits();
    } catch (e) {
      await showAlert(e.message, 'error');
      return;
    }

    var body = {
      id:          planName,
      displayName: displayName,
      description: v('pol-desc') || undefined,
      limits:      limits,
      refId:       v('pol-refid') || undefined,
    };

    try {
      var method = editPlanId ? 'PUT' : 'POST';
      var res = await fetch(window.devportalApi.root('/subscription-plans'), {
        method: method,
        headers: { 'Content-Type':'application/json', 'X-CSRF-Token': window.devportalApi.csrfToken() },
        body: JSON.stringify(body),
      });
      if (res.ok) {
        await showAlert(editPlanId ? 'Plan updated.' : 'Plan created.', 'success');
        window.location.reload();
      } else {
        var err = await res.json().catch(function(){ return {}; });
        await showAlert('Failed: '+(err.description||err.message||res.statusText), 'error');
      }
    } catch(e) { await showAlert('Error: '+e.message, 'error'); }
  });

  document.getElementById('cfg-plan-modal-close').addEventListener('click', closePlanModal);
  document.getElementById('cfg-plan-modal-cancel').addEventListener('click', closePlanModal);
  document.getElementById('cfg-plan-modal').addEventListener('click', function(e){ if(e.target===this) closePlanModal(); });
  document.getElementById('cfg-add-plan-btn').addEventListener('click', function() { openPlanModal('add'); });

  /* ── edit / delete delegation ── */
  var pendingDelPlanId = null;
  document.addEventListener('click', function(e) {
    if (e.target.closest('.cfg-plan-edit-btn')) {
      var btn = e.target.closest('.cfg-plan-edit-btn');
      openPlanModal('edit', {
        planId:      btn.dataset.id,
        planName:    btn.dataset.name,
        displayName: btn.dataset.display,
        description: btn.dataset.desc,
        limits:      btn.dataset.limits,
        refId:       btn.dataset.refid,
      });
      return;
    }
    if (e.target.closest('.cfg-plan-delete-btn')) {
      var btn = e.target.closest('.cfg-plan-delete-btn');
      pendingDelPlanId = btn.dataset.id;
      document.getElementById('cfg-del-plan-name-txt').textContent = btn.dataset.display || btn.dataset.name;
      document.getElementById('cfg-delete-plan-modal').style.display = 'flex';
      return;
    }
  });

  document.getElementById('cfg-del-plan-cancel').addEventListener('click', function() {
    document.getElementById('cfg-delete-plan-modal').style.display = 'none';
  });
  document.getElementById('cfg-delete-plan-modal').addEventListener('click', function(e){ if(e.target===this) this.style.display='none'; });
  document.getElementById('cfg-del-plan-confirm').addEventListener('click', async function() {
    if (!pendingDelPlanId) return;
    document.getElementById('cfg-delete-plan-modal').style.display = 'none';
    try {
      var res = await fetch(window.devportalApi.root('/subscription-plans/'+encodeURIComponent(pendingDelPlanId)), {
        method: 'DELETE',
        headers: { 'X-CSRF-Token': window.devportalApi.csrfToken() },
      });
      if (res.ok || res.status===204) {
        await showAlert('Plan deleted.', 'success');
        window.location.reload();
      } else {
        var err = await res.json().catch(function(){ return {}; });
        await showAlert('Delete failed: '+(err.description||err.message||res.statusText), 'error');
      }
    } catch(e) { await showAlert('Error: '+e.message, 'error'); }
    pendingDelPlanId = null;
  });
}());
