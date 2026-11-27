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
  /* ── a plan's name and description are one line; the full text shows on hover only when cut ── */
  document.addEventListener('mouseover', function(e) {
    var el = e.target.closest && e.target.closest('.cfg-plan-name, .cfg-plan-desc');
    if (!el) return;
    if (el.scrollWidth > el.clientWidth) el.title = el.textContent.trim();
    else el.removeAttribute('title');
  });

  /* ── condition chips: the first two fit, or the ones that do not move into "+N" ── */
  function tipRow(title, details) {
    var row = document.createElement('span');
    row.className = 'cfg-cond-tip-row';
    row.dataset.moved = 'true';
    var t = document.createElement('span'); t.className = 'cfg-cond-tip-title'; t.textContent = title;
    var d = document.createElement('span'); d.className = 'cfg-cond-tip-line'; d.textContent = details;
    row.appendChild(t); row.appendChild(d);
    return row;
  }
  function moreChip(box) {
    var more = box.querySelector('.cfg-cond-chip--more');
    if (more) return more;
    more = document.createElement('span');
    more.className = 'cfg-cond-chip cfg-cond-chip--more';
    more.tabIndex = 0;
    more.dataset.added = 'true';
    more.innerHTML = '<span class="cfg-cond-chip-more"></span><span class="cfg-cond-tip" role="tooltip"></span>';
    box.appendChild(more);
    return more;
  }
  function setMoreCount(more) {
    var n = more.querySelectorAll('.cfg-cond-tip-row').length;
    more.querySelector('.cfg-cond-chip-more').textContent = '+' + n;
    more.setAttribute('aria-label', n + ' more conditions');
  }
  function fitChips(box) {
    // Back to what the server rendered, then move chips from the end while the row overflows.
    box.querySelectorAll('.cfg-cond-tip-row[data-moved]').forEach(function(r) { r.remove(); });
    box.querySelectorAll('.cfg-cond-chip[hidden]').forEach(function(c) { c.hidden = false; });
    var added = box.querySelector('.cfg-cond-chip--more[data-added]');
    if (added) added.remove();
    var more = box.querySelector('.cfg-cond-chip--more');
    if (more) setMoreCount(more);
    box.classList.add('is-measuring');
    var chips = Array.prototype.slice.call(box.querySelectorAll('.cfg-cond-chip:not(.cfg-cond-chip--more)'));
    while (chips.length > 1 && box.scrollWidth > box.clientWidth) {
      var chip = chips.pop();
      chip.hidden = true;
      more = moreChip(box);
      more.querySelector('.cfg-cond-tip').prepend(tipRow(chip.dataset.tipTitle, chip.dataset.tipDetails));
      setMoreCount(more);
    }
    box.classList.remove('is-measuring');
  }
  function fitAllChips() { document.querySelectorAll('.cfg-plans-table .cfg-cond-chips').forEach(fitChips); }
  var fitTimer;
  window.addEventListener('resize', function() { clearTimeout(fitTimer); fitTimer = setTimeout(fitAllChips, 100); });
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', fitAllChips);
  else fitAllChips();
  // The plans panel may be shown after load (settings tabs): fit it when it becomes visible.
  window.addEventListener('hashchange', function() { setTimeout(fitAllChips, 0); });

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
