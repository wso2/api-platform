/*
 * Copyright (c) 2024, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
/* eslint-disable no-undef */
const { Sequelize } = require("sequelize");
const sequelize = require('../db/sequelizeConfig');
const apiDao = require("../dao/apiDao");
const subDao = require('../dao/subscriptionDao');
const labelDao = require('../dao/labelDao');
const tagDao = require('../dao/tagDao');
const viewDao = require('../dao/viewDao');
const subscriptionPlanDao = require('../dao/subscriptionPlanDao');
const apiFileDao = require('../dao/apiFileDao');
const apiKeyDao = require("../dao/apiKeyDao");
const util = require("../utils/util");
const logger = require("../config/logger");
const { config } = require('../config/configLoader');
const path = require("path");
const fs = require("fs").promises;
const fsDir = require("fs");
const yaml = require('js-yaml');
const APIDTO = require("../dto/apiDto");
const ViewDTO = require("../dto/viewsDto");
const APIDocDTO = require("../dto/apiDocDto");
const constants = require("../utils/constants");
const subscriptionPlanDTO = require("../dto/subscriptionPlanDto");
const { CustomError } = require("../utils/errors/customErrors");
const LabelDTO = require("../dto/labelDto");

const createAPIMetadata = async (req, res) => {
    const orgId = req.orgId;
    const userId = util.resolveActor(req);
    logger.info('Creating API metadata...', {
        orgId
    });
    let apiMetadata;
    let apiDefinitionFile, apiFileName = "";
    let fullApiBundle;
    const apiArtifactFile = req.files?.artifact?.[0];

    try {
        let artifactApiContent = [];
        let resolvedImageMetadata = {};
        if (apiArtifactFile?.buffer) {
            fullApiBundle = await extractFullApiBundleFromUploadedZip(apiArtifactFile, orgId, 'new-api');
            apiMetadata = fullApiBundle.apiMetadata;
            const preparedDefinition = prepareApiDefinitionForStorage(
                fullApiBundle.apiDefinitionFileName,
                fullApiBundle.apiDefinitionFile
            );
            apiDefinitionFile = preparedDefinition.apiDefinitionFile;
            apiFileName = preparedDefinition.apiDefinitionFileName;
            artifactApiContent = await extractApiContentFromUploadedZip(apiArtifactFile, orgId, 'new-api', 'artifact');
            resolvedImageMetadata = buildImageMetadataFromContent(artifactApiContent);
            const filenameToKey = Object.fromEntries(Object.entries(resolvedImageMetadata).map(([key, fileName]) => [fileName, key]));
            artifactApiContent.forEach(file => {
                if (file.type === constants.DOC_TYPES.IMAGES) {
                    file.key = filenameToKey[file.fileName];
                }
            });
        } else if (req.files?.api?.[0]) {
            apiMetadata = parseApiMetadataFromYamlRequest(req);
            if (req.files?.apiDefinition?.[0]) {
                const file = req.files.apiDefinition[0];
                const preparedDefinition = prepareApiDefinitionForStorage(file.originalname, file.buffer);
                apiDefinitionFile = preparedDefinition.apiDefinitionFile;
                apiFileName = preparedDefinition.apiDefinitionFileName;
            }
        } else {
            apiMetadata = JSON.parse(req.body.apiMetadata);
            if (req.files?.apiDefinition?.[0]) {
                const file = req.files.apiDefinition[0];
                const preparedDefinition = prepareApiDefinitionForStorage(file.originalname, file.buffer);
                apiDefinitionFile = preparedDefinition.apiDefinitionFile;
                apiFileName = preparedDefinition.apiDefinitionFileName;
            }
        }

        // Validate input
        const hasGraphQLSchema = apiMetadata.apiInfo?.apiType === constants.API_TYPE.GRAPHQL &&
            req.files?.schemaDefinition?.[0];
        if (!apiMetadata.apiInfo || (!apiDefinitionFile && !hasGraphQLSchema) || !apiMetadata.endPoints) {
            throw new Sequelize.ValidationError(
                "Missing or Invalid fields in the request payload"
            );
        }
        const { apiStatus, agentVisibility: infoAgentVisibility } = apiMetadata.apiInfo;
        const agentVisibility = apiMetadata.agentVisibility || infoAgentVisibility;
        if (apiStatus && !Object.values(constants.API_STATUS).includes(apiStatus)) {
            throw new Sequelize.ValidationError(`Invalid apiStatus '${apiStatus}'. Must be one of: ${Object.values(constants.API_STATUS).join(', ')}.`);
        }
        if (agentVisibility) {
            const normalizedAgentVisibility = agentVisibility.toUpperCase();
            if (!Object.values(constants.AGENT_VISIBILITY).includes(normalizedAgentVisibility)) {
                throw new Sequelize.ValidationError(`Invalid agentVisibility '${agentVisibility}'. Must be one of: ${Object.values(constants.AGENT_VISIBILITY).join(', ')}.`);
            }
            apiMetadata.agentVisibility = normalizedAgentVisibility;
            apiMetadata.apiInfo.agentVisibility = normalizedAgentVisibility;
        }
        apiMetadata.endPoints.productionURL = changeEndpoint(apiMetadata.endPoints.productionURL);
        apiMetadata.endPoints.sandboxURL = changeEndpoint(apiMetadata.endPoints.sandboxURL);
        normalizeGraphQLEndpoints(apiMetadata);
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            // Create apimetadata record
            const createdAPI = await apiDao.create(orgId, apiMetadata, userId, t);
            const apiId = createdAPI.dataValues.UUID;
            if (apiMetadata.subscriptionPlans) {
                const subscriptionPlans = [];
                const apiSubscriptionPlans = apiMetadata.subscriptionPlans;
                if (!Array.isArray(apiSubscriptionPlans)) {
                    throw new Sequelize.ValidationError(
                        "Missing or Invalid fields in the request payload"
                    );
                } else {
                    for (const plan of apiSubscriptionPlans) {
                        const subscriptionPlan = await subscriptionPlanDao.getByName(orgId, plan.planName);
                        if (!subscriptionPlan) {
                            throw new Sequelize.EmptyResultError("Subscription plan not found");
                        } else {
                            subscriptionPlans.push({ apiId: apiId, planId: subscriptionPlan.UUID });
                        }
                    };
                }
                await subscriptionPlanDao.createApiMapping(subscriptionPlans, apiId, userId, t);
            }
            //store api labels
            if (apiMetadata.apiInfo.labels) {
                const labels = apiMetadata.apiInfo.labels;
                if (!Array.isArray(labels)) {
                    throw new Sequelize.ValidationError(
                        "Missing or Invalid fields in the request payload"
                    );
                }
                await labelDao.createApiMapping(orgId, apiId, labels, userId, t);
            } else {
                await labelDao.createApiMapping(orgId, apiId, ['default'], userId, t);
            }
            //store api tags
            if (apiMetadata.apiInfo.tags) {
                const tags = apiMetadata.apiInfo.tags;
                if (!Array.isArray(tags)) {
                    throw new Sequelize.ValidationError(
                        "Missing or Invalid fields in the request payload"
                    );
                }
                await tagDao.createApiMapping(orgId, apiId, tags, userId, t);
            }
            // store api definition file (skipped for GraphQL — schema stored below via schemaDefinition)
            if (apiDefinitionFile) {
                await apiFileDao.store(apiDefinitionFile, apiFileName, apiId, constants.DOC_TYPES.API_DEFINITION, userId, t);
            }
            // store uploaded documentation files
            if (req.files?.docs) {
                for (const doc of req.files.docs) {
                    await apiFileDao.store(doc.buffer, doc.originalname, apiId, constants.DOC_TYPES.DOC_ID + constants.DOC_TYPES.DOCS.OTHER, userId, t);
                }
            }
            // Save MCP tools as schema definition if the API type is MCP
            if (constants.API_TYPE.MCP === apiMetadata.apiInfo.apiType) {
                let schemaFile;
                if (req.files?.schemaDefinition?.[0]) {
                    schemaFile = req.files.schemaDefinition[0];
                } else if (fullApiBundle?.schemaDefinitionFile) {
                    schemaFile = {
                        originalname: fullApiBundle.schemaDefinitionFileName,
                        buffer: fullApiBundle.schemaDefinitionFile
                    };
                }
                if (schemaFile) {
                    const schemaDefinition = prepareSchemaDefinitionForStorage(schemaFile.originalname, schemaFile.buffer);
                    logger.debug('Schema definition file received', {
                        apiId: apiId,
                        schemaDefinitionFileSize: schemaDefinition.schemaDefinitionFile.length,
                        schemaFileName: schemaDefinition.schemaDefinitionFileName
                    });
                    await apiFileDao.store(schemaDefinition.schemaDefinitionFile, schemaDefinition.schemaDefinitionFileName, apiId,
                        constants.DOC_TYPES.SCHEMA_DEFINITION, userId, t);
                    logger.info('Schema definition file stored', {
                        apiId: apiId,
                        schemaFileName: schemaDefinition.schemaDefinitionFileName
                    });
                }
            }

            if (constants.API_TYPE.GRAPHQL === apiMetadata.apiInfo.apiType && req.files?.schemaDefinition?.[0]) {
                const file = req.files.schemaDefinition[0];
                const schemaDefinitionFile = file.buffer;
                logger.debug('GraphQL schema definition file received', {
                    apiId: apiId,
                    schemaDefinitionFileSize: schemaDefinitionFile.length,
                    schemaFileName: file.originalname
                });
                const schemaFileName = constants.FILE_NAME.API_DEFINITION_GRAPHQL;
                await apiFileDao.store(schemaDefinitionFile, schemaFileName, apiId,
                    constants.DOC_TYPES.API_DEFINITION, userId, t);
                logger.info('GraphQL schema definition file stored', {
                    apiId: apiId,
                    schemaFileName
                });
            }

            if (apiArtifactFile?.buffer && artifactApiContent.length > 0) {
                await apiFileDao.storeMany(artifactApiContent, apiId, userId, t);
            }
            apiMetadata.apiId = apiId;
        });


        res.status(201).send(apiMetadata);
    } catch (error) {
        logger.error('API metadata creation failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.orgId
        });
        util.handleError(res, error);
    }
};

function changeEndpoint(endPoint) {

    if (endPoint !== undefined && endPoint !== null && endPoint.includes("choreoapis")) {
        return endPoint.replace("choreoapis", "bijiraapis");
    }
    return endPoint;
}

function normalizeGraphQLEndpoint(endPoint) {
    if (!endPoint || typeof endPoint !== 'string') {
        return endPoint;
    }
    if (endPoint.startsWith('ws://')) {
        return endPoint.replace('ws://', 'http://');
    }
    if (endPoint.startsWith('wss://')) {
        return endPoint.replace('wss://', 'https://');
    }
    return endPoint;
}

function normalizeGraphQLEndpoints(apiMetadata) {
    if (!apiMetadata?.apiInfo || !apiMetadata?.endPoints) {
        return;
    }
    if (constants.API_TYPE.GRAPHQL !== apiMetadata.apiInfo.apiType) {
        return;
    }
    apiMetadata.endPoints.productionURL = normalizeGraphQLEndpoint(apiMetadata.endPoints.productionURL);
    apiMetadata.endPoints.sandboxURL = normalizeGraphQLEndpoint(apiMetadata.endPoints.sandboxURL);
}

async function allowAPIStatusChange(apiStatus, orgId, apiId) {
    
    if (apiStatus === constants.API_STATUS.CREATED) {

        const subApis = await subDao.listByApi(orgId, apiId);
        if (subApis.length > 0) {
            return false;
        }
    }
    return true;
}

const getAPIMetadata = async (req, res) => {

    const orgId = req.orgId;
    const { apiId } = req.params;
    try {
        const retrievedAPI = await getMetadataFromDB(orgId, apiId);
        if (retrievedAPI !== "") {
            // Create response object
            res.status(200).send(retrievedAPI);
        } else {
            res.status(404).send("API not found");
        }
    } catch (error) {
        logger.error('API metadata retrieval failed', {
            error: error.message,
            stack: error.stack,
            orgId,
            apiId
        });
        util.handleError(res, error);
    }
};

const getMetadataFromDB = async (orgId, apiId) => {

    return await sequelize.transaction({
        timeout: 60000,
    }, async (t) => {
        const retrievedAPI = await apiDao.getByCondition({ ORG_UUID: orgId, UUID: apiId }, t);
        if (retrievedAPI.length > 0) {
            return new APIDTO(retrievedAPI[0]);
        } else {
            return "";
        }
    });
};

const getAllAPIMetadata = async (req, res) => {
    try {
        const orgId = req.orgId;
        const searchTerm = req.query.query;
        const apiName = req.query.apiName;
        const apiVersion = req.query.version;
        const tags = req.query.tags;
        const view = req.query.view;
        const retrievedAPIs = await getMetadataListFromDB(orgId, searchTerm, tags, apiName, apiVersion, view);
        res.status(200).json(util.toPaginatedList(retrievedAPIs, req));
    } catch (error) {
        logger.error('API metadata list retrieval failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.orgId,
            searchTerm: req.query.query,
            apiName: req.query.apiName,
            apiVersion: req.query.version,
            tags: req.query.tags,
            view: req.query.view
        });
        util.handleError(res, error);
    }
};

const getMetadataListFromDB = async (orgId, searchTerm, tags, apiName, apiVersion, viewName) => {
    return await sequelize.transaction({
        timeout: 60000,
    }, async (t) => {
        let retrievedAPIs;
        if (apiName || apiVersion || tags) {
            const condition = {};
            if (apiName) condition.NAME = apiName;
            if (apiVersion) condition.VERSION = apiVersion;
            condition.ORG_UUID = orgId;
            retrievedAPIs = await apiDao.getByCondition(condition, t, tags);
        } else if (searchTerm) {
            retrievedAPIs = await apiDao.search(orgId, searchTerm, viewName, t);
        } else if (viewName) {
            retrievedAPIs = await apiDao.list(orgId, viewName, t);
        }
        // Create response object
        const apiCreationResponse = retrievedAPIs ? retrievedAPIs.map((api) => new APIDTO(api)) : [];
        return apiCreationResponse;
    });
};

const updateAPIMetadata = async (req, res) => {
    const orgId = req.orgId;
    const { apiId } = req.params;
    const userId = util.resolveActor(req);
    logger.info('Updating API metadata', {
        orgId,
        apiId
    });
    let apiMetadata;
    let apiDefinitionFile, apiFileName = "";
    let fullApiBundle;
    const apiArtifactFile = req.files?.artifact?.[0];
    logger.debug('MCP API Definition file', {
        apiFileName,
        hasApiDefinitionFile: !!apiDefinitionFile,
        orgId,
        apiId
    });

    try {
        let artifactApiContent = [];
        let resolvedImageMetadata = {};
        if (apiArtifactFile?.buffer) {
            fullApiBundle = await extractFullApiBundleFromUploadedZip(apiArtifactFile, orgId, apiId);
            apiMetadata = fullApiBundle.apiMetadata;
            const preparedDefinition = prepareApiDefinitionForStorage(
                fullApiBundle.apiDefinitionFileName,
                fullApiBundle.apiDefinitionFile
            );
            apiDefinitionFile = preparedDefinition.apiDefinitionFile;
            apiFileName = preparedDefinition.apiDefinitionFileName;
            artifactApiContent = await extractApiContentFromUploadedZip(apiArtifactFile, orgId, apiId, 'artifact');
            resolvedImageMetadata = buildImageMetadataFromContent(artifactApiContent);
            const filenameToKey = Object.fromEntries(Object.entries(resolvedImageMetadata).map(([key, fileName]) => [fileName, key]));
            artifactApiContent.forEach(file => {
                if (file.type === constants.DOC_TYPES.IMAGES) {
                    file.key = filenameToKey[file.fileName];
                }
            });
        } else if (req.files?.api?.[0]) {
            apiMetadata = parseApiMetadataFromYamlRequest(req);
            if (req.files?.apiDefinition?.[0]) {
                const file = req.files.apiDefinition[0];
                const preparedDefinition = prepareApiDefinitionForStorage(file.originalname, file.buffer);
                apiDefinitionFile = preparedDefinition.apiDefinitionFile;
                apiFileName = preparedDefinition.apiDefinitionFileName;
            }
        } else {
            apiMetadata = JSON.parse(req.body.apiMetadata);
            if (req.files?.apiDefinition?.[0]) {
                const file = req.files.apiDefinition[0];
                const preparedDefinition = prepareApiDefinitionForStorage(file.originalname, file.buffer);
                apiDefinitionFile = preparedDefinition.apiDefinitionFile;
                apiFileName = preparedDefinition.apiDefinitionFileName;
            }
        }

        // Validate input — spec file is optional on update (already stored from create)
        if (!apiMetadata.apiInfo || !apiMetadata.endPoints) {
            throw new Sequelize.ValidationError(
                "Missing or Invalid fields in the request payload"
            );
        }
        const { apiStatus: updateApiStatus, agentVisibility: updateInfoAgentVisibility } = apiMetadata.apiInfo;
        const updateAgentVisibility = apiMetadata.agentVisibility || updateInfoAgentVisibility;
        if (updateApiStatus && !Object.values(constants.API_STATUS).includes(updateApiStatus)) {
            throw new Sequelize.ValidationError(`Invalid apiStatus '${updateApiStatus}'. Must be one of: ${Object.values(constants.API_STATUS).join(', ')}.`);
        }
        if (updateAgentVisibility) {
            const normalizedUpdateAgentVisibility = updateAgentVisibility.toUpperCase();
            if (!Object.values(constants.AGENT_VISIBILITY).includes(normalizedUpdateAgentVisibility)) {
                throw new Sequelize.ValidationError(`Invalid agentVisibility '${updateAgentVisibility}'. Must be one of: ${Object.values(constants.AGENT_VISIBILITY).join(', ')}.`);
            }
            apiMetadata.agentVisibility = normalizedUpdateAgentVisibility;
            apiMetadata.apiInfo.agentVisibility = normalizedUpdateAgentVisibility;
        }

        // Compute added/removed labels diff for YAML and artifact paths
        let existingAPI;
        if (orgId && apiId && Array.isArray(apiMetadata.apiInfo.labels) && (apiArtifactFile?.buffer || req.files?.api?.[0])) {
            existingAPI = await getMetadataFromDB(orgId, apiId);
        }
        if (Array.isArray(apiMetadata.apiInfo.labels) && !apiMetadata.apiInfo.addedLabels && existingAPI !== undefined) {
            const desiredLabels = [...new Set(apiMetadata.apiInfo.labels.map(label => String(label)))];
            const currentLabels = new Set(existingAPI?.apiInfo?.labels || []);
            apiMetadata.apiInfo.addedLabels = desiredLabels.filter(label => !currentLabels.has(label));
            apiMetadata.apiInfo.removedLabels = [...currentLabels].filter(label => !desiredLabels.includes(label));
        }

        apiMetadata.endPoints.productionURL = changeEndpoint(apiMetadata.endPoints.productionURL);
        apiMetadata.endPoints.sandboxURL = changeEndpoint(apiMetadata.endPoints.sandboxURL);
        normalizeGraphQLEndpoints(apiMetadata);
        let allowStatusChange = await allowAPIStatusChange(apiMetadata.apiInfo.apiStatus, orgId, apiId);
        if (!allowStatusChange) {
            throw new CustomError(409, constants.ERROR_MESSAGE.ERR_SUB_EXIST, "API has subscriptions.");
        }
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            // Create apimetadata record
            logger.info('Updating API metadata in database', {
                orgId,
                apiId
            });
            let [updatedRows, updatedAPI] = await apiDao.update(orgId, apiId, apiMetadata, userId, t);
            if (!updatedRows) {
                throw new Sequelize.EmptyResultError("No record found to update");
            }
            if (apiMetadata.apiInfo.addedLabels) {
                const labels = apiMetadata.apiInfo.addedLabels;
                if (!Array.isArray(labels)) {
                    throw new Sequelize.ValidationError(
                        "Missing or Invalid fields in the request payload"
                    );
                }
                await labelDao.createApiMapping(orgId, apiId, labels, userId, t);
                updatedAPI[0].dataValues.addedLabels = apiMetadata.apiInfo.addedLabels;
            }
            if (apiMetadata.apiInfo.removedLabels) {
                const labels = apiMetadata.apiInfo.removedLabels;
                if (!Array.isArray(labels)) {
                    throw new Sequelize.ValidationError(
                        "Missing or Invalid fields in the request payload"
                    );
                }
                const labelDelete = await labelDao.deleteApiMapping(orgId, apiId, labels, t);
                if (labelDelete === 0) {
                    throw new Sequelize.EmptyResultError("API Labels not found to delete");
                }
                updatedAPI[0].dataValues.removedLabels = apiMetadata.apiInfo.removedLabels;
            }
            // Tags are fully replaced on every update, matching the previous TAGS column's overwrite semantics
            await tagDao.replaceApiMapping(orgId, apiId, apiMetadata.apiInfo.tags || [], userId, t);
            if (apiMetadata.subscriptionPlans) {
                const subscriptionPlans = [];
                const apiSubscriptionPlans = apiMetadata.subscriptionPlans;
                if (!Array.isArray(apiSubscriptionPlans)) {
                    throw new Sequelize.ValidationError(
                        "Missing or Invalid fields in the request payload"
                    );
                } else {
                    for (const plan of apiSubscriptionPlans) {
                        const subscriptionPlan = await subscriptionPlanDao.getByName(orgId, plan.planName);
                        if (!subscriptionPlan) {
                            throw new Sequelize.EmptyResultError("Subscription plan not found");
                        } else {
                            subscriptionPlans.push({ apiId: apiId, planId: subscriptionPlan.UUID });
                        }
                    };
                }
                // Get subscription plan IDs and fail if any plan is not found
                await subscriptionPlanDao.updateApiMapping(subscriptionPlans, apiId, userId, t);
                updatedAPI[0].dataValues["DP_SUBSCRIPTION_PLANs"] = await subscriptionPlanDao.listByApi(apiId, t);
            }
            // update api definition file (only when a new file was uploaded)
            if (apiDefinitionFile) {
                const updatedFileCount = await apiFileDao.update(apiDefinitionFile, apiFileName, apiId, orgId,
                    constants.DOC_TYPES.API_DEFINITION, userId, t);
                if (!updatedFileCount) {
                    throw new Sequelize.EmptyResultError("No record found to update");
                }
            }
            // remove docs the user deleted in the wizard
            if (Array.isArray(apiMetadata.docsToRemove)) {
                for (const fileName of apiMetadata.docsToRemove) {
                    await apiFileDao.deleteByFileName(fileName, orgId, apiId, t);
                }
            }
            // upsert newly uploaded documentation files
            if (req.files?.docs) {
                for (const doc of req.files.docs) {
                    await apiFileDao.store(doc.buffer, doc.originalname, apiId, constants.DOC_TYPES.DOC_ID + constants.DOC_TYPES.DOCS.OTHER, userId, t);
                }
            }
            // Update MCP tools schema definition if the API type is MCP
            const hasSchemaDefinitionFile = !!req.files?.schemaDefinition?.[0] || !!fullApiBundle?.schemaDefinitionFile;
            logger.debug('Processing MCP API schema definition', {
                hasSchemaDefinition: hasSchemaDefinitionFile,
                apiType: apiMetadata.apiInfo.apiType,
                apiId
            });
            if (constants.API_TYPE.MCP === apiMetadata.apiInfo.apiType && hasSchemaDefinitionFile) {
                let schemaFile;
                if (req.files?.schemaDefinition?.[0]) {
                    schemaFile = req.files.schemaDefinition[0];
                } else if (fullApiBundle?.schemaDefinitionFile) {
                    schemaFile = {
                        originalname: fullApiBundle.schemaDefinitionFileName,
                        buffer: fullApiBundle.schemaDefinitionFile
                    };
                }
                if (schemaFile) {
                    const schemaDefinition = prepareSchemaDefinitionForStorage(schemaFile.originalname, schemaFile.buffer);
                    logger.debug('Schema definition file received for update', {
                        schemaDefinitionFileSize: schemaDefinition.schemaDefinitionFile.length,
                        schemaFileName: schemaDefinition.schemaDefinitionFileName,
                        apiId
                    });
                    await apiFileDao.upsert(schemaDefinition.schemaDefinitionFile, schemaDefinition.schemaDefinitionFileName, apiId, orgId,
                        constants.DOC_TYPES.SCHEMA_DEFINITION, userId, t);
                    logger.info('Schema definition file updated', {
                        schemaFileName: schemaDefinition.schemaDefinitionFileName,
                        apiId
                    });
                }
            }

            if (constants.API_TYPE.GRAPHQL === apiMetadata.apiInfo.apiType && req.files?.schemaDefinition?.[0]) {
                const file = req.files.schemaDefinition[0];
                const schemaDefinitionFile = file.buffer;
                const schemaFileName = constants.FILE_NAME.API_DEFINITION_GRAPHQL;
                logger.debug('GraphQL schema definition file received for update', {
                    schemaDefinitionFileSize: schemaDefinitionFile.length,
                    schemaFileName,
                    apiId
                });
                await apiFileDao.update(schemaDefinitionFile, schemaFileName, apiId, orgId,
                    constants.DOC_TYPES.API_DEFINITION, userId, t);
                logger.info('GraphQL schema definition file updated', {
                    schemaFileName,
                    apiId
                });
            }

            if (apiArtifactFile?.buffer && artifactApiContent.length > 0) {
                await apiFileDao.upsertMany(artifactApiContent, apiId, orgId, userId, t);
            }
            res.status(200).send(new APIDTO(updatedAPI[0].dataValues));
        });
    } catch (error) {
        logger.error('API metadata update failed', {
            error: error.message,
            stack: error.stack,
            orgId,
            apiId
        });
        util.handleError(res, error);
    }
};

const deleteAPIMetadata = async (req, res) => {
    const orgId = req.orgId;
    const { apiId } = req.params;
    await sequelize.transaction({
        timeout: 60000,
    }, async (t) => {
        try {
            const subApis = await subDao.listByApi(orgId, apiId);
            if (subApis.length > 0) {
                throw new CustomError(409, constants.ERROR_MESSAGE.ERR_SUB_EXIST, "API has subscriptions.");
            }
            const activeKeys = await apiKeyDao.list(orgId, { apiId, status: 'ACTIVE' });
            if (activeKeys.length > 0) {
                throw new CustomError(409, constants.ERROR_MESSAGE.ERR_KEY_EXIST, "API has active keys.");
            }
            const apiDeleteResponse = await apiDao.delete(orgId, apiId, t);
            if (apiDeleteResponse === 0) {
                throw new Sequelize.EmptyResultError("Resource not found to delete");
            } else {
                res.status(200).send("Resouce Deleted Successfully");
            }
        } catch (error) {
            logger.error('API metadata deletion failed', {
                error: error.message,
                stack: error.stack,
                orgId,
                apiId
            });
            util.handleError(res, error);
        }
    });
};

const createAPITemplate = async (req, res) => {
    logger.info('Creating API template...', {
        orgId: req.orgId,
        apiId: req.params.apiId,
        fileName: req.file?.originalname,
    });
    try {
        const orgId = req.orgId;
        const { apiId } = req.params;
        const userId = util.resolveActor(req);
        const zipFilePath = req.file.path;
        const extractPath = path.join("/tmp", orgId + "/" + apiId);
        await fs.mkdir(extractPath, { recursive: true });
        await util.unzipDirectory(zipFilePath, extractPath);

        const apiContentFileName = req.file.originalname.split(".zip")[0];

        // Build complete paths
        const contentPath = path.join(extractPath, apiContentFileName, "content");
        const imagesPath = path.join(extractPath, apiContentFileName, "images");
        const documentPath = path.join(extractPath, apiContentFileName, "documents");

        // Verify directories exist
        try {
            if (fsDir.existsSync(contentPath)) {
                await fs.access(contentPath);
            }
            if (fsDir.existsSync(imagesPath)) {
                await fs.access(imagesPath);
            }
            if (fsDir.existsSync(documentPath)) {
                await fs.access(documentPath);
            }
        } catch (err) {
            logger.error('Error while trying to access directories', {
                error: err.message,
                contentPath,
                imagesPath,
                documentPath,
                orgId,
                apiId
            });
            throw new Error(
                `Required directories not found after extraction. Content path: ${contentPath}, Images path: ${imagesPath}
                , Documents path: ${documentPath}`
            );
        }
        let apiContent = [];

        //get api files
        if (fsDir.existsSync(contentPath)) {
            let apiLanding = await util.getAPIFileContent(contentPath);
            apiContent.push(...apiLanding);
        }
        //get api images
        if (fsDir.existsSync(imagesPath)) {
            const apiImages = await util.getAPIImages(imagesPath);
            apiContent.push(...apiImages);
        }
        //get api documents
        if (fsDir.existsSync(documentPath)) {
            const apiDocuments = await util.readDocFiles(documentPath);
            apiContent.push(...apiDocuments);
        }
        let docMetadata = "";
        if (req.body.docMetadata) {
            docMetadata = JSON.parse(req.body.docMetadata);
            const links = util.getAPIDocLinks(docMetadata);
            apiContent.push(...links);
        }
        let imageMetadata = {};
        if (req.body.imageMetadata) {
            imageMetadata = JSON.parse(req.body.imageMetadata);
        }
        const resolvedImageMetadata = buildImageMetadataFromContent(apiContent, imageMetadata);
        const filenameToKey = Object.fromEntries(Object.entries(resolvedImageMetadata).map(([key, fileName]) => [fileName, key]));
        apiContent.forEach(file => {
            if (file.type === constants.DOC_TYPES.IMAGES) {
                file.key = filenameToKey[file.fileName];
            }
        });
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            //check whether api belongs to given org
            let apiMetadata = await apiDao.get(orgId, apiId, t);

            if (apiMetadata) {
                // Replace any previously stored images with this upload's set
                await apiFileDao.deleteAllByType(constants.DOC_TYPES.IMAGES, apiId, t);
                await apiFileDao.storeMany(apiContent, apiId, userId, t);
            } else {
                throw new Sequelize.ValidationError(constants.ERROR_MESSAGE.API_NOT_IN_ORG);
            }
        });
        await fs.rm(extractPath, { recursive: true, force: true });
        res.status(201).type("application/json").send({ message: "API Template updated successfully" });
    } catch (error) {
        logger.error('API content creation failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.orgId,
            apiId: req.params.apiId,
            fileName: req.file?.originalname,
        });
        util.handleError(res, error);
    }
};

const createAPIContent = async (req, res) => {
    const uploadedFile = req.files?.apiContent?.[0] ?? req.file;
    logger.info('Creating API content...', {
        orgId: req.orgId,
        apiId: req.params.apiId,
        fileName: uploadedFile?.originalname,
    });
    try {
        const orgId = req.orgId;
        const { apiId } = req.params;
        const userId = util.resolveActor(req);
        let apiContent = await extractApiContentFromUploadedZip(uploadedFile, orgId, apiId, 'classic');
        let docMetadata = "";
        if (req.body.docMetadata) {
            docMetadata = JSON.parse(req.body.docMetadata);
            const links = util.getAPIDocLinks(docMetadata);
            apiContent.push(...links);
        }
        let imageMetadata = {};
        if (req.body.imageMetadata) {
            imageMetadata = JSON.parse(req.body.imageMetadata);
        }
        const resolvedImageMetadata = buildImageMetadataFromContent(apiContent, imageMetadata);
        const filenameToKey = Object.fromEntries(Object.entries(resolvedImageMetadata).map(([key, fileName]) => [fileName, key]));
        apiContent.forEach(file => {
            if (file.type === constants.DOC_TYPES.IMAGES) {
                file.key = filenameToKey[file.fileName];
            }
        });
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            //check whether api belongs to given org
            let apiMetadata = await apiDao.get(orgId, apiId, t);

            if (apiMetadata) {
                // Replace any previously stored images with this upload's set
                await apiFileDao.deleteAllByType(constants.DOC_TYPES.IMAGES, apiId, t);
                await apiFileDao.storeMany(apiContent, apiId, userId, t);
            } else {
                throw new Sequelize.ValidationError(constants.ERROR_MESSAGE.API_NOT_IN_ORG);
            }
        });
        res.status(201).type("application/json").send({ message: "API content updated successfully" });
    } catch (error) {
        logger.error('API content creation failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.orgId,
            apiId: req.params.apiId,
            fileName: uploadedFile?.originalname,
        });
        util.handleError(res, error);
    }
};

const updateAPITemplate = async (req, res) => {
    logger.info('Updating API template...', {
        orgId: req.orgId,
        apiId: req.params.apiId,
        fileName: req.file?.originalname
    });
    try {
        const orgId = req.orgId;
        const { apiId } = req.params;
        const userId = util.resolveActor(req);
        let imageMetadata;
        if (req.body.imageMetadata) {
            imageMetadata = JSON.parse(req.body.imageMetadata);
        }
        const zipFilePath = req.file.path;
        const extractPath = path.join("/tmp", orgId + "/" + apiId);
        await fs.mkdir(extractPath, { recursive: true });
        await util.unzipDirectory(zipFilePath, extractPath);
        const apiContentFileName = req.file.originalname.split(".zip")[0];

        // Build complete paths
        const contentPath = path.join(extractPath, apiContentFileName, "content");
        const imagesPath = path.join(extractPath, apiContentFileName, "images");
        const documentPath = path.join(extractPath, apiContentFileName, "documents");

        // Verify directories exist
        try {
            if (fsDir.existsSync(contentPath)) {
                await fs.access(contentPath);
            }
            if (fsDir.existsSync(imagesPath)) {
                await fs.access(imagesPath);
            }
            if (fsDir.existsSync(documentPath)) {
                await fs.access(documentPath);
            }
        } catch (err) {
            logger.error('Error accessing directories during API template update', {
                error: err.message,
                contentPath,
                imagesPath,
                documentPath,
                orgId: req.orgId,
                apiId: req.params.apiId
            });
            throw new Error(
                `Required directories not found after extraction. Content path: ${contentPath}, Images path: ${imagesPath},
                Documents path: ${documentPath}`
            );
        }
        let apiContent = [];
        //get api files
        if (fsDir.existsSync(contentPath)) {
            let apiLanding = await util.getAPIFileContent(contentPath);
            apiContent.push(...apiLanding);
        }
        //get api images
        if (fsDir.existsSync(imagesPath)) {
            const apiImages = await util.getAPIImages(imagesPath);
            apiContent.push(...apiImages);
        }
        //get api documents
        if (fsDir.existsSync(documentPath)) {
            const apiDocuments = await util.readDocFiles(documentPath);
            apiContent.push(...apiDocuments);
        }

        if (req.body.docMetadata) {
            const docMetadata = JSON.parse(req.body.docMetadata);
            const links = util.getAPIDocLinks(docMetadata);
            apiContent.push(...links);
        }
        const filenameToKey = Object.fromEntries(Object.entries(imageMetadata || {}).map(([key, fileName]) => [fileName, key]));
        apiContent.forEach(file => {
            if (file.type === constants.DOC_TYPES.IMAGES) {
                file.key = filenameToKey[file.fileName];
            }
        });
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            //check whether api belongs to given org
            const apiMetadata = await apiDao.get(orgId, apiId, t);
            if (apiMetadata) {
                // Update API files (including images, keyed by their named slot)
                await apiFileDao.upsertMany(apiContent, apiId, orgId, userId, t);
            } else {
                throw new Sequelize.ValidationError(constants.ERROR_MESSAGE.API_NOT_IN_ORG);
            }
        });
        await fs.rm(extractPath, { recursive: true, force: true });
        res.status(201).send({ message: "API Files updated successfully" });
    } catch (error) {
        logger.error('API content update failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.orgId,
            apiId: req.params.apiId,
            fileName: req.file?.originalname
        });
        util.handleError(res, error);
    }
};

const updateAPIContent = async (req, res) => {
    const uploadedFile = req.files?.apiContent?.[0] ?? req.file;
    logger.info('Updating API content...', {
        orgId: req.orgId,
        apiId: req.params.apiId,
        fileName: uploadedFile?.originalname
    });
    try {
        const orgId = req.orgId;
        const { apiId } = req.params;
        const userId = util.resolveActor(req);
        let imageMetadata;
        if (req.body.imageMetadata) {
            imageMetadata = JSON.parse(req.body.imageMetadata);
        }
        let apiContent = await extractApiContentFromUploadedZip(uploadedFile, orgId, apiId, 'classic');

        if (req.body.docMetadata) {
            const docMetadata = JSON.parse(req.body.docMetadata);
            const links = util.getAPIDocLinks(docMetadata);
            apiContent.push(...links);
        }
        const resolvedImageMetadata = buildImageMetadataFromContent(apiContent, imageMetadata || {});
        const filenameToKey = Object.fromEntries(Object.entries(resolvedImageMetadata).map(([key, fileName]) => [fileName, key]));
        apiContent.forEach(file => {
            if (file.type === constants.DOC_TYPES.IMAGES) {
                file.key = filenameToKey[file.fileName];
            }
        });
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            //check whether api belongs to given org
            const apiMetadata = await apiDao.get(orgId, apiId, t);
            if (apiMetadata) {
                // Update API files (including images, keyed by their named slot)
                await apiFileDao.upsertMany(apiContent, apiId, orgId, userId, t);
            } else {
                throw new Sequelize.ValidationError(constants.ERROR_MESSAGE.API_NOT_IN_ORG);
            }
        });
        res.status(201).send({ message: "API Files updated successfully" });
    } catch (error) {
        logger.error('API content update failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.orgId,
            apiId: req.params.apiId,
            fileName: uploadedFile?.originalname
        });
        util.handleError(res, error);
    }
};

const getAPIFile = async (req, res) => {

    const orgId = req.orgId;
    const { apiId } = req.params;
    const apiFileName = req.query.fileName;
    const type = req.query.type;
    let apiFileResponse = "";
    let apiFile;
    let contentType = "";
    try {
        const fileExtension = path.extname(apiFileName).toLowerCase();
        apiFileResponse = await apiFileDao.get(apiFileName, type, orgId, apiId);
        if (apiFileResponse) {
            apiFile = apiFileResponse.FILE_CONTENT;
            //convert to text to check if link
            const textContent = new TextDecoder().decode(apiFile);
            if (textContent.startsWith("http") || textContent.startsWith("https")) {
                apiFile = textContent;
                contentType = constants.MIME_TYPES.TEXT;
            } else if (util.isTextFile(fileExtension)) {
                contentType = util.retrieveContentType(apiFileName, constants.TEXT)
            } else {
                contentType = util.retrieveContentType(apiFileName, constants.IMAGE);
            }
            res.set(constants.MIME_TYPES.CONYEMT_TYPE, contentType);

            if (apiFileResponse) {
                // Send file content as text
                return res.status(200).send(Buffer.isBuffer(apiFile) ? apiFile : constants.CHARSET_UTF8);
            } else {
                res.status(404).send("API File not found");
            }
        }
    } catch (error) {
        logger.error('API content retrieval failed', {
            error: error.message,
            stack: error.stack,
            orgId,
            apiId,
            fileName: req.query.fileName
        });
        util.handleError(res, error);
    }
};

const getAPIDocTypes = async (orgId, apiId) => {

    try {
        const docTypeResponse = await apiFileDao.getDocTypes(orgId, apiId);
        const apiCreationResponse = docTypeResponse.map((doc) => new APIDocDTO(doc.dataValues));
        return apiCreationResponse;
    } catch (error) {
        logger.error('API doc types retrieval failed', {
            error: error.message,
            stack: error.stack,
            orgId: orgId,
            apiId: apiId
        });
        // Note: This function doesn't have access to res, so we can't call util.handleError
        throw error;
    }
}

const listApiDocs = async (req, res) => {
    const orgId = req.orgId;
    const { apiId } = req.params;
    try {
        const names = await apiFileDao.listDocNames(orgId, apiId);
        res.status(200).json(names.map(fileName => ({ fileName })));
    } catch (error) {
        logger.error('API doc list failed', { error: error.message, orgId, apiId });
        util.handleError(res, error);
    }
};

const deleteAPIFile = async (req, res) => {
    logger.info('Deleting API file...', {
        orgId: req.orgId,
        apiId: req.params.apiId,
        fileName: req.query.fileName,
        fileType: req.query.type
    });
    const orgId = req.orgId;
    const { apiId } = req.params;
    const apiFileName = req.query.fileName;
    const fileType = req.query.type;
    try {
        let apiFileResponse;
        if (apiFileName) {
            apiFileResponse = await apiFileDao.delete(apiFileName, fileType, orgId, apiId);
        } else {
            apiFileResponse = await apiFileDao.deleteAll(fileType, orgId, apiId);
        }
        if (!apiFileResponse) {
            res.status(204).send();
        } else {
            res.status(404).send("API Content not found");
        }
    } catch (error) {
        logger.error('API content deletion failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.orgId,
            apiId: req.params?.apiId
        });
        util.handleError(res, error);
    }
};

const addSubscriptionPlans = async (req, res) => {
    if (req.files?.subscriptionPlan?.[0]) {
        try {
            const plans = parseSubscriptionPlansFromYamlFile(req.files.subscriptionPlan[0].buffer);
            req.body = plans.length === 1 ? plans[0] : plans;
        } catch (error) {
            return util.handleError(res, error);
        }
    }
    if (Array.isArray(req.body)) {
        await createSubscriptionPlans(req, res);
    } else {
        await createSubscriptionPlan(req, res);
    }
}

const putSubscriptionPlans = async (req, res) => {
    if (req.files?.subscriptionPlan?.[0]) {
        try {
            const plans = parseSubscriptionPlansFromYamlFile(req.files.subscriptionPlan[0].buffer);
            req.body = plans.length === 1 ? plans[0] : plans;
        } catch (error) {
            return util.handleError(res, error);
        }
    }
    if (Array.isArray(req.body)) {
        await updateSubscriptionPlans(req, res);
    } else {
        await updateSubscriptionPlan(req, res);
    }
}

const createSubscriptionPlan = async (req, res) => {
    const orgId = req.orgId;
    const subscriptionPlan = req.body;
    const userId = util.resolveActor(req);
    logger.info('Creating subscription plan...', {
        orgId
    });

    if (!subscriptionPlan || typeof subscriptionPlan !== "object") {
        return res.status(400).json({ message: "Request body is missing or invalid" });
    }

    const validTypes = ["requestcount", "eventcount"];
    if (!subscriptionPlan.type || typeof subscriptionPlan.type !== 'string' || !validTypes.includes(subscriptionPlan.type.toLowerCase())) {
        return res.status(400).json({ message: "Invalid or missing subscription plan type" });
    }

    try {
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            const subscriptionPlanResponse = await subscriptionPlanDao.create(orgId, subscriptionPlan, userId, t);
            if (subscriptionPlanResponse) {
                logger.info('Created subscription plan', {
                    orgId
                });
                res.status(201).send(new subscriptionPlanDTO(subscriptionPlanResponse));
            } else {
                throw new CustomError(500, constants.ERROR_CODE[500], constants.ERROR_MESSAGE.SUBSCRIPTION_PLAN_CREATE_ERROR);
            }
        });
    } catch (error) {
        logger.error('subscription plan create error failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
};

const createSubscriptionPlans = async (req, res) => {
    try {
        if (config.generateDefaultSubPlans) {
            const msg = "Bulk creation of subscription plans is not allowed because 'generateDefaultSubPlans' is enabled in the Developer Portal.";
            logger.info(msg, {
                orgId: req.orgId
            });
            res.status(200).json({ message: msg });
        } else {
            const orgId = req.orgId;
            const subscriptionPlans = req.body;
            const userId = util.resolveActor(req);

            if (!Array.isArray(subscriptionPlans) || subscriptionPlans.length === 0) {
                return res.status(400).json({ message: "Missing or invalid fields in the request payload" });
            }

            const createdPlans = [];

            await sequelize.transaction({
                timeout: 60000,
            }, async (t) => {
                // TODO: Try using SubscriptionPlan.bulkCreate() once Table is finalised and manipulating each data is not needed
                for (const plan of subscriptionPlans) {
                    if (typeof plan.type !== 'string') {
                        throw new CustomError(400, constants.ERROR_CODE[400], 'subscriptionPlan.type must be a string');
                    }
                    if (plan.type.toLowerCase() == "requestcount" || plan.type.toLowerCase() == "eventcount") {
                        const created = await subscriptionPlanDao.create(orgId, plan, userId, t);
                        if (!created) {
                            throw new CustomError(
                                500,
                                constants.ERROR_CODE[500],
                                `Failed to create plan: ${plan.planName || "unknown"}`
                            );
                        }
                        createdPlans.push(new subscriptionPlanDTO(created));
                    } else {
                        throw new CustomError(400, constants.ERROR_CODE[400], `Unsupported plan type: ${plan.type}`);
                    }
                }
            });
            logger.info('Created subscription plans', {
                orgId
            });
            res.status(201).send(createdPlans);
        }
    } catch (error) {
        logger.error('subscription plan create error failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.orgId
        });
        util.handleError(res, error);
    }
};

const updateSubscriptionPlan = async (req, res) => {
    const orgId = req.orgId;
    logger.info('Updating subscription plan...', {
        orgId
    });
    const subscriptionPlan = req.body;
    const userId = util.resolveActor(req);

    if (!subscriptionPlan || typeof subscriptionPlan !== "object") {
        return res.status(400).json({ message: "Request body is missing or invalid" });
    }

    const validTypes = ["requestcount", "eventcount"];
    if (!subscriptionPlan.type || typeof subscriptionPlan.type !== 'string' || !validTypes.includes(subscriptionPlan.type.toLowerCase())) {
        return res.status(400).json({ message: "Invalid or missing subscription plan type" });
    }
    
    try {
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            const { subscriptionPlanResponse, statusCode } =  await subscriptionPlanDao.put(orgId, subscriptionPlan, userId, t);
            if (subscriptionPlanResponse) {
                res.status(statusCode).send(new subscriptionPlanDTO(subscriptionPlanResponse));
            } else {
                throw new CustomError(404, constants.ERROR_CODE[404], constants.ERROR_MESSAGE.SUBSCRIPTION_PLAN_NOT_FOUND);
            }
        });
    } catch (error) {
        logger.error('subscription plan not found failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.orgId
        });
        util.handleError(res, error);
    }
};

const updateSubscriptionPlans = async (req, res) => {
    try {
        if (config.generateDefaultSubPlans) {
            const msg = "Bulk updating of subscription plans is not allowed because 'generateDefaultSubPlans' is enabled in the Developer Portal.";
            logger.info(msg, {
                orgId: req.orgId
            });
            res.status(200).json({ message: msg });
        } else {
            const orgId = req.orgId;
            const subscriptionPlans = req.body;
            const userId = util.resolveActor(req);

            if (!Array.isArray(subscriptionPlans) || subscriptionPlans.length === 0) {
                return res.status(400).json({ message: "Missing or invalid fields in the request payload" });
            }

            const updatedPlans = [];

            await sequelize.transaction({
                timeout: 60000,
            }, async (t) => {
                for (const plan of subscriptionPlans) {
                    if (typeof plan.type !== 'string') {
                        throw new CustomError(400, constants.ERROR_CODE[400], 'subscriptionPlan.type must be a string');
                    }
                    if (plan.type.toLowerCase() == "requestcount" || plan.type.toLowerCase() == "eventcount") {
                        const created = await subscriptionPlanDao.put(orgId, plan, userId, t);
                        if (!created) {
                            throw new CustomError(
                                500,
                                constants.ERROR_CODE[500],
                                `Failed to create plan: ${plan.planName || "unknown"}`
                            );
                        }
                        updatedPlans.push(new subscriptionPlanDTO(created.subscriptionPlanResponse));
                    } else {
                        throw new CustomError(400, constants.ERROR_CODE[400], `Unsupported plan type: ${plan.type}`);
                    }
                }
            });

            res.status(201).send(updatedPlans);
        }
    } catch (error) {
        logger.error('subscription plan create error failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.orgId
        });
        util.handleError(res, error);
    }
};

const deleteSubscriptionPlan = async (req, res) => {
    const orgId = req.orgId;
    const { planId } = req.params;
    logger.info('Deleting subscription plan...', {
        orgId,
        planId
    });
    try {
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            const deleteCount = await subscriptionPlanDao.deleteById(orgId, planId, t);
            if (deleteCount === 0) {
                throw new CustomError(404, constants.ERROR_CODE[404], constants.ERROR_MESSAGE.SUBSCRIPTION_PLAN_NOT_FOUND);
            } else {
                res.status(204).send();
            }
        });
    } catch (error) {
        logger.error('subscription plan delete error failed', {
            error: error.message,
            stack: error.stack,
            orgId,
            planId
        });
        util.handleError(res, error);
    }
};

const getSubscriptionPlan = async (req, res) => {

    const orgId = req.orgId;

    const { planId } = req.params;

    try {
        const subscriptionPlanResponse = await subscriptionPlanDao.get(planId, orgId);
        if (subscriptionPlanResponse) {
            res.status(200).send(new subscriptionPlanDTO(subscriptionPlanResponse));
        } else {
            throw new CustomError(404, constants.ERROR_CODE[404], constants.ERROR_MESSAGE.SUBSCRIPTION_PLAN_NOT_FOUND);
        }
    } catch (error) {
        logger.error('subscription plan not found failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
};

// Lists subscription plans for an org. With ?name=<exact>, returns an array
// containing the single matching plan (or empty array) — name is unique per
// org. Without it, returns all plans for the org.
const listSubscriptionPlans = async (req, res) => {

    const orgId = req.orgId;
    const { name } = req.query;

    try {
        let plans;
        if (name) {
            const plan = await subscriptionPlanDao.getByName(orgId, name);
            plans = plan ? [plan] : [];
        } else {
            plans = await subscriptionPlanDao.list(orgId);
        }
        res.status(200).json(util.toPaginatedList(plans.map((plan) => new subscriptionPlanDTO(plan)), req));
    } catch (error) {
        logger.error('subscription plan list failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
};

const createLabels = async (req, res) => {

    const orgId = req.orgId;
    const labels = req.body;
    const userId = util.resolveActor(req);
    try {
        await labelDao.createMany(orgId, labels, userId);
        res.status(201).send(labels);
    } catch (error) {
        logger.error('label create error failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
}

const updateLabel = async (req, res) => {

    const orgId = req.orgId;
    const labels = req.body;
    const userId = util.resolveActor(req);
    try {
        for (const label of labels) {
            await labelDao.update(orgId, label, userId);
        };
        res.status(201).send(labels);
    } catch (error) {
        logger.error('label update error failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
}

const deleteLabels = async (req, res) => {

    const orgId = req.orgId;
    const labelNames = req.query.names;
    const labelList = labelNames.split(",");
    try {
        await labelDao.delete(orgId, labelList);
        res.status(204).send();
    } catch (error) {
        logger.error('label delete error failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
}

const retrieveLabels = async (req, res) => {

    const orgId = req.orgId;
    try {
        const labels = await getOrgLabels(orgId);
        res.status(200).json(util.toPaginatedList(labels, req));
    } catch (error) {
        logger.error('label retrieve error failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
}

const getOrgLabels = async (orgId) => {

    try {
        const labels = await labelDao.list(orgId);
        return labels.map((label) => new LabelDTO(label));
    } catch (error) {
        logger.error('label update error failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
}

const addView = async (req, res) => {

    const orgId = req.orgId;
    const labels = req.body.labels;
    const userId = util.resolveActor(req);
    await sequelize.transaction({
        timeout: 60000,
    }, async (t) => {
        try {
            const viewResponse = await viewDao.create(orgId, req.body, userId, t);
            const viewId = viewResponse.dataValues.UUID;
            await viewDao.addLabels(orgId, viewId, labels, userId, t);
            res.status(201).send({ message: "View added successfully" });
        } catch (error) {
            logger.error('view create error failed', {
                error: error.message,
                stack: error.stack,
                orgId
            });
            util.handleError(res, error);
        }
    });
}

const updateView = async (req, res) => {

    const orgId = req.orgId;
    const removedLabels = req.body.removedLabels ? req.body.removedLabels : [];
    const addedLabels = req.body.addedLabels ? req.body.addedLabels : [];
    const viewName = req.params.viewName;
    const userId = util.resolveActor(req);
    try {
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {

            let viewId = "";
            if (req.body.name) {
                let viewResponse = await viewDao.update(orgId, viewName, req.body.name, userId, t);
                viewId = viewResponse.dataValues.UUID;
            }
            if (removedLabels.length !== 0 || addedLabels.length !== 0) {
                viewId = viewId ? viewId : await viewDao.getId(orgId, viewName, t);
            }
            if (removedLabels.length !== 0) {
                await viewDao.deleteLabels(orgId, viewId, removedLabels, t);
            }
            if (addedLabels.length !== 0) {
                await viewDao.addLabels(orgId, viewId, addedLabels, userId, t);
            }
            res.status(200).send(req.body);
        });
    } catch (error) {
        logger.error('view update error failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
}

const deleteView = async (req, res) => {

    const orgId = req.orgId;
    const name = req.params.viewName;
    try {
        const viewDelete = await viewDao.delete(orgId, name);
        if (viewDelete === 0) {
            throw new Sequelize.EmptyResultError("Resource not found to delete");
        } else {
            res.status(204).send("View Deleted Successfully");
        }
    } catch (error) {
        logger.error('view delete error failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
}

const getView = async (req, res) => {

    const orgId = req.orgId;
    const name = req.params.viewName;
    try {
        const view = await getViewInfo(orgId, name);
        if (view) {
            res.status(200).send(view);
        } else {
            res.status(404).send(`View ${name} not found`);
        }
    } catch (error) {
        logger.error('view retrieve error failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
}

const getViewInfo = async (orgId, name) => {

    const view = await viewDao.get(orgId, name);
    if (view.dataValues) {
        return new ViewDTO(view.dataValues);
    } else {
        return null;
    }
}

const getAllViews = async (req, res) => {

    const orgId = req.orgId;
    try {
        const views = await getViewsFromDB(orgId);
        return res.status(200).json(util.toPaginatedList(views, req));
    } catch (error) {
        logger.error('view retrieve error failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
}

const getViewsFromDB = async (orgId) => {

    const views = await viewDao.list(orgId);
    if (views.length > 0) {
        return views.map((view) => new ViewDTO(view));
    } else {
        return [];
    }
}

const collectWebContentFiles = async (webPath) => {
    const files = await fs.readdir(webPath, { withFileTypes: true });
    const contentFiles = [];
    for (const file of files) {
        if (!file.isFile() || file.name === '.DS_Store') {
            continue;
        }
        const filePath = path.join(webPath, file.name);
        const fileExtension = path.extname(file.name).toLowerCase();
        if (util.isTextFile(fileExtension)) {
            const content = await fs.readFile(filePath, constants.CHARSET_UTF8);
            contentFiles.push({ fileName: file.name, content: content, type: constants.DOC_TYPES.API_LANDING });
        } else if (util.isImageFile(fileExtension)) {
            const content = await fs.readFile(filePath);
            contentFiles.push({ fileName: file.name, content: content, type: constants.DOC_TYPES.IMAGES });
        }
    }
    return contentFiles;
};

const buildImageMetadataFromContent = (contentFiles = [], providedImageMetadata = {}) => {
    const resolvedImageMetadata = { ...(providedImageMetadata || {}) };

    for (const file of contentFiles) {
        if (file?.type !== constants.DOC_TYPES.IMAGES || !file.fileName) {
            continue;
        }
        const imageTag = path.parse(file.fileName).name;
        if (imageTag && !resolvedImageMetadata[imageTag]) {
            resolvedImageMetadata[imageTag] = file.fileName;
        }
    }

    return resolvedImageMetadata;
};

async function resolveZipRootPath(extractPath) {
    const entries = await fs.readdir(extractPath, { withFileTypes: true });
    const relevantEntries = entries.filter(entry => entry.name !== '.DS_Store' && entry.name !== '__MACOSX');
    if (relevantEntries.length === 1 && relevantEntries[0].isDirectory()) {
        return path.join(extractPath, relevantEntries[0].name);
    }
    return extractPath;
}

async function extractApiContentFromUploadedZip(zipFile, orgId, apiId, mode = 'classic') {
    if (!zipFile) {
        throw new Sequelize.ValidationError("Missing required zip file");
    }

    const zipFileName = path.basename(String(zipFile.originalname || ''));
    if (!zipFileName?.toLowerCase().endsWith('.zip')) {
        throw new Sequelize.ValidationError('Invalid zip file. Expected a .zip file');
    }

    const extractionKey = `${orgId || 'org'}-${apiId || 'api'}-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
    const tempBasePath = path.join('/tmp', 'api-content-upload', extractionKey);
    const extractPath = path.join(tempBasePath, 'extracted');
    const tempZipPath = path.join(tempBasePath, `${extractionKey}.zip`);

    try {
        await fs.mkdir(extractPath, { recursive: true });

        if (zipFile.path) {
            await util.unzipDirectory(zipFile.path, extractPath);
        } else if (zipFile.buffer) {
            await fs.writeFile(tempZipPath, zipFile.buffer);
            await util.unzipDirectory(tempZipPath, extractPath);
        } else {
            throw new Sequelize.ValidationError('Invalid zip input. Missing file path or buffer');
        }

        const rootPath = await resolveZipRootPath(extractPath);

        const webPath = path.join(rootPath, constants.ARTIFACT_DIR.WEB);
        const docsPath = path.join(rootPath, constants.ARTIFACT_DIR.DOCS);
        const hasWebDir = fsDir.existsSync(webPath);
        const hasDocsDir = fsDir.existsSync(docsPath);

        if (!hasWebDir && !hasDocsDir) {
            if (mode === 'artifact') {
                return [];
            }
            throw new Sequelize.ValidationError('Missing required directories in uploaded zip. At least one of web or docs is required at root level');
        }

        const apiContent = [];
        if (hasWebDir) {
            await fs.access(webPath);
            const apiWebFiles = await collectWebContentFiles(webPath);
            apiContent.push(...apiWebFiles);
        }
        if (hasDocsDir) {
            await fs.access(docsPath);
            const apiDocuments = await util.readDocFiles(docsPath, '', true);
            apiContent.push(...apiDocuments);
        }

        return apiContent;
    } catch (error) {
        if (error instanceof Sequelize.ValidationError) {
            throw error;
        }
        throw new Sequelize.ValidationError(`Invalid zip file: ${error.message}`);
    } finally {
        await fs.rm(tempBasePath, { recursive: true, force: true });
        // Clean up the original upload file when multer saved it to disk (zipFile.path)
        if (zipFile.path) {
            await fs.rm(zipFile.path, { force: true });
        }
    }
}

async function extractFullApiBundleFromUploadedZip(zipFile, orgId, apiId) {
    if (!zipFile?.buffer) {
        throw new Sequelize.ValidationError("Missing required multipart file field: 'artifact'");
    }

    if (!zipFile.originalname?.toLowerCase().endsWith('.zip')) {
        throw new Sequelize.ValidationError("Invalid artifact file. Expected a .zip file");
    }

    const extractionKey = `${orgId || 'org'}-${apiId || 'api'}-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
    const tempBasePath = path.join('/tmp', 'api-content-upload', extractionKey);
    const tempZipPath = path.join(tempBasePath, 'apiContent.zip');
    const extractPath = path.join(tempBasePath, 'extracted');

    try {
        await fs.mkdir(extractPath, { recursive: true });
        await fs.writeFile(tempZipPath, zipFile.buffer);
        await util.unzipDirectory(tempZipPath, extractPath);

        const rootPath = await resolveZipRootPath(extractPath);
        const metadataFilePath = await util.findFileByNameRecursive(rootPath, ['api.yaml', 'mcp.yaml', 'devportal.yaml']);
        if (!metadataFilePath) {
            throw new Sequelize.ValidationError("Invalid full API zip: missing api.yaml, mcp.yaml or devportal.yaml");
        }

        const definitionFilePath = await util.findFileByNameRecursive(rootPath, [
            'definition.yaml',
            'definition.yml',
            'definition.json',
            'apiDefinition.yaml',
            'apiDefinition.yml',
            'apiDefinition.json',
        ]);
        if (!definitionFilePath) {
            throw new Sequelize.ValidationError("Invalid full API zip: missing definition file (definition.yaml/yml/json)");
        }

        const apiMetadataBuffer = await fs.readFile(metadataFilePath);
        const apiMetadata = parseApiMetadataFromYamlFile(path.basename(metadataFilePath), apiMetadataBuffer);
        const apiDefinitionFile = await fs.readFile(definitionFilePath);
        const apiDefinitionFileName = path.basename(definitionFilePath);

        const schemaDefinitionFilePath = await util.findFileByNameRecursive(rootPath, [
            constants.FILE_NAME.SCHEMA_DEFINITION_FILE_NAME,
            constants.FILE_NAME.SCHEMA_DEFINITION_YAML_FILE_NAME,
        ]);
        let schemaDefinitionFile;
        let schemaDefinitionFileName;
        if (schemaDefinitionFilePath) {
            schemaDefinitionFile = await fs.readFile(schemaDefinitionFilePath);
            schemaDefinitionFileName = path.basename(schemaDefinitionFilePath);
        }

        return {
            apiMetadata,
            apiDefinitionFile,
            apiDefinitionFileName,
            schemaDefinitionFile,
            schemaDefinitionFileName,
        };
    } catch (error) {
        if (error instanceof Sequelize.ValidationError) {
            throw error;
        }
        throw new Sequelize.ValidationError(`Invalid artifact zip file: ${error.message}`);
    } finally {
        await fs.rm(tempBasePath, { recursive: true, force: true });
    }
}

function mapDevportalYamlToApiMetadata(parsedYaml) {
    if (!parsedYaml || typeof parsedYaml !== 'object') {
        throw new Sequelize.ValidationError('Invalid API YAML content');
    }
    const metadata = parsedYaml.metadata || {};
    const spec = parsedYaml.spec || {};
    const apiType = util.resolveApiType(spec.type);
    const apiStatus = spec.status || constants.API_STATUS.PUBLISHED;
    if (!Object.values(constants.API_STATUS).includes(apiStatus)) {
        throw new Sequelize.ValidationError(`Invalid API status '${apiStatus}'. Must be one of: ${Object.values(constants.API_STATUS).join(', ')}.`);
    }
    const agentVisibility = (spec.agentVisibility || constants.AGENT_VISIBILITY.VISIBLE).toUpperCase();
    if (!Object.values(constants.AGENT_VISIBILITY).includes(agentVisibility)) {
        throw new Sequelize.ValidationError(`Invalid agentVisibility '${spec.agentVisibility}'. Must be one of: ${Object.values(constants.AGENT_VISIBILITY).join(', ')}.`);
    }
    const endpoints = spec.endpoints || {};
    const businessInformation = spec.businessInformation || {};

    const subscriptionPlans = util.normalizeStringArray(spec.subscriptionPlans)
        .map(planName => ({ planName }));

    return {
        apiInfo: {
            apiName: spec.displayName,
            apiVersion: spec.version,
            apiDescription: spec.description,
            referenceId: spec.referenceId,
            apiHandle: metadata.name,
            apiType,
            apiStatus,
            agentVisibility,
            tags: util.normalizeStringArray(spec.tags),
            labels: util.normalizeStringArray(spec.labels),
            owners: {
                businessOwner: businessInformation.businessOwner,
                businessOwnerEmail: businessInformation.businessOwnerEmail,
                technicalOwner: businessInformation.technicalOwner,
                technicalOwnerEmail: businessInformation.technicalOwnerEmail,
            },
        },
        endPoints: {
            sandboxURL: endpoints.sandboxUrl,
            productionURL: endpoints.productionUrl,
        },
        subscriptionPlans,
    };
}

function parseApiMetadataFromYamlFile(fileName, fileBuffer) {
    const allowedMetadataFileNames = new Set(['api.yaml', 'mcp.yaml', 'devportal.yaml']);
    if (!allowedMetadataFileNames.has(String(fileName).toLowerCase())) {
        throw new Sequelize.ValidationError("Invalid metadata file name. Expected 'api.yaml', 'mcp.yaml' or 'devportal.yaml'");
    }

    let parsedYaml;
    try {
        parsedYaml = yaml.load(fileBuffer.toString(constants.CHARSET_UTF8));
    } catch (e) {
        throw new Sequelize.ValidationError(`Invalid API YAML file: ${e.message}`);
    }

    return mapDevportalYamlToApiMetadata(parsedYaml);
}

function parseApiMetadataFromYamlRequest(req) {
    const apiFile = req.files?.api?.[0];
    if (!apiFile?.buffer) {
        throw new Sequelize.ValidationError(
            "Missing required multipart file field: 'api'"
        );
    }

    return parseApiMetadataFromYamlFile(apiFile.originalname, apiFile.buffer);
}

function mapYamlToSubscriptionPlan(item) {
    const { metadata = {}, spec = {} } = item;
    return {
        planName: metadata.name,
        displayName: spec.displayName,
        description: spec.description,
        refId: spec.refId,
        type: spec.type,
        requestCount: spec.requestCount,
        eventCount: spec.eventCount,
    };
}

function parseSubscriptionPlansFromYamlFile(fileBuffer) {
    let parsed;
    try {
        parsed = yaml.load(fileBuffer.toString(constants.CHARSET_UTF8));
    } catch (e) {
        throw new Sequelize.ValidationError(`Invalid subscription plan YAML file: ${e.message}`);
    }

    if (!parsed || typeof parsed !== 'object') {
        throw new Sequelize.ValidationError('Subscription plan YAML file is empty or invalid');
    }

    const kind = parsed.kind;
    if (kind === 'SubscriptionPlan') {
        return [mapYamlToSubscriptionPlan(parsed)];
    } else if (kind === 'SubscriptionPlanList') {
        if (!Array.isArray(parsed.items) || parsed.items.length === 0) {
            throw new Sequelize.ValidationError("SubscriptionPlanList must have a non-empty 'items' array");
        }
        return parsed.items.map(mapYamlToSubscriptionPlan);
    } else {
        throw new Sequelize.ValidationError(
            `Unknown subscription plan YAML kind '${kind}'. Expected 'SubscriptionPlan' or 'SubscriptionPlanList'`
        );
    }
}

function prepareApiDefinitionForStorage(fileName, fileBuffer) {
    const sanitizedFileName = path.basename(String(fileName || ''));
    const extension = path.extname(String(fileName || '')).toLowerCase();
    const fileContent = fileBuffer.toString(constants.CHARSET_UTF8);
    if (!sanitizedFileName) {
        throw new Sequelize.ValidationError('Invalid API definition file name');
    }

    if (extension === '.json') {
        try {
            JSON.parse(fileContent);
        } catch (e) {
            throw new Sequelize.ValidationError(`Invalid API definition JSON file: ${e.message}`);
        }
    } else if (extension === '.yaml' || extension === '.yml') {
        try {
            const parsedDefinition = yaml.load(fileContent);
            if (parsedDefinition === undefined) {
                throw new Sequelize.ValidationError('Invalid API definition YAML file: empty content');
            }
        } catch (e) {
            if (e instanceof Sequelize.ValidationError) {
                throw e;
            }
            throw new Sequelize.ValidationError(`Invalid API definition YAML file: ${e.message}`);
        }
    } else if (extension === '.xml' || extension === '.wsdl') {
        if (!fileContent || fileContent.trim() === '') {
            throw new Sequelize.ValidationError('Invalid API definition XML file: empty content');
        }
    } else {
        throw new Sequelize.ValidationError("Invalid API definition file type. Expected '.json', '.yaml', '.yml', '.xml', or '.wsdl'");
    }

    return {
        apiDefinitionFile: fileBuffer,
        apiDefinitionFileName: sanitizedFileName,
    };
}

function validateSchemaDefinitionFileName(fileName) {
    const sanitizedFileName = path.basename(String(fileName || ''));
    const extension = path.extname(sanitizedFileName).toLowerCase();
    if (extension !== '.json' && extension !== '.yaml') {
        throw new Sequelize.ValidationError("Invalid schema definition file type. Expected '.json' or '.yaml'");
    }
    return sanitizedFileName;
}

function prepareSchemaDefinitionForStorage(fileName, fileBuffer) {
    const sanitizedFileName = validateSchemaDefinitionFileName(fileName);
    const fileContent = fileBuffer.toString(constants.CHARSET_UTF8);
    if (sanitizedFileName.toLowerCase().endsWith('.json')) {
        try {
            JSON.parse(fileContent);
        } catch (e) {
            throw new Sequelize.ValidationError(`Invalid schema definition JSON file: ${e.message}`);
        }
    } else {
        try {
            const parsedDefinition = yaml.load(fileContent);
            if (parsedDefinition === undefined) {
                throw new Sequelize.ValidationError('Invalid schema definition YAML file: empty content');
            }
        } catch (e) {
            if (e instanceof Sequelize.ValidationError) {
                throw e;
            }
            throw new Sequelize.ValidationError(`Invalid schema definition YAML file: ${e.message}`);
        }
    }

    return {
        schemaDefinitionFile: fileBuffer,
        schemaDefinitionFileName: sanitizedFileName,
    };
}

module.exports = {
    createAPIMetadata,
    getAPIMetadata,
    getAllAPIMetadata,
    updateAPIMetadata,
    deleteAPIMetadata,
    createAPITemplate,
    updateAPITemplate,
    createAPIContent,
    updateAPIContent,
    getAPIFile,
    getAPIDocTypes,
    listApiDocs,
    deleteAPIFile,
    getMetadataListFromDB,
    getMetadataFromDB,
    addSubscriptionPlans,
    putSubscriptionPlans,
    deleteSubscriptionPlan,
    getSubscriptionPlan,
    listSubscriptionPlans,
    createLabels,
    deleteLabels,
    retrieveLabels,
    getOrgLabels,
    updateLabel,
    addView,
    updateView,
    deleteView,
    getView,
    getAllViews,
    getViewsFromDB,
    getViewInfo,
    parseApiMetadataFromYamlFile,
    prepareApiDefinitionForStorage,
};
