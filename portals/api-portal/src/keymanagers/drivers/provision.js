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

'use strict';

/*
 * A key manager whose OAuth applications are created somewhere else.
 *
 * The developer makes the client by hand at the identity server and pastes its
 * client id here; the portal records that id and associates it with an
 * application, but never created the client and never manages it. WSO2 API
 * Manager calls the same idea "provisioning out-of-band OAuth2 clients".
 *
 * This is also what every key manager that predates Dynamic Client Registration
 * support becomes: a `key_managers` row with no `key_manager_configurations`
 * row has no registration endpoint and no credential to register with, which is
 * exactly this driver's situation. Resolving those to this driver rather than to
 * nothing is what keeps them usable for key creation.
 *
 * The four key operations divide on one question — does the portal own this
 * client?
 *
 *   createKey  records the pasted id. No call goes anywhere.
 *   getKey     answers from what was recorded. There is nothing to read back:
 *              RFC 7592 read requires a registration this portal never made.
 *   updateKey  refuses (409, inherited). The client's metadata lives at the key
 *              manager and is edited there.
 *   deleteKey  does nothing, deliberately — see the note on the method.
 *
 * `requestToken` is inherited and works: it needs only the token endpoint, which
 * a `key_managers` row always has, and the developer supplies the secret.
 */

const { register } = require('../core/registry');
const { KeyManager, prop, toKey } = require('../core/keyManager');

const TYPE = 'provision';

class ProvisionKeyManager extends KeyManager {
    constructor(cfg, authRequest) {
        super(cfg);
        /*
         * Needed for exactly one thing: requestToken, which dials the key manager's
         * token endpoint with the developer's own credential. Nothing else here
         * leaves the process — registration, read, update and delete are all local.
         *
         * So the requester carries no credential of its own; what it must carry is
         * the guarded client, which is where the address checks, the redirect ban
         * and the byte ceilings live.
         */
        this.authRequest = authRequest;
        this.type = TYPE;
        // The developer brings a client id; nothing is created here.
        this.keyCreation = 'provide';
    }

    /*
     * Two properties, and they are the whole form.
     *
     * The Add Key page builds itself from this list, so declaring them here is
     * all it takes for the import form to render — there is no second code path
     * in the UI for this kind of key manager.
     *
     * `client_name` is RFC 7591's member name, which every driver here uses, so
     * the caller can read a key's display name from one place whatever created
     * it. Nothing is sent anywhere for this driver; it is stored so the key is
     * recognisable in a list.
     */
    metadata() {
        return this._metaEnvelope([
            prop('client_name', 'Name', 'string',
                'What to call this key in the portal. Not sent anywhere — the client is '
                + 'already named at the key manager.', true),
            prop('consumerKey', 'Consumer key', 'string',
                'The client id of an application you already created in this key manager.', true),
        ]);
    }

    /**
     * Record a client that already exists.
     *
     * Nothing is registered and nothing is dialled. The secret is deliberately
     * absent from the result: the portal never sees it, and token generation
     * asks for it per request instead of storing it.
     */
    async createKey(properties) {
        const consumerKey = properties && typeof properties.consumerKey === 'string'
            ? properties.consumerKey.trim()
            : '';
        if (!consumerKey) {
            throw this._unsupported('createKey');
        }
        return toKey(this, properties, { client_id: consumerKey });
    }

    /**
     * Answer from the portal's own record.
     *
     * Overridden rather than left to refuse, because "read" here means two
     * different things and only one of them is impossible: the portal cannot
     * read the client back from the key manager, but it can perfectly well
     * report what it stored. Refusing would leave a developer unable to open a
     * key they can see in the list.
     */
    async getKey(consumerKey) {
        return toKey(this, {}, { client_id: consumerKey });
    }

    /**
     * Deliberately does nothing.
     *
     * The caller removes the portal's record after this resolves; this method
     * is the step that would have deleted the client upstream, and must not.
     * Someone else created that client and other systems may depend on it.
     *
     * This is safe only because a key manager's type is fixed at creation: a
     * key recorded here can never later be operated on by a driver that would
     * issue a real delete. If that rule is ever relaxed, this method stops
     * being sufficient and the decision has to move onto the key itself.
     */
    async deleteKey() {
        // no-op
    }
}

register(TYPE, (cfg, authRequest) => new ProvisionKeyManager(cfg, authRequest), 'Provision');

module.exports = { ProvisionKeyManager, TYPE };
