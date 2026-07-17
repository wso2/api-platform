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

// Global "API Keys" page. The list is rendered server-side; this script handles inline
// per-key management using the SAME modals as the per-API API Keys page
// (regenerateApiKeyModal, showApiKeySecretModal, ak-revoke-modal, ak-app-modal). It reads
// each row's data-* attributes and calls the per-API or per-MCP-server key endpoints,
// then reloads on success. MCP-server keys are routed to /mcp-servers/{handle}/api-keys/...
// — the /apis/{apiId} path only resolves non-MCP APIs.
(function () {
    var root = document.getElementById('api-keys-overview-root');
    if (!root) return;
    var readOnly = root.dataset.readOnly === 'true';

    var _manageKey = null; // key whose row action is currently in progress
    var _apps = null;      // cached applications list (fetched lazily on first associate)
    var _copyTimer = null;
    var _actionInFlight = false;

    /* ── helpers ──────────────────────────────────────────────── */

    function mutationHeaders() {
        return { 'Content-Type': 'application/json', 'X-CSRF-Token': window.devportalApi.csrfToken() };
    }
    function show(id) { var el = document.getElementById(id); if (el) el.style.display = 'flex'; }
    function hide(id) { var el = document.getElementById(id); if (el) el.style.display = 'none'; }
    async function alertMsg(message, type) {
        if (typeof showAlert === 'function') { try { await showAlert(message, type); } catch (e) { /* noop */ } }
    }
    function isMcp(key) {
        return String((key && key.apiType) || '').toUpperCase() === 'MCP';
    }
    function expiresToIso(val) {
        if (!val || !String(val).trim()) return null;
        var d = new Date(val);
        if (isNaN(d.getTime())) return null;
        return d.toISOString();
    }

    // Resolve the acting key from the clicked button's row. Keys are identified by their
    // handle (the external id); the uuid is never rendered or used here.
    function setActingKey(btn) {
        var tr = btn.closest ? btn.closest('tr') : null;
        var d = tr && tr.dataset;
        if (!d || !d.keyHandle) return false;
        _manageKey = {
            handle: d.keyHandle,   // what the mutation endpoints expect as `keyId`
            apiId: d.apiId,
            apiType: d.apiType,
            displayName: d.keyName || '',
            status: d.status || '',
            appId: d.appId || '',
            appDisplayName: d.appDisplayName || '',
        };
        return true;
    }

    // MCP servers expose an identical but separately-rooted key API
    // (/mcp-servers/{handle}/api-keys/...). Pick the base from the key's apiType.
    function keyEndpoint(suffix) {
        var base = isMcp(_manageKey) ? '/mcp-servers/' : '/apis/';
        return devportalApi.root(base + encodeURIComponent(_manageKey.apiId) + '/api-keys/' + suffix);
    }

    /* ── regenerate modal ─────────────────────────────────────── */

    function openRegenerate() {
        document.getElementById('regenerate-key-id').value = _manageKey.handle;
        document.getElementById('regenerate-api-key-name').value = _manageKey.displayName || '';
        var exp = document.getElementById('regenerate-api-key-expires');
        if (exp) exp.value = '';
        show('regenerateApiKeyModal');
    }

    async function submitRegenerate() {
        if (!_manageKey || _actionInFlight) return;
        var key = _manageKey;
        var expInput = document.getElementById('regenerate-api-key-expires');
        var body = { keyId: key.handle };
        var iso = expInput ? expiresToIso(expInput.value) : null;
        if (iso) body.expiresAt = iso;
        _actionInFlight = true;
        var btn = document.getElementById('btn-submit-regenerate-api-key');
        btn.disabled = true;
        try {
            var resp = await fetch(keyEndpoint('regenerate'), {
                method: 'POST', credentials: 'same-origin',
                headers: mutationHeaders(), body: JSON.stringify(body),
            });
            var data = await resp.json().catch(function () { return {}; });
            if (!resp.ok) throw new Error(data.description || data.message || 'Failed to regenerate API key');
            hide('regenerateApiKeyModal');
            showSecret(data.key || '', key.displayName || '');
        } catch (e) {
            await alertMsg(e.message || 'Failed to regenerate API key', 'error');
        } finally {
            _actionInFlight = false;
            btn.disabled = false;
        }
    }

    /* ── revoke modal ─────────────────────────────────────────── */

    function openRevoke() {
        document.getElementById('ak-revoke-name').textContent = _manageKey.displayName || '';
        var confirmBtn = document.getElementById('ak-revoke-confirm');
        if (confirmBtn) { confirmBtn.disabled = false; confirmBtn.textContent = 'Revoke key'; }
        show('ak-revoke-modal');
    }

    async function submitRevoke() {
        if (!_manageKey || _actionInFlight) return;
        var key = _manageKey;
        _actionInFlight = true;
        var btn = document.getElementById('ak-revoke-confirm');
        btn.disabled = true; btn.textContent = 'Revoking…';
        try {
            var resp = await fetch(keyEndpoint('revoke'), {
                method: 'POST', credentials: 'same-origin',
                headers: mutationHeaders(), body: JSON.stringify({ keyId: key.handle }),
            });
            if (!resp.ok) {
                var data = await resp.json().catch(function () { return {}; });
                throw new Error(data.description || data.message || 'Failed to revoke API key');
            }
            hide('ak-revoke-modal');
            await alertMsg('API key revoked.', 'success');
            window.location.reload();
        } catch (e) {
            await alertMsg(e.message || 'Failed to revoke API key', 'error');
            _actionInFlight = false;
            btn.disabled = false; btn.textContent = 'Revoke key';
        }
    }

    /* ── associate app modal ──────────────────────────────────── */

    async function loadApps() {
        if (_apps) return _apps;
        try {
            var resp = await fetch(devportalApi.root('/applications'), { headers: mutationHeaders() });
            if (!resp.ok) return [];
            var data = await resp.json();
            // The associate endpoint expects the app handle, returned as `id` here.
            _apps = ((data && data.list) || []).map(function (a) { return { appId: a.id, displayName: a.displayName }; });
        } catch (e) {
            return [];
        }
        return _apps;
    }

    async function openAppModal() {
        if (!_manageKey) return;
        var keyIdInput = document.getElementById('ak-app-key-id');
        if (keyIdInput) keyIdInput.value = _manageKey.handle;
        var apps = await loadApps();
        var select = document.getElementById('ak-app-select');
        select.innerHTML = '<option value="">— None —</option>';
        apps.forEach(function (app) {
            var opt = document.createElement('option');
            opt.value = app.appId;
            opt.textContent = app.displayName;
            if (app.appId === _manageKey.appId) opt.selected = true;
            select.appendChild(opt);
        });
        show('ak-app-modal');
    }

    async function saveAppAssociation() {
        if (!_manageKey || _actionInFlight) return;
        var key = _manageKey;
        var select = document.getElementById('ak-app-select');
        var appId = select ? select.value : '';
        _actionInFlight = true;
        var btn = document.getElementById('btn-submit-app-association');
        btn.disabled = true;
        try {
            var resp = await fetch(keyEndpoint(appId ? 'associate' : 'dissociate'), {
                method: 'POST', credentials: 'same-origin',
                headers: mutationHeaders(),
                body: JSON.stringify(appId ? { keyId: key.handle, appId: appId } : { keyId: key.handle }),
            });
            if (!resp.ok) {
                var data = await resp.json().catch(function () { return {}; });
                throw new Error(data.description || data.message || 'Failed to update app association');
            }
            hide('ak-app-modal');
            window.location.reload();
        } catch (e) {
            await alertMsg(e.message || 'Failed to update app association', 'error');
            _actionInFlight = false;
            btn.disabled = false;
        }
    }

    /* ── secret modal ─────────────────────────────────────────── */

    function showSecret(value, keyName) {
        document.getElementById('api-key-secret-value').textContent = value || '';
        document.getElementById('ak-secret-key-name').textContent = keyName || '';
        var copyBtn = document.getElementById('btn-copy-api-key-secret');
        if (copyBtn) copyBtn.classList.remove('copy-btn--copied');
        show('showApiKeySecretModal');
    }

    function closeSecret() {
        hide('showApiKeySecretModal');
        window.location.reload(); // refresh the list after regeneration
    }

    function copySecret() {
        var codeEl = document.getElementById('api-key-secret-value');
        var text = codeEl ? codeEl.textContent : '';
        if (!text) return;
        try {
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(text).catch(function () {});
            } else {
                var ta = document.createElement('textarea');
                ta.value = text; ta.style.cssText = 'position:fixed;opacity:0';
                document.body.appendChild(ta); ta.select(); document.execCommand('copy');
                document.body.removeChild(ta);
            }
        } catch (e) { /* noop */ }
        var btn = document.getElementById('btn-copy-api-key-secret');
        if (btn) {
            btn.classList.add('copy-btn--copied');
            if (_copyTimer) clearTimeout(_copyTimer);
            _copyTimer = setTimeout(function () { btn.classList.remove('copy-btn--copied'); }, 1600);
        }
    }

    /* ── wiring ───────────────────────────────────────────────── */

    function backdrop(id) {
        var el = document.getElementById(id);
        if (el) el.addEventListener('click', function (e) { if (e.target === el) hide(id); });
    }

    function wire() {
        if (readOnly) return;

        // Row action buttons (server-rendered).
        document.querySelectorAll('.ak-row-app').forEach(function (btn) {
            btn.addEventListener('click', function () { if (btn.disabled || !setActingKey(btn)) return; openAppModal(); });
        });
        document.querySelectorAll('.ak-row-regen').forEach(function (btn) {
            btn.addEventListener('click', function () { if (btn.disabled || !setActingKey(btn)) return; openRegenerate(); });
        });
        document.querySelectorAll('.ak-row-revoke').forEach(function (btn) {
            btn.addEventListener('click', function () { if (btn.disabled || !setActingKey(btn)) return; openRevoke(); });
        });

        // Regenerate modal
        var regClose = document.getElementById('ak-regen-close');
        if (regClose) regClose.addEventListener('click', function () { hide('regenerateApiKeyModal'); });
        var regCancel = document.getElementById('ak-regen-cancel');
        if (regCancel) regCancel.addEventListener('click', function () { hide('regenerateApiKeyModal'); });
        var regSubmit = document.getElementById('btn-submit-regenerate-api-key');
        if (regSubmit) regSubmit.addEventListener('click', submitRegenerate);
        backdrop('regenerateApiKeyModal');

        // Revoke modal
        var revCancel = document.getElementById('ak-revoke-cancel');
        if (revCancel) revCancel.addEventListener('click', function () { hide('ak-revoke-modal'); });
        var revConfirm = document.getElementById('ak-revoke-confirm');
        if (revConfirm) revConfirm.addEventListener('click', submitRevoke);
        backdrop('ak-revoke-modal');

        // Associate app modal
        var appClose = document.getElementById('ak-app-close');
        if (appClose) appClose.addEventListener('click', function () { hide('ak-app-modal'); });
        var appCancel = document.getElementById('ak-app-cancel');
        if (appCancel) appCancel.addEventListener('click', function () { hide('ak-app-modal'); });
        var appSubmit = document.getElementById('btn-submit-app-association');
        if (appSubmit) appSubmit.addEventListener('click', saveAppAssociation);
        backdrop('ak-app-modal');

        // Secret modal
        var secDone = document.getElementById('ak-secret-done');
        if (secDone) secDone.addEventListener('click', closeSecret);
        var secClose = document.getElementById('ak-secret-close');
        if (secClose) secClose.addEventListener('click', closeSecret);
        var secCopy = document.getElementById('btn-copy-api-key-secret');
        if (secCopy) secCopy.addEventListener('click', copySecret);
        var secretModal = document.getElementById('showApiKeySecretModal');
        if (secretModal) secretModal.addEventListener('click', function (e) { if (e.target === secretModal) closeSecret(); });
    }

    function init() { wire(); }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
}());
