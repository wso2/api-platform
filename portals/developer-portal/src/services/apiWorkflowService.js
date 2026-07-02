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
const apiWorkflowDao = require('../dao/apiWorkflowDao');
const userIdpReferenceDao = require('../dao/userIdpReferenceDao');
const viewDao = require('../dao/viewDao');
const orgDao = require('../dao/organizationDao');
const sequelize = require('../db/sequelizeConfig');
const { UniqueConstraintError } = require('sequelize');
const logger = require('../config/logger');
const { config } = require('../config/configLoader');
const constants = require('../utils/constants');
const util = require('../utils/util');
const yaml = require('js-yaml');
const { CustomError } = require('../utils/errors/customErrors');

const resolveViewId = async (orgId, viewName) => {
    return await viewDao.getId(orgId, viewName);
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
 * @param {string} name - API Workflow name
 * @param {string} description - API Workflow description
 * @param {Array} apis - Array of API metadata (unused, kept for backward compatibility)
 * @param {string} orgHandle - Organization handle for building workflow URL
 * @param {string} viewName - View name
 * @param {string} baseUrl - Base URL of the portal
 * @param {string} handle - API Workflow handle for constructing the workflow detail URL
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
8. On error: log step UUID, status code, and response body

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

// Handles are used to build route segments and markdown file links, so user-supplied
// ids must be restricted to the same safe character set generateHandle() produces.
const HANDLE_PATTERN = /^[a-zA-Z0-9_-]+$/;

const createAPIWorkflow = async (req, res) => {
    const orgId = req.orgId;
    const viewHandle = req.params.viewId;
    const userId = util.resolveActor(req);
    const { name, id, description, agentPrompt, status, agentVisibility, apiWorkflowDefinition, markdownContent, contentType } = req.body;
    let resolvedHandle = (id && id.trim()) ? id.trim() : generateHandle(name);
    if (!resolvedHandle) {
        const suffix = Math.random().toString(36).slice(2, 10);
        resolvedHandle = `workflow-${suffix}`;
    }
    if (id && id.trim() && !HANDLE_PATTERN.test(resolvedHandle)) {
        return res.status(400).json({ message: "Invalid 'id'. Must contain only letters, numbers, underscores, and hyphens." });
    }
    const resolvedContentType = contentType || constants.API_WORKFLOW_CONTENT_TYPE.ARAZZO;
    if (!Object.values(constants.API_WORKFLOW_CONTENT_TYPE).includes(resolvedContentType)) {
        return res.status(400).json({ message: `Invalid contentType. Must be one of: ${Object.values(constants.API_WORKFLOW_CONTENT_TYPE).join(', ')}.` });
    }
    if (status && !Object.values(constants.API_WORKFLOW_STATUS).includes(status)) {
        return res.status(400).json({ message: `Invalid status. Must be one of: ${Object.values(constants.API_WORKFLOW_STATUS).join(', ')}.` });
    }
    if (agentVisibility && !Object.values(constants.AGENT_VISIBILITY).includes(agentVisibility)) {
        return res.status(400).json({ message: `Invalid agentVisibility. Must be one of: ${Object.values(constants.AGENT_VISIBILITY).join(', ')}.` });
    }
    const resolvedContent = resolvedContentType === 'MD'
        ? (markdownContent || null)
        : normalizeToJSON(apiWorkflowDefinition);
    if (resolvedContentType !== 'MD' && resolvedContent === null) {
        return res.status(400).json({ message: 'Invalid API workflow definition: content could not be parsed as valid JSON or YAML.' });
    }
    let t;
    try {
        const orgDetails = await orgDao.getByUuid(orgId);
        t = await sequelize.transaction();
        const viewId = await resolveViewId(orgId, viewHandle);
        const resolvedPrompt = agentPrompt && agentPrompt.trim()
            ? agentPrompt.trim()
            : generateAgentPrompt(name, description, [], orgDetails.idp_ref_id || '', viewHandle, '', resolvedHandle);

        const apiWorkflow = await apiWorkflowDao.create(orgId, viewId, {
            name,
            handle: resolvedHandle,
            description,
            agentPrompt: resolvedPrompt,
            status: status || constants.API_WORKFLOW_STATUS.PUBLISHED,
            agentVisibility: agentVisibility || constants.AGENT_VISIBILITY.VISIBLE,
            apiWorkflowDefinition: resolvedContent,
            contentType: resolvedContentType
        }, userId, t);

        await t.commit();
        logger.info('API Workflow created', { apiWorkflowId: apiWorkflow.uuid, orgId, viewId });
        res.status(201).json({
            apiWorkflowId: apiWorkflow.handle,
            name: apiWorkflow.name,
            status: apiWorkflow.status
        });
    } catch (error) {
        if (t) await t.rollback();
        if (error instanceof UniqueConstraintError) {
            return res.status(409).json({ message: 'An API workflow with this handle already exists. Please use a different handle.' });
        }
        if (error instanceof CustomError) {
            return res.status(error.statusCode).json({ message: error.message });
        }
        logger.error('Error creating API Workflow', { error: error.message, stack: error.stack });
        res.status(500).json({ message: constants.ERROR_MESSAGE.API_WORKFLOW_CREATE_ERROR });
    }
};

const updateAPIWorkflow = async (req, res) => {
    const orgId = req.orgId;
    const { apiWorkflowId: apiWorkflowHandle, viewId: viewHandle } = req.params;
    const userId = util.resolveActor(req);
    const { name, id, description, agentPrompt, status, agentVisibility, apiWorkflowDefinition, markdownContent, contentType } = req.body;
    if (status !== undefined && !Object.values(constants.API_WORKFLOW_STATUS).includes(status)) {
        return res.status(400).json({ message: `Invalid status. Must be one of: ${Object.values(constants.API_WORKFLOW_STATUS).join(', ')}.` });
    }
    if (agentVisibility !== undefined && !Object.values(constants.AGENT_VISIBILITY).includes(agentVisibility)) {
        return res.status(400).json({ message: `Invalid agentVisibility. Must be one of: ${Object.values(constants.AGENT_VISIBILITY).join(', ')}.` });
    }
    if (contentType !== undefined && !Object.values(constants.API_WORKFLOW_CONTENT_TYPE).includes(contentType)) {
        return res.status(400).json({ message: `Invalid contentType. Must be one of: ${Object.values(constants.API_WORKFLOW_CONTENT_TYPE).join(', ')}.` });
    }
    if (id !== undefined && !HANDLE_PATTERN.test(id)) {
        return res.status(400).json({ message: "Invalid 'id'. Must contain only letters, numbers, underscores, and hyphens." });
    }
    const resolvedContentType = contentType;
    const resolvedContent = resolvedContentType === 'MD'
        ? (markdownContent !== undefined ? markdownContent : undefined)
        : (apiWorkflowDefinition !== undefined ? normalizeToJSON(apiWorkflowDefinition) : undefined);
    if (resolvedContentType !== 'MD' && apiWorkflowDefinition !== undefined && resolvedContent === null) {
        return res.status(400).json({ message: 'Invalid API workflow definition: content could not be parsed as valid JSON or YAML.' });
    }
    const t = await sequelize.transaction();
    try {
        const viewId = await resolveViewId(orgId, viewHandle);
        const existing = await apiWorkflowDao.getByHandle(orgId, viewId, apiWorkflowHandle);
        if (!existing) {
            await t.rollback();
            return res.status(404).json({ message: constants.ERROR_MESSAGE.API_WORKFLOW_NOT_FOUND });
        }
        const [count] = await apiWorkflowDao.update(orgId, viewId, existing.uuid, {
            name,
            handle: id,
            description,
            agentPrompt,
            status,
            agentVisibility,
            apiWorkflowDefinition: resolvedContent,
            contentType: resolvedContentType
        }, userId, t);

        if (count === 0) {
            await t.rollback();
            return res.status(404).json({ message: constants.ERROR_MESSAGE.API_WORKFLOW_NOT_FOUND });
        }

        await t.commit();
        logger.info('API Workflow updated', { apiWorkflowId: existing.uuid, orgId, viewId });
        res.status(200).json({ message: 'API Workflow updated successfully' });
    } catch (error) {
        await t.rollback();
        if (error instanceof UniqueConstraintError) {
            return res.status(409).json({ message: 'An API workflow with this handle already exists. Please use a different handle.' });
        }
        if (error instanceof CustomError) {
            return res.status(error.statusCode).json({ message: error.message });
        }
        logger.error('Error updating API Workflow', { error: error.message, stack: error.stack });
        res.status(500).json({ message: constants.ERROR_MESSAGE.API_WORKFLOW_UPDATE_ERROR });
    }
};

const deleteAPIWorkflow = async (req, res) => {
    const orgId = req.orgId;
    const { apiWorkflowId: apiWorkflowHandle, viewId: viewHandle } = req.params;
    const t = await sequelize.transaction();
    try {
        const viewId = await resolveViewId(orgId, viewHandle);
        const existing = await apiWorkflowDao.getByHandle(orgId, viewId, apiWorkflowHandle);
        if (!existing) {
            await t.rollback();
            return res.status(404).json({ message: constants.ERROR_MESSAGE.API_WORKFLOW_NOT_FOUND });
        }
        const count = await apiWorkflowDao.delete(orgId, viewId, existing.uuid, t);
        if (count === 0) {
            await t.rollback();
            return res.status(404).json({ message: constants.ERROR_MESSAGE.API_WORKFLOW_NOT_FOUND });
        }
        await t.commit();
        logger.info('API Workflow deleted', { apiWorkflowId: existing.uuid, orgId, viewId });
        res.status(200).json({ message: 'API Workflow deleted successfully' });
    } catch (error) {
        await t.rollback();
        if (error instanceof CustomError) {
            return res.status(error.statusCode).json({ message: error.message });
        }
        logger.error('Error deleting API Workflow', { error: error.message, stack: error.stack });
        res.status(500).json({ message: constants.ERROR_MESSAGE.API_WORKFLOW_DELETE_ERROR });
    }
};

const getAPIWorkflow = async (req, res) => {
    const orgId = req.orgId;
    const { apiWorkflowId: apiWorkflowHandle, viewId: viewHandle } = req.params;
    try {
        const viewId = await resolveViewId(orgId, viewHandle);
        const apiWorkflow = await apiWorkflowDao.getByHandle(orgId, viewId, apiWorkflowHandle);
        if (!apiWorkflow) {
            return res.status(404).json({ message: constants.ERROR_MESSAGE.API_WORKFLOW_NOT_FOUND });
        }
        const audit = await userIdpReferenceDao.buildSingleAuditFields(apiWorkflow);
        res.status(200).json(toAPIWorkflowDTO(apiWorkflow, audit));
    } catch (error) {
        if (error instanceof CustomError) {
            return res.status(error.statusCode).json({ message: error.message });
        }
        logger.error('Error fetching API Workflow', { error: error.message, stack: error.stack });
        res.status(500).json({ message: constants.ERROR_MESSAGE.API_WORKFLOW_RETRIEVE_ERROR });
    }
};

const getAllAPIWorkflows = async (req, res) => {
    const orgId = req.orgId;
    const { viewId: viewHandle } = req.params;
    try {
        const viewId = await resolveViewId(orgId, viewHandle);
        const apiWorkflows = await apiWorkflowDao.list(orgId, viewId);
        res.status(200).json(util.toPaginatedList(await toAPIWorkflowListDTOs(apiWorkflows), req));
    } catch (error) {
        if (error instanceof CustomError) {
            return res.status(error.statusCode).json({ message: error.message });
        }
        logger.error('Error fetching API Workflows', { error: error.message, stack: error.stack });
        res.status(500).json({ message: constants.ERROR_MESSAGE.API_WORKFLOW_RETRIEVE_ERROR });
    }
};

const generatePrompt = async (req, res) => {
    const { name, description, apis, orgHandle, viewName, id } = req.body;
    try {
        const baseUrl = config.baseUrl || `${req.protocol}://${req.get('host')}`;
        const prompt = generateAgentPrompt(name, description, apis || [], orgHandle || '', viewName || 'default', baseUrl, id || '');
        res.status(200).json({ agentPrompt: prompt });
    } catch (error) {
        logger.error('Error generating agent prompt', { error: error.message });
        res.status(500).json({ message: 'Error generating agent prompt' });
    }
};

// Internal utility used by settingsController
const getAllAPIWorkflowsFromDB = async (orgId, viewId) => {
    const apiWorkflows = await apiWorkflowDao.list(orgId, viewId);
    return toAPIWorkflowListDTOs(apiWorkflows);
};

const parseFileContent = (raw) => {
    if (raw == null) return null;
    const str = Buffer.isBuffer(raw) ? raw.toString('utf8') : String(raw);
    try { return JSON.stringify(JSON.parse(str), null, 2); } catch { return str; }
};

const toAPIWorkflowDTO = (apiWorkflow, audit) => {
    const fileContent = parseFileContent(apiWorkflow.file_content);
    return {
    apiWorkflowId: apiWorkflow.handle,
    name: apiWorkflow.name,
    description: apiWorkflow.description,
    agentPrompt: apiWorkflow.agent_prompt,
    status: apiWorkflow.status,
    agentVisibility: apiWorkflow.agent_visibility || constants.AGENT_VISIBILITY.VISIBLE,
    contentType: apiWorkflow.content_type || constants.API_WORKFLOW_CONTENT_TYPE.ARAZZO,
    apiWorkflowDefinition: (apiWorkflow.content_type || constants.API_WORKFLOW_CONTENT_TYPE.ARAZZO) === constants.API_WORKFLOW_CONTENT_TYPE.ARAZZO ? fileContent : null,
    markdownContent: apiWorkflow.content_type === 'MD' ? fileContent : null,
    createdAt: apiWorkflow.created_at ? new Date(apiWorkflow.created_at).toLocaleDateString('en-US', {
        year: 'numeric', month: 'short', day: 'numeric'
    }) : '',
    updatedAt: apiWorkflow.updated_at ? new Date(apiWorkflow.updated_at).toLocaleDateString('en-US', {
        year: 'numeric', month: 'short', day: 'numeric'
    }) : '',
    createdBy: audit?.createdBy,
    updatedBy: audit?.updatedBy,
    };
};

async function toAPIWorkflowListDTOs(apiWorkflows) {
    const auditList = await userIdpReferenceDao.buildListAuditFields(apiWorkflows);
    return apiWorkflows.map((wf, i) => toAPIWorkflowDTO(wf, auditList[i]));
}

module.exports = {
    createAPIWorkflow,
    updateAPIWorkflow,
    deleteAPIWorkflow,
    getAPIWorkflow,
    getAllAPIWorkflows,
    generatePrompt,
    getAllAPIWorkflowsFromDB,
    generateAgentPrompt
};
