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
    const cfg = document.getElementById('application-api-keys-config');
    if (!cfg) return;

    const orgId = cfg.dataset.orgId;
    const appId = cfg.dataset.appId;
    let keysByApi = [];
    try {
        keysByApi = JSON.parse(cfg.dataset.availableKeysByApi || '[]');
    } catch (e) {
        keysByApi = [];
    }

    function jsonMutationHeaders() {
        return { 'Content-Type': 'application/json', 'X-CSRF-Token': window.devportalApi.csrfToken() };
    }

    function showModal(id) {
        const el = document.getElementById(id);
        if (el) el.style.display = 'flex';
    }

    function hideModal(id) {
        const el = document.getElementById(id);
        if (el) el.style.display = 'none';
    }

    document.getElementById('associate-key-modal')?.addEventListener('click', function (e) {
        if (e.target === e.currentTarget) hideModal('associate-key-modal');
    });

    /* ── Open / close ──────────────────────────────────────────── */

    const openBtn = document.getElementById('btn-open-associate-key');
    const apiSelect = document.getElementById('associate-key-api-select');
    const keySelect = document.getElementById('associate-key-select');
    const submitBtn = document.getElementById('btn-submit-associate-key');

    function populateApiSelect() {
        if (!apiSelect) return;
        apiSelect.innerHTML = '<option value="">— Select an API —</option>';
        keysByApi.forEach(function (entry) {
            const opt = document.createElement('option');
            opt.value = entry.apiId;
            opt.textContent = entry.apiName;
            apiSelect.appendChild(opt);
        });
    }
    populateApiSelect();

    function resetModal() {
        if (apiSelect) apiSelect.value = '';
        if (keySelect) {
            keySelect.innerHTML = '<option value="">— Select an API first —</option>';
            keySelect.disabled = true;
        }
        if (submitBtn) submitBtn.disabled = true;
    }

    if (openBtn) {
        openBtn.addEventListener('click', function () {
            resetModal();
            showModal('associate-key-modal');
        });
    }

    const closeBtn = document.getElementById('associate-key-close');
    if (closeBtn) closeBtn.addEventListener('click', function () { hideModal('associate-key-modal'); });
    const cancelBtn = document.getElementById('associate-key-cancel');
    if (cancelBtn) cancelBtn.addEventListener('click', function () { hideModal('associate-key-modal'); });

    /* ── Cascading API -> key select ───────────────────────────── */

    if (apiSelect) {
        apiSelect.addEventListener('change', function () {
            const entry = keysByApi.find(function (a) { return a.apiId === apiSelect.value; });
            if (!keySelect) return;
            if (!entry) {
                keySelect.innerHTML = '<option value="">— Select an API first —</option>';
                keySelect.disabled = true;
                if (submitBtn) submitBtn.disabled = true;
                return;
            }
            keySelect.innerHTML = '<option value="">— Select a key —</option>';
            entry.keys.forEach(function (k) {
                const opt = document.createElement('option');
                opt.value = k.id;
                opt.textContent = k.displayName;
                keySelect.appendChild(opt);
            });
            keySelect.disabled = false;
            if (submitBtn) submitBtn.disabled = true;
        });
    }

    if (keySelect) {
        keySelect.addEventListener('change', function () {
            if (submitBtn) submitBtn.disabled = !keySelect.value;
        });
    }

    /* ── Submit associate ──────────────────────────────────────── */

    if (submitBtn) {
        submitBtn.addEventListener('click', async function () {
            if (submitBtn.disabled || submitBtn.dataset.loading === 'true' || !keySelect || !keySelect.value) return;
            submitBtn.dataset.loading = 'true';
            submitBtn.disabled = true;
            try {
                const selectedApiId = apiSelect ? apiSelect.value : '';
                const response = await fetch(devportalApi.root('/apis/' + encodeURIComponent(selectedApiId) + '/api-keys/associate'), {
                    method: 'POST', credentials: 'same-origin',
                    headers: jsonMutationHeaders(), body: JSON.stringify({ keyId: keySelect.value, appId: appId }),
                });
                if (!response.ok) {
                    const data = await response.json().catch(function () { return {}; });
                    throw new Error(data.description || data.message || response.statusText || 'Request failed');
                }
                hideModal('associate-key-modal');
                window.location.reload();
            } catch (e) {
                if (typeof showAlert === 'function') await showAlert(e.message || 'Failed to associate key', 'error');
                submitBtn.disabled = false;
            } finally {
                delete submitBtn.dataset.loading;
            }
        });
    }

    /* ── Remove association ───────────────────────────────────── */

    document.querySelectorAll('.btn-remove-key-association').forEach(function (btn) {
        btn.addEventListener('click', async function () {
            if (btn.disabled) return;
            const keyId = btn.getAttribute('data-key-id') || '';
            const keyApiId = btn.getAttribute('data-api-id') || '';
            if (!keyId || !keyApiId) return;
            btn.disabled = true;
            try {
                const response = await fetch(devportalApi.root('/apis/' + encodeURIComponent(keyApiId) + '/api-keys/dissociate'), {
                    method: 'POST', credentials: 'same-origin',
                    headers: jsonMutationHeaders(), body: JSON.stringify({ keyId: keyId }),
                });
                if (!response.ok) {
                    const data = await response.json().catch(function () { return {}; });
                    throw new Error(data.description || data.message || response.statusText || 'Request failed');
                }
                window.location.reload();
            } catch (e) {
                if (typeof showAlert === 'function') await showAlert(e.message || 'Failed to remove association', 'error');
                btn.disabled = false;
            }
        });
    });
}());
