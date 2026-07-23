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
  var ORG_ID      = _cfg.dataset.orgId || '';
  var BASE_URL    = _cfg.dataset.baseUrl || '';
  var currentStep = 0;
  var specFile    = null;
  var docFiles    = [];
  var editingId   = null;
  var editMode    = false;
  var existingDocs  = [];
  var docsToRemove  = [];
  var agentVis    = 'Visible';
  var allPolicies = [];
  var policyChips = [];
  var contentZipFile = null;
  var hasContentStep = false;   /* the Content step exists only for existing, non-MCP APIs */
  var wizardOrigin = 'cfg-apis'; /* tab that launched the wizard: 'cfg-apis' or 'cfg-mcps' */

  /* build id→api lookup and load all policies from server-rendered data blobs */
  var apiMap = {};
  (function() {
    try {
      var el = document.getElementById('cfg-org-apis-data');
      if (el) {
        var list = JSON.parse(el.textContent);
        list.forEach(function(api) { apiMap[api.apiId] = api; });
      }
    } catch(e) {}
    try {
      var pe = document.getElementById('cfg-plans-data');
      if (pe) allPolicies = JSON.parse(pe.textContent) || [];
    } catch(e) {}
  }());

  /* ── helpers ── */
  function esc(s) {
    return String(s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
  }
  function v(id)   { var e=document.getElementById(id); return e?e.value.trim():''; }
  function sv(id,val){ var e=document.getElementById(id); if(e) e.value=val||''; }
  function sel(id,val){ var e=document.getElementById(id); if(e) e.value=val; }

  /* type-color map */
  var typeMap = {
    RestApi:   { av:'cfg-api-avatar--rest',    tb:'cfg-type-badge--rest',    label:'REST' },
    WS:        { av:'cfg-api-avatar--ws',      tb:'cfg-type-badge--ws',      label:'WebSocket' },
    GRAPHQL:   { av:'cfg-api-avatar--graphql', tb:'cfg-type-badge--graphql', label:'GraphQL' },
    SOAP:      { av:'cfg-api-avatar--soap',    tb:'cfg-type-badge--soap',    label:'SOAP' },
    WebSubApi: { av:'cfg-api-avatar--websub',  tb:'cfg-type-badge--websub',  label:'WebSub' },
    Mcp:       { av:'cfg-api-avatar--mcp',     tb:'cfg-type-badge--mcp',     label:'MCP' },
  };
  function typeInfo(t) { return typeMap[t] || typeMap['RestApi']; }
  function initials(name) { var w=String(name||'').trim().split(/\s+/); return (w[0]?w[0][0]:'') + (w[1]?w[1][0]:''); }

  /* ── handle auto-generate ── */
  var handleTouched = false;
  document.getElementById('wz-handle').addEventListener('input', function() { handleTouched = true; });
  function autoHandle() {
    if (handleTouched) return;
    var nm  = v('wz-name').toLowerCase().replace(/[^a-z0-9]+/g,'-').replace(/^-|-$/g,'');
    var ver = v('wz-version').toLowerCase().replace(/[^a-z0-9.]+/g,'-').replace(/^-|-$/g,'');
    sv('wz-handle', nm && ver ? nm + '-' + ver : nm || ver);
  }
  document.getElementById('wz-name').addEventListener('input', autoHandle);
  document.getElementById('wz-version').addEventListener('input', autoHandle);

  /* ── agent visibility toggle ── */
  document.getElementById('wz-vis-visible').addEventListener('click', function() {
    agentVis = 'Visible';
    document.getElementById('wz-vis-visible').classList.add('active');
    document.getElementById('wz-vis-hidden').classList.remove('active');
  });
  document.getElementById('wz-vis-hidden').addEventListener('click', function() {
    agentVis = 'Hidden';
    document.getElementById('wz-vis-hidden').classList.add('active');
    document.getElementById('wz-vis-visible').classList.remove('active');
  });

  /* ── production URL required when status is Published ── */
  function syncProdRequiredMarker() {
    var required = document.getElementById('wz-status').value === 'PUBLISHED';
    document.getElementById('wz-prod-required').style.display = required ? '' : 'none';
  }
  document.getElementById('wz-status').addEventListener('change', function() {
    syncProdRequiredMarker();
    setFieldError('wz-prod', 'wz-prod-error', '');
  });

  /* ── subscription policy chip input ── */
  function renderPolicyChips() {
    var container = document.getElementById('wz-policy-chips');
    if (!container) return;
    container.innerHTML = '';
    policyChips.forEach(function(pol) {
      var chip = document.createElement('span');
      chip.className = 'cfg-chip';
      chip.innerHTML = esc(pol.displayName || pol.planName) +
        '<button type="button" class="cfg-chip-remove" data-name="'+esc(pol.planName)+'" title="Remove"><i class="bi bi-x"></i></button>';
      chip.querySelector('.cfg-chip-remove').addEventListener('click', function(e) {
        e.stopPropagation();
        var name = e.currentTarget.dataset.name;
        policyChips = policyChips.filter(function(p){ return p.planName !== name; });
        renderPolicyChips();
      });
      container.appendChild(chip);
    });
  }

  function showPolicyDropdown(query) {
    var dd = document.getElementById('wz-policy-dropdown');
    if (!dd) return;
    var q = (query||'').toLowerCase().trim();
    var matches = allPolicies.filter(function(p) {
      if (policyChips.some(function(c){ return c.planName === p.planName; })) return false;
      if (!q) return true;
      return (p.displayName||'').toLowerCase().indexOf(q) >= 0 ||
             (p.planName||'').toLowerCase().indexOf(q) >= 0 ||
             (p.description||'').toLowerCase().indexOf(q) >= 0;
    });
    if (!matches.length) {
      dd.innerHTML = '<div class="cfg-chip-dd-empty">No matching plans</div>';
    } else {
      dd.innerHTML = matches.map(function(p) {
        return '<div class="cfg-chip-dd-item" data-name="'+esc(p.planName)+'">' +
          '<div class="cfg-chip-dd-name">'+esc(p.displayName||p.planName)+'</div>' +
          (p.description ? '<div class="cfg-chip-dd-desc">'+esc(p.description)+'</div>' : '') +
          '</div>';
      }).join('');
      dd.querySelectorAll('.cfg-chip-dd-item').forEach(function(item) {
        item.addEventListener('mousedown', function(e) {
          e.preventDefault();
          var name = item.dataset.name;
          var pol = allPolicies.find(function(p){ return p.planName === name; });
          if (pol && !policyChips.some(function(c){ return c.planName === name; })) {
            policyChips.push(pol);
            renderPolicyChips();
          }
          document.getElementById('wz-policy-search').value = '';
          dd.style.display = 'none';
        });
      });
    }
    dd.style.display = 'block';
  }

  function hidePolicyDropdown() {
    var dd = document.getElementById('wz-policy-dropdown');
    if (dd) dd.style.display = 'none';
  }

  function selectedPolicies() {
    return policyChips.map(function(p){ return p.planName; });
  }

  function setSelectedPolicies(namesOrObjs) {
    policyChips = [];
    if (namesOrObjs && namesOrObjs.length) {
      namesOrObjs.forEach(function(item) {
        var name = typeof item === 'string' ? item : (item.planName || item.displayName || '');
        if (!name) return;
        var pol = allPolicies.find(function(p){ return p.planName === name || p.displayName === name; });
        policyChips.push(pol || { planName: name, displayName: name, description: '' });
      });
    }
    renderPolicyChips();
  }

  (function() {
    var wrap = document.getElementById('wz-policy-chip-wrap');
    var searchEl = document.getElementById('wz-policy-search');
    if (!searchEl) return;
    if (wrap) wrap.addEventListener('click', function() { searchEl.focus(); });
    searchEl.addEventListener('focus', function() { showPolicyDropdown(this.value); });
    searchEl.addEventListener('input', function() { showPolicyDropdown(this.value); });
    searchEl.addEventListener('blur', function() { setTimeout(hidePolicyDropdown, 160); });
    searchEl.addEventListener('keydown', function(e) {
      if (e.key === 'Escape') { hidePolicyDropdown(); searchEl.blur(); }
    });
  }());

  /* ── wizard show/hide ── */
  function showWizard(api, kindHint) {
    /* MCP servers and APIs share this wizard. Derive the "kind" from the record being
       edited, or from the launching button (kindHint) when adding. */
    var isMcp = api ? (api.apiType === 'Mcp') : (kindHint === 'mcp');
    wizardOrigin = isMcp ? 'cfg-mcps' : 'cfg-apis';

    /* The wizard markup lives inside #cfg-apis; reveal that panel (with its list hidden)
       regardless of which tab launched it, and keep the launching tab highlighted. */
    document.querySelectorAll('.cfg-panel').forEach(function(p) { p.style.display = p.id === 'cfg-apis' ? 'flex' : 'none'; });
    document.querySelectorAll('.cfg-nav-item').forEach(function(a) { a.classList.toggle('active', a.dataset.panel === wizardOrigin); });
    document.getElementById('cfg-apis-list').style.display = 'none';
    document.getElementById('cfg-apis-wizard').style.display = 'flex';

    var backLink = document.getElementById('cfg-wizard-back-link');
    if (backLink) backLink.innerHTML = '<i class="bi bi-arrow-left"></i> ' + (isMcp ? 'Back to MCP Servers' : 'Back to Manage APIs');

    /* MCP servers carry a tools schema instead of an API contract — relabel the Spec step. */
    var specStepLabel = document.querySelector('#cfg-apis-wizard .cfg-step[data-step="1"] .cfg-step-label');
    if (specStepLabel) specStepLabel.textContent = isMcp ? 'Tools Schema' : 'Spec';
    var specDesc = document.querySelector('#cfg-wstep-1 .cfg-wstep-desc');
    if (specDesc) specDesc.innerHTML = isMcp
      ? 'Upload the MCP tools schema that defines this server’s tools. <span class="required">*</span> Required. Accepted: YAML or JSON (e.g. <code>definition.yaml</code>).'
      : 'Upload the contract that defines this API. <span class="required">*</span> Required. Accepted: OpenAPI (.json / .yaml), AsyncAPI, GraphQL SDL (.graphql), or WSDL (.wsdl / .xml).';
    var specOnFileTxt = document.querySelector('#wz-spec-onfile span');
    if (specOnFileTxt) specOnFileTxt.textContent = isMcp
      ? 'Existing tools schema on file — attach a new file above to replace it'
      : 'Existing spec on file — attach a new file above to replace it';

    currentStep = 0; specFile = null; docFiles = []; handleTouched = false;
    editMode = !!api; existingDocs = []; docsToRemove = [];

    if (api) {
      /* edit mode */
      editingId = api.apiId;
      document.getElementById('cfg-wizard-title').textContent = isMcp ? 'Edit MCP Server' : 'Edit API';
      sv('wz-name',       api.apiName);
      sv('wz-handle',     api.apiHandle);   handleTouched = true;
      sv('wz-version',    api.apiVersion);
      sel('wz-type',      api.apiType);
      sel('wz-status',    api.apiStatus === 'DEPRECATED' ? 'DEPRECATED' : 'PUBLISHED');
      sv('wz-desc',       api.apiDescription);
      sv('wz-tags',       (api.tags && api.tags.length) ? (Array.isArray(api.tags) ? api.tags.join(', ') : api.tags) : '');
      sv('wz-prod',       api.productionUrl);
      sv('wz-sandbox',    api.sandboxUrl);
      sv('wz-tech-owner', (api.owners && api.owners.technicalOwner)      || api.technicalOwner      || '');
      sv('wz-tech-email', (api.owners && api.owners.technicalOwnerEmail) || api.technicalOwnerEmail || '');
      sv('wz-biz-owner',  (api.owners && api.owners.businessOwner)       || api.businessOwner       || '');
      sv('wz-biz-email',  (api.owners && api.owners.businessOwnerEmail)  || api.businessOwnerEmail  || '');
      agentVis = api.agentVisibility || 'Visible';
      document.getElementById('wz-vis-visible').classList.toggle('active', agentVis !== 'Hidden');
      document.getElementById('wz-vis-hidden').classList.toggle('active',  agentVis === 'Hidden');
      setSelectedPolicies(api.subscriptionPlans || []);
      /* existing docs are embedded server-side in cfg-org-apis-data */
      existingDocs = api.existingDocs || [];
    } else {
      /* add mode */
      editingId = null;
      document.getElementById('cfg-wizard-title').textContent = isMcp ? 'Add MCP Server' : 'Add API';
      ['wz-name','wz-version','wz-handle','wz-desc','wz-tags','wz-prod','wz-sandbox','wz-tech-owner','wz-tech-email','wz-biz-owner','wz-biz-email'].forEach(function(id){ sv(id,''); });
      sel('wz-type', isMcp ? 'Mcp' : 'RestApi'); sel('wz-status','PUBLISHED');
      agentVis = 'Visible';
      document.getElementById('wz-vis-visible').classList.add('active');
      document.getElementById('wz-vis-hidden').classList.remove('active');
      setSelectedPolicies([]);
    }

    syncProdRequiredMarker();

    /* clear any leftover validation errors */
    ['wz-name-error','wz-version-error','wz-handle-error','wz-desc-error','wz-prod-error','wz-tech-email-error','wz-biz-email-error'].forEach(function(eid) {
      var el = document.getElementById(eid); if (el) el.style.display = 'none';
    });
    ['wz-name','wz-version','wz-handle','wz-desc','wz-prod','wz-tech-email','wz-biz-email'].forEach(function(fid) {
      var el = document.getElementById(fid); if (el) el.classList.remove('cfg-form-input--error');
    });

    document.getElementById('wz-spec-chip').classList.remove('visible');
    var specErrEl = document.getElementById('wz-spec-error');
    if (specErrEl) specErrEl.style.display = 'none';
    var specOnFile = document.getElementById('wz-spec-onfile');
    if (specOnFile) specOnFile.style.display = editMode ? 'flex' : 'none';
    document.getElementById('wz-docs-list').innerHTML = '';
    renderExistingDocs();
    document.getElementById('wz-docs-empty').style.display = 'block';

    /* The Content step (landing page & assets) exists only for existing, non-MCP APIs.
       The /apis/{handle}/assets endpoint resolves only REST/SOAP/WS/WebSub/GraphQL APIs;
       MCP servers manage content via /mcp-servers/{id}/assets instead. */
    hasContentStep = editMode && !!api && api.apiType !== 'Mcp';
    var contentStepEl = document.getElementById('cfg-step-content');
    var contentConnEl = document.getElementById('cfg-step-connector-content');
    if (contentStepEl) contentStepEl.style.display = hasContentStep ? '' : 'none';
    if (contentConnEl) contentConnEl.style.display = hasContentStep ? '' : 'none';
    resetContentUpload();

    updateWizardStep(0);
  }

  function hideWizard() {
    document.getElementById('cfg-apis-wizard').style.display = 'none';
    document.getElementById('cfg-apis-list').style.display = '';
    editingId = null;
    /* Return to the tab that launched the wizard (activate() also clears wizard state). */
    if (window.cfgActivatePanel) window.cfgActivatePanel(wizardOrigin || 'cfg-apis');
  }

  /* ── step navigation ── */
  function updateWizardStep(step) {
    currentStep = step;
    /* Content (step 3) is the terminal step when present (edit, non-MCP); otherwise Documentation (2). */
    var lastStep = hasContentStep ? 3 : 2;
    for (var i=0; i<4; i++) {
      var p = document.getElementById('cfg-wstep-'+i);
      if (p) p.style.display = i===step ? 'flex' : 'none';
    }
    var stepEls = document.querySelectorAll('#cfg-apis-wizard .cfg-step');
    var connEls = document.querySelectorAll('#cfg-apis-wizard .cfg-step-connector');
    stepEls.forEach(function(el,i) {
      el.classList.remove('cfg-step--active','cfg-step--done');
      var circ = el.querySelector('.cfg-step-circle');
      if (i < step) { el.classList.add('cfg-step--done'); circ.innerHTML='<i class="bi bi-check" style="font-size:.75rem;line-height:1;"></i>'; }
      else          { if(i===step) el.classList.add('cfg-step--active'); circ.textContent=String(i+1); }
    });
    connEls.forEach(function(el,i) { el.classList.toggle('cfg-step-connector--done', i < step); });
    var backBtn = document.getElementById('cfg-wizard-back-step');
    var nextBtn = document.getElementById('cfg-wizard-next');
    backBtn.style.display = step > 0 ? 'inline-flex' : 'none';
    nextBtn.textContent = step === lastStep ? (editingId ? 'Save changes' : 'Save API') : 'Next';
    nextBtn.disabled = false;
    nextBtn.title = '';
  }

  /* ── save API (create / update) ── */
  async function saveApi() {
    var name    = v('wz-name');
    var version = v('wz-version');
    var handle  = v('wz-handle');
    if (!name || !version || !handle) {
      await showAlert('API name, version, and handle are required.', 'error');
      return;
    }

    var meta = {
      name:        name,
      version:     version,
      handle:      handle,
      type:        document.getElementById('wz-type').value,
      status:      document.getElementById('wz-status').value,
      description: v('wz-desc'),
      tags:           v('wz-tags') ? v('wz-tags').split(',').map(function(t){return t.trim();}).filter(Boolean) : [],
      agentVisibility: agentVis,
      owners: {
        technicalOwner:      v('wz-tech-owner') || undefined,
        technicalOwnerEmail: v('wz-tech-email') || undefined,
        businessOwner:       v('wz-biz-owner') || undefined,
        businessOwnerEmail:  v('wz-biz-email') || undefined,
      },
      endPoints: {
        productionURL: v('wz-prod') || undefined,
        sandboxURL:    v('wz-sandbox') || undefined,
      },
      subscriptionPlans: selectedPolicies().map(function(n){ return { id: n }; }),
      docsToRemove: docsToRemove.length ? docsToRemove : undefined,
    };

    var fd = new FormData();
    fd.append('metadata', JSON.stringify(meta));
    /* The single `definition` field carries the contract for every type. For MCP servers it is
       the tools schema (stored as SCHEMA_DEFINITION); for other types it is the API definition. */
    if (specFile) fd.append('definition', specFile);
    docFiles.forEach(function(f) { fd.append('docs', f); });

    /* MCP servers are created/updated via /mcp-servers (which requires type MCP and rejects
       non-MCP); everything else goes through /apis (which rejects MCP). */
    var base   = meta.type === 'Mcp' ? '/mcp-servers' : '/apis';
    var url    = editingId
      ? window.devportalApi.root(base + '/' + encodeURIComponent(editingId))
      : window.devportalApi.root(base);
    var method = editingId ? 'PUT' : 'POST';

    try {
      var res = await fetch(url, {
        method: method,
        headers: { 'X-CSRF-Token': window.devportalApi.csrfToken() },
        body: fd,
      });
      if (res.ok) {
        var savedNoun = meta.type === 'Mcp' ? 'MCP server' : 'API';
        await showAlert(savedNoun + (editingId ? ' updated successfully.' : ' created successfully.'), 'success');
        window.location.reload();
      } else {
        var err = await res.json().catch(function(){ return {}; });
        await showAlert('Failed: ' + (err.description || err.message || res.statusText), 'error');
      }
    } catch(e) {
      await showAlert('Error: ' + e.message, 'error');
    }
  }

  /* ── bulk selection (shared by the APIs and MCP tables, each scoped to its own tbody) ── */
  var pendingBulkIds = null;

  function initBulkSelect(cfg) {
    var tbody = document.getElementById(cfg.tbodyId);
    if (!tbody) return;

    function selected() {
      return Array.prototype.slice.call(tbody.querySelectorAll('.cfg-row-check:checked'));
    }
    function sync() {
      var checks = selected();
      var bar = document.getElementById(cfg.barId);
      var countEl = document.getElementById(cfg.countId);
      if (bar) bar.style.display = checks.length > 0 ? 'flex' : 'none';
      if (countEl) countEl.textContent = checks.length + ' ' + cfg.noun + (checks.length === 1 ? '' : 's') + ' selected';
      var allChecks = tbody.querySelectorAll('.cfg-row-check');
      var selectAll = document.getElementById(cfg.selectAllId);
      if (selectAll) {
        selectAll.checked = allChecks.length > 0 && checks.length === allChecks.length;
        selectAll.indeterminate = checks.length > 0 && checks.length < allChecks.length;
      }
    }

    var selectAll = document.getElementById(cfg.selectAllId);
    if (selectAll) selectAll.addEventListener('change', function() {
      var checked = this.checked;
      tbody.querySelectorAll('.cfg-row-check').forEach(function(c) { c.checked = checked; });
      sync();
    });

    tbody.addEventListener('change', function(e) {
      if (e.target.classList.contains('cfg-row-check')) sync();
    });

    var delBtn = document.getElementById(cfg.deleteBtnId);
    if (delBtn) delBtn.addEventListener('click', function() {
      var checks = selected();
      if (!checks.length) return;
      pendingBulkIds = checks.map(function(c) { return { id: c.dataset.id, name: c.dataset.name }; });
      pendingDelId = null;
      var label = checks.length + ' selected ' + cfg.noun + (checks.length === 1 ? '' : 's');
      document.getElementById('cfg-del-api-name-txt').textContent = label;
      document.getElementById('cfg-delete-api-modal').style.display = 'flex';
    });
  }

  initBulkSelect({ tbodyId: 'cfg-apis-tbody', selectAllId: 'cfg-select-all',      barId: 'cfg-bulk-bar',      countId: 'cfg-bulk-bar-count',      deleteBtnId: 'cfg-bulk-delete-btn',      noun: 'API' });
  initBulkSelect({ tbodyId: 'cfg-mcps-tbody', selectAllId: 'cfg-mcps-select-all', barId: 'cfg-mcps-bulk-bar', countId: 'cfg-mcps-bulk-bar-count', deleteBtnId: 'cfg-mcps-bulk-delete-btn', noun: 'MCP server' });

  /* ── delete ── */
  var pendingDelId = null;
  document.getElementById('cfg-del-api-cancel').addEventListener('click', function() {
    document.getElementById('cfg-delete-api-modal').style.display = 'none';
  });
  document.getElementById('cfg-delete-api-modal').addEventListener('click', function(e) {
    if (e.target===this) this.style.display='none';
  });
  document.getElementById('cfg-del-api-confirm').addEventListener('click', async function() {
    document.getElementById('cfg-delete-api-modal').style.display = 'none';

    if (pendingBulkIds && pendingBulkIds.length) {
      var ids = pendingBulkIds.slice();
      pendingBulkIds = null;
      var failCount = 0;
      await Promise.all(ids.map(async function(item) {
        try {
          var r = await fetch(window.devportalApi.root(mutationBasePath(item.id)+'/'+encodeURIComponent(item.id)), {
            method: 'DELETE',
            headers: { 'X-CSRF-Token': window.devportalApi.csrfToken() },
          });
          if (r.ok || r.status===204) {
            var row = document.getElementById('cfg-api-row-'+item.id);
            if (row) row.remove();
          } else { failCount++; }
        } catch(e) { failCount++; }
      }));
      document.querySelectorAll('.cfg-row-check').forEach(function(c){ c.checked = false; });
      document.querySelectorAll('#cfg-select-all, #cfg-mcps-select-all').forEach(function(sa){ sa.checked = false; sa.indeterminate = false; });
      document.querySelectorAll('.cfg-bulk-bar').forEach(function(b){ b.style.display = 'none'; });
      await showAlert(
        failCount > 0 ? failCount + ' record(s) could not be deleted.' : 'Deleted.',
        failCount > 0 ? 'error' : 'success'
      );
      return;
    }

    if (!pendingDelId) return;
    try {
      var res = await fetch(window.devportalApi.root(mutationBasePath(pendingDelId)+'/'+encodeURIComponent(pendingDelId)), {
        method: 'DELETE',
        headers: { 'X-CSRF-Token': window.devportalApi.csrfToken() },
      });
      if (res.ok || res.status===204) {
        await showAlert(mutationBasePath(pendingDelId) === '/mcp-servers' ? 'MCP server deleted.' : 'API deleted.', 'success');
        var row = document.getElementById('cfg-api-row-'+pendingDelId);
        if (row) row.remove();
        pendingDelId = null;
      } else {
        var err = await res.json().catch(function(){ return {}; });
        await showAlert('Delete failed: ' + (err.description || err.message || res.statusText), 'error');
      }
    } catch(e) { await showAlert('Error: ' + e.message, 'error'); }
  });

  /* ── deprecate confirmation modal ── */
  var pendingDeprecateId   = null;
  var pendingDeprecateName = null;
  document.getElementById('cfg-dep-api-cancel').addEventListener('click', function() {
    document.getElementById('cfg-deprecate-api-modal').style.display = 'none';
    pendingDeprecateId = pendingDeprecateName = null;
  });
  document.getElementById('cfg-deprecate-api-modal').addEventListener('click', function(e) {
    if (e.target === this) { this.style.display = 'none'; pendingDeprecateId = pendingDeprecateName = null; }
  });
  document.getElementById('cfg-dep-api-confirm').addEventListener('click', async function() {
    document.getElementById('cfg-deprecate-api-modal').style.display = 'none';
    if (!pendingDeprecateId) return;
    await setApiStatus(pendingDeprecateId, pendingDeprecateName, 'DEPRECATED');
    pendingDeprecateId = pendingDeprecateName = null;
  });

  /* ── publish/unpublish ── */
  async function setApiStatus(apiId, apiName, newStatus) {
    /* fetch current metadata, patch status, PUT back — MCP records use /mcp-servers */
    var base = mutationBasePath(apiId);
    try {
      var res = await fetch(window.devportalApi.root(base+'/'+encodeURIComponent(apiId)), {
        headers: { 'Content-Type':'application/json', 'X-CSRF-Token': window.devportalApi.csrfToken() }
      });
      if (!res.ok) throw new Error(res.statusText);
      var apiData = await res.json();
      /* rebuild minimal meta with new status */
      var meta = Object.assign({}, apiData, {
        status: newStatus,
        endPoints: apiData.endPoints || {},
        subscriptionPlans: apiData.subscriptionPlans || [],
      });
      var fd = new FormData();
      fd.append('metadata', JSON.stringify(meta));
      var putRes = await fetch(window.devportalApi.root(base+'/'+encodeURIComponent(apiId)), {
        method: 'PUT',
        headers: { 'X-CSRF-Token': window.devportalApi.csrfToken() },
        body: fd,
      });
      if (putRes.ok) {
        await showAlert(newStatus === 'PUBLISHED' ? apiName+' published.' : apiName+' deprecated.', 'success');
        window.location.reload();
      } else {
        var e2 = await putRes.json().catch(function(){ return {}; });
        await showAlert('Failed: '+(e2.description||e2.message||putRes.statusText), 'error');
      }
    } catch(e) { await showAlert('Error: '+e.message, 'error'); }
  }

  /* ── load API data from pre-built map ── */
  function apiDataFromRow(id) { return apiMap[id] || null; }

  /* Which REST resource family owns this handle — MCP servers are managed via /mcp-servers,
     everything else via /apis. Falls back to /apis for unknown ids. */
  function mutationBasePath(id) {
    var api = apiMap[id];
    return (api && api.apiType === 'Mcp') ? '/mcp-servers' : '/apis';
  }

  /* ── event delegation for table actions ── */
  document.addEventListener('click', function(e) {
    /* open dropdown */
    if (e.target.closest('.cfg-menu-trigger')) {
      e.stopPropagation();
      var t  = e.target.closest('.cfg-menu-trigger');
      var dd = t.parentElement.querySelector('.cfg-dropdown');
      var open = dd.style.display === 'flex';
      document.querySelectorAll('.cfg-dropdown').forEach(function(d){ d.style.display='none'; });
      if (!open) dd.style.display = 'flex';
      return;
    }
    if (!e.target.closest('.cfg-dropdown')) {
      document.querySelectorAll('.cfg-dropdown').forEach(function(d){ d.style.display='none'; });
    }

    /* view details */
    if (e.target.closest('.cfg-view-trigger')) {
      var btn = e.target.closest('.cfg-view-trigger');
      document.querySelectorAll('.cfg-dropdown').forEach(function(d){ d.style.display='none'; });
      openDrawer(apiDataFromRow(btn.dataset.id));
      return;
    }

    /* edit */
    if (e.target.closest('.cfg-edit-trigger')) {
      btn = e.target.closest('.cfg-edit-trigger');
      document.querySelectorAll('.cfg-dropdown').forEach(function(d){ d.style.display='none'; });
      var api = apiDataFromRow(btn.dataset.id);
      if (api) showWizard(api);
      return;
    }

    /* publish */
    if (e.target.closest('.cfg-publish-trigger')) {
      btn = e.target.closest('.cfg-publish-trigger');
      document.querySelectorAll('.cfg-dropdown').forEach(function(d){ d.style.display='none'; });
      setApiStatus(btn.dataset.id, btn.dataset.name, 'PUBLISHED');
      return;
    }

    /* deprecate */
    if (e.target.closest('.cfg-unpublish-trigger')) {
      btn = e.target.closest('.cfg-unpublish-trigger');
      document.querySelectorAll('.cfg-dropdown').forEach(function(d){ d.style.display='none'; });
      pendingDeprecateId   = btn.dataset.id;
      pendingDeprecateName = btn.dataset.name;
      document.getElementById('cfg-dep-api-name-txt').textContent = btn.dataset.name;
      document.getElementById('cfg-deprecate-api-modal').style.display = 'flex';
      return;
    }

    /* delete */
    if (e.target.closest('.cfg-delete-trigger')) {
      btn = e.target.closest('.cfg-delete-trigger');
      e.stopPropagation();
      document.querySelectorAll('.cfg-dropdown').forEach(function(d){ d.style.display='none'; });
      pendingDelId = btn.dataset.id;
      document.getElementById('cfg-del-api-name-txt').textContent = btn.dataset.name;
      document.getElementById('cfg-delete-api-modal').style.display = 'flex';
      return;
    }
  });

  /* ── Add API btn ── */
  /* ── step 0 validation ── */
  function setFieldError(inputId, errorId, msg) {
    var errEl   = document.getElementById(errorId);
    var inputEl = document.getElementById(inputId);
    if (errEl)   { errEl.textContent = msg; errEl.style.display = msg ? 'block' : 'none'; }
    if (inputEl) { inputEl.classList.toggle('cfg-form-input--error', !!msg); }
  }

  function validateStep0() {
    var ok = true;
    var emailRe = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

    var name = v('wz-name');
    setFieldError('wz-name', 'wz-name-error', name ? '' : 'API name is required.');
    if (!name) ok = false;

    var version = v('wz-version');
    setFieldError('wz-version', 'wz-version-error', version ? '' : 'Version is required.');
    if (!version) ok = false;

    var desc = v('wz-desc');
    setFieldError('wz-desc', 'wz-desc-error', desc ? '' : 'Description is required.');
    if (!desc) ok = false;

    var handle = v('wz-handle');
    if (!handle) {
      setFieldError('wz-handle', 'wz-handle-error', 'Handle is required.');
      ok = false;
    } else if (!/^[a-z0-9][a-z0-9\-.]*[a-z0-9]$|^[a-z0-9]$/.test(handle)) {
      setFieldError('wz-handle', 'wz-handle-error', 'Must be lowercase letters, numbers, hyphens, and dots only.');
      ok = false;
    } else {
      setFieldError('wz-handle', 'wz-handle-error', '');
    }

    var status = document.getElementById('wz-status').value;
    var prodUrl = v('wz-prod');
    if (status === 'PUBLISHED' && !prodUrl) {
      setFieldError('wz-prod', 'wz-prod-error', 'Production URL is required to publish an API.');
      ok = false;
    } else {
      setFieldError('wz-prod', 'wz-prod-error', '');
    }

    var techEmail = v('wz-tech-email');
    if (techEmail && !emailRe.test(techEmail)) {
      setFieldError('wz-tech-email', 'wz-tech-email-error', 'Invalid email address.');
      ok = false;
    } else {
      setFieldError('wz-tech-email', 'wz-tech-email-error', '');
    }

    var bizEmail = v('wz-biz-email');
    if (bizEmail && !emailRe.test(bizEmail)) {
      setFieldError('wz-biz-email', 'wz-biz-email-error', 'Invalid email address.');
      ok = false;
    } else {
      setFieldError('wz-biz-email', 'wz-biz-email-error', '');
    }

    return ok;
  }

  document.getElementById('cfg-add-api-btn').addEventListener('click', function() { showWizard(null); });
  document.getElementById('cfg-add-mcp-btn').addEventListener('click', function() { showWizard(null, 'mcp'); });

  document.getElementById('cfg-wizard-back-link').addEventListener('click', function(e) { e.preventDefault(); hideWizard(); });
  document.getElementById('cfg-wizard-cancel').addEventListener('click', function() { hideWizard(); });
  document.getElementById('cfg-wizard-back-step').addEventListener('click', function() { if(currentStep>0) updateWizardStep(currentStep-1); });
  document.getElementById('cfg-wizard-next').addEventListener('click', async function() {
    if (currentStep === 0) {
      if (!validateStep0()) return;
      updateWizardStep(1);
    } else if (currentStep === 1) {
      if (!specFile && !editMode) {
        var errEl = document.getElementById('wz-spec-error');
        if (errEl) errEl.style.display = 'block';
        return;
      }
      updateWizardStep(2);
    } else if (currentStep === 2 && hasContentStep) {
      updateWizardStep(3);
    } else {
      await saveApi();
    }
  });

  document.querySelectorAll('#cfg-apis-wizard .cfg-step').forEach(function(el) {
    el.addEventListener('click', function() {
      var s = parseInt(el.getAttribute('data-step'),10);
      if (s < currentStep) updateWizardStep(s);
    });
  });

  /* ── Spec ── */
  function handleSpecFile(f) {
    if (!f) return;
    if (!/\.(json|ya?ml|graphql|gql|wsdl|xml)$/i.test(f.name)) {
      showAlert('Unsupported file type. Accepted: .json, .yaml, .yml, .graphql, .gql, .wsdl, .xml', 'error');
      return;
    }
    specFile = f;
    document.getElementById('wz-spec-name').textContent = f.name;
    document.getElementById('wz-spec-chip').classList.add('visible');
    var errEl = document.getElementById('wz-spec-error');
    if (errEl) errEl.style.display = 'none';
    var onFile = document.getElementById('wz-spec-onfile');
    if (onFile) onFile.style.display = 'none';
  }

  document.getElementById('wz-spec-input').addEventListener('change', function(e) {
    handleSpecFile(e.target.files && e.target.files[0]);
  });
  document.getElementById('wz-spec-remove').addEventListener('click', function() {
    specFile = null;
    document.getElementById('wz-spec-input').value = '';
    document.getElementById('wz-spec-chip').classList.remove('visible');
    var onFile = document.getElementById('wz-spec-onfile');
    if (onFile) onFile.style.display = editMode ? 'flex' : 'none';
  });

  /* drag & drop on the upload zone */
  (function() {
    var zone = document.getElementById('wz-spec-zone');
    if (!zone) return;
    zone.addEventListener('dragover', function(e) {
      e.preventDefault();
      e.stopPropagation();
      zone.classList.add('cfg-upload-zone--active');
    });
    zone.addEventListener('dragleave', function(e) {
      if (!zone.contains(e.relatedTarget)) {
        zone.classList.remove('cfg-upload-zone--active');
      }
    });
    zone.addEventListener('drop', function(e) {
      e.preventDefault();
      e.stopPropagation();
      zone.classList.remove('cfg-upload-zone--active');
      var files = e.dataTransfer && e.dataTransfer.files;
      if (files && files.length) handleSpecFile(files[0]);
    });
  }());

  /* ── Existing docs (edit mode) ── */
  function renderExistingDocs() {
    var list = document.getElementById('wz-existing-docs-list');
    if (!list) return;
    list.innerHTML = '';
    existingDocs.forEach(function(name) {
      var item = document.createElement('div');
      item.className = 'cfg-docs-item';
      item.innerHTML = '<span class="cfg-docs-icon"><i class="bi bi-file-text"></i></span>' +
        '<span class="cfg-docs-name">' + esc(name) + '</span>' +
        '<button type="button" class="cfg-docs-remove" title="Remove"><i class="bi bi-trash"></i></button>';
      item.querySelector('.cfg-docs-remove').addEventListener('click', function() {
        existingDocs = existingDocs.filter(function(n) { return n !== name; });
        docsToRemove.push(name);
        renderExistingDocs();
      });
      list.appendChild(item);
    });
  }

  /* ── Docs ── */
  function handleDocsFiles(files) {
    var rejected = [];
    Array.prototype.slice.call(files||[]).forEach(function(f) {
      if (!/\.md$|\.markdown$/i.test(f.name)) { rejected.push(f.name); return; }
      if (!docFiles.find(function(x){ return x.name===f.name; })) {
        docFiles.push(f);
        appendDocItem(f);
      }
    });
    document.getElementById('wz-docs-empty').style.display = docFiles.length ? 'none' : 'block';
    if (rejected.length) showAlert('Only .md files are accepted. Skipped: ' + rejected.join(', '), 'error');
  }

  document.getElementById('wz-docs-input').addEventListener('change', function(e) {
    handleDocsFiles(e.target.files);
    e.target.value = '';
  });

  (function() {
    var zone = document.getElementById('wz-docs-zone');
    if (!zone) return;
    zone.addEventListener('dragover', function(e) {
      e.preventDefault();
      e.stopPropagation();
      zone.classList.add('cfg-upload-compact--active');
    });
    zone.addEventListener('dragleave', function(e) {
      if (!zone.contains(e.relatedTarget)) {
        zone.classList.remove('cfg-upload-compact--active');
      }
    });
    zone.addEventListener('drop', function(e) {
      e.preventDefault();
      e.stopPropagation();
      zone.classList.remove('cfg-upload-compact--active');
      handleDocsFiles(e.dataTransfer && e.dataTransfer.files);
    });
  }());
  function appendDocItem(f) {
    var list = document.getElementById('wz-docs-list');
    var item = document.createElement('div');
    item.className = 'cfg-docs-item';
    item.innerHTML = '<span class="cfg-docs-icon"><i class="bi bi-file-text"></i></span>' +
      '<span class="cfg-docs-name">'+esc(f.name)+'</span>' +
      '<button type="button" class="cfg-docs-remove" title="Remove"><i class="bi bi-trash"></i></button>';
    item.querySelector('.cfg-docs-remove').addEventListener('click', function() {
      docFiles = docFiles.filter(function(x){ return x.name!==f.name; });
      item.remove();
      document.getElementById('wz-docs-empty').style.display = docFiles.length ? 'none' : 'block';
    });
    list.appendChild(item);
  }

  /* ── API content (landing page & assets) ZIP upload — edit mode only ──
     Independent of the wizard's metadata Save: uploads immediately to
     PUT /apis/{handle}/assets, mirroring the org-theming apply flow. */
  function resetContentUpload() {
    contentZipFile = null;
    var input  = document.getElementById('wz-content-input');
    var chip   = document.getElementById('wz-content-chip');
    var errEl  = document.getElementById('wz-content-error');
    var btn    = document.getElementById('wz-content-upload-btn');
    if (input) input.value = '';
    if (chip)  chip.classList.remove('visible');
    if (errEl) errEl.style.display = 'none';
    if (btn)   btn.disabled = true;
  }

  function handleContentZipFile(f) {
    var errEl = document.getElementById('wz-content-error');
    if (!f) return;
    if (!/\.zip$/i.test(f.name)) {
      if (errEl) { errEl.textContent = 'Unsupported file type. Accepted: .zip'; errEl.style.display = 'block'; }
      return;
    }
    contentZipFile = f;
    var nameEl = document.getElementById('wz-content-name');
    var chip   = document.getElementById('wz-content-chip');
    var btn    = document.getElementById('wz-content-upload-btn');
    if (nameEl) nameEl.textContent = f.name;
    if (chip)   chip.classList.add('visible');
    if (errEl)  errEl.style.display = 'none';
    if (btn)    btn.disabled = false;
  }

  async function uploadApiContent() {
    if (!contentZipFile || !editingId) return;
    var btn = document.getElementById('wz-content-upload-btn');
    var original = btn ? btn.innerHTML : '';
    if (btn) {
      btn.disabled = true;
      btn.innerHTML = '<span class="spinner-border spinner-border-sm me-1" role="status" aria-hidden="true"></span> Uploading…';
    }
    try {
      var fd = new FormData();
      fd.append('content', contentZipFile);
      var res = await fetch(window.devportalApi.root('/apis/' + encodeURIComponent(editingId) + '/assets'), {
        method: 'PUT',
        headers: { 'X-CSRF-Token': window.devportalApi.csrfToken() },
        credentials: 'same-origin',
        body: fd,
      });
      if (res.ok) {
        await showAlert('API content uploaded successfully.', 'success');
        resetContentUpload();
      } else {
        var err = await res.json().catch(function(){ return {}; });
        await showAlert('Upload failed: ' + (err.description || err.message || res.statusText), 'error');
      }
    } catch (e) {
      await showAlert('Error: ' + e.message, 'error');
    } finally {
      if (btn) { btn.innerHTML = original; btn.disabled = !contentZipFile; }
    }
  }

  (function() {
    var input  = document.getElementById('wz-content-input');
    var zone   = document.getElementById('wz-content-zone');
    var remove = document.getElementById('wz-content-remove');
    var btn    = document.getElementById('wz-content-upload-btn');
    if (input) input.addEventListener('change', function(e) {
      handleContentZipFile(e.target.files && e.target.files[0]);
    });
    if (remove) remove.addEventListener('click', resetContentUpload);
    if (btn) btn.addEventListener('click', uploadApiContent);
    if (zone) {
      zone.addEventListener('dragover', function(e) {
        e.preventDefault(); e.stopPropagation();
        zone.classList.add('cfg-upload-zone--active');
      });
      zone.addEventListener('dragleave', function(e) {
        if (!zone.contains(e.relatedTarget)) zone.classList.remove('cfg-upload-zone--active');
      });
      zone.addEventListener('drop', function(e) {
        e.preventDefault(); e.stopPropagation();
        zone.classList.remove('cfg-upload-zone--active');
        handleContentZipFile(e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files[0]);
      });
    }
  }());

  /* ── API Detail Drawer ── */
  function openDrawer(api) {
    if (!api) return;
    var ti = typeInfo(api.apiType);
    var av = document.getElementById('drw-avatar');
    av.className = 'cfg-drawer-avatar ' + ti.av;
    av.textContent = initials(api.apiName).toUpperCase();
    document.getElementById('drw-name').textContent   = api.apiName;
    document.getElementById('drw-handle').textContent = '/' + api.apiHandle;
    var metaRow = document.getElementById('drw-meta-row');
    var statusClass = api.apiStatus === 'PUBLISHED'   ? 'cfg-status-badge cfg-status-published'   :
                      api.apiStatus === 'DEPRECATED'  ? 'cfg-status-badge cfg-status-deprecated'  :
                                                        'cfg-status-badge cfg-status-draft';
    var statusLabel = api.apiStatus === 'PUBLISHED'   ? '<span class="cfg-status-dot"></span>Published'  :
                      api.apiStatus === 'DEPRECATED'  ? '<span class="cfg-status-dot"></span>Deprecated' :
                                                        '<span class="cfg-status-dot"></span>Draft';
    metaRow.innerHTML =
      '<span class="cfg-type-badge '+ti.tb+'">'+esc(ti.label)+'</span>' +
      '<span class="'+statusClass+'">'+statusLabel+'</span>';
    document.getElementById('drw-desc').textContent = api.apiDescription || '';
    var grid = document.getElementById('drw-grid');
    function row(lbl, val, mono) {
      return '<span class="cfg-drawer-lbl">'+esc(lbl)+'</span><span class="cfg-drawer-val'+(mono?' cfg-drawer-val--mono':'')+'">'+esc(val||'—')+'</span>';
    }
    var polNames = (api.subscriptionPlans||[]).map(function(p){ return typeof p === 'string' ? p : (p.planName||''); }).filter(Boolean);
    grid.innerHTML = [
      row('Version', api.apiVersion),
      row('Agent visibility', api.agentVisibility),
      row('Production URL', api.productionUrl, true),
      row('Sandbox URL', api.sandboxUrl, true),
      row('Tags', Array.isArray(api.tags) ? api.tags.join(', ') : api.tags),
      row('Plans', polNames.join(', ')),
    ].join('');
    document.getElementById('cfg-drawer-edit-btn').onclick = function() {
      document.getElementById('cfg-detail-drawer').classList.remove('open');
      showWizard(api);
    };
    document.getElementById('cfg-detail-drawer').classList.add('open');
  }
  document.getElementById('cfg-drawer-close').addEventListener('click', function() {
    document.getElementById('cfg-detail-drawer').classList.remove('open');
  });
  document.getElementById('cfg-drawer-close-foot').addEventListener('click', function() {
    document.getElementById('cfg-detail-drawer').classList.remove('open');
  });
  document.getElementById('cfg-detail-drawer').addEventListener('click', function(e) {
    if (e.target === this) this.classList.remove('open');
  });
}());
