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

const { renderTemplateWithView, resolveActor, pageErrorStatus } = require('../utils/util');
const logger = require('../config/logger');
const constants = require('../utils/constants');
const orgDao = require('../dao/organizationDao');
const oauth2KeyDao = require('../dao/oauth2ConsumerKeyDao');
const keyAppMappingDao = require('../dao/oauth2KeyAppMappingDao');
const applicationDao = require('../dao/applicationDao');
const kmRegistry = require('../services/keyManagerRegistry');

/**
 * Renders the org-wide "OAuth2 Keys" page — every OAuth2 application the signed-in
 * user has registered on a key manager through Dynamic Client Registration.
 *
 * Server-rendered, like the Subscriptions and API Keys pages: the whole list is
 * written into the page rather than fetched by the browser, so it is present on
 * first paint and needs no client-side pagination.
 *
 * Two separate reads, and a failure of either is degraded rather than fatal:
 *
 *   - the caller's keys, from the portal's own table;
 *   - the key managers, so a key can be labelled with a display name instead of a
 *     raw handle, and so the page knows whether generating a key is possible at all.
 *
 * Only a config-declared key manager can register a client — a database-backed one
 * carries a token endpoint and no driver config — so that is what decides whether
 * the "Generate key" button is offered. Offering it with nothing behind it would
 * send the user into a modal that can only fail.
 */
const loadOAuth2Keys = async (req, res, next) => {
    let html;
    const { orgName, viewName } = req.params;

    try {
        const orgDetails = await orgDao.get(orgName);
        const orgId = orgDetails.uuid;

        if (!req.user) {
            return res.redirect(`${constants.ROUTE.BASE_PATH}/${orgName}${constants.ROUTE.VIEWS_PATH}${viewName}/login`);
        }

        // Key manager names first: a key row is labelled from this, so resolving it
        // before the keys keeps the mapping in one place below.
        const keyManagerNames = new Map();
        let generationAvailable = false;
        try {
            /*
             * Disabled ones included, deliberately, and this is the one place the
             * distinction bites: a key issued against a key manager that was later
             * disabled still exists, still appears in this list, and still needs a
             * name. Left out, the row falls back to the raw handle below and the
             * column shows a UUID — which is what it did until this was fixed.
             * `_keyManagerNames` in oauth2KeyService already passed the flag; this
             * copy of the same lookup did not.
             */
            const keyManagers = await kmRegistry.list(orgId, { includeDisabled: true });
            for (const km of keyManagers) {
                keyManagerNames.set(km.handle, km.display_name || km.handle);
            }
            // Any key manager at all means a key can be made here. It used to mean a
            // config-declared one, because nothing else could register a client — but
            // a stored key manager now either registers one or takes a client id you
            // already have, and both end in a key. Leaving the old test in place
            // disabled the button on exactly the portals this was meant to support:
            // those whose key managers all predate the configuration table.
            // Enabled only, unlike the names above: a disabled key manager is one an
            // admin has taken out of service, so it must not be what makes the
            // "Add key" button live.
            generationAvailable = keyManagers.some((km) => Boolean(km.enabled));
        } catch (kmError) {
            // A key manager that cannot be built (an unreadable mTLS certificate, say)
            // must not take the whole page down: the keys already issued are still
            // worth showing, just with their raw handle as the label.
            logger.warn('Failed to resolve key managers for the OAuth2 keys page', {
                error: kmError.message, orgId,
            });
        }

        const actor = resolveActor(req);
        let oauth2Keys = [];
        let keysLoadError = false;
        // The caller's own applications, for the associate picker. Loaded once for
        // the page rather than fetched by the browser when a modal opens: it is a
        // short list the server already has, and a picker that populates on click
        // is a spinner for no reason.
        let applications = [];
        try {
            const records = await oauth2KeyDao.listByCreator(orgId, actor);

            // Associations for the whole page in one query, then the applications
            // they name in a second — not two lookups per row.
            const appByKey = await keyAppMappingDao.getByKeys((records || []).map((k) => k.keyId));
            const appNames = new Map();
            for (const appUuid of new Set(appByKey.values())) {
                const app = await applicationDao.get(orgId, appUuid, actor);
                if (app) appNames.set(appUuid, { id: app.handle, displayName: app.display_name });
            }

            oauth2Keys = (records || []).map((k) => {
                const app = appNames.get(appByKey.get(k.keyId));
                return {
                    keyId: k.keyId,
                    name: k.name || '',
                    keyManagerId: k.keyManagerId,
                    keyManagerName: keyManagerNames.get(k.keyManagerId) || k.keyManagerId,
                    consumerKey: k.consumerKey,
                    createdAt: k.createdAt,
                    applicationId: app ? app.id : '',
                    applicationName: app ? app.displayName : '',
                };
            });

            applications = (await applicationDao.list(orgId, actor) || [])
                .map((a) => ({ id: a.handle, displayName: a.display_name }));
        } catch (dbError) {
            keysLoadError = true;
            logger.warn('Failed to load OAuth2 keys', { error: dbError.message, orgId });
        }

        const profile = {
            firstName: req.user.firstName,
            lastName: req.user.lastName,
            email: req.user.email,
            imageURL: req.user.picture || req.user.imageURL || constants.DEFAULT_PROFILE_IMAGE_URL,
            isAdmin: req.user.isAdmin,
        };

        const templateContent = {
            baseUrl: constants.ROUTE.BASE_PATH + '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
            profile: profile,
            orgId: orgId,
            oauth2Keys: oauth2Keys,
            oauth2KeysCount: oauth2Keys.length,
            applications: applications,
            keysLoadError,
            generationAvailable,
        };

        html = await renderTemplateWithView('../pages/oauth2-keys/page.hbs', './src/defaultContent/layout/main.hbs', templateContent, true, orgId, viewName);
        res.send(html);
    } catch (error) {
        logger.error('Error loading OAuth2 keys page', {
            error: error.message,
            stack: error.stack,
            orgName,
        });
        error.status = pageErrorStatus(error);
        return next(error);
    }
};

module.exports = { loadOAuth2Keys };
