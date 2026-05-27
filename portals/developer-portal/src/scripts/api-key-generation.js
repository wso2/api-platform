function openApiKeyNameModal(projectID, apiRefID, subPlan, cpAppID, appID, subID, subIDs, subscribedScopes, keyType) {

  const modal = document.getElementById('apiKeyNameModal-' + subID + '-' + keyType);
  if (!modal) {
    return;
  }
  modal.style.display = 'flex';

  const confirmBtn = document.getElementById('confirmAPIKeyNameBtn-' + subID + '-' + keyType);
  if (confirmBtn) {
    confirmBtn.onclick = async function () {
      const nameInput = document.getElementById('apiKeyName-' + subID + '-' + keyType);
      const apiKeyName = nameInput ? nameInput.value.trim() : '';
      if (!apiKeyName) {
        await showAlert('Please enter an API Key name.', 'error');
        return;
      }
      modal.style.display = 'none';
      openApiKeyModal(projectID, apiRefID, subPlan, cpAppID, appID, subID, subIDs, subscribedScopes, keyType);
    };
  }
}

function openApiKeyModal(projectID, apiRefID, subPlan, cpAppID, appID, subID, subIDs, subscribedScopes, keyType) {

  let scopes = Array.isArray(subscribedScopes) ? subscribedScopes : JSON.parse(subscribedScopes);

  if (scopes.length < 1) {
    return generateAPIKey(projectID, apiRefID, subPlan, cpAppID, appID, subID, subIDs, 'generateKeyBtn-', keyType)
  }
  const modal = document.getElementById('apiKeyModal-' + subID + '-' + keyType);
  if (!modal) {
    return;
  }
  modal.style.display = 'flex';

  const generateBtn = document.getElementById("generateAPIKeyBtn-" + subID + '-' + keyType);
  if (generateBtn) {
    generateBtn.style.display = 'block';
  }
  const apiKeyCard = document.getElementById('apiKeyCard-' + subID + '-' + keyType);
  if (apiKeyCard) {
    apiKeyCard.classList.add('d-none');
  }
  const apiKeyInfo = document.getElementById('apiKeyInfo-' + subID + '-' + keyType);
  if (apiKeyInfo) {
    apiKeyInfo.classList.add('d-none');
  }

  const scopeContainer = document.getElementById('scopeContainer-' + subID + '-' + keyType);
  if (scopeContainer) {
    scopeContainer.setAttribute('data-scopes', subscribedScopes);
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
    scopeContainer.appendChild(span);

  }

  document.getElementById('generateAPIKeyBtn-' + subID + '-' + keyType).setAttribute('onclick', `generateAPIKey('${projectID}', '${apiRefID}', '${subPlan}', '${cpAppID}', '${appID}', '${subID}', '${subIDs}', 'generateAPIKeyBtn-', '${keyType}')`);
}

async function generateAPIKey(projectID, apiID, subPlan, cpAppID, appID, subID, subIDs, tokenBtnPrefix, keyType) {

  const tokenBtn = document.getElementById(tokenBtnPrefix + subID + '-' + keyType);
  if (!tokenBtn) {
    return;
  }
  const normalState = tokenBtn.querySelector('.button-normal-state');
  const loadingState = tokenBtn.querySelector('.button-loading-state');
  if (!normalState || !loadingState) {
    return;
  }
  const nameInput = document.getElementById('apiKeyName-' + subID + '-' + keyType);
  const apiKeyName = nameInput ? nameInput.value.trim() : '';

  const policyElement = document.getElementById('policy_' + subID);
  if (!policyElement) {
    console.error('Unable to find subscription plan.');
    normalState.style.display = 'inline-block';
    loadingState.style.display = 'none';
    return;
  }
  const subscriptionPlan = policyElement.textContent.trim();
  const scopeContainer = document.getElementById('scopeContainer-' + subID + '-' + keyType);
  if (!scopeContainer) {
    console.error('Unable to find scope container.');
    normalState.style.display = 'inline-block';
    loadingState.style.display = 'none';
    return;
  }
  const scopeTags = scopeContainer.querySelectorAll('.span-tag');
  const scopes = Array.from(scopeTags).map(el => el.textContent.replace('×', '').trim());
  scopeContainer.setAttribute('data-scopes', [scopes]);

  normalState.style.display = 'none';
  loadingState.style.display = 'inline-block';

  if (!apiKeyName) {
    await showAlert('Please enter an API Key name.', 'error');
    normalState.style.display = 'inline-block';
    loadingState.style.display = 'none';
    return;
  }

  const uri = `/devportal/api-keys/generate`;

  const JSONbody = JSON.stringify(
    {
      "applicationId": cpAppID ? `${cpAppID}` : tokenBtn.getAttribute('data-app-ref-id'),
      "apiId": `${apiID}`,
      "subscriptionPlan": `${subscriptionPlan}`,
      "scopes": scopes,
      "keyType": `${keyType.toUpperCase()}`,
      "projectID": `${projectID}`,
      "devportalAppId": `${appID}`,
      "name": apiKeyName,
    });

  try {
    const response = await fetch(uri, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSONbody,
    });

    const responseData = await response.json();

    if (response.ok) {
      const modal = document.getElementById('apiKeyModal-' + subID + '-' + keyType);
      if (!modal) {
        return;
      }
      modal.style.display = 'flex';

      const apiKeyCard = document.getElementById('apiKeyCard-' + subID + '-' + keyType);
      if (apiKeyCard) {
        apiKeyCard.classList.remove('d-none');
      }
      const apiKeyInfo = document.getElementById('apiKeyInfo-' + subID + '-' + keyType);
      if (apiKeyInfo) {
        apiKeyInfo.classList.remove('d-none');
      }
      const tokenApiKeyText = document.getElementById("token_apiKeyText-" + subID + '-' + keyType);
      if (tokenApiKeyText) {
        tokenApiKeyText.textContent = responseData.value;
      }
      
      // Hide the generate button in the modal
      const apiKeyModalGenerateBtn = document.getElementById('generateAPIKeyBtn-' + subID + '-' + keyType);
      if (apiKeyModalGenerateBtn) {
        apiKeyModalGenerateBtn.style.display = 'none';
      }

      const generateAPIKeyBtn = document.getElementById(
        tokenBtnPrefix + subID + "-" + keyType
      );
      if (generateAPIKeyBtn) {
        generateAPIKeyBtn.style.display = "none";
      }

      // Update API Key Name table cell on manage-keys page without reload
      const nameCellId = "apiKeyNameCell-" + subID + "-" + keyType;
      const nameCell = document.getElementById(nameCellId);
      if (nameCell) {
        nameCell.textContent = apiKeyName;
      }

      // Also update regenerate/revoke buttons in manage-keys page if they exist
      let regenerateBtn = document.getElementById('regenerateKeyBtn-' + subID + '-' + keyType);
      if (regenerateBtn) {
        regenerateBtn.style.display = 'inline-flex';
        regenerateBtn.setAttribute('data-api-key-id', responseData.id);
        regenerateBtn.setAttribute('data-scopes', `${JSON.stringify(scopes)}`);
      }

      let revokeBtn = document.getElementById('revokeKeyBtn-' + subID + '-' + keyType);
      if (revokeBtn) {
        revokeBtn.style.display = 'inline-flex';
        revokeBtn.setAttribute('data-api-key-id', responseData.id);
      }

      const removeElementSpan = scopeContainer.querySelectorAll('.remove');

      if (removeElementSpan.length > 0) {
        removeElementSpan.forEach(el => {
          el.style.display = 'none';
        });
      } else {
        const scopeContainerEl = document.getElementById("scopeContainer-" + subID + '-' + keyType);
        const scopeTitleEl = document.getElementById("scopeTitle-" + subID + '-' + keyType);
        if (scopeContainerEl) scopeContainerEl.style.display = 'none';
        if (scopeTitleEl) scopeTitleEl.style.display = 'none';
      }

      const subList = JSON.parse(subIDs);
      subList.forEach(subID => {
        document.getElementById("generateAPIKeyBtn-" + subID + '-' + keyType)?.setAttribute('data-app-ref-id', `${responseData.appRefId}`);
      })

      await showAlert('API Key generated successfully!', 'success');

    } else {
      await showAlert(`Failed to generate API Key. Please try again.\n${responseData?.description || ''}`, 'error');
    }
  } catch (error) {
    await showAlert(`Failed to generate API Key. Please try again.\n${error}`, 'error');
  }
  normalState.style.display = 'inline-block';
  loadingState.style.display = 'none';
}

async function revokeAPIKey(apiKeyID, subID, appID, apiRefID, keyType) {

  let revokeBtn = document.getElementById('revokeKeyBtn-' + subID + '-' + keyType);
  if (!revokeBtn) {
    return;
  }
  const normalState = revokeBtn.querySelector('.button-normal-state');
  const loadingState = revokeBtn.querySelector('.button-loading-state');

  normalState.style.display = 'none';
  loadingState.style.display = 'inline-block';

  const uri = `/devportal/api-keys/${apiKeyID}/revoke`;

  try {
    const response = await fetch(uri, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        "applicationId": `${appID}`,
        "apiRefID": `${apiRefID}`,
      }),
    });

    const responseData = await response.json();
    if (response.ok) {
      let regenerateBtn = document.getElementById('regenerateKeyBtn-' + subID + '-' + keyType);
      if (regenerateBtn) {
        regenerateBtn.style.display = 'none';
      }

      revokeBtn.style.display = 'none';

      // Show the generate button if it exists
      let generateBtn = document.getElementById('generateKeyBtn-' + subID + '-' + keyType);
      if (generateBtn) {
        generateBtn.style.display = 'inline-flex';
      }

      // Clear API Key Name cell on manage-keys page without reload
      const nameCellId = 'apiKeyNameCell-' + subID + '-' + keyType;
      const nameCell = document.getElementById(nameCellId);
      if (nameCell) {
        nameCell.textContent = '';
      }

      await showAlert('API Key revoked successfully!', 'success');

    } else {
      await showAlert(`Failed to revoke API Key. Please try again.\n${responseData?.description || ''}`, 'error');
    }
  } catch (error) {
    await showAlert(`Failed to revoke API Key. Please try again.\n${error}`, 'error');
  }
  normalState.style.display = 'inline-block';
  loadingState.style.display = 'none';
}

async function regenerateAPIKey(apiKeyID, subID, keyType) {

  const tokenBtn = document.getElementById('regenerateKeyBtn-' + subID + '-' + keyType);
  if (!tokenBtn) {
    return;
  }
  const normalState = tokenBtn.querySelector('.button-normal-state');
  const loadingState = tokenBtn.querySelector('.button-loading-state');
  if (!normalState || !loadingState) {
    return;
  }
  normalState.style.display = 'none';
  loadingState.style.display = 'inline-block';

  const uri = `/devportal/api-keys/${apiKeyID}/regenerate`;

  try {
    const response = await fetch(uri, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
    });

    const responseData = await response.json();

    if (response.ok) {
      const modal = document.getElementById('apiKeyModal-' + subID + '-' + keyType);
      if (!modal) {
        return;
      }
      modal.style.display = 'flex';

      const tokenText = document.getElementById("token_apiKeyText-" + subID + '-' + keyType);
      if (tokenText) {
        tokenText.textContent = responseData.value;
      }

      const regenerateBtn = document.getElementById("regenerateKeyBtn-" + subID + '-' + keyType);

      const scopeContainer = document.getElementById('scopeContainer-' + subID + '-' + keyType);
      if (!scopeContainer) {
        return;
      }
      const scopes = regenerateBtn?.dataset?.scopes || '[]';

      if (JSON.parse(scopes).length > 0) {
        const existingScopes = Array.from(scopeContainer.querySelectorAll('.span-tag'));
        existingScopes.forEach(el => scopeContainer.removeChild(el));

        JSON.parse(scopes).forEach(scope => {
          const span = document.createElement('span');
          span.className = 'span-tag';
          span.innerHTML = `${scope}`;

          scopeContainer.appendChild(span);
        });
      } else {
        scopeContainer.style.display = 'none';
        const scopeTitle = document.getElementById("scopeTitle-" + subID + '-' + keyType);
        if (scopeTitle) {
          scopeTitle.style.display = 'none';
        }
      }

      await showAlert('API Key regenerated successfully!', 'success');
    } else {
      await showAlert(`Failed to regenerate API Key. Please try again.\n${responseData?.description || ''}`, 'error');
    }
  } catch (error) {
    console.log(error);
    await showAlert(`Failed to generate API Key. Please try again.\n${error}`, 'error');
  }
  normalState.style.display = 'inline-block';
  loadingState.style.display = 'none';
}
