document.addEventListener("DOMContentLoaded", () => {
  checkQueryParamsAndLoadModal();
  wireStripeReturnIfPresent();

  // Set up the delete subscription handler
  setDeleteConfirmationHandler('removeSubscription', function(data) {
    removeSubscription(data.orgID, data.appID, data.apiRefID, data.subID);
  });

  // Set up event listeners for unsubscribe buttons
  document.addEventListener('click', function(e) {
    const btn = e.target.closest('[data-unsubscribe-id]');
    if (btn) {
      const subID = btn.getAttribute('data-unsubscribe-id');
      const orgID = btn.getAttribute('data-org-id');
      const appID = btn.getAttribute('data-app-id');
      const apiRefID = btn.getAttribute('data-api-ref-id');
      openDeleteModal(subID, orgID, appID, apiRefID);
    }
  });

  // Show subscription success message if redirected after payment or free flow
  const urlParams = new URLSearchParams(window.location.search);
  if (urlParams.get('subscription') === 'success') {
    // Try to find the message overlay and card for UI update
    const messageOverlay = document.getElementById('subscription-message-overlay') || null;
    showSubscriptionMessage(messageOverlay, "Successfully subscribed", "success");
    // Optionally, mark the UI as subscribed if you can get the card and applicationID
    // Example: markSubscribedUI(card, applicationID); // You may need to adapt this line
    // Clean up the query param so the message doesn't show again on reload
    urlParams.delete('subscription');
    const url = new URL(window.location.href);
    url.searchParams.delete('subscription');
    window.history.replaceState({}, document.title, url.toString());
  }
});

// backend endpoints (keep your existing base)
const DEVPORTAL_BASE = "/devportal";

// UI behavior
const POLL_AFTER_PAYMENT_MS = 2500;
const POLL_MAX_ATTEMPTS = 20;

// Store the Stripe checkout instance globally so we can destroy it
let currentStripeCheckout = null;

// Flag to prevent duplicate payment processing
let isProcessingPayment = false;

// If you render these into the template, use them.
// Example in HBS:
// <script>
//   window.__API_META__ = {{{json apiMetadata}}};
// </script>
function getApiMeta() {
  return window.__API_META__ || null;
}

function isMonetizationEnabled() {
  const meta = getApiMeta();
  return !!meta?.monetizationInfo?.enabled;
}

function getBillingEngine() {
  const meta = getApiMeta();
  return (meta?.monetizationInfo?.billingEngine || "").toUpperCase();
}

/**
 * Each plan card should provide whether it's paid.
 * Recommended: pass isPaid + priceId from backend and render them as data attributes.
 * For example in HBS:
 *   <a ... data-is-paid="{{isPaid}}" data-price-id="{{priceId}}" ...>
 */
function isPaidPlan(policyId) {
  const el = document.getElementById(`subscribe-btn-${policyId}`);
  if (!el) return false;
  const val = el.getAttribute("data-is-paid");
  return val === "true" || val === "1";
}

function getPriceId(policyId) {
  const el = document.getElementById(`subscribe-btn-${policyId}`);
  if (!el) return null;
  return el.getAttribute("data-external-price-id") || null;
}

/**
 * Strong sanitization for IDs used in requests.
 */
function safeId(input) {
  if (!input) return "";
  return String(input).replace(/[^a-zA-Z0-9\s-_.:]/g, "");
}

function safeText(input) {
  if (!input) return "";
  // allow hyphen/dot/space
  return String(input).replace(/[^a-zA-Z0-9\s-_.]/g, "");
}

function checkQueryParamsAndLoadModal() {
    const urlParams = new URLSearchParams(window.location.search);
    const subPlan = urlParams.get('policyName');
    const apiName = urlParams.get('apiName');
    const apiVersion = urlParams.get('apiVersion');

    if (subPlan && apiName && apiVersion) {
        const modal = document.getElementById('planModal');
        modal.style.display = 'block';

        const planName = document.getElementById('planName');
        planName.innerText = subPlan;
    }
}

async function unsubscribe(subscriptionId) {
    try {
        const response = await fetch(`/devportal/subscriptions/${subscriptionId}`, {
            method: 'DELETE',
            headers: {
                'Content-Type': 'application/json',
            },
        });

        if (response.ok) {
            await showAlert(`Unsubscribed successfully!`, 'success');
            const url = new URL(window.location.origin + window.location.pathname);
            window.location.href = url.toString();
        } else {
            const responseData = await response.json();
            console.error('Failed to unsubscribe:', responseData);
            await showAlert(`Failed to unsubscribe.\n${responseData.description}`, 'error');
        }
    } catch (error) {
        console.error('Error:', error);
        await showAlert(`An error occurred.\n${error.message}`, 'error');
    }
}

function hideElementById(elementId) {
    const element = document.getElementById(elementId);
    if (element) {
        element.style.display = 'none';
    }
}

function closeModal(elementId) {
    hideElementById(elementId);

    // Reset any subscribe buttons left in loading state
    var modalEl = elementId ? document.getElementById(elementId) : null;
    if (modalEl) {
        modalEl.querySelectorAll('.subscription-plan-subscribe-btn, .subscribe-btn').forEach(function(btn) {
            if (btn.dataset.originalText) {
                btn.innerHTML = btn.dataset.originalText;
                btn.disabled = false;
                delete btn.dataset.originalText;
            } else if ((btn.textContent || '').trim() === 'Subscribing...') {
                btn.textContent = 'Subscribe';
                btn.disabled = false;
            }
        });
    }

    // Scope form reset to within the modal, not document-wide
    var form = modalEl ? modalEl.querySelector('form') : null;
    if (form) form.reset();

    if (elementId) {
        // Clear inline token display so it's fresh next time the modal opens
        var apiId = elementId.replace('planModal-', '');
        var tokenArea = document.getElementById('subscriptionTokenArea-' + apiId);
        if (tokenArea) { tokenArea.innerHTML = ''; tokenArea.style.display = 'none'; }

        if (window.__platformSubscriptionChanged) {
            window.__platformSubscriptionChanged = false;
            var apiCard = document.getElementById('apiCard-' + apiId);
            if (apiCard) {
                var flag = apiCard.querySelector('.subscription-flag');
                if (flag) {
                    var hasActiveSubs = (window.existingPlatformSubscriptions || []).some(function(s) {
                        return s.status === 'ACTIVE';
                    });
                    flag.style.display = hasActiveSubs ? 'block' : 'none';
                }
            }
        }
    }
}

window.onclick = function (event) {
    const modal = document.getElementById('planModal');
    if (event.target === modal) {
        closeModal();
    }
};

// Function to show the application creation form
function showApplicationForm() {
    const creationForm = document.getElementById('applicationFormCreation');
    creationForm.style.display = 'block';

    hideElementById('applicationFormSection');

    const subButton = document.getElementById('createSubButton');
    subButton.style.display = 'block';
}

// Function to handle application creation
async function handleCreateSubscribe() {
    const urlParams = new URLSearchParams(window.location.search);
    try {
        // Sanitize the application name
        const appName = document.getElementById('appName').value.trim().replace(/[^a-zA-Z0-9\s]/g, '');
        if (!appName) {
            await showAlert('Application name cannot be empty.', 'error');
            return;
        }

        const response = await fetch(`/devportal/applications`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ name: sanitize(appName) }),
        });

        const responseData = await response.json();

        if (response.ok) {
            handleSubscribe(responseData.applicationId);
        } else {
            console.error('Failed to create application:', responseData);
            await showAlert(`Failed to create application. Please try again.\n${responseData.description}`, 'error');
        }
    } catch (error) {
        console.error('Error:', error);
        await showAlert(`An error occurred while subscribing: \n${error.message}`, 'error');
    }
}

function sanitize(input) {
    const div = document.createElement('div');
    div.appendChild(document.createTextNode(input));
    return div.innerHTML;
}

async function handleSubscribe(appId, apiName, apiVersion, apiRefId) {
    const applicationSelect = document.getElementById('applicationSelect');
    let policyName;

    const applicationId = appId !== null
        ? appId
        : (applicationSelect ? applicationSelect.value : window.location.pathname.split('/').pop());


    if (!applicationId) {
        await showAlert('Please select an application.', 'error');
        return;
    }

    try {
        const urlParams = new URLSearchParams(window.location.search);

        if (urlParams.has('apiName') && urlParams.has('policyName')) {
            apiName = urlParams.get('apiName');
            apiVersion = urlParams.get('apiVersion');
            policyName = urlParams.get('policyName');
        } else {
            policyName = document.getElementById('subscriptionPlan').value;
        }

        const response = await fetch(`/devportal/subscriptions`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                applicationId: applicationId.replace(/[^a-zA-Z0-9\s-]/g, ''),
                apiName: apiName.replace(/[^a-zA-Z0-9\s-]/g, ''),
                apiVersion: apiVersion.replace(/[^a-zA-Z0-9\s-.]/g, ''),
                throttlingPolicy: policyName.replace(/[^a-zA-Z0-9\s-]/g, ''),
                apiRefId: apiRefId,
            }),
        });

        const responseData = await response.json();
        if (response.ok) {
            await showAlert('Subscribed successfully!', 'success');
            const url = new URL(window.location.origin + window.location.pathname);
            window.location.href = url.toString();
        } else {
            console.error('Failed to subscribe:', responseData);
            await showAlert(`Failed to subscribe. Please try again.\n${responseData.description}`, 'error');
        }
    } catch (error) {
        console.error('Error:', error);
        await showAlert(`An error occurred while subscribing: \n${error.message}`, 'error');
    }
}

function loadModal(modalID) {
    const modal = document.getElementById(modalID);
    modal.style.display = 'flex';

    if (typeof prepareSubscriptionModal === 'function') {
        try { prepareSubscriptionModal(modalID); } catch (e) { /* noop */ }
    }
}

function handleAppBasedSubscription(btnElement) {
    // If a plan modal exists for this API, open the modal to let the user choose/create app and confirm.
    const { orgId, apiId, apiReferenceId, policyId, policyName } = btnElement.dataset;
    const modalId = 'planModal-' + apiId;
    if (document.getElementById(modalId)) {
        loadModal(modalId);
        return;
    }

    showSubscribeButtonLoading(btnElement);
    subscribe(orgId, '', apiId, apiReferenceId, policyId, policyName);
}

async function subscribe(orgID, applicationID, apiId, apiReferenceID, policyId, policyName, buttonElement) {
  const card = (buttonElement && buttonElement.closest('.subscription-card'))
    ? buttonElement.closest('.subscription-card')
    : getSubscriptionCard(apiId, policyId);
  const subscribeButton = card ? card.querySelector(".common-btn-primary") : null;
  const messageOverlay = card ? card.querySelector(".message-overlay") : null;

  try {
    if (!applicationID) {
      const policyField = card
        ? card.querySelector('input[type="hidden"][id^="selectedAppId-"]')
        : document.getElementById('selectedAppId-' + policyId);
      if (policyField?.value) {
        applicationID = policyField.value;
      } else if (card) {
        const hiddenField = card.querySelector('input[type="hidden"]');
        if (hiddenField?.value) applicationID = hiddenField.value;
      }
    }

    if (!applicationID) {
      const modal = card ? card.closest('.subscription-plan-modal') : document.getElementById('planModal-' + apiId);
      const isPlatformGateway = modal && modal.dataset.gatewayType === 'wso2/api-platform';
      if (!isPlatformGateway) {
        showSubscriptionMessage(messageOverlay, "Please select an application.", "error");
        return;
      }
    }

    const paid = isPaidPlan(policyId);

    // Always reset UI button state safely on exit paths
    const resetUi = () => {
      const planButton = document.getElementById("subscribe-btn-" + policyId);
      if (planButton) resetSubscribeButtonState(planButton);
      resetSubscribeButtonState(subscribeButton);
    };

    // ✅ FREE FLOW: create DP subscription immediately (existing behavior)
    if (!paid) {
      const response = await fetch(`${DEVPORTAL_BASE}/organizations/${safeId(orgID)}/subscriptions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          applicationID: safeId(applicationID),
          apiId: safeId(apiId),
          apiReferenceID: safeId(apiReferenceID),
          policyId: safeId(policyId),
          policyName: safeText(policyName),
        }),
      });

      const responseData = await response.json().catch(() => ({}));
      resetUi();

      if (response.ok) {
        showSubscriptionMessage(messageOverlay, "Successfully subscribed", "success");
        closeModal('planModal-' + apiId);
        markSubscribedUI(card, applicationID, apiId);
      } else {
        console.error("Failed to create subscription:", responseData);
        const errMsg = responseData.message || responseData.description || responseData.error || "Subscription failed. Please try again.";
        showSubscriptionMessage(messageOverlay, errMsg, "error");
      }
      return;
    }

    // ✅ PAID FLOW: start Stripe embedded checkout first
    const priceId = getPriceId(policyId);
    if (!priceId) {
      resetUi();
      showSubscriptionMessage(messageOverlay, "Paid plan misconfigured (missing priceId).", "error");
      return;
    }

    // Call backend to create embedded checkout session
    const checkoutRes = await fetch(`${DEVPORTAL_BASE}/organizations/${safeId(orgID)}/monetization/checkout`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        applicationID: safeId(applicationID),
        apiId: safeId(apiId),
        apiReferenceID: safeId(apiReferenceID),
        policyId: safeId(policyId),
        policyName: safeText(policyName),
        priceId: safeId(priceId),
        sourcePage: window.location.pathname,
      }),
    });

    const checkoutData = await checkoutRes.json().catch(() => ({}));
    resetUi();

    if (!checkoutRes.ok) {
      console.error("Checkout creation failed:", checkoutData);
      const errMsg = checkoutData.message || checkoutData.error || checkoutData.description || "Failed to start checkout. Please try again.";
      // Treat an "already active" conflict as success info
      if (checkoutRes.status === 409) {
        showSubscriptionMessage(messageOverlay, errMsg, "info");
      } else {
        showSubscriptionMessage(messageOverlay, errMsg, "error");
      }
      return;
    }

    // Expect backend returns:
    // { clientSecret, publishableKey, checkoutContextId, returnUrl }
    const { clientSecret, publishableKey, checkoutContextId, returnUrl } = checkoutData;

    if (!clientSecret || !publishableKey) {
      showSubscriptionMessage(messageOverlay, "Checkout session response missing clientSecret/publishableKey.", "error");
      return;
    }

    // Open embedded checkout UI (modal)
    await openStripeEmbeddedCheckout({
      publishableKey,
      clientSecret,
      orgID,
      checkoutContextId,
      returnUrl,
    });

    // After checkout completes, user will be redirected or we can poll (see wireStripeReturnIfPresent)
  } catch (error) {
    console.error("Error:", error);
    showSubscriptionMessage(messageOverlay, `Error while subscribing: ${error.message}`, "error");
  }
}

/**
 * =========================
 *  Stripe Embedded Checkout (UI)
 * =========================
 * Requires you to include Stripe JS in layout:
 * <script src="https://js.stripe.com/v3/"></script>
 *
 * And create a modal container in your HBS layout:
 *  <div id="stripeCheckoutModal" class="modal hidden">
 *     <div class="modal-content">
 *        <button onclick="closeStripeCheckoutModal()">×</button>
 *        <div id="stripe-embedded-checkout"></div>
 *     </div>
 *  </div>
 */
async function openStripeEmbeddedCheckout({ publishableKey, clientSecret, returnUrl }) {
  if (typeof publishableKey !== 'string' || publishableKey.length < 10) {
    console.warn("publishableKey seems too short or invalid");
  }
  if (typeof clientSecret !== 'string' || clientSecret.length < 10) {
    console.warn("clientSecret seems too short or invalid");
  }
  
  // Validate inputs
  if (!publishableKey) {
    await showAlert("Stripe publishable key is missing.", "error");
    return;
  }
  if (!clientSecret) {
    await showAlert("Stripe client secret is missing.", "error");
    return;
  }
  
  // If your UX is: redirect to a dedicated page, you can do:
  // window.location.href = returnUrl;
  // But since you want embedded, do in a modal.

  if (!window.Stripe) {
    await showAlert("Stripe.js not loaded. Add https://js.stripe.com/v3/ to layout.", "error");
    return;
  }
  const stripe = window.Stripe(publishableKey);
  const modal = document.getElementById("stripeCheckoutModal");
  const mountPoint = document.getElementById("stripe-embedded-checkout");
  if (!modal || !mountPoint) {
    await showAlert("Stripe checkout modal container is missing in template.", "error");
    return;
  }

  // Destroy previous checkout instance if it exists
  if (currentStripeCheckout) {
    try {
      currentStripeCheckout.destroy();
    } catch (e) {
      console.warn("Error destroying previous checkout:", e);
    }
    currentStripeCheckout = null;
  }

  // clear previous mount
  mountPoint.innerHTML = "";

  modal.classList.remove("hidden");
  modal.style.display = "flex";

  try {
    const checkout = await stripe.initEmbeddedCheckout({ clientSecret });

    // Store the checkout instance globally
    currentStripeCheckout = checkout;
    checkout.mount("#stripe-embedded-checkout");
  } catch (e) {
    console.error("Failed to init embedded checkout - Full error:", {
      name: e.name,
      message: e.message,
      type: e.type,
      code: e.code,
      decline_code: e.decline_code,
      param: e.param,
      stack: e.stack
    });

    let errorMessage = "Failed to load checkout UI";
    if (e.message) {
      errorMessage += ": " + e.message;
    }
    if (e.type === 'invalid_request_error') {
      errorMessage = "Invalid checkout session. Please try again.";
    }

    await showAlert(errorMessage, "error");
    closeStripeCheckoutModal();
  }
}

function closeStripeCheckoutModal() {  
  // Destroy the checkout instance when closing
  if (currentStripeCheckout) {
    try {
      currentStripeCheckout.destroy();
    } catch (e) {
      console.warn("Error destroying checkout:", e);
    }
    currentStripeCheckout = null;
  }
  
  const modal = document.getElementById("stripeCheckoutModal");
  if (modal) {
    modal.classList.add("hidden");
    modal.style.display = "none";
  }
  
  // Clear the mount point
  const mountPoint = document.getElementById("stripe-embedded-checkout");
  if (mountPoint) {
    mountPoint.innerHTML = "";
  }

  document.querySelectorAll('.subscription-plan-subscribe-btn, .subscribe-btn, [id^="landing-subscribe-btn-"], [id^="subscribe-btn-"]').forEach(function(btn) {
    if (btn.dataset.originalText) {
      btn.innerHTML = btn.dataset.originalText;
      btn.disabled = false;
      delete btn.dataset.originalText;
    }
  });
}

/**
 * =========================
 *  After Return: confirm checkout + refresh UI
 * =========================
 * Called when Stripe redirects back after payment completion.
 * This finalizes the subscription by calling the register endpoint.
 */
async function wireStripeReturnIfPresent() {
  const params = new URLSearchParams(window.location.search);
  const sessionId = params.get("session_id");
  const dpSubId = params.get("dp_sub_id");
  const orgId = params.get("org_id");
  
  if (!sessionId) return;

  // Prevent duplicate processing
  if (isProcessingPayment) {
    return;
  }

  isProcessingPayment = true;

  if (!orgId) {
    console.error("❌ Missing org_id in URL parameters");
    await showAlert("Missing organization ID. Cannot complete subscription.", "error");
    isProcessingPayment = false;
    return;
  }

  // Show a loading indicator
  showAlert("Processing your payment...", "info");

  const sourcePage = params.get("sourcePage");
  let redirectTarget = null;
  if (sourcePage) {
    try {
      const parsed = new URL(sourcePage, window.location.origin);
      if (parsed.origin === window.location.origin) {
        redirectTarget = parsed.pathname + parsed.search;
      }
    } catch (_) {
      // handle exception
    }
  }

  function buildCleanRedirectUrl() {
    if (redirectTarget) return redirectTarget;
    const url = new URL(window.location.href);
    url.searchParams.delete("session_id");
    url.searchParams.delete("dp_sub_id");
    url.searchParams.delete("org_id");
    url.searchParams.delete("sourcePage");
    return url.toString();
  }

  try {
    // Call register endpoint to finalize the subscription
    const response = await fetch(`${DEVPORTAL_BASE}/organizations/${safeId(orgId)}/monetization/stripe/register/${encodeURIComponent(sessionId)}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
    });

    if (response.ok) {
      const result = await response.json();
      await showAlert("Payment successful! Your subscription is now active.", "success");
      
      // Wait a moment before redirecting
      await new Promise(r => setTimeout(r, 2000));
      
      // Redirect to source page
      window.location.href = buildCleanRedirectUrl();
    } else {
      const errorData = await response.json().catch(() => ({}));
      const errMsg = errorData.message || errorData.error || "An error occurred while activating your subscription.";
      // If already active (conflict), treat as success
      if (response.status === 409 && errMsg.toLowerCase().includes("active")) {
        await showAlert("Subscription already active.", "success");
        await new Promise(r => setTimeout(r, 1500));
        // Terminal outcome — redirect to source page
        window.location.href = buildCleanRedirectUrl();
      } else {
        // Non-terminal error — keep checkout params so user can retry
        await showAlert("Unable to complete payment", "error");
      }
      isProcessingPayment = false;
    }
  } catch (error) {
    await showAlert("Error processing payment", "error");
    isProcessingPayment = false;
  }
}

async function updateSubscription(orgID, applicationID, apiId, apiReferenceID, policyId, policyName, oldPolicyName, subID) {

    // Find the related card and button elements
    const card = getSubscriptionCard(apiId, policyId);
    const subscribeButton = card ? card.querySelector(".common-btn-primary") : null;
    const messageOverlay = card ? card.querySelector(".message-overlay") : null;

    try {
        // Get application ID from hidden field if not provided
        if (!applicationID && card) {
            const hiddenField = card.querySelector('input[type="hidden"]');
            if (hiddenField && hiddenField.value) {
                applicationID = hiddenField.value;
            }
        }

        // Make the API request
        const response = await fetch(`/devportal/organizations/${orgID}/subscriptions`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ applicationID, apiId, apiReferenceID, policyId, policyName }),
        });

        const responseData = await response.json();

        // Always reset button state and close modal
        const updatedButton = document.getElementById('subscribe-btn-' + policyId);
        if (updatedButton) {
            resetSubscribeButtonState(updatedButton);
        }
        // closeModal('planModal-' + apiId);
        resetSubscribeButtonState(subscribeButton);

        if (response.ok) {

            // Show success notification — api-card elements exist on the API listing page only
            const updatedPlanCard = document.getElementById('api-card-' + apiId + "-" + policyName);
            if (updatedPlanCard) {
                updatedPlanCard.style.borderColor = 'var(--primary-main-color)';
            }

            if (updatedButton) {
                updatedButton.setAttribute('disabled', 'disabled');
                updatedButton.classList.add('disabled');
                updatedButton.style.setProperty('pointer-events', 'none', 'important');
                updatedButton.textContent = 'Updated';
            }

            const apiCards = document.querySelectorAll(`[id^="api-card-${apiId}-"]`);
            apiCards.forEach(card => {
                if (card.id !== `api-card-${apiId}-${policyName}`) {
                    card.style.borderColor = '';
                    const cardButton = card.querySelector('a[type="button"]');
                    if (cardButton) {
                        cardButton.style.pointerEvents = 'auto';
                        cardButton.removeAttribute('disabled');
                        cardButton.classList.remove('disabled');
                    }
                }
            });
            const policyCell = document.getElementById('policy_' + subID);
            if (policyCell) {
                policyCell.textContent = policyName;
            }
            showSubscriptionMessage(messageOverlay, 'Successfully updated API subscription', 'success');
        } else {
            // Handle API error
            console.error('Failed to create subscription:', responseData);
            const errorMessage = `Failed to subscribe: ${responseData.description || 'Unknown error'}`;
            showSubscriptionMessage(messageOverlay, errorMessage, 'error');
        }
    } catch (error) {
        // Handle exceptions
        console.error('Error:', error);

        // Always reset button state and close modal
        const planButton = document.getElementById('subscribe-btn-' + policyId);
        if (planButton) {
            resetSubscribeButtonState(planButton);
        }
        closeModal('planModal-' + apiId);
        resetSubscribeButtonState(subscribeButton);

        const errorMessage = `Error while updating subscription: ${error.message}`;
        showSubscriptionMessage(messageOverlay, errorMessage, 'error');
    }
}

// Helper functions for the subscribe function
function getSubscriptionCard(apiId, policyId) {
    return document.getElementById('subscriptionCard-' + policyId) ||
        document.getElementById('apiCard-' + apiId) ||
        null;
}

function resetSubscribeButtonState(button) {
    if (button && typeof window.resetSubscribeButtonState === 'function') {
        window.resetSubscribeButtonState(button);
    }
}

function showSubscriptionMessage(messageOverlay, message, type) {
    if (messageOverlay && typeof window.showApiMessage === 'function') {
        window.showApiMessage(messageOverlay, message, type);
    } else {
        showAlert(message, type);
    }
}

function addAPISubscription(selectElement) {
    const selectedOption = selectElement.options[selectElement.selectedIndex];
    const subscriptionPolicies = JSON.parse(selectedOption.getAttribute("data-policies") || "[]");
    const planSelect = document.getElementById("planSelect");

    planSelect.innerHTML = '<option value="" disabled selected>Select a Plan</option>';

    subscriptionPolicies.forEach(policy => {
        const option = document.createElement("option");
        option.value = policy.policyID;
        option.textContent = policy.displayName;
        option.setAttribute("data-policyName", policy.policyName);
        planSelect.appendChild(option);
    });

}

async function removeSubscription(orgID, appID, apiRefID, subID) {

    try {
        const response = await fetch(`/devportal/organizations/${orgID}/subscriptions?appID=${appID}&apiReferenceID=${apiRefID}&subscriptionID=${subID}`, {
            method: 'DELETE',
            headers: {
                'Content-Type': 'application/json',
            },
        });
        if (response.ok) {
            await showAlert(`Unsubscribed successfully!`, 'success');
            document.getElementById(`data-row-${subID}`)?.remove();
            const rowCount = document.getElementById(`app-table-${appID}`).rows.length;
            if (rowCount === 1) {
                document.getElementById(`app-table-${appID}`).remove();
                document.getElementById('no-subscription').style.display = 'block';
            }
            const mcpTable = document.getElementById(`app-table-mcp-${appID}`);
            if (mcpTable && mcpTable.rows.length === 1) {
                mcpTable.remove();
                document.getElementById('no-subscription-mcp').style.display = 'block';
            }
        } else {
            const responseData = await response.json();
            console.error('Failed to unsubscribe:', responseData);
            await showAlert(`Failed to unsubscribe.\n${responseData.description}`, 'error');
        }
    } catch (error) {
        console.error('Error:', error);
        await showAlert(`An error occurred.\n${error.message}`, 'error');
    }
}

/**
 * Open the delete confirmation modal for removing a subscription
 * @param {string} subID - Subscription ID
 * @param {string} orgID - Organization ID
 * @param {string} appID - Application ID
 * @param {string} apiRefID - API Reference ID
 */
function openDeleteModal(subID, orgID, appID, apiRefID) {
    const modal = document.getElementById('deleteConfirmation');
    if (!modal) {
        console.error('openDeleteModal: Modal not found');
        return;
    }

    const titleEl = modal.querySelector('.modal-title');
    const messageEl = modal.querySelector('.modal-message');
    if (titleEl) titleEl.textContent = 'Do you really want to remove the subscription?';
    if (messageEl) messageEl.textContent = 'This will remove the subscription entry stored in the devportal.';

    setDeleteConfirmationAction('removeSubscription', {
        subID: subID,
        orgID: orgID,
        appID: appID,
        apiRefID: apiRefID
    });

    const bootstrapModal = new bootstrap.Modal(modal);
    bootstrapModal.show();
}

function markSubscribedUI(card, applicationID, apiId) {
  if (!card) return;

  const subscriptionFlag = card.querySelector(".subscription-flag");
  if (subscriptionFlag) subscriptionFlag.style.display = "block";

  if (apiId) {
    const apiCard = document.getElementById('apiCard-' + apiId);
    if (apiCard) {
      const listingFlag = apiCard.querySelector('.subscription-flag');
      if (listingFlag) listingFlag.style.display = 'block';
    }
  }

  // Scope dropdown marking to both modal and card containers (like upstream)
  var apiContainer = apiId
    ? (document.getElementById('planModal-' + apiId) || document.getElementById('apiCard-' + apiId))
    : null;
  var allDropdowns = apiContainer
    ? apiContainer.querySelectorAll('.custom-dropdown')
    : card.querySelectorAll('.custom-dropdown');

  allDropdowns.forEach(function(dropdown) {
    const appOption = dropdown.querySelector('.select-item[data-value="' + applicationID + '"]');
    if (appOption) {
      let subscriptionIcon = appOption.querySelector(".subscription-icon");
      if (subscriptionIcon) {
        subscriptionIcon.style.display = "inline-block";
      } else {
        const subscriptionIconHtml = '<img src="https://raw.githubusercontent.com/wso2/docs-bijira/refs/heads/main/en/devportal-theming/success-rounded.svg"'
          + ' alt="Subscribed" class="subscription-icon" style="display: inline-block;" />';
        const tempDiv = document.createElement("div");
        tempDiv.innerHTML = subscriptionIconHtml;
        subscriptionIcon = tempDiv.firstElementChild;
        appOption.appendChild(subscriptionIcon);
      }
      appOption.classList.add("disabled");
    }
  });

  const btn = card.querySelector(".common-btn-primary");
  if (btn) btn.setAttribute("disabled", "disabled");
}

/**
 * =========================
 *  Handler for Plan Selection Modal
 *  - Called from subscription-plans.hbs modal
 * =========================
 */
function handlePlanSubscription(buttonElement) {

  const orgID = buttonElement.dataset.orgId;
  const apiID = buttonElement.dataset.apiId;
  const policyID = buttonElement.dataset.policyId;
  const policyName = buttonElement.dataset.policyName;
  const isPaid = buttonElement.dataset.isPaid === 'true';

  // Resolve apiReferenceID from the parent modal's data attribute
  const modal = buttonElement.closest('.subscription-plan-modal');
  const apiReferenceID = (modal && modal.dataset.apiRefid) || apiID;

  const card = buttonElement.closest('.subscription-card');
  const perPlanField = card
    ? card.querySelector(`input[type="hidden"][id^="selectedAppId-"]`)
    : document.getElementById(`selectedAppId-${policyID}`);
  const modalAppField = document.getElementById(`modal-selected-app-${apiID}`);

  const applicationID = (perPlanField && perPlanField.value) ? perPlanField.value
    : (modalAppField ? modalAppField.value : '');

  if (!applicationID && !isPaid) {
    // Platform flows don't use an app selector; app-based free plans require one.
    const isPlatformGateway = modal && modal.dataset.gatewayType === 'wso2/api-platform';
    if (!isPlatformGateway) {
      showAlert('Please select an application first.', 'error');
      return;
    }
  }

  if (isPaid) {
    subscribeAndCheckout(orgID, apiID, apiReferenceID, policyID, policyName, applicationID);
  } else {
    // Free plan - direct subscription
    subscribe(orgID, applicationID, apiID, apiReferenceID, policyID, policyName);
  }
}

/**
 * =========================
 *  Handler for API Landing Page (single plan or paid plan)
 *  - Called from api-subscription-plans.hbs
 * =========================
 */
function handleSubscribeWithPayment(orgID, apiID, apiReferenceID, policyID, policyName, buttonElement) {
  const isPaid = buttonElement.dataset.isPaid === 'true';
  
  if (isPaid) {
    // Paid plan - trigger Stripe checkout flow
    subscribeAndCheckout(orgID, apiID, apiReferenceID, policyID, policyName);
  } else {
    // Free plan - direct subscription
    const applicationID = ''; // Will be extracted from hidden field in subscribe()
    subscribe(orgID, applicationID, apiID, apiReferenceID, policyID, policyName);
  }
}

/**
 * =========================
 *  Landing Page Subscribe (Free plans)
 *  - Uses landing-prefixed element IDs
 * =========================
 */
function landingSubscribe(orgID, apiID, apiReferenceID, policyID, policyName) {
  // Get application ID from landing page hidden field
  const hiddenField = document.getElementById(`landing-selectedAppId-${policyID}`);
  const applicationID = hiddenField?.value || '';
  
  if (!applicationID) {
    showAlert("Please select an application.", "error");
    // Reset button state
    const btn = document.getElementById(`landing-subscribe-btn-${policyID}`);
    if (btn) {
      btn.innerHTML = 'Subscribe';
      btn.disabled = false;
    }
    return;
  }
  
  // Call the main subscribe function
  subscribe(orgID, applicationID, apiID, apiReferenceID, policyID, policyName);
}

/**
 * =========================
 *  Handler for API Landing Page Plan Subscription
 *  - Similar to handlePlanSubscription but uses landing-prefixed IDs
 * =========================
 */
function handleLandingPlanSubscription(orgID, apiID, apiReferenceID, policyID, policyName, buttonElement) {
  if (buttonElement && buttonElement.dataset.originalText) return;

  showSubscribeButtonLoading(buttonElement);

  const isPaid = buttonElement.dataset.isPaid === 'true';  
  // Get application ID from landing page hidden field
  const hiddenField = document.getElementById(`landing-selectedAppId-${policyID}`);
  const applicationID = hiddenField?.value || '';

  if (!applicationID) {
    showAlert('Please select an application first.', 'error');
    resetSubscribeButtonState(buttonElement);
    return;
  }

  if (isPaid) {
    landingSubscribeAndCheckout(orgID, apiID, apiReferenceID, policyID, policyName, buttonElement);
  } else {
    subscribe(orgID, applicationID, apiID, apiReferenceID, policyID, policyName);
  }
}

/**
 * =========================
 *  Landing Page Stripe Checkout Flow
 *  - Uses landing-prefixed element IDs
 * =========================
 */
async function landingSubscribeAndCheckout(orgID, apiID, apiReferenceID, policyID, policyName, buttonElement) {

  const card = document.getElementById(`subscriptionCard-${policyID}`);
  const subscribeButton = buttonElement || (card ? card.querySelector(".common-btn-primary") : null);
  const messageOverlay = card ? card.querySelector(".message-overlay") : null;

  try {
    // Get application ID from landing page hidden field
    const hiddenField = document.getElementById(`landing-selectedAppId-${policyID}`);
    const applicationID = hiddenField?.value || '';
    if (!applicationID) {
      showAlert("Please select an application.", "error");
      resetSubscribeButtonState(subscribeButton);
      return;
    }

    // Get price ID from button data attribute
    const priceId = document.getElementById(`landing-subscribe-btn-${policyID}`)?.getAttribute('data-external-price-id');
    
    if (!priceId) {
      showAlert("Paid plan misconfigured (missing priceId).", "error");
      resetSubscribeButtonState(subscribeButton);
      return;
    }

    // Show loading state (only if not already set by caller)
    if (subscribeButton && !subscribeButton.dataset.originalText) {
      showSubscribeButtonLoading(subscribeButton);
    }

    // Get policy name from button data attribute
    const policyNameAttr = document.getElementById(`landing-subscribe-btn-${policyID}`)?.getAttribute('data-policy-name');

    // Create Stripe checkout session
    const response = await fetch(`${DEVPORTAL_BASE}/organizations/${safeId(orgID)}/monetization/checkout`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        applicationID: safeId(applicationID),
        apiId: safeId(apiID),
        apiReferenceID: safeId(apiReferenceID),
        policyId: safeId(policyID),
        policyName: safeText(policyNameAttr || policyName),
        priceId: safeId(priceId),
        sourcePage: window.location.pathname
      })
    });

    const result = await response.json();
    if (!response.ok) {
      const errMsg = result.message || result.error || "Failed to start checkout. Please try again.";
      if (response.status === 409) {
        showAlert(errMsg, "info");
        resetSubscribeButtonState(subscribeButton);
        return;
      }
      throw new Error(errMsg);
    }

    // Use embedded checkout (same as subscribeAndCheckout)
    const { clientSecret, publishableKey, checkoutContextId, returnUrl } = result;

    if (!clientSecret || !publishableKey) {
      throw new Error("Checkout session response missing clientSecret/publishableKey.");
    }

    // Reset button state before opening modal
    resetSubscribeButtonState(subscribeButton);

    // Open Stripe embedded checkout modal
    await openStripeEmbeddedCheckout({
      publishableKey,
      clientSecret,
      orgID,
      checkoutContextId,
      returnUrl,
    });

  } catch (error) {
    showAlert(error.message || "Failed to initiate checkout", "error");
    resetSubscribeButtonState(subscribeButton);
  }
}

/**
 * =========================
 *  Stripe Checkout Flow (for paid plans)
 * =========================
 */
async function subscribeAndCheckout(orgID, apiID, apiReferenceID, policyID, policyName, applicationID = '') {
  const card = getSubscriptionCard(apiID, policyID);
  const subscribeButton = card ? card.querySelector(".common-btn-primary") : null;
  const messageOverlay = card ? card.querySelector(".message-overlay") : null;

  try {
    if (!applicationID) {
      const perPlanField = card
        ? card.querySelector('input[type="hidden"][id^="selectedAppId-"]')
        : document.getElementById(`selectedAppId-${policyID}`);
      if (perPlanField?.value) applicationID = perPlanField.value;
    }

    if (!applicationID) {
      const modalAppField = document.getElementById(`modal-selected-app-${apiID}`);
      if (modalAppField?.value) applicationID = modalAppField.value;
    }

    // Fall back to card hidden field (for landing page)
    if (!applicationID && card) {
      const hiddenField = card.querySelector('input[type="hidden"]');
      if (hiddenField?.value) applicationID = hiddenField.value;
    }

    if (!applicationID) {
      showAlert("Please select an application.", "error");
      return;
    }

    // Get price ID from button data attribute
    const priceId = document.getElementById(`subscribe-btn-${policyID}`)?.getAttribute('data-external-price-id');
    
    if (!priceId) {
      console.error("❌ No priceId found");
      showAlert("Paid plan misconfigured (missing priceId).", "error");
      return;
    }

    // Call backend to create Stripe checkout session
    const requestBody = {
      applicationID: safeId(applicationID),
      apiId: safeId(apiID),
      apiReferenceID: safeId(apiReferenceID),
      policyId: safeId(policyID),
      policyName: safeText(policyName),
      priceId: safeId(priceId), // <-- ensure priceId is sent
      sourcePage: window.location.pathname, // Store current page to redirect back after payment
    };
    
    const checkoutRes = await fetch(`${DEVPORTAL_BASE}/organizations/${safeId(orgID)}/monetization/checkout`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(requestBody),
    });

    const checkoutData = await checkoutRes.json().catch(() => ({}));
    
    if (!checkoutRes.ok) {
      console.error("❌ Checkout creation failed:", checkoutData);
      const errorMsg = checkoutData.message || checkoutData.error || checkoutData.description || "Failed to start checkout. Please try again.";
      if (checkoutRes.status === 409) {
        showSubscriptionMessage(messageOverlay, errorMsg, "info");
      } else {
        showSubscriptionMessage(messageOverlay, errorMsg, "error");
      }
      return;
    }

    // Expect backend returns: { clientSecret, publishableKey, checkoutContextId, returnUrl }
    const { clientSecret, publishableKey, checkoutContextId, returnUrl } = checkoutData;

    if (!clientSecret || !publishableKey) {
      showSubscriptionMessage(messageOverlay, "Checkout session response missing clientSecret/publishableKey.", "error");
      return;
    }

    // Close the plan selection modal if open (ID is planModal-{apiID})
    closeModal(`planModal-${apiID}`);

    // Open Stripe embedded checkout modal
    await openStripeEmbeddedCheckout({
      publishableKey,
      clientSecret,
      orgID,
      checkoutContextId,
      returnUrl,
    });

  } catch (error) {
    console.error("Error during paid subscription:", error);
    showSubscriptionMessage(messageOverlay, `Error: ${error.message}`, "error");
  }
}

/**
 * =========================
 *  Open modal and pass selected application ID
 * =========================
 */
function openModalWithApp(modalId, appIdFieldId) {
  const appIdField = document.getElementById(appIdFieldId);
  if (!appIdField || !appIdField.value) {
    showAlert('Please select an application first.', 'error');
    return;
  }
  
  const selectedAppId = appIdField.value;
  
  // Extract API ID from modal ID (e.g., "planModal-api123" -> "api123")
  const apiId = modalId.replace('planModal-', '');
  
  // Store the selected app ID in the modal's hidden field
  const modalAppField = document.getElementById(`modal-selected-app-${apiId}`);
  if (modalAppField) {
    modalAppField.value = selectedAppId;
  }
  
  // Open the modal
  loadModal(modalId);
}

/**
 * =========================
 *  Handler for single plan subscription from listing page
 *  - Handles both paid and free plans
 * =========================
 */
function handleListingSubscribe(orgID, apiID, apiReferenceID, policyID, policyName, appIdFieldId, buttonElement) {
  
  const isPaid = buttonElement.dataset.isPaid === 'true';

  // Get selected application ID
  const appIdField = document.getElementById(appIdFieldId);
  const applicationID = appIdField ? appIdField.value : '';  
  if (!applicationID) {
    showAlert('Please select an application first.', 'error');
    return;
  }
  
  // Store app ID in the hidden field that subscribeAndCheckout looks for
  const modalAppField = document.getElementById(`modal-selected-app-${apiID}`);
  if (modalAppField) {
    modalAppField.value = applicationID;
  }
  
  if (isPaid) {
        // Paid plan - trigger Stripe checkout
    subscribeAndCheckout(orgID, apiID, apiReferenceID, policyID, policyName, applicationID);
  } else {
    // Free plan - direct subscription
    subscribe(orgID, applicationID, apiID, apiReferenceID, policyID, policyName);
  }
}

/**
 * =========================
 *  Export to window (important for inline onclick in HBS)
 * =========================
 */
window.subscribe = subscribe;
window.unsubscribe = unsubscribe;
window.updateSubscription = updateSubscription;
window.removeSubscription = removeSubscription;
window.openDeleteModal = openDeleteModal;
window.handleCreateSubscribe = handleCreateSubscribe;
window.showApplicationForm = showApplicationForm;
window.closeStripeCheckoutModal = closeStripeCheckoutModal;
window.handlePlanSubscription = handlePlanSubscription;
window.handleSubscribeWithPayment = handleSubscribeWithPayment;
window.subscribeAndCheckout = subscribeAndCheckout;
window.openModalWithApp = openModalWithApp;
window.handleListingSubscribe = handleListingSubscribe;
window.landingSubscribe = landingSubscribe;
window.handleLandingPlanSubscription = handleLandingPlanSubscription;
window.landingSubscribeAndCheckout = landingSubscribeAndCheckout;
