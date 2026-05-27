/**
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

let activeTabController = null;

// Function to initialize the billing page
function initializeBillingPage() {
  document.getElementById('usagePeriod')?.addEventListener('change', () => {
    const period = document.getElementById('usagePeriod')?.value;
    const customRange = document.getElementById('customDateRange');
    if (customRange) customRange.style.display = period === 'custom' ? '' : 'none';
    if (period !== 'custom') {
      if (activeTabController) activeTabController.abort();
      activeTabController = new AbortController();
      loadUsageData(activeTabController.signal);
    }
  });
  const triggerCustomLoad = () => {
    const from = document.getElementById('usageFrom')?.value;
    const to = document.getElementById('usageTo')?.value;
    if (from && to) {
      if (activeTabController) activeTabController.abort();
      activeTabController = new AbortController();
      loadUsageData(activeTabController.signal);
    }
  };
  document.getElementById('usageFrom')?.addEventListener('change', triggerCustomLoad);
  document.getElementById('usageTo')?.addEventListener('change', triggerCustomLoad);
  document.getElementById('invoicePeriod')?.addEventListener('change', () => {
    if (activeTabController) activeTabController.abort();
    activeTabController = new AbortController();
    loadInvoices(activeTabController.signal);
  });

  const tabs = document.querySelectorAll('.billing-tabs .nav-link[data-bs-toggle="tab"]');
  tabs.forEach((tab) => {
    const targetId = tab.getAttribute('data-bs-target');

    tab.addEventListener('show.bs.tab', function (event) {
      // event.relatedTarget is undefined when the previously-active tab was set
      // active via static HTML classes (not via Bootstrap JS). Fall back to
      // finding the currently-active pane directly in the DOM.
      const activePane = document.querySelector('.tab-content .tab-pane.show.active');
      const oldTarget = event.relatedTarget?.getAttribute('data-bs-target')
        ?? (activePane ? '#' + activePane.id : null);

      if (activeTabController) {
        activeTabController.abort();
      }
      activeTabController = new AbortController();

      // Immediately strip show+active from the outgoing pane so our CSS
      // (.tab-content > .active { display: block }) stops displaying it.
      // Without this, Bootstrap's fade-out keeps the outgoing pane visible
      // while the incoming pane also becomes active — causing both to show.
      if (oldTarget) {
        const outgoing = document.querySelector(oldTarget);
        if (outgoing) outgoing.classList.remove('show', 'active');
        clearTabContent(oldTarget);
      }
      clearTabContent(targetId);
    });

    tab.addEventListener('shown.bs.tab', function () {
      const signal = activeTabController.signal;
      if (targetId === '#billing-usage') {
        loadUsageData(signal);
      } else if (targetId === '#billing-invoices') {
        loadInvoices(signal);
      } else if (targetId === '#billing-subscriptions') {
        loadActiveSubscriptions(signal);
      } else if (targetId === '#billing-payment') {
        loadPaymentDetails(signal);
      }
    });
  });


  const usageTab = document.getElementById('billing-usage-tab');
  const usagePane = document.getElementById('billing-usage');
  if (usageTab) usageTab.classList.remove('active');
  if (usagePane) usagePane.classList.remove('show', 'active');

  if (usageTab && window.bootstrap?.Tab) {
    new window.bootstrap.Tab(usageTab).show();
  } else {
    activeTabController = new AbortController();
    if (usageTab) { usageTab.classList.add('active'); usageTab.setAttribute('aria-selected', 'true'); }
    if (usagePane) usagePane.classList.add('show', 'active');
    loadUsageData(activeTabController.signal);
  }
}


function clearTabContent(tabId) {
  const pane = document.querySelector(tabId);
  if (!pane) return;

  const tableSpinner = (cols) =>
    `<tr><td colspan="${cols}" class="text-center py-4">` +
    `<div class="spinner-border spinner-border-sm me-2" role="status"></div>Loading...</td></tr>`;

  const tableMap = {
    usageTableBody: tableSpinner(6),
    invoicesTableBody: tableSpinner(6),
    subscriptionsTableBody: tableSpinner(8),
  };
  Object.entries(tableMap).forEach(([id, html]) => {
    const el = pane.querySelector('#' + id);
    if (el) el.innerHTML = html;
  });

  const divSpinner = '<div class="text-center py-3"><div class="spinner-border spinner-border-sm me-2" role="status"></div>Loading...</div>';
  const divMap = {
    paymentMethodsList: divSpinner,
    billingInfoContent: divSpinner,
    subscriptionsFilterContainer: '',
  };
  Object.entries(divMap).forEach(([id, html]) => {
    const el = pane.querySelector('#' + id);
    if (el) el.innerHTML = html;
  });

  const noInvoices = pane.querySelector('#noInvoices');
  if (noInvoices) noInvoices.style.display = 'none';
}

// Initialize when DOM is loaded
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", initializeBillingPage);
} else {
  initializeBillingPage();
}

/**
 * Load usage data for the selected period
 */
async function loadUsageData(signal) {
  const period = document.getElementById("usagePeriod")?.value || "current";
  const tableBody = document.getElementById("usageTableBody");
  const orgId = getOrgIdFromContext();
  if (!tableBody) {
    console.error(" usageTableBody element not found!");
    return;
  }

  try {
    tableBody.innerHTML = `
      <tr>
        <td colspan="6" class="text-center py-4">
          <div class="spinner-border spinner-border-sm me-2" role="status"></div>
          Loading usage data...
        </td>
      </tr>
    `;

    const url = `/devportal/organizations/${orgId}/billing/usage-data?period=${period}`;
    let fetchUrl = url;
    if (period === "custom") {
      const from = document.getElementById("usageFrom")?.value;
      const to = document.getElementById("usageTo")?.value;
      if (from) fetchUrl += `&from=${encodeURIComponent(from)}`;
      if (to) fetchUrl += `&to=${encodeURIComponent(to)}`;
    }
    const response = await fetch(fetchUrl, { signal });

    if (!response.ok) {
      const errorText = await response.text();
      console.error("Usage data error response:", errorText);
      throw new Error(`Failed to fetch usage data: ${response.status}`);
    }

    const data = await response.json();
    // Update statistics cards
    document.getElementById("totalRequests").textContent = formatNumber(
      data.totalRequests || 0,
    );
    document.getElementById("activeSubscriptions").textContent =
      data.activeSubscriptions || 0;
    document.getElementById("estimatedCost").textContent = formatUsageCost(
      data.estimatedCost || 0,
      data.currency || "USD",
    );

    // Populate usage table
    if (data.subscriptions && data.subscriptions.length > 0) {
      tableBody.innerHTML = data.subscriptions
        .map(
          (sub) => `
        <tr>
          <td><strong>${escapeHtml(sub.apiName)}</strong></td>
          <td>${escapeHtml(sub.applicationName)}</td>
          <td><span class="badge bg-secondary">${escapeHtml(sub.planName)}</span></td>
          <td>${formatNumber(sub.requests)}</td>
          <td>
            ${
              sub.pricingModel === "PER_UNIT" ||
              sub.pricingModel === "VOLUME_TIERS" ||
              sub.pricingModel === "GRADUATED_TIERS"
                ? '<span class="badge bg-info">Metered</span>'
                : '<span class="badge bg-success">Flat</span>'
            }
          </td>
          <td><strong>${formatUsageCost(sub.cost, sub.currency || "USD")}</strong></td>
        </tr>
      `,
        )
        .join("");
    } else {
      tableBody.innerHTML = `
        <tr>
          <td colspan="6" class="text-center py-4 text-muted">
            <i class="fas fa-chart-line fa-2x mb-3"></i>
            <p>No usage data available for this period</p>
          </td>
        </tr>
      `;
    }
  } catch (error) {
    if (error.name === "AbortError") return;
    console.error("Error loading usage data:", error);
    tableBody.innerHTML = `
      <tr>
        <td colspan="6" class="text-center py-4 text-danger">
          <i class="fas fa-exclamation-circle me-2"></i>
          Failed to load usage data
          <div class="small text-muted mt-2">${escapeHtml(error.message)}</div>
        </td>
      </tr>
    `;
    showAlert("Failed to load usage data", "danger");
  }
}

/**
 * Load invoice history
 */
async function loadInvoices(signal) {
  const tableBody = document.getElementById("invoicesTableBody");
  const noInvoices = document.getElementById("noInvoices");
  const period =
    document.getElementById("invoicePeriod")?.value || "last3months";
  const orgId = getOrgIdFromContext();

  try {
    tableBody.innerHTML = `
      <tr>
        <td colspan="5" class="text-center py-4">
          <div class="spinner-border spinner-border-sm me-2" role="status"></div>
          Loading invoices...
        </td>
      </tr>
    `;

    const response = await fetch(
      `/devportal/organizations/${orgId}/invoices?period=${period}`,
      { signal },
    );

    if (!response.ok) {
      throw new Error("Failed to fetch invoices");
    }

    const data = await response.json();

    if (data.invoices && data.invoices.length > 0) {
      noInvoices.style.display = "none";
      tableBody.innerHTML = data.invoices
        .map(
          (invoice) => `
        <tr>
          <td><strong>${escapeHtml(invoice.number)}</strong></td>
          <td>${formatDate(invoice.created)}</td>
          <td><strong>${formatCurrency(invoice.amount, invoice.currency)}</strong></td>
          <td>
            <span class="badge-status badge-${getInvoiceStatusClass(invoice.status)}">
              ${escapeHtml(invoice.status)}
            </span>
          </td>
          <td>
            <button class="btn btn-sm btn-outline-primary" onclick="downloadInvoice('${invoice.id}')">
              <i class="fas fa-download"></i> Download
            </button>
            ${
              invoice.hostedInvoiceUrl
                ? `
              <a href="${escapeHtml(invoice.hostedInvoiceUrl)}" target="_blank" rel="noopener noreferrer" class="btn btn-sm btn-outline-secondary ms-2">
                <i class="fas fa-external-link-alt"></i> View
              </a>
            `
                : ""
            }
          </td>
        </tr>
      `,
        )
        .join("");
    } else {
      tableBody.innerHTML = "";
      noInvoices.style.display = "block";
    }
  } catch (error) {
    if (error.name === "AbortError") return;
    console.error("Error loading invoices:", error);
    tableBody.innerHTML = `
      <tr>
        <td colspan="5" class="text-center py-4 text-danger">
          <i class="fas fa-exclamation-circle me-2"></i>
          Failed to load invoices. Please try again.
        </td>
      </tr>
    `;
    showAlert("Failed to load invoices", "danger");
  }
}

/**
 * Load payment details including payment methods and billing info
 */
async function loadPaymentDetails(signal) {
  try {
    await Promise.all([
      loadPaymentMethods(signal),
      loadBillingInfo(signal),
    ]);
  } catch (error) {
    if (error.name === "AbortError") return;
    console.error("Error loading payment details:", error);
  }
}

/**
 * Load payment methods
 */
async function loadPaymentMethods(signal) {
  const container = document.getElementById("paymentMethodsList");
  const orgId = getOrgIdFromContext();

  if (!orgId) {
    container.innerHTML = `
      <div class="text-center py-4 text-danger">
        <i class="fas fa-exclamation-circle me-2"></i>
        Organization ID not found
      </div>
    `;
    return;
  }

  if (!orgId) {
    container.innerHTML = `
      <div class="text-center py-4 text-danger">
        <i class="fas fa-exclamation-circle me-2"></i>
        Organization ID not found
      </div>
    `;
    return;
  }

  try {
    container.innerHTML = `
      <div class="text-center py-4">
        <div class="spinner-border spinner-border-sm me-2" role="status"></div>
        Loading payment methods...
      </div>
    `;

    const response = await fetch(
      `/devportal/organizations/${orgId}/billing/payment-methods`,
      { signal },
    );

    if (!response.ok) {
      const errorText = await response.text();
      console.error(
        "Payment methods fetch failed:",
        response.status,
        errorText,
      );

      // Try to parse error JSON
      let errorMessage = "Failed to load payment methods";
      try {
        const errorData = JSON.parse(errorText);
        errorMessage = errorData.message || errorMessage;
      } catch (e) {
        // Use default message if not JSON
      }

      throw new Error(errorMessage);
    }

    const data = await response.json();
    if (data.paymentMethods && data.paymentMethods.length > 0) {
      container.innerHTML = data.paymentMethods
        .map(
          (method) => `
        <div class="payment-method-card ${method.isDefault ? "default" : ""}">
          <div class="d-flex justify-content-between align-items-center">
            <div class="d-flex align-items-center">
              <i class="fab fa-cc-${method.brand.toLowerCase()} fa-2x me-3"></i>
              <div>
                <strong>${escapeHtml(method.brand)}</strong> •••• ${escapeHtml(method.last4)}
                <div class="text-muted small">Expires ${escapeHtml(method.expMonth)}/${escapeHtml(method.expYear)}</div>
              </div>
            </div>
            <div>
              ${method.isDefault ? '<span class="badge bg-primary">Default</span>' : ""}
            </div>
          </div>
        </div>
      `,
        )
        .join("");
    } else {
      container.innerHTML = `
        <div class="text-center py-4 text-muted">
          <i class="fas fa-credit-card fa-2x mb-3"></i>
          <p>No payment methods found</p>
          <p class="small">Add a payment method through Stripe to get started</p>
        </div>
      `;
    }
  } catch (error) {
    if (error.name === "AbortError") return;
    console.error("Error loading payment methods:", error);
    container.innerHTML = `
      <div class="text-center py-4">
        <div class="text-danger mb-3">
          <i class="fas fa-exclamation-circle fa-2x mb-2"></i>
          <p>Failed to load payment methods</p>
          <p class="small text-muted">Please contact your organization administrator.</p>
        </div>
        <button class="common-btn-outlined" onclick="loadPaymentMethods()">
          <i class="fas fa-redo me-2"></i>Retry
        </button>
      </div>
    `;
  }
}

/**
 * Load billing information
 */
async function loadBillingInfo(signal) {
  const container = document.getElementById("billingInfoContent");
  const orgId = getOrgIdFromContext();

  try {
    container.innerHTML = `
      <div class="text-center py-4">
        <div class="spinner-border spinner-border-sm me-2" role="status"></div>
        Loading billing information...
      </div>
    `;

    const response = await fetch(
      `/devportal/organizations/${orgId}/billing/info`,
      { signal },
    );

    if (!response.ok) {
      throw new Error("Failed to fetch billing info");
    }

    const data = await response.json();

    container.innerHTML = `
      <div class="billing-info">
        <div class="mb-3">
          <label class="text-muted small">Organization</label>
          <div class="fw-bold">${escapeHtml(data.organizationName || "N/A")}</div>
        </div>
        <div class="mb-3">
          <label class="text-muted small">Email</label>
          <div>${escapeHtml(data.email || "N/A")}</div>
        </div>
        ${
          data.address
            ? `
          <div class="mb-3">
            <label class="text-muted small">Billing Address</label>
            <div>
              ${escapeHtml(data.address.line1 || "")}<br>
              ${data.address.line2 ? escapeHtml(data.address.line2) + "<br>" : ""}
              ${escapeHtml(data.address.city || "")}, ${escapeHtml(data.address.state || "")} ${escapeHtml(data.address.postalCode || "")}<br>
              ${escapeHtml(data.address.country || "")}
            </div>
          </div>
        `
            : ""
        }
        ${
          data.taxId
            ? `
          <div class="mb-3">
            <label class="text-muted small">Tax ID</label>
            <div>${escapeHtml(data.taxId)}</div>
          </div>
        `
            : ""
        }
        <div class="text-center mt-4">
          <button class="common-btn-outlined" onclick="editBillingInfo()">
            <i class="fas fa-external-link-alt me-2"></i>Update in Stripe Portal
          </button>
        </div>
      </div>
    `;
  } catch (error) {
    if (error.name === "AbortError") return;
    console.error("Error loading billing info:", error);
    container.innerHTML = `
      <div class="text-center py-4 text-danger">
        <i class="fas fa-exclamation-circle me-2"></i>
        Failed to load billing information
      </div>
    `;
  }
}

/**
 * Load active subscriptions
 */
async function loadActiveSubscriptions(signal) {
  const tableBody = document.getElementById("subscriptionsTableBody");
  const orgId = getOrgIdFromContext();
  const filterContainer = document.getElementById(
    "subscriptionsFilterContainer",
  );

  try {
    tableBody.innerHTML = `
      <tr>
        <td colspan="8" class="text-center py-4">
          <div class="spinner-border spinner-border-sm me-2" role="status"></div>
          Loading subscriptions...
        </td>
      </tr>
    `;

    const url = `/devportal/organizations/${orgId}/billing/subscriptions`;
    const response = await fetch(url, { signal });
    if (!response.ok) {
      const errorText = await response.text();
      console.error("Subscriptions error response:", errorText);
      throw new Error("Failed to fetch subscriptions: " + response.status);
    }

    const data = await response.json();
    const allStatuses = Array.from(
      new Set((data.subscriptions || []).map((sub) => sub.status)),
    ).sort();

    // Render filter dropdown
    if (filterContainer) {
      filterContainer.innerHTML = `
        <label for="subscriptionStatusFilter" class="me-2">Filter by Status:</label>
        <select id="subscriptionStatusFilter" class="form-select form-select-sm" style="width: auto; display: inline-block;">
          <option value="ALL">All</option>
           ${allStatuses
             .map((status) => {
               const safeStatus = escapeHtml(status);
               return `<option value="${safeStatus}">${safeStatus}</option>`;
             })
             .join("")}
        </select>
      `;
    }

    function renderTable(filteredSubs) {
      if (filteredSubs.length > 0) {
        tableBody.innerHTML = filteredSubs
          .map(
            (sub) => `
          <tr>
            <td><strong>${escapeHtml(sub.apiName)}</strong></td>
            <td>${escapeHtml(sub.applicationName || "N/A")}</td>
            <td><span class="badge bg-secondary">${escapeHtml(sub.planName)}</span></td>
            <td>${escapeHtml(sub.billingCycle)}</td>
            <td><strong>${formatCurrency(sub.amount, sub.currency)}</strong></td>
            <td>${formatDate(sub.nextBillingDate)}</td>
            <td>
              <span class="badge-status badge-${sub.status === "active" ? "paid" : "pending"}">
                ${escapeHtml(sub.status)}
              </span>
            </td>
            <td>
              <button class="btn btn-sm btn-outline-secondary" onclick="manageSubscription('${sub.id}')">
                <i class="fas fa-cog"></i> Manage
              </button>
            </td>
          </tr>
        `,
          )
          .join("");
      } else {
        tableBody.innerHTML = `
          <tr>
            <td colspan="8" class="text-center py-4 text-muted">
              <i class="fas fa-inbox fa-2x mb-3"></i>
              <p>No subscriptions found</p>
            </td>
          </tr>
        `;
      }
    }

    // Initial render (all)
    renderTable(data.subscriptions || []);

    // Filter logic
    if (filterContainer) {
      const filterSelect = document.getElementById("subscriptionStatusFilter");
      filterSelect.addEventListener("change", function () {
        const selected = this.value;
        if (selected === "ALL") {
          renderTable(data.subscriptions || []);
        } else {
          renderTable(
            (data.subscriptions || []).filter((sub) => sub.status === selected),
          );
        }
      });
    }
  } catch (error) {
    if (error.name === "AbortError") return;
    console.error("Error loading subscriptions:", error);
    tableBody.innerHTML = `
      <tr>
        <td colspan="8" class="text-center py-4 text-danger">
          <i class="fas fa-exclamation-circle me-2"></i>
          Failed to load subscriptions
          <div class="small text-muted mt-2">${escapeHtml(error.message)}</div>
        </td>
      </tr>
    `;
  }
}

/**
 * Download a specific invoice
 */
async function downloadInvoice(invoiceId) {
  const orgId = getOrgIdFromContext();
  try {
    const response = await fetch(
      `/devportal/organizations/${orgId}/invoices/${invoiceId}/pdf`,
    );
    let data = {};
    try {
      data = await response.json();
    } catch (jsonErr) {
      // If not JSON, fallback to text
      data = { error: await response.text() };
    }
    if (!response.ok) {
      // Show backend error message if available
      const msg =
        data && data.message ? data.message : "Failed to download invoice";
      showAlert(msg, "danger");
      throw new Error(msg);
    }
    if (data.invoice_pdf) {
      window.open(data.invoice_pdf, "_blank", "noopener,noreferrer");
      showAlert("Invoice opened in new tab", "success");
    } else {
      const msg =
        data && data.message ? data.message : "Invoice PDF URL not available";
      showAlert(msg, "danger");
      throw new Error(msg);
    }
  } catch (error) {
    console.error("Error downloading invoice:", error);
    // showAlert already called above for most cases
  }
}

/**
 * Download all invoices
 */
async function downloadAllInvoices() {
  const orgId = getOrgIdFromContext();
  try {
    showAlert("Preparing invoices for download...", "info");

    const response = await fetch(
      `/devportal/organizations/${orgId}/invoices/download-all`,
    );

    if (!response.ok) {
      throw new Error("Failed to download invoices");
    }

    const blob = await response.blob();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `all-invoices-${Date.now()}.zip`;
    document.body.appendChild(a);
    a.click();
    window.URL.revokeObjectURL(url);
    document.body.removeChild(a);

    showAlert("Invoices downloaded successfully", "success");
  } catch (error) {
    console.error("Error downloading invoices:", error);
    showAlert("Failed to download invoices", "danger");
  }
}

/**
 * Add a new payment method
 */
async function addPaymentMethod() {
  const orgId = getOrgIdFromContext();

  try {
    // Use the new endpoint that doesn't require subscription ID
    const portalUrl = `/devportal/organizations/${orgId}/billing-portal`;

    const response = await fetch(portalUrl, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      console.error("Failed to create portal session:", errorText);
      throw new Error("portal_session_failed");
    }

    const data = await response.json();
    if (data.url) {
      // Open Stripe portal in new tab instead of redirecting
      window.open(data.url, "_blank", "noopener,noreferrer");
      showAlert("Billing portal opened in new tab", "success");
    } else {
      throw new Error("No portal URL returned");
    }
  } catch (error) {
    showAlert(
      "Failed to open payment method management. Please try again.",
      "danger",
    );
  }
}

/**
 * Edit billing information
 */
async function editBillingInfo() {
  const orgId = getOrgIdFromContext();
  try {
    // First, get active subscriptions to find a subscription ID
    const subsResponse = await fetch(
      `/devportal/organizations/${orgId}/subscriptions`,
    );
    if (!subsResponse.ok) {
      throw new Error("Failed to get subscriptions");
    }

    const subsData = await subsResponse.json();

    // Check if there are any subscriptions
    if (!subsData.subscriptions || subsData.subscriptions.length === 0) {
      showAlert(
        "No subscriptions found. Please create a subscription first.",
        "warning",
      );
      return;
    }

    // Use the first subscription's ID
    const subId = subsData.subscriptions[0].id;

    // Redirect to Stripe Customer Portal for billing info management
    const response = await fetch(
      `/devportal/organizations/${orgId}/subscriptions/${subId}/billing-portal`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
      },
    );

    if (!response.ok) {
      throw new Error("Failed to create portal session");
    }

    const data = await response.json();
    // Open Stripe portal in new tab instead of redirecting
    window.open(data.url, "_blank", "noopener,noreferrer");
    showAlert("Billing portal opened in new tab", "success");
  } catch (error) {
    console.error("Error editing billing info:", error);
    showAlert("Failed to open billing information editor", "danger");
  }
}

/**
 * Manage a subscription
 */
async function manageSubscription(subscriptionId) {
  const orgId = getOrgIdFromContext();
  try {
    // Redirect to Stripe Customer Portal for subscription management
    const response = await fetch(
      `/devportal/organizations/${orgId}/subscriptions/${subscriptionId}/billing-portal`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
      },
    );

    if (!response.ok) {
      throw new Error("Failed to create portal session");
    }

    const data = await response.json();
    // Open Stripe portal in new tab instead of redirecting
    window.open(data.url, "_blank", "noopener,noreferrer");
    showAlert("Billing portal opened in new tab", "success");
  } catch (error) {
    console.error("Error managing subscription:", error);
    showAlert("Failed to open subscription management", "danger");
  }
}

// Utility Functions

function formatNumber(num) {
  return new Intl.NumberFormat().format(num);
}

function formatUsageCost(amount, currency) {
  const c = (currency || "USD").toUpperCase();
  return c + " " + new Intl.NumberFormat("en-US", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(amount);
}

function formatCurrency(amount, currency) {
  const c = (currency || "usd").toUpperCase();
  return c + " " + new Intl.NumberFormat("en-US", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(amount / 100); // Stripe amounts are in cents
}

function formatDate(timestamp) {
  // If timestamp is in milliseconds (e.g., > 10^12), use as is; if in seconds, multiply by 1000
  const ts = timestamp > 1e12 ? timestamp : timestamp * 1000;
  return new Date(ts).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function escapeHtml(text) {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

function getInvoiceStatusClass(status) {
  const statusMap = {
    paid: "paid",
    open: "pending",
    void: "failed",
    uncollectible: "failed",
    draft: "pending",
  };
  return statusMap[status] || "pending";
}

function getOrgIdFromContext() {
  // Get orgId from main or div element's data attribute
  const element = document.querySelector("[data-org-id]");
  if (element && element.dataset.orgId) {
    return element.dataset.orgId;
  }

  // Fallback: try to extract from URL path (e.g., /hiranyakavi/views/default/billing)
  const pathParts = window.location.pathname.split("/");
  if (pathParts.length > 1 && pathParts[1]) {
    return pathParts[1];
  }
  return "default";
}

function showAlert(message, type = "info") {
  const alertContainer = document.querySelector(".alert-container");
  if (alertContainer) {
    const escapedMessage = escapeHtml(message);
    const alertDiv = document.createElement("div");
    alertDiv.className = `alert alert-${type} alert-dismissible fade show`;
    alertDiv.setAttribute("role", "alert");
    alertDiv.innerHTML = `
      ${escapedMessage}
      <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
    `;
    alertContainer.appendChild(alertDiv);

    setTimeout(() => {
      alertDiv.classList.remove("show");
      setTimeout(() => alertDiv.remove(), 150);
    }, 5000);
  }
}

// Expose functions to global scope for onclick handlers
window.loadUsageData = loadUsageData;
window.loadInvoices = loadInvoices;
window.loadPaymentDetails = loadPaymentDetails;
window.loadPaymentMethods = loadPaymentMethods;
window.loadBillingInfo = loadBillingInfo;
window.loadActiveSubscriptions = loadActiveSubscriptions;
window.downloadInvoice = downloadInvoice;
window.downloadAllInvoices = downloadAllInvoices;
window.addPaymentMethod = addPaymentMethod;
window.editBillingInfo = editBillingInfo;
window.manageSubscription = manageSubscription;
