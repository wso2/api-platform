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
 * "AS IS" BASIS, WITHOUT WARRANTIES OR ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

(function () {
    const cfg = document.getElementById('api-keys-config');
    if (!cfg) return;

    const orgId = cfg.dataset.orgId;
    const apiId = cfg.dataset.apiId;
    const readOnly = cfg.dataset.readOnly === 'true';
    const csrfToken = cfg.dataset.csrfToken || '';
    let applications = [];
    try {
        applications = JSON.parse(cfg.dataset.applications || '[]');
    } catch (e) {
        applications = [];
    }
    if (cfg.dataset.loadError === 'true') return;

    /* ── App filter ────────────────────────────────────────────── */

    const appFilter = document.getElementById('ak-app-filter');
    if (appFilter) {
        appFilter.addEventListener('change', function () {
            const url = new URL(window.location.href);
            if (appFilter.value) {
                url.searchParams.set('appId', appFilter.value);
            } else {
                url.searchParams.delete('appId');
            }
            window.location.href = url.toString();
        });
    }

    function jsonMutationHeaders() {
        const h = { 'Content-Type': 'application/json' };
        if (csrfToken) h['X-CSRF-Token'] = csrfToken;
        return h;
    }

    function expiresToIso(val) {
        if (!val || !String(val).trim()) return null;
        const d = new Date(val);
        if (Number.isNaN(d.getTime())) return null;
        return d.toISOString();
    }

    /* ── Custom modal helpers ─────────────────────────────────── */

    function akShowModal(id) {
        const el = document.getElementById(id);
        if (el) el.style.display = 'flex';
    }

    function akHideModal(id) {
        const el = document.getElementById(id);
        if (el) el.style.display = 'none';
    }

    // Close any overlay when clicking on the backdrop
    ['generateApiKeyModal', 'regenerateApiKeyModal', 'showApiKeySecretModal', 'ak-revoke-modal', 'ak-app-modal'].forEach(function (id) {
        const el = document.getElementById(id);
        if (!el) return;
        el.addEventListener('click', function (e) {
            if (e.target === el) akHideModal(id);
        });
    });

    /* ── Secret modal ─────────────────────────────────────────── */

    var _secretReloadOnClose = false;
    var _copyTimer = null;

    function showSecretModal(value, reloadOnClose, keyName) {
        _secretReloadOnClose = !!reloadOnClose;
        const codeEl = document.getElementById('api-key-secret-value');
        if (codeEl) codeEl.textContent = value || '';
        const nameEl = document.getElementById('ak-secret-key-name');
        if (nameEl) nameEl.textContent = keyName || '';
        const copyBtn = document.getElementById('btn-copy-api-key-secret');
        if (copyBtn) copyBtn.classList.remove('copy-btn--copied');
        akShowModal('showApiKeySecretModal');
    }

    function closeSecretModal() {
        akHideModal('showApiKeySecretModal');
        if (_secretReloadOnClose) {
            _secretReloadOnClose = false;
            window.location.reload();
        }
    }

    const secretDoneBtn = document.getElementById('ak-secret-done');
    if (secretDoneBtn) secretDoneBtn.addEventListener('click', closeSecretModal);
    const secretCloseBtn = document.getElementById('ak-secret-close');
    if (secretCloseBtn) secretCloseBtn.addEventListener('click', closeSecretModal);

    const copyBtn = document.getElementById('btn-copy-api-key-secret');
    if (copyBtn) {
        copyBtn.addEventListener('click', function () {
            const codeEl = document.getElementById('api-key-secret-value');
            const text = codeEl ? codeEl.textContent : '';
            if (!text) return;
            try {
                if (navigator.clipboard && navigator.clipboard.writeText) {
                    navigator.clipboard.writeText(text).catch(function () {});
                } else {
                    const ta = document.createElement('textarea');
                    ta.value = text; ta.style.position = 'fixed'; ta.style.opacity = '0';
                    document.body.appendChild(ta); ta.select(); document.execCommand('copy');
                    document.body.removeChild(ta);
                }
            } catch (e) {}
            copyBtn.classList.add('copy-btn--copied');
            if (_copyTimer) clearTimeout(_copyTimer);
            _copyTimer = setTimeout(function () {
                copyBtn.classList.remove('copy-btn--copied');
            }, 1600);
        });
    }

    /* ── Generate modal ───────────────────────────────────────── */

    function openGenModal() {
        const nameInput = document.getElementById('api-key-name');
        const expInput = document.getElementById('api-key-expires');
        if (nameInput) nameInput.value = '';
        if (expInput) expInput.value = '';
        akShowModal('generateApiKeyModal');
        setTimeout(function () { if (nameInput) nameInput.focus(); }, 60);
    }

    function closeGenModal() { akHideModal('generateApiKeyModal'); }

    const genOpenBtn = document.getElementById('btn-open-generate-api-key');
    if (genOpenBtn) genOpenBtn.addEventListener('click', openGenModal);
    const genOpenBtnEmpty = document.getElementById('btn-open-generate-api-key-empty');
    if (genOpenBtnEmpty) genOpenBtnEmpty.addEventListener('click', openGenModal);

    const genCloseBtn = document.getElementById('ak-gen-close');
    if (genCloseBtn) genCloseBtn.addEventListener('click', closeGenModal);
    const genCancelBtn = document.getElementById('ak-gen-cancel');
    if (genCancelBtn) genCancelBtn.addEventListener('click', closeGenModal);

    /* ── Regenerate modal ─────────────────────────────────────── */

    function closeRegenModal() { akHideModal('regenerateApiKeyModal'); }

    const regenCloseBtn = document.getElementById('ak-regen-close');
    if (regenCloseBtn) regenCloseBtn.addEventListener('click', closeRegenModal);
    const regenCancelBtn = document.getElementById('ak-regen-cancel');
    if (regenCancelBtn) regenCancelBtn.addEventListener('click', closeRegenModal);

    /* ── Associate app modal ──────────────────────────────────── */

    function closeAppModal() { akHideModal('ak-app-modal'); }

    const appCloseBtn = document.getElementById('ak-app-close');
    if (appCloseBtn) appCloseBtn.addEventListener('click', closeAppModal);
    const appCancelBtn = document.getElementById('ak-app-cancel');
    if (appCancelBtn) appCancelBtn.addEventListener('click', closeAppModal);

    document.querySelectorAll('.btn-app-key').forEach(function (btn) {
        btn.addEventListener('click', function () {
            if (readOnly) return;
            const keyId = btn.getAttribute('data-key-id') || '';
            const currentAppId = btn.getAttribute('data-app-id') || '';
            const keyIdInput = document.getElementById('ak-app-key-id');
            if (keyIdInput) keyIdInput.value = keyId;
            const select = document.getElementById('ak-app-select');
            if (select) {
                select.innerHTML = '<option value="">— None —</option>';
                applications.forEach(function (app) {
                    const opt = document.createElement('option');
                    opt.value = app.appId;
                    opt.textContent = app.name;
                    if (app.appId === currentAppId) opt.selected = true;
                    select.appendChild(opt);
                });
            }
            akShowModal('ak-app-modal');
        });
    });

    /* ── API requests ─────────────────────────────────────────── */

    const namePattern = /^[a-z0-9][a-z0-9_-]{0,127}$/;

    async function postGenerate(body) {
        const response = await fetch(devportalApi.org('/apis/' + encodeURIComponent(apiId) + '/api-keys/generate'), {
            method: 'POST', credentials: 'same-origin',
            headers: jsonMutationHeaders(), body: JSON.stringify(body),
        });
        const data = await response.json().catch(function () { return {}; });
        if (!response.ok) throw new Error(data.description || data.message || response.statusText || 'Request failed');
        return data;
    }

    async function postRegenerate(keyId, body) {
        const response = await fetch(devportalApi.org('/apis/' + encodeURIComponent(apiId) + '/api-keys/regenerate'), {
            method: 'POST', credentials: 'same-origin',
            headers: jsonMutationHeaders(), body: JSON.stringify(Object.assign({ keyId: keyId }, body)),
        });
        const data = await response.json().catch(function () { return {}; });
        if (!response.ok) throw new Error(data.description || data.message || response.statusText || 'Request failed');
        return data;
    }

    async function postRevoke(keyId) {
        const response = await fetch(devportalApi.org('/apis/' + encodeURIComponent(apiId) + '/api-keys/revoke'), {
            method: 'POST', credentials: 'same-origin',
            headers: jsonMutationHeaders(), body: JSON.stringify({ keyId: keyId }),
        });
        if (!response.ok) {
            const data = await response.json().catch(function () { return {}; });
            throw new Error(data.description || data.message || response.statusText || 'Request failed');
        }
    }

    async function postAssociate(keyId, appId) {
        const response = await fetch(devportalApi.org('/apis/' + encodeURIComponent(apiId) + '/api-keys/associate'), {
            method: 'POST', credentials: 'same-origin',
            headers: jsonMutationHeaders(), body: JSON.stringify({ keyId: keyId, appId: appId }),
        });
        if (!response.ok) {
            const data = await response.json().catch(function () { return {}; });
            throw new Error(data.description || data.message || response.statusText || 'Request failed');
        }
    }

    async function postDissociate(keyId) {
        const response = await fetch(devportalApi.org('/apis/' + encodeURIComponent(apiId) + '/api-keys/dissociate'), {
            method: 'POST', credentials: 'same-origin',
            headers: jsonMutationHeaders(), body: JSON.stringify({ keyId: keyId }),
        });
        if (!response.ok) {
            const data = await response.json().catch(function () { return {}; });
            throw new Error(data.description || data.message || response.statusText || 'Request failed');
        }
    }

    /* ── Associate app submit ─────────────────────────────────── */

    const submitAppBtn = document.getElementById('btn-submit-app-association');
    if (submitAppBtn) {
        submitAppBtn.addEventListener('click', async function () {
            if (submitAppBtn.disabled || submitAppBtn.dataset.loading === 'true') return;
            const keyId = document.getElementById('ak-app-key-id')?.value || '';
            const select = document.getElementById('ak-app-select');
            const appId = select ? select.value : '';
            submitAppBtn.dataset.loading = 'true';
            submitAppBtn.disabled = true;
            try {
                if (appId) {
                    await postAssociate(keyId, appId);
                } else {
                    await postDissociate(keyId);
                }
                closeAppModal();
                window.location.reload();
            } catch (e) {
                if (typeof showAlert === 'function') await showAlert(e.message || 'Failed to update app association', 'error');
            } finally {
                submitAppBtn.disabled = false;
                delete submitAppBtn.dataset.loading;
            }
        });
    }

    /* ── Generate submit ──────────────────────────────────────── */

    const submitGenBtn = document.getElementById('btn-submit-generate-api-key');
    if (submitGenBtn) {
        submitGenBtn.addEventListener('click', async function () {
            if (submitGenBtn.disabled || submitGenBtn.dataset.loading === 'true') return;
            const nameInput = document.getElementById('api-key-name');
            const expInput = document.getElementById('api-key-expires');
            const name = (nameInput && nameInput.value) ? nameInput.value.trim() : '';
            if (!namePattern.test(name)) {
                if (typeof showAlert === 'function') await showAlert('Enter a valid name: start with a letter or number, then up to 128 URL-safe characters.', 'error');
                return;
            }
            const body = { name: name };
            const iso = expInput ? expiresToIso(expInput.value) : null;
            if (iso) body.expiresAt = iso;
            submitGenBtn.dataset.loading = 'true';
            submitGenBtn.disabled = true;
            let data;
            try {
                data = await postGenerate(body);
                closeGenModal();
                if (nameInput) nameInput.value = '';
                if (expInput) expInput.value = '';
            } catch (e) {
                if (typeof showAlert === 'function') await showAlert(e.message || 'Failed to generate API key', 'error');
            } finally {
                submitGenBtn.disabled = false;
                delete submitGenBtn.dataset.loading;
            }
            if (data && data.key) {
                showSecretModal(data.key, true, name);
            } else if (data) {
                window.location.reload();
            }
        });
    }

    /* ── Regenerate buttons ───────────────────────────────────── */

    document.querySelectorAll('.btn-regenerate-key').forEach(function (btn) {
        btn.addEventListener('click', function () {
            if (readOnly) return;
            const keyId = btn.getAttribute('data-key-id') || '';
            const keyName = btn.getAttribute('data-key-name') || keyId;
            const keyIdInput = document.getElementById('regenerate-key-id');
            if (keyIdInput) keyIdInput.value = keyId;
            const nameField = document.getElementById('regenerate-api-key-name');
            if (nameField) nameField.value = keyName;
            const expField = document.getElementById('regenerate-api-key-expires');
            if (expField) expField.value = '';
            akShowModal('regenerateApiKeyModal');
        });
    });

    /* ── Regenerate submit ────────────────────────────────────── */

    const submitRegenBtn = document.getElementById('btn-submit-regenerate-api-key');
    if (submitRegenBtn) {
        submitRegenBtn.addEventListener('click', async function () {
            if (submitRegenBtn.disabled || submitRegenBtn.dataset.loading === 'true') return;
            const keyId = document.getElementById('regenerate-key-id')?.value || '';
            const nameField = document.getElementById('regenerate-api-key-name');
            const expField = document.getElementById('regenerate-api-key-expires');
            const name = (nameField && nameField.value) ? nameField.value.trim() : '';
            const body = {};
            const iso = expField ? expiresToIso(expField.value) : null;
            if (iso) body.expiresAt = iso;
            submitRegenBtn.dataset.loading = 'true';
            submitRegenBtn.disabled = true;
            let data;
            try {
                data = await postRegenerate(keyId, body);
                closeRegenModal();
            } catch (e) {
                if (typeof showAlert === 'function') await showAlert(e.message || 'Failed to regenerate API key', 'error');
            } finally {
                submitRegenBtn.disabled = false;
                delete submitRegenBtn.dataset.loading;
            }
            if (data && data.key) {
                showSecretModal(data.key, true, name);
            } else if (data) {
                window.location.reload();
            }
        });
    }

    /* ── Revoke modal ─────────────────────────────────────────── */

    var _pendingRevokeKeyId = null;

    document.querySelectorAll('.btn-revoke-key').forEach(function (btn) {
        btn.addEventListener('click', function () {
            if (readOnly) return;
            const keyId = btn.getAttribute('data-key-id') || '';
            const keyName = btn.getAttribute('data-key-name') || keyId;
            if (!keyId) return;
            _pendingRevokeKeyId = keyId;
            const nameEl = document.getElementById('ak-revoke-name');
            if (nameEl) nameEl.textContent = keyName;
            const confirmBtn = document.getElementById('ak-revoke-confirm');
            if (confirmBtn) { confirmBtn.disabled = false; confirmBtn.textContent = 'Revoke key'; }
            akShowModal('ak-revoke-modal');
        });
    });

    const revokeCancelBtn = document.getElementById('ak-revoke-cancel');
    if (revokeCancelBtn) revokeCancelBtn.addEventListener('click', function () { akHideModal('ak-revoke-modal'); });

    const revokeConfirmBtn = document.getElementById('ak-revoke-confirm');
    if (revokeConfirmBtn) {
        revokeConfirmBtn.addEventListener('click', async function () {
            if (!_pendingRevokeKeyId) return;
            const keyId = _pendingRevokeKeyId;
            revokeConfirmBtn.disabled = true;
            revokeConfirmBtn.textContent = 'Revoking…';
            try {
                await postRevoke(keyId);
                if (typeof showAlert === 'function') await showAlert('API key revoked.', 'success');
                window.location.reload();
            } catch (e) {
                if (typeof showAlert === 'function') await showAlert(e.message || 'Failed to revoke API key', 'error');
                revokeConfirmBtn.disabled = false;
                revokeConfirmBtn.textContent = 'Revoke key';
            } finally {
                _pendingRevokeKeyId = null;
            }
        });
    }

    // Backward-compat shim in case any external code still calls executeRevokeApiKey
    window.executeRevokeApiKey = async function () {
        if (revokeConfirmBtn) revokeConfirmBtn.click();
    };

}());
