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
  /* ── delete; adding and editing a plan open its own page ── */
  var pendingDelPlanId = null;
  document.addEventListener('click', function(e) {
    if (e.target.closest('.cfg-plan-delete-btn')) {
      var btn = e.target.closest('.cfg-plan-delete-btn');
      // A plan that an API offers, or that has subscriptions, can't be deleted: the dialog says it is in
      // use, and Delete is disabled.
      var name = btn.dataset.display || btn.dataset.name;
      var apis = Number(btn.dataset.apis) || 0;
      var subscriptions = Number(btn.dataset.subscriptions) || 0;
      var blocked = apis > 0 || subscriptions > 0;
      document.getElementById('cfg-del-plan-ask').style.display = blocked ? 'none' : '';
      document.getElementById('cfg-del-plan-blocked').style.display = blocked ? '' : 'none';
      document.getElementById('cfg-del-plan-confirm').disabled = blocked;
      pendingDelPlanId = blocked ? null : btn.dataset.id;
      if (blocked) {
        var strong = document.createElement('strong');
        strong.textContent = name;
        document.getElementById('cfg-del-plan-blocked')
          .replaceChildren(strong, document.createTextNode(' is in use and can\'t be deleted.'));
      } else {
        document.getElementById('cfg-del-plan-name-txt').textContent = name;
      }
      document.getElementById('cfg-delete-plan-modal').style.display = 'flex';
      return;
    }
  });

  document.getElementById('cfg-del-plan-cancel').addEventListener('click', function() {
    document.getElementById('cfg-delete-plan-modal').style.display = 'none';
  });
  document.getElementById('cfg-delete-plan-modal').addEventListener('click', function(e){ if(e.target===this) this.style.display='none'; });
  document.getElementById('cfg-del-plan-confirm').addEventListener('click', function() {
    if (!pendingDelPlanId) return;
    withButtonBusy(this, 'Deleting…', async function() {
      try {
        var res = await fetch(window.apiPortalApi.root('/subscription-plans/'+encodeURIComponent(pendingDelPlanId)), {
          method: 'DELETE',
          headers: { 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
        });
        if (res.ok || res.status===204) {
          await showAlert('Plan deleted.', 'success');
          window.location.reload();
          return;
        }
        var err = await res.json().catch(function(){ return {}; });
        await showAlert('Plan could not be deleted: '+(err.message||err.description||res.statusText), 'error');
      } catch(e) { await showAlert('The portal could not be reached. Check your connection and try again.', 'error'); }
      document.getElementById('cfg-delete-plan-modal').style.display = 'none';
      pendingDelPlanId = null;
    });
  });
}());
