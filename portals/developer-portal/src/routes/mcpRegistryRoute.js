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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
const express = require('express');
const router = express.Router({ mergeParams: true });
const mcpRegistryService = require('../services/mcpRegistryService');
const { enforceSecuirty } = require('../middlewares/ensureAuthenticated');
const constants = require('../utils/constants');

router.use((req, res, next) => {
    res.setHeader('Access-Control-Allow-Origin', '*');
    res.setHeader('Access-Control-Allow-Methods', 'GET');
    if (req.method === 'OPTIONS') {
        return res.sendStatus(204);
    }
    next();
});

// Discovery endpoints (public)
router.get('/v0.1/servers', mcpRegistryService.listServers);
router.get('/v0.1/servers/:serverName/versions', mcpRegistryService.listVersions);
router.get('/v0.1/servers/:serverName/versions/:version', mcpRegistryService.getVersion);

// Publishing endpoints (API key or session auth — same as all other devportal write routes)
router.post('/v0.1/publish', enforceSecuirty(constants.SCOPES.DEVELOPER), mcpRegistryService.publishServer);
router.put('/v0.1/servers/:serverName/versions/:version', enforceSecuirty(constants.SCOPES.DEVELOPER), mcpRegistryService.updateVersion);
router.delete('/v0.1/servers/:serverName/versions/:version', enforceSecuirty(constants.SCOPES.DEVELOPER), mcpRegistryService.deleteVersion);
router.patch('/v0.1/servers/:serverName/versions/:version/status', enforceSecuirty(constants.SCOPES.DEVELOPER), mcpRegistryService.updateVersionStatus);
router.patch('/v0.1/servers/:serverName/status', enforceSecuirty(constants.SCOPES.DEVELOPER), mcpRegistryService.updateAllVersionsStatus);

module.exports = router;
