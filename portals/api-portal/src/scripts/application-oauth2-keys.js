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

// The application page's "OAuth2 keys" section. Associating and removing only —
// keys themselves are created on the Keys → OAuth2 Keys page, because creating
// one registers a real client on an identity server and that is not something to
// bury inside an application's detail view.
//
// Mirrors application-api-keys.js, which does the same job for API keys.
(function () {
    var config = document.getElementById('application-oauth2-keys-config');
    if (!config) return;

    var appId = config.getAttribute('data-app-id');
    var _submitting = false;

    function mutationHeaders() {
        return { 'Content-Type': 'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() };
    }
    function show(id) { var el = document.getElementById(id); if (el) el.style.display = 'flex'; }
    function hide(id) { var el = document.getElementById(id); if (el) el.style.display = 'none'; }
    async function alertMsg(message, type) {
        if (typeof showAlert === 'function') { try { await showAlert(message, type); } catch (e) { /* noop */ } }
    }

    // Keys the caller owns that are not already on this application. Rendered with
    // the page by the controller, so the picker needs no round trip to open.
    function availableKeys() {
        try {
            return JSON.parse(config.getAttribute('data-available-oauth2-keys') || '[]');
        } catch (e) {
            return [];
        }
    }

    /* ── associate ────────────────────────────────────────────── */

    function openAssociate() {
        var keys = availableKeys();
        var sel = document.getElementById('associate-oauth2-key-select');
        sel.textContent = '';

        var placeholder = document.createElement('option');
        placeholder.value = '';
        placeholder.textContent = keys.length ? '— Select a key —' : '— No keys available —';
        sel.appendChild(placeholder);

        keys.forEach(function (key) {
            var opt = document.createElement('option');
            opt.value = key.keyId;
            // Named by key manager and consumer key: a key has no display name of
            // its own, and the consumer key is what identifies it to its owner.
            var label = key.keyManagerName + ' · ' + key.consumerKey;
            // Say where it is now, so moving it is a decision rather than a surprise.
            opt.textContent = key.applicationName ? label + '  (on ' + key.applicationName + ')' : label;
            sel.appendChild(opt);
        });

        document.getElementById('btn-submit-associate-oauth2-key').disabled = true;
        if (!keys.length) {
            alertMsg('You have no OAuth2 keys to associate. Generate one from Keys → OAuth2 Keys first.', 'error');
            return;
        }
        show('associate-oauth2-key-modal');
    }

    async function submitAssociate() {
        if (_submitting) return;
        var keyId = document.getElementById('associate-oauth2-key-select').value;
        if (!keyId) return;

        _submitting = true;
        var btn = document.getElementById('btn-submit-associate-oauth2-key');
        btn.disabled = true;
        try {
            var resp = await fetch(
                window.apiPortalApi.root('/oauth2-keys/' + encodeURIComponent(keyId) + '/associate'),
                { method: 'POST', headers: mutationHeaders(), body: JSON.stringify({ applicationId: appId }) }
            );
            if (!resp.ok) {
                var body = null;
                try { body = await resp.json(); } catch (e) { /* not JSON */ }
                await alertMsg((body && body.message) || 'Could not associate the key.', 'error');
                return;
            }
            hide('associate-oauth2-key-modal');
            window.location.reload();
        } catch (e) {
            await alertMsg('Could not reach the server. Check your connection and try again.', 'error');
        } finally {
            _submitting = false;
            btn.disabled = false;
        }
    }

    /* ── remove ───────────────────────────────────────────────── */

    async function removeAssociation(keyId) {
        if (_submitting) return;
        _submitting = true;
        try {
            // Dissociate, not delete: the OAuth2 client at the key manager is left
            // alone, and the key stays usable — it simply stops counting towards
            // this application.
            var resp = await fetch(
                window.apiPortalApi.root('/oauth2-keys/' + encodeURIComponent(keyId) + '/dissociate'),
                { method: 'POST', headers: mutationHeaders() }
            );
            if (!resp.ok) {
                var body = null;
                try { body = await resp.json(); } catch (e) { /* not JSON */ }
                await alertMsg((body && body.message) || 'Could not remove the association.', 'error');
                return;
            }
            window.location.reload();
        } catch (e) {
            await alertMsg('Could not reach the server. Check your connection and try again.', 'error');
        } finally {
            _submitting = false;
        }
    }

    /* ── wiring ───────────────────────────────────────────────── */

    var openBtn = document.getElementById('btn-open-associate-oauth2-key');
    if (openBtn) openBtn.addEventListener('click', openAssociate);

    ['associate-oauth2-key-close', 'associate-oauth2-key-cancel'].forEach(function (id) {
        var b = document.getElementById(id);
        if (b) b.addEventListener('click', function () { hide('associate-oauth2-key-modal'); });
    });

    var sel = document.getElementById('associate-oauth2-key-select');
    if (sel) {
        sel.addEventListener('change', function () {
            document.getElementById('btn-submit-associate-oauth2-key').disabled = !sel.value;
        });
    }

    var submit = document.getElementById('btn-submit-associate-oauth2-key');
    if (submit) submit.addEventListener('click', submitAssociate);

    // Delegated: the rows are server-rendered and replaced on every reload.
    document.addEventListener('click', function (e) {
        var btn = e.target.closest ? e.target.closest('.btn-remove-oauth2-key-association') : null;
        if (btn) removeAssociation(btn.getAttribute('data-key-id'));
    });
})();
