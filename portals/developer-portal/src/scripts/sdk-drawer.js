/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
/* eslint-disable no-undef */

/**
 * Gets the application ID from the drawer's data attribute
 * @returns {string} - The application ID
 */
function getApplicationId() {
    const drawer = document.getElementById('sdkDrawer');
    const fromData = drawer ? drawer.dataset.applicationId : null;

    return fromData;
}


/**
 * Opens the SDK drawer and initializes its state
 * Validates that at least 1 API is selected before opening
 * Sets up initial drawer state and event handlers
 */
function openSdkDrawer() {
    const checkedCheckboxes = document.querySelectorAll('.api-checkbox:checked');
    if (checkedCheckboxes.length < 1) {
        alert('Please select at least 1 API to generate SDK');
        return;
    }

    window.sdkGenerationActive = false;
    
    storeOriginalChipHTML();
    
    const drawer = document.getElementById('sdkDrawer');
    if (drawer) {
        drawer.style.display = 'block';
        
        document.body.classList.add('drawer-open');
        
        resetDrawerState();
        
        initializeDrawerEventHandlers();
        
        setTimeout(() => {
            drawer.classList.add('open');
        }, 10);
    }
}

/**
 * Handles SDK drawer close requests
 * Shows confirmation modal if SDK generation is in progress
 * Otherwise proceeds with normal drawer closure
 */
function closeSdkDrawer() {
    if (isSDKGenerationInProgress()) {
        showSdkCancelConfirmModal();
        return; 
    }
    
    proceedWithDrawerClosure();
}

/**
 * Initializes all event handlers for the SDK drawer
 * Sets up language selection, generate button, textarea auto-resize,
 * form submission prevention, escape key, and backdrop click handlers
 */
function initializeDrawerEventHandlers() {
    
    setupLanguageHandlers('programmingLanguageAI');
    
    const generateBtn = document.querySelector('#aiGenerateBtn');
    if (generateBtn) {
        generateBtn.removeEventListener('click', handleGenerateClick); 
        generateBtn.addEventListener('click', handleGenerateClick);
    }
    
    const descriptionTextArea = document.getElementById('sdkDescription');
    if (descriptionTextArea) {
        descriptionTextArea.addEventListener('input', function() {
            autoResizeTextarea(this);
        });
        descriptionTextArea.addEventListener('keyup', function() {
            autoResizeTextarea(this);
        });
        autoResizeTextarea(descriptionTextArea);
    }

    const drawer = document.getElementById('sdkDrawer');
    if (drawer) {
        drawer.addEventListener('submit', function(e) {
            e.preventDefault();
            e.stopPropagation();
            return false;
        });
        
        document.addEventListener('keydown', handleEscapeKey);
        
        drawer.addEventListener('click', handleDrawerBackdropClick);
    }
}

/**
 * Sets up event listeners for suggestion chips
 * Handles click events for chip text and play button
 * Also sets up close button event handlers for removing chips
 */
function setupSuggestionListeners() {    
    const suggestionElements = document.querySelectorAll('.suggestion-chip .suggestion-text, .suggestion-chip .suggestion-play-button');
    
    suggestionElements.forEach((element, index) => {
        element.removeEventListener('click', handleSuggestionClick);
        element.addEventListener('click', handleSuggestionClick);
    });

    const closeButtons = document.querySelectorAll('.suggestion-chip .btn-close');
    
    closeButtons.forEach((button, index) => {
        button.removeEventListener('click', handleSuggestionClose);
        button.addEventListener('click', handleSuggestionClose);
    });
}

/**
 * Handles suggestion chip click events
 * Prevents multiple typing animations and retrieves prompt text
 * Initiates typing animation to fill textarea with suggestion content
 * @param {Event} event - The click event from the suggestion chip
 */
function handleSuggestionClick(event) {
    const chip = event.target.closest('.suggestion-chip');
    if (!chip) {
        console.error('Could not find suggestion chip parent');
        return;
    }
    
    if (window.currentTypingAnimation) {
        return;
    }
    
    const promptText = chip.dataset.prompt;

    if (!promptText) {
        console.error('No prompt text found in data-prompt attribute');
        return;
    }
    
    const descriptionTextarea = document.getElementById('sdkDescription');
    if (descriptionTextarea) {
        hideChipAndStartTyping(chip, descriptionTextarea, promptText);
    } else {
        console.error('Could not find sdkDescription textarea');
    }
}

/**
 * Handles suggestion chip close button clicks
 * Hides the chip and marks it as manually closed
 * @param {Event} event - The click event from the close button
 */
function handleSuggestionClose(event) {
    event.stopPropagation();

    const chip = event.target.closest('.suggestion-chip');
    if (chip) {
        chip.style.display = 'none';
        chip.classList.add('manually-closed');
    }
}

/**
 * Handles generate button click events
 * Prevents default form submission and initiates SDK generation process
 * @param {Event} e - The click event from the generate button
 */
function handleGenerateClick(e) {
    e.preventDefault();
    e.stopPropagation();
    console.log('Generate button clicked via event listener');
    generateSDKFromDrawerInternal();
    return false;
}

/**
 * Sets up event handlers for programming language radio buttons
 * Attaches change and click handlers to all radio buttons with specified name
 * @param {string} radioName - The name attribute of the radio button group
 */
function setupLanguageHandlers(radioName) {
    const radios = document.querySelectorAll(`input[name="${radioName}"]`);
    radios.forEach(radio => {
        radio.removeEventListener('change', handleLanguageChange);
        radio.removeEventListener('click', handleLanguageClick);
        
        radio.addEventListener('change', handleLanguageChange);
        radio.addEventListener('click', handleLanguageClick);
    });
}

/**
 * Handles programming language selection changes
 * Prevents default behavior and updates UI to reflect language selection
 * @param {Event} e - The change event from the radio button
 */
function handleLanguageChange(e) {
    e.preventDefault();
    e.stopPropagation();
    
    console.log(`Language changed to:`, e.target.value);
    updateLanguageSelection();

    return false;
}

function handleLanguageClick(e) {
    e.stopPropagation();
}

/**
 * Updates the visual selection state of language options
 * Removes 'selected' class from all options and adds it to the checked option
 * Synchronizes UI state with radio button selection
 */
function updateLanguageSelection() {
    document.querySelectorAll('.language-option').forEach(option => {
        option.classList.remove('selected');
    });
    
    const languageOptions = document.querySelectorAll('.language-option');

    languageOptions.forEach(option => {
        const radio = option.querySelector('input[type="radio"]');
        
        if (radio && radio.checked) {
            option.classList.add('selected');
        } else {
            option.classList.remove('selected');
        }
    });
}

/**
 * Executes the actual drawer closure process
 * Stops animations, closes connections, removes event listeners
 * Applies closing animation and resets drawer state
 */
function proceedWithDrawerClosure() {
    stopProgressAnimation();
    
    if (window.currentTypingAnimation) {
        cancelTypingAnimation();
    }
    
    if (window.currentSDKEventSource) {
        window.currentSDKEventSource.close();
        window.currentSDKEventSource = null;
    }
    
    document.removeEventListener('keydown', handleEscapeKey);
    
    const drawer = document.getElementById('sdkDrawer');
    if (drawer) {
        drawer.classList.remove('open');
        
        document.body.classList.remove('drawer-open');
        
        setTimeout(() => {
            drawer.style.display = 'none';
            
            resetDrawerState();
        }, 300);
    }
}

/**
 * Stores the original HTML content of suggestion chips
 * Used for restoring chips to their initial state when drawer is reopened
 * Only stores HTML if not already stored to preserve original state
 */
function storeOriginalChipHTML() {
    if (!window.originalChipHTML) {
        const chipContainer = document.querySelector('.prompt-suggestions-container');
        if (chipContainer) {
            window.originalChipHTML = chipContainer.innerHTML;
        }
    }
}

/**
 * Restores suggestion chips from originally stored HTML
 * Recreates all chips in their initial state and re-establishes event listeners
 * @returns {boolean} - True if restoration was successful, false otherwise
 */
function restoreOriginalChips() {
    if (window.originalChipHTML) {
        const chipContainer = document.querySelector('.prompt-suggestions-container');
        if (chipContainer) {
            chipContainer.innerHTML = window.originalChipHTML;
            setTimeout(() => {
                setupSuggestionListeners();
            }, 50);
            return true;
        }
    }
    return false;
}

/**
 * Resets the state of all existing suggestion chips
 */ 
function resetChipStates() {
    const allChips = document.querySelectorAll('.suggestion-chip');
    
    allChips.forEach((chip) => {
        chip.style.display = 'inline-flex';
        chip.classList.remove('typing-in-progress', 'manually-closed');
    });
    
    enableSuggestionChips();
}

/**
 * Restores hidden suggestion chips to their visible state
 * Cancels any ongoing typing animation and attempts restoration from original HTML
 * Falls back to resetting existing chip states if original HTML is unavailable
 */
function restoreHiddenSuggestionChips() {
    if (window.currentTypingAnimation) {
        cancelTypingAnimation();
    }
    
    const restoredFromOriginal = restoreOriginalChips();
    
    if (!restoredFromOriginal) {
        resetChipStates();
    }
    
    enableSuggestionChips();
}

/**
 * Resets the entire drawer to its initial state
 * Clears generation flags, typing animations, form values
 * Restores default language selection and suggestion chips
 * Re-establishes event listeners for proper functionality
 */
function resetDrawerState() {
    window.sdkGenerationActive = false;
    window.currentSDKJobId = null;
    
    if (window.currentTypingAnimation) {
        window.currentTypingAnimation.active = false;
        window.currentTypingAnimation = null;
    }
    
    const defaultLanguage = document.querySelector('input[name="programmingLanguageAI"][value="java"]');
    if (defaultLanguage) {
        defaultLanguage.checked = true;
    }
    
    const description = document.getElementById('sdkDescription');
    if (description) {
        description.value = '';
        description.classList.remove('typing');
        description.style.height = 'auto';
    }
    
    const descriptionSection = document.getElementById('aiDescriptionSection');
    if (descriptionSection) {
        descriptionSection.style.display = 'block';
    }

    const footer = document.getElementById('aiModeFooter');
    if (footer) footer.style.display = 'block';

    const languages = document.querySelectorAll('.language-option');
    languages.forEach(lang => lang.style.display = 'block');

    updateLanguageSelection();

    restoreHiddenSuggestionChips();
    
    showSuggestionChips();
    
    setTimeout(() => {
        setupSuggestionListeners();
    }, 100);
}

/**
 * Gets the currently selected programming language from radio buttons
 * @returns {string} - The value of the selected radio button, defaults to 'java'
 */
function getSelectedLanguage() {
    const selectedRadio = document.querySelector('input[name="programmingLanguageAI"]:checked');
    return selectedRadio ? selectedRadio.value : 'java'; // Default to java
}

/**
 * Initiates SDK generation process with validation and API calls
 * Validates API selection and description requirements
 * Constructs configuration object and sends generation request
 * Handles success/error responses and progress tracking
 * @param {string} language - Optional language override for SDK generation
 */
function generateSDKFromDrawerInternal(language) {
    console.log('generateSDKFromDrawerInternal called');
    
    const checkedCheckboxes = document.querySelectorAll('.api-checkbox:checked');
    const selectedLanguage = language || getSelectedLanguage();
    const descriptionElement = document.getElementById('sdkDescription');
    const description = descriptionElement ? descriptionElement.value : '';
    
    console.log('Validation checks:', {
        checkedCheckboxes: checkedCheckboxes.length,
        selectedLanguage: selectedLanguage,
        description: description
    });
    
    if (checkedCheckboxes.length < 1) {
        alert('Please select at least 1 API for SDK generation.');
        return;
    }
    
    if (!description || description.trim() === '') {
        showSdkDescriptionRequiredModal();
        return;
    }
    
    const selectedAPIs = Array.from(checkedCheckboxes).map(checkbox => ({
        id: checkbox.dataset.apiId,
        name: checkbox.dataset.apiName,
        version: checkbox.dataset.apiVersion
    }));
    
    const sdkConfiguration = {
        language: selectedLanguage,
        description: description.trim()
    };
    
    const pathParts = window.location.pathname.split('/');
    const orgName = pathParts[1];
    const applicationId = getApplicationId() || pathParts[5];
    
    console.log('SDK Generation Configuration:', {
        language: selectedLanguage,
        description: description.trim(),
        selectedAPIs: selectedAPIs.length
    });
    
    showSDKGenerationLoading();
    
    fetch(`/devportal/applications/${applicationId}/generate-sdk`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            selectedAPIs: selectedAPIs.map(api => api.id),
            sdkConfiguration: sdkConfiguration,
            orgName: orgName
        })
    })
    .then(response => response.json())
    .then(data => {
        if (data.success && data.data.jobId) {
            window.currentSDKJobId = data.data.jobId;
            startSDKProgressStream(data.data.sseEndpoint);
        } else {
            hideSDKGenerationLoading();
            showSDKGenerationError(data.message || 'SDK generation failed');
        }
    })
    .catch(error => {
        hideSDKGenerationLoading();
        console.error('Error generating SDK:', error);
        showSDKGenerationError('An error occurred while generating the SDK');
    });
}

/**
 * Establishes Server-Sent Events connection for real-time progress updates
 * Handles progress messages, completion, and error states
 * Manages EventSource lifecycle and cleanup
 * @param {string} jobId - The unique identifier for the SDK generation job
 * @param {string} sseEndpoint - The SSE endpoint URL for progress updates
 */
function startSDKProgressStream(sseEndpoint) {
    const eventSource = new EventSource(sseEndpoint);

    eventSource.onmessage = function(event) {
        try {
            const data = JSON.parse(event.data);
            console.log('SSE Progress Update:', data);

            if (data.type === 'progress') {
                stopProgressAnimation();
                
                updateProgressBar(data.progress);
                updateProgressStatus(data.currentStep, data.message);

                if (data.status === 'completed') {
                    eventSource.close();
                    handleSDKGenerationComplete(data);
                } else if (data.status === 'failed') {
                    eventSource.close();
                    hideSDKGenerationLoading();
                    showSDKGenerationError(data.message || data.error || 'SDK generation failed');
                }
            } else if (data.type === 'ping') {
                console.log('SSE Keep alive ping received');
            } else if (data.type === 'error') {
                eventSource.close();
                hideSDKGenerationLoading();
                showSDKGenerationError(data.message || 'SDK generation failed');
            }
        } catch (error) {
            console.error('Error parsing SDK progress event:', error);
        }
    };

    eventSource.onerror = function(error) {
        console.error('SSE error:', error);
        eventSource.close();
        
        hideSDKGenerationLoading();
        showSDKGenerationError('Connection lost. Please try again.');
    };
    
    // Store reference for cleanup
    window.currentSDKEventSource = eventSource;
}

/**
 * Updates the progress status display with current step and message
 * Modifies the progress title and status text elements
 * @param {string} currentStep - The current step name/title
 * @param {string} message - Detailed message about the current step
 */
function updateProgressStatus(currentStep, message) {
    const progressStatus = document.getElementById('sdkProgressStatus');
    const progressTitle = document.querySelector('.sdk-progress-title');
    
    if (progressTitle) {
        progressTitle.textContent = currentStep;
    }
    
    if (progressStatus) {
        progressStatus.textContent = message;
    }
}

/**
 * Handles successful completion of SDK generation
 * Updates progress to 100%, shows completion message
 * Triggers file download and initiates success display sequence
 * @param {Object} data - The completion data containing result information
 */
function handleSDKGenerationComplete(data) {
    updateProgressBar(100);
    updateProgressStatus('Completed!', 'SDK generation completed successfully');
    
    window.currentSDKJobId = null;
    
    console.log('SDK Generation Complete - Full data:', data);
    console.log('Result data:', data.resultData);
    
    if (data.resultData && data.resultData.finalDownloadUrl) {
        console.log('Download URL found:', data.resultData.finalDownloadUrl);
        setTimeout(() => {
            triggerAutoDownload(data.resultData.finalDownloadUrl);
        }, 1000);
    } else {
        console.warn('No download URL found in completion data');
        console.log('Available data keys:', Object.keys(data));
        if (data.resultData) {
            console.log('Available resultData keys:', Object.keys(data.resultData));
        }
    }
    
    showSDKGenerationSuccess();
}

/**
 * Hides the suggestion chips container
 * Used during SDK generation to focus user attention on progress
 */
function hideSuggestionChips() {
    const suggestionContainer = document.querySelector('.prompt-input-area');
    if (suggestionContainer) {
        suggestionContainer.style.display = 'none';
        console.log('Suggestion chips hidden');
    }
}

/**
 * Shows the suggestion chips container
 * Restores visibility of suggestion chips after generation completion
 */
function showSuggestionChips() {
    const suggestionContainer = document.querySelector('.prompt-input-area');
    if (suggestionContainer) {
        suggestionContainer.style.display = 'block';
        console.log('Suggestion chips shown');
    }
}

/**
 * Initiates the SDK generation loading state
 * Sets generation flag, hides suggestions, and shows progress bar
 */
function showSDKGenerationLoading() {
    console.log('SDK generation started...');
    
    window.sdkGenerationActive = true;
    
    hideSuggestionChips();
    
    showProgressBarInPrompt();
}

/**
 * Ends the SDK generation loading state
 * Clears generation flag, hides progress bar, and restores suggestions
 */
function hideSDKGenerationLoading() {
    console.log('SDK generation completed');
    
    window.sdkGenerationActive = false;
    
    hideProgressBarInPrompt();
    
    showSuggestionChips();
}

/**
 * Displays success state for completed SDK generation
 * Applies green success styling to progress bar
 * Schedules automatic drawer reset after 10 seconds
 */
function showSDKGenerationSuccess() {
    console.log('SDK generation completed successfully - showing success state');
    
    window.sdkGenerationActive = false;
    
    // Update progress bar to show success state
    const progressContainer = document.getElementById('sdkProgressContainer');
    if (progressContainer) {
        // Add success styling
        progressContainer.classList.add('success');
        
        // Update the progress bar to green success color
        const progressFill = document.getElementById('sdkProgressFill');
        if (progressFill) {
            progressFill.style.backgroundColor = 'var(--success-color)';
        }
        
        // Update title and status for success
        updateProgressStatus('Success!', 'SDK generation completed successfully');
    }
    
    // After 10 seconds, reset the drawer to initial state
    setTimeout(() => {
        resetDrawerToInitialState();
    }, 13000);
}

/**
 * Handles and displays SDK generation errors
 * Shows error notifications and progress error state
 * Schedules automatic UI restoration after error display
 * @param {string} message - The error message to display
 */
function showSDKGenerationError(message) {
    console.error('SDK Generation Error:', message);
    
    window.currentSDKJobId = null;
    
    showErrorNotification(message);
    
    showProgressError(message);
    
    setTimeout(() => {
        hideProgressBarInPrompt();
        showSuggestionChips();
    }, 3000);
}

/**
 * Creates and displays an in-place progress bar within the prompt area
 * Replaces textarea and generate button with progress visualization
 * Starts initial progress animation for user feedback
 */
function showProgressBarInPrompt() {
    const aiSection = document.getElementById('sdkDescriptionSection');
    const textarea = document.getElementById('sdkDescription');
    const generateButton = document.querySelector('#aiGenerateBtn');
    
    if (aiSection && textarea) {
        // Hide the textarea and generate button
        textarea.style.display = 'none';
        if (generateButton) {
            generateButton.style.display = 'none';
        }
        
        // Create progress bar container
        const progressContainer = document.createElement('div');
        progressContainer.id = 'sdkProgressContainer';
        progressContainer.className = 'sdk-progress-container';
        progressContainer.innerHTML = `
            <div class="sdk-progress-header">
                <span class="sdk-progress-title">Merging API Specifications</span>
                <span class="sdk-progress-percentage" id="sdkProgressPercentage">0%</span>
            </div>
            <div class="sdk-progress-bar-wrapper">
                <div class="sdk-progress-bar" id="sdkProgressBar">
                    <div class="sdk-progress-fill" id="sdkProgressFill"></div>
                </div>
            </div>
            <div class="sdk-progress-status" id="sdkProgressStatus">
                In progress: Merging API Specifications...
            </div>
        `;
        
        // Insert progress container after the label
        const label = aiSection.querySelector('label[for="sdkDescription"]');
        if (label) {
            label.insertAdjacentElement('afterend', progressContainer);
        } else {
            aiSection.insertBefore(progressContainer, textarea);
        }
        
        // Start progress animation
        startProgressAnimation();
    }
}

/**
 * Removes the in-place progress bar and restores original UI elements
 * Shows textarea and generate button after progress completion or cancellation
 */
function hideProgressBarInPrompt() {
    stopProgressAnimation();
    
    const progressContainer = document.getElementById('sdkProgressContainer');
    const textarea = document.getElementById('sdkDescription');
    const generateButton = document.querySelector('#aiGenerateBtn');
    
    if (progressContainer) {
        progressContainer.remove();
    }
    
    if (textarea) {
        textarea.style.display = 'block';
    }
    
    if (generateButton) {
        generateButton.style.display = 'flex';
    }
}

/**
 * Updates the visual progress bar with a specified percentage
 * Prevents backwards progress updates and provides smooth transitions
 * Updates both the progress fill width and percentage display
 * @param {number} progress - Progress percentage (0-100)
 */
function updateProgressBar(progress) {
    const boundedProgress = Math.max(0, Math.min(100, progress));
    
    const progressFill = document.getElementById('sdkProgressFill');
    const progressPercentage = document.getElementById('sdkProgressPercentage');
    
    if (progressFill) {
        const currentWidth = parseFloat(progressFill.style.width) || 0;
        if (boundedProgress > currentWidth || boundedProgress === 0 || boundedProgress === 100) {
            progressFill.style.width = `${boundedProgress}%`;
            console.log(`Progress bar updated to: ${boundedProgress}%`);
        } else {
            console.log(`Skipping backwards progress update: ${boundedProgress}% (current: ${currentWidth}%)`);
        }
    }
    
    if (progressPercentage) {
        progressPercentage.textContent = `${Math.round(boundedProgress)}%`;
    }
}

/**
 * Displays error state in the progress bar
 * Applies error styling and updates title and status with error information
 * @param {string} message - The error message to display
 */
function showProgressError(message) {
    const progressContainer = document.getElementById('sdkProgressContainer');
    const progressStatus = document.getElementById('sdkProgressStatus');
    const progressTitle = document.querySelector('.sdk-progress-title');
    
    if (progressContainer) {
        progressContainer.classList.add('error');
    }
    
    if (progressTitle) {
        progressTitle.textContent = 'Generation Failed';
    }
    
    if (progressStatus) {
        progressStatus.textContent = message;
        progressStatus.style.color = '#dc3545';
    }
}

/**
 * Starts animated progress bar simulation for initial user feedback
 * Provides visual progress indication before real server updates arrive
 * Stops at 20% to wait for actual progress data
 */
function startProgressAnimation() {
    stopProgressAnimation();
    
    let progress = 0;
    const interval = setInterval(() => {
        progress += Math.random() * 8 + 2; // Random increment between 2-10
        
        if (progress > 20) {
            progress = 20; // Stop at 20% until actual completion
            clearInterval(interval);
        }
        
        updateProgressBar(progress);
    }, 1000);
    
    const progressContainer = document.getElementById('sdkProgressContainer');
    if (progressContainer) {
        progressContainer.dataset.intervalId = interval;
    }
    
    window.currentProgressInterval = interval;
}

/**
 * Stops any running progress animation intervals
 * Cleans up animation timers to prevent memory leaks
 */
function stopProgressAnimation() {
    if (window.currentProgressInterval) {
        clearInterval(window.currentProgressInterval);
        window.currentProgressInterval = null;
    }
    
    const progressContainer = document.getElementById('sdkProgressContainer');
    if (progressContainer && progressContainer.dataset.intervalId) {
        clearInterval(parseInt(progressContainer.dataset.intervalId));
        delete progressContainer.dataset.intervalId;
    }
}

/**
 * Triggers automatic file download for the generated SDK
 * Creates temporary download link and programmatically clicks it
 * @param {string} downloadUrl - The URL of the file to download
 */
function triggerAutoDownload(downloadUrl) {
    console.log('Triggering download for URL:', downloadUrl);
    
    if (!downloadUrl) {
        console.error('Download URL is empty or undefined');
        showErrorNotification('Download URL is not available');
        return;
    }
    
    try {
        const link = document.createElement('a');
        link.href = downloadUrl;
        link.download = ''; 
        link.style.display = 'none';
        
        // Add error handling for download
        link.onerror = function(error) {
            console.error('Download link error:', error);
            showErrorNotification('Failed to download SDK file');
        };
        
        document.body.appendChild(link);
        
        console.log('Clicking download link for:', downloadUrl);
        link.click();
        
        // Clean up
        setTimeout(() => {
            document.body.removeChild(link);
        }, 1000);
        
        console.log('Auto-download triggered successfully for:', downloadUrl);
    } catch (error) {
        console.error('Error triggering download:', error);
        showErrorNotification('Failed to initiate download');
    }
}

// Error Notification System
function showErrorNotification(message, title = 'SDK Generation Error') {
    // Remove any existing error notifications
    const existingNotifications = document.querySelectorAll('.sdk-error-notification');
    existingNotifications.forEach(notification => notification.remove());
    
    // Create notification container
    const notification = document.createElement('div');
    notification.className = 'sdk-error-notification alert alert-danger';
    
    notification.innerHTML = `
        <div class="d-flex align-items-start">
            <div class="flex-shrink-0 me-3">
                <i class="bi bi-exclamation-triangle-fill text-danger"></i>
            </div>
            <div class="flex-grow-1">
                <div class="fw-bold mb-1">${title}</div>
                <div class="error-message">${message}</div>
            </div>
            <button type="button" class="btn-close ms-2" onclick="closeErrorNotification(this)" aria-label="Close"></button>
        </div>
    `;
    
    // Add to page
    document.body.appendChild(notification);
    
    // Auto-hide after 8 seconds
    setTimeout(() => {
        closeErrorNotification(notification);
    }, 8000);
}

// function showSuccessNotification(message, title = 'Success') {
//     const existingNotifications = document.querySelectorAll('.sdk-success-notification');
//     existingNotifications.forEach(notification => notification.remove());
    
//     const notification = document.createElement('div');
//     notification.className = 'sdk-success-notification alert alert-success';
    
//     notification.innerHTML = `
//         <div class="d-flex align-items-start">
//             <div class="flex-shrink-0 me-3">
//                 <i class="bi bi-check-circle-fill text-success"></i>
//             </div>
//             <div class="flex-grow-1">
//                 <div class="fw-bold mb-1">${title}</div>
//                 <div class="success-message">${message}</div>
//             </div>
//             <button type="button" class="btn-close ms-2" onclick="closeSuccessNotification(this)" aria-label="Close"></button>
//         </div>
//     `;
    
//     document.body.appendChild(notification);
    
//     setTimeout(() => {
//         closeSuccessNotification(notification);
//     }, 5000);
// }

/**
 * Displays warning notifications as dismissible alert banners
 * Creates styled notification with warning icon, title, message, and close button
 * Auto-hides after 6 seconds with moderate urgency indication
 * @param {string} message - The warning message content
 * @param {string} title - The warning notification title (default: 'Warning')
 */
function showWarningNotification(message, title = 'Warning') {
    const existingNotifications = document.querySelectorAll('.sdk-warning-notification');
    existingNotifications.forEach(notification => notification.remove());
    
    const notification = document.createElement('div');
    notification.className = 'sdk-warning-notification alert alert-warning';
    
    notification.innerHTML = `
        <div class="d-flex align-items-start">
            <div class="flex-shrink-0 me-3">
                <i class="bi bi-exclamation-triangle-fill text-warning"></i>
            </div>
            <div class="flex-grow-1">
                <div class="fw-bold mb-1">${title}</div>
                <div class="warning-message">${message}</div>
            </div>
            <button type="button" class="btn-close ms-2" onclick="closeWarningNotification(this)" aria-label="Close"></button>
        </div>
    `;
    
    document.body.appendChild(notification);
    
    setTimeout(() => {
        closeWarningNotification(notification);
    }, 6000);
}

/**
 * Closes error notifications with slide-out animation
 * Handles both direct element and event target scenarios
 * @param {Element} element - The notification element or close button
 */
function closeErrorNotification(element) {
    const notification = element.closest ? element.closest('.sdk-error-notification') : element;
    if (notification) {
        notification.style.animation = 'slideOutToRight 0.3s ease-in';
        setTimeout(() => {
            if (notification.parentNode) {
                notification.parentNode.removeChild(notification);
            }
        }, 300);
    }
}

function closeSuccessNotification(element) {
    const notification = element.closest ? element.closest('.sdk-success-notification') : element;
    if (notification) {
        notification.style.animation = 'slideOutToRight 0.3s ease-in';
        setTimeout(() => {
            if (notification.parentNode) {
                notification.parentNode.removeChild(notification);
            }
        }, 300);
    }
}

/**
 * Closes warning notifications with slide-out animation
 * Handles both direct element and event target scenarios
 * @param {Element} element - The notification element or close button
 */
function closeWarningNotification(element) {
    const notification = element.closest ? element.closest('.sdk-warning-notification') : element;
    if (notification) {
        notification.style.animation = 'slideOutToRight 0.3s ease-in';
        setTimeout(() => {
            if (notification.parentNode) {
                notification.parentNode.removeChild(notification);
            }
        }, 300);
    }
}

/**
 * Checks if SDK generation is currently in progress
 * Evaluates multiple indicators: generation flag, progress UI, SSE connection
 * @returns {boolean} - True if generation is active, false otherwise
 */
function isSDKGenerationInProgress() {
    // Check global flag
    if (window.sdkGenerationActive === true) {
        return true;
    }
    
    const progressContainer = document.getElementById('sdkProgressContainer');
    
    const hasActiveSSE = window.currentSDKEventSource && window.currentSDKEventSource.readyState === EventSource.OPEN;
    
    const hasProgressAnimation = window.currentProgressInterval !== null && window.currentProgressInterval !== undefined;
    
    return !!(progressContainer || hasActiveSSE || hasProgressAnimation);
}

/**
 * Cancels ongoing SDK generation process
 * Stops all related activities: SSE connection, progress animation, UI state
 * Sends cancellation request to server and shows user notification
 */
function cancelSDKGeneration() {
    console.log('Cancelling SDK generation...');
    
    window.sdkGenerationActive = false;
    
    if (window.currentSDKEventSource) {
        window.currentSDKEventSource.close();
        window.currentSDKEventSource = null;
        console.log('SSE connection closed');
    }
    
    stopProgressAnimation();
    
    hideSDKGenerationLoading();
    
    if (window.currentSDKJobId) {
        const orgName = window.location.pathname.split('/')[1];
        const viewName = window.location.pathname.split('/')[3];
        const applicationId = window.location.pathname.split('/')[5];
        const cancelUrl = `/devportal/applications/${applicationId}/sdk/cancel/${window.currentSDKJobId}`;
        
        fetch(cancelUrl, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                console.log('SDK job cancelled on server:', data.message);
            } else {
                console.error('Failed to cancel SDK job on server:', data.error);
            }
        })
        .catch(error => {
            console.error('Error sending cancellation request:', error);
        });
        
        window.currentSDKJobId = null;
    }
    
    showWarningNotification('SDK generation has been cancelled.', 'Generation Cancelled');
    
    console.log('SDK generation cancelled successfully');
}

/**
 * Shows the SDK cancellation confirmation modal
 * Displays modal with backdrop and establishes keyboard/click event handlers
 */
function showSdkCancelConfirmModal() {
    const modal = document.getElementById('sdkCancelConfirmModal');
    if (modal) {
        modal.style.display = 'flex';
        document.body.classList.add('modal-open');
        
        document.addEventListener('keydown', handleModalEscapeKey);
        
        modal.addEventListener('click', handleModalBackdropClick);
    }
}

/**
 * Closes the SDK cancellation confirmation modal
 * Hides modal, removes event listeners, and restores document scroll behavior
 */
function closeSdkCancelConfirmModal() {
    const modal = document.getElementById('sdkCancelConfirmModal');
    if (modal) {
        modal.style.display = 'none';
        document.body.classList.remove('modal-open');
        document.removeEventListener('keydown', handleModalEscapeKey);
        modal.removeEventListener('click', handleModalBackdropClick);
    }
}

/**
 * Confirms SDK cancellation and proceeds with cancellation process
 * Closes confirmation modal, cancels generation, and closes drawer
 */
function confirmSdkCancellation() {
    closeSdkCancelConfirmModal();
    
    cancelSDKGeneration();
    
    proceedWithDrawerClosure();
}

/**
 * Handles Escape key press for modal closure
 * Only responds when the cancellation modal is currently visible
 * @param {KeyboardEvent} event - The keyboard event
 */
function handleModalEscapeKey(event) {
    if (event.key === 'Escape') {
        const modal = document.getElementById('sdkCancelConfirmModal');
        if (modal && modal.style.display === 'flex') {
            event.preventDefault();
            closeSdkCancelConfirmModal();
        }
    }
}

/**
 * Handles backdrop clicks for modal closure
 * Only closes modal when clicking on the backdrop, not modal content
 * @param {MouseEvent} event - The click event
 */
function handleModalBackdropClick(event) {
    // Only close if clicking on the modal backdrop (not on modal content)
    if (event.target.id === 'sdkCancelConfirmModal') {
        event.preventDefault();
        closeSdkCancelConfirmModal();
    }
}

/**
 * Automatically resizes textarea height based on content
 * Adjusts height to fit text content without scrollbars
 * @param {HTMLTextAreaElement} textArea - The textarea element to resize
 */
function autoResizeTextarea(textArea) {
    if (textArea) {
        textArea.style.height = 'auto';
        textArea.style.height = textArea.scrollHeight + 'px';
    }
}

/**
 * Handles Escape key press for drawer closure
 * Only responds when the SDK drawer is currently open
 * @param {KeyboardEvent} event - The keyboard event
 */
function handleEscapeKey(event) {
    if (event.key === 'Escape') {
        const drawer = document.getElementById('sdkDrawer');
        if (drawer && drawer.classList.contains('open')) {
            event.preventDefault();
            closeSdkDrawer();
        }
    }
}

/**
 * Handles backdrop clicks for drawer closure
 * Only closes drawer when clicking on the backdrop, not drawer content
 * @param {MouseEvent} event - The click event
 */
function handleDrawerBackdropClick(event) {
    if (event.target.id === 'sdkDrawer') {
        event.preventDefault();
        closeSdkDrawer();
    }
}

/**
 * Initiates the chip hiding and typing animation sequence
 * Disables other chips, hides the clicked chip, and starts typing animation
 * @param {Element} chip - The suggestion chip element that was clicked
 * @param {HTMLTextAreaElement} textarea - The target textarea for typing
 * @param {string} text - The text content to type into the textarea
 */
function hideChipAndStartTyping(chip, textarea, text) {
    disableSuggestionChips();
    chip.style.display = 'none';
    
    if (!window.currentTypingAnimation) {
        window.currentTypingAnimation = {};
    }
    window.currentTypingAnimation.hiddenChip = chip;
    
    startTypingAnimation(textarea, text);
}

/**
 * Disables all suggestion chips during typing animation
 * Adds visual indication that chips are temporarily unavailable
 */
function disableSuggestionChips() {
    const allChips = document.querySelectorAll('.suggestion-chip');
    allChips.forEach(chip => {
        chip.classList.add('typing-in-progress');
    });
}

/**
 * Re-enables all suggestion chips after typing completion
 * Removes visual indication and restores chip interactivity
 */
function enableSuggestionChips() {
    const allChips = document.querySelectorAll('.suggestion-chip');
    allChips.forEach(chip => {
        chip.classList.remove('typing-in-progress');
    });
}

/**
 * Starts character-by-character typing animation into textarea
 * Preserves existing hidden chip reference and manages animation state
 * Provides realistic typing speed and visual feedback
 * @param {HTMLTextAreaElement} textarea - The target textarea element
 * @param {string} text - The text content to type character by character
 */
function startTypingAnimation(textarea, text) {
    let existingHiddenChip = null;
    if (window.currentTypingAnimation && window.currentTypingAnimation.hiddenChip) {
        existingHiddenChip = window.currentTypingAnimation.hiddenChip;
    }
    
    if (window.currentTypingAnimation) {
        window.currentTypingAnimation.active = false;
        if (window.currentTypingAnimation.textarea) {
            window.currentTypingAnimation.textarea.classList.remove('typing');
        }
    }
    
    textarea.value = '';
    textarea.focus();
    
    textarea.classList.add('typing');
    
    let currentIndex = 0;
    const typingSpeed = 30; // milliseconds per character
    
    // Store animation state for cancellation
    window.currentTypingAnimation = {
        active: true,
        textarea: textarea,
        targetText: text,
        hiddenChip: existingHiddenChip
    };
    
    function typeNextCharacter() {
        // Check if animation was cancelled
        if (!window.currentTypingAnimation || !window.currentTypingAnimation.active) {
            return;
        }
        
        if (currentIndex < text.length) {

            textarea.value += text.charAt(currentIndex);
            currentIndex++;
            
            autoResizeTextarea(textarea);
            
            textarea.dispatchEvent(new Event('input', { bubbles: true }));
            
            setTimeout(typeNextCharacter, typingSpeed);
        } else {
            finishTypingAnimation(textarea, text);
        }
    }
    
    setTimeout(typeNextCharacter, 100);
}

/**
 * Cancels ongoing typing animation and restores UI state
 * Restores hidden chip visibility and re-enables all suggestion chips
 */
function cancelTypingAnimation() {
    if (window.currentTypingAnimation) {
        window.currentTypingAnimation.active = false;
        
        // Remove typing class
        if (window.currentTypingAnimation.textarea) {
            window.currentTypingAnimation.textarea.classList.remove('typing');
        }
        
        // Restore the hidden chip if it exists
        if (window.currentTypingAnimation.hiddenChip) {
            const chip = window.currentTypingAnimation.hiddenChip;
            chip.style.display = 'inline-flex'; // Show the chip again
        }
        
        window.currentTypingAnimation = null;
        
        // Re-enable suggestion chips
        enableSuggestionChips();
        
        console.log('Typing animation cancelled');
    }
}

function finishTypingAnimation(textarea, text) {
    // Remove typing class
    textarea.classList.remove('typing');
    
    // Clear animation state
    window.currentTypingAnimation = null;
    
    // Re-enable suggestion chips
    enableSuggestionChips();
    
    // Trigger final change event
    textarea.dispatchEvent(new Event('change', { bubbles: true }));
    
    console.log('Typing animation completed:', text);
}

function resetDrawerToInitialState() {
    console.log('Resetting drawer to initial state after successful SDK generation');
    
    hideProgressBarInPrompt();
    
    window.sdkGenerationActive = false;
    window.currentSDKJobId = null;
    
    if (window.currentTypingAnimation) {
        window.currentTypingAnimation.active = false;
        window.currentTypingAnimation = null;
    }
    
    const description = document.getElementById('sdkDescription');
    if (description) {
        description.value = '';
        description.classList.remove('typing');
        description.style.height = 'auto';
    }
    
    const defaultLanguage = document.querySelector('input[name="programmingLanguageAI"][value="java"]');
    if (defaultLanguage) {
        defaultLanguage.checked = true;
    }
    updateLanguageSelection();
    
    restoreHiddenSuggestionChips();
    
    showSuggestionChips();
    
    setTimeout(() => {
        setupSuggestionListeners();
    }, 100);
    
    console.log('Drawer reset to initial state completed');
}

/**
 * Shows the description required modal
 * Displays modal when user tries to generate SDK without providing description
 */
function showSdkDescriptionRequiredModal() {
    const modal = document.getElementById('sdkDescriptionRequiredModal');
    if (modal) {
        modal.style.display = 'flex';
        document.body.classList.add('modal-open');

        document.addEventListener('keydown', handleDescriptionModalEscapeKey);
        modal.addEventListener('click', handleDescriptionModalBackdropClick);
    }
}

/**
 * Closes the description required modal
 * Hides modal, removes event listeners, and restores document scroll behavior
 */
function closeSdkDescriptionRequiredModal() {
    const modal = document.getElementById('sdkDescriptionRequiredModal');
    if (modal) {
        modal.style.display = 'none';
        document.body.classList.remove('modal-open');
        document.removeEventListener('keydown', handleDescriptionModalEscapeKey);
        modal.removeEventListener('click', handleDescriptionModalBackdropClick);
    }
}

/**
 * Handles Escape key press for description modal closure
 * @param {KeyboardEvent} event - The keyboard event
 */
function handleDescriptionModalEscapeKey(event) {
    if (event.key === 'Escape') {
        const modal = document.getElementById('sdkDescriptionRequiredModal');
        if (modal && modal.style.display === 'flex') {
            event.preventDefault();
            closeSdkDescriptionRequiredModal();
        }
    }
}

/**
 * Handles backdrop clicks for description modal closure
 * @param {MouseEvent} event - The click event
 */
function handleDescriptionModalBackdropClick(event) {
    if (event.target.id === 'sdkDescriptionRequiredModal') {
        event.preventDefault();
        closeSdkDescriptionRequiredModal();
    }
}

// Make functions available globally
window.openSdkDrawer = openSdkDrawer;
window.closeSdkDrawer = closeSdkDrawer;

// Make modal functions globally available
window.showSdkCancelConfirmModal = showSdkCancelConfirmModal;
window.closeSdkCancelConfirmModal = closeSdkCancelConfirmModal;
window.confirmSdkCancellation = confirmSdkCancellation;
window.handleModalEscapeKey = handleModalEscapeKey;
window.handleModalBackdropClick = handleModalBackdropClick;

// Make notification functions globally available
window.showErrorNotification = showErrorNotification;
// window.showSuccessNotification = showSuccessNotification;
window.showWarningNotification = showWarningNotification;
window.closeErrorNotification = closeErrorNotification;
window.closeSuccessNotification = closeSuccessNotification;
window.closeWarningNotification = closeWarningNotification;
window.showSdkDescriptionRequiredModal = showSdkDescriptionRequiredModal;
window.closeSdkDescriptionRequiredModal = closeSdkDescriptionRequiredModal;
window.handleDescriptionModalEscapeKey = handleDescriptionModalEscapeKey;
window.handleDescriptionModalBackdropClick = handleDescriptionModalBackdropClick;

