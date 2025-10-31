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
const { renderTemplate, renderTemplateFromAPI, renderGivenTemplate, loadLayoutFromAPI, loadMarkdown } = require('../utils/util');
const config = require(process.cwd() + '/config.json');
const logger = require('../config/logger');
const { logUserAction } = require('../middlewares/auditLogger');
const fs = require('fs');
const path = require('path');
const exphbs = require('express-handlebars');
const util = require('../utils/util');
const constants = require('../utils/constants');
const adminDao = require('../dao/admin');
const apiDao = require('../dao/apiMetadata');
const apiMetadataService = require('../services/apiMetadataService');
const adminService = require('../services/adminService');
const subscriptionPolicyDTO = require('../dto/subscriptionPolicy');
const { ApplicationDTO } = require('../dto/application');
const APIDTO = require('../dto/apiDTO');
const controlPlaneUrl = config.controlPlane.url;

const filePrefix = config.pathToContent;
const generateArray = (length) => Array.from({ length });
const baseURLDev = config.baseUrl + constants.ROUTE.VIEWS_PATH;

const loadAPIs = async (req, res) => {

    const { orgName, viewName } = req.params;
    let html;
    if (config.mode === constants.DEV_MODE) {
        const metaDataList = await loadAPIMetaDataList();
        for (const metaData of metaDataList) {
            let subscriptionPlans = [];
            subscriptionPlans.push({
                displayName: "Sample",
                policyName: "Sample",
                description: "Sample",
                billingPlan: "Sample",
                requestCount: "1000",
            });
            metaData.subscriptionPolicyDetails = subscriptionPlans;
        }
        const templateContent = {
            apiMetadata: metaDataList,
            baseUrl: baseURLDev + viewName
        }
        html = renderTemplate(filePrefix + 'pages/apis/page.hbs', filePrefix + 'layout/main.hbs', templateContent, false);
    } else {
        const orgDetails = await adminDao.getOrganization(orgName);
        const devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.API_TYPE.DEFAULT;
        try {
            const cpOrgID = orgDetails.ORGANIZATION_IDENTIFIER;
            req.cpOrgID = cpOrgID;
            const orgID = orgDetails.ORG_ID;
            const searchTerm = req.query.query;
            const tags = req.query.tags;
            let metaDataList = await loadAPIMetaDataListFromAPI(req, orgID, orgName, searchTerm, tags, viewName);
            const apiData = await loadAPIMetaDataListFromAPI(req, orgID, orgName, searchTerm, tags, viewName);
            let appList = [];
            let apiTags = [];
            apiData.forEach(api => {
                if (api.apiInfo.tags) {
                    api.apiInfo.tags.forEach(tag => {
                        if (!apiTags.includes(tag)) {
                            apiTags.push(tag);
                        }
                    });
                }
            });

            for (const metaData of metaDataList) {
                metaData.subscriptionPolicyDetails = await util.appendSubscriptionPlanDetails(orgID, metaData.subscriptionPolicies);
                if (req.user) {
                    let applications = await adminDao.getApplications(orgID, req.user.sub);
                    if (applications.length > 0) {
                        appList = await Promise.all(
                            applications.map(async (app) => {
                                const subscription = await adminDao.getAppApiSubscription(orgID, app.APP_ID, metaData.apiID);
                                return {
                                    ...new ApplicationDTO(app),
                                    subscribed: subscription.length > 0,
                                };
                            })
                        );
                    }
                }
                metaData.applications = appList;
            }
            //retrieve api list from control plane
            if (config.controlPlane?.enabled !== false) {
                let publicMode = false;
                if (cpOrgID && Array.isArray(req.user?.authorizedOrgs) && !req.user.authorizedOrgs?.includes(cpOrgID)) {
                    publicMode = true;
                }
                try {
                    const allowedAPIList = await util.invokeApiRequest(req, 'GET', `${controlPlaneUrl}/apis?limit=1000`, {}, {}, publicMode);
                    if (allowedAPIList) {
                        //filter apis based on the roles
                        metaDataList = util.filterAllowedAPIs(metaDataList, allowedAPIList.list);
                    }
                } catch (error) {
                    logger.warn("Cannot retrieve allowed API list from control plane", {
                        orgName: req.params.orgName,
                        error: error.message,
                        stack: error.stack
                    });
                }
            } else {
                logger.debug("Control plane is disabled, skipping API filtering", {
                    orgName: req.params.orgName
                });
            }

            let profile = null;
            if (req.user) {
                profile = {
                    imageURL: req.user.imageURL,
                    firstName: req.user.firstName,
                    lastName: req.user.lastName,
                    email: req.user.email,
                }
            }
            const templateContent = {
                isAuthenticated: req.isAuthenticated(),
                apiMetadata: metaDataList,
                tags: apiTags,
                baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
                orgID: orgID,
                profile: req.isAuthenticated() ? profile : null,
                devportalMode: devportalMode,
                isReadOnlyMode: config.readOnlyMode
            };

            if (req.originalUrl.includes("/mcps")) {
                html = await renderTemplateFromAPI(templateContent, orgID, orgName, "pages/mcp", viewName);
            } else {
                html = await renderTemplateFromAPI(templateContent, orgID, orgName, "pages/apis", viewName);
            }
        } catch (error) {
            logger.error(constants.ERROR_MESSAGE.API_LISTING_LOAD_ERROR, {
                orgName: orgName,
                error: error.message, 
                stack: error.stack
            });
            const templateContent = {
                baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
                devportalMode: devportalMode,
            }
            if (Number(error?.statusCode) === 401) {
                logger.warn("User is not authorized to access the API or user session expired, hence redirecting to login page", {
                    orgName: orgName,
                });
                templateContent.errorMessage = constants.ERROR_MESSAGE.COMMON_AUTH_ERROR_MESSAGE;
                html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
            } else {
                templateContent.errorMessage = constants.ERROR_MESSAGE.COMMON_ERROR_MESSAGE;
                html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
            }
        }
    }
    res.send(html);
}

const loadAPIContent = async (req, res) => {

    let html;
    const hbs = exphbs.create({});
    let { orgName, apiHandle, viewName } = req.params;

    if (config.mode === constants.DEV_MODE) {
        const metaData = loadAPIMetaDataFromFile(apiHandle);
        const filePath = path.join(process.cwd(), filePrefix + '../mock', apiHandle + "/" + constants.FILE_NAME.API_HBS_CONTENT_FILE_NAME);

        if (fs.existsSync(filePath)) {
            hbs.handlebars.registerPartial('api-content', fs.readFileSync(filePath, constants.CHARSET_UTF8));
        }
        let subscriptionPlans = [];
        metaData.subscriptionPolicies.forEach(policy => {
            const subscriptionPlan = {
                name: policy.policyName,
                description: "Sample description",
                tierPlan: "Sample tier"
            };
            subscriptionPlans.push(subscriptionPlan);
        });

        const templateContent = {
            devMode: true,
            providerUrl: '#subscriptionPlans',
            apiContent: await loadMarkdown(constants.FILE_NAME.API_MD_CONTENT_FILE_NAME, filePrefix + '../mock/' + req.params.apiName),
            apiMetadata: metaData,
            subscriptionPlans: subscriptionPlans,
            baseUrl: baseURLDev + viewName,
            schemaUrl: orgName + '/mock/' + apiHandle + '/apiDefinition.xml'
        }
        html = renderTemplate(filePrefix + 'pages/api-landing/page.hbs', filePrefix + 'layout/main.hbs', templateContent, false);
        res.send(html);
    } else {
        const orgDetails = await adminDao.getOrganization(orgName);
        const devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.API_TYPE.DEFAULT;
        try {
            const orgDetails = await adminDao.getOrganization(orgName);
            const cpOrgID = orgDetails.ORGANIZATION_IDENTIFIER;
            req.cpOrgID = cpOrgID;
            const orgID = orgDetails.ORG_ID;
            const apiID = await apiDao.getAPIId(orgID, apiHandle);
            const metaData = await loadAPIMetaData(req, orgID, apiID);
            
            // Log API access for audit trail
            logUserAction('API_ACCESS', req, {
                orgName: orgName,
                apiHandle: apiHandle,
                apiID: apiID,
            });
            
            let apiDetail = null;
            //check whether user has access to the API via control plane
            if (config.controlPlane?.enabled !== false) {
                try {
                    let apiName = metaData ? metaData.apiHandle?.split('-v')[0] : "";
                    const version = metaData ? metaData.apiInfo.apiVersion : "";
                    let allowedAPIList = await util.invokeApiRequest(req, 'GET', `${controlPlaneUrl}/apis?query=name:"${apiName}"+version:${version}`, {}, {});
                    if (allowedAPIList.count === 0) {
                        apiName = metaData.apiInfo.apiName;
                        allowedAPIList = await util.invokeApiRequest(req, 'GET', `${controlPlaneUrl}/apis?query=name:"${apiName}"+version:${version}`, {}, {});
                    }

                    if (allowedAPIList.count === 0) {
                        let templateContent = {
                            baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
                            errorMessage: constants.ERROR_MESSAGE.UNAUTHORIZED_API,
                            devportalMode: devportalMode,
                        }
                        if (!(req.user)) {
                            logger.warn("User is not authorized to access the API or user session expired, hence redirecting to login page", {
                                orgName: orgName,
                                apiID: apiID
                            });
                            res.redirect(req.originalUrl.split("/api/")[0] + '/login');
                            return;
                        } else {
                            html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
                            res.send(html);
                            return;
                        }
                    }
                    apiDetail = await util.invokeApiRequest(req, 'GET', controlPlaneUrl + `/apis/${metaData.apiReferenceID}`, null, null);
                } catch (error) {
                    logger.warn("Error checking API authorization from control plane, proceeding without authorization check", {
                        orgName: orgName,
                        apiID: apiID,
                        error: error.message
                    });
                }
            } else {
                logger.debug("Control plane is disabled, skipping API authorization check", {
                    orgName: orgName,
                    apiID: apiID
                });
            }
            let subscriptionPlans = await util.appendSubscriptionPlanDetails(orgID, metaData.subscriptionPolicies);
            let providerUrl;
            if (metaData.provider === "WSO2") {
                providerUrl = '#subscriptionPlans';
            } else {
                const providerList = await adminService.getAllProviders(orgID);
                providerUrl = providerList.find(provider => provider.name === metaData.provider)?.providerURL || '#subscriptionPlans';
            }
            //check whether api content exists
            let loadDefault = false
            let apiDetails = "";
            let schemaDefinition = "";
            let apiDefinition = {};
            const markdownResponse = await apiDao.getAPIFile(constants.FILE_NAME.API_MD_CONTENT_FILE_NAME, constants.DOC_TYPES.API_LANDING, orgID, apiID);
            if (!markdownResponse) {
                let additionalAPIContentResponse = await apiDao.getAPIFile(constants.FILE_NAME.API_HBS_CONTENT_FILE_NAME, constants.DOC_TYPES.API_LANDING, orgID, apiID);
                if (!additionalAPIContentResponse) {
                    loadDefault = true;
                    if (
                      metaData.apiInfo &&
                      metaData.apiInfo.apiType !== "GraphQL" &&
                      metaData.apiInfo.apiType !== "WS"
                    ) {
                        apiDefinition = "";
                        apiDefinition = await apiDao.getAPIFile(constants.FILE_NAME.API_DEFINITION_FILE_NAME, constants.DOC_TYPES.API_DEFINITION, orgID, apiID);
                        apiDefinition = apiDefinition.API_FILE.toString(constants.CHARSET_UTF8);
                        apiDetails = await parseSwagger(JSON.parse(apiDefinition))
                        if (metaData.endPoints.productionURL === "" && metaData.endPoints.sandboxURL === "") {
                            apiDetails["serverDetails"] = "";
                        } else {
                            apiDetails["serverDetails"] = metaData.endPoints;
                        }
                    }
                    if (metaData.apiInfo.apiType === "WS") {
                        apiDefinition = "";
                        apiDefinition = await apiDao.getAPIFile(constants.FILE_NAME.API_DEFINITION_FILE_NAME, constants.DOC_TYPES.API_DEFINITION, orgID, apiID);
                        apiDefinition = apiDefinition.API_FILE.toString(constants.CHARSET_UTF8);
                        apiDetails = await parseAsyncAPI(JSON.parse(apiDefinition))
                        if (metaData.endPoints.productionURL === "" && metaData.endPoints.sandboxURL === "") {
                            apiDetails["serverDetails"] = "";
                        } else {
                            apiDetails["serverDetails"] = metaData.endPoints;
                        }
                    }
                    if (constants.API_TYPE.MCP === metaData.apiInfo?.apiType) {
                        try {
                            let rawSchema = await apiDao.getAPIFile(
                                constants.FILE_NAME.SCHEMA_DEFINITION_FILE_NAME,
                                constants.DOC_TYPES.SCHEMA_DEFINITION,
                                orgID,
                                apiID
                            );
                            const schemaString = rawSchema.API_FILE.toString(constants.CHARSET_UTF8);
                            schemaDefinition = JSON.parse(schemaString);
                        } catch (err) {
                            logger.error("Failed to load or parse schema definition", {
                                orgID: orgID,
                                apiID: apiID,
                                error: error.message, 
                                stack: error.stack
                            });
                            throw err;
                        }
                    }
                }
            }
            let appList = [];
            if (req.user) {
                let applications = await adminDao.getApplications(orgID, req.user.sub);
                if (applications.length > 0) {
                    appList = await Promise.all(
                        applications.map(async (app) => {
                            const subscription = await adminDao.getAppApiSubscription(orgID, app.APP_ID, metaData.apiID);
                            return {
                                ...new ApplicationDTO(app),
                                subscribed: subscription.length > 0,
                            };
                        })
                    );
                }
            }
            let profile = null;
            if (req.user) {
                profile = {
                    imageURL: req.user.imageURL,
                    firstName: req.user.firstName,
                    lastName: req.user.lastName,
                    email: req.user.email,
                }
            }
            templateContent = {
                isAuthenticated: req.isAuthenticated(),
                applications: appList,
                provider: metaData.provider,
                providerUrl: providerUrl,
                apiMetadata: metaData,
                subscriptionPlans: subscriptionPlans,
                baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
                schemaUrl: `${req.protocol}://${req.get('host')}${constants.ROUTE.DEVPORTAL_ASSETS_BASE_PATH}${orgID}/${constants.ROUTE.API_FILE_PATH}${apiID}${constants.API_TEMPLATE_FILE_NAME}${constants.FILE_NAME.API_DEFINITION_XML}`,
                loadDefault: loadDefault,
                resources: apiDetails,
                orgID: orgID,
                schemaDefinition: schemaDefinition,
                scopes: apiDetail?.scopes,
                devportalMode: devportalMode,
                profile: req.isAuthenticated() ? profile : null,
                isReadOnlyMode: config.readOnlyMode
            };
            if (metaData.apiInfo.apiType == "MCP") {
                html = await renderTemplateFromAPI(templateContent, orgID, orgName, "pages/mcp-landing", viewName);
            } else {
                html = await renderTemplateFromAPI(templateContent, orgID, orgName, "pages/api-landing", viewName);
            }
        } catch (error) {
            logger.error(`Failed to load api content`, {
                orgName: orgName,
                error: error.message, 
                stack: error.stack
            });
            const templateContent = {
                baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
                devportalMode: devportalMode,
            }
            if (Number(error?.statusCode) === 401) {
                templateContent.errorMessage = constants.ERROR_MESSAGE.COMMON_AUTH_ERROR_MESSAGE;
                html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
            } else {
                templateContent.errorMessage = constants.ERROR_MESSAGE.COMMON_ERROR_MESSAGE;
                html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
            }
            res.send(html);
        }
        res.send(html);
    }
}

const loadAPIDefinition = async (orgName, viewName, apiHandle) => {

    let metaData, templateContent = {};
    if (config.mode === constants.DEV_MODE) {
        metaData = loadAPIMetaDataFromFile(apiHandle);
        let apiDefinition = path.join(process.cwd(), filePrefix + '../mock', apiHandle + '/apiDefinition.json');
        if (fs.existsSync(apiDefinition)) {
            apiDefinition = await fs.readFileSync(apiDefinition, constants.CHARSET_UTF8);
        }
        templateContent.apiType = metaData.apiInfo.apiType;
        templateContent.swagger = JSON.parse(apiDefinition);
    } else {
        const orgID = await adminDao.getOrgId(orgName);
        const apiID = await apiDao.getAPIId(orgID, apiHandle);
        metaData = await apiMetadataService.getMetadataFromDB(orgID, apiID, viewName);
        const data = metaData ? JSON.stringify(metaData) : {};
        metaData = JSON.parse(data);
        let apiDefinition = await apiDao.getAPIFile(constants.FILE_NAME.API_DEFINITION_FILE_NAME, constants.DOC_TYPES.API_DEFINITION, orgID, apiID);
        apiDefinition = apiDefinition.API_FILE.toString(constants.CHARSET_UTF8);
        templateContent.apiType = metaData.apiInfo.apiType;
        if (metaData.apiInfo.apiType === constants.API_TYPE.WS) {
            templateContent.asyncapi = apiDefinition;
        } else {
            templateContent.swagger = apiDefinition;
        }
        templateContent.metaData = metaData;
    }
    return templateContent;
}

const loadDocsPage = async (req, res) => {

    const { orgName, apiHandle, viewName, docType } = req.params;
    let html = "";
    if (config.mode === constants.DEV_MODE) {
        const apiMetadata = await loadAPIMetaDataFromFile(apiHandle);
        const docNames = apiMetadata.docTypes;
        const orgDetails = await adminDao.getOrganization(orgName);
        const devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.API_TYPE.DEFAULT;
        const templateContent = {
            apiMD: await loadMarkdown("api-doc.md", filePrefix + '../mock/' + apiHandle + "/" + docType),
            baseUrl: constants.BASE_URL + config.port + "/views/" + viewName + "/api/" + apiHandle,
            docTypes: docNames,
            devportalMode: devportalMode,
            apiType: apiMetadata.apiInfo?.apiType
        }
        html = renderTemplate(filePrefix + 'pages/docs/page.hbs', filePrefix + 'layout/main.hbs', templateContent, false);
    } else {
        const orgDetails = await adminDao.getOrganization(orgName);
        const devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.API_TYPE.DEFAULT;

        try {
            const orgID = await adminDao.getOrgId(orgName);
            const apiID = await apiDao.getAPIId(orgID, apiHandle);
            const viewName = req.params.viewName;
            const docNames = await apiMetadataService.getAPIDocTypes(orgID, apiID);

            let profile = null;
            if (req.user) {
                profile = {
                    imageURL: req.user.imageURL,
                    firstName: req.user.firstName,
                    lastName: req.user.lastName,
                    email: req.user.email,
                }
            }

            const apiMetadata = await apiDao.getAPIMetadata(orgID, apiID);
            let apiType = apiMetadata[0].dataValues.API_TYPE;
            const templateContent = {
                baseUrl: '/' + orgName + '/views/' + viewName + "/api/" + apiHandle,
                docTypes: docNames,
                apiType: apiType,
                profile: req.isAuthenticated() ? profile : null,
                devportalMode: devportalMode,
            };
            html = await renderTemplateFromAPI(templateContent, orgID, orgName, "pages/docs", viewName);
        } catch (error) {
            const templateContent = {
                baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
                baseDocUrl: '/' + orgName + '/views/' + viewName + "/api/" + apiHandle,
                errorMessage: constants.ERROR_MESSAGE.COMMON_ERROR_MESSAGE,
                devportalMode: devportalMode,
            }
            logger.error(`Failed to load api docs`, {
                orgName: orgName,
                error: error.message, 
                stack: error.stack
            });
            html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
        }
    }
    res.send(html);
}

const loadDocument = async (req, res) => {
    const { orgName, apiHandle, viewName, docType, docName } = req.params;
    const isWebSocketTryout = req.query.tryout ? true : false;
    const orgDetails = await adminDao.getOrganization(orgName);
    const devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.API_TYPE.DEFAULT;
    let baseDocUrl = '/' + orgName + '/views/' + viewName + "/api/" + apiHandle
    if (req.originalUrl.includes('/mcp')) {
        baseDocUrl = '/' + orgName + '/views/' + viewName + "/mcp/" + apiHandle
    }
    try {
        const hbs = exphbs.create({});
        let templateContent = {
            "isAPIDefinition": false,
            "isWebSocketTryout": isWebSocketTryout
        };
        const cpOrgID = orgDetails.ORGANIZATION_IDENTIFIER;
        req.cpOrgID = cpOrgID;
        const definitionResponse = await loadAPIDefinition(orgName, viewName, apiHandle);
        templateContent.apiType = definitionResponse.apiType;
        let apiMetadata = definitionResponse.metaData;
        //check whether user has access to the API via control plane
        if (config.controlPlane?.enabled !== false) {
            try {
                let apiName = apiMetadata ? apiMetadata.apiHandle?.split('-v')[0] : "";
                const version = apiMetadata ? apiMetadata.apiInfo.apiVersion : "";
                let allowedAPIList = await util.invokeApiRequest(req, 'GET', `${controlPlaneUrl}/apis?query=name:"${apiName}"+version:${version}`, {}, {});
                if (allowedAPIList.count == 0) {
                    apiName = apiMetadata.apiInfo.apiName;
                    allowedAPIList = await util.invokeApiRequest(req, 'GET', `${controlPlaneUrl}/apis?query=name:"${apiName}"+version:${version}`, {}, {});
                }
                if (allowedAPIList.count === 0) {
                    templateContent = {
                        errorMessage: constants.ERROR_MESSAGE.UNAUTHORIZED_API,
                        baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
                        baseDocUrl: baseDocUrl,
                        devportalMode: devportalMode,
                    }
                    html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
                    res.send(html);
                    return;
                }
            } catch (error) {
                logger.warn("Error checking API authorization from control plane, proceeding without authorization check", {
                    orgName: orgName,
                    error: error.message
                });
            }
        } else {
            logger.debug("Control plane is disabled, skipping API authorization check", {
                orgName: orgName
            });
        }

        //load API definition
        if (req.originalUrl.includes(constants.FILE_NAME.API_SPECIFICATION_PATH)) {

            if (definitionResponse.apiType !== constants.API_TYPE.WS) {
                let modifiedSwagger = replaceEndpointParams(JSON.parse(definitionResponse.swagger), apiMetadata.endPoints.productionURL, apiMetadata.endPoints.sandboxURL);
                if (config.controlPlane?.enabled !== false) {
                    try {
                        const response = await util.invokeApiRequest(req, 'GET', controlPlaneUrl + `/apis/${apiMetadata.apiReferenceID}`, null, null);
                        if (response.securityScheme.includes("api_key")) {
                            modifiedSwagger.components.securitySchemes.ApiKeyAuth = { "type": "apiKey", "name": `${response.apiKeyHeader}`, "in": "header" };
                            for (let path in modifiedSwagger.paths) {
                                for (let method in modifiedSwagger.paths[path]) {
                                    if (modifiedSwagger.paths[path].hasOwnProperty(method)) {
                                        modifiedSwagger.paths[path][method].security[0].ApiKeyAuth = [];
                                    }
                                }
                            }
                        }
                    } catch (error) {
                        logger.warn("Error fetching API security details from control plane", {
                            orgName: orgName,
                            error: error.message
                        });
                    }
                }
                templateContent.swagger = JSON.stringify(modifiedSwagger);
            } else {
                let modifiedAsyncAPI = replaceEndpointParamsAsyncAPI(JSON.parse(definitionResponse.asyncapi), apiMetadata.endPoints.productionURL, apiMetadata.endPoints.sandboxURL);
                templateContent.asyncapi = JSON.stringify(modifiedAsyncAPI);
                templateContent.isWebSocketTryout = isWebSocketTryout;
            }
            templateContent.isAPIDefinition = true;
        }
        if (config.mode === constants.DEV_MODE) {
            const apiMetadata = await loadAPIMetaDataFromFile(apiHandle);
            const docNames = apiMetadata.docTypes;
            let apiMD = "";
            if (docType !== undefined && docName !== undefined) {
                const filePath = path.join(process.cwd(), filePrefix + '../mock', apiHandle + "/" + docType + "/" + docName);
                if (fs.existsSync(filePath) && (filePath.endsWith('.hbs') || filePath.endsWith('.html'))) {
                    hbs.handlebars.registerPartial(constants.FILE_NAME.API_DOC_PARTIAL_NAME, fs.readFileSync(filePath, constants.CHARSET_UTF8));
                }
                apiMD = await loadMarkdown(docName, filePrefix + '../mock/' + apiHandle + "/" + docType);
            }
            templateContent.baseUrl = constants.BASE_URL + config.port + "/views/" + viewName + "/api/" + apiHandle;
            templateContent.docTypes = docNames;
            templateContent.apiMD = apiMD;
            templateContent.apiType = apiMetadata.apiInfo?.apiType;
            html = renderTemplate(filePrefix + 'pages/docs/page.hbs', filePrefix + 'layout/main.hbs', templateContent, false);
        } else {

            try {
                const orgID = await adminDao.getOrgId(orgName);
                const apiID = await apiDao.getAPIId(orgID, apiHandle);
                const viewName = req.params.viewName;
                const docNames = await apiMetadataService.getAPIDocTypes(orgID, apiID);
                const apiMetadata = await apiDao.getAPIMetadata(orgID, apiID);
                let apiType = apiMetadata[0].dataValues.API_TYPE;
                templateContent.baseUrl = '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName;
                templateContent.baseDocUrl = baseDocUrl;                
                templateContent.docTypes = docNames;
                let profile = null;
                if (req.user) {
                    profile = {
                        imageURL: req.user.imageURL,
                        firstName: req.user.firstName,
                        lastName: req.user.lastName,
                        email: req.user.email,
                    }
                }
                templateContent.profile = req.isAuthenticated() ? profile : null;
                templateContent.apiType = apiType;
                templateContent.devportalMode = devportalMode;
                html = await renderTemplateFromAPI(templateContent, orgID, orgName, "pages/docs", viewName);
            } catch (error) {
                const templateContent = {
                    devportalMode: devportalMode,
                    baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
                    baseDocUrl: baseDocUrl,
                    errorMessage: constants.ERROR_MESSAGE.COMMON_ERROR_MESSAGE,
                }
                logger.error('Failed to load api content', { 
                    error: error.message, 
                    stack: error.stack
                });
                html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
            }
        }
        res.send(html);
    } catch (error) {
        const templateContent = {
            baseUrl: '/' + orgName + '/views/' + viewName,
            baseDocUrl: baseDocUrl,
            devportalMode: orgDetails.ORG_CONFIG?.devportalMode || constants.API_TYPE.DEFAULT,
        }
        if (Number(error?.statusCode) === 401) {
            templateContent.errorMessage = constants.ERROR_MESSAGE.COMMON_AUTH_ERROR_MESSAGE;
            html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
        } else {
            templateContent.errorMessage = constants.ERROR_MESSAGE.COMMON_ERROR_MESSAGE;
            html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
        }
        res.send(html);
    }
}

async function loadAPIMetaDataList() {

    const mockAPIMetaDataPath = path.join(process.cwd(), filePrefix + '../mock', 'apiMetadata.json');
    let mockAPIMetaData = JSON.parse(fs.readFileSync(mockAPIMetaDataPath, 'utf-8'));
    mockAPIMetaData.forEach(element => {
        const randomNumber = Math.floor(Math.random() * 3) + 3;
        element.apiInfo.ratings = generateArray(randomNumber);
        element.apiInfo.ratingsNoFill = generateArray(5 - randomNumber);
    });
    return mockAPIMetaData;
}

async function loadAPIMetaDataListFromAPI(req, orgID, orgName, searchTerm, tags, viewName) {

    let groups = "";
    let groupList = [];
    if (req.user && req.user[constants.ROLES.GROUP_CLAIM]) {
        groups = req.user[constants.ROLES.GROUP_CLAIM];
    }
    if (groups !== "") {
        groupList = groups.split(" ");
    }
    let metaData = await apiMetadataService.getMetadataListFromDB(orgID, groupList, searchTerm, tags, null, null, viewName);
    metaData.forEach(element => {
        const randomNumber = Math.floor(Math.random() * 3) + 3;
        element.apiInfo.ratings = generateArray(randomNumber);
        element.apiInfo.ratingsNoFill = generateArray(5 - randomNumber);
    });
    util.appendAPIImageURL(metaData, req, orgID);

    let data = JSON.stringify(metaData);
    return JSON.parse(data);
}

async function loadAPIMetaData(req, orgID, apiID, viewName) {

    let metaData = {};
    metaData = await apiMetadataService.getMetadataFromDB(orgID, apiID, viewName);
    if (metaData !== "") {
        const data = metaData ? JSON.stringify(metaData) : {};
        metaData = JSON.parse(data);
        //replace image urls
        let images = metaData.apiInfo.apiImageMetadata;
        for (const key in images) {
            let apiImageUrl = `${constants.ROUTE.DEVPORTAL_ASSETS_BASE_PATH}${orgID}${constants.ROUTE.API_FILE_PATH}${apiID}${constants.API_TEMPLATE_FILE_NAME}`;
            const modifiedApiImageURL = apiImageUrl + images[key];
            images[key] = modifiedApiImageURL;
        }
    }
    return metaData;
}

function loadAPIMetaDataFromFile(apiName) {

    const mockAPIDataPath = path.join(process.cwd(), filePrefix + '../mock', apiName + '/apiMetadata.json');
    return JSON.parse(fs.readFileSync(mockAPIDataPath, constants.CHARSET_UTF8));
}

async function parseSwagger(api) {
    try {
        // Extract general API info
        const title = api.info?.title || "No title";
        const apiDescription = api.info?.description || "No description available";
        //const servers = api.servers || [];

        // Extract endpoints
        const endpoints = Object.entries(api.paths || {}).map(([path, methods]) => ({
            path,
            methods: Object.keys(methods).map(method => ({
                method: method.toUpperCase(),
                summary: methods[method]?.summary || "No summary",
                description: methods[method]?.description || "No description",
            })),
        }));
        return { title, description: apiDescription, endpoints };
    } catch (error) {
        logger.error('Error parsing OpenAPI', { 
            error: error.message, 
            stack: error.stack
        });
    }
}

async function parseAsyncAPI(api) {
    try {
        // Extract general API info
        const title = api.info?.title || "No title";
        const apiDescription = api.info?.description || "No description available";
        const version = api.info?.version || "1.0.0";
        
        // Extract servers
        const servers = Object.entries(api.servers || {}).map(([name, server]) => ({
            name,
            url: server.url || "No URL",
            protocol: server.protocol || "Unknown",
            description: server.description || "No description"
        }));

        // Extract channels (AsyncAPI equivalent of endpoints)
        const channels = Object.entries(api.channels || {}).map(([channelName, channel]) => {
            const operations = [];
            
            // Extract publish operations
            if (channel.publish) {
                operations.push({
                    type: "publish",
                    summary: channel.publish.summary || "No summary",
                    description: channel.publish.description || "No description",
                    message: channel.publish.message || {}
                });
            }
            
            // Extract subscribe operations
            if (channel.subscribe) {
                operations.push({
                    type: "subscribe",
                    summary: channel.subscribe.summary || "No summary",
                    description: channel.subscribe.description || "No description",
                    message: channel.subscribe.message || {}
                });
            }

            return {
                name: channelName,
                operations
            };
        });

        // Extract messages
        const messages = Object.entries(api.components?.messages || {}).map(([messageName, message]) => ({
            name: messageName,
            summary: message.summary || "No summary",
            description: message.description || "No description",
            payload: message.payload || {}
        }));

        return { 
            title, 
            description: apiDescription, 
            version,
            servers, 
            channels, 
            messages 
        };
    } catch (error) {
        logger.error('Error parsing AsyncAPI', { 
            error: error.message, 
            stack: error.stack
        });
    }
}

function replaceEndpointParams(apiDefinition, prodEndpoint, sandboxEndpoint) {

    if (apiDefinition?.swagger?.startsWith('2.')) {
        if (prodEndpoint.trim().length !== 0) {
            apiDefinition.host = prodEndpoint.replace(/https?:\/\//, '');
            apiDefinition.schemes = [prodEndpoint.startsWith('https') ? 'https' : 'http'];
        }
    }
    let servers = [];
    if (prodEndpoint.trim().length !== 0) {
        servers.push({
            url: prodEndpoint
        });
    }
    if (sandboxEndpoint.trim().length !== 0) {
        servers.push({
            url: sandboxEndpoint
        });
    }
    apiDefinition.servers = servers;
    return apiDefinition;
}

function replaceEndpointParamsAsyncAPI(apiDefinition, prodEndpoint, sandboxEndpoint) {
    if (apiDefinition?.asyncapi && apiDefinition.asyncapi.startsWith('2.')) {
        if (prodEndpoint.trim().length !== 0) {
            apiDefinition.servers = {"production": {
                url: prodEndpoint,
                protocol: prodEndpoint.startsWith('ws') ? 'ws' : 'wss'
            }};
        }
        if (sandboxEndpoint.trim().length !== 0) {
            apiDefinition.servers["sandbox"] = {
                url: sandboxEndpoint,
                protocol: sandboxEndpoint.startsWith('ws') ? 'ws' : 'wss'
            };
        }
    }
    return apiDefinition;
}

module.exports = {
    loadAPIs,
    loadAPIContent,
    loadDocsPage,
    loadDocument,
};
