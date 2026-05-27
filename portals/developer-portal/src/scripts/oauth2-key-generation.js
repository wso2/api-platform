async function generateApplicationKey(formId, appId, keyType, keyManager, clientName, subscriptions, orgID, consumerKeyID, consumerSecretID) {
    // Validate required parameters
    if (!formId || !appId || !keyType || !keyManager || !clientName || !subscriptions || !orgID || !consumerKeyID || !consumerSecretID) {
        console.error('generateApplicationKey: Missing required parameters');
        return;
    }

    // Get the generate button and set loading state
    const generateBtn = document.getElementById('generateKeyBtn-' + keyType);
    if (!generateBtn) {
        console.error('generateApplicationKey: Generate button not found');
        return;
    }

    const normalState = generateBtn.querySelector('.button-normal-state');
    const loadingState = generateBtn.querySelector('.button-loading-state');

    if (!normalState || !loadingState) {
        console.error('generateApplicationKey: Button states not found');
        return;
    }

    // Clear any previous error messages
    const errorContainer = document.getElementById('keyGenerationErrorContainer-' + keyType);
    if (errorContainer) {
        errorContainer.style.display = 'none';
        errorContainer.textContent = '';
    }

    // Show generating state
    normalState.style.display = 'none';
    loadingState.style.display = 'inline-block';
    generateBtn.disabled = true;

    const form = document.getElementById(formId);
    if (!form) {
        console.error('generateApplicationKey: Form not found', formId);
        // Reset button state
        normalState.style.display = 'inline-block';
        loadingState.style.display = 'none';
        generateBtn.disabled = false;
        if (errorContainer) {
            errorContainer.textContent = 'Please refresh the page and try again.';
            errorContainer.style.display = 'block';
        }
        return;
    }

    const apiList = [];
    let subList;
    try {
        subList = JSON.parse(subscriptions);
    } catch (error) {
        console.error('generateApplicationKey: Failed to parse subscriptions', error);
        normalState.style.display = 'inline-block';
        loadingState.style.display = 'none';
        generateBtn.disabled = false;
        if (errorContainer) {
            errorContainer.textContent = 'Invalid subscriptions data. Please refresh the page and try again.';
            errorContainer.style.display = 'block';
        }
        return;
    }
    subList.forEach(subscription => {
        apiList.push({
            "apiName": subscription.name,
            "apiRefId": subscription.refID,
            "policyID": subscription.policyID
        });
    });
    const formData = new FormData(form);
    const jsonObject = getFormData(formData, keyManager, clientName);
    let grantTypes;
    if (!jsonObject.grantTypes) {
        grantTypes = ["client_credentials"];
    } else {
        grantTypes = jsonObject.grantTypes;
    }
    const payload = JSON.stringify({
        "applicationName": clientName,
        "apis": apiList,
        "tokenType": "OAUTH",
        "tokenDetails": {
            "grantTypesToBeSupported": grantTypes,
            "keyType": keyType,
            "keyManager": keyManager,
            "callbackUrl": jsonObject.callbackURL,
            "scopes": [
                "default"
            ],
            "validityTime": 3600,
            "additionalProperties": jsonObject.additionalProperties,
        },
        "clientID": document.getElementById("clientIDInput-" + keyType)?.value?.trim(),
    })
    try {
        const response = await fetch(`/devportal/organizations/${orgID}/app-key-mapping`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: payload,
        });


        const responseData = await response.json();
        if (response.ok) {
            // close modal
            const modal = document.getElementById('OauthKeyModal');
            //await showAlert('Application keys generated successfully!', 'success');
            // reload the page to reflect the changes
            // TODO: update the function to handle the response and update the UI accordingly
            //window.location.reload();

            const consumerKey = responseData.consumerKey;
            const consumerSecret = responseData.consumerSecret;
            const keyManagerId = formId.replace("keysview-", "").replace(/-(SANDBOX|PRODUCTION)$/, "");
            const envSuffix = keyType;
            
            // Update form fields safely
            const consumerKeyElement = document.getElementById(consumerKeyID);
            const consumerSecretElement = document.getElementById(consumerSecretID);
            const consumerKeyElementView = document.getElementById(consumerKeyID + "-view");
            const consumerSecretElementView = document.getElementById(consumerSecretID + "-view");
            const appRefElement = document.getElementById("app-ref-" + keyManagerId + "-" + envSuffix);
            const keyMapElement = document.getElementById("key-map-" + keyManagerId + "-" + envSuffix);

            if (consumerKeyElement) consumerKeyElement.value = consumerKey || '';
            if (consumerSecretElement) consumerSecretElement.value = consumerSecret || '';

            if (consumerKeyElementView) consumerKeyElementView.value = consumerKey || '';
            if (consumerSecretElementView) consumerSecretElementView.value = consumerSecret || '';

            if (appRefElement && responseData.appRefId) appRefElement.value = responseData.appRefId;
            if (keyMapElement && responseData.keyMappingId) keyMapElement.value = responseData.keyMappingId;

            if (consumerSecret) {
                const modalBody = document.getElementById("keysViewModal-" + keyType + "-Body");
                if (modalBody && modalBody.hasAttribute("style")) {
                    modalBody.removeAttribute("style");
                }

                const consumerKeyEl = document.getElementById("consumerKey-" + keyType);
                const consumerSecretEl = document.getElementById("consumerSecret-" + keyType);
                const keyActionsContainer = document.getElementById("keyActionsContainer-" + keyType);

                const consumerKeyContainer = document.getElementById("consumerKeyContainer-" + keyType);
                const consumerKeyContainerView = document.getElementById("consumerKey-" + keyType + "-view");
                const consumerSecretContainerView = document.getElementById("consumerSecret-" + keyType + "-view");

                const curlDisplay = document.getElementById("curlDisplay_" + keyManager + "_" + keyType);
                const kmData = document.getElementById("KMData_" + keyManager + "_" + keyType);

                if (consumerKeyEl) {
                    consumerKeyEl.removeAttribute("class");
                    if (consumerKeyEl.hasAttribute("style")) {
                        consumerKeyEl.removeAttribute("style");
                    }
                    consumerKeyEl.classList.add("col-md-6");
                    consumerKeyEl.value = consumerKey || '';
                }
                if (consumerKeyContainerView && consumerKeyContainerView.hasAttribute("style")) {
                    consumerKeyContainerView.removeAttribute("style");
                }
                if (consumerSecretContainerView && consumerSecretContainerView.hasAttribute("style")) {
                    consumerSecretContainerView.removeAttribute("style");
                }

                if (consumerSecretEl) {
                    consumerSecretEl.value = consumerSecret || '';
                    if (consumerSecretEl.hasAttribute("style")) {
                        consumerSecretEl.removeAttribute("style");
                    }
                }
                if (keyActionsContainer && keyActionsContainer.hasAttribute("style")) {
                    keyActionsContainer.removeAttribute("style");
                }
                if (consumerKeyContainer && consumerKeyContainer.hasAttribute("style")) {
                    consumerKeyContainer.removeAttribute("style");
                }
                if (curlDisplay && curlDisplay.hasAttribute("style")) {
                    curlDisplay.removeAttribute("style");
                }
                if (kmData && kmData.hasAttribute("style")) {
                    kmData.removeAttribute("style");
                }
            }

            // Show containers after key generation (regardless of whether consumerSecret exists)
            const consumerKeyContainerEl = document.getElementById("consumerKeyContainer-" + keyType);
            const consumerKeyViewEl = document.getElementById("consumerKey-" + keyType + "-view");
            const consumerSecretViewEl = document.getElementById("consumerSecret-" + keyType + "-view");
            const keyActionsContainerEl = document.getElementById("keyActionsContainer-" + keyType);
            
            if (consumerKeyContainerEl) {
                consumerKeyContainerEl.style.display = "block";
            }
            if (consumerKeyViewEl) {
                consumerKeyViewEl.style.display = "block";
            }
            if (consumerSecretViewEl && consumerSecret) {
                consumerSecretViewEl.style.display = "block";
            }
            if (keyActionsContainerEl) {
                keyActionsContainerEl.style.display = "flex";
            }
            
            // Update UI elements in the overview section
            const generateKeyContainer = document.getElementById("generateKeyContainer" + "-" +  keyType);
            if (generateKeyContainer) {
                generateKeyContainer.style.display = "none";
            }

            //enable token view
            // document.querySelectorAll("#tokenDisplay_" + keyManager).forEach(tokenDetails => {
            //     tokenDetails.style.display = "block";
            // });
            // //openApiKeyModal(responseData.accessToken, "Generated OAuth Token", "OAuth Token");
            // document.querySelectorAll("#token_" + keyManager).forEach(tokenDetails => {
            //     tokenDetails.textContent = responseData.accessToken;
            // });
            const tokenbtn = document.getElementById('tokenKeyBtn-' + keyType);
            if (tokenbtn) {
                tokenbtn.setAttribute("data-keyMappingId", responseData.keyMappingId);
                tokenbtn.setAttribute("data-consumerSecretID", consumerSecretID);
                tokenbtn.setAttribute("data-app-ref-id", responseData.appRefId);
            }
            subList.forEach(subscription => {
                document.getElementById("generateApiKey_" + subscription.subID)?.setAttribute('data-app-ref-id', `${responseData.appRefId}`);
            })

            if (tokenbtn) {
                tokenbtn.setAttribute("data-scopes", JSON.stringify(responseData.subscriptionScopes));
            }

            if (generateKeyContainer) {
                generateKeyContainer.style.display = 'none';
                generateKeyContainer.classList.add('d-none');
            }
            
            loadKeysViewModal(keyType);


            // // Hide the key action container
            // const keyActionContainer = document.getElementById("key-action-container");
            // keyActionContainer.style.display = "none";
            // const generateKeysButton = document.getElementById("applicationKeyGenerateButton");
            // generateKeysButton.style.display = "none";

            // // Move the advanced configuration section to the placeholder
            // const advancedConfig = document.getElementById("KMData_" + keyManager);
            // const advancedConfigPlaceholder = document.getElementById("advanced-config-placeholder");
            // if (advancedConfig && advancedConfigPlaceholder) {
            //     advancedConfigPlaceholder.appendChild(advancedConfig);
            //     advancedConfig.style.display = "none"; // Keep it hidden initially
            // }

            // // Show the advanced config button
            // const advancedConfigButton = document.getElementById("advanced-config-button");
            // if (advancedConfigButton) {
            //     advancedConfigButton.style.display = "flex";
            // }

            // // Show the token generation buttons
            // const tokenGenerationButtons = document.getElementById("tokenGenerationButtons_" + keyManager);
            // if (tokenGenerationButtons) {
            //     tokenGenerationButtons.style.display = "flex";

            //     // Get the generate token button and update its onClick handler with correct values
            //     const generateTokenButton = tokenGenerationButtons.querySelector(`#apiKeyGenerateButton-${keyType.toLowerCase()}`);
            //     if (generateTokenButton) {
            //         generateTokenButton.setAttribute("onClick", 
            //             `generateOauthKey('${formId}', '${responseData.appRefId}', '${responseData.keyMappingId}', '${keyManager}', '${clientName}')`);
            //     }
            // }

            // // Show the update button container
            // const updateButtonContainer = document.getElementById("applicationKeyUpdateButtonContainer");
            // if (updateButtonContainer) {
            //     updateButtonContainer.style.display = "flex";

            //     // Get the update button and set its onClick handler with the correct appRefID
            //     const updateButton = document.getElementById("applicationKeyUpdateButton");
            //     if (updateButton) {
            //         updateButton.setAttribute("onClick", 
            //             `updateApplicationKey('${formId}', '${JSON.stringify([{appRefID: responseData.appRefId}])}', '${keyType}', '${keyManager}', '${responseData.keyMappingId}', '${clientName}')`);
            //     }
            // }

            // const KMURLs = document.getElementById("KMURl_" + keyManager);
            // KMURLs.style.display = "block";




        } else {
            console.error('Failed to generate keys:', responseData);

            // Show error in the error container
            const errorMessage = responseData.description || responseData.message || 'Unknown error';
            if (errorContainer) {
                errorContainer.textContent = `Failed to generate application keys: ${errorMessage}`;
                errorContainer.style.display = 'block';
            }

            // Reset button state on error
            if (normalState && loadingState && generateBtn) {
                normalState.style.display = 'inline-block';
                loadingState.style.display = 'none';
                generateBtn.disabled = false;
            }
        }
    } catch (error) {
        console.error('Error generating application keys:', error);

        // Show error in the error container
        const errorMessage = error.message || 'Unknown error occurred';
        if (errorContainer) {
            errorContainer.textContent = `Error generating application keys: ${errorMessage}`;
            errorContainer.style.display = 'block';
        }

        // Reset button state on error
        if (normalState && loadingState && generateBtn) {
            normalState.style.display = 'inline-block';
            loadingState.style.display = 'none';
            generateBtn.disabled = false;
        }
    }
}


async function cleanUp(applicationId, keyMappingId) {
    try {
        const response = await fetch(`/devportal/applications/${applicationId}/oauth-keys/${keyMappingId}/clean-up`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
        });
        const responseData = await response.json();
        if (response.ok) {
            await showAlert('Application keys cleaned up successfully!', 'success');
            const url = new URL(window.location.origin + window.location.pathname);
            window.location.href = url.toString();
        } else {
            console.error('Failed to clean up keys:', responseData);
            await showAlert(`Failed to clean up application keys. Please try again.\n${responseData.description}`, 'error');
        }
    } catch (error) {
        console.error('Error:', error);
        await showAlert(`An error occurred cleaning up application keys: \n${error.message}`, 'error');
    }
}


function getFormData(formData, keyManager, clientName, appID) {
    let jsonObject = {
        additionalProperties: {},
    };


    if (keyManager !== 'Resident Key Manager' && !keyManager.includes('_internal_key_manager') && !keyManager.includes('appdev_sts_key_manager')) {
        additionalProperties = {
            "client_id": formData.get('consumerKey'),
            "client_name": clientName,
            "redirect_uris": [formData.get('callbackURL')],
            "grant_types": formData.getAll('grantTypes')
        }
        jsonObject.additionalProperties = additionalProperties;
    }
    const checkboxes = document.querySelectorAll('input[type="checkbox"]');
    const appCheckboxes = Array.from(checkboxes).filter(cb => {
        if (cb.id.includes(appID) && cb.id.includes('additionalProperties')) {
            return true;
        }
        return false;
    });

    formData.forEach((value, key) => {
        if (key.startsWith("additionalProperties.")) {
            const propName = key.replace("additionalProperties.", "");
            // Handle multiple optional value selection
            if (jsonObject.additionalProperties[propName]) {
                if (Array.isArray(jsonObject.additionalProperties[propName])) {
                    jsonObject.additionalProperties[propName].push(value);
                } else {
                    jsonObject.additionalProperties[propName] = [jsonObject.additionalProperties[propName], value];
                }
            } else {
                if (propName === 'response_types') {
                    jsonObject.additionalProperties[propName] = [value];
                } else {
                    jsonObject.additionalProperties[propName] = value;
                }
            }
        } else {
            // Handle multiple checkbox values
            if (jsonObject[key]) {
                if (Array.isArray(jsonObject[key])) {
                    jsonObject[key].push(value);
                } else {
                    jsonObject[key] = [jsonObject[key], value];
                }
            } else {
                if (key === 'grantTypes') {
                    jsonObject[key] = [value];
                } else {
                    jsonObject[key] = value;
                }
            }
        }
    });

    appCheckboxes.forEach(checkbox => {
        let name = checkbox.name.replace("additionalProperties.", "");
        let value = checkbox.checked;

        if (jsonObject.additionalProperties.hasOwnProperty(name)) {
            delete jsonObject.additionalProperties[name];
        }
        jsonObject.additionalProperties[name] = value;

    });


    return jsonObject;
};


async function updateApplicationKey(formId, appMap, keyType, keyManager, keyManagerId, clientName) {
    // Get the update button and set loading state
    console.log("Updating application key with formId:", formId);
    const updateBtn = document.getElementById('applicationKeyUpdateButton' + '-' + keyType);
    if (!updateBtn) {
        console.error('updateApplicationKey: Update button not found for keyType:', keyType);
        return;
    }
    const originalContent = updateBtn.innerHTML;
    updateBtn.innerHTML = '<span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span> Updating...';
    updateBtn.disabled = true;

    // Clear any previous error messages
    const errorContainer = document.getElementById('keyUpdateErrorContainer-' + keyType);
    if (errorContainer) {
        errorContainer.style.display = 'none';
        errorContainer.textContent = '';
    }

    const form = document.getElementById(formId);
    const formData = new FormData(form);
    const jsonAppdata = appMap ? JSON.parse(appMap) : null;
    //TODO: Handle multiple CP applications
    const appKeyManagerId = formId.replace("applicationKeyGenerateForm-", "").replace(/-(SANDBOX|PRODUCTION)$/, "");
    const envSuffix = keyType;
    const appId = jsonAppdata ? jsonAppdata[0].appRefID : document.getElementById("app-ref-" + appKeyManagerId + "-" + envSuffix)?.value;
    const keyMappingId = keyManagerId ? keyManagerId : document.getElementById("key-map-" + appKeyManagerId + "-" + envSuffix)?.value;
    const jsonObject = getFormData(formData, keyManager, clientName, keyManagerId);
    const validationResponse = validateOauthUpdate(jsonObject);
    if (!validationResponse.valid) {
        errorContainer.textContent = validationResponse.message;
        errorContainer.style.display = 'block';

        // Restore button state
        updateBtn.innerHTML = originalContent;
        updateBtn.disabled = false;
    } else {
        const payload = JSON.stringify({
            "supportedGrantTypes": jsonObject.grantTypes,
            "keyType": keyType,
            "keyManager": keyManager,
            "callbackUrl": jsonObject.callbackURL,
            "consumerKey": document.getElementById("consumer-key-" + formId.replace("applicationKeyGenerateForm-", "")).value,
            "consumerSecret": document.getElementById("consumer-secret-" + formId.replace("applicationKeyGenerateForm-", "")).value,
            "keyMappingId": keyMappingId,
            "additionalProperties": jsonObject.additionalProperties
        });
        try {
            const response = await fetch(`/devportal/applications/${appId}/oauth-keys/${keyMappingId}`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: payload,
            });

            const responseData = await response.json();
            if (response.ok) {
                // Restore button state
                updateBtn.innerHTML = originalContent;
                updateBtn.disabled = false;

                closeModal('keysModifyModal-' + keyType);
                await showAlert('Updated Oauth application successfully!', 'success');
                const url = new URL(window.location.origin + window.location.pathname);
                window.location.href = url.toString();
            } else {
                console.error('Failed to update keys:', responseData);

                // Enhanced error message with better formatting
                let errorMessage = 'Failed to update application credentials';
                if (responseData.description) {
                    errorMessage += `: ${responseData.description}`;
                } else if (responseData.message) {
                    errorMessage += `: ${responseData.message}`;
                }

                errorContainer.textContent = errorMessage;
                errorContainer.style.display = 'block';

                // Restore button state
                updateBtn.innerHTML = originalContent;
                updateBtn.disabled = false;
            }
        } catch (error) {
            console.error('Error:', error);

            // Display error message in the modal
            let errorMessage = 'Failed to update application credentials';
            if (error.message) {
                errorMessage += `: ${error.message}`;
            }

            errorContainer.textContent = errorMessage;
            errorContainer.style.display = 'block';

            // Restore button state
            updateBtn.innerHTML = originalContent;
            updateBtn.disabled = false;
        }
    }
}

function validateOauthUpdate(payload) {

    if (!payload.grantTypes) {
        return {
            valid: false,
            message: "Grant types cannot be empty"
        };
    }
    return {
        valid: true
    }
}

async function removeApplicationKey() {
    const modal = document.getElementById('deleteConfirmation');
    const applicationId = modal.dataset.applicationId;
    const keyMappingId = modal.dataset.param2;


    try {
        const response = await fetch(`/devportal/applications/${applicationId}/oauth-keys/${keyMappingId}`, {
            method: 'DELETE',
            headers: {
                'Content-Type': 'application/json',
            },
        });


        const responseData = await response.json();
        if (response.ok) {
            await showAlert('Application keys removed successfully!', 'success');
            const url = new URL(window.location.origin + window.location.pathname);
            window.location.href = url.toString();
        } else {
            console.error('Failed to removed keys:', responseData);
            await showAlert(`Failed to removed application keys. Please try again.\n${responseData.description}`, 'error');
        }
    } catch (error) {
        console.error('Error:', error);
        await showAlert(`An error occurred removing application keys: \n${error.message}`, 'error');
    }
}

async function generateOauthKey(formId, appId, keyMappingId, keyManager, clientName, clientSecret, subscribedScopes, keyType) {
    let tokenBtn = document.getElementById('tokenKeyBtn-' + keyType);
    let regenerateBtn = document.getElementById('regenerateButton_' + keyManager + '_' + keyType);
    const devAppId = tokenBtn?.dataset?.appId
    const scopeContainer = document.getElementById('scopeContainer-' + devAppId + '-' + keyType);
    const scopeInput = document.getElementById('scope-' + devAppId + '-' + keyType);

    if (!(subscribedScopes)) {
        // In the regenerate token request, the scopes are fetched from the span tags
        const scopeElements = document.querySelectorAll(`#scopeContainer-${devAppId}-${keyType} .span-tag`);
        subscribedScopes = Array.from(scopeElements).map(el => el.textContent.replace('×', '').trim());
        scopeContainer.setAttribute('data-scopes', JSON.stringify(subscribedScopes));
        tokenBtn = document.getElementById('regenerateKeyBtn-' + keyType);
    } else {
        /**
         * During the intial generate token request, the data-scopes attribute is set with subcribed scopes
         * after the reload the scopes are fetched from the backend
        */ 
        if (subscribedScopes === '[]') {
            // If the scopes are empty, set it to an empty array
            subscribedScopes = [];
            if (tokenBtn && tokenBtn.dataset?.scopes) {
                scopeContainer.setAttribute('data-scopes', tokenBtn?.dataset?.scopes);
            }
            if (regenerateBtn && regenerateBtn.dataset?.scopes) {
                scopeContainer.setAttribute("data-scopes", regenerateBtn.dataset?.scopes);
            }

            const existingScopes = Array.from(scopeContainer.querySelectorAll('.span-tag'))
            .map(el => el.textContent.replace('×', '').trim());
            if (existingScopes.length > 0) {
                subscribedScopes = existingScopes;
            }
        } else { 
            scopeContainer.setAttribute('data-scopes', subscribedScopes);
            subscribedScopes = JSON.parse(subscribedScopes);
        }
        tokenBtn = document.getElementById('tokenKeyBtn-' + keyType);
        regenerateBtn = document.getElementById('regenerateButton_' + keyManager + '_' + keyType);
    }

    const scopesData = scopeContainer?.dataset?.scopes;
    if (scopesData) {
        // Clear existing scopes
        scopeContainer.querySelectorAll('.span-tag').forEach(el => el.remove());
        const scopes = JSON.parse(scopesData);

        scopes.forEach(scope => {
            addScope(scope);
        });
    }

    scopeContainer?.addEventListener('keypress', function (event) {
        if (event.key === 'Enter') {
            event.preventDefault();
            const input = scopeContainer.querySelector('input');
            const scope = input.value.trim();

            // Add additional scopes
            if (scope) {
                addScope(scope);
                this.value = '';
            }
        }
    });

    function addScope(scope) {
        // Create a new span element for the scope
        const span = document.createElement('span');
        span.className = 'span-tag';
        span.innerHTML = `${scope}<span class="remove">&times;</span>`;

        // Append the new span to the scope container only if it doesn't already exist
        const existingScopes = Array.from(scopeContainer.querySelectorAll('.span-tag'))
            .map(el => el.textContent.replace('×', '').trim());

        if (!existingScopes.includes(scope)) {
            span.querySelector('.remove').addEventListener('click', function () {
                scopeContainer.removeChild(span);
            });
        }

        // Append the new span to the scope container
        scopeContainer.setAttribute('data-scopes', JSON.stringify(subscribedScopes));
        scopeContainer.insertBefore(span, scopeInput);
        scopeInput.value = '';
    }

    // Ensure the input is always visible
    scopeContainer?.addEventListener('click', function () {
        scopeInput.focus();
    });

    const normalState = tokenBtn.querySelector('.button-normal-state');
    const loadingState = tokenBtn.querySelector('.button-loading-state');

    const regenerateNormalState = regenerateBtn.querySelector('.button-normal-state');
    const regenerateLoadingState = regenerateBtn.querySelector('.button-loading-state');
    
    if (regenerateNormalState && regenerateLoadingState && regenerateBtn) {
        regenerateNormalState.style.display = 'none';
        regenerateLoadingState.style.display = 'inline-block';
        regenerateBtn.disabled = true;
    }

    // Clear any previous error messages
    const errorContainer = document.getElementById('keyGenerationErrorContainer-' + keyType);
    if (errorContainer) {
        errorContainer.style.display = 'none';
        errorContainer.textContent = '';
    }

    // Show generating state
    if (normalState && loadingState && tokenBtn) {
        normalState.style.display = 'none';
        loadingState.style.display = 'inline-block';
        tokenBtn.disabled = true;
    }

    const form = document.getElementById(formId);
    const formData = new FormData(form);

    if (!keyMappingId) {
        const tokenbtn = document.getElementById('tokenKeyBtn-' + keyType);
        let clientSecretID = tokenbtn.getAttribute("data-consumerSecretID");
        clientSecret = document.getElementById(clientSecretID).value;
        keyMappingId = tokenbtn.getAttribute("data-keyMappingId");
        appId = tokenbtn.getAttribute("data-app-ref-id");
    }
    const jsonObject = getFormData(formData, keyManager, clientName);

    try {
        const response = await fetch(`/devportal/applications/${appId}/oauth-keys/${keyMappingId}/generate-token`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                "additionalProperties": jsonObject.additionalProperties,
                "consumerSecret": clientSecret,
                "revokeToken": null,
                "scopes": subscribedScopes,
                "validityPeriod": 3600
            }),
            credentials: 'include'
        });


        const responseData = await response.json();


        if (response.ok) {
            // let tokenDetails = document.getElementById("tokenDisplay_" + keyManager);
            // tokenDetails.style.display = "block";
            // tokenDetails.textContent = responseData.accessToken;

            // document.querySelectorAll("#tokenDisplay_" + keyManager).forEach(tokenDetails => {
            //     tokenDetails.style.display = "block";
            // });
            //openApiKeyModal(responseData.accessToken, "Generated OAuth Token", "OAuth Token");
            // document.querySelectorAll("#token_" + keyManager).forEach(tokenDetails => {
            //     tokenDetails.textContent = responseData.accessToken;
            // });
            let tokenDetails = document.getElementById("tokenDisplay_" + keyManager + '_' + keyType);
            if (tokenDetails) {
                tokenDetails.style.display = "block";
            }
            let tokenText = document.getElementById("token_" + keyManager + '_' + keyType);
            if (tokenText) {
                tokenText.textContent = responseData.accessToken;
                tokenText.style.display = "block";
            }
            let copyButton = document.getElementById("copyButton_" + keyManager + '_' + keyType);
            if (copyButton) {
                copyButton.style.display = "block";
            }
            loadKeysTokenModal(keyType);

            // Reset button state
            if (normalState && loadingState && tokenBtn) {
                normalState.style.display = 'inline-block';
                loadingState.style.display = 'none';
                tokenBtn.disabled = false;
            }

            if (regenerateNormalState && regenerateLoadingState && regenerateBtn) {
                regenerateNormalState.style.display = 'inline-block';
                regenerateLoadingState.style.display = 'none';
                regenerateBtn.disabled = false;
            }

            const responseScopeContainer = document.getElementById('responseScopeContainer-' + devAppId + '-' + keyType);
            if (responseScopeContainer) {
              responseScopeContainer.innerHTML = "";
              for (const scope of responseData.tokenScopes) {
                const span = document.createElement("span");
                span.className = "span-tag";
                span.innerHTML = `${scope}`;

                responseScopeContainer.appendChild(span);
              }

              // If no scopes are present, hide the title
              const resScopeTitle = document.getElementById(
                "resScopeTitle-" + keyType
              );
              if (resScopeTitle) {
                if (responseScopeContainer.innerHTML === "") {
                  resScopeTitle.style.display = "none";
                } else {
                  resScopeTitle.style.display = "block";
                  responseScopeContainer.style.display = "block";
                }
              }
            }

            await showAlert('Token generated successfully!', 'success');
        } else {
            console.error('Failed to generate access token:', responseData);

            // Show error in the error container
            if (errorContainer) {
                errorContainer.textContent = `Failed to generate access token: ${responseData.description || 'Unknown error'}`;
                errorContainer.style.display = 'block';
            }

            // Reset button state
            if (normalState && loadingState && tokenBtn) {
                normalState.style.display = 'inline-block';
                loadingState.style.display = 'none';
                tokenBtn.disabled = false;
            }
            if (regenerateNormalState && regenerateLoadingState && regenerateBtn) {
                regenerateNormalState.style.display = 'inline-block';
                regenerateLoadingState.style.display = 'none';
                regenerateBtn.disabled = false;
            }
        }
    } catch (error) {
        console.error('Error:', error);

        // Show error in the error container
        if (errorContainer) {
            errorContainer.textContent = `Error generating access token: ${error.message || 'Unknown error'}`;
            errorContainer.style.display = 'block';
        }

        // Reset button state
        if (normalState && loadingState && tokenBtn) {
            normalState.style.display = 'inline-block';
            loadingState.style.display = 'none';
            tokenBtn.disabled = false;
        }

        if (regenerateNormalState && regenerateLoadingState && regenerateBtn) {
            regenerateNormalState.style.display = 'inline-block';
            regenerateLoadingState.style.display = 'none';
            regenerateBtn.disabled = false;
        }
    }


}

document.addEventListener('DOMContentLoaded', () => {

    const selectElement = document.getElementById("select-idp-list");


    function copyToClipboard(button) {
        const textToCopy = button.parentElement.querySelector('.endpoint-value').textContent;
        navigator.clipboard.writeText(textToCopy)
            .then(() => {
                // Optional: Show a copied notification
                const originalSvg = button.innerHTML;
                button.innerHTML = `
                        <svg class="copy-icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="green" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <polyline points="20 6 9 17 4 12"></polyline>
                        </svg>
                    `;
                setTimeout(() => {
                    button.innerHTML = originalSvg;
                }, 1500);
            })
            .catch(err => {
                console.error('Failed to copy: ', err);
            });
    }


    function updateKeyManagerInfo() {
        if (!selectElement) return;
        document.querySelectorAll(".KMConfig").forEach((el) => {
            el.style.display = "none";
        });
        const selectedValue = selectElement.value;
        const kmURL = document.getElementById("KMURL_" + selectedValue);
        if (kmURL) {
            kmURL.style.display = "block";
        }
    }
    if (selectElement) {
        selectElement.addEventListener("change", updateKeyManagerInfo);
        // Initialize with selected value
        updateKeyManagerInfo();
    }

});



function loadKeyGenModal() {
    const modal = document.getElementById('OauthKeyModal');
    modal.style.display = 'flex';

    // Collapse all advanced configurations and reset UI state
    document.querySelectorAll(".KMConfig").forEach(el => el.style.display = "none");
    document.querySelectorAll(".arrow-icon").forEach(icon => icon.classList.remove('rotated'));

    // Move generate keys button back to original container if needed
    const generateKeysBtn = document.getElementById("applicationKeyGenerateButton");
    const originalContainer = document.getElementById("generate-keys-btn-container");
    const advancedContainer = document.getElementById("generate-keys-btn-advanced-container");

    if (generateKeysBtn && originalContainer && advancedContainer) {
        advancedContainer.style.display = "none";
        originalContainer.style.display = "flex";
        if (generateKeysBtn.parentElement === advancedContainer) {
            originalContainer.appendChild(generateKeysBtn);
        }
    }
}

function loadKeysViewModal(keyType) {
    const modal = document.getElementById('keysViewModal-' + keyType);
    if (!modal) {
        console.error(`Modal keysViewModal-${keyType} not found`);
        return;
    }

    // Re-populate consumer key if the modal input was cleared (e.g., by form.reset() on close)
    const modalConsumerKeyInput = modal.querySelector('input[name="consumerKey"]');
    if (modalConsumerKeyInput && !modalConsumerKeyInput.value && document.getElementById(modalConsumerKeyInput.id + '-view') != null) {
        const mainPageInput = document.getElementById(modalConsumerKeyInput.id + '-view');
        if (mainPageInput && mainPageInput.value) {
            modalConsumerKeyInput.value = mainPageInput.value;
        }
    }

    // Re-populate consumer secret if the modal input was cleared
    const modalConsumerSecretInput = modal.querySelector('input[name="consumerSecret"]');
    if (modalConsumerSecretInput && !modalConsumerSecretInput.value && document.getElementById(modalConsumerSecretInput.id + '-view') != null) {
        const mainPageInput = document.getElementById(modalConsumerSecretInput.id + '-view');
        if (mainPageInput && mainPageInput.value) {
            modalConsumerSecretInput.value = mainPageInput.value;
        }
    }

    modal.style.display = 'flex';

    const authorizationCodeCheckbox = modal.querySelector('input[id^="grant-type-view-authorization_code-"]');
    if (authorizationCodeCheckbox) {
        const pkceFields = modal.querySelectorAll('[id^="row-pkceMandatory"], [id^="row-pkceSupportPlain"]');
        // Handle PKCE fields visibility
        pkceFields.forEach(field => {
            field.style.display = authorizationCodeCheckbox.checked ? 'flex' : 'none';
        });
    }
}

function loadKeysModifyModal(keyType) {
    const modalId = 'keysModifyModal-' + keyType;
    const modal = document.getElementById(modalId);
    if (!modal) {
        console.error(`Modal ${modalId} not found`);
        return;
    }
    modal.style.display = 'flex';

    // Collapse all advanced configurations and reset UI state
    document.querySelectorAll(".arrow-icon").forEach(icon => icon.classList.remove('rotated'));

    // Find the authorization_code checkbox inside this specific modal
    const authorizationCodeCheckbox = modal.querySelector('input[id^="grant-type-authorization_code-"]');
    if (authorizationCodeCheckbox) {
        const callbackUrlRow = modal.querySelector('[id^="callback-url-row"]');
        // Find PKCE-related configuration fields
        const pkceFields = modal.querySelectorAll('[id^="row-pkceMandatory"], [id^="row-pkceSupportPlain"]');

        // Handle callback URL visibility
        if (callbackUrlRow) {
            // Set initial visibility based on checkbox state
            callbackUrlRow.style.display = authorizationCodeCheckbox.checked ? 'flex' : 'none';
            const callbackUrlInput = callbackUrlRow.querySelector('input');
            if (callbackUrlInput) {
                callbackUrlInput.required = authorizationCodeCheckbox.checked;
            }
        }

        // Handle PKCE fields visibility
        pkceFields.forEach(field => {
            field.style.display = authorizationCodeCheckbox.checked ? 'flex' : 'none';
        });

        // Add event listener to toggle visibility when checkbox changes
        authorizationCodeCheckbox.addEventListener('change', function () {
            // Toggle callback URL row
            if (callbackUrlRow) {
                callbackUrlRow.style.display = this.checked ? 'flex' : 'none';
                const callbackUrlInput = callbackUrlRow.querySelector('input');
                if (callbackUrlInput) {
                    callbackUrlInput.required = this.checked;
                }
            }

            // Toggle PKCE fields
            pkceFields.forEach(field => {
                field.style.display = this.checked ? 'flex' : 'none';
            });
        });
    }

    // Add validation for grant types
    validateGrantTypes(modal, keyType);

    // Add event listeners to all grant type checkboxes
    const grantTypeCheckboxes = modal.querySelectorAll('input[name="grantTypes"]');
    grantTypeCheckboxes.forEach(checkbox => {
        checkbox.addEventListener('change', function () {
            validateGrantTypes(modal, keyType);
        });
    });
}

function validateGrantTypes(modal, keyType) {
    // Find the update button with keyType suffix
    const updateButton = modal.querySelector('[id^="applicationKeyUpdateButton"]');
    if (!updateButton) return;

    // Find all grant type checkboxes
    const grantTypeCheckboxes = modal.querySelectorAll('input[name="grantTypes"]');

    // Check if any checkbox is checked
    let isAnyChecked = false;
    grantTypeCheckboxes.forEach(checkbox => {
        if (checkbox.checked) {
            isAnyChecked = true;
        }
    });

    // Update the button state
    updateButton.disabled = !isAnyChecked;

    // Show/hide validation message
    const validationMsg = modal.querySelector('[id^="grantTypeValidationMsg"]');
    if (validationMsg) {
        validationMsg.style.display = isAnyChecked ? 'none' : 'block';
    }
}

function loadKeysTokenModal(keyType) {
    const modalId = 'keysTokenModal-' + keyType;
    const modal = document.getElementById(modalId);
    if (!modal) {
        console.error(`Modal ${modalId} not found`);
        return;
    }
    modal.style.display = 'flex';
}

function loadKeysInstructionsModal(keyType) {
    const modalId = 'keysInstructionsModal-' + keyType;
    const modal = document.getElementById(modalId);
    if (!modal) {
        console.error(`Modal ${modalId} not found`);
        return;
    }
    modal.style.display = 'flex';
}

function showAdvanced(configId) {
    document.querySelectorAll("#" + configId).forEach(content => {
        const isExpanding = content.style.display !== "block";
        content.style.display = isExpanding ? "block" : "none";

        // Get the arrow icon from the clicked header and toggle its rotation
        const headerElement = event.currentTarget;
        const arrowIcon = headerElement.querySelector('.arrow-icon');
        if (arrowIcon) {
            if (isExpanding) {
                arrowIcon.classList.add('rotated');
            } else {
                arrowIcon.classList.remove('rotated');
            }
        }

        // Handle Generate Keys button movement
        const generateKeysBtn = content.querySelector("#applicationKeyGenerateButton");
        const originalContainer = content.querySelector("#generate-keys-btn-container");
        const advancedContainer = content.querySelector("#generate-keys-btn-advanced-container");

        if (generateKeysBtn && originalContainer && advancedContainer) {
            if (isExpanding) {
                // Move Generate Keys button to the advanced container
                originalContainer.style.display = "none";
                advancedContainer.style.display = "flex";
                advancedContainer.appendChild(generateKeysBtn);
            } else {
                // Move Generate Keys button back to its original container
                advancedContainer.style.display = "none";
                originalContainer.style.display = "flex";
                originalContainer.appendChild(generateKeysBtn);
            }
        }
    });
}


async function copyToken(tokenId) {
    // Copy access token
    const tokenElement = document.getElementById('token_' + tokenId);
    if (!tokenElement) {
        return;
    }

    const tokenText = tokenElement.textContent.trim();

    try {
        // Copy to clipboard
        await navigator.clipboard.writeText(tokenText);
        await showAlert('Copied to clipboard!');
    } catch (err) {
        console.error('Could not copy text:', err);
        await showAlert('Failed to copy', true);
    }
}

/**
 * Toggles password visibility for the specified input field
 * @param {string} inputId - The ID of the input field
 */
function togglePasswordVisibility(inputId) {
    document.querySelectorAll('#' + inputId).forEach(inputElement => {
        const buttonElement = inputElement.nextElementSibling;
        const iconElement = buttonElement.querySelector('i');

        // Toggle the input type between password and text
        if (inputElement.type === 'password') {
            inputElement.type = 'text';
            // Change to eye-slash icon
            iconElement.classList.remove('bi-eye');
            iconElement.classList.add('bi-eye-slash');
        } else {
            inputElement.type = 'password';
            // Change back to eye icon
            iconElement.classList.remove('bi-eye-slash');
            iconElement.classList.add('bi-eye');
        }
    });
}

/**
 * Copies the consumer secret to the clipboard
 * @param {string} inputId - The ID of the input field
 */
async function copyConsumerSecret(inputId) {
    const inputElement = document.getElementById(inputId);
    const buttonElement = inputElement.nextElementSibling.nextElementSibling;

    const iconElement = buttonElement.querySelector('i');

    try {
        // Get the value regardless of whether it's shown as password or text
        const secretValue = inputElement.value;

        // Copy to clipboard
        await navigator.clipboard.writeText(secretValue);

        // Show visual feedback
        iconElement.classList.remove('bi-clipboard');
        iconElement.classList.add('bi-clipboard-check');

        // Show alert
        await showAlert('Consumer Secret copied to clipboard!');

        // Revert to original icon after 1.5 seconds
        setTimeout(() => {
            iconElement.classList.remove('bi-clipboard-check');
            iconElement.classList.add('bi-clipboard');
        }, 1500);
    } catch (err) {
        console.error('Could not copy text:', err);
        await showAlert('Failed to copy Consumer Secret', true);
    }
}

async function copyConsumerKey(inputId) {
    const inputElement = document.getElementById(inputId);
    const buttonElement = inputElement.nextElementSibling;
    const iconElement = buttonElement.querySelector('i');

    try {
        // Get the value
        const keyValue = inputElement.value;

        // Copy to clipboard
        await navigator.clipboard.writeText(keyValue);

        // Show visual feedback
        iconElement.classList.remove('bi-clipboard');
        iconElement.classList.add('bi-clipboard-check');

        // Show alert
        await showAlert('Consumer Key copied to clipboard!');

        // Revert to original icon after 1.5 seconds
        setTimeout(() => {
            iconElement.classList.remove('bi-clipboard-check');
            iconElement.classList.add('bi-clipboard');
        }, 1500);
    } catch (err) {
        console.error('Could not copy text:', err);
        await showAlert('Failed to copy Consumer Key', true);
    }
}

async function copyRealCurl(button) {
    console.log("Copying cURL command...", button);
    const keyManagerId = button.id.replace("curl-copy-", "");
    console.log("Key Manager ID:", keyManagerId);
    const tokenEndpoint = button.getAttribute('data-endpoint');
    const consumerKey = document.getElementById("consumer-key-" + keyManagerId).value;
    const consumerSecret = document.getElementById("consumer-secret-" + keyManagerId).value;

    if (!consumerKey || !consumerSecret) {
        await showAlert('Consumer key or secret not available. Please generate keys first.', 'warning');
        return;
    }

    try {
        const credentials = `${consumerKey}:${consumerSecret}`;
        const encodedCredentials = btoa(credentials);
        const curlCommand = `curl -k -X POST ${tokenEndpoint} -d "grant_type=client_credentials" -H "Authorization: Basic ${encodedCredentials}"`;

        // Copy to clipboard
        await navigator.clipboard.writeText(curlCommand);

        // Show visual feedback
        const originalSvg = button.innerHTML;
        button.innerHTML = `
            <svg class="copy-icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="green" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="20 6 9 17 4 12"></polyline>
            </svg>
        `;

        // Show alert
        await showAlert('cURL command with your credentials has been copied to clipboard!');

        // Revert to original icon after 1.5 seconds
        setTimeout(() => {
            button.innerHTML = originalSvg;
        }, 1500);
    } catch (err) {
        console.error('Could not copy text:', err);
        await showAlert('Failed to copy cURL command: ' + err.message, 'error');
    }
}

async function copyOauthURLs(inputId) {

    const inputElement = document.getElementById(inputId);
    const buttonElement = inputElement.nextElementSibling;
    const iconElement = buttonElement.querySelector('i');

    try {
        // Get the value regardless of whether it's shown as password or text
        const secretValue = inputElement.value;

        // Copy to clipboard
        await navigator.clipboard.writeText(secretValue);

        // Show visual feedback
        iconElement.classList.remove('bi-clipboard');
        iconElement.classList.add('bi-clipboard-check');

        // Show alert
        await showAlert('URL copied to clipboard!');

        // Revert to original icon after 1.5 seconds
        setTimeout(() => {
            iconElement.classList.remove('bi-clipboard-check');
            iconElement.classList.add('bi-clipboard');
        }, 1500);
    } catch (err) {
        console.error('Could not copy text:', err);
        await showAlert('Failed to copy Consumer Secret', true);
    }
}

function loadModal(modalID) {
    const modal = document.getElementById(modalID);
    modal.style.display = 'flex';
}


