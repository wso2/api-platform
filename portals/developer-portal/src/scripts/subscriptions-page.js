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

/* ── State ─────────────────────────────────────────────────────── */
const tokenCache = {};
let _manageSub = null;
let _manageRevealed = false;
let _manageCopyTimer = null;
let _regenerateInFlight = false;

/* ── Open / close manage modal ─────────────────────────────────── */
function openSubManage(subId) {
    const row = document.getElementById('sub-row-' + subId);
    if (!row) return;
    _manageSub = {
        id: subId,
        apiName: row.dataset.apiName || '',
        planName: row.dataset.planName || '',
        status: row.dataset.status || 'ACTIVE',
    };
    _manageRevealed = false;

    document.getElementById('subManageApiName').textContent = _manageSub.apiName;
    document.getElementById('subManagePlanName').textContent = _manageSub.planName;
    updateSubManageStatus(_manageSub.status);
    document.getElementById('subManageTokenVal').textContent = '•'.repeat(28);
    const revealIcon = document.getElementById('subManageRevealIcon');
    if (revealIcon) revealIcon.className = 'bi bi-eye';
    resetSubManageCopyBtn();
    document.getElementById('subManageModal').style.display = 'flex';
}

function closeSubManage() {
    const modal = document.getElementById('subManageModal');
    if (modal) modal.style.display = 'none';
    _manageSub = null;
    _manageRevealed = false;
}

function updateSubManageStatus(status) {
    const isSuspended = status === 'INACTIVE';
    const badge = document.getElementById('subManageStatusBadge');
    if (badge) {
        badge.textContent = isSuspended ? 'Suspended' : 'Active';
        badge.className = 'mc-manage-status-badge ' + (isSuspended ? 'mc-manage-status-badge--suspended' : 'mc-manage-status-badge--active');
    }
    const btn = document.getElementById('subManageSuspendBtn');
    if (btn) {
        btn.querySelector('i').className = 'bi bi-' + (isSuspended ? 'play-circle' : 'pause-circle');
        btn.querySelector('.sub-suspend-label').textContent = isSuspended ? 'Resume' : 'Suspend';
        btn.className = 'dp-btn mc-manage-action-btn ' + (isSuspended ? 'mc-manage-action-btn--resume' : 'mc-manage-action-btn--suspend');
    }
}

function resetSubManageCopyBtn() {
    const btn = document.getElementById('subManageCopyBtn');
    if (btn) btn.classList.remove('copy-btn--copied');
}

/* ── Token fetch / reveal / copy ───────────────────────────────── */
async function fetchSubToken(subId) {
    if (tokenCache[subId]) return tokenCache[subId];
    const orgId = window.__subscriptionOrgId;
    if (!orgId) return null;
    try {
        const resp = await fetch(
            devportalApi.root(`/subscriptions/${encodeURIComponent(subId)}`),
            { headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.devportalApi.csrfToken() } }
        );
        if (!resp.ok) return null;
        const data = await resp.json();
        const token = data.subscriptionToken || null;
        if (token) tokenCache[subId] = token;
        return token;
    } catch (_) { return null; }
}

async function revealSubToken() {
    if (!_manageSub) return;
    if (_manageRevealed) {
        _manageRevealed = false;
        document.getElementById('subManageTokenVal').textContent = '•'.repeat(28);
        const ri = document.getElementById('subManageRevealIcon');
        if (ri) ri.className = 'bi bi-eye';
        return;
    }
    const token = await fetchSubToken(_manageSub.id);
    if (!token) return;
    _manageRevealed = true;
    document.getElementById('subManageTokenVal').textContent = token;
    const ri = document.getElementById('subManageRevealIcon');
    if (ri) ri.className = 'bi bi-eye-slash';
}

async function copySubToken() {
    if (!_manageSub) return;
    const token = await fetchSubToken(_manageSub.id);
    if (!token) return;
    try {
        await navigator.clipboard.writeText(token);
    } catch (_) {
        const ta = document.createElement('textarea');
        ta.value = token;
        ta.style.cssText = 'position:fixed;opacity:0';
        document.body.appendChild(ta);
        ta.select();
        document.execCommand('copy');
        document.body.removeChild(ta);
    }
    const btn = document.getElementById('subManageCopyBtn');
    if (btn) btn.classList.add('copy-btn--copied');
    if (_manageCopyTimer) clearTimeout(_manageCopyTimer);
    _manageCopyTimer = setTimeout(resetSubManageCopyBtn, 1600);
}

/* ── Regenerate token ───────────────────────────────────────────── */
function askRegenerateToken() {
    if (!_manageSub) return;
    const dialog = document.getElementById('subRegenerateDialog');
    if (dialog) dialog.style.display = 'flex';
}

function closeRegenerateDialog() {
    const dialog = document.getElementById('subRegenerateDialog');
    if (dialog) dialog.style.display = 'none';
}

async function confirmRegenerateToken() {
    if (!_manageSub || _regenerateInFlight) return;
    const subId = _manageSub.id;
    closeRegenerateDialog();
    const orgId = window.__subscriptionOrgId;
    if (!orgId) return;
    _regenerateInFlight = true;
    try {
        const resp = await fetch(
            devportalApi.root(`/subscriptions/${encodeURIComponent(subId)}/regenerate-token`),
            { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.devportalApi.csrfToken() } }
        );
        if (resp.ok) {
            const data = await resp.json();
            const newToken = data.subscriptionToken;
            delete tokenCache[subId];
            if (newToken) tokenCache[subId] = newToken;
            _manageRevealed = true;
            document.getElementById('subManageTokenVal').textContent = newToken || '•'.repeat(28);
            const ri = document.getElementById('subManageRevealIcon');
            if (ri) ri.className = 'bi bi-eye-slash';
            resetSubManageCopyBtn();
            if (typeof showAlert === 'function') {
                await showAlert('Token regenerated. Copy your new token before closing this window.', 'success');
            }
        } else {
            const data = await resp.json().catch(() => ({}));
            if (typeof showAlert === 'function') {
                await showAlert('Failed to regenerate token: ' + (data.description || 'Unknown error'), 'error');
            }
        }
    } catch (e) {
        if (typeof showAlert === 'function') {
            await showAlert('Error: ' + e.message, 'error');
        }
    } finally {
        _regenerateInFlight = false;
    }
}

/* ── Suspend / Resume ───────────────────────────────────────────── */
async function toggleSubSuspend() {
    if (!_manageSub) return;
    const newStatus = _manageSub.status === 'ACTIVE' ? 'INACTIVE' : 'ACTIVE';
    try {
        const resp = await fetch(
            devportalApi.root(`/subscriptions/${encodeURIComponent(_manageSub.id)}`),
            {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.devportalApi.csrfToken() },
                body: JSON.stringify({ status: newStatus }),
            }
        );
        if (resp.ok) {
            _manageSub.status = newStatus;
            updateSubManageStatus(newStatus);
            // Sync the status pill and data attribute in the table row
            const row = document.getElementById('sub-row-' + _manageSub.id);
            if (row) {
                row.dataset.status = newStatus;
                const pill = row.querySelector('.sub-status-pill');
                if (pill) {
                    pill.className = 'sub-status-pill ' + (newStatus === 'ACTIVE' ? 'sub-status-pill--active' : 'sub-status-pill--inactive');
                    pill.innerHTML = '<span class="sub-status-dot"></span>' + newStatus;
                }
            }
        } else {
            const data = await resp.json().catch(() => ({}));
            await showAlert('Failed to update subscription: ' + (data.description || 'Unknown error'), 'error');
        }
    } catch (e) {
        await showAlert('Error: ' + e.message, 'error');
    }
}

/* ── Unsubscribe ────────────────────────────────────────────────── */
function askSubUnsub() {
    if (!_manageSub) return;
    const title = document.getElementById('subUnsubTitle');
    if (title) title.textContent = 'Unsubscribe from ' + (_manageSub.planName || 'this plan') + '?';
    const dialog = document.getElementById('subUnsubDialog');
    if (dialog) dialog.style.display = 'flex';
}

function closeSubUnsub() {
    const dialog = document.getElementById('subUnsubDialog');
    if (dialog) dialog.style.display = 'none';
}

async function confirmSubUnsub() {
    if (!_manageSub) return;
    closeSubUnsub();
    await executeSubRowDelete(_manageSub.id);
}

/* ── Delete / unsubscribe (API call) ────────────────────────────── */
async function executeSubRowDelete(subscriptionId) {
    try {
        const response = await fetch(
            devportalApi.root(`/subscriptions/${encodeURIComponent(subscriptionId)}`),
            { method: 'DELETE', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.devportalApi.csrfToken() } }
        );

        if (response.ok) {
            closeSubManage();
            await showAlert('Subscription removed successfully!', 'success');
            const row = document.getElementById('sub-row-' + subscriptionId);
            if (row) row.remove();
            const tbody = document.querySelector('#subscriptions-table tbody');
            if (tbody && tbody.children.length === 0) {
                document.getElementById('subscriptions-table')?.closest('.sub-scroll')?.remove();
                const noSubs = document.getElementById('no-subscriptions');
                if (noSubs) noSubs.style.display = 'block';
            }
        } else {
            let message = 'Unknown error';
            try {
                const data = await response.json();
                message = data.description || message;
            } catch (_) {
                message = await response.text().catch(() => response.statusText || message);
            }
            await showAlert(`Failed to remove subscription: ${message}`, 'error');
        }
    } catch (error) {
        await showAlert(`Error: ${error.message}`, 'error');
    }
}

/* ── Wire modals ────────────────────────────────────────────────── */
(function wireSubModals() {
    const manageModal = document.getElementById('subManageModal');
    if (manageModal) {
        document.getElementById('subManageClose')?.addEventListener('click', closeSubManage);
        document.getElementById('subManageRevealBtn')?.addEventListener('click', revealSubToken);
        document.getElementById('subManageCopyBtn')?.addEventListener('click', copySubToken);
        document.getElementById('subManageRegenerateBtn')?.addEventListener('click', askRegenerateToken);
        document.getElementById('subManageSuspendBtn')?.addEventListener('click', toggleSubSuspend);
        document.getElementById('subManageUnsubBtn')?.addEventListener('click', askSubUnsub);
        manageModal.addEventListener('click', function (e) { if (e.target === manageModal) closeSubManage(); });
    }

    const unsubDialog = document.getElementById('subUnsubDialog');
    if (unsubDialog) {
        document.getElementById('subUnsubCancelBtn')?.addEventListener('click', closeSubUnsub);
        document.getElementById('subUnsubConfirmBtn')?.addEventListener('click', confirmSubUnsub);
        unsubDialog.addEventListener('click', function (e) { if (e.target === unsubDialog) closeSubUnsub(); });
    }

    const regenerateDialog = document.getElementById('subRegenerateDialog');
    if (regenerateDialog) {
        document.getElementById('subRegenerateCancelBtn')?.addEventListener('click', closeRegenerateDialog);
        document.getElementById('subRegenerateConfirmBtn')?.addEventListener('click', confirmRegenerateToken);
        regenerateDialog.addEventListener('click', function (e) { if (e.target === regenerateDialog) closeRegenerateDialog(); });
    }
})();
