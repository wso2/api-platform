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
const apiDao = require("../dao/apiMetadataDao");
const adminDao = require("../dao/adminDao");
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
const subscriptionPolicyDTO = require("../dto/subscriptionPolicyDto");
const { CustomError } = require("../utils/errors/customErrors");
const LabelDTO = require("../dto/labelDto");

const createAPIMetadata = async (req, res) => {
    const orgId = req.params.orgId;
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
        if (apiMetadata.apiInfo.visibility === constants.API_VISIBILITY.PUBLIC && apiMetadata.apiInfo.visibleGroups) {
            throw new Sequelize.ValidationError(
                "Visible groups cannot be specified for a public API"
            );
        }
        apiMetadata.endPoints.productionURL = changeEndpoint(apiMetadata.endPoints.productionURL);
        apiMetadata.endPoints.sandboxURL = changeEndpoint(apiMetadata.endPoints.sandboxURL);
        normalizeGraphQLEndpoints(apiMetadata);
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            // Create apimetadata record
            const createdAPI = await apiDao.createAPIMetadata(orgId, apiMetadata, t);
            const apiID = createdAPI.dataValues.API_ID;
            if (apiMetadata.subscriptionPolicies) {
                const subscriptionPolicies = [];
                const apiSubscriptionPolicies = apiMetadata.subscriptionPolicies;
                if (!Array.isArray(apiSubscriptionPolicies)) {
                    throw new Sequelize.ValidationError(
                        "Missing or Invalid fields in the request payload"
                    );
                } else {
                    for (const policy of apiSubscriptionPolicies) {
                        const subscriptionPolicy = await apiDao.getSubscriptionPolicyByName(orgId, policy.policyName);
                        if (!subscriptionPolicy) {
                            throw new Sequelize.EmptyResultError("Subscription policy not found");
                        } else {
                            subscriptionPolicies.push({ apiID: apiID, policyID: subscriptionPolicy.POLICY_ID });
                        }
                    };
                }
                await apiDao.createAPISubscriptionPolicy(subscriptionPolicies, apiID, t);
            }
            //store api labels
            if (apiMetadata.apiInfo.labels) {
                const labels = apiMetadata.apiInfo.labels;
                if (!Array.isArray(labels)) {
                    throw new Sequelize.ValidationError(
                        "Missing or Invalid fields in the request payload"
                    );
                }
                await apiDao.createAPILabelMapping(orgId, apiID, labels, t);
            } else {
                await apiDao.createAPILabelMapping(orgId, apiID, ['default'], t);
            }
            // store api definition file (skipped for GraphQL — schema stored below via schemaDefinition)
            if (apiDefinitionFile) {
                await apiDao.storeAPIFile(apiDefinitionFile, apiFileName, apiID, constants.DOC_TYPES.API_DEFINITION, t);
            }
            if (Object.keys(resolvedImageMetadata).length > 0) {
                await apiDao.storeAPIImageMetadata(resolvedImageMetadata, apiID, t);
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
                        apiId: apiID,
                        schemaDefinitionFileSize: schemaDefinition.schemaDefinitionFile.length,
                        schemaFileName: schemaDefinition.schemaDefinitionFileName
                    });
                    await apiDao.storeAPIFile(schemaDefinition.schemaDefinitionFile, schemaDefinition.schemaDefinitionFileName, apiID,
                        constants.DOC_TYPES.SCHEMA_DEFINITION, t);
                    logger.info('Schema definition file stored', {
                        apiId: apiID,
                        schemaFileName: schemaDefinition.schemaDefinitionFileName
                    });
                }
            }

            if (constants.API_TYPE.GRAPHQL === apiMetadata.apiInfo.apiType && req.files?.schemaDefinition?.[0]) {
                const file = req.files.schemaDefinition[0];
                const schemaDefinitionFile = file.buffer;
                logger.debug('GraphQL schema definition file received', {
                    apiId: apiID,
                    schemaDefinitionFileSize: schemaDefinitionFile.length,
                    schemaFileName: file.originalname
                });
                const schemaFileName = constants.FILE_NAME.API_DEFINITION_GRAPHQL;
                await apiDao.storeAPIFile(schemaDefinitionFile, schemaFileName, apiID,
                    constants.DOC_TYPES.API_DEFINITION, t);
                logger.info('GraphQL schema definition file stored', {
                    apiId: apiID,
                    schemaFileName
                });
            }

            if (apiArtifactFile?.buffer && artifactApiContent.length > 0) {
                await apiDao.storeAPIFiles(artifactApiContent, apiID, t);
            }
            apiMetadata.apiID = apiID;
        });


        res.status(201).send(apiMetadata);
    } catch (error) {
        logger.error('API metadata creation failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.params.orgId
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
    
    if (apiStatus === constants.API_STATUS.UNPUBLISHED) {

        const subApis = await adminDao.getSubscriptions(orgId, '', apiId);
        if (subApis.length > 0) {
            return false;
        }
    }
    return true;
}

const getAPIMetadata = async (req, res) => {

    const { orgId, apiId } = req.params;
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

const getMetadataFromDB = async (orgID, apiID) => {

    return await sequelize.transaction({
        timeout: 60000,
    }, async (t) => {
        const retrievedAPI = await apiDao.getAPIMetadata(orgID, apiID, t);
        if (retrievedAPI.length > 0) {
            return new APIDTO(retrievedAPI[0]);
        } else {
            return "";
        }
    });
};

const getAllAPIMetadata = async (req, res) => {
    try {
        const orgID = req.params.orgId;
        const searchTerm = req.query.query;
        const apiName = req.query.apiName;
        const apiVersion = req.query.version;
        const tags = req.query.tags;
        const view = req.query.view;
        let groupList = [];

        if (req.query.groups) {
            groupList.push(req.query.groups.split(" "));
        }
        const retrievedAPIs = await getMetadataListFromDB(orgID, groupList, searchTerm, tags, apiName, apiVersion, view);
        res.status(200).send(retrievedAPIs);
    } catch (error) {
        logger.error('API metadata list retrieval failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.params.orgId,
            searchTerm: req.query.query,
            apiName: req.query.apiName,
            apiVersion: req.query.version,
            tags: req.query.tags,
            view: req.query.view
        });
        util.handleError(res, error);
    }
};

const getMetadataListFromDB = async (orgID, groups, searchTerm, tags, apiName, apiVersion, viewName) => {
    return await sequelize.transaction({
        timeout: 60000,
    }, async (t) => {
        let retrievedAPIs;
        if (apiName || apiVersion || tags) {
            const condition = {};
            if (apiName) condition.API_NAME = apiName;
            if (apiVersion) condition.API_VERSION = apiVersion;
            if (tags) condition.TAGS = tags;
            condition.ORG_ID = orgID;
            retrievedAPIs = await apiDao.getAPIMetadataByCondition(condition);
        } else if (searchTerm) {
            retrievedAPIs = await apiDao.searchAPIMetadata(orgID, groups, searchTerm, viewName, t);
        } else if (viewName) {
            retrievedAPIs = await apiDao.getAllAPIMetadata(orgID, groups, viewName, t);
        }
        // Create response object
        const apiCreationResponse = retrievedAPIs ? retrievedAPIs.map((api) => new APIDTO(api)) : [];
        return apiCreationResponse;
    });
};

const updateAPIMetadata = async (req, res) => {
    //TODO: Get orgId from the orgName
    const { orgId, apiId } = req.params;
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
            let [updatedRows, updatedAPI] = await apiDao.updateAPIMetadata(orgId, apiId, apiMetadata, t);
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
                await apiDao.createAPILabelMapping(orgId, apiId, labels, t);
                updatedAPI[0].dataValues.addedLabels = apiMetadata.apiInfo.addedLabels;
            }
            if (apiMetadata.apiInfo.removedLabels) {
                const labels = apiMetadata.apiInfo.removedLabels;
                if (!Array.isArray(labels)) {
                    throw new Sequelize.ValidationError(
                        "Missing or Invalid fields in the request payload"
                    );
                }
                const labelDelete = await apiDao.deleteAPILabels(orgId, apiId, labels, t);
                if (labelDelete === 0) {
                    throw new Sequelize.EmptyResultError("API Labels not found to delete");
                }
                updatedAPI[0].dataValues.removedLabels = apiMetadata.apiInfo.removedLabels;
            }
            if (apiMetadata.subscriptionPolicies) {
                const subscriptionPolicies = [];
                const apiSubscriptionPolicies = apiMetadata.subscriptionPolicies;
                if (!Array.isArray(apiSubscriptionPolicies)) {
                    throw new Sequelize.ValidationError(
                        "Missing or Invalid fields in the request payload"
                    );
                } else {
                    for (const policy of apiSubscriptionPolicies) {
                        const subscriptionPolicy = await apiDao.getSubscriptionPolicyByName(orgId, policy.policyName);
                        if (!subscriptionPolicy) {
                            throw new Sequelize.EmptyResultError("Subscription policy not found");
                        } else {
                            subscriptionPolicies.push({ apiID: apiId, policyID: subscriptionPolicy.POLICY_ID });
                        }
                    };
                }
                // Get subscription policy IDs and fail if any policy is not found
                await apiDao.updateAPISubscriptionPolicy(subscriptionPolicies, apiId, t);
                updatedAPI[0].dataValues["DP_SUBSCRIPTION_POLICies"] = await apiDao.getSubscriptionPolicies(apiId, t);
            }
            // update api definition file
            const updatedFileCount = await apiDao.updateAPIFile(apiDefinitionFile, apiFileName, apiId, orgId,
                constants.DOC_TYPES.API_DEFINITION, t);
            if (!updatedFileCount) {
                throw new Sequelize.EmptyResultError("No record found to update");
            }
            if (Object.keys(resolvedImageMetadata).length > 0) {
                await apiDao.updateAPIImageMetadata(resolvedImageMetadata, orgId, apiId, t);
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
                    await apiDao.upsertAPIFileByType(schemaDefinition.schemaDefinitionFile, schemaDefinition.schemaDefinitionFileName, apiId, orgId,
                        constants.DOC_TYPES.SCHEMA_DEFINITION, t);
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
                await apiDao.updateAPIFile(schemaDefinitionFile, schemaFileName, apiId, orgId,
                    constants.DOC_TYPES.API_DEFINITION, t);
                logger.info('GraphQL schema definition file updated', {
                    schemaFileName,
                    apiId
                });
            }

            if (apiArtifactFile?.buffer && artifactApiContent.length > 0) {
                await apiDao.updateOrCreateAPIFiles(artifactApiContent, apiId, orgId, t);
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
    const { orgId, apiId } = req.params;
    await sequelize.transaction({
        timeout: 60000,
    }, async (t) => {
        try {
            //check if subscriptions exist for the application
            const subApis = await adminDao.getSubscriptions(orgId, '', apiId);
            if (subApis.length > 0) {
                throw new CustomError(409, constants.ERROR_MESSAGE.ERR_SUB_EXIST, "API has subscriptions.");
            }
            const apiDeleteResponse = await apiDao.deleteAPIMetadata(orgId, apiId, t);
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
        orgId: req.params.orgId,
        apiId: req.params.apiId,
        fileName: req.file?.originalname,
    });
    try {
        const { orgId, apiId } = req.params;
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
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            //check whether api belongs to given org
            let apiMetadata = await apiDao.getAPIMetadata(orgId, apiId, t);
            let existingAPIImage = await apiDao.getImage(constants.API_ICON, apiId, t);

            if (imageMetadata[constants.API_ICON] && existingAPIImage) {
                await apiDao.deleteImage(constants.API_ICON, apiId, t);
            }

            if (apiMetadata) {
                // Store image metadata
                await apiDao.storeAPIImageMetadata(imageMetadata, apiId, t);
                await apiDao.storeAPIFiles(apiContent, apiId, t);
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
            orgId: req.params.orgId,
            apiId: req.params.apiId,
            fileName: req.file?.originalname,
        });
        util.handleError(res, error);
    }
};

const createAPIContent = async (req, res) => {
    const uploadedFile = req.files?.apiContent?.[0] ?? req.file;
    logger.info('Creating API content...', {
        orgId: req.params.orgId,
        apiId: req.params.apiId,
        fileName: uploadedFile?.originalname,
    });
    try {
        const { orgId, apiId } = req.params;
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
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            //check whether api belongs to given org
            let apiMetadata = await apiDao.getAPIMetadata(orgId, apiId, t);
            let existingAPIImage = await apiDao.getImage(constants.API_ICON, apiId, t);

            if (resolvedImageMetadata[constants.API_ICON] && existingAPIImage) {
                await apiDao.deleteImage(constants.API_ICON, apiId, t);
            }

            if (apiMetadata) {
                // Store image metadata
                if (Object.keys(resolvedImageMetadata).length > 0) {
                    await apiDao.storeAPIImageMetadata(resolvedImageMetadata, apiId, t);
                }
                await apiDao.storeAPIFiles(apiContent, apiId, t);
            } else {
                throw new Sequelize.ValidationError(constants.ERROR_MESSAGE.API_NOT_IN_ORG);
            }
        });
        res.status(201).type("application/json").send({ message: "API content updated successfully" });
    } catch (error) {
        logger.error('API content creation failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.params.orgId,
            apiId: req.params.apiId,
            fileName: uploadedFile?.originalname,
        });
        util.handleError(res, error);
    }
};

const updateAPITemplate = async (req, res) => {
    logger.info('Updating API template...', {
        orgId: req.params.orgId,
        apiId: req.params.apiId,
        fileName: req.file?.originalname
    });
    try {
        const { orgId, apiId } = req.params;
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
                orgId: req.params.orgId,
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
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            //check whether api belongs to given org
            const apiMetadata = await apiDao.getAPIMetadata(orgId, apiId, t);
            if (apiMetadata) {
                // Update image metadata
                await apiDao.updateAPIImageMetadata(imageMetadata, orgId, apiId, t);
                // Update API files
                await apiDao.updateOrCreateAPIFiles(apiContent, apiId, orgId, t);
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
            orgId: req.params.orgId,
            apiId: req.params.apiId,
            fileName: req.file?.originalname
        });
        util.handleError(res, error);
    }
};

const updateAPIContent = async (req, res) => {
    const uploadedFile = req.files?.apiContent?.[0] ?? req.file;
    logger.info('Updating API content...', {
        orgId: req.params.orgId,
        apiId: req.params.apiId,
        fileName: uploadedFile?.originalname
    });
    try {
        const { orgId, apiId } = req.params;
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
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            //check whether api belongs to given org
            const apiMetadata = await apiDao.getAPIMetadata(orgId, apiId, t);
            if (apiMetadata) {
                // Update image metadata
                if (Object.keys(resolvedImageMetadata).length > 0) {
                    await apiDao.updateAPIImageMetadata(resolvedImageMetadata, orgId, apiId, t);
                }
                // Update API files
                await apiDao.updateOrCreateAPIFiles(apiContent, apiId, orgId, t);
            } else {
                throw new Sequelize.ValidationError(constants.ERROR_MESSAGE.API_NOT_IN_ORG);
            }
        });
        res.status(201).send({ message: "API Files updated successfully" });
    } catch (error) {
        logger.error('API content update failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.params.orgId,
            apiId: req.params.apiId,
            fileName: uploadedFile?.originalname
        });
        util.handleError(res, error);
    }
};

const getAPIFile = async (req, res) => {

    const { orgId, apiId } = req.params;
    const apiFileName = req.query.fileName;
    const type = req.query.type;
    let apiFileResponse = "";
    let apiFile;
    let contentType = "";
    try {
        const fileExtension = path.extname(apiFileName).toLowerCase();
        apiFileResponse = await apiDao.getAPIFile(apiFileName, type, orgId, apiId);
        if (apiFileResponse) {
            apiFile = apiFileResponse.API_FILE;
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

const getAPIDocTypes = async (orgID, apiID) => {

    try {
        const docTypeResponse = await apiDao.getAPIDocTypes(orgID, apiID);
        const apiCreationResponse = docTypeResponse.map((doc) => new APIDocDTO(doc.dataValues));
        return apiCreationResponse;
    } catch (error) {
        logger.error('API doc types retrieval failed', {
            error: error.message,
            stack: error.stack,
            orgId: orgID,
            apiId: apiID
        });
        // Note: This function doesn't have access to res, so we can't call util.handleError
        throw error;
    }
}

const deleteAPIFile = async (req, res) => {
    logger.info('Deleting API file...', {
        orgId: req.params.orgId,
        apiId: req.params.apiId,
        fileName: req.query.fileName,
        fileType: req.query.type
    });
    const { orgId, apiId } = req.params;
    const apiFileName = req.query.fileName;
    const fileType = req.query.type;
    try {
        let apiFileResponse;
        if (apiFileName) {
            apiFileResponse = await apiDao.deleteAPIFile(apiFileName, fileType, orgId, apiId);
        } else {
            apiFileResponse = await apiDao.deleteAllAPIFiles(fileType, orgId, apiId);
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
            orgId: req.params?.orgId,
            apiId: req.params?.apiId
        });
        util.handleError(res, error);
    }
};

const addSubscriptionPolicies = async (req, res) => {
    if (req.files?.subscriptionPolicy?.[0]) {
        try {
            const policies = parseSubscriptionPoliciesFromYamlFile(req.files.subscriptionPolicy[0].buffer);
            req.body = policies.length === 1 ? policies[0] : policies;
        } catch (error) {
            return util.handleError(res, error);
        }
    }
    if (Array.isArray(req.body)) {
        await createSubscriptionPolicies(req, res);
    } else {
        await createSubscriptionPolicy(req, res);
    }
}

const putSubscriptionPolicies = async (req, res) => {
    if (req.files?.subscriptionPolicy?.[0]) {
        try {
            const policies = parseSubscriptionPoliciesFromYamlFile(req.files.subscriptionPolicy[0].buffer);
            req.body = policies.length === 1 ? policies[0] : policies;
        } catch (error) {
            return util.handleError(res, error);
        }
    }
    if (Array.isArray(req.body)) {
        await updateSubscriptionPolicies(req, res);
    } else {
        await updateSubscriptionPolicy(req, res);
    }
}

const createSubscriptionPolicy = async (req, res) => {
    const { orgId } = req.params;
    const subscriptionPolicy = req.body;
    logger.info('Creating subscription policy...', {
        orgId
    });

    if (!subscriptionPolicy || typeof subscriptionPolicy !== "object") {
        return res.status(400).json({ message: "Request body is missing or invalid" });
    }

    const validTypes = ["requestcount", "eventcount"];
    if (!subscriptionPolicy.type || typeof subscriptionPolicy.type !== 'string' || !validTypes.includes(subscriptionPolicy.type.toLowerCase())) {
        return res.status(400).json({ message: "Invalid or missing subscription policy type" });
    }

    try {
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            const subscriptionPolicyResponse = await apiDao.createSubscriptionPolicy(orgId, subscriptionPolicy, t);
            if (subscriptionPolicyResponse) {
                logger.info('Created subscription policy', {
                    orgId
                });
                res.status(201).send(new subscriptionPolicyDTO(subscriptionPolicyResponse));
            } else {
                throw new CustomError(500, constants.ERROR_CODE[500], constants.ERROR_MESSAGE.SUBSCRIPTION_POLICY_CREATE_ERROR);
            }
        });
    } catch (error) {
        logger.error('subscription policy create error failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
};

const createSubscriptionPolicies = async (req, res) => {
    try {
        if (config.generateDefaultSubPolicies) {
            const msg = "Bulk creation of subscription policies is not allowed because 'generateDefaultSubPolicies' is enabled in the Developer Portal.";
            logger.info(msg, {
                orgId: req.params?.orgId
            });
            res.status(200).json({ message: msg });
        } else {
            const { orgId } = req.params;
            const subscriptionPolicies = req.body;

            if (!Array.isArray(subscriptionPolicies) || subscriptionPolicies.length === 0) {
                return res.status(400).json({ message: "Missing or invalid fields in the request payload" });
            }

            const createdPolicies = [];

            await sequelize.transaction({
                timeout: 60000,
            }, async (t) => {
                // TODO: Try using SubscriptionPolicy.bulkCreate() once Table is finalised and manipulating each data is not needed
                for (const policy of subscriptionPolicies) {
                    if (typeof policy.type !== 'string') {
                        throw new CustomError(400, constants.ERROR_CODE[400], 'subscriptionPolicy.type must be a string');
                    }
                    if (policy.type.toLowerCase() == "requestcount" || policy.type.toLowerCase() == "eventcount") {
                        const created = await apiDao.createSubscriptionPolicy(orgId, policy, t);
                        if (!created) {
                            throw new CustomError(
                                500,
                                constants.ERROR_CODE[500],
                                `Failed to create policy: ${policy.policyName || "unknown"}`
                            );
                        }
                        createdPolicies.push(new subscriptionPolicyDTO(created));
                    }
                }
            });
            logger.info('Created subscription policies', {
                orgId
            });
            res.status(201).send(createdPolicies);
        }
    } catch (error) {
        logger.error('subscription policy create error failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.params?.orgId
        });
        util.handleError(res, error);
    }
};

const updateSubscriptionPolicy = async (req, res) => {
    const { orgId } = req.params;
    logger.info('Updating subscription policy...', {
        orgId
    });
    const subscriptionPolicy = req.body;

    if (!subscriptionPolicy || typeof subscriptionPolicy !== "object") {
        return res.status(400).json({ message: "Request body is missing or invalid" });
    }

    const validTypes = ["requestcount", "eventcount"];
    if (!subscriptionPolicy.type || typeof subscriptionPolicy.type !== 'string' || !validTypes.includes(subscriptionPolicy.type.toLowerCase())) {
        return res.status(400).json({ message: "Invalid or missing subscription policy type" });
    }
    
    try {
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            const { subscriptionPolicyResponse, statusCode } =  await apiDao.putSubscriptionPolicy(orgId, subscriptionPolicy, t);
            if (subscriptionPolicyResponse) {
                res.status(statusCode).send(new subscriptionPolicyDTO(subscriptionPolicyResponse));
            } else {
                throw new CustomError(404, constants.ERROR_CODE[404], constants.ERROR_MESSAGE.SUBSCRIPTION_POLICY_NOT_FOUND);
            }
        });
    } catch (error) {
        logger.error('subscription policy not found failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.params?.orgId
        });
        util.handleError(res, error);
    }
};

const updateSubscriptionPolicies = async (req, res) => {
    try {
        if (config.generateDefaultSubPolicies) {
            const msg = "Bulk updating of subscription policies is not allowed because 'generateDefaultSubPolicies' is enabled in the Developer Portal.";
            logger.info(msg, {
                orgId: req.params?.orgId
            });
            res.status(200).json({ message: msg });
        } else {
            const { orgId } = req.params;
            const subscriptionPolicies = req.body;

            if (!Array.isArray(subscriptionPolicies) || subscriptionPolicies.length === 0) {
                return res.status(400).json({ message: "Missing or invalid fields in the request payload" });
            }

            const updatedPolicies = [];

            await sequelize.transaction({
                timeout: 60000,
            }, async (t) => {
                for (const policy of subscriptionPolicies) {
                    if (typeof policy.type !== 'string') {
                        throw new CustomError(400, constants.ERROR_CODE[400], 'subscriptionPolicy.type must be a string');
                    }
                    if (policy.type.toLowerCase() == "requestcount" || policy.type.toLowerCase() == "eventcount") {
                        const created = await apiDao.putSubscriptionPolicy(orgId, policy, t);
                        if (!created) {
                            throw new CustomError(
                                500,
                                constants.ERROR_CODE[500],
                                `Failed to create policy: ${policy.policyName || "unknown"}`
                            );
                        }
                        updatedPolicies.push(new subscriptionPolicyDTO(created));
                    }
                }
            });

            res.status(201).send(updatedPolicies);
        }
    } catch (error) {
        logger.error('subscription policy create error failed', {
            error: error.message,
            stack: error.stack,
            orgId: req.params?.orgId
        });
        util.handleError(res, error);
    }
};

const deleteSubscriptionPolicy = async (req, res) => {
    const { orgId, policyId } = req.params;
    logger.info('Deleting subscription policy...', {
        orgId,
        policyId
    });
    try {
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            const deleteCount = await apiDao.deleteSubscriptionPolicyById(orgId, policyId, t);
            if (deleteCount === 0) {
                throw new CustomError(404, constants.ERROR_CODE[404], constants.ERROR_MESSAGE.SUBSCRIPTION_POLICY_NOT_FOUND);
            } else {
                res.status(204).send();
            }
        });
    } catch (error) {
        logger.error('subscription policy delete error failed', {
            error: error.message,
            stack: error.stack,
            orgId,
            policyId
        });
        util.handleError(res, error);
    }
};

const getSubscriptionPolicy = async (req, res) => {

    const { orgId, policyId } = req.params;

    try {
        const subscriptionPolicyResponse = await apiDao.getSubscriptionPolicy(policyId, orgId);
        if (subscriptionPolicyResponse) {
            res.status(200).send(new subscriptionPolicyDTO(subscriptionPolicyResponse));
        } else {
            throw new CustomError(404, constants.ERROR_CODE[404], constants.ERROR_MESSAGE.SUBSCRIPTION_POLICY_NOT_FOUND);
        }
    } catch (error) {
        logger.error('subscription policy not found failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
};

// Lists subscription policies for an org. With ?name=<exact>, returns an array
// containing the single matching policy (or empty array) — name is unique per
// org. Without it, returns all policies for the org.
const listSubscriptionPolicies = async (req, res) => {

    const { orgId } = req.params;
    const { name } = req.query;

    try {
        let policies;
        if (name) {
            const policy = await apiDao.getSubscriptionPolicyByName(orgId, name);
            policies = policy ? [policy] : [];
        } else {
            policies = await apiDao.getAllSubscriptionPolicies(orgId);
        }
        res.status(200).send(policies.map((policy) => new subscriptionPolicyDTO(policy)));
    } catch (error) {
        logger.error('subscription policy list failed', {
            error: error.message,
            stack: error.stack,
            orgId
        });
        util.handleError(res, error);
    }
};

const createLabels = async (req, res) => {

    const orgId = req.params.orgId;
    const labels = req.body;
    try {
        await apiDao.createLabels(orgId, labels);
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

    const orgId = req.params.orgId;
    const labels = req.body;
    try {
        for (const label of labels) {
            await apiDao.updateLabel(orgId, label);
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

    const orgId = req.params.orgId;
    const labelNames = req.query.names;
    const labelList = labelNames.split(",");
    try {
        await apiDao.deleteLabel(orgId, labelList);
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

    const orgId = req.params.orgId;
    try {
        const labels = await getOrgLabels(orgId);
        res.status(200).send(labels);
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
        const labels = await apiDao.getLabels(orgId);
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

    const orgId = req.params.orgId;
    const labels = req.body.labels;
    await sequelize.transaction({
        timeout: 60000,
    }, async (t) => {
        try {
            const viewResponse = await apiDao.addView(orgId, req.body, t);
            const viewID = viewResponse.dataValues.VIEW_ID;
            await apiDao.addViewLabels(orgId, viewID, labels, t);
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

    const orgId = req.params.orgId;
    const removedLabels = req.body.removedLabels ? req.body.removedLabels : [];
    const addedLabels = req.body.addedLabels ? req.body.addedLabels : [];
    const viewName = req.params.viewName;
    try {
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {

            let viewID = "";
            if (req.body.displayName) {
                let viewResponse = await apiDao.updateView(orgId, viewName, req.body.displayName, t);
                viewID = viewResponse.dataValues.VIEW_ID;
            }
            if (removedLabels.length !== 0) {
                await apiDao.deleteViewLabels(orgId, viewID, removedLabels, t);
            }
            if (addedLabels.length !== 0) {
                viewID = viewID ? viewID : await apiDao.getViewID(orgId, viewName, t);
                await apiDao.addViewLabels(orgId, viewID, addedLabels, t);
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

    const orgId = req.params.orgId;
    const name = req.params.viewName;
    try {
        const viewDelete = await apiDao.deleteView(orgId, name);
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

    const orgId = req.params.orgId;
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

    const view = await apiDao.getView(orgId, name);
    if (view.dataValues) {
        return new ViewDTO(view.dataValues);
    } else {
        return null;
    }
}

const getAllViews = async (req, res) => {

    const orgId = req.params.orgId;
    try {
        const views = await getViewsFromDB(orgId);
        if (views.length > 0) {
            return res.status(200).send(views);;
        } else {
            res.status(404).send("No views found");
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

const getViewsFromDB = async (orgId) => {

    const views = await apiDao.getAllViews(orgId);
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
    const endpoints = spec.endpoints || {};
    const businessInformation = spec.businessInformation || {};

    const subscriptionPolicies = util.normalizeStringArray(spec.subscriptionPolicies)
        .map(policyName => ({ policyName }));
    const visibleGroups = util.normalizeStringArray(spec.visibleGroups);

    return {
        apiInfo: {
            apiName: spec.displayName,
            apiVersion: spec.version,
            apiDescription: spec.description,
            provider: spec.provider,
            referenceID: spec.referenceID,
            apiHandle: metadata.name,
            apiType,
            apiStatus,
            visibility: spec.visibility || constants.API_VISIBILITY.PUBLIC,
            visibleGroups: visibleGroups.length > 0 ? visibleGroups : null,
            tags: util.normalizeStringArray(spec.tags),
            labels: util.normalizeStringArray(spec.labels),
            gatewayType: spec.gatewayType || null,
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
        subscriptionPolicies,
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

function mapYamlToSubscriptionPolicy(item) {
    const { metadata = {}, spec = {} } = item;
    return {
        policyName: metadata.name,
        displayName: spec.displayName,
        description: spec.description,
        refId: spec.refId,
        type: spec.type,
        requestCount: spec.requestCount,
        eventCount: spec.eventCount,
    };
}

function parseSubscriptionPoliciesFromYamlFile(fileBuffer) {
    let parsed;
    try {
        parsed = yaml.load(fileBuffer.toString(constants.CHARSET_UTF8));
    } catch (e) {
        throw new Sequelize.ValidationError(`Invalid subscription policy YAML file: ${e.message}`);
    }

    if (!parsed || typeof parsed !== 'object') {
        throw new Sequelize.ValidationError('Subscription policy YAML file is empty or invalid');
    }

    const kind = parsed.kind;
    if (kind === 'SubscriptionPolicy') {
        return [mapYamlToSubscriptionPolicy(parsed)];
    } else if (kind === 'SubscriptionPolicyList') {
        if (!Array.isArray(parsed.items) || parsed.items.length === 0) {
            throw new Sequelize.ValidationError("SubscriptionPolicyList must have a non-empty 'items' array");
        }
        return parsed.items.map(mapYamlToSubscriptionPolicy);
    } else {
        throw new Sequelize.ValidationError(
            `Unknown subscription policy YAML kind '${kind}'. Expected 'SubscriptionPolicy' or 'SubscriptionPolicyList'`
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
    deleteAPIFile,
    getMetadataListFromDB,
    getMetadataFromDB,
    addSubscriptionPolicies,
    putSubscriptionPolicies,
    deleteSubscriptionPolicy,
    getSubscriptionPolicy,
    listSubscriptionPolicies,
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
    getViewInfo
};
