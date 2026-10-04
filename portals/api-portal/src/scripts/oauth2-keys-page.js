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

// Global "OAuth2 Keys" page. The list itself is rendered server-side; this script
// only drives the Add-key flow and the copy buttons.
//
// The form is built from GET /key-managers/metadata rather than hardcoded. Each key
// manager declares the properties it accepts, and that declaration is the authority
// on what POST /oauth2-keys will take — a property absent from it is rejected, and
// one marked required must be supplied. A form written here instead would silently
// drift from whichever driver the operator configured.
(function () {
    var root = document.getElementById('oauth2-keys-root');
    if (!root) return;

    var _metadata = null;      // cached /key-managers/metadata list
    var _submitting = false;
    var _copyTimer = null;

    /* ── helpers ──────────────────────────────────────────────── */

    function mutationHeaders() {
        return { 'Content-Type': 'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() };
    }
    function show(id) { var el = document.getElementById(id); if (el) el.style.display = 'flex'; }
    function hide(id) { var el = document.getElementById(id); if (el) el.style.display = 'none'; }
    async function alertMsg(message, type) {
        if (typeof showAlert === 'function') { try { await showAlert(message, type); } catch (e) { /* noop */ } }
    }

    // Built with createElement rather than innerHTML: label and description text comes
    // from the key manager's own metadata, and interpolating that into markup would
    // make a driver's description field an injection point on this page.
    /*
     * The two ways a key comes into being, and what the developer has to know
     * before choosing. `keyCreation` is declared by the driver, so a new
     * provision-shaped key manager is classified correctly without touching this.
     *
     * The wording carries the prerequisite, because that is the part that cannot
     * be discovered from the form: "provide" needs the application to exist
     * already, and nothing later in the flow will say so.
     */
    var MODES = {
        register: {
            group: 'Portal creates the application',
            submit: 'Generate key',
            hint: function (km) {
                return (km.displayName || km.id) + ' will create the application and show you its '
                    + 'client secret once. The secret is not stored.';
            },
        },
        provide: {
            group: 'Use an application you already created',
            submit: 'Add existing key',
            hint: function (km) {
                return 'Create the application in ' + (km.displayName || km.id)
                    + ' first, then paste its client id below. No secret is issued here.';
            },
        },
    };

    function modeOf(km) {
        return MODES[km && km.keyCreation] || MODES.register;
    }

    /* The mode line always shows; an operator's own description follows it. */
    function describeKeyManager(km) {
        var host = document.getElementById('ok-km-hint');
        if (!host || !km) return;
        host.textContent = '';
        /*
         * Sandbox is called out first and in its own voice, because it is the one
         * property of the choice that changes what the resulting credential is for
         * rather than how it is obtained. Production is left unsaid: it is the
         * default, and labelling it would put a line on every selection and make
         * neither stand out — the same reason the listings badge only sandbox.
         */
        if (km.keyType === 'SANDBOX') {
            host.appendChild(el('span', 'ok-km-sandbox', 'Keys created here are sandbox keys.'));
        }
        host.appendChild(el('span', 'ok-km-mode', modeOf(km).hint(km)));
        if (km.description) host.appendChild(el('span', 'ok-km-note', km.description));
    }

    function el(tag, className, text) {
        var node = document.createElement(tag);
        if (className) node.className = className;
        if (text != null) node.textContent = text;
        return node;
    }

    /* ── the dynamic property form ────────────────────────────── */

    function selectedKeyManager() {
        var sel = document.getElementById('ok-km-select');
        if (!sel || !_metadata) return null;
        for (var i = 0; i < _metadata.length; i++) {
            if (_metadata[i].id === sel.value) return _metadata[i];
        }
        return null;
    }

    function renderFields(values) {
        var host = document.getElementById('ok-fields');
        var km = selectedKeyManager();
        if (!host) return;
        host.textContent = '';
        // Cleared here rather than by each caller: this is the one place the fields
        // are rebuilt, so a read-only state can never outlive the controls it froze.
        host.classList.remove('ok-fields--readonly');
        if (!km) return;
        // The hint under the selector is not written here: describeKeyManager owns
        // it, and this function runs after it on every change. Two writers meant
        // the later one silently won.
        var prefill = values || {};

        (km.properties || []).forEach(function (p) {
            /*
             * A checkbox is labelled beside itself, not under a caption. Every other
             * type here is a caption above a full-width control, and putting a
             * checkbox through that layout leaves a bold heading with a lone box
             * under it, reading as an empty field rather than as an option. So this
             * type builds its own group and returns.
             *
             * No required marker: a checkbox always submits a value, so "required"
             * has nothing to ask for, and an asterisk would imply the developer had
             * to tick it.
             */
            if (p.type === 'boolean') {
                var bGroup = el('div', 'ok-form-group');
                var bRow = el('label', 'ok-check-row');
                bRow.setAttribute('for', 'ok-prop-' + p.name);
                var box = document.createElement('input');
                box.type = 'checkbox';
                box.className = 'ok-form-check';
                box.id = 'ok-prop-' + p.name;
                box.setAttribute('data-prop-name', p.name);
                box.setAttribute('data-prop-type', 'boolean');
                var cur = (values || {})[p.name];
                box.checked = cur === true || cur === 'true';
                bRow.appendChild(box);
                bRow.appendChild(el('span', 'ok-check-text', p.label || p.name));
                bGroup.appendChild(bRow);
                if (p.description) bGroup.appendChild(el('p', 'ok-form-hint', p.description));
                host.appendChild(bGroup);
                return;
            }

            var group = el('div', 'ok-form-group');
            var label = el('label', null, p.label || p.name);
            label.setAttribute('for', 'ok-prop-' + p.name);
            if (p.required) {
                label.appendChild(el('span', 'ok-required', ' *'));
            } else if (p.requiredWhen) {
                // Rendered up front but hidden, and revealed the moment the deciding
                // property makes it necessary — so the asterisk appearing is itself the
                // explanation of why the field is suddenly needed.
                var cond = el('span', 'ok-required', ' *');
                cond.hidden = true;
                cond.setAttribute('data-required-for', p.name);
                label.appendChild(cond);
            }
            group.appendChild(label);

            // The key's current value for this property, when updating. PUT replaces
            // the metadata wholesale, so anything not shown back here would be
            // dropped by an edit that never intended to touch it.
            var current = prefill[p.name];
            var field;
            if (p.type === 'select') {
                field = el('select', 'ok-form-input');
                // A non-required select needs an empty choice, or the first option is
                // submitted by default and the caller cannot express "leave it unset".
                // The value must be set explicitly: an <option> with no value attribute
                // reports its TEXT as its value, so an untouched dropdown would submit
                // the literal "— None —" to the key manager.
                if (!p.required) {
                    var none = el('option', null, '— None —');
                    none.value = '';
                    field.appendChild(none);
                }
                (p.options || []).forEach(function (o) {
                    var opt = el('option', null, o.label || o.value);
                    opt.value = o.value;
                    field.appendChild(opt);
                });
                if (current != null) field.value = String(current);
            } else if (p.type === 'multiselect') {
                var held = Array.isArray(current) ? current : (current ? [current] : []);
                field = el('div', 'ok-checks');
                (p.options || []).forEach(function (o) {
                    var wrap = el('label', 'ok-check');
                    var box = document.createElement('input');
                    box.type = 'checkbox';
                    box.value = o.value;
                    box.checked = held.indexOf(o.value) !== -1;
                    box.setAttribute('data-prop', p.name);
                    wrap.appendChild(box);
                    wrap.appendChild(el('span', null, o.label || o.value));
                    field.appendChild(wrap);
                });
            } else if (p.type === 'string_list') {
                field = el('textarea', 'ok-form-input ok-form-textarea');
                field.rows = 2;
                field.placeholder = 'One per line';
                if (Array.isArray(current)) field.value = current.join('\n');
                else if (current != null) field.value = String(current);
            } else if (p.type === 'number') {
                field = el('input', 'ok-form-input');
                field.type = 'number';
                if (current != null) field.value = String(current);
            } else {
                field = el('input', 'ok-form-input');
                field.type = p.type === 'uri' ? 'url' : 'text';
                if (current != null) field.value = String(current);
            }
            field.id = 'ok-prop-' + p.name;
            field.setAttribute('data-prop-name', p.name);
            field.setAttribute('data-prop-type', p.type || 'string');
            group.appendChild(field);

            if (p.description) group.appendChild(el('p', 'ok-form-hint', p.description));
            host.appendChild(group);
        });
        refreshConditionalMarks();
    }

    // One property's value, or undefined when the field was left untouched. An empty
    // string or empty array is deliberately undefined rather than a value: sending one
    // asserts "no callback URLs", which some key managers reject where absence is fine.
    function readValue(p, host) {
        if (p.type === 'multiselect') {
            var picked = [];
            host.querySelectorAll('input[type=checkbox][data-prop="' + CSS.escape(p.name) + '"]:checked')
                .forEach(function (b) { picked.push(b.value); });
            return picked.length ? picked : undefined;
        }
        if (p.type === 'string_list') {
            var raw = (document.getElementById('ok-prop-' + p.name) || {}).value || '';
            var lines = raw.split('\n').map(function (v) { return v.trim(); }).filter(Boolean);
            return lines.length ? lines : undefined;
        }
        var node = document.getElementById('ok-prop-' + p.name);
        if (p.type === 'boolean') {
            // Always a value, never undefined. A checkbox has no "untouched" state to
            // represent, and update is a wholesale PUT — omitting an unticked box would
            // read at the key manager as "leave as is" rather than the "off" the
            // developer just expressed.
            return !!(node && node.checked);
        }
        var v = ((node || {}).value || '').trim();
        if (v === '') return undefined;
        if (p.type === 'number') {
            var n = Number(v);
            return Number.isFinite(n) ? n : undefined;
        }
        return v;
    }

    // Whether a `requiredWhen` rule is currently satisfied. The deciding value is an
    // array for a multiselect and a string for a select, so both shapes are matched.
    function conditionMet(rule, values) {
        if (!rule || !rule.property || !Array.isArray(rule.anyOf)) return false;
        var deciding = values[rule.property];
        if (deciding === undefined) return false;
        var held = Array.isArray(deciding) ? deciding : [deciding];
        return rule.anyOf.some(function (want) { return held.indexOf(want) !== -1; });
    }

    // Collect the form into the `properties` object POST /oauth2-keys expects.
    // Read in full first, then validated: a conditional rule consults another field's
    // value, which a single pass would not reliably have yet.
    function collectProperties() {
        var host = document.getElementById('ok-fields');
        var km = selectedKeyManager();
        var props = {};
        var missing = [];
        if (!host || !km) return { props: props, missing: missing };

        var descriptors = km.properties || [];
        var values = {};
        descriptors.forEach(function (p) { values[p.name] = readValue(p, host); });

        descriptors.forEach(function (p) {
            var value = values[p.name];
            if (value === undefined) {
                if (p.required || conditionMet(p.requiredWhen, values)) missing.push(p.label || p.name);
                return;
            }
            props[p.name] = value;
        });
        return { props: props, missing: missing };
    }

    // Show or hide each conditional asterisk against the form's current state.
    function refreshConditionalMarks() {
        var host = document.getElementById('ok-fields');
        var km = selectedKeyManager();
        if (!host || !km) return;
        var descriptors = km.properties || [];
        var values = {};
        descriptors.forEach(function (p) { values[p.name] = readValue(p, host); });
        descriptors.forEach(function (p) {
            if (!p.requiredWhen) return;
            var mark = host.querySelector('[data-required-for="' + CSS.escape(p.name) + '"]');
            if (mark) mark.hidden = !conditionMet(p.requiredWhen, values);
        });
    }

    /* ── open / close ─────────────────────────────────────────── */

    var _editKeyId = null;   // set while the modal is in update mode

    /*
     * The modal is shared, so switching modes has to switch every part of it that
     * differs — title, which submit button is live, and whether the key manager can
     * still be chosen. A key cannot move between key managers, so in update mode the
     * selector is fixed to the one that issued it.
     */
    /*
     * Three modes, one dialog: 'add', 'edit', and 'view'.
     *
     * View exists because selecting a row used to open the update form, which put
     * a developer in an editing state just for looking — and on a form whose save
     * replaces the client metadata wholesale. Looking is now the default and
     * changing is a deliberate choice, from the row menu or from this dialog's own
     * Update button.
     */
    /*
     * `fallbackKmName` is used only when `km` is null — a view of a key whose key
     * manager is disabled, where there are no descriptors but the selector must
     * still say which key manager issued it.
     */
    function setMode(mode, km, fallbackKmName) {
        var editing = mode === 'edit';
        var viewing = mode === 'view';
        _editKeyId = (editing || viewing) ? _editKeyId : null;
        document.getElementById('ok-add-title').textContent =
            editing ? 'Update OAuth2 key' : (viewing ? 'OAuth2 key' : 'Add OAuth2 key');
        document.getElementById('ok-add-submit').hidden = editing || viewing;
        document.getElementById('ok-edit-submit').hidden = !editing;
        // Named for what it does in each mode: nothing has been typed in view mode,
        // so "Cancel" would be offering to undo something that never happened.
        document.getElementById('ok-add-cancel').textContent = viewing ? 'Close' : 'Cancel';

        var sel = document.getElementById('ok-km-select');
        sel.disabled = editing || viewing;
        if (viewing && !km && fallbackKmName) {
            // No metadata entry to read a display name from, so the name travels
            // with the key itself. The selector is disabled either way.
            sel.textContent = '';
            var fb = el('option', null, fallbackKmName);
            fb.value = '';
            sel.appendChild(fb);
            sel.value = '';
            var hint = document.getElementById('ok-km-hint');
            if (hint) hint.textContent = '';
        } else if ((editing || viewing) && km) {
            sel.textContent = '';
            var opt = el('option', null, km.displayName || km.id);
            opt.value = km.id;
            sel.appendChild(opt);
            sel.value = km.id;
            describeKeyManager(km);
        }
    }

    /** The metadata list, fetched once and reused by both modes. */
    async function loadMetadata() {
        if (_metadata) return _metadata;
        try {
            var resp = await fetch(window.apiPortalApi.root('/key-managers/metadata'), { headers: mutationHeaders() });
            if (!resp.ok) throw new Error('HTTP ' + resp.status);
            var body = await resp.json();
            _metadata = (body && body.list) || [];
        } catch (e) {
            _metadata = null;
        }
        return _metadata;
    }

    /**
     * Open one key for update.
     *
     * The GET is the point: a key's client metadata lives at the key manager, not
     * in the portal, so the only way to show what it currently holds — and the only
     * way an update can avoid discarding fields the user never touched — is to read
     * it back first.
     */
    async function openKey(keyId, mode) {
        var meta = await loadMetadata();
        if (!meta) {
            await alertMsg('Could not load the key manager details. Refresh the page and try again.', 'error');
            return;
        }

        var key;
        try {
            var resp = await fetch(window.apiPortalApi.root('/oauth2-keys/' + encodeURIComponent(keyId)), {
                headers: mutationHeaders(),
            });
            if (!resp.ok) {
                var body = null;
                try { body = await resp.json(); } catch (e) { /* not JSON */ }
                await alertMsg((body && body.message) || 'Could not load this key.', 'error');
                return;
            }
            key = await resp.json();
        } catch (e) {
            await alertMsg('Could not reach the server. Check your connection and try again.', 'error');
            return;
        }

        var km = null;
        for (var i = 0; i < meta.length; i++) {
            if (meta[i].id === key.keyManagerId) { km = meta[i]; break; }
        }

        /*
         * No descriptors for this key's key manager. The common cause is that an
         * admin disabled it: GET /key-managers/metadata lists only enabled ones,
         * while GET /oauth2-keys/{id} deliberately still answers for a disabled one.
         *
         * That asymmetry is intentional on the server — disabling does not revoke
         * anything, so the credentials are still live, and the key has to stay
         * visible and deletable or a developer is left holding working credentials
         * they cannot see or retire. So reading falls back to the raw properties
         * rather than refusing.
         *
         * Editing is a different matter: the server answers 409 for an update
         * through a disabled key manager, so there is nothing to render a form for.
         */
        if (!km) {
            if (mode !== 'view') {
                await alertMsg(
                    'This key manager is disabled or no longer available, so this key cannot be updated. '
                    + 'It can still be viewed and deleted.', 'error');
                return;
            }
            _editKeyId = keyId;
            setMode('view', null, key.keyManagerName || key.keyManagerId);
            renderRawProperties(key.properties || {});
            show('ok-add-modal');
            return;
        }

        _editKeyId = keyId;
        setMode(mode, km);
        // Rendered from the metadata descriptors, seeded with what the key manager
        // just told us this client currently holds.
        renderFields(key.properties || {});
        if (mode === 'view') freezeFields();
        show('ok-add-modal');
    }

    /*
     * Render a key's properties with no descriptors to lay them out.
     *
     * Only reachable from the view above. Without the metadata there is no label,
     * type or ordering for any property, so each one is shown under its own wire
     * name with its value as text. Deliberately not inputs: there is nothing to
     * submit them to, and a disabled input would imply the form could be unlocked.
     *
     * The notice is the point — a plainer layout with no explanation reads as a
     * rendering fault rather than as the state it actually is.
     */
    function renderRawProperties(values) {
        var host = document.getElementById('ok-fields');
        if (!host) return;
        host.textContent = '';
        host.classList.add('ok-fields--readonly');

        host.appendChild(el('p', 'ok-fields-notice',
            'This key manager is disabled, so this key is shown read-only and without '
            + 'its field descriptions. The key itself still works; it can be deleted '
            + 'from the row menu.'));

        var names = Object.keys(values || {});
        if (!names.length) {
            host.appendChild(el('p', 'ok-form-hint', 'The key manager returned no properties for this key.'));
            return;
        }
        names.sort();
        names.forEach(function (name) {
            var group = el('div', 'ok-form-group');
            group.appendChild(el('label', null, name));
            var raw = values[name];
            var text = Array.isArray(raw) ? raw.join(', ')
                : (raw === null || raw === undefined) ? ''
                    : (typeof raw === 'object' ? JSON.stringify(raw) : String(raw));
            group.appendChild(el('p', 'ok-raw-value', text || '—'));
            host.appendChild(group);
        });
    }

    /** Open one key read-only. Selecting a row lands here. */
    function openView(keyId) { return openKey(keyId, 'view'); }

    /** Open one key for update. The row menu's Update lands here. */
    function openEdit(keyId) { return openKey(keyId, 'edit'); }

    /*
     * Make the rendered form inert.
     *
     * The same descriptors build the view and the edit form, so rather than a
     * second renderer that would drift from the first, the controls are disabled
     * after the fact. `disabled` rather than `readonly`: readonly does not apply to
     * a checkbox or a select, which would leave two of the four property types
     * quietly editable.
     */
    function freezeFields() {
        var host = document.getElementById('ok-fields');
        if (!host) return;
        host.classList.add('ok-fields--readonly');
        host.querySelectorAll('input, select, textarea').forEach(function (c) {
            c.disabled = true;
        });
    }

    async function submitEdit() {
        if (_submitting || !_editKeyId) return;
        var km = selectedKeyManager();
        if (!km) return;

        var collected = collectProperties();
        if (collected.missing.length) {
            await alertMsg('Fill in the required field(s): ' + collected.missing.join(', ') + '.', 'error');
            return;
        }

        _submitting = true;
        var btn = document.getElementById('ok-edit-submit');
        if (btn) btn.disabled = true;
        try {
            var resp = await fetch(window.apiPortalApi.root('/oauth2-keys/' + encodeURIComponent(_editKeyId)), {
                method: 'PUT',
                headers: mutationHeaders(),
                // A full replacement, which is what the endpoint does — hence the
                // form having been seeded from the current values.
                body: JSON.stringify({ properties: collected.props }),
            });
            if (!resp.ok) {
                var body = null;
                try { body = await resp.json(); } catch (e) { /* not JSON */ }
                // 409 here means this key manager's driver implements no update.
                await alertMsg((body && body.message) || 'Could not update the key.', 'error');
                return;
            }
            hide('ok-add-modal');

            /*
             * A replacement secret must be shown before this page reloads, or it is
             * gone: the portal stores no secrets, and the spec has already retired
             * the old one. So the reload waits for the panel to be dismissed.
             */
            var body2 = null;
            try { body2 = await resp.json(); } catch (e2) { /* no body is fine */ }
            if (body2 && body2.consumerSecret) {
                showCredentials(km, body2, true);
                return;
            }
            window.location.reload();
        } catch (e) {
            await alertMsg('Could not reach the server. Check your connection and try again.', 'error');
        } finally {
            _submitting = false;
            if (btn) btn.disabled = false;
        }
    }

    /* ── access token ─────────────────────────────────────────── */

    var _tokenKeyId = null;

    /*
     * Why the client cannot use this grant, or null when it can.
     *
     * Read from the client's own registration. Anything unknown — a provision-type
     * key manager with nothing to read back, an older server that omits the field —
     * is treated as usable, because the server does the same check before dialling
     * and is the one that decides. This only saves the developer from filling in a
     * form that cannot succeed.
     */
    function tokenBlockedReason(key) {
        var props = (key && key.properties) || {};
        if (props.token_endpoint_auth_method === 'none') {
            return 'This is a public client, so it has no secret. Tokens for it come from signing a '
                + 'user in, not from here.';
        }
        if (props.token_endpoint_auth_method === 'private_key_jwt') {
            return 'This client authenticates with a private key, which the portal cannot present on '
                + 'your behalf. Request its token directly from the key manager.';
        }
        var grants = props.grant_types;
        if (Array.isArray(grants) && grants.length && grants.indexOf('client_credentials') < 0) {
            return 'This client is registered for ' + grants.join(', ') + ', not the client credentials '
                + 'grant, so it cannot get a token here.';
        }
        return null;
    }

    async function openToken(row) {
        _tokenKeyId = row.getAttribute('data-key-id');
        document.getElementById('ok-token-consumer-key').textContent = row.getAttribute('data-consumer-key') || '';
        // Cleared on every open: a secret typed for one key must never be carried
        // into the next key's dialog.
        document.getElementById('ok-token-secret').value = '';
        document.getElementById('ok-token-scopes').value = '';
        document.getElementById('ok-token-resources').value = '';

        // Open first, then ask. The dialog appearing immediately is worth more than
        // it appearing already-decided, and the read is quick.
        var blocked = document.getElementById('ok-token-blocked');
        var form = document.getElementById('ok-token-form');
        var submit = document.getElementById('ok-token-submit');
        blocked.hidden = true;
        form.hidden = false;
        submit.hidden = false;
        show('ok-token-modal');
        document.getElementById('ok-token-secret').focus();

        var key = null;
        try {
            var resp = await fetch(
                window.apiPortalApi.root('/oauth2-keys/' + encodeURIComponent(_tokenKeyId)),
                { headers: mutationHeaders() }
            );
            if (resp.ok) key = await resp.json();
        } catch (e) { /* treated as unknown — the server checks too */ }

        var reason = tokenBlockedReason(key);
        if (reason) {
            blocked.textContent = reason;
            blocked.hidden = false;
            form.hidden = true;
            submit.hidden = true;
        }
    }

    async function submitToken() {
        if (_submitting || !_tokenKeyId) return;
        var secret = document.getElementById('ok-token-secret').value;
        if (!secret) {
            await alertMsg('Enter the consumer secret for this key.', 'error');
            return;
        }
        var scopes = document.getElementById('ok-token-scopes').value.split(/\s+/).filter(Boolean);
        // Space-separated like scopes, but sent as repeated `resource` parameters
        // rather than one joined string — RFC 8707 defines it that way, and a
        // joined list would reach the key manager as a single malformed URI.
        var resources = document.getElementById('ok-token-resources').value.split(/\s+/).filter(Boolean);

        _submitting = true;
        var btn = document.getElementById('ok-token-submit');
        if (btn) btn.disabled = true;
        try {
            var body = { consumerSecret: secret };
            if (scopes.length) body.scopes = scopes;
            if (resources.length) body.resources = resources;
            var resp = await fetch(
                window.apiPortalApi.root('/oauth2-keys/' + encodeURIComponent(_tokenKeyId) + '/generate-token'),
                { method: 'POST', headers: mutationHeaders(), body: JSON.stringify(body) }
            );
            var data = null;
            try { data = await resp.json(); } catch (e) { /* not JSON */ }
            if (!resp.ok) {
                await alertMsg((data && data.message) || 'Could not get a token.', 'error');
                return;
            }
            // Dropped as soon as it has been sent: the field is the only copy the
            // page ever held, and there is no reason to keep it after the request.
            document.getElementById('ok-token-secret').value = '';
            hide('ok-token-modal');
            showToken(data || {});
        } catch (e) {
            await alertMsg('Could not reach the server. Check your connection and try again.', 'error');
        } finally {
            _submitting = false;
            if (btn) btn.disabled = false;
        }
    }

    function showToken(token) {
        document.getElementById('ok-token-value').textContent = token.accessToken || '';
        var parts = [];
        if (token.tokenType) parts.push(token.tokenType);
        // The key manager's answer, not the request's: it may grant less than asked.
        if (token.validityTime) parts.push('valid for ' + token.validityTime + 's');
        if (token.tokenScopes && token.tokenScopes.length) {
            parts.push('scopes: ' + token.tokenScopes.join(' '));
        } else {
            parts.push('no scopes granted');
        }
        document.getElementById('ok-token-summary').textContent = parts.join(' · ');
        show('ok-token-result-modal');
    }

    /* ── delete ───────────────────────────────────────────────── */

    var _deleteKeyId = null;

    function openDelete(row) {
        _deleteKeyId = row.getAttribute('data-key-id');
        document.getElementById('ok-delete-name').textContent = row.getAttribute('data-consumer-key') || '';
        show('ok-delete-modal');
    }

    async function confirmDelete() {
        if (!_deleteKeyId || _submitting) return;
        _submitting = true;
        var btn = document.getElementById('ok-delete-confirm');
        if (btn) btn.disabled = true;
        try {
            var resp = await fetch(window.apiPortalApi.root('/oauth2-keys/' + encodeURIComponent(_deleteKeyId)), {
                method: 'DELETE',
                headers: mutationHeaders(),
            });
            if (!resp.ok) {
                var body = null;
                try { body = await resp.json(); } catch (e) { /* not JSON */ }
                // 409 here means this key manager's driver implements no delete.
                await alertMsg((body && body.message) || 'Could not delete the key.', 'error');
                return;
            }
            hide('ok-delete-modal');
            window.location.reload();
        } catch (e) {
            await alertMsg('Could not reach the server. Check your connection and try again.', 'error');
        } finally {
            _submitting = false;
            if (btn) btn.disabled = false;
        }
    }

    async function openAdd() {
        if (!_metadata) {
            try {
                var resp = await fetch(window.apiPortalApi.root('/key-managers/metadata'), { headers: mutationHeaders() });
                if (!resp.ok) throw new Error('HTTP ' + resp.status);
                var body = await resp.json();
                _metadata = (body && body.list) || [];
            } catch (e) {
                _metadata = null;
                await alertMsg('Could not load the available key managers. Refresh the page and try again.', 'error');
                return;
            }
        }
        if (!_metadata.length) {
            await alertMsg('No key manager is configured for OAuth2 key generation.', 'error');
            return;
        }

        // Reset the shared modal before filling it. Without this the add path
        // inherits whatever the last opened row left behind — edit title, the
        // wrong submit button, and a key manager selector locked to that key's
        // key manager, because only openEdit was setting the mode.
        setMode('add');

        var sel = document.getElementById('ok-km-select');
        sel.textContent = '';
        // Grouped, because the split is the one thing a developer has to get right
        // and the option label alone cannot carry it. Groups are emitted only when
        // non-empty, so a portal with one kind of key manager sees a plain list
        // rather than a heading over everything.
        ['register', 'provide'].forEach(function (mode) {
            var members = _metadata.filter(function (km) { return modeOf(km) === MODES[mode]; });
            if (!members.length) return;
            var holder = sel;
            if (_metadata.length > members.length) {
                holder = el('optgroup');
                holder.label = MODES[mode].group;
                sel.appendChild(holder);
            }
            members.forEach(function (km) {
                // Marked in the option text as well as in the hint below, because
                // the environment is part of what is being chosen here — seeing it
                // only after selecting means discovering it after the decision.
                // An <option> renders no markup, so this is text, not a badge.
                var label = km.displayName || km.id;
                if (km.keyType === 'SANDBOX') label += ' · Sandbox';
                var opt = el('option', null, label);
                opt.value = km.id;
                holder.appendChild(opt);
            });
        });
        sel.selectedIndex = 0;
        syncKeyManagerChoice();
        renderFields();
        show('ok-add-modal');
    }

    /* Everything that depends on WHICH key manager is selected, in one place. */
    function syncKeyManagerChoice() {
        var km = selectedKeyManager();
        if (!km) return;
        describeKeyManager(km);
        var submit = document.getElementById('ok-add-submit');
        if (submit && !submit.hidden) submit.textContent = modeOf(km).submit;
    }

    /* ── submit ───────────────────────────────────────────────── */

    async function submitAdd() {
        if (_submitting) return;
        var km = selectedKeyManager();
        if (!km) return;

        var collected = collectProperties();
        if (collected.missing.length) {
            await alertMsg('Fill in the required field(s): ' + collected.missing.join(', ') + '.', 'error');
            return;
        }

        _submitting = true;
        var btn = document.getElementById('ok-add-submit');
        if (btn) btn.disabled = true;
        try {
            var resp = await fetch(window.apiPortalApi.root('/oauth2-keys'), {
                method: 'POST',
                headers: mutationHeaders(),
                body: JSON.stringify({ keyManagerId: km.id, properties: collected.props }),
            });
            var body = null;
            try { body = await resp.json(); } catch (e) { /* a 204 or a non-JSON error page */ }
            if (!resp.ok) {
                // The portal returns a specific message for a caller-caused failure and a
                // generic one for anything server-side; either way it is the right text to
                // show, so it is surfaced as-is rather than replaced with our own guess.
                await alertMsg((body && body.message) || 'Could not create the OAuth2 key.', 'error');
                return;
            }
            hide('ok-add-modal');
            showCredentials(km, body || {});
        } catch (e) {
            await alertMsg('Could not reach the server. Check your connection and try again.', 'error');
        } finally {
            _submitting = false;
            if (btn) btn.disabled = false;
        }
    }

    /**
     * The one place a consumer secret is ever shown, used for both events that
     * produce one: a key created, and an update the key manager answered with a
     * replacement secret.
     *
     * @param {object} km
     * @param {object} key
     * @param {boolean} [rotated] true when this is a replacement rather than a new key
     */
    function showCredentials(km, key, rotated) {
        var title = document.getElementById('ok-secret-title');
        var subtitle = document.getElementById('ok-secret-subtitle');
        if (title) title.textContent = rotated ? 'New consumer secret issued' : 'OAuth2 key created';
        if (subtitle) {
            subtitle.textContent = rotated
                ? 'The update replaced this key\'s secret. The previous one no longer works.'
                : '';
            if (!rotated) {
                subtitle.appendChild(document.createTextNode('Registered on '));
                var strong = el('strong');
                strong.id = 'ok-secret-km';
                subtitle.appendChild(strong);
                subtitle.appendChild(document.createTextNode('.'));
            }
        }
        var kmNode = document.getElementById('ok-secret-km');
        if (kmNode) kmNode.textContent = km.displayName || km.id;
        document.getElementById('ok-secret-consumer-key').textContent = key.consumerKey || '';

        // A public client (token_endpoint_auth_method "none") gets no secret. Showing an
        // empty secret box plus a "copy it now" warning would read as a failure, so the
        // whole secret block is swapped for a note instead.
        var hasSecret = !!key.consumerSecret;
        document.getElementById('ok-secret-consumer-secret').textContent = key.consumerSecret || '';
        ['ok-secret-label', 'ok-secret-field', 'ok-secret-warning'].forEach(function (id) {
            var node = document.getElementById(id);
            if (node) node.hidden = !hasSecret;
        });
        var note = document.getElementById('ok-public-note');
        if (note) note.hidden = hasSecret;

        show('ok-secret-modal');
    }

    /* ── application association ──────────────────────────────── */

    var _appRow = null;   // the row whose association is being edited

    function applications() {
        var node = document.getElementById('ok-applications-data');
        if (!node) return [];
        try {
            return JSON.parse(node.textContent || '[]');
        } catch (e) {
            return [];
        }
    }

    function openAppModal(btn) {
        var tr = btn.closest ? btn.closest('tr') : null;
        if (!tr) return;
        _appRow = tr;

        var apps = applications();
        var sel = document.getElementById('ok-app-select');
        sel.textContent = '';
        // "None" is the first option and a real choice — picking it dissociates,
        // which is why this modal has no separate remove button.
        var none = el('option', null, '— None —');
        none.value = '';
        sel.appendChild(none);
        apps.forEach(function (app) {
            var opt = el('option', null, app.displayName || app.id);
            opt.value = app.id;
            sel.appendChild(opt);
        });
        sel.value = tr.getAttribute('data-application-id') || '';

        if (!apps.length) {
            alertMsg('You have no applications yet. Create one first, then associate this key with it.', 'error');
            return;
        }
        show('ok-app-modal');
    }

    async function saveAssociation() {
        if (!_appRow || _submitting) return;
        var keyId = _appRow.getAttribute('data-key-id');
        var current = _appRow.getAttribute('data-application-id') || '';
        var chosen = document.getElementById('ok-app-select').value;
        if (chosen === current) { hide('ok-app-modal'); return; }

        _submitting = true;
        var btn = document.getElementById('ok-app-save');
        if (btn) btn.disabled = true;
        try {
            // Two endpoints, one control: choosing an application associates,
            // choosing "None" dissociates.
            var path = '/oauth2-keys/' + encodeURIComponent(keyId) + (chosen ? '/associate' : '/dissociate');
            var resp = await fetch(window.apiPortalApi.root(path), {
                method: 'POST',
                headers: mutationHeaders(),
                body: chosen ? JSON.stringify({ applicationId: chosen }) : undefined,
            });
            if (!resp.ok) {
                var body = null;
                try { body = await resp.json(); } catch (e) { /* not JSON */ }
                await alertMsg((body && body.message) || 'Could not update the association.', 'error');
                return;
            }
            hide('ok-app-modal');
            // The row's application cell and button label are both server-rendered,
            // so a reload is the honest way to show the new state.
            window.location.reload();
        } catch (e) {
            await alertMsg('Could not reach the server. Check your connection and try again.', 'error');
        } finally {
            _submitting = false;
            if (btn) btn.disabled = false;
        }
    }

    /* ── copy ─────────────────────────────────────────────────── */

    function flashCopied(btn) {
        btn.classList.add('is-copied');
        clearTimeout(_copyTimer);
        _copyTimer = setTimeout(function () { btn.classList.remove('is-copied'); }, 1400);
    }

    document.addEventListener('click', function (e) {
        var btn = e.target.closest ? e.target.closest('.ok-copy') : null;
        if (!btn) return;
        var targetId = btn.getAttribute('data-copy-target');
        var text = targetId
            ? ((document.getElementById(targetId) || {}).textContent || '')
            : (btn.getAttribute('data-copy') || '');
        if (!text) return;
        // clipboard.writeText needs a secure context; over plain HTTP it rejects, and a
        // silent no-op would look like the button is broken.
        if (!navigator.clipboard) { alertMsg('Copying needs a secure (https) connection.', 'error'); return; }
        navigator.clipboard.writeText(text).then(function () { flashCopied(btn); }, function () {
            alertMsg('Could not copy to the clipboard.', 'error');
        });
    });

    /* ── wiring ───────────────────────────────────────────────── */

    ['ok-add-btn', 'ok-add-btn-empty'].forEach(function (id) {
        var b = document.getElementById(id);
        if (b) b.addEventListener('click', openAdd);
    });
    ['ok-add-close', 'ok-add-cancel'].forEach(function (id) {
        var b = document.getElementById(id);
        if (b) b.addEventListener('click', function () { hide('ok-add-modal'); });
    });
    /* ── relative timestamps ──────────────────────────────────
     *
     * The Created column shows "3 days ago" and keeps the exact timestamp on the
     * element's title, so the precise value is one hover away without spending a
     * column on it.
     *
     * Computed here rather than server-side for two reasons: the reader's clock
     * and time zone are the ones that matter, and a relative string rendered into
     * HTML starts going stale immediately — a tab left open would still claim
     * "just now" an hour later. The markup ships an absolute date, so with no
     * script running the column is still correct, just less friendly.
     */
    var RELATIVE_UNITS = [
        ['year', 365 * 24 * 60 * 60],
        ['month', 30 * 24 * 60 * 60],
        ['day', 24 * 60 * 60],
        ['hour', 60 * 60],
        ['minute', 60],
    ];

    function relativeTime(date) {
        var seconds = Math.round((date.getTime() - Date.now()) / 1000);
        var magnitude = Math.abs(seconds);
        // Under a minute is "just now" rather than "in 0 seconds" / "41 seconds
        // ago" — the precision is noise at that range, and the sign flips around
        // zero if the two clocks disagree by a second.
        if (magnitude < 60) return 'just now';
        for (var i = 0; i < RELATIVE_UNITS.length; i++) {
            var unit = RELATIVE_UNITS[i][0];
            var size = RELATIVE_UNITS[i][1];
            if (magnitude >= size) {
                var value = Math.round(seconds / size);
                // Intl handles pluralisation and wording per locale, including
                // "yesterday" for -1 day, which a hand-rolled table gets wrong.
                try {
                    return new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' }).format(value, unit);
                } catch (e) {
                    return Math.abs(value) + ' ' + unit + (Math.abs(value) === 1 ? '' : 's')
                        + (value < 0 ? ' ago' : ' from now');
                }
            }
        }
        return 'just now';
    }

    function refreshTimestamps() {
        var nodes = document.querySelectorAll('.ok-created-at[datetime]');
        Array.prototype.forEach.call(nodes, function (node) {
            var when = new Date(node.getAttribute('datetime'));
            // An unparseable value leaves the server-rendered text alone rather
            // than replacing a real date with "Invalid Date".
            if (isNaN(when.getTime())) return;
            node.textContent = relativeTime(when);
        });
    }

    refreshTimestamps();
    // A minute is the finest granularity shown, so that is how often it can change.
    // Skipped while the tab is hidden; the visibility handler catches up on return,
    // so a backgrounded page is not re-rendering a column nobody is looking at.
    setInterval(function () {
        if (!document.hidden) refreshTimestamps();
    }, 60000);
    document.addEventListener('visibilitychange', function () {
        if (!document.hidden) refreshTimestamps();
    });

    /* ── row overflow menu ────────────────────────────────────
     *
     * The panel is `position: fixed`, so it is placed here rather than by CSS:
     * .ok-tablewrap scrolls on the x axis, and a box that scrolls on one axis
     * clips on the other, so a dropdown anchored inside the cell would be cut
     * off at the table's edge. Fixed takes it out of that box entirely — at the
     * cost of having to set the coordinates, and of closing on scroll, since
     * fixed coordinates do not follow the row.
     */
    function closeRowMenus() {
        document.querySelectorAll('.ok-dropdown').forEach(function (d) { d.style.display = 'none'; });
        document.querySelectorAll('.ok-menu-trigger[aria-expanded="true"]').forEach(function (t) {
            t.setAttribute('aria-expanded', 'false');
        });
    }

    function openRowMenu(trigger) {
        var dd = trigger.parentElement.querySelector('.ok-dropdown');
        if (!dd) return;
        var wasOpen = dd.style.display === 'flex';
        closeRowMenus();
        if (wasOpen) return;

        // Measured while shown but off-screen: a display:none element has no size,
        // so the flip-up decision below would be made against a height of zero.
        dd.style.display = 'flex';
        dd.style.visibility = 'hidden';
        var r = trigger.getBoundingClientRect();
        var h = dd.offsetHeight;
        var w = dd.offsetWidth;
        var below = window.innerHeight - r.bottom;
        // Right-aligned to the trigger, and opening upward when the row is near
        // the bottom of the viewport rather than running off it.
        dd.style.top = (below < h + 8 ? Math.max(8, r.top - h - 4) : r.bottom + 4) + 'px';
        dd.style.left = Math.max(8, Math.min(r.right - w, window.innerWidth - w - 8)) + 'px';
        dd.style.visibility = '';
        trigger.setAttribute('aria-expanded', 'true');
    }

    // Fixed coordinates do not track the element they were measured from, so any
    // scroll — the page or the table's own horizontal scroller — closes the menu
    // rather than leaving it floating beside nothing.
    window.addEventListener('scroll', closeRowMenus, true);
    window.addEventListener('resize', closeRowMenus);
    document.addEventListener('keydown', function (e) { if (e.key === 'Escape') closeRowMenus(); });

    // Delegated: the rows are server-rendered, so one listener covers them all.
    document.addEventListener('click', function (e) {
        if (!e.target.closest) return;

        var menuBtn = e.target.closest('.ok-row-menu');
        if (menuBtn) { openRowMenu(menuBtn); return; }
        // Any other click closes an open menu. This runs before the branches
        // below so that choosing an item dismisses the menu it was chosen from.
        if (!e.target.closest('.ok-dropdown')) closeRowMenus();

        var appBtn = e.target.closest('.ok-row-app');
        if (appBtn) { openAppModal(appBtn); return; }

        var tokenBtn = e.target.closest('.ok-row-token');
        if (tokenBtn) {
            var tokenRow = tokenBtn.closest('tr');
            if (tokenRow) openToken(tokenRow);
            return;
        }

        var editBtn = e.target.closest('.ok-row-edit');
        if (editBtn) {
            closeRowMenus();
            var editRow = editBtn.closest('tr');
            if (editRow) openEdit(editRow.getAttribute('data-key-id'));
            return;
        }

        var delBtn = e.target.closest('.ok-row-delete');
        if (delBtn) {
            closeRowMenus();
            var delRow = delBtn.closest('tr');
            if (delRow) openDelete(delRow);
            return;
        }

        // Selecting the row opens the key read-only. It used to open the update
        // form, which put a developer into an editing state just for looking — on a
        // form whose save replaces the client metadata wholesale. Updating is now
        // reached deliberately, from the row menu or from the view's own button.
        //
        // A click inside the actions cell is excluded here rather than by
        // stopPropagation on the cell: this listener is on `document`, so stopping
        // the event at the cell would prevent it reaching the button branches above
        // and kill every row action.
        // `.ok-copy` sits in the consumer-key cell rather than in `.ok-actions`, so
        // it needs excluding too — otherwise copying a key also opened the view,
        // firing a GET and dropping a modal over the page the user was reading.
        if (e.target.closest('.ok-actions') || e.target.closest('.ok-copy')) return;
        var row = e.target.closest('tr.ok-row');
        if (row) openView(row.getAttribute('data-key-id'));
    });

    // Keyboard parity: the row is focusable and announced as a button, so it has to
    // respond to Enter and Space like one.
    document.addEventListener('keydown', function (e) {
        if (e.key !== 'Enter' && e.key !== ' ') return;
        if (!e.target.classList || !e.target.classList.contains('ok-row')) return;
        e.preventDefault();
        openView(e.target.getAttribute('data-key-id'));
    });
    ['ok-app-close', 'ok-app-cancel'].forEach(function (id) {
        var b = document.getElementById(id);
        if (b) b.addEventListener('click', function () { hide('ok-app-modal'); });
    });
    var appSave = document.getElementById('ok-app-save');
    if (appSave) appSave.addEventListener('click', saveAssociation);

    var kmSelect = document.getElementById('ok-km-select');
    if (kmSelect) {
        kmSelect.addEventListener('change', function () {
            syncKeyManagerChoice();
            renderFields();
        });
    }
    // Delegated, because the fields themselves are replaced whenever the key manager
    // changes — a listener bound to an input would not survive that.
    var fieldHost = document.getElementById('ok-fields');
    if (fieldHost) {
        fieldHost.addEventListener('change', refreshConditionalMarks);
        fieldHost.addEventListener('input', refreshConditionalMarks);
    }
    var submit = document.getElementById('ok-add-submit');
    if (submit) submit.addEventListener('click', submitAdd);
    var editSubmit = document.getElementById('ok-edit-submit');
    if (editSubmit) editSubmit.addEventListener('click', submitEdit);

    ['ok-token-close', 'ok-token-cancel'].forEach(function (id) {
        var b = document.getElementById(id);
        if (b) b.addEventListener('click', function () { hide('ok-token-modal'); });
    });
    var tokenSubmit = document.getElementById('ok-token-submit');
    if (tokenSubmit) tokenSubmit.addEventListener('click', submitToken);
    // The token is not stored anywhere, so closing this simply ends it — no reload.
    ['ok-token-result-close', 'ok-token-done'].forEach(function (id) {
        var b = document.getElementById(id);
        if (b) {
            b.addEventListener('click', function () {
                document.getElementById('ok-token-value').textContent = '';
                hide('ok-token-result-modal');
            });
        }
    });

    var delCancel = document.getElementById('ok-delete-cancel');
    if (delCancel) delCancel.addEventListener('click', function () { hide('ok-delete-modal'); });
    var delConfirm = document.getElementById('ok-delete-confirm');
    if (delConfirm) delConfirm.addEventListener('click', confirmDelete);

    // Reload on dismiss, because the list is server-rendered: a created key is not in
    // it yet, and an updated one may show a stale name. This is also the only exit
    // from the panel, so a shown secret is never skipped past.
    ['ok-secret-close', 'ok-secret-done'].forEach(function (id) {
        var b = document.getElementById(id);
        if (b) b.addEventListener('click', function () { window.location.reload(); });
    });
})();
