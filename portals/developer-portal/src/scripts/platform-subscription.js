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

async function subscribePlatformGateway(orgID, apiId, planName, applicationId) {
    try {
        const body = { apiId, subscriptionPlanName: planName };
        if (applicationId) {
            body.applicationId = applicationId;
        }

        const response = await fetch(`/devportal/organizations/${encodeURIComponent(orgID)}/api-platform-subscriptions`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
        });

        const responseData = await response.json();

        if (response.ok) {
            window.__platformSubscriptionChanged = true;
            window.existingPlatformSubscriptions = window.existingPlatformSubscriptions || [];
            window.existingPlatformSubscriptions.push({
                subscriptionId: responseData.subscriptionId,
                subscriptionPlanName: planName,
                status: 'ACTIVE'
            });
            const modalId = 'planModal-' + apiId;
            const modalEl = document.getElementById(modalId);
            const isTokenBased = modalEl && modalEl.dataset.hasSubscriptionToken === 'true';
            if (isTokenBased) {
                const token = responseData.subscriptionToken;
                if (modalEl && modalEl.style.display && modalEl.style.display !== 'none') {
                    try {
                        showSubscriptionTokenInModal(apiId, token, planName);
                    } catch (e) {
                        await showSubscriptionTokenModal(token, planName);
                    }
                } else {
                    await showSubscriptionTokenModal(token, planName);
                }
            } else {
                await refreshLandingPageSubscriptions();
            }
        } else {
            await showAlert(`Failed to subscribe: ${responseData.description || 'Unknown error'}`, 'error');
        }
    } catch (error) {
        await showAlert(`Error while subscribing: ${error.message}`, 'error');
    }
}

async function handlePlanSubscription(btnElement) {
    const orgID = btnElement.dataset.orgId;
    const apiId = btnElement.dataset.apiId;
    const planName = btnElement.dataset.policyName;
    const displayName = btnElement.dataset.displayName;

    // If a modal exists for this API and the button is NOT inside it, open the modal.
    // If the button IS inside the modal, proceed directly to subscribe.
    const modalId = 'planModal-' + apiId;
    const modalEl = document.getElementById(modalId);
    if (modalEl && !btnElement.closest('#' + modalId)) {
        loadModal(modalId);
        return;
    }

    const existingSubs = window.existingPlatformSubscriptions || [];

    if (existingSubs.length === 0) {
        showSubscribeButtonLoading(btnElement);
        await subscribePlatformGateway(orgID, apiId, planName);
        return;
    }

    const currentSub = existingSubs[0];
    const currentPlan = currentSub.subscriptionPlanName || 'current plan';

    if (typeof openWarningModal !== 'function') {
        await showAlert('Confirmation dialog is not available. Please refresh the page.', 'error');
        return;
    }
    window.__pendingPlanSwitchBtn = btnElement;
    openWarningModal(
        'SwitchPlatformSubscriptionPlan',
        orgID,
        apiId,
        planName,
        displayName,
        currentSub.subscriptionId,
        currentPlan
    );
}

async function togglePlatformSubscriptionStatus(orgID, subscriptionId, newStatus) {
    try {
        const response = await fetch(`/devportal/organizations/${encodeURIComponent(orgID)}/api-platform-subscriptions/${encodeURIComponent(subscriptionId)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ status: newStatus }),
        });

        if (response.ok) {
            window.__platformSubscriptionChanged = true;
            await showAlert(`Subscription ${newStatus === 'ACTIVE' ? 'activated' : 'deactivated'} successfully!`, 'success');
            refreshPlatformModalOrReload(orgID);
        } else {
            const responseData = await response.json();
            await showAlert(`Failed to update subscription: ${responseData.description || 'Unknown error'}`, 'error');
        }
    } catch (error) {
        await showAlert(`Error: ${error.message}`, 'error');
    }
}

function confirmDeletePlatformSubscription(orgID, subscriptionId) {
    if (typeof openWarningModal !== 'function') {
        showAlert('Confirmation dialog is not available. Please refresh the page.', 'error');
        return;
    }
    openWarningModal('DeletePlatformSubscription', orgID, subscriptionId, '', '', '', '');
}

async function executeDeletePlatformSubscription(orgID, subscriptionId) {
    try {
        const response = await fetch(`/devportal/organizations/${encodeURIComponent(orgID)}/api-platform-subscriptions/${encodeURIComponent(subscriptionId)}`, {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json' },
        });

        if (response.ok) {
            window.__platformSubscriptionChanged = true;
            await showAlert('Subscription deleted successfully!', 'success');
            refreshPlatformModalOrReload(orgID);
        } else {
            const responseData = await response.json().catch(() => ({}));
            await showAlert(`Failed to delete subscription: ${responseData.description || 'Unknown error'}`, 'error');
        }
    } catch (error) {
        await showAlert(`Error: ${error.message}`, 'error');
    }
}

async function runPendingPlatformPlanSwitch(orgID, apiId, planName, displayName, subscriptionId) {
    const btnElement = window.__pendingPlanSwitchBtn;
    window.__pendingPlanSwitchBtn = null;

    if (btnElement && typeof showSubscribeButtonLoading === 'function') {
        showSubscribeButtonLoading(btnElement);
    }

    try {
        const deleteResponse = await fetch(`/devportal/organizations/${encodeURIComponent(orgID)}/api-platform-subscriptions/${encodeURIComponent(subscriptionId)}`, {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json' },
        });

        if (!deleteResponse.ok) {
            const errorData = await deleteResponse.json().catch(() => ({}));
            await showAlert(`Failed to remove existing subscription: ${errorData.description || 'Unknown error'}`, 'error');
            return;
        }

        await subscribePlatformGateway(orgID, apiId, planName);
    } catch (error) {
        await showAlert(`Error during plan change: ${error.message}`, 'error');
    }
}

function refreshPlatformModalOrReload(orgID) {
    // If inside a visible modal, re-render its content instead of reloading the page
    var visibleModal = document.querySelector('.modal.custom-modal[style*="flex"]');
    if (visibleModal && visibleModal.id && typeof prepareSubscriptionModal === 'function') {
        prepareSubscriptionModal(visibleModal.id);
        return;
    }
    // On the landing page, refresh inline without full reload
    if (document.getElementById('subscriptionPlans')) {
        refreshLandingPageSubscriptions();
        return;
    }
    window.location.reload();
}

async function refreshLandingPageSubscriptions() {
    var planBtn = document.querySelector('#subscriptionPlans [data-api-id]');
    var orgID = window.__subscriptionOrgID || (planBtn && planBtn.dataset.orgId);
    if (!orgID) { window.location.reload(); return; }

    var apiId = planBtn ? planBtn.dataset.apiId : null;
    if (!apiId) { window.location.reload(); return; }

    try {
        var resp = await fetch('/devportal/organizations/' + encodeURIComponent(orgID) + '/api-platform-subscriptions?apiId=' + encodeURIComponent(apiId), { headers: { 'Content-Type': 'application/json' } });
        if (!resp.ok) { window.location.reload(); return; }
        var data = await resp.json();
        var existing = data.list || data || [];

        // Update window state
        window.existingPlatformSubscriptions = existing.map(function(s) {
            return { subscriptionId: s.subscriptionId, subscriptionPlanName: s.subscriptionPlanName, status: s.status };
        });
        window.__tokenMeta = window.__tokenMeta || {};
        existing.forEach(function(sub) {
            window.__tokenMeta[sub.subscriptionId] = { maskedToken: sub.maskedToken, subscriptionPlanName: sub.subscriptionPlanName, status: sub.status };
        });

        // Re-render existing subscriptions table
        var existingSection = document.querySelector('#subscriptionPlans .existing-subscriptions');
        if (existing.length > 0) {
            if (!existingSection) {
                existingSection = document.createElement('div');
                existingSection.className = 'existing-subscriptions mb-4';
                var plansHeader = document.querySelector('#subscriptionPlans .container-header');
                if (plansHeader) {
                    plansHeader.parentNode.insertBefore(existingSection, plansHeader);
                } else {
                    document.querySelector('#subscriptionPlans .container-fluid').prepend(existingSection);
                }
            }
            var modalEl = document.getElementById('planModal-' + apiId);
            var isTokenBased = modalEl && modalEl.dataset.hasSubscriptionToken === 'true';

            existingSection.innerHTML = '<div class="container-header mb-4">Subscriptions</div>';
            var table = document.createElement('table');
            table.className = 'table';
            var headerHtml = '<thead><tr><th>Plan</th><th>Status</th>';
            if (isTokenBased) headerHtml += '<th>Subscription Token</th>';
            headerHtml += '<th>Actions</th></tr></thead><tbody></tbody>';
            table.innerHTML = headerHtml;
            var tbody = table.querySelector('tbody');
            existing.forEach(function(sub) {
                var tr = document.createElement('tr');

                // Plan name cell
                var tdPlan = document.createElement('td');
                tdPlan.textContent = sub.subscriptionPlanName || '';
                tr.appendChild(tdPlan);

                // Status cell
                var tdStatus = document.createElement('td');
                var badge = document.createElement('span');
                badge.className = 'badge ' + (sub.status === 'ACTIVE' ? 'bg-success' : 'bg-secondary');
                badge.textContent = sub.status || '';
                tdStatus.appendChild(badge);
                tr.appendChild(tdStatus);

                // Token cell — only for token-based subscription APIs
                if (isTokenBased) {
                    var tdToken = document.createElement('td');
                    var tokenDisplay = document.createElement('div');
                    tokenDisplay.className = 'token-display';
                    var code = document.createElement('code');
                    code.className = 'masked-token';
                    code.id = 'token-' + sub.subscriptionId;
                    code.dataset.revealed = 'false';
                    code.textContent = '****';
                    var revealBtn = document.createElement('button');
                    revealBtn.className = 'btn btn-sm btn-outline-secondary';
                    revealBtn.title = 'Reveal token';
                    revealBtn.innerHTML = '<i class="bi bi-eye"></i>';
                    revealBtn.dataset.subscriptionId = sub.subscriptionId;
                    revealBtn.addEventListener('click', function() { toggleTokenVisibility(this.dataset.subscriptionId); });
                    var copyBtn = document.createElement('button');
                    copyBtn.className = 'btn btn-sm btn-outline-secondary';
                    copyBtn.title = 'Copy token';
                    copyBtn.innerHTML = '<i class="bi bi-clipboard"></i>';
                    copyBtn.dataset.subscriptionId = sub.subscriptionId;
                    copyBtn.addEventListener('click', function() { copySubscriptionToken(this.dataset.subscriptionId); });
                    tokenDisplay.appendChild(code);
                    tokenDisplay.appendChild(revealBtn);
                    tokenDisplay.appendChild(copyBtn);
                    tdToken.appendChild(tokenDisplay);
                    tr.appendChild(tdToken);
                }

                // Actions cell
                var tdActions = document.createElement('td');
                var newStatus = sub.status === 'ACTIVE' ? 'INACTIVE' : 'ACTIVE';
                var toggleBtn = document.createElement('button');
                toggleBtn.className = 'btn btn-sm btn-outline-warning';
                toggleBtn.innerHTML = sub.status === 'ACTIVE' ? '<i class="bi bi-pause-circle"></i>' : '<i class="bi bi-play-circle"></i>';
                toggleBtn.dataset.orgId = orgID;
                toggleBtn.dataset.subscriptionId = sub.subscriptionId;
                toggleBtn.dataset.newStatus = newStatus;
                toggleBtn.addEventListener('click', function() {
                    togglePlatformSubscriptionStatus(this.dataset.orgId, this.dataset.subscriptionId, this.dataset.newStatus);
                });
                var deleteBtn = document.createElement('button');
                deleteBtn.className = 'btn btn-sm btn-outline-danger';
                deleteBtn.innerHTML = '<i class="bi bi-trash"></i>';
                deleteBtn.dataset.orgId = orgID;
                deleteBtn.dataset.subscriptionId = sub.subscriptionId;
                deleteBtn.addEventListener('click', function() {
                    confirmDeletePlatformSubscription(this.dataset.orgId, this.dataset.subscriptionId);
                });
                tdActions.appendChild(toggleBtn);
                tdActions.appendChild(deleteBtn);
                tr.appendChild(tdActions);

                tbody.appendChild(tr);
            });
            existingSection.appendChild(table);
        } else if (existingSection) {
            existingSection.remove();
        }

        // Update plan card buttons: mark current plan or reset to Subscribe
        var activePlanNames = existing
            .filter(function(s) { return s.status === 'ACTIVE'; })
            .map(function(s) { return (s.subscriptionPlanName || '').toLowerCase(); });

        var planCards = document.querySelectorAll('#subscriptionPlans .subscription-card');
        planCards.forEach(function(card) {
            var btn = card.querySelector('.subscription-plan-subscribe-btn, .subscribe-btn, .current-plan-btn');
            if (!btn) return;
            var policyName = (btn.dataset.policyName || '').toLowerCase();
            if (activePlanNames.indexOf(policyName) !== -1) {
                btn.textContent = 'Current Plan';
                btn.disabled = true;
                btn.classList.add('disabled', 'current-plan-btn');
                btn.removeAttribute('onclick');
            } else {
                btn.textContent = 'Subscribe';
                btn.disabled = false;
                btn.classList.remove('disabled', 'current-plan-btn');
                btn.setAttribute('onclick', 'handlePlanSubscription(this)');
            }
        });
    } catch (e) {
        window.location.reload();
    }
}

function copySubscriptionToken(subscriptionId) {
    (async function() {
        try {
            const token = await fetchPlatformTokenIfNeeded(subscriptionId);
            if (!token) return;
            navigator.clipboard.writeText(token).then(() => {
                showAlert('Subscription token copied to clipboard!', 'success');
            }).catch(() => {
                const textArea = document.createElement('textarea');
                textArea.value = token;
                textArea.style.position = 'fixed';
                textArea.style.opacity = '0';
                document.body.appendChild(textArea);
                textArea.select();
                document.execCommand('copy');
                document.body.removeChild(textArea);
                showAlert('Subscription token copied to clipboard!', 'success');
            });
        } catch (e) {
            // noop
        }
    })();
}

function toggleTokenVisibility(subscriptionId) {
    (async function() {
        const tokenEl = document.getElementById('token-' + subscriptionId);
        if (!tokenEl) return;
        if (tokenEl.dataset.revealed === 'true') {
            // hide and show masked value from token meta if available
            const meta = (window.__tokenMeta || {})[subscriptionId];
            tokenEl.textContent = meta && meta.maskedToken ? meta.maskedToken : '****';
            tokenEl.dataset.revealed = 'false';
            return;
        }

        try {
            const fullToken = await fetchPlatformTokenIfNeeded(subscriptionId);
            if (!fullToken) return;
            tokenEl.textContent = fullToken;
            tokenEl.dataset.revealed = 'true';
        } catch (e) {
            // noop
        }
    })();
}

const _platformTokenCache = {};

async function fetchPlatformTokenIfNeeded(subscriptionId) {
    if (_platformTokenCache[subscriptionId]) return _platformTokenCache[subscriptionId];
    const existing = (window.__tokenMap || {})[subscriptionId];
    if (existing && typeof existing === 'string' && !existing.startsWith('****')) {
        _platformTokenCache[subscriptionId] = existing;
        return existing;
    }
    const orgID = window.__subscriptionOrgID;
    if (!orgID) return null;
    try {
        const resp = await fetch(`/devportal/organizations/${encodeURIComponent(orgID)}/api-platform-subscriptions/${encodeURIComponent(subscriptionId)}`, { headers: { 'Content-Type': 'application/json' } });
        if (!resp.ok) return null;
        const data = await resp.json();
        const token = data.subscriptionToken;
        if (!token) return null;
        _platformTokenCache[subscriptionId] = token;
        window.__tokenMap = window.__tokenMap || {};
        window.__tokenMap[subscriptionId] = token;
        return token;
    } catch (e) {
        return null;
    }
}

function showSubscriptionTokenModal(token, planName) {
    return new Promise((resolve) => {
        const overlay = document.createElement('div');
        overlay.className = 'modal custom-modal';
        overlay.style.display = 'flex';

        const safeplanName = document.createElement('span');
        safeplanName.textContent = planName;

        const safeToken = document.createElement('code');
        safeToken.textContent = token;
        safeToken.className = 'flex-grow-1 p-2 bg-light border rounded';
        safeToken.style.wordBreak = 'break-all';

        overlay.innerHTML = `
            <div class="modal-dialog" role="document">
                <div class="modal-content custom-modal-content">
                    <div class="custom-modal-header">
                        <h2 class="custom-modal-title m-0">Subscription Created</h2>
                        <button type="button" class="btn-close" id="closeTokenModal"></button>
                    </div>
                    <div class="custom-modal-body">
                        <p>Your subscription to the <strong id="planNameDisplay"></strong> plan has been created successfully.</p>
                        <p class="mb-2"><strong>Subscription Token:</strong></p>
                        <div class="d-flex align-items-center gap-2 mb-3" id="tokenContainer"></div>
                        <div class="alert alert-warning mb-0">
                            <i class="bi bi-exclamation-triangle"></i>
                            Use this token as the <code>Subscription-Key</code> header when invoking the API.
                        </div>
                    </div>
                </div>
            </div>
        `;

        document.body.appendChild(overlay);

        overlay.querySelector('#planNameDisplay').textContent = planName;

        const tokenContainer = overlay.querySelector('#tokenContainer');
        tokenContainer.appendChild(safeToken);

        const copyBtn = document.createElement('button');
        copyBtn.className = 'btn btn-sm btn-outline-secondary';
        copyBtn.innerHTML = '<i class="bi bi-clipboard"></i> Copy';
        copyBtn.addEventListener('click', () => {
            // copy raw token string directly (token is available in this scope)
            navigator.clipboard.writeText(token).then(() => {
                showAlert('Subscription token copied to clipboard!', 'success');
            }).catch(() => {
                const textArea = document.createElement('textarea');
                textArea.value = token;
                textArea.style.position = 'fixed';
                textArea.style.opacity = '0';
                document.body.appendChild(textArea);
                textArea.select();
                document.execCommand('copy');
                document.body.removeChild(textArea);
                showAlert('Subscription token copied to clipboard!', 'success');
            });
        });
        tokenContainer.appendChild(copyBtn);

        overlay.querySelector('#closeTokenModal').addEventListener('click', () => {
            overlay.remove();
            if (window.__platformSubscriptionChanged) {
                window.__platformSubscriptionChanged = false;
                if (document.getElementById('subscriptionPlans')) {
                    refreshLandingPageSubscriptions();
                } else {
                    window.location.reload();
                }
            }
            resolve();
        });
    });
}

function showSubscriptionTokenInModal(apiId, token, planName) {
    const area = document.getElementById('subscriptionTokenArea-' + apiId);
    if (!area) {
        return showSubscriptionTokenModal(token, planName);
    }

    area.innerHTML = '';
    const wrapper = document.createElement('div');
    wrapper.className = 'p-3 mb-3 border rounded bg-light';

    const title = document.createElement('div');
    title.innerHTML = `<strong>Subscription Created</strong> — ${escapeHtml(planName)}`;

    const tokenBlock = document.createElement('div');
    tokenBlock.className = 'd-flex gap-2 align-items-center mt-2';
    const code = document.createElement('code');
    code.textContent = token;
    code.style.wordBreak = 'break-all';
    code.className = 'p-2 bg-white border rounded flex-grow-1';

    const copyBtn = document.createElement('button');
    copyBtn.className = 'btn btn-sm btn-outline-secondary';
    copyBtn.innerHTML = '<i class="bi bi-clipboard"></i> Copy';
    copyBtn.addEventListener('click', function() {
        navigator.clipboard.writeText(token).then(() => showAlert('Subscription token copied to clipboard!', 'success'))
            .catch(() => showAlert('Could not copy token', 'error'));
    });

    tokenBlock.appendChild(code);
    tokenBlock.appendChild(copyBtn);
    wrapper.appendChild(title);
    wrapper.appendChild(tokenBlock);

    const info = document.createElement('div');
    info.className = 'alert alert-warning mt-2 mb-0';
    info.innerHTML = '<i class="bi bi-exclamation-triangle"></i> Use this token as the <code>Subscription-Key</code> header when invoking the API.';
    wrapper.appendChild(info);

    area.appendChild(wrapper);
    area.style.display = 'block';

    // Refresh the subscriptions/plans below the token with updated data
    const modalEl = area.closest('.modal');
    if (modalEl && modalEl.id && typeof prepareSubscriptionModal === 'function') {
        window.__preserveTokenArea = true;
        prepareSubscriptionModal(modalEl.id);
    }
}

function escapeHtml(unsafe) {
    return String(unsafe).replace(/[&<>"'`]/g, function (m) { return ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":"&#39;","`":"&#96;"})[m]; });
}
