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
    // Whether this application already holds a key. The only thing that tells the
    // two empty-picker cases apart — see openAssociate below.
    var hasAssociatedKeys = config.getAttribute('data-has-associated-keys') === 'true';
    // Keys the caller owns that sit on other applications. They are not offered —
    // taking one would detach it from an application this page is not about — but
    // an empty picker has to say that rather than imply the caller owns nothing.
    var keysElsewhere = parseInt(config.getAttribute('data-keys-elsewhere'), 10) || 0;
    var _submitting = false;

    function mutationHeaders() {
        return { 'Content-Type': 'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() };
    }
    function show(id) { var el = document.getElementById(id); if (el) el.style.display = 'flex'; }
    function hide(id) { var el = document.getElementById(id); if (el) el.style.display = 'none'; }
    async function alertMsg(message, type) {
        if (typeof showAlert === 'function') { try { await showAlert(message, type); } catch (e) { /* noop */ } }
    }

    // Keys the caller owns that this application could actually take: not already
    // on it, and not of a key type it already holds. Rendered with the page by the
    // controller, so the picker needs no round trip to open.
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
            // Named by the key's own name, with the consumer key after it. Keys
            // created before the name column existed have none, and a blank one
            // would render a leading separator — so those fall back to the
            // consumer key alone rather than an empty label.
            var label = key.name ? key.name + ' · ' + key.consumerKey : key.consumerKey;
            // Marked in the text, not with a badge: an <option> renders no markup.
            // This is the one control where a sandbox key gets attached to an
            // application, so it is the one place the distinction has to be legible
            // before the choice rather than after it.
            if (key.keyType === 'SANDBOX') label += ' · Sandbox';
            // No "(on X)" suffix: a key on another application is not offered here at
            // all, so every option in this list is attached to nothing.
            opt.textContent = label;
            sel.appendChild(opt);
        });

        document.getElementById('btn-submit-associate-oauth2-key').disabled = true;
        if (!keys.length) {
            /*
             * Three reasons the list can be empty, and they are not the same
             * answer. A key already on this application means the slot is taken.
             * Keys parked on other applications means they exist but are spoken
             * for. Only with neither does the caller genuinely own nothing to
             * pick. Collapsing these sends someone who knows they own keys looking
             * for a bug that is not there.
             *
             * The second wording is the server's own: it mirrors
             * constants.ERROR_MESSAGE.OAUTH2_KEY_ALREADY_ASSOCIATED, so the
             * refusal reads the same whether the picker pre-empts it or the
             * request reaches the 409.
             */
            var reason = 'No OAuth2 key is available to associate.';
            if (hasAssociatedKeys) {
                reason = 'A key is already associated with this application. Remove it to associate another.';
            } else if (keysElsewhere) {
                reason = 'Your OAuth2 keys are associated with other applications. '
                    + 'Remove one of those associations, or move the key from the OAuth2 Keys page.';
            }
            alertMsg(reason, 'error');
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
