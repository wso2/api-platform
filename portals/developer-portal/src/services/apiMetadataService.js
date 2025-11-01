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
const sequelize = require("../db/sequelize");
const apiDao = require("../dao/apiMetadata");
const adminDao = require("../dao/admin");
const util = require("../utils/util");
const logger = require("../config/logger");
const config = require(process.cwd() + '/config.json');
const path = require("path");
const fs = require("fs").promises;
const fsDir = require("fs");
const { validationResult } = require('express-validator');
const APIDTO = require("../dto/apiDTO");
const ViewDTO = require("../dto/views");
const APIDocDTO = require("../dto/apiDoc");
const constants = require("../utils/constants");
const subscriptionPolicyDTO = require("../dto/subscriptionPolicy");
const { CustomError } = require("../utils/errors/customErrors");
const LabelDTO = require("../dto/label");

const createAPIMetadata = async (req, res) => {
    const orgId = req.params.orgId;
    logger.info('Creating API metadata...', {
        orgId
    });
    const apiMetadata = JSON.parse(req.body.apiMetadata);
    let apiDefinitionFile, apiFileName = "";
    if (req.files?.apiDefinition?.[0]) {
        const file = req.files.apiDefinition[0];
        apiDefinitionFile = file.buffer;
        apiFileName = file.originalname;
    }
    
    try {
        // Validate input
        if (!apiMetadata.apiInfo || !apiDefinitionFile || !apiMetadata.endPoints) {
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
            // store api definition file
            await apiDao.storeAPIFile(apiDefinitionFile, apiFileName, apiID, constants.DOC_TYPES.API_DEFINITION, t);
            // Save MCP tools as schema definition if the API type is MCP
            if (constants.API_TYPE.MCP === apiMetadata.apiInfo.apiType && req.files?.schemaDefinition?.[0]) {
                const file = req.files.schemaDefinition[0];
                const schemaDefinitionFile = file.buffer;
                logger.debug('Schema definition file received', {
                    apiId: apiID,
                    schemaDefinitionFileSize: schemaDefinitionFile.length,
                    schemaFileName: file.originalname
                });
                const schemaFileName = file.originalname;
                await apiDao.storeAPIFile(schemaDefinitionFile, schemaFileName, apiID,
                    constants.DOC_TYPES.SCHEMA_DEFINITION, t);
                logger.info('Schema definition file stored', {
                    apiId: apiID,
                    schemaFileName
                });
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
    if (!orgId || !apiId) {
        throw new Sequelize.ValidationError(
            "Missing or Invalid fields in the request payload"
        );
    }
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

        const allowedQueryParams = ['query', 'apiName', 'version', 'tags', 'groups', 'view'];
        const invalidParams = Object.keys(req.query).filter(param => !allowedQueryParams.includes(param));

        if (invalidParams.length > 0) {
            const parameterMessage = invalidParams.length === 1
                ? `Invalid query parameter: ${invalidParams.join(', ')}`
                : `Invalid query parameters: ${invalidParams.join(', ')}`;
            throw new Sequelize.ValidationError(parameterMessage);
        }

        if (req.query.groups) {
            groupList.push(req.query.groups.split(" "));
        }
        if (!orgID) {
            throw new Sequelize.ValidationError("Missing or Invalid fields in the request payload");
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
    const apiMetadata = JSON.parse(req.body.apiMetadata);
    let apiDefinitionFile, apiFileName = "";
    if (req.files?.apiDefinition?.[0]) {
        const file = req.files.apiDefinition[0];
        apiDefinitionFile = file.buffer;
        apiFileName = file.originalname;
    }
    logger.debug('MCP API Definition file', {
        apiFileName,
        hasApiDefinitionFile: !!apiDefinitionFile,
        orgId,
        apiId
    });

    try {
        // Validate input
        if (!apiMetadata.apiInfo || !apiDefinitionFile || !apiMetadata.endPoints) {
            throw new Sequelize.ValidationError(
                "Missing or Invalid fields in the request payload"
            );
        }
        apiMetadata.endPoints.productionURL = changeEndpoint(apiMetadata.endPoints.productionURL);
        apiMetadata.endPoints.sandboxURL = changeEndpoint(apiMetadata.endPoints.sandboxURL);

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
            // Update MCP tools schema definition if the API type is MCP
            logger.debug('Processing MCP API schema definition', {
                hasSchemaDefinition: !!req.files?.schemaDefinition?.[0],
                apiType: apiMetadata.apiInfo.apiType,
                apiId
            });
            if (constants.API_TYPE.MCP === apiMetadata.apiInfo.apiType && req.files?.schemaDefinition?.[0]) {
                const file = req.files.schemaDefinition[0];
                const schemaDefinitionFile = file.buffer;
                const schemaFileName = file.originalname;
                logger.debug('Schema definition file received for update', {
                    schemaDefinitionFileSize: schemaDefinitionFile.length,
                    schemaFileName,
                    apiId
                });
                await apiDao.updateAPIFile(schemaDefinitionFile, schemaFileName, apiId, orgId,
                    constants.DOC_TYPES.SCHEMA_DEFINITION, t);
                logger.info('Schema definition file updated', {
                    schemaFileName,
                    apiId
                });
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
    if (!orgId || !apiId) {
        throw new Sequelize.ValidationError(
            "Missing or Invalid fields in the request payload"
        );
    }
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
        if (!orgId) {
            throw new CustomError(400, "Bad Request", "Missing required parameter: 'orgId'");
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
            let exsistingAPIImage = await apiDao.getImage(constants.API_ICON, apiId, t);

            if (imageMetadata[constants.API_ICON], exsistingAPIImage) {
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
        if (!orgId) {
            throw new CustomError(400, "Bad Request", "Missing required parameter: 'orgId'");
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
            docMetadata = JSON.parse(req.body.docMetadata);
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

const getAPIFile = async (req, res) => {

    const rules = util.validateRequestParameters();
    for (let validation of rules) {
        await validation.run(req);
    }
    const errors = validationResult(req);
    if (!errors.isEmpty()) {
        return res.status(400).json(util.getErrors(errors));
    }
    const { orgId, apiId } = req.params;
    const apiFileName = req.query.fileName;
    const type = req.query.type;
    if (!orgId || !apiId || !apiFileName || !type) {
        throw new Sequelize.ValidationError("Missing or Invalid fields in the request payload");
    }
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
    if (!orgId || !apiId) {
        throw new Sequelize.ValidationError(
            "Missing or Invalid fields in the request payload"
        );
    }
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
    if (Array.isArray(req.body)) {
        await createSubscriptionPolicies(req, res);
    } else {
        await createSubscriptionPolicy(req, res);
    }
}

const putSubscriptionPolicies = async (req, res) => {
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

    if (!orgId) {
        return res.status(400).json({ message: "Organization ID is missing" });
    }

    if (!subscriptionPolicy || typeof subscriptionPolicy !== "object") {
        return res.status(400).json({ message: "Request body is missing or invalid" });
    }

    if (subscriptionPolicy.type !== "requestCount") {
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

            if (!orgId) {
                return res.status(400).json({ message: "Organization ID is missing" });
            }

            if (!Array.isArray(subscriptionPolicies) || subscriptionPolicies.length === 0) {
                return res.status(400).json({ message: "Missing or invalid fields in the request payload" });
            }

            const createdPolicies = [];

            await sequelize.transaction({
                timeout: 60000,
            }, async (t) => {
                // TODO: Try using SubscriptionPolicy.bulkCreate() once Table is finalised and manipulating each data is not needed
                for (const policy of subscriptionPolicies) {
                    if (policy.type == "requestCount") {
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

    if (!orgId) {
        return res.status(400).json({ message: "Organization ID is missing" });
    }

    if (!subscriptionPolicy || typeof subscriptionPolicy !== "object") {
        return res.status(400).json({ message: "Request body is missing or invalid" });
    }

    if (subscriptionPolicy.type !== "requestCount") {
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

            if (!orgId) {
                return res.status(400).json({ message: "Organization ID is missing" });
            }

            if (!Array.isArray(subscriptionPolicies) || subscriptionPolicies.length === 0) {
                return res.status(400).json({ message: "Missing or invalid fields in the request payload" });
            }

            const updatedPolicies = [];

            await sequelize.transaction({
                timeout: 60000,
            }, async (t) => {
                for (const policy of subscriptionPolicies) {
                    if (policy.type == "requestCount") {
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
    const { orgId, policyName } = req.params;
    logger.info('Deleting subscription policy...', {
        orgId,
        policyName
    });
    if (!orgId || !policyName) {
        throw new Sequelize.ValidationError(
            "Missing or Invalid fields in the request payload"
        );
    }
    try {
        await sequelize.transaction({
            timeout: 60000,
        }, async (t) => {
            const deleteCount = await apiDao.deleteSubscriptionPolicy(orgId, policyName, t);
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
            policyName
        });
        util.handleError(res, error);
    }
};

const getSubscriptionPolicy = async (req, res) => {

    const { orgId, policyID } = req.params;

    if (!orgId || !policyID) {
        throw new Sequelize.ValidationError(
            "Missing or Invalid fields in the request payload"
        );
    }

    try {
        const subscriptionPolicyResponse = await apiDao.getSubscriptionPolicy(policyID, orgId);
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

const createLabels = async (req, res) => {

    const orgId = req.params.orgId;
    const labels = req.body;
    try {
        if (!orgId || !labels) {
            throw new Sequelize.ValidationError(
                "Missing or Invalid fields in the request payload"
            );
        }
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
        if (!orgId || !labels) {
            throw new Sequelize.ValidationError(
                "Missing or Invalid fields in the request payload"
            );
        }
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
    if (!orgId || !labelNames) {
        throw new Sequelize.ValidationError(
            "Missing or Invalid fields in the request payload"
        );
    }
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
    if (!orgId) {
        throw new Sequelize.ValidationError(
            "Missing or Invalid fields in the request payload"
        );
    }
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
    const viewName = req.body.name;
    if (!orgId || !viewName || !labels) {
        throw new Sequelize.ValidationError(
            "Missing or Invalid fields in the request payload"
        );
    }
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
    const viewName = req.params.name;
    if (!orgId || !viewName) {
        throw new Sequelize.ValidationError(
            "Missing or Invalid fields in the request payload"
        );
    }
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
    const name = req.params.name;
    if (!orgId || !name) {
        throw new Sequelize.ValidationError(
            "Missing or Invalid fields in the request payload"
        );
    }
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
    const name = req.params.name;
    if (!orgId || !name) {
        throw new Sequelize.ValidationError(
            "Missing or Invalid fields in the request payload"
        );
    }
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
    if (!orgId) {
        throw new Sequelize.ValidationError(
            "Missing or Invalid fields in the request payload"
        );
    }
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

module.exports = {
    createAPIMetadata,
    getAPIMetadata,
    getAllAPIMetadata,
    updateAPIMetadata,
    deleteAPIMetadata,
    createAPITemplate,
    updateAPITemplate,
    getAPIFile,
    getAPIDocTypes,
    deleteAPIFile,
    getMetadataListFromDB,
    getMetadataFromDB,
    addSubscriptionPolicies,
    putSubscriptionPolicies,
    deleteSubscriptionPolicy,
    getSubscriptionPolicy,
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
