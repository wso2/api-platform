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

/*
 * Thin, type-scoped wrapper around apiMetadataService for the /mcp-servers resource
 * family. MCP servers live in the same dp_api_metadata table as REST APIs, distinguished
 * only by the `type` column, so this module reuses apiMetadataService's battle-tested
 * create/update/delete/content logic rather than duplicating it. Delegation works by
 * aliasing the `mcpServerId` path param to `apiId` (what the underlying functions read)
 * and setting `req.__forceApiType = 'MCP'`, which apiMetadataService's shared
 * `resolveScopedApiId` helper checks to resolve handles scoped to MCP-typed records only.
 */
const apiMetadataService = require('../services/apiMetadataService');
const util = require('../utils/util');
const constants = require('../utils/constants');

function asMcpRequest(req) {
    req.__forceApiType = constants.API_TYPE.MCP;
    req.params.apiId = req.params.mcpServerId;
    return req;
}

const createMcpServer = async (req, res) => {
    return apiMetadataService.createAPIMetadata(asMcpRequest(req), res);
};

const getMcpServer = async (req, res) => {
    return apiMetadataService.getAPIMetadata(asMcpRequest(req), res);
};

const updateMcpServer = async (req, res) => {
    return apiMetadataService.updateAPIMetadata(asMcpRequest(req), res);
};

const deleteMcpServer = async (req, res) => {
    return apiMetadataService.deleteAPIMetadata(asMcpRequest(req), res);
};

const createMcpServerContent = async (req, res) => {
    return apiMetadataService.createAPIContent(asMcpRequest(req), res);
};

const replaceMcpServerContent = async (req, res) => {
    return apiMetadataService.updateAPIContent(asMcpRequest(req), res);
};

const getMcpServerContentFile = async (req, res) => {
    return apiMetadataService.getAPIFile(asMcpRequest(req), res);
};

const deleteMcpServerContentFile = async (req, res) => {
    return apiMetadataService.deleteAPIFile(asMcpRequest(req), res);
};

const getAllMcpServersForOrganization = async (req, res) => {
    try {
        const orgId = req.orgId;
        const searchTerm = req.query.query;
        const apiName = req.query.name;
        const apiVersion = req.query.version;
        const tags = req.query.tags;
        const view = req.query.view;
        const retrievedAPIs = await apiMetadataService.getMetadataListFromDB(orgId, searchTerm, tags, apiName, apiVersion, view, { include: constants.API_TYPE.MCP });
        res.status(200).json(util.toPaginatedList(retrievedAPIs, req));
    } catch (error) {
        util.handleError(res, error);
    }
};

module.exports = {
    createMcpServer,
    getMcpServer,
    getAllMcpServersForOrganization,
    updateMcpServer,
    deleteMcpServer,
    createMcpServerContent,
    replaceMcpServerContent,
    getMcpServerContentFile,
    deleteMcpServerContentFile,
};
