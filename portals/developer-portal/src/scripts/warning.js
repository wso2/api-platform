function openWarningModal(param1, param2, param3, param4, param5, param6, param7) {
    const modal = document.getElementById('warningModal');
    if (!modal) {
        console.error('openWarningModal: Warning modal not found');
        return;
    }

    // Sanitize parameters to prevent XSS
    const sanitizeParam = (param) => {
        if (param === null || param === undefined) return '';
        return String(param).replace(/['"<>]/g, '');
    };

    const sanitizedParam1 = sanitizeParam(param1);
    const sanitizedParam2 = sanitizeParam(param2);
    const sanitizedParam3 = sanitizeParam(param3);
    const sanitizedParam4 = sanitizeParam(param4);
    const sanitizedParam5 = sanitizeParam(param5);
    const sanitizedParam6 = sanitizeParam(param6);
    const sanitizedParam7 = sanitizeParam(param7);


    modal.dataset.param1 = sanitizedParam1;
    modal.dataset.param2 = sanitizedParam2;
    modal.dataset.param3 = sanitizedParam3;
    modal.dataset.param4 = sanitizedParam4;
    modal.dataset.param5 = sanitizedParam5;
    modal.dataset.param6 = sanitizedParam6;
    modal.dataset.param7 = sanitizedParam7;

    try {
        const bootstrapModal = new bootstrap.Modal(modal);
        bootstrapModal.show();
    } catch (error) {
        console.error('openWarningModal: Failed to show modal', error);
        return;
    }

    const modalTitle = document.getElementById('modalTitle');
    const modalMessage = document.getElementById('modalMessage');
    const modalFunction = document.getElementById('modalFunction');

    if (!modalTitle || !modalMessage || !modalFunction) {
        console.error('openWarningModal: Required modal elements not found');
        return;
    }

    if (param1 === 'regenerate') {
        modalTitle.innerText = "Regenerate API Key";
        modalMessage.innerText = "Your current key will be revoked immediately. Update your applications with the new key to avoid disruptions.";
        modalFunction.innerText = "Regenerate";
        // Use data attributes and event listeners instead of inline onclick to prevent XSS
        modalFunction.onclick = function() {
            if (typeof regenerateAPIKey === 'function') {
                regenerateAPIKey(sanitizedParam2, sanitizedParam3, sanitizedParam6);
            }
        };
    } else if (param1 === 'revoke') {
        modalTitle.innerText = "Revoke API Key";
        modalMessage.innerText = "Revoking the API key will impact applications using the current key. Are you sure you want to proceed? This action cannot be undone.";
        modalFunction.innerText = "Revoke";
        modalFunction.onclick = function() {
            if (typeof revokeAPIKey === 'function') {
                revokeAPIKey(sanitizedParam2, sanitizedParam3, sanitizedParam4, sanitizedParam5, sanitizedParam6);
            }
        };
    } else if (param1 === 'Unsubscribe') {
        modalTitle.innerText = "Do you really want to remove the subscription?";
        modalMessage.innerText = "This will remove the subscription entry stored in the devportal.";
        modalFunction.innerText = "Confirm";
        modalFunction.onclick = function() {
            if (typeof removeSubscription === 'function') {
                removeSubscription(sanitizedParam2, sanitizedParam3, sanitizedParam4, sanitizedParam5);
            }
        };
    } else if (param1 === 'DeletePlatformSubscription') {
        modalTitle.innerText = 'Delete subscription?';
        modalMessage.innerText =
            'The subscription token will be immediately invalidated and any API calls using it will fail.';
        modalFunction.innerText = 'Delete';
        modalFunction.onclick = function() {
            if (typeof executeDeletePlatformSubscription === 'function') {
                executeDeletePlatformSubscription(sanitizedParam2, sanitizedParam3);
            }
        };
    } else if (param1 === 'DeleteTokenBasedSubscription') {
        modalTitle.innerText = 'Delete subscription?';
        modalMessage.innerText =
            'The subscription token will be immediately invalidated and any API calls using it will fail.';
        modalFunction.innerText = 'Delete';
        modalFunction.onclick = function() {
            if (typeof executeDeleteSubscription === 'function') {
                executeDeleteSubscription();
            }
        };
    } else if (param1 === 'DeleteAppBasedSubscription') {
        modalTitle.innerText = 'Unsubscribe from this API?';
        modalMessage.innerText =
            'This will remove the subscription entry stored in the devportal. Are you sure you want to unsubscribe from this API?';
        modalFunction.innerText = 'Confirm';
        modalFunction.onclick = function() {
            if (typeof executeDeleteSubscription === 'function') {
                executeDeleteSubscription();
            }
        };
    } else if (param1 === 'deleteApplication') {
        modalTitle.innerText = 'Delete Application';
        modalMessage.innerText = 'Are you sure you want to delete this application? This action cannot be undone and will revoke all associated access tokens and subscriptions.';
        modalFunction.innerText = 'Delete';
        modalFunction.onclick = function() {
            deleteApplication();
        };
    } else if (param1 === 'SwitchPlatformSubscriptionPlan') {
        modalTitle.innerText = 'Switch subscription plan?';
        modalMessage.innerText =
            'You are currently subscribed to the "' +
            sanitizedParam7 +
            '" plan. Switching to "' +
            sanitizedParam5 +
            '" will delete your existing subscription and generate a new token. Do you want to proceed?';
        modalFunction.innerText = 'Confirm';
        modalFunction.onclick = function() {
            if (typeof runPendingPlatformPlanSwitch === 'function') {
                runPendingPlatformPlanSwitch(
                    sanitizedParam2,
                    sanitizedParam3,
                    sanitizedParam4,
                    sanitizedParam5,
                    sanitizedParam6
                );
            }
        };
    }
}

async function deleteApplication() {
    const modal = document.getElementById('warningModal');
    const applicationId = modal.dataset.param2;
    const messageOverlay = document.getElementById(`message-overlay-${applicationId}`);
    const trashButton = document.getElementById(`trash-btn-${applicationId}`);

    try {
        // Show loading state
        if (trashButton) {
            trashButton.innerHTML =
                '<span style="font-size: 0.875rem;"><span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span>&nbsp;Deleting...</span>';
            trashButton.disabled = true;
        }
        const response = await fetch(`/devportal/applications/${applicationId}`, {
            method: 'DELETE',
        });
        if (response.ok) {
            // remove the application card from the UI
            const cardWrapper = document.getElementById(`app-card-${applicationId}`);
            if (cardWrapper) {
                const colElement = cardWrapper.closest('.col-12');
                if (colElement) {
                    colElement.remove();
                } else {
                    cardWrapper.remove();
                }
            }
            const remainingCards = document.querySelectorAll('[id^="app-card-"]');
            if (remainingCards.length === 0) {
                const createButton = document.getElementById('createButton');
                if (createButton) {
                    createButton.classList.add('d-none');
                }
                const plusCardWrapper = document.getElementById('createApplicationCardWrapper');
                if (plusCardWrapper) {
                    plusCardWrapper.classList.remove('d-none');
                }
                const plusCard = document.getElementById('applicationCreateCard');
                  if (plusCard) {
                      plusCard.classList.remove('d-none');
                }
            }
        } else {
            if (messageOverlay && typeof window.showAppDeleteMessage === 'function') {
                window.showAppDeleteMessage(messageOverlay, 'Failed to delete application. Please try again.', 'error');
            }
            trashButton.disabled = false;
            trashButton.innerHTML = '<i class="bi bi-trash"></i>';
        }
    } catch (error) {
        if (messageOverlay && typeof window.showAppDeleteMessage === 'function') {
            window.showAppDeleteMessage(
                messageOverlay,
                'An error occurred while deleting the application. Please try again.',
                'error',
            );
        }
        trashButton.disabled = false;
        trashButton.innerHTML = '<i class="bi bi-trash"></i>';
    }
}

window.showAppDeleteMessage = function (overlay, message, type = 'success') {
    if (overlay) {
        // Clear any existing auto-hide timers
        if (overlay.hideTimer) {
            clearTimeout(overlay.hideTimer);
            overlay.hideTimer = null;
        }

        // Set message - keeping it simple and concise
        const messageText = overlay.querySelector('.message-text');
        if (messageText) messageText.textContent = message;

        // Set type (success/error)
        overlay.classList.remove('success', 'error');
        overlay.classList.add(type);

        // Update icon - ensure proper class structure for alignment
        const icon = overlay.querySelector('.message-icon');
        if (icon) {
            icon.className = 'bi message-icon ' + type;
            icon.classList.add(type === 'success' ? 'bi-check-circle-fill' : 'bi-exclamation-circle-fill');
        }

        // Show the overlay (remove hidden class if it exists)
        overlay.classList.remove('hidden');

        return overlay;
    }
    return null;
};
