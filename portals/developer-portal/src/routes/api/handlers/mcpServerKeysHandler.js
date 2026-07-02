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
 *
 * Tag: MCP Server Keys
 *
 * API keys aren't type-specific internally — these thin wrappers alias the
 * `mcpServerId` path param to `apiId` (what apiKeyController reads) and mark the
 * request as MCP-scoped so handle resolution is limited to MCP-typed records,
 * then delegate straight into the existing apiKeyController handlers.
 *
 * Mutating ops are CSRF-protected.
 */
const apiKeyController = require('../../../controllers/apiKeyController');
const { requireCsrfForMutatingApi } = require('../../../middlewares/csrfProtection');
const { compose } = require('./compose');
const constants = require('../../../utils/constants');

function asMcpRequest(req) {
    req.__forceApiType = constants.API_TYPE.MCP;
    req.params.apiId = req.params.mcpServerId;
    return req;
}

function delegate(fn) {
    return (req, res) => fn(asMcpRequest(req), res);
}

module.exports = {
    generateMcpServerApiKey: compose(requireCsrfForMutatingApi, delegate(apiKeyController.generateApiKey)),
    listMcpServerApiKeys: delegate(apiKeyController.listApiKeys),
    regenerateMcpServerApiKey: compose(requireCsrfForMutatingApi, delegate(apiKeyController.regenerateApiKey)),
    revokeMcpServerApiKey: compose(requireCsrfForMutatingApi, delegate(apiKeyController.revokeApiKey)),
    associateMcpServerApiKeyApplication: compose(requireCsrfForMutatingApi, delegate(apiKeyController.associateApiKeyApplication)),
    removeMcpServerApiKeyApplication: compose(requireCsrfForMutatingApi, delegate(apiKeyController.removeApiKeyApplication)),
};
