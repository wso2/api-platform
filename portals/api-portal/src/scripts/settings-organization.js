/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
 

(function () {
  // Local fallback for the shared bindFormValidity (defined in alert.js, loaded first
  // on the settings page): keeps save working even if it were ever unavailable, only
  // skipping the disable-until-valid behaviour instead of throwing at init.
  var bindFormValidity = window.bindFormValidity || function () { return function () {}; };

  function g(id) { return document.getElementById(id); }
  var saveBtn = g('cfg-save-org-btn');
  if (!saveBtn) return;

  var emailRe = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

  /* Disable save while the organization Name is empty. */
  bindFormValidity(saveBtn, ['org-display'], function () {
    return g('org-display').value.trim() !== '';
  });

  saveBtn.addEventListener('click', async function () {
    var handle = saveBtn.dataset.handle;
    var name = g('org-display').value.trim();
    if (!name) { await showAlert('Name is required.', 'error'); return; }
    var oe = g('org-owner-email').value.trim();
    if (oe && !emailRe.test(oe)) { await showAlert('Business owner email is not a valid email address.', 'error'); return; }
    // displayName, id and idpRefId are required by the update schema; send them always.
    var body = {
      displayName: name,
      id: handle,
      idpRefId: g('org-idp-ref').value.trim(),
    };
    // orgDao.update overwrites the whole configuration column, so resend what is
    // already stored rather than dropping keys this form doesn't edit. Which
    // artifact types the portal serves are operator config ([api_portal.artifacts]),
    // not per-organization state, so nothing here writes to it.
    try {
      var parsed = JSON.parse(saveBtn.dataset.configuration || '{}');
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) body.configuration = parsed;
    } catch (e) { /* fall back to omitting it — a bad blob must not block the save */ }
    // Optional fields — only send non-blank (businessOwnerEmail is format-validated above).
    var owner = g('org-owner').value.trim();         if (owner) body.businessOwner = owner;
    var oc    = g('org-owner-contact').value.trim(); if (oc)    body.businessOwnerContact = oc;
    if (oe) body.businessOwnerEmail = oe;
    // Always sent, blank included — same as idpRefId above. orgDao.update writes
    // cp_ref_id unconditionally, so omitting a cleared field would leave the old
    // value in place and make the reference impossible to remove from this form.
    body.cpRefId = g('org-cp-ref').value.trim();
    await withButtonBusy(saveBtn, 'Saving…', async function () {
      try {
        var res = await fetch(window.apiPortalApi.root('/organizations/' + encodeURIComponent(handle)), {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
          credentials: 'same-origin', body: JSON.stringify(body),
        });
        if (res.ok) {
          await showAlert('Organization updated.', 'success');
          window.location.reload();
        } else {
          var err = await res.json().catch(function () { return {}; });
          await showAlert('Failed: ' + (err.description || err.message || res.statusText), 'error');
        }
      } catch (e) { await showAlert('Error: ' + e.message, 'error'); }
    });
  });
}());
