/* eslint-disable no-undef */
/* eslint-disable no-unused-vars */
document.addEventListener('DOMContentLoaded', function () {
    
    const createViewBtn = document.getElementById('createViewBtn');
    const viewDetails = document.getElementById('viewDetails');
    const createView = document.getElementById('createView');
    const cancelViewBtn = document.getElementById('cancelViewBtn');

    createViewBtn.addEventListener('click', function () {
        viewDetails.style.display = 'none';
        createView.style.display = 'block';
    });

    cancelViewBtn.addEventListener('click', function () {
        viewDetails.style.display = 'block';
        createView.style.display = 'none';
    });

    //upload new organization content
    const uploadArea = document.getElementById('uploadArea');
    const zipFile = document.getElementById('file');
    const fileName = document.getElementById('fileName');

    if (uploadArea) {
        uploadArea.addEventListener('click', () => zipFile.click());
        uploadArea.addEventListener('dragover', (e) => {
            e.preventDefault();
            uploadArea.style.borderColor = '#007bff';
        });
        uploadArea.addEventListener('dragleave', () => {
            uploadArea.style.borderColor = '#ccc';
        });
        uploadArea.addEventListener('drop', (e) => {
            e.preventDefault();
            uploadArea.style.borderColor = '#ccc';
            if (e.dataTransfer.files.length) {
                zipFile.files = e.dataTransfer.files;
                updateFileName();
            }
        });
        zipFile.addEventListener('change', updateFileName);
        
        function updateFileName() {
            fileName.textContent = zipFile.files[0] ? `Selected file: ${zipFile.files[0].name}` : '';
        }
    }

    //update IDP
    const editIDPButton = document.getElementById('idpEdit');
    if (editIDPButton) {
        editIDPButton.addEventListener('click', function () {
            const details = this.closest('.organization').querySelector('.organization-details');
            if (details.style.display === 'block') {
                details.style.display = 'none';
                this.textContent = 'Edit';
            } else {
                details.style.display = 'block';
                this.textContent = 'Close';
            }
        });
    }

    const editOrgPButton = document.getElementById('orgEdit');
    if (editOrgPButton) {
        editOrgPButton.addEventListener('click', function () {
            const details = this.closest('.organization').querySelector('.editOrg');
            if (details.style.display === 'block') {
                details.style.display = 'none';
                this.textContent = 'Edit';
            } else {
                details.style.display = 'block';
                this.textContent = 'Close';
            }
        });
    }

    const removeButtons = document.querySelectorAll('.span-item .remove-btn');
    
    // Add event listener to each remove button
    removeButtons.forEach(button => {
        button.addEventListener('click', function() {
           // Remove the parent span item
            const spanItem = button.closest('.span-item');
            if (spanItem) {
                spanItem.remove();
            }
        });
    });

    //add labels
    const textInput = document.getElementById('textInput');
        const inputContainer = document.getElementById('inputContainer');

        textInput.addEventListener('keypress', function(event) {
            if (event.key === 'Enter') {
                event.preventDefault();
                const text = this.value.trim();
                if (text) {
                    addSpan(text);
                    this.value = '';
                }
            }
        });

        function addSpan(text) {
            const span = document.createElement('span');
            span.className = 'span-tag';
            span.innerHTML = `${text}<span class="remove">&times;</span>`;
            
            span.querySelector('.remove').addEventListener('click', function() {
                inputContainer.removeChild(span);
            });

            inputContainer.insertBefore(span, textInput);
        }

        // Ensure the input is always visible
        inputContainer.addEventListener('click', function() {
            textInput.focus();
        });

        //remove labels from div
        const removeLabels = document.querySelectorAll('.span-tag .remove');
    
        // Add event listener to each remove button
        removeLabels.forEach(label => {
            label.addEventListener('click', function() {
                // Remove the parent span item
                const spanItem = label.closest('.span-tag');
                if (spanItem) {
                    spanItem.remove();
                }
            });
        });
});

function sanitizeInput(input) {
    const div = document.createElement('div');
    div.appendChild(document.createTextNode(input));
    return div.innerHTML;
}

async function createIDP(orgID) {

    const formData = new FormData(document.getElementById("createIDP"));
    const data = {};
    formData.forEach((value, key) => {
        data[key] = sanitizeInput(value);
    });
    data['scope'] = 'openid';
    const response = await fetch(`/devportal/organizations/${orgID}/identityProvider`, {
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

    // Clear the form
    document.getElementById('providerId').value = '';
    document.getElementById('providerUrl').value = '';
}

async function editIDP(orgID, formID) {

    const editIDP = document.getElementById(formID);
    const formData = new FormData(editIDP);
    const data = {};
    formData.forEach((value, key) => {
        data[key] = sanitizeInput(value);
    });
    data['scope'] = 'openid';
    const response = await fetch(`/devportal/organizations/${orgID}/identityProvider`, {
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

async function deleteIDP(orgID) {

    const response = await fetch(`/devportal/organizations/${orgID}/identityProvider`, {
        method: 'DELETE',
    });
    if (response.ok) {
        window.location.href = 'configure';
    } else {
        showAlert(`Field validation failed`, `error`);
    }
}

async function updateOrgContent(orgID) {

    const zipFile = document.getElementById('editZipFile');
    if (!zipFile.files[0]) {
        alert('Please select a ZIP file to upload.');
        return;
    }
    const formData = new FormData();
    formData.append('file', zipFile.files[0]);
    const response = await fetch(`/devportal/organizations/${orgID}/layout`, {
        method: 'PUT',
        body: formData,
        credentials: 'same-origin' // Include cookies if needed
    });
    if (response.ok) {
        const result = await response.json();
        alert(`Upload successful! Organization ID: ${result.orgId}, File Name: ${result.fileName}`);
        window.location.href = 'configure';
    } else {
        const error = await response.text();
        alert(`Upload failed: ${error}`);
    }
}

async function uploadContent(orgID) {

    const zipFile = document.getElementById('file');
    const formData = new FormData();
    formData.append('file', zipFile.files[0]);

    const view = document.getElementById('uploadViewContent').value;
    const response = await fetch(`/devportal/organizations/${orgID}/views/${view}/layout`, {
        method: 'PUT',
        body: formData,
        credentials: 'same-origin'
    });
    if (response.ok) {
        const result = await response.json();
        alert(`Upload successful! Organization ID: ${result.orgId}, File Name: ${result.fileName}`);
        window.location.href = 'configure';
    } else {
        const error = await response.text();
        alert(`Upload failed: ${error}`);
    }
}

function addViewLabel(labelSelectID, labelsContainerID) {

    const select = document.getElementById(labelSelectID);
    const labelsContainer = document.getElementById(labelsContainerID);
    const selectedValue = select.value;
    
    //Check if label already exists
    if ([...labelsContainer.children].some(span => span.textContent.includes(selectedValue))) {
        return;
    }
    const span = document.createElement("span");
    span.className = "span-tag";
    span.textContent = selectedValue;
    
    const removeBtn = document.createElement("span");
    removeBtn.className = "remove";
    removeBtn.textContent = "×";
    removeBtn.onclick = function () {
        labelsContainer.removeChild(span);
    };
    span.appendChild(removeBtn);
    labelsContainer.appendChild(span);
}

async function editView(existingLabels, labelsContainerID, displayNameID, nameID, orgID) {

    const labelsContainer = document.getElementById(labelsContainerID);
    const displayName = document.getElementById(displayNameID).value;
    const name = document.getElementById(nameID).value;
    const selected = [...labelsContainer.children].map(span => span.textContent.replace("×", "").trim());
    const addedLabels = selected.filter(label => !existingLabels.includes(label));
    let removedLabels = [];
    if (existingLabels.length > 0) {
        const existinglabelsList = JSON.parse(existingLabels);
         removedLabels = existinglabelsList.filter(label => !selected.includes(label));
    }
    const sanitizAddedLabels = addedLabels.map(label => sanitizeInput(label));
    const sanitizeRemovedLabels = removedLabels.map(label => sanitizeInput(label));
    
    const data = {
        displayName: displayName,
        addedLabels: sanitizAddedLabels,
        removedLabels: sanitizeRemovedLabels
    }
    const response = await fetch(`/devportal/organizations/${orgID}/views/${name}`, {
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

async function deleteView(orgID, viewName) {
    
    const response = await fetch(`/devportal/organizations/${orgID}/views/${viewName}`, {
        method: 'DELETE',
    });
    if (response.ok) {
        window.location.href = 'configure';
    } else {
        showAlert(`Field validation failed`, `error`);
    }
}

async function addLabels(orgID, orgLabels) {

    const labelsContainer = document.getElementById("inputContainer");
    const selected = [...labelsContainer.getElementsByClassName('span-tag')].map(span => span.textContent.replace('×', "").trim());
    const existingLabels = orgLabels.map(label => label.name);
    const addedLabels = selected.filter(label => !existingLabels.includes(label));
    const removedLabels = existingLabels.filter(label => !selected.includes(label));
    const sanitizeAdd = addedLabels.map(label => sanitizeInput(label));

    if (removedLabels.length > 0) {
        const sanitizeDelete = removedLabels.map(label => sanitizeInput(label));
        const labelName = sanitizeDelete.join(",");
        const response = await fetch(`/devportal/organizations/${orgID}/labels?names=${labelName}`, {
            method: "DELETE",
            headers: {
                'Content-Type': 'application/json',
            }
        });
        if (response.ok) {
           window.location.href = 'configure';
         } else {
             showAlert(`Field validation failed`, `error`);
         } 

    }
    const labels = []
    sanitizeAdd.forEach(async label => {    
        labels.push({
            "name": label,
            "displayName": label
        });
    });
 
    const response = await fetch(`/devportal/organizations/${orgID}/labels`, {
        method: "PUT",
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(labels),
    });
    if (response.ok) {
       window.location.href = 'configure';
     } else {
         showAlert(`Field validation failed`, `error`);
     }
}

