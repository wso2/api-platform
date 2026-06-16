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

const REGISTRY_META_KEY = 'io.modelcontextprotocol.registry/official';

const STATUS_MAP = {
    PUBLISHED: 'active',
    DEPRECATED: 'deprecated',
    DELETED: 'deleted'
};

/**
 * Maps a DP_API_METADATA row (+ optional parsed schema) to the MCP registry spec response shape.
 *
 * @param {object} row        - Sequelize APIMetadata instance or plain object
 * @param {object} [schema]   - Parsed schema object { tools, resources, prompts } from DP_API_CONTENT
 */
class ServerResponseDTO {
    constructor(row, schema) {
        const source = typeof row?.get === 'function' ? row.get({ plain: true }) : row;

        const storedRemotes = source.METADATA_SEARCH?.apiInfo?.remotes;
        const remotes = Array.isArray(storedRemotes) && storedRemotes.length > 0
            ? storedRemotes
            : (source.PRODUCTION_URL ? [{ type: 'streamable-http', url: source.PRODUCTION_URL }] : []);

        this.server = {
            $schema: 'https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json',
            name: source.METADATA_SEARCH?.apiInfo?.proxyId || source.API_NAME,
            title: source.METADATA_SEARCH?.apiInfo?.apiTitle || undefined,
            version: source.API_VERSION,
            description: source.API_DESCRIPTION,
            remotes
        };

        const choreoCapabilities = {};
        if (schema) {
            choreoCapabilities.tools = schema.tools || [];
            choreoCapabilities.resources = schema.resources || [];
            choreoCapabilities.prompts = schema.prompts || [];
        }

        const registryMeta = source.METADATA_SEARCH?.apiInfo || {};
        this._meta = {
            [REGISTRY_META_KEY]: {
                status: STATUS_MAP[source.STATUS] || source.STATUS,
                publishedAt: registryMeta.publishedAt || undefined,
                updatedAt: registryMeta.updatedAt || undefined,
                isLatest: true
            },
            ...(schema ? { 'io.api-platform/mcp-capabilities': choreoCapabilities } : {})
        };
    }
}

module.exports = ServerResponseDTO;
