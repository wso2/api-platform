document.addEventListener('DOMContentLoaded', checkQueryParamsAndLoadModal);

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
    document.querySelector("form").reset();
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
}

async function subscribe(orgID, applicationID, apiId, apiReferenceID, policyId, policyName) {
    console.log('Subscribing to API:', apiId);

    // Find the related card and button elements
    const card = getSubscriptionCard(apiId, policyId);
    const subscribeButton = card ? card.querySelector(".common-btn-primary") : null;
    const messageOverlay = card ? card.querySelector(".message-overlay") : null;
    let response;

    try {
        // Get application ID from hidden field if not provided
        if (!applicationID && card) {
            const hiddenField = card.querySelector('input[type="hidden"]');
            if (hiddenField && hiddenField.value) {
                applicationID = hiddenField.value;
            }
        }

        // Make the API request
        response = await fetch(`/devportal/organizations/${orgID}/subscriptions`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ applicationID, apiId, apiReferenceID, policyId, policyName }),
        });

        const responseData = await response.json();

        // Always reset button state and close modal
        const planButton = document.getElementById('subscribe-btn-' + policyId);
        if (planButton) {
            resetSubscribeButtonState(planButton);
        }
        closeModal('planModal-' + apiId);
        resetSubscribeButtonState(subscribeButton);

        if (response.ok) {
            // Show success notification
            showSubscriptionMessage(messageOverlay, 'Successfully subscribed', 'success');

            if (card) {
                // Show "Subscribed" label
                const subscriptionFlag = card.querySelector('.subscription-flag');
                if (subscriptionFlag) {
                    subscriptionFlag.style.display = 'block';
                }

                // Mark the application as subscribed
                const dropdown = card.querySelector('.custom-dropdown');
                if (dropdown) {
                    // Mark the application as subscribed in the dropdown
                    const appOption = dropdown.querySelector(`.select-item[data-value="${applicationID}"]`);
                    if (appOption) {
                        // Show the subscription icon by changing display style
                        let subscriptionIcon = appOption.querySelector('.subscription-icon');
                        if (subscriptionIcon) {
                            subscriptionIcon.style.display = 'inline-block';
                        } else {
                            const subscriptionIconHtml = `<img src="https://raw.githubusercontent.com/wso2/docs-bijira/refs/heads/main/en/devportal-theming/success-rounded.svg"
                            alt="Subscribed" class="subscription-icon" style="display: inline-block;" />`;
                            const tempDiv = document.createElement('div');
                            tempDiv.innerHTML = subscriptionIconHtml;
                            subscriptionIcon = tempDiv.firstElementChild;
                            appOption.appendChild(subscriptionIcon);
                        }

                        // Mark option as disabled
                        appOption.classList.add('disabled');
                    }

                    // Disable the subscribe button
                    const subscribeButton = card.querySelector('.common-btn-primary');
                    if (subscribeButton) {
                        subscribeButton.setAttribute('disabled', 'disabled');
                    }
                }
            }
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

        let errorMessage = ``
        if (response.headers.get("content-type") && response.headers.get("content-type").includes("application/json")) {
            errorMessage = `: ${error.message}`;
        }
        showSubscriptionMessage(messageOverlay, `Error while subscribing${errorMessage}`, 'error');
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
        const updatedButton = document.getElementById('subscribe-btn-' + apiId + "-" + policyName);
        if (updatedButton) {
            resetSubscribeButtonState(updatedButton);
        }
        // closeModal('planModal-' + apiId);
        resetSubscribeButtonState(subscribeButton);

        if (response.ok) {

            // Show success notification
            const updatedPlanCard = document.getElementById('api-card-' + apiId + "-" + policyName);
            updatedPlanCard.style.borderColor = 'var(--primary-main-color)';

            updatedButton.setAttribute('disabled', 'disabled');
            updatedButton.classList.add('disabled');
            updatedButton.style.setProperty('pointer-events', 'none', 'important');
            updatedButton.textContent = 'Update';

            const apiCards = document.querySelectorAll(`[id^="api-card-${apiId}-"]`);
            apiCards.forEach(card => {
                if (card.id !== `api-card-${apiId}-${policyName}`) {
                    card.style.borderColor = '';
                    const cardButton = card.querySelector('a[type="button"]');
                    cardButton.style.pointerEvents = 'auto';
                    cardButton.removeAttribute('disabled');
                    cardButton.classList.remove('disabled');
                }
            });
            document.getElementById('policy_' + subID).textContent = policyName;
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
    return document.getElementById('apiCard-' + apiId) ||
        document.getElementById('subscriptionCard-' + policyId) ||
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