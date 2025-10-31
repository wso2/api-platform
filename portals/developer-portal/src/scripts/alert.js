function showAlert(message, type) {
    return new Promise((resolve) => {
        const modalElement = document.getElementById('alertModal');
        const modalMessage = modalElement.querySelector('.modal-message');
        const modalBody = modalElement.querySelector('.modal-body');
        const alertIcon = modalElement.querySelector('.alert-icon');

        modalMessage.textContent = message;

        modalBody.classList.remove('success', 'error');
        modalBody.classList.add(type);
        
        // Set appropriate icon based on alert type
        alertIcon.className = 'alert-icon bi';
        if (type === 'success') {
            alertIcon.classList.add('bi-check-circle-fill');
        } else if (type === 'error') {
            alertIcon.classList.add('bi-exclamation-circle-fill');
        }

        const bootstrapModal = new bootstrap.Modal(modalElement, { backdrop: false });
        bootstrapModal.show();

        setTimeout(() => {
            modalElement.classList.add('fade-out');
            setTimeout(() => {
                bootstrapModal.hide();
                modalElement.classList.remove('fade-out');
                resolve(); 
            }, 500); 
        }, 2300);
    });
}
