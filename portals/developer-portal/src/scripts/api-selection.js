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
 * API Selection Management for SDK Generation
 * Handles checkbox selection states and SDK button visibility
 */

/**
 * Toggles all API checkboxes based on the select all checkbox state
 * @param {HTMLElement} selectAllCheckbox - The select all checkbox element
 */
function toggleAllAPISelection(selectAllCheckbox) {
    const checkboxes = document.querySelectorAll('.api-checkbox');
    checkboxes.forEach(checkbox => {
        checkbox.checked = selectAllCheckbox.checked;
    });
    updateSDKButtonVisibility();
}

/**
 * Handles individual API checkbox selection changes
 * Updates the select all checkbox state and SDK button visibility
 */
function toggleAPISelection() {
    const checkboxes = document.querySelectorAll('.api-checkbox');
    const selectAllCheckbox = document.getElementById('selectAllAPIs');
    
    // Update "Select All" checkbox state
    const checkedCount = document.querySelectorAll('.api-checkbox:checked').length;
    if (selectAllCheckbox) {
        selectAllCheckbox.checked = checkedCount === checkboxes.length;
        selectAllCheckbox.indeterminate = checkedCount > 0 && checkedCount < checkboxes.length;
    }
    
    updateSDKButtonVisibility();
}

/**
 * Updates the visibility and state of the SDK generation button
 * Enables the button when at least 1 API is selected
 */
function updateSDKButtonVisibility() {
    const checkedCount = document.querySelectorAll('.api-checkbox:checked').length;
    const sdkButton = document.querySelector('button[onclick="redirectToSDKGeneration()"]');
    
    // Only update if the button exists (i.e., if SDK generation feature is enabled)
    if (sdkButton) {
        if (checkedCount >= 1) {
            sdkButton.disabled = false;
            sdkButton.classList.remove('disabled');
        } else {
            sdkButton.disabled = true;
            sdkButton.classList.add('disabled');
        }
    }
}

/**
 * Redirects to SDK generation process
 * Validates API selection and opens the SDK drawer
 */
function redirectToSDKGeneration() {
    const checkedCheckboxes = document.querySelectorAll('.api-checkbox:checked');
    if (checkedCheckboxes.length < 1) {
        alert('Please select at least 1 API to generate SDK');
        return;
    }
    
    // Call the global openSdkDrawer function from the SDK drawer
    if (typeof window.openSdkDrawer === 'function') {
        window.openSdkDrawer();
    } else {
        console.error('openSdkDrawer function not found. Make sure sdk-drawer.js is loaded.');
    }
}

/**
 * Initialize API selection functionality when the page loads
 * Sets up event listeners and initial button states only if SDK feature is enabled
 */
function initializeAPISelection() {
    // Only initialize if API checkboxes exist (feature is enabled)
    const apiCheckboxes = document.querySelectorAll('.api-checkbox');
    if (apiCheckboxes.length === 0) {
        console.log('SDK Generation feature is disabled - no API checkboxes found');
        return;
    }
    
    updateSDKButtonVisibility();
    
    // Set up event listeners for API checkboxes
    apiCheckboxes.forEach(checkbox => {
        checkbox.addEventListener('change', toggleAPISelection);
    });
    
    const selectAllCheckbox = document.getElementById('selectAllAPIs');
    if (selectAllCheckbox) {
        selectAllCheckbox.addEventListener('change', function() {
            toggleAllAPISelection(this);
        });
    }
    
    console.log('SDK Generation feature is enabled - API selection initialized');
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', initializeAPISelection);

// Make functions globally available for inline onclick handlers
window.toggleAllAPISelection = toggleAllAPISelection;
window.toggleAPISelection = toggleAPISelection;
window.updateSDKButtonVisibility = updateSDKButtonVisibility;
window.redirectToSDKGeneration = redirectToSDKGeneration;
