/* eslint-disable no-unused-vars */
/* eslint-disable no-undef */
document.addEventListener('DOMContentLoaded', function () {

    const orgDefaultContent = document.getElementById('orgDefaultContent');
    const addOrg = document.getElementById('addOrg');
    const editOrg = document.getElementById('editOrg');

    const createOrgBtn = document.getElementById('createOrgBtn');
    const cancelAddBtn = document.getElementById('cancelAddBtn');
    const cancelEditBtn = document.getElementById('cancelEditBtn');

    // Show form
    createOrgBtn.addEventListener('click', function () {
        orgDefaultContent.style.display = 'none';
        addOrg.style.display = 'block';
    });

    // Hide form (cancel)
    cancelAddBtn.addEventListener('click', function () {
        orgDefaultContent.style.display = 'block';
        addOrg.style.display = 'none';
    });

    cancelEditBtn.addEventListener('click', function () {
        orgDefaultContent.style.display = 'block';
        editOrg.style.display = 'none';
    });

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
    modal.dataset.orgID = orgID;
    const bootstrapModal = new bootstrap.Modal(modal);
    bootstrapModal.show();
}


async function deleteOrg() {

    const modal = document.getElementById('deleteConfirmation');
    const orgID = modal.dataset.orgID;
    const response = await fetch(`/devportal/organizations/${orgID}`, {
        method: 'DELETE'
    });
    if (response.ok) {
        window.location.href = 'configure';
    } else {
        showAlert(`Field validation failed`, `error`);
    }
}