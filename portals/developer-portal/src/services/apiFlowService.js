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
const apiFlowDao = require('../dao/apiFlow');
const adminDao = require('../dao/admin');
const apiMetadataDao = require('../dao/apiMetadata');
const sequelize = require('../db/sequelize');
const { UniqueConstraintError } = require('sequelize');
const logger = require('../config/logger');
const { config } = require('../config/configLoader');
const constants = require('../utils/constants');
const yaml = require('js-yaml');

const resolveViewId = async (orgID, viewName) => {
    return await apiMetadataDao.getViewID(orgID, viewName);
};

/**
 * If content is YAML, converts it to a JSON string. If already JSON, returns it as-is.
 * Returns null for null/undefined input.
 */
const normalizeToJSON = (content) => {
    if (content == null) return null;
    if (typeof content === 'object') return JSON.stringify(content);
    const str = typeof content === 'string' ? content.trim() : String(content).trim();
    if (!str) return null;
    try {
        JSON.parse(str);
        return str; // already valid JSON
    } catch {
        // try YAML parse
        try {
            const parsed = yaml.load(str);
            return JSON.stringify(parsed);
        } catch (yamlError) {
            logger.error('Failed to parse content as JSON or YAML', { error: yamlError.message });
            return null;
        }
    }
};

/**
 * Generates a minimal, LLM-ready agent prompt that references the workflow definition.
 * The workflow (llms.txt) contains all execution details, API associations, and instructions.
 * The prompt provides execution guidance for two personas: execution agents and app builder agents.
 * @param {string} name - APIFlow name
 * @param {string} description - APIFlow description
 * @param {Array} apis - Array of API metadata (unused, kept for backward compatibility)
 * @param {string} orgHandle - Organization handle for building workflow URL
 * @param {string} viewName - View name
 * @param {string} baseUrl - Base URL of the portal
 * @param {string} handle - APIFlow handle for constructing the workflow detail URL
 * @returns {string} Agent prompt with two sections (execution and app building)
 */
const generateAgentPrompt = (name, description, apis = [], orgHandle = '', viewName = 'default', baseUrl = '', handle = '') => {
    const workflowUrl = (handle && orgHandle && baseUrl)
        ? `${baseUrl}/${orgHandle}/views/${viewName}/api-workflows/${handle}.md`
        : '';

    const workflowReference = workflowUrl
        ? `\n\nWorkflow Definition (source of truth): ${workflowUrl}`
        : '';

    const section1 = `You are an API orchestration agent executing the "${name}" workflow.${workflowReference}

## Objective
${description}

## Execution Mode
- Execute deterministically, following the workflow steps exactly as defined
- Do not fabricate data or skip steps
- Do not make creative interpretations—follow the workflow literally

## Workflow Source
Read the workflow definition for:
- Complete execution steps and their sequence
- Associated APIs and their OpenAPI specifications
- Security schemes for each API
- Any additional instructions or constraints

## Execution State
Maintain state throughout execution:
- Track the current step
- Record completed steps
- Store data outputs from each step
- Track any errors encountered
- Return the final state and results

## Step Execution Protocol
For each step in the workflow:
1. Identify required inputs (from previous steps, user input, or defaults)
2. Read the OpenAPI specification for the API endpoint
3. Identify required security scheme(s) from the spec
4. Collect any missing credentials from the user
5. Execute the API call with proper authentication
6. Validate response (expect 2xx status)
7. Extract outputs and pass to next step
8. On error: log step ID, status code, and response body

## Retry Policy
- On network errors or 5xx responses: retry up to 3 times with backoff (0s, 1s, 2s)
- On 4xx responses: stop execution immediately (non-retryable)
- Never expose raw credentials or tokens in output

## Constraints
- Only call APIs explicitly listed in the workflow
- Follow all instructions defined in the workflow`;

    const llmsIndexUrl = (orgHandle && baseUrl)
        ? `${baseUrl}/${orgHandle}/views/${viewName}/llms.txt`
        : '';

    const section2 = `You are a software development agent helping build a web application that implements the "${name}" workflow by integrating multiple backend APIs.

## Objective
${description}

## Always follow this order of sources:
- Workflow definition: ${workflowUrl}
- API index: ${llmsIndexUrl}
- For each API in the index, read its markdown doc and then its OpenAPI spec (the source of truth for endpoints, schemas, and security)

## Do not fetch or process full documents. Extract only what is needed:
- From the workflow: list steps in order with API, method, path, inputs, outputs, and copy the authentication exactly
- From each API: list endpoints (method, path, parameters, request/response) and copy the securitySchemes exactly

Do not write any code until all APIs are processed, the workflow is fully mapped to endpoints, and you have confirmed your understanding with the developer.

Use a modular architecture with one service per API and a single orchestrator for the workflow. Keep API clients, business logic, and UI clearly separated.

All configurations must use environment variables. Use per-API API keys (e.g. TREATMENTS_API_KEY). OAuth2 credentials may be shared only if confirmed in securitySchemes; otherwise use per-API prefixes. Provide a .env.example listing all required variables grouped by API.

Fail fast if any required configuration is missing. Never log API keys, tokens, or sensitive data. Do not assume undocumented behavior or invent endpoints—rely only on the OpenAPI specs. Ask for clarification if anything is unclear.`;

    return `> **Which prompt should I use?**
> If you are a web-based agent (e.g. claude.ai or any browser-based AI assistant), use **Section 1: API Execution Agent** below.
> If you are a coding/IDE agent building an application, use **Section 2: App Builder Agent**.
> If you are unsure which applies to you, ask the user how they intend to use this workflow.

# Section 1: API Execution Agent

${section1}

# Section 2: App Builder Agent

${section2}`;
};

const generateHandle = (name) =>
    name.toLowerCase().trim()
        .replace(/[^\w\s-]/g, '')
        .replace(/\s+/g, '-')
        .replace(/-+/g, '-')
        .substring(0, 100);

const createAPIFlow = async (req, res) => {
    const orgID = req.params.orgId;
    const viewName = req.params.viewName;
    const { name, handle, description, agentPrompt, status, visibility, agentVisibility, apiFlowDefinition, markdownContent, contentType } = req.body;
    let resolvedHandle = (handle && handle.trim()) ? handle.trim() : generateHandle(name);
    if (!resolvedHandle) {
        const suffix = Math.random().toString(36).slice(2, 10);
        resolvedHandle = `flow-${suffix}`;
    }
    const resolvedContentType = contentType || 'ARAZZO';
    const resolvedContent = resolvedContentType === 'MD'
        ? (markdownContent || null)
        : normalizeToJSON(apiFlowDefinition);
    if (resolvedContentType !== 'MD' && resolvedContent === null) {
        return res.status(400).json({ message: 'Invalid API flow definition: content could not be parsed as valid JSON or YAML.' });
    }
    const t = await sequelize.transaction();
    try {
        const viewId = await resolveViewId(orgID, viewName);
        const resolvedPrompt = agentPrompt && agentPrompt.trim()
            ? agentPrompt.trim()
            : generateAgentPrompt(name, description, [], req.params.orgName, viewName, '', resolvedHandle);

        const apiFlow = await apiFlowDao.createAPIFlow(orgID, viewId, {
            name,
            handle: resolvedHandle,
            description,
            agentPrompt: resolvedPrompt,
            status: status || 'PUBLISHED',
            visibility: visibility || 'PUBLIC',
            agentVisibility: agentVisibility || 'VISIBLE',
            apiFlowDefinition: resolvedContent,
            contentType: resolvedContentType
        }, t);

        await t.commit();
        logger.info('APIFlow created', { apiFlowId: apiFlow.API_FLOW_ID, orgID, viewId });
        res.status(201).json({
            apiFlowId: apiFlow.API_FLOW_ID,
            name: apiFlow.NAME,
            status: apiFlow.STATUS
        });
    } catch (error) {
        await t.rollback();
        if (error instanceof UniqueConstraintError) {
            return res.status(409).json({ message: 'An API workflow with this handle already exists. Please use a different handle.' });
        }
        logger.error('Error creating APIFlow', { error: error.message, stack: error.stack });
        res.status(500).json({ message: constants.ERROR_MESSAGE.API_FLOW_CREATE_ERROR });
    }
};

const updateAPIFlow = async (req, res) => {
    const { orgId, apiFlowId, viewName } = req.params;
    const { name, handle, description, agentPrompt, status, visibility, agentVisibility, apiFlowDefinition, markdownContent, contentType } = req.body;
    const resolvedContentType = contentType;
    const resolvedContent = resolvedContentType === 'MD'
        ? (markdownContent !== undefined ? markdownContent : undefined)
        : (apiFlowDefinition !== undefined ? normalizeToJSON(apiFlowDefinition) : undefined);
    if (resolvedContentType !== 'MD' && apiFlowDefinition !== undefined && resolvedContent === null) {
        return res.status(400).json({ message: 'Invalid API flow definition: content could not be parsed as valid JSON or YAML.' });
    }
    const t = await sequelize.transaction();
    try {
        const viewId = await resolveViewId(orgId, viewName);
        const [count] = await apiFlowDao.updateAPIFlow(orgId, viewId, apiFlowId, {
            name,
            handle,
            description,
            agentPrompt,
            status,
            visibility,
            agentVisibility,
            apiFlowDefinition: resolvedContent,
            contentType: resolvedContentType
        }, t);

        if (count === 0) {
            await t.rollback();
            return res.status(404).json({ message: constants.ERROR_MESSAGE.API_FLOW_NOT_FOUND });
        }

        await t.commit();
        logger.info('APIFlow updated', { apiFlowId, orgId, viewId });
        res.status(200).json({ message: 'APIFlow updated successfully' });
    } catch (error) {
        await t.rollback();
        if (error instanceof UniqueConstraintError) {
            return res.status(409).json({ message: 'An API workflow with this handle already exists. Please use a different handle.' });
        }
        logger.error('Error updating APIFlow', { error: error.message, stack: error.stack });
        res.status(500).json({ message: constants.ERROR_MESSAGE.API_FLOW_UPDATE_ERROR });
    }
};

const deleteAPIFlow = async (req, res) => {
    const { orgId, apiFlowId, viewName } = req.params;
    const t = await sequelize.transaction();
    try {
        const viewId = await resolveViewId(orgId, viewName);
        const count = await apiFlowDao.deleteAPIFlow(orgId, viewId, apiFlowId, t);
        if (count === 0) {
            await t.rollback();
            return res.status(404).json({ message: constants.ERROR_MESSAGE.API_FLOW_NOT_FOUND });
        }
        await t.commit();
        logger.info('APIFlow deleted', { apiFlowId, orgId, viewId });
        res.status(200).json({ message: 'APIFlow deleted successfully' });
    } catch (error) {
        await t.rollback();
        logger.error('Error deleting APIFlow', { error: error.message, stack: error.stack });
        res.status(500).json({ message: constants.ERROR_MESSAGE.API_FLOW_DELETE_ERROR });
    }
};

const getAPIFlow = async (req, res) => {
    const { orgId, apiFlowId, viewName } = req.params;
    try {
        const viewId = await resolveViewId(orgId, viewName);
        const apiFlow = await apiFlowDao.getAPIFlow(orgId, viewId, apiFlowId);
        if (!apiFlow) {
            return res.status(404).json({ message: constants.ERROR_MESSAGE.API_FLOW_NOT_FOUND });
        }
        res.status(200).json(toAPIFlowDTO(apiFlow));
    } catch (error) {
        logger.error('Error fetching APIFlow', { error: error.message, stack: error.stack });
        res.status(500).json({ message: constants.ERROR_MESSAGE.API_FLOW_RETRIEVE_ERROR });
    }
};

const getAllAPIFlows = async (req, res) => {
    const { orgId, viewName } = req.params;
    try {
        const viewId = await resolveViewId(orgId, viewName);
        const apiFlows = await apiFlowDao.getAllAPIFlows(orgId, viewId);
        res.status(200).json(apiFlows.map(toAPIFlowDTO));
    } catch (error) {
        logger.error('Error fetching APIFlows', { error: error.message, stack: error.stack });
        res.status(500).json({ message: constants.ERROR_MESSAGE.API_FLOW_RETRIEVE_ERROR });
    }
};

const generatePrompt = async (req, res) => {
    const { name, description, apis, orgHandle, viewName, handle } = req.body;
    try {
        const baseUrl = config.baseUrl || `${req.protocol}://${req.get('host')}`;
        const prompt = generateAgentPrompt(name, description, apis || [], orgHandle || '', viewName || 'default', baseUrl, handle || '');
        res.status(200).json({ agentPrompt: prompt });
    } catch (error) {
        logger.error('Error generating agent prompt', { error: error.message });
        res.status(500).json({ message: 'Error generating agent prompt' });
    }
};

// Internal utility used by settingsController
const getAllAPIFlowsFromDB = async (orgID, viewId) => {
    const apiFlows = await apiFlowDao.getAllAPIFlows(orgID, viewId);
    return apiFlows.map(toAPIFlowDTO);
};

const parseFileContent = (raw) => {
    if (raw == null) return null;
    const str = Buffer.isBuffer(raw) ? raw.toString('utf8') : String(raw);
    try { return JSON.stringify(JSON.parse(str), null, 2); } catch { return str; }
};

const toAPIFlowDTO = (apiFlow) => {
    const fileContent = parseFileContent(apiFlow.FILE_CONTENT);
    return {
    apiFlowId: apiFlow.API_FLOW_ID,
    name: apiFlow.NAME,
    handle: apiFlow.HANDLE,
    description: apiFlow.DESCRIPTION,
    agentPrompt: apiFlow.AGENT_PROMPT,
    status: apiFlow.STATUS,
    visibility: apiFlow.VISIBILITY || 'PUBLIC',
    agentVisibility: apiFlow.AGENT_VISIBILITY || 'VISIBLE',
    contentType: apiFlow.CONTENT_TYPE || 'ARAZZO',
    apiFlowDefinition: (apiFlow.CONTENT_TYPE || 'ARAZZO') === 'ARAZZO' ? fileContent : null,
    markdownContent: apiFlow.CONTENT_TYPE === 'MD' ? fileContent : null,
    createdAt: apiFlow.CREATED_AT ? new Date(apiFlow.CREATED_AT).toLocaleDateString('en-US', {
        year: 'numeric', month: 'short', day: 'numeric'
    }) : '',
    updatedAt: apiFlow.UPDATED_AT
    };
};

module.exports = {
    createAPIFlow,
    updateAPIFlow,
    deleteAPIFlow,
    getAPIFlow,
    getAllAPIFlows,
    generatePrompt,
    getAllAPIFlowsFromDB,
    generateAgentPrompt
};
