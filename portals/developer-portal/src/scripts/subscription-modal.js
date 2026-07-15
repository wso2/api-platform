if (typeof window.loadModal !== 'function') {
    window.loadModal = function loadModal(modalID) {
        const modal = document.getElementById(modalID);
        if (modal) modal.style.display = 'flex';
    };
}

if (typeof window.closeModal !== 'function') {
    window.closeModal = function closeModal(modalID) {
        const modal = document.getElementById(modalID);
        if (modal) modal.style.display = 'none';
    };
}

document.addEventListener('click', function (e) {
    const btn = e.target.closest('.subscription-plan-subscribe-btn');
    if (!btn) return;

    // Only handle buttons inside the subscription modal
    const modal = btn.closest('.modal');
    if (!modal) return;

    // Show loading state for the clicked button
    if (typeof window.showSubscribeButtonLoading === 'function') {
        try { window.showSubscribeButtonLoading(btn); } catch (e) { /* noop */ }
    }

    // Ensure modal is visible and focus first focusable element
    modal.style.display = 'flex';
    const focusEl = modal.querySelector('button, a, input, select, textarea');
    if (focusEl && typeof focusEl.focus === 'function') focusEl.focus();
});

// Close visible modal on Escape
document.addEventListener('keydown', function (e) {
    if (e.key !== 'Escape') return;
    const modals = document.querySelectorAll('.modal.custom-modal');
    modals.forEach(m => {
        if (m.style.display && m.style.display !== 'none') {
            // find id and call closeModal if available
            if (typeof window.closeModal === 'function' && m.id) {
                try { window.closeModal(m.id); } catch (err) { m.style.display = 'none'; }
            } else {
                m.style.display = 'none';
            }
        }
    });
});

async function prepareSubscriptionModal(modalId) {
    const modal = document.getElementById(modalId);
    if (!modal) return;

    const apiId = modalId.replace('planModal-', '');
    const orgId = modal.dataset.orgId || window.__subscriptionOrgId;
    let apiRefId = modal.dataset.apiRefid || '';
    if (!apiRefId) apiRefId = apiId;
    const subscriptionContainer = document.getElementById('subscriptionContent-' + apiId);
    const plansBody = modal.querySelector('.subscription-plans-body');

    // Only clear the token area if it has no fresh content (i.e. left from a prior modal session)
    var tokenArea = document.getElementById('subscriptionTokenArea-' + apiId);
    if (tokenArea && !window.__preserveTokenArea) {
        tokenArea.innerHTML = '';
        tokenArea.style.display = 'none';
    }
    window.__preserveTokenArea = false;

    if (!subscriptionContainer) return;
    subscriptionContainer.innerHTML = '';

    if (!orgId) {
        subscriptionContainer.innerHTML = '<div class="alert alert-warning">Organization not available.</div>';
        subscriptionContainer.style.display = 'block';
        if (plansBody) plansBody.style.display = 'none';
        return;
    }

    try {
        const resp = await fetch(devportalApi.root(`/subscriptions?artifactId=${encodeURIComponent(apiRefId)}`), { headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.devportalApi.csrfToken() } });
        if (!resp.ok) throw new Error('Failed to fetch subscriptions');
        const data = await resp.json();

        // data may contain list (existing subscriptions) and subscriptionPlans
        const existing = data.list || data || [];
        const plans = data.subscriptionPlans || data.plans || [];

        // expose token meta and org id for other helpers
        window.__subscriptionOrgId = window.__subscriptionOrgId || orgId;

        // Render existing subscriptions table
        if (existing && existing.length > 0) {
            const table = document.createElement('table');
            table.className = 'table mb-3';
            table.innerHTML = `
                <thead>
                    <tr>
                        <th>Plan</th>
                        <th>Status</th>
                        <th>Subscription Token</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody></tbody>
            `;
            const tbody = table.querySelector('tbody');
            existing.forEach(sub => {
                // store masked token meta
                window.__tokenMeta = window.__tokenMeta || {};
                window.__tokenMeta[sub.subscriptionId] = { maskedToken: sub.maskedToken, subscriptionPlanName: sub.subscriptionPlanName, status: sub.status };

                const tr = document.createElement('tr');

                // Plan name cell
                const tdPlan = document.createElement('td');
                tdPlan.textContent = sub.subscriptionPlanName || '';
                tr.appendChild(tdPlan);

                // Status cell
                const tdStatus = document.createElement('td');
                const statusBadge = document.createElement('span');
                statusBadge.className = 'badge ' + (sub.status === 'ACTIVE' ? 'bg-success' : 'bg-secondary');
                statusBadge.textContent = sub.status || '';
                tdStatus.appendChild(statusBadge);
                tr.appendChild(tdStatus);

                // Token cell
                const tdToken = document.createElement('td');
                const tokenDisplay = document.createElement('div');
                tokenDisplay.className = 'token-display';
                const tokenCode = document.createElement('code');
                tokenCode.className = 'masked-token';
                tokenCode.id = 'token-' + sub.subscriptionId;
                tokenCode.dataset.revealed = 'false';
                tokenCode.textContent = '****';
                const revealBtn = document.createElement('button');
                revealBtn.className = 'btn btn-sm btn-outline-secondary';
                revealBtn.title = 'Reveal token';
                revealBtn.innerHTML = '<i class="bi bi-eye"></i>';
                revealBtn.dataset.subscriptionId = sub.subscriptionId;
                revealBtn.addEventListener('click', function() { toggleTokenVisibility(this.dataset.subscriptionId); });
                const copyBtn = document.createElement('button');
                copyBtn.className = 'btn btn-sm btn-outline-secondary';
                copyBtn.title = 'Copy token';
                copyBtn.innerHTML = '<i class="bi bi-clipboard"></i>';
                copyBtn.dataset.subscriptionId = sub.subscriptionId;
                copyBtn.addEventListener('click', function() { copySubscriptionToken(this.dataset.subscriptionId); });
                tokenDisplay.appendChild(tokenCode);
                tokenDisplay.appendChild(revealBtn);
                tokenDisplay.appendChild(copyBtn);
                tdToken.appendChild(tokenDisplay);
                tr.appendChild(tdToken);

                // Actions cell
                const tdActions = document.createElement('td');
                const newStatus = sub.status === 'ACTIVE' ? 'INACTIVE' : 'ACTIVE';
                const toggleBtn = document.createElement('button');
                toggleBtn.className = 'btn btn-sm btn-outline-warning';
                toggleBtn.innerHTML = sub.status === 'ACTIVE' ? '<i class="bi bi-pause-circle"></i>' : '<i class="bi bi-play-circle"></i>';
                toggleBtn.dataset.orgId = orgId;
                toggleBtn.dataset.subscriptionId = sub.subscriptionId;
                toggleBtn.dataset.newStatus = newStatus;
                if (window.isReadOnly) toggleBtn.disabled = true;
                toggleBtn.addEventListener('click', function() {
                    toggleSubscriptionStatus(this.dataset.orgId, this.dataset.subscriptionId, this.dataset.newStatus);
                });
                const deleteBtn = document.createElement('button');
                deleteBtn.className = 'btn btn-sm btn-outline-danger';
                deleteBtn.innerHTML = '<i class="bi bi-trash"></i>';
                deleteBtn.dataset.orgId = orgId;
                deleteBtn.dataset.subscriptionId = sub.subscriptionId;
                if (window.isReadOnly) deleteBtn.disabled = true;
                deleteBtn.addEventListener('click', function() {
                    confirmDeleteSubscription(this.dataset.orgId, this.dataset.subscriptionId);
                });
                tdActions.appendChild(toggleBtn);
                tdActions.appendChild(deleteBtn);
                tr.appendChild(tdActions);

                tbody.appendChild(tr);
            });
            subscriptionContainer.appendChild(table);
        }

        // Render subscription plans from CP if available
        if (plans && plans.length > 0) {
            const header = document.createElement('div');
            header.className = 'container-header mb-3';
            header.textContent = 'Subscription Plans';
            subscriptionContainer.appendChild(header);

            const row = document.createElement('div');
            row.className = 'row row-gap-4 justify-content-center';
            plans.forEach(plan => {
                const col = document.createElement('div');
                col.className = 'col-xl-3 col-lg-4 col-md-6 col-12';
                col.innerHTML = `
                    <div class="card dev-card subscription-card">
                        <div class="card-body align-items-center text-center p-0">
                            <span class="subscription-plans-card-title">${escapeHtml(plan.displayName || plan.subscriptionPlanName || '')}</span>
                            <h1 class="subscription-plans-request-count">${escapeHtml(formatPlanLimitSummary(plan))}</h1>
                            <p class="subscription-plans-card-subtitle pt-0">${escapeHtml(formatPlanLimitSubtitle(plan))}</p>
                        </div>
                        <div class="position-relative">
                            <div class="message-overlay hidden"><div class="message-content"><i class="bi message-icon"></i><p class="message-text"></p></div><button type="button" class="close-message" aria-label="Close">&times;</button></div>
                        </div>
                    </div>
                `;
                const card = col.querySelector('.card');
                const btn = document.createElement('button');
                btn.className = 'common-btn-primary subscribe-btn w-100';
                btn.textContent = 'Subscribe';
                btn.dataset.orgId = orgId;
                btn.dataset.apiId = apiId;
                btn.dataset.planId = plan.id || '';
                btn.dataset.planName = plan.id || plan.subscriptionPlanName || '';
                btn.dataset.displayName = plan.displayName || plan.subscriptionPlanName || '';
                if (window.isReadOnly) {
                    btn.disabled = true;
                    btn.setAttribute('aria-disabled', 'true');
                    btn.classList.add('disabled');
                } else {
                    btn.addEventListener('click', function () { handlePlanSubscription(this); });
                }
                card.querySelector('.position-relative').appendChild(btn);

                row.appendChild(col);
            });
            subscriptionContainer.appendChild(row);
            if (plansBody) plansBody.style.display = 'none';
        } else {
            if (plansBody) plansBody.style.display = '';
        }

        // Store existing subscriptions for plan-change confirmation flow
        if (existing && existing.length > 0) {
            window.existingSubscriptions = existing.map(function(sub) {
                return { subscriptionId: sub.subscriptionId, subscriptionPlanName: sub.subscriptionPlanName, status: sub.status };
            });
        } else {
            window.existingSubscriptions = [];
        }

        subscriptionContainer.style.display = ((existing && existing.length > 0) || (plans && plans.length > 0)) ? 'block' : 'none';
    } catch (e) {
        subscriptionContainer.innerHTML = '<div class="alert alert-danger">Could not load subscriptions.</div>';
        subscriptionContainer.style.display = 'block';
        if (plansBody) plansBody.style.display = '';
    }
}

function escapeHtml(unsafe) {
    return String(unsafe).replace(/[&<>"'`]/g, function (m) { return ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":"&#39;","`":"&#96;"})[m]; });
}

var LIMIT_TYPE_LABELS = { REQUEST_COUNT: 'req', EVENT_COUNT: 'events', BANDWIDTH: 'bytes', TOTAL_TOKEN_COUNT: 'tokens' };
var TIME_UNIT_LABELS  = { MINUTE: 'min', HOUR: 'hr', DAY: 'day', MONTH: 'mo' };

function formatPlanLimitSummary(plan) {
    var limits = plan.limits;
    if (!limits || limits.length === 0) return 'Unlimited';
    var first = limits[0];
    return first.limitCount === -1 ? '∞' : String(first.limitCount);
}

function formatPlanLimitSubtitle(plan) {
    var limits = plan.limits;
    if (!limits || limits.length === 0) return '';
    var first = limits[0];
    var typeLabel = LIMIT_TYPE_LABELS[first.limitType] || (first.limitType || '').toLowerCase().replace(/_/g, ' ');
    if (!first.timeUnit) return typeLabel;
    var unitLabel = TIME_UNIT_LABELS[first.timeUnit] || (first.timeUnit || '').toLowerCase();
    var amount = first.timeAmount && first.timeAmount !== 1 ? first.timeAmount + ' ' : '';
    return typeLabel + ' / ' + amount + unitLabel;
}
