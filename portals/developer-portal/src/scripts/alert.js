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

        alertElement.classList.remove('success', 'error');
        alertElement.classList.add(type);

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
