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
    const cfg = document.getElementById('platform-api-keys-config');
    if (!cfg) {
        return;
    }

    const orgId = cfg.dataset.orgId;
    const apiId = cfg.dataset.apiId;
    const readOnly = cfg.dataset.readOnly === 'true';
    const csrfToken = cfg.dataset.csrfToken || '';
    if (cfg.dataset.loadError === 'true') {
        return;
    }

    function jsonMutationHeaders() {
        const h = { 'Content-Type': 'application/json' };
        if (csrfToken) {
            h['X-CSRF-Token'] = csrfToken;
        }
        return h;
    }

    function expiresToIso(val) {
        if (!val || !String(val).trim()) {
            return null;
        }
        const d = new Date(val);
        if (Number.isNaN(d.getTime())) {
            return null;
        }
        return d.toISOString();
    }

    function showSecretModal(value, reloadOnClose) {
        const input = document.getElementById('platform-api-key-secret-value');
        const modalEl = document.getElementById('showPlatformApiKeySecretModal');
        if (!input || !modalEl || typeof bootstrap === 'undefined') {
            if (typeof showAlert === 'function') {
                showAlert('API key: ' + value, 'success');
            }
            if (reloadOnClose) {
                window.location.reload();
            }
            return;
        }
        input.value = value;
        if (reloadOnClose) {
            const onHidden = function () {
                modalEl.removeEventListener('hidden.bs.modal', onHidden);
                window.location.reload();
            };
            modalEl.addEventListener('hidden.bs.modal', onHidden);
        }
        const modal = bootstrap.Modal.getOrCreateInstance(modalEl);
        modal.show();
        const copyBtn = document.getElementById('btn-copy-platform-api-key-secret');
        if (copyBtn) {
            copyBtn.onclick = function () {
                input.select();
                document.execCommand('copy');
                if (typeof showAlert === 'function') {
                    showAlert('Copied to clipboard', 'success');
                }
            };
        }
    }

    async function postGenerate(body) {
        const response = await fetch(
            '/devportal/organizations/' + encodeURIComponent(orgId) + '/platform-api-keys/generate',
            {
                method: 'POST',
                credentials: 'same-origin',
                headers: jsonMutationHeaders(),
                body: JSON.stringify(body)
            }
        );
        const data = await response.json().catch(function () { return {}; });
        if (!response.ok) {
            const msg = data.description || data.message || response.statusText || 'Request failed';
            throw new Error(msg);
        }
        return data;
    }

    async function postRegenerate(keyId, body) {
        const response = await fetch(
            '/devportal/organizations/' + encodeURIComponent(orgId) + '/platform-api-keys/' + encodeURIComponent(keyId) + '/regenerate',
            {
                method: 'POST',
                credentials: 'same-origin',
                headers: jsonMutationHeaders(),
                body: JSON.stringify(body)
            }
        );
        const data = await response.json().catch(function () { return {}; });
        if (!response.ok) {
            const msg = data.description || data.message || response.statusText || 'Request failed';
            throw new Error(msg);
        }
        return data;
    }

    async function postRevoke(keyId) {
        const url = '/devportal/organizations/' + encodeURIComponent(orgId) + '/platform-api-keys/' + encodeURIComponent(keyId) + '/revoke?apiId=' + encodeURIComponent(apiId);
        const response = await fetch(url, {
            method: 'POST',
            credentials: 'same-origin',
            headers: jsonMutationHeaders()
        });
        if (!response.ok) {
            const data = await response.json().catch(function () { return {}; });
            const msg = data.description || data.message || response.statusText || 'Request failed';
            throw new Error(msg);
        }
    }

    const namePattern = /^[a-z0-9][a-z0-9_-]{0,127}$/;

    document.getElementById('btn-submit-generate-platform-api-key')?.addEventListener('click', async function () {
        const submitBtn = document.getElementById('btn-submit-generate-platform-api-key');
        const nameInput = document.getElementById('platform-api-key-name');
        const expInput = document.getElementById('platform-api-key-expires');
        const name = (nameInput && nameInput.value) ? nameInput.value.trim() : '';
        if (!namePattern.test(name)) {
            if (typeof showAlert === 'function') {
                await showAlert('Enter a valid name: start with a letter or number, then up to 128 URL-safe characters.', 'error');
            }
            return;
        }
        if (submitBtn && (submitBtn.disabled || submitBtn.dataset.loading === 'true')) {
            return;
        }
        const body = { apiId: apiId, name: name };
        const iso = expInput ? expiresToIso(expInput.value) : null;
        if (iso) {
            body.expiresAt = iso;
        }
        let data;
        if (submitBtn) {
            submitBtn.dataset.loading = 'true';
            submitBtn.disabled = true;
        }
        try {
            data = await postGenerate(body);
            const modalEl = document.getElementById('generatePlatformApiKeyModal');
            if (modalEl && typeof bootstrap !== 'undefined') {
                const m = bootstrap.Modal.getInstance(modalEl);
                if (m) {
                    m.hide();
                }
            }
            if (nameInput) {
                nameInput.value = '';
            }
            if (expInput) {
                expInput.value = '';
            }
        } catch (e) {
            if (typeof showAlert === 'function') {
                await showAlert(e.message || 'Failed to generate API key', 'error');
            }
        } finally {
            if (submitBtn) {
                submitBtn.disabled = false;
                delete submitBtn.dataset.loading;
            }
        }
        if (data && data.key) {
            showSecretModal(data.key, true);
        } else if (data) {
            window.location.reload();
        }
    });

    document.querySelectorAll('.btn-regenerate-key').forEach(function (btn) {
        btn.addEventListener('click', function () {
            if (readOnly || btn.dataset.loading === 'true') {
                return;
            }
            const keyId = btn.getAttribute('data-key-id') || '';
            const keyName = btn.getAttribute('data-key-name') || keyId;
            document.getElementById('regenerate-platform-key-id').value = keyId;
            const nameField = document.getElementById('regenerate-platform-api-key-name');
            if (nameField) {
                nameField.value = keyName;
            }
            const expField = document.getElementById('regenerate-platform-api-key-expires');
            if (expField) {
                expField.value = '';
            }
            const modalEl = document.getElementById('regeneratePlatformApiKeyModal');
            if (modalEl && typeof bootstrap !== 'undefined') {
                btn.dataset.loading = 'true';
                btn.disabled = true;
                const onShown = function () {
                    modalEl.removeEventListener('shown.bs.modal', onShown);
                    btn.disabled = false;
                    delete btn.dataset.loading;
                };
                modalEl.addEventListener('shown.bs.modal', onShown);
                try {
                    bootstrap.Modal.getOrCreateInstance(modalEl).show();
                } catch (err) {
                    modalEl.removeEventListener('shown.bs.modal', onShown);
                    btn.disabled = false;
                    delete btn.dataset.loading;
                }
            }
        });
    });

    document.getElementById('btn-submit-regenerate-platform-api-key')?.addEventListener('click', async function () {
        const submitBtn = document.getElementById('btn-submit-regenerate-platform-api-key');
        const keyId = document.getElementById('regenerate-platform-key-id')?.value || '';
        const nameField = document.getElementById('regenerate-platform-api-key-name');
        const expField = document.getElementById('regenerate-platform-api-key-expires');
        const name = (nameField && nameField.value) ? nameField.value.trim() : '';
        if (!namePattern.test(name)) {
            if (typeof showAlert === 'function') {
                await showAlert('Enter a valid name for the new key.', 'error');
            }
            return;
        }
        if (submitBtn && (submitBtn.disabled || submitBtn.dataset.loading === 'true')) {
            return;
        }
        const body = { apiId: apiId, name: name };
        const iso = expField ? expiresToIso(expField.value) : null;
        if (iso) {
            body.expiresAt = iso;
        }
        let data;
        if (submitBtn) {
            submitBtn.dataset.loading = 'true';
            submitBtn.disabled = true;
        }
        try {
            data = await postRegenerate(keyId, body);
            const modalEl = document.getElementById('regeneratePlatformApiKeyModal');
            if (modalEl && typeof bootstrap !== 'undefined') {
                const m = bootstrap.Modal.getInstance(modalEl);
                if (m) {
                    m.hide();
                }
            }
        } catch (e) {
            if (typeof showAlert === 'function') {
                await showAlert(e.message || 'Failed to regenerate API key', 'error');
            }
        } finally {
            if (submitBtn) {
                submitBtn.disabled = false;
                delete submitBtn.dataset.loading;
            }
        }
        if (data && data.key) {
            showSecretModal(data.key, true);
        } else if (data) {
            window.location.reload();
        }
    });

    document.querySelectorAll('.btn-revoke-key').forEach(function (btn) {
        btn.addEventListener('click', async function () {
            if (readOnly || btn.dataset.loading === 'true') {
                return;
            }
            const keyId = btn.getAttribute('data-key-id') || '';
            if (!keyId) {
                return;
            }
            if (!confirm('Revoke this API key? Clients using it will fail immediately.')) {
                return;
            }
            btn.dataset.loading = 'true';
            btn.disabled = true;
            try {
                await postRevoke(keyId);
                if (typeof showAlert === 'function') {
                    await showAlert('API key revoked.', 'success');
                }
                window.location.reload();
            } catch (e) {
                if (typeof showAlert === 'function') {
                    await showAlert(e.message || 'Failed to revoke API key', 'error');
                }
            } finally {
                btn.disabled = false;
                delete btn.dataset.loading;
            }
        });
    });
})();
