/**
 * Delete Confirmation Modal Handler
 *
 * Provides a safe way to handle delete confirmation modals without inline onclick handlers.
 *
 * Usage:
 *   1. Set up a handler: setDeleteConfirmationHandler('deleteOrg', deleteOrgFunction)
 *   2. When opening modal: setDeleteConfirmationAction('deleteOrg', { orgID: '123' })
 *   3. The confirm button will call the registered handler with the data
 */

const deleteConfirmationHandlers = {};
let currentModalAction = null;
let currentModalData = null;

/**
 * Register a handler function for a specific action
 * @param {string} actionId - Unique identifier for this action
 * @param {Function} handler - Function to call when confirmed, receives the data object as parameter
 */
function setDeleteConfirmationHandler(actionId, handler) {
    if (typeof handler !== 'function') {
        console.error(`setDeleteConfirmationHandler: handler for "${actionId}" must be a function`);
        return;
    }
    deleteConfirmationHandlers[actionId] = handler;
}

/**
 * Set the action to perform when the delete confirmation modal is confirmed
 * @param {string} actionId - The registered handler ID to use
 * @param {Object} data - Data to pass to the handler
 */
function setDeleteConfirmationAction(actionId, data = {}) {
    if (!deleteConfirmationHandlers[actionId]) {
        console.error(`setDeleteConfirmationAction: No handler registered for action "${actionId}"`);
        return;
    }
    currentModalAction = actionId;
    currentModalData = data;
}

/**
 * Clear the current modal action (called when modal is dismissed)
 */
function clearDeleteConfirmationAction() {
    currentModalAction = null;
    currentModalData = null;
}

/**
 * Initialize the delete confirmation modal button listeners
 * This should be called once on page load
 */
function initializeDeleteConfirmationModal() {
    const modal = document.getElementById('deleteConfirmation');
    const confirmBtn = document.getElementById('deleteConfirmationBtn');

    if (!modal || !confirmBtn) {
        console.error('initializeDeleteConfirmationModal: Required modal elements not found');
        return;
    }

    // Handle confirm button click
    confirmBtn.addEventListener('click', function(e) {
        if (!currentModalAction) {
            console.error('deleteConfirmationBtn clicked but no action is set');
            return;
        }

        const handler = deleteConfirmationHandlers[currentModalAction];
        if (typeof handler === 'function') {
            try {
                handler(currentModalData);
            } catch (error) {
                console.error(`Error executing delete confirmation handler for "${currentModalAction}":`, error);
            }
        }
    });

    // Clear action when modal is dismissed
    modal.addEventListener('hidden.bs.modal', function() {
        clearDeleteConfirmationAction();
    });
}

// Initialize when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initializeDeleteConfirmationModal);
} else {
    // DOM already loaded
    initializeDeleteConfirmationModal();
}

// Export for use in other scripts
window.setDeleteConfirmationHandler = setDeleteConfirmationHandler;
window.setDeleteConfirmationAction = setDeleteConfirmationAction;
