/* eslint-disable no-unused-vars */
/* eslint-disable no-undef */
document.addEventListener('DOMContentLoaded', function () {

    const orgDefaultContent = document.getElementById('orgDefaultContent');
    const addOrg = document.getElementById('addOrg');
    const editOrg = document.getElementById('editOrg');

    const createOrgBtn = document.getElementById('createOrgBtn');
    const cancelAddBtn = document.getElementById('cancelAddBtn');
    const cancelEditBtn = document.getElementById('cancelEditBtn');

    // Set up the delete organization handler
    setDeleteConfirmationHandler('deleteOrg', function(data) {
        deleteOrg(data.orgID);
    });

    // Show form
    if (createOrgBtn && orgDefaultContent && addOrg) {
        createOrgBtn.addEventListener('click', function () {
            orgDefaultContent.style.display = 'none';
            addOrg.style.display = 'block';
        });
    }

    // Hide form (cancel)
    if (cancelAddBtn && orgDefaultContent && addOrg) {
        cancelAddBtn.addEventListener('click', function () {
            orgDefaultContent.style.display = 'block';
            addOrg.style.display = 'none';
        });
    }

    if (cancelEditBtn && orgDefaultContent && editOrg) {
        cancelEditBtn.addEventListener('click', function () {
            orgDefaultContent.style.display = 'block';
            editOrg.style.display = 'none';
        });
    }

    const editButtons = document.querySelectorAll('.edit-btn');
    editButtons.forEach(button => {
        button.addEventListener('click', function () {
            const details = this.closest('.organization').querySelector('.organization-details');
            if (details.style.display === 'block') {
                details.style.display = 'none';
                this.textContent = 'Edit';
            } else {
                details.style.display = 'block';
                this.textContent = 'Close';
            }
        });
    });
    const deleteForms = document.querySelectorAll('.delete-org');
    deleteForms.forEach(form => {
        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            let orgID = form.querySelector('#orgId').value;
            orgID = sanitizeInput(orgID);
            const response = await fetch(`/devportal/organizations/${orgID}`, {
                method: 'DELETE'
            });
            if (response.ok) {
                window.location.href = 'configure';
            } else {
                showAlert(`Field validation failed`, `error`);
            }
        });
    });
});

function showEditForm(id) {
    const orgDefaultContent = document.getElementById('orgDefaultContent');
    orgDefaultContent.style.display = 'none';
    document.getElementById('editForm-' + id).style.display = 'block';
}

function hideEditForm(id) {
    document.getElementById('editForm-' + id).style.display = 'none';
    const orgDefaultContent = document.getElementById('orgDefaultContent');
    orgDefaultContent.style.display = 'block';
}

function sanitizeInput(input) {
    const div = document.createElement('div');
    div.appendChild(document.createTextNode(input));
    return div.innerHTML;
}

async function createOrg() {

    const formData = new FormData(document.getElementById("createOrg"));
    const data = {};
    formData.forEach((value, key) => {
        data[key] = sanitizeInput(value);
    });
    const response = await fetch(`/devportal/organizations`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
    });
    if (response.ok) {
        window.location.href = 'configure';
    } else {
        showAlert(`Field validation failed`, `error`);
    }
}

async function editOrg(orgID, formID) {

    const formData = new FormData(document.getElementById(formID));
    const data = {};
    formData.forEach((value, key) => {
        data[key] = sanitizeInput(value);
    });
    const response = await fetch(`/devportal/organizations/${orgID}`, {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
    });
    if (response.ok) {
        window.location.href = 'configure';
    } else {
        showAlert(`Field validation failed`, `error`);
    }
}

function openOrgDeleteModal(orgID) {
    const modal = document.getElementById('deleteConfirmation');
    if (!modal) {
        console.error('openOrgDeleteModal: Modal not found');
        return;
    }

    const titleEl = modal.querySelector('.modal-title');
    const messageEl = modal.querySelector('.modal-message');
    if (titleEl) titleEl.textContent = 'Do you really want to delete this organization?';
    if (messageEl) messageEl.textContent = 'This will remove the organization stored in devportal';

    setDeleteConfirmationAction('deleteOrg', { orgID: orgID });

    const bootstrapModal = new bootstrap.Modal(modal);
    bootstrapModal.show();
}


async function deleteOrg(orgID) {
    const response = await fetch(`/devportal/organizations/${orgID}`, {
        method: 'DELETE'
    });
    if (response.ok) {
        window.location.href = 'configure';
    } else {
        showAlert(`Field validation failed`, `error`);
    }
}

// Export functions to global scope
window.openOrgDeleteModal = openOrgDeleteModal;
window.deleteOrg = deleteOrg;