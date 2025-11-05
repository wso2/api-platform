function openWarningModal(param1, param2, param3, param4, param5) {
    const modal = document.getElementById('warningModal');
    modal.dataset.param1 = param1;
    modal.dataset.param2 = param2;
    modal.dataset.param3 = param3;
    modal.dataset.param4 = param4;
    modal.dataset.param5 = param5;
    const bootstrapModal = new bootstrap.Modal(modal);
    bootstrapModal.show();

    if (param1 === 'regenerate') {
        document.getElementById('modalTitle').innerText = "Regenerate API Key";
        document.getElementById('modalMessage').innerText = "Regenerating the API key will impact applications using the current key. \nThe current key will expire in 1 hour. To avoid service disruptions, copy the new key and update it in your applications immediately.";
        const regenerateBtn = document.getElementById('modalFunction');
        regenerateBtn.innerText = "Regenerate";
        regenerateBtn.setAttribute('onclick', `regenerateAPIKey(${param2}, '${param3}')`);
    } else if (param1 === 'revoke') {
        document.getElementById('modalTitle').innerText = "Revoke API Key";
        document.getElementById('modalMessage').innerText = "Revoking the API key will impact applications using the current key. Are you sure you want to proceed? This action cannot be undone.";
        const revokeBtn = document.getElementById('modalFunction');
        revokeBtn.innerText = "Revoke";
        revokeBtn.setAttribute('onclick', `revokeAPIKey(${param2}, '${param3}', '${param4}', '${param5}')`);
    } else if (param1 === 'Unsubscribe') {
        document.getElementById('modalTitle').innerText = "Do you really want to remove the subscription?";
        document.getElementById('modalMessage').innerText = "This will remove the subscription entry stored in the devportal.";
        const unsubscribeBtn = document.getElementById('modalFunction');
        unsubscribeBtn.innerText = "Confirm";
        unsubscribeBtn.setAttribute('onclick', `removeSubscription('${param2}', '${param3}', '${param4}', '${param5}')`);
    }
}

async function deleteApplication() {
    const modal = document.getElementById('warningModal');
    const applicationId = modal.dataset.param1;
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
