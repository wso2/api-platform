document.addEventListener('DOMContentLoaded', () => {
    const MAX_CHARACTERS = 256;

    // In-place editing functionality
    const setupInlineEditing = () => {
        const applicationName = document.getElementById('applicationName');
        const applicationDescription = document.getElementById('applicationDescription');
        const editNameBtn = document.getElementById('editNameBtn');
        const editDescriptionBtn = document.getElementById('editDescriptionBtn');
        const saveNameBtn = document.getElementById('saveNameBtn');
        const saveDescriptionBtn = document.getElementById('saveDescriptionBtn');
        const cancelNameBtn = document.getElementById('cancelNameBtn');
        const cancelDescriptionBtn = document.getElementById('cancelDescriptionBtn');
        const nameEditActions = document.getElementById('nameEditActions');
        const descriptionEditActions = document.getElementById('descriptionEditActions');
        const nameEditError = document.getElementById('nameEditError');
        const descriptionEditError = document.getElementById('descriptionEditError');
        const inlineEditForm = document.getElementById('inlineEditForm');
        const addClientID = document.getElementById('addClientID');
        const clientIDInput = document.getElementById('clientIDInput');
        const cancelClientIDBtn = document.getElementById('cancelClientIDBtn');
        const sharedKeyTooltip = document.getElementById('sharedKeyTooltip');

        if (!applicationName || !applicationDescription) return;

        const applicationId = inlineEditForm.dataset.applicationId;

        // Function to set loading state
        const setButtonLoadingState = (button, isLoading) => {
            const normalState = button.querySelector('.button-normal-state');
            const loadingState = button.querySelector('.button-loading-state');

            if (isLoading) {
                button.disabled = true;
                if (normalState) normalState.style.display = 'none';
                if (loadingState) loadingState.style.display = 'inline-block';
            } else {
                button.disabled = false;
                if (normalState) normalState.style.display = 'inline-block';
                if (loadingState) loadingState.style.display = 'none';
            }
        };

        // Make name editable
        editNameBtn.addEventListener('click', () => {
            applicationName.contentEditable = true;
            applicationName.focus();
            editNameBtn.style.display = 'none';
            nameEditActions.style.display = 'inline-flex';
            // Clear any existing error
            nameEditError.style.display = 'none';
        });

        // Make description editable
        editDescriptionBtn.addEventListener('click', () => {
            applicationDescription.contentEditable = true;
            applicationDescription.focus();
            editDescriptionBtn.style.display = 'none';
            descriptionEditActions.style.display = 'inline-flex';
            // Clear any existing error
            descriptionEditError.style.display = 'none';
        });

        // Cancel name edit
        cancelNameBtn.addEventListener('click', () => {
            applicationName.textContent = applicationName.dataset.original;
            applicationName.contentEditable = false;
            nameEditActions.style.display = 'none';
            editNameBtn.style.display = 'inline-block';
            nameEditError.style.display = 'none';
        });

        // Cancel description edit
        cancelDescriptionBtn.addEventListener('click', () => {
            applicationDescription.textContent = applicationDescription.dataset.original;
            applicationDescription.contentEditable = false;
            descriptionEditActions.style.display = 'none';
            editDescriptionBtn.style.display = 'inline-block';
            descriptionEditError.style.display = 'none';
        });

        // Save name changes
        saveNameBtn.addEventListener('click', async () => {
            const newName = applicationName.textContent.trim();
            if (!newName) {
                nameEditError.textContent = 'Name cannot be empty';
                nameEditError.style.display = 'block';
                return;
            }

            // Enable loading state
            setButtonLoadingState(saveNameBtn, true);
            cancelNameBtn.disabled = true;

            const result = await saveApplicationChanges(applicationId, {
                name: newName,
                description: applicationDescription.dataset.original
            }, nameEditError);

            // Reset loading state
            setButtonLoadingState(saveNameBtn, false);
            cancelNameBtn.disabled = false;

            if (result) {
                applicationName.dataset.original = newName;
                applicationName.contentEditable = false;
                nameEditActions.style.display = 'none';
                editNameBtn.style.display = 'inline-block';
                nameEditError.style.display = 'none';
            }
        });

        // Save description changes
        saveDescriptionBtn.addEventListener('click', async () => {
            const newDescription = applicationDescription.textContent.trim();
            if (newDescription.length > MAX_CHARACTERS) {
                descriptionEditError.textContent = `Description exceeds ${MAX_CHARACTERS} character limit`;
                descriptionEditError.style.display = 'block';
                return;
            }

            // Enable loading state
            setButtonLoadingState(saveDescriptionBtn, true);
            cancelDescriptionBtn.disabled = true;

            const result = await saveApplicationChanges(applicationId, {
                name: applicationName.dataset.original,
                description: newDescription
            }, descriptionEditError);

            // Reset loading state
            setButtonLoadingState(saveDescriptionBtn, false);
            cancelDescriptionBtn.disabled = false;

            if (result) {
                document.location.reload();
                // applicationDescription.dataset.original = newDescription;
                // applicationDescription.contentEditable = false;
                // descriptionEditActions.style.display = 'none';
                // editDescriptionBtn.style.display = 'inline-block';
                // descriptionEditError.style.display = 'none';
            }
        });

        // Add client ID
        addClientID.addEventListener('click', () => {
            clientIDInput.contentEditable = true;
            clientIDInput.focus();
            addClientID.style.display = 'none';
            clientIDEditActions.style.display = 'inline-flex';
            sharedKeyTooltip.style.display = 'none';
        });


        // Cancel description edit
        cancelClientIDBtn.addEventListener('click', () => {
            clientIDInput.textContent = applicationDescription.dataset.original;
            clientIDInput.contentEditable = false;
            clientIDEditActions.style.display = 'none';
            addClientID.style.display = 'inline-block';
            sharedKeyTooltip.style.display = 'inline-block';
        });

    };

    // Function to save application changes
    const saveApplicationChanges = async (applicationId, data, errorElement) => {
        try {
            if (errorElement) {
                errorElement.style.display = 'none';
            }

            const response = await fetch(`/devportal/applications/${applicationId}`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    ...data,
                    type: 'WEB'
                }),
            });

            if (!response.ok) {
                throw new Error(`HTTP error! Status: ${response.status}`);
            }

            return true;
        } catch (error) {
            console.error('Error saving application:', error);
            if (errorElement) {
                errorElement.textContent = 'Failed to update. Please try again.';
                errorElement.style.display = 'block';
            }
            return false;
        }
    };

    // Initialize inline editing
    setupInlineEditing();
});
