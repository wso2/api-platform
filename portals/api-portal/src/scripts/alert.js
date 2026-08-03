/**
 * Run an async `action` while showing a "processing" state on `button`: disable it and
 * swap its label for a Bootstrap spinner + `busyText`, then always restore the original
 * label and disabled state afterwards — even if `action` throws. Shared by the admin
 * settings forms (views, webhooks, labels, plans, key managers, …) so every save/delete
 * shows progress and can't be double-submitted. Follows the inline spinner convention
 * already used elsewhere in the portal. Returns the action's promise.
 */
function withButtonBusy(button, busyText, action) {
    if (!button) return Promise.resolve().then(action);
    const originalHtml = button.innerHTML;
    const wasDisabled = button.disabled;
    button.disabled = true;
    button.setAttribute('aria-busy', 'true');
    button.innerHTML =
        '<span class="spinner-border spinner-border-sm me-1" role="status" aria-hidden="true"></span>' +
        (busyText || 'Processing…');
    return Promise.resolve().then(action).finally(() => {
        button.innerHTML = originalHtml;
        button.disabled = wasDisabled;
        button.removeAttribute('aria-busy');
    });
}

/**
 * Keep a submit button disabled until a form is valid. `watchIds` lists the input /
 * textarea / select ids whose changes can affect validity; `validate` returns true when
 * the button should be enabled (typically: all required fields non-blank). The check runs
 * on every input/change of a watched field and once immediately. Returns a `sync()` you can
 * call after programmatically changing field values (opening or resetting a modal/form) so
 * the button state stays correct. Shared by the admin settings forms.
 */
function bindFormValidity(button, watchIds, validate) {
    if (!button) return function () {};
    var els = (watchIds || []).map(function (id) { return document.getElementById(id); })
        .filter(Boolean);
    function sync() { button.disabled = !validate(); }
    els.forEach(function (el) {
        el.addEventListener('input', sync);
        el.addEventListener('change', sync);
    });
    sync();
    return sync;
}

function showAlert(message, type) {
    return new Promise((resolve) => {
        const alertElement = document.getElementById('alertToast');
        if (!alertElement) {
            resolve();
            return;
        }
        const alertMessage = alertElement.querySelector('.alert-toast-message');
        const alertIcon = alertElement.querySelector('.alert-icon');

        if (alertMessage) {
            alertMessage.textContent = message;
        }

        alertElement.classList.remove('success', 'error', 'info');
        if (type) alertElement.classList.add(type);

        // Set appropriate icon based on alert type
        if (alertIcon) {
            alertIcon.className = 'alert-icon bi';
            if (type === 'success') {
                alertIcon.classList.add('bi-check-circle-fill');
            } else if (type === 'error') {
                alertIcon.classList.add('bi-exclamation-circle-fill');
            }
        }

        // Show the toast
        alertElement.classList.remove('alert-toast-hidden');
        alertElement.classList.add('alert-toast-visible');

        setTimeout(() => {
            alertElement.classList.add('alert-toast-fade-out');
            setTimeout(() => {
                alertElement.classList.remove('alert-toast-visible', 'alert-toast-fade-out');
                alertElement.classList.add('alert-toast-hidden');
                resolve();
            }, 500);
        }, 2300);
    });
}
