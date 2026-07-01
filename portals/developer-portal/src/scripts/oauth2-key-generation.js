async function addClientId(kmId, keyType, appId, orgId, keyManager) {
    const input = document.getElementById('addClientIdInput-' + kmId + '-' + keyType);
    const btn = document.getElementById('addClientIdBtn-' + kmId + '-' + keyType);
    const errorContainer = document.getElementById('addClientIdError-' + kmId + '-' + keyType);
    if (errorContainer) {
        errorContainer.style.display = 'none';
        errorContainer.textContent = '';
    }

    const consumerKey = input ? input.value.trim() : '';
    if (!consumerKey) {
        if (errorContainer) {
            errorContainer.textContent = 'Please enter a client ID.';
            errorContainer.style.display = 'block';
        }
        return;
    }

    const normalState = btn?.querySelector('.button-normal-state');
    const loadingState = btn?.querySelector('.button-loading-state');
    if (normalState && loadingState && btn) {
        normalState.style.display = 'none';
        loadingState.style.display = 'inline-block';
        btn.disabled = true;
    }

    try {
        const response = await fetch(devportalApi.org(`/applications/${appId}/generate-keys`), {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ keyManager, type: keyType, consumerKey }),
        });
        const data = await response.json();
        if (response.ok) {
            window.location.reload();
        } else {
            const message = data.description || data.message || 'Failed to add client ID.';
            if (errorContainer) {
                errorContainer.textContent = message;
                errorContainer.style.display = 'block';
            }
            if (normalState && loadingState && btn) {
                normalState.style.display = 'inline-block';
                loadingState.style.display = 'none';
                btn.disabled = false;
            }
        }
    } catch (error) {
        if (errorContainer) {
            errorContainer.textContent = error.message || 'Failed to add client ID.';
            errorContainer.style.display = 'block';
        }
        if (normalState && loadingState && btn) {
            normalState.style.display = 'inline-block';
            loadingState.style.display = 'none';
            btn.disabled = false;
        }
    }
}

function confirmAndRevokeKeys(applicationId, keyMappingId, keyType) {
    const modal = document.getElementById('deleteConfirmation');
    if (modal) {
        const titleEl = modal.querySelector('.modal-title');
        const msgEl = modal.querySelector('.modal-message');
        if (titleEl) titleEl.textContent = 'Revoke Application Keys';
        if (msgEl) msgEl.textContent = 'Are you sure you want to remove this client ID? Tokens already issued remain valid until they expire.';
        modal.dataset.applicationId = applicationId;
        modal.dataset.param2 = keyMappingId;

        const confirmBtn = document.getElementById('deleteConfirmationBtn');
        const originalConfirmHtml = confirmBtn?.innerHTML;
        const handler = async () => {
            confirmBtn.removeEventListener('click', handler);
            if (confirmBtn) {
                confirmBtn.disabled = true;
                confirmBtn.innerHTML = '<span class="spinner-border spinner-border-sm" role="status"></span> Revoking…';
            }
            await removeApplicationKeys(applicationId, keyMappingId, keyType);
            if (confirmBtn) {
                confirmBtn.disabled = false;
                confirmBtn.innerHTML = originalConfirmHtml;
            }
        };
        confirmBtn.addEventListener('click', handler);

        const bsModal = new bootstrap.Modal(modal);
        bsModal.show();
    } else if (confirm('Are you sure you want to remove this client ID? This cannot be undone.')) {
        removeApplicationKeys(applicationId, keyMappingId, keyType);
    }
}

async function removeApplicationKey() {
    const modal = document.getElementById('deleteConfirmation');
    const applicationId = modal.dataset.applicationId;
    const keyMappingId = modal.dataset.param2;
    await removeApplicationKeys(applicationId, keyMappingId);
}

async function removeApplicationKeys(applicationId, keyMappingId, keyType) {
    if (!keyMappingId && keyType) {
        const tokenBtn = document.getElementById('tokenKeyBtn-' + keyType);
        keyMappingId = tokenBtn?.dataset?.keymappingid;
    }
    if (!applicationId || !keyMappingId) {
        await showAlert('Unable to remove keys. Please reload the page and try again.', 'error');
        return;
    }
    try {
        const response = await fetch(devportalApi.org(`/applications/${applicationId}/oauth-keys/${keyMappingId}`), {
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
            console.error('Failed to remove keys:', responseData);
            await showAlert(`Failed to remove application keys. Please try again.\n${responseData.description}`, 'error');
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
            try {
                const parsed = JSON.parse(subscribedScopes);
                scopeContainer.setAttribute('data-scopes', subscribedScopes);
                subscribedScopes = parsed;
            } catch (e) {
                subscribedScopes = [];
            }
        }
        tokenBtn = document.getElementById('tokenKeyBtn-' + keyType);
        regenerateBtn = document.getElementById('regenerateButton_' + keyManager + '_' + keyType);
    }

    const scopesData = scopeContainer?.dataset?.scopes;
    if (scopesData) {
        scopeContainer.querySelectorAll('.span-tag').forEach(el => el.remove());
        try {
            const scopes = JSON.parse(scopesData);
            scopes.forEach(scope => {
                addScope(scope);
            });
        } catch (e) {
            // Scopes data is not valid JSON; skip rendering
        }
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

    const regenerateNormalState = regenerateBtn?.querySelector('.button-normal-state');
    const regenerateLoadingState = regenerateBtn?.querySelector('.button-loading-state');

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
        keyMappingId = tokenbtn?.getAttribute("data-keymappingid") || tokenbtn?.getAttribute("data-keyMappingId");
        if (!appId) appId = tokenbtn?.getAttribute("data-app-id");
        if (!clientSecret) {
            const clientSecretId = tokenbtn?.getAttribute("data-consumerSecretId");
            if (clientSecretId) clientSecret = document.getElementById(clientSecretId)?.value;
        }
    }

    try {
        const response = await fetch(devportalApi.org(`/applications/${appId}/oauth-keys/${keyMappingId}/generate-token`), {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                "consumerSecret": clientSecret,
                "revokeToken": null,
                "scopes": subscribedScopes,
                "validityPeriod": 3600
            }),
            credentials: 'include'
        });


        const responseData = await response.json();


        if (response.ok) {
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

async function copyToken(btn, tokenId) {
    // Copy access token
    const tokenElement = document.getElementById('token_' + tokenId);
    if (!tokenElement) return;
    const tokenText = tokenElement.textContent.trim();
    try { navigator.clipboard.writeText(tokenText).catch(function(){}); } catch(e) {}
    if (!btn) return;
    btn.classList.add('copy-btn--copied');
    if (btn._copyTimer) clearTimeout(btn._copyTimer);
    btn._copyTimer = setTimeout(function() { btn.classList.remove('copy-btn--copied'); }, 1600);
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

async function copyConsumerKey(inputId) {
    const inputElement = document.getElementById(inputId);
    const buttonElement = inputElement.nextElementSibling;
    try {
        await navigator.clipboard.writeText(inputElement.value);
        buttonElement.classList.add('copy-btn--copied');
        if (buttonElement._copyTimer) clearTimeout(buttonElement._copyTimer);
        buttonElement._copyTimer = setTimeout(() => { buttonElement.classList.remove('copy-btn--copied'); }, 1600);
    } catch (err) {
        console.error('Could not copy text:', err);
    }
}

async function copyRealCurl(button) {
    const keyManagerId = button.id.replace("curl-copy-", "");
    const tokenEndpoint = button.getAttribute('data-endpoint');
    const consumerKeyEl = document.getElementById("consumer-key-" + keyManagerId);
    const consumerKey = consumerKeyEl ? consumerKeyEl.value : '';

    if (!consumerKey) return;

    try {
        const curlCommand = `curl -k -X POST ${tokenEndpoint} -d "grant_type=client_credentials" -H "Authorization: Basic $(echo -n '${consumerKey}:<your_consumer_secret>' | base64)"`;
        await navigator.clipboard.writeText(curlCommand);
        button.classList.add('copy-btn--copied');
        if (button._copyTimer) clearTimeout(button._copyTimer);
        button._copyTimer = setTimeout(() => { button.classList.remove('copy-btn--copied'); }, 1600);
    } catch (err) {
        console.error('Could not copy text:', err);
    }
}

function loadModal(modalID) {
    const modal = document.getElementById(modalID);
    modal.style.display = 'flex';
}

function closeModal(modalID) {
    const modal = document.getElementById(modalID);
    if (modal) modal.style.display = 'none';
}

// ---------------------------------------------------------------------------
// Generate-token secret prompt
// ---------------------------------------------------------------------------

/** Pending params captured when the user clicks a Generate token button. */
let _pendingGenerateTokenParams = null;

/**
 * Open the consumer-secret prompt modal before generating a token.
 * Called by all "Generate" token buttons instead of generateOauthKey directly.
 */
function openGenerateTokenModal(formId, appId, keyMappingId, keyManager, clientName, subscribedScopes, keyType) {
    const tokenBtn = document.getElementById('tokenKeyBtn-' + keyType);
    if ((!appId || appId === 'undefined') && tokenBtn?.dataset?.appId) {
        appId = tokenBtn.dataset.appId;
    }
    if ((!keyMappingId || keyMappingId === 'undefined') && tokenBtn?.dataset?.keymappingid) {
        keyMappingId = tokenBtn.dataset.keymappingid;
    }
    if (!subscribedScopes || subscribedScopes === 'undefined') {
        subscribedScopes = tokenBtn?.dataset?.scopes || '[]';
    }

    closeModal('keysTokenModal-' + keyType);

    _pendingGenerateTokenParams = { formId, appId, keyMappingId, keyManager, clientName, subscribedScopes, keyType };
    const input = document.getElementById('generateTokenPromptSecretInput');
    const errorEl = document.getElementById('generateTokenPromptError');
    if (input) input.value = '';
    if (errorEl) errorEl.style.display = 'none';
    const modal = document.getElementById('generateTokenPromptModal');
    if (modal) modal.style.display = 'flex';
}

/**
 * Confirm button handler inside the prompt modal.
 * Validates the entered secret then delegates to generateOauthKey.
 */
function confirmGenerateTokenPrompt() {
    const input = document.getElementById('generateTokenPromptSecretInput');
    const errorEl = document.getElementById('generateTokenPromptError');
    const secret = input ? input.value.trim() : '';

    if (!secret) {
        if (errorEl) errorEl.style.display = 'block';
        return;
    }
    if (errorEl) errorEl.style.display = 'none';
    closeModal('generateTokenPromptModal');

    if (_pendingGenerateTokenParams) {
        const { formId, appId, keyMappingId, keyManager, clientName, subscribedScopes, keyType } = _pendingGenerateTokenParams;
        _pendingGenerateTokenParams = null;
        generateOauthKey(formId, appId, keyMappingId, keyManager, clientName, secret, subscribedScopes, keyType);
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
