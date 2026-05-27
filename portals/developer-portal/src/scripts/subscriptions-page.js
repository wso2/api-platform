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

let pendingDeleteOrgID = null;
let pendingDeleteSubID = null;
let pendingDeleteType = null;
let pendingDeleteAppID = null;
let pendingDeleteApiRefID = null;

function prepareDeleteSubscription(orgID, subID, type, appID, apiRefID) {
    pendingDeleteOrgID = orgID;
    pendingDeleteSubID = subID;
    pendingDeleteType = type;
    pendingDeleteAppID = appID || null;
    pendingDeleteApiRefID = apiRefID || null;

    if (typeof openWarningModal === 'function') {
        if (type === 'TOKEN_BASED') {
            openWarningModal('DeleteTokenBasedSubscription', '', '', '', '', '', '');
        } else {
            openWarningModal('DeleteAppBasedSubscription', '', '', '', '', '', '');
        }
        return;
    }

    const message = type === 'TOKEN_BASED'
        ? 'Are you sure you want to delete this subscription? The subscription token will be immediately invalidated.'
        : 'Are you sure you want to unsubscribe from this API?';

    if (confirm(message)) {
        executeDeleteSubscription();
    }
}

async function executeDeleteSubscription() {
    try {
        let response;
        if (pendingDeleteType === 'TOKEN_BASED') {
            response = await fetch(
                `/devportal/organizations/${encodeURIComponent(pendingDeleteOrgID)}/api-platform-subscriptions/${encodeURIComponent(pendingDeleteSubID)}`,
                { method: 'DELETE', headers: { 'Content-Type': 'application/json' } }
            );
        } else {
            // App-based: use existing unsubscribe endpoint
            response = await fetch(
                `/devportal/organizations/${encodeURIComponent(pendingDeleteOrgID)}/subscriptions?appID=${encodeURIComponent(pendingDeleteAppID)}&apiReferenceID=${encodeURIComponent(pendingDeleteApiRefID)}&subscriptionID=${encodeURIComponent(pendingDeleteSubID)}`,
                { method: 'DELETE', headers: { 'Content-Type': 'application/json' } }
            );
        }

        if (response.ok) {
            await showAlert('Subscription removed successfully!', 'success');
            const row = document.getElementById('sub-row-' + pendingDeleteSubID);
            if (row) {
                row.remove();
            }
            // Check if table is now empty
            const tbody = document.querySelector('#subscriptions-table tbody');
            if (tbody && tbody.children.length === 0) {
                const tableContainer = document.getElementById('subscriptions-table')?.closest('.table-responsive');
                if (tableContainer) {
                    tableContainer.remove();
                }
                const noSubs = document.getElementById('no-subscriptions');
                if (noSubs) {
                    noSubs.style.display = 'block';
                }
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

async function toggleSubscriptionStatus(orgID, subscriptionId, newStatus) {
    try {
        const response = await fetch(
            `/devportal/organizations/${encodeURIComponent(orgID)}/api-platform-subscriptions/${encodeURIComponent(subscriptionId)}`,
            {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ status: newStatus }),
            }
        );

        if (response.ok) {
            await showAlert(`Subscription ${newStatus === 'ACTIVE' ? 'activated' : 'deactivated'}!`, 'success');
            window.location.reload();
        } else {
            let message = 'Unknown error';
            try {
                const data = await response.json();
                message = data.description || message;
            } catch (_) {
                message = await response.text().catch(() => response.statusText || message);
            }
            await showAlert(`Failed: ${message}`, 'error');
        }
    } catch (error) {
        await showAlert(`Error: ${error.message}`, 'error');
    }
}

const tokenCache = {};

async function toggleListTokenVisibility(subscriptionId) {
    const tokenEl = document.getElementById('list-token-' + subscriptionId);
    if (!tokenEl) return;

    // If currently revealed, hide and restore masked value (keep tail if available)
    if (tokenEl.dataset.revealed === 'true') {
        // try to reuse stored tail or cached full token to rebuild masked suffix
        const tail = tokenEl.dataset.tail || (tokenCache[subscriptionId] ? String(tokenCache[subscriptionId]).slice(-4) : null);
        if (tail) {
            tokenEl.textContent = '****' + tail;
        } else {
            tokenEl.textContent = '****';
        }
        tokenEl.dataset.revealed = 'false';
        return;
    }

    // Reveal path: fetch token if needed, then store masked tail for future hides
    let fullToken = tokenCache[subscriptionId];
    if (!fullToken) {
        const orgID = window.__subscriptionOrgID;
        if (!orgID) return;
        try {
            const response = await fetch(
                `/devportal/organizations/${encodeURIComponent(orgID)}/api-platform-subscriptions/${encodeURIComponent(subscriptionId)}`,
                { headers: { 'Content-Type': 'application/json' } }
            );
            if (!response.ok) return;
            const data = await response.json();
            fullToken = data.subscriptionToken;
            if (!fullToken) return;
            tokenCache[subscriptionId] = fullToken;
            window.__tokenMap = window.__tokenMap || {};
            window.__tokenMap[subscriptionId] = fullToken;
        } catch (_) {
            return;
        }
    }

    // store the full token and the tail on the element for future toggles
    try {
        tokenEl.dataset.full = fullToken;
        tokenEl.dataset.tail = String(fullToken).slice(-4);
    } catch (e) {
        // ignore dataset write errors
    }

    tokenEl.textContent = fullToken;
    tokenEl.dataset.revealed = 'true';
}
