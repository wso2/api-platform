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
const { renderTemplate, renderTemplateFromAPI, isAiDisabledForPortal } = require('../utils/util');
const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const { logUserAction } = require('../middlewares/auditLogger');
const fs = require('fs');
const path = require('path');
const exphbs = require('express-handlebars');
const util = require('../utils/util');
const constants = require('../utils/constants');
const orgDao = require('../dao/organizationDao');
const apiDao = require('../dao/apiDao');
const apiFileDao = require('../dao/apiFileDao');
const viewDao = require('../dao/viewDao');
const subDao = require('../dao/subscriptionDao');
const apiMetadataService = require('../services/apiMetadataService');
const { apiUsesApiKeySecurity, findSubscriptionTokenHeader } = require('../utils/apiDefinitionUtil');
const sampleApiLoader = require('../utils/sampleApiLoader');
const apiWorkflowService = require('../services/apiWorkflowService');
const { buildSchema, getIntrospectionQuery, graphql: executeGraphQL } = require('graphql');
const yaml = require('../utils/yaml');
const generateArray = (length) => Array.from({ length });

const loadAPIs = async (req, res, next) => {

    const { orgName, viewName } = req.params;
    let html;
    if (config.designMode?.enabled) {
        const layoutPath = config.designMode.pathToLayout;
        const isMcpListing = req.originalUrl.includes('/mcps');
        const listingSamplesPath = isMcpListing ? config.designMode.mcpSamplesPath : config.designMode.apiSamplesPath;
        const metaDataList = await loadAPIMetaDataList(listingSamplesPath);
        for (const metaData of metaDataList) {
            metaData.subscriptionPlanDetails = metaData.subscriptionPlans;
        }
        const templateContent = {
            apiMetadata: metaDataList,
            baseUrl: constants.ROUTE.VIEWS_PATH + viewName,
            devMode: true,
        }
        const listingPage = isMcpListing ? 'pages/mcp' : 'pages/apis';
        html = renderTemplate(layoutPath + listingPage + '/page.hbs', layoutPath + 'layout/main.hbs', templateContent, false);
    } else {
        const orgDetails = await orgDao.get(orgName);
        const devportalMode = orgDetails.configuration?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
        try {
            const orgId = orgDetails.uuid;
            const searchTerm = req.query.query;
            const tags = req.query.tags;
            let metaDataList = await loadAPIMetaDataListFromAPI(req, orgId, orgName, searchTerm, tags, viewName);
            const apiData = await loadAPIMetaDataListFromAPI(req, orgId, orgName, searchTerm, tags, viewName);
            let apiTags = [];
            apiData.forEach(api => {
                if (api.tags) {
                    api.tags.forEach(tag => {
                        if (!apiTags.includes(tag)) {
                            apiTags.push(tag);
                        }
                    });
                }
            });

            for (const metaData of metaDataList) {
                metaData.subscriptionPlanDetails = await util.appendSubscriptionPlanDetails(orgId, metaData.subscriptionPlans);
            }

            // Load subscriptions for APIs with subscription plans (single call for all)
            if (req.user) {
                try {
                    const createdBy = req.user && util.resolveActor(req);
                    const localSubs = await subDao.list(orgId, { createdBy });
                    const subscribedApiIds = new Set(localSubs.map(sub => sub.api_uuid));
                    for (const metaData of metaDataList) {
                        const hasPlans = (metaData.subscriptionPlans || []).length > 0;
                        if (hasPlans) {
                            metaData.hasSubscription = subscribedApiIds.has(metaData.id);
                        }
                    }
                } catch (err) {
                    logger.warn('Failed to load subscriptions for API listing', {
                        error: err.message
                    });
                }
            }

            let profile = null;
            if (req.user) {
                profile = {
                    imageURL: req.user.imageURL,
                    firstName: req.user.firstName,
                    lastName: req.user.lastName,
                    email: req.user.email,
                    isAdmin: req.user.isAdmin,
                }
            }
            const isMcpPage = req.originalUrl.includes("/mcps");
            const filteredList = metaDataList.filter(api =>
                isMcpPage
                    ? api?.type === constants.API_TYPE.MCP
                    : api?.type !== constants.API_TYPE.MCP
            );

            const templateContent = {
                isAuthenticated: req.isAuthenticated(),
                apiMetadata: filteredList,
                tags: apiTags,
                baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
                orgId: orgId,
                profile: req.isAuthenticated() ? profile : null,
                devportalMode: devportalMode,
                applications: []
            };

            if (isMcpPage) {
                html = await renderTemplateFromAPI(templateContent, orgId, orgName, "pages/mcp", viewName);
            } else {
                html = await renderTemplateFromAPI(templateContent, orgId, orgName, "pages/apis", viewName);
            }
        } catch (error) {
            logger.error(constants.ERROR_MESSAGE.API_LISTING_LOAD_ERROR, {
                orgName: orgName,
                error: error.message, 
                stack: error.stack
            });
            if (Number(error?.statusCode) === 401) {
                logger.warn("User is not authorized to access the API or user session expired, hence redirecting to login page", {
                    orgName: orgName,
                });
                const err = Object.assign(new Error(constants.ERROR_MESSAGE.COMMON_AUTH_ERROR_MESSAGE), { status: 401 });
                return next(err);
            } else {
                error.status = 500;
                return next(error);
            }
        }
    }
    res.send(html);
}

const loadAPIContent = async (req, res, next) => {

    let html;
    const hbs = exphbs.create({});
    let { orgName, apiHandle, viewName } = req.params;

    if (config.designMode?.enabled) {
        const layoutPath = config.designMode.pathToLayout;
        const samplesPath = resolveSamplesPath(apiHandle);
        const metaData = loadAPIMetaDataFromFile(apiHandle);
        const apiDir = sampleApiLoader.getApiDir(apiHandle, samplesPath);
        const dirName = apiDir ? path.basename(apiDir) : apiHandle;
        const hbsContentPath = path.join(process.cwd(), samplesPath, dirName, constants.ARTIFACT_DIR.WEB, constants.FILE_NAME.API_HBS_CONTENT_FILE_NAME);

        const apiType = metaData?.type;
        const isMCP = apiType === constants.API_TYPE.MCP;
        let loadDefault = false;
        let apiDetails = '';
        let schemaDefinition = '';

        if (fs.existsSync(hbsContentPath)) {
            hbs.handlebars.registerPartial('api-content', fs.readFileSync(hbsContentPath, constants.CHARSET_UTF8));
        } else {
            loadDefault = true;
            if (isMCP) {
                // MCP: load schema definition (tools/resources/prompts) + server URL
                schemaDefinition = sampleApiLoader.getMcpSchema(apiHandle, samplesPath) || '';
                const mcpUrl = metaData.endPoints?.productionURL || metaData.endPoints?.sandboxURL || '';
                apiDetails = { serverDetails: mcpUrl ? { productionURL: mcpUrl, sandboxURL: metaData.endPoints?.sandboxURL || '' } : '' };
            } else {
                // REST/SOAP/WS: parse OpenAPI definition for default endpoint view
                const definitionContent = sampleApiLoader.getDefinition(apiHandle, samplesPath);
                if (definitionContent && apiType !== constants.API_TYPE.GRAPHQL && apiType !== constants.API_TYPE.SOAP && apiType !== constants.API_TYPE.WS && apiType !== constants.API_TYPE.WEBSUB) {
                    apiDetails = await parseSwagger(parseApiDefinitionContent(definitionContent));
                    apiDetails.serverDetails = (metaData.endPoints.productionURL || metaData.endPoints.sandboxURL)
                        ? metaData.endPoints : '';
                }
                if (apiType === constants.API_TYPE.SOAP) {
                    apiDetails = {};
                    apiDetails.serverDetails = (metaData.endPoints.productionURL || metaData.endPoints.sandboxURL)
                        ? metaData.endPoints : '';
                }
            }
        }

        const definitionResponse = await getAPIDefinition(orgName, viewName, apiHandle);
        const templateContent = {
            devMode: true,
            apiContent: '',
            loadDefault,
            resources: apiDetails,
            schemaDefinition,
            apiMetadata: metaData,
            subscriptionPlans: metaData.subscriptionPlans,
            baseUrl: constants.ROUTE.VIEWS_PATH + viewName,
            schemaUrl: `/mock/${apiHandle}/definition.yml`,
            showApiKeysNav: await resolveShowApiKeysNav(null, null, apiType, metaData, definitionResponse.swagger ?? null),
            showSubscriptionsNav: (metaData.subscriptionPlans || []).length > 0,
        }
        const landingPage = isMCP ? 'pages/mcp-landing' : 'pages/api-landing';
        html = renderTemplate(layoutPath + landingPage + '/page.hbs', layoutPath + 'layout/main.hbs', templateContent, false);
        res.send(html);
    } else {
        const orgDetails = await orgDao.get(orgName);
        const devportalMode = orgDetails.configuration?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
        try {
            const orgDetails = await orgDao.get(orgName);
            const orgId = orgDetails.uuid;
            const apiId = await apiDao.getId(orgId, apiHandle);
            const metaData = await loadAPIMetaData(req, orgId, apiId);
            
            // Log API access for audit trail
            logUserAction('API_ACCESS', req, {
                orgName: orgName,
                apiHandle: apiHandle,
                apiId: apiId,
            });
            
            let subscriptionPlans = await util.appendSubscriptionPlanDetails(orgId, metaData.subscriptionPlans);
            //check whether api content exists
            let loadDefault = false
            let apiDetails = "";
            let schemaDefinition = "";
            let apiDefinition = {};
            const markdownResponse = await apiFileDao.get(constants.FILE_NAME.API_MD_CONTENT_FILE_NAME, constants.DOC_TYPES.API_LANDING, orgId, apiId);
            if (!markdownResponse) {
                let additionalAPIContentResponse = await apiFileDao.get(constants.FILE_NAME.API_HBS_CONTENT_FILE_NAME, constants.DOC_TYPES.API_LANDING, orgId, apiId);
                if (!additionalAPIContentResponse) {
                    loadDefault = true;
                    if (
                      metaData.type &&
                      metaData.type !== constants.API_TYPE.GRAPHQL &&
                      metaData.type !== constants.API_TYPE.SOAP &&
                      metaData.type !== constants.API_TYPE.WS &&
                      metaData.type !== constants.API_TYPE.WEBSUB &&
                      metaData.type !== constants.API_TYPE.MCP
                    ) {
                        apiDefinition = await getApiDefinitionFileContent(orgId, apiId);
                        apiDetails = await parseSwagger(parseApiDefinitionContent(apiDefinition))
                        if (metaData.endPoints.productionURL === "" && metaData.endPoints.sandboxURL === "") {
                            apiDetails["serverDetails"] = "";
                        } else {
                            apiDetails["serverDetails"] = metaData.endPoints;
                        }
                    }
                    if (metaData.type === constants.API_TYPE.SOAP) {
                        apiDetails = {};
                        apiDetails["serverDetails"] = (metaData.endPoints.productionURL || metaData.endPoints.sandboxURL)
                            ? metaData.endPoints : "";
                    }
                    if (metaData.type === constants.API_TYPE.WS || metaData.type === constants.API_TYPE.WEBSUB) {
                        apiDefinition = await getApiDefinitionFileContent(orgId, apiId);
                        apiDetails = await parseAsyncAPI(parseApiDefinitionContent(apiDefinition))
                        if (metaData.endPoints.productionURL === "" && metaData.endPoints.sandboxURL === "") {
                            apiDetails["serverDetails"] = "";
                        } else {
                            apiDetails["serverDetails"] = metaData.endPoints;
                        }
                    }
                    if (metaData.type === constants.API_TYPE.GRAPHQL) {
                        apiDefinition = "";
                        apiDefinition = await apiFileDao.get(constants.FILE_NAME.API_DEFINITION_GRAPHQL, constants.DOC_TYPES.API_DEFINITION, orgId, apiId);
                        apiDefinition = apiDefinition.file_content.toString(constants.CHARSET_UTF8);
                        apiDetails = {
                            title: metaData.name || "No title",
                            description: metaData.description || "No description",
                            schema: apiDefinition 
                        };
                        if (metaData.endPoints.productionURL === "" && metaData.endPoints.sandboxURL === "") {
                            apiDetails["serverDetails"] = "";
                        } else {
                            apiDetails["serverDetails"] = metaData.endPoints;
                        }
                    }
                    if (constants.API_TYPE.MCP === metaData?.type) {
                        const mcpRemotes = metaData?.remotes || [];
                        const mcpProductionURL = mcpRemotes.length > 0 ? mcpRemotes[0].url : (metaData.endPoints?.productionURL || '');
                        apiDetails = {};
                        apiDetails['serverDetails'] = mcpProductionURL ? { productionURL: mcpProductionURL, sandboxURL: '' } : '';
                        try {
                            let rawSchema = await apiFileDao.getByType(
                                constants.DOC_TYPES.SCHEMA_DEFINITION,
                                orgId,
                                apiId
                            );
                            if (rawSchema) {
                                const schemaString = rawSchema.file_content.toString(constants.CHARSET_UTF8);
                                const schemaFileName = String(rawSchema.file_name || '').toLowerCase();
                                let parsed;
                                if (schemaFileName.endsWith('.yaml') || schemaFileName.endsWith('.yml')) {
                                    parsed = yaml.load(schemaString);
                                } else {
                                    parsed = JSON.parse(schemaString);
                                }
                                if (Array.isArray(parsed)) {
                                    schemaDefinition = {
                                        tools: parsed.filter(item => item.type === 'TOOL'),
                                        resources: parsed.filter(item => item.type === 'RESOURCE'),
                                        prompts: parsed.filter(item => item.type === 'PROMPT'),
                                    };
                                } else {
                                    schemaDefinition = parsed;
                                }
                            }
                        } catch (err) {
                            logger.error("Failed to load or parse schema definition", {
                                orgId: orgId,
                                apiId: apiId,
                                error: err.message,
                                stack: err.stack
                            });
                        }
                    }
                }
            }
            // Load subscriptions for APIs with subscription plans
            let subscriptions = [];
            const hasPlans = (subscriptionPlans || []).length > 0;
            if (req.user && hasPlans) {
                try {
                    const createdBy = req.user && util.resolveActor(req);
                    const localSubs = await subDao.list(orgId, { apiId: apiId, createdBy });
                    subscriptions = (localSubs || []).map(sub => ({
                        subscriptionId: sub.uuid,
                        // policyName (raw POLICY_NAME) is what isCurrentPlan compares against in the template.
                        // subscriptionPlanName keeps the human-readable label (DISPLAY_NAME when set).
                        policyName: sub.dp_subscription_plan?.handle || '',
                        subscriptionPlanName: sub.dp_subscription_plan?.display_name || '',
                        status: sub.status,
                        subscriptionToken: sub.token,
                        maskedToken: sub.token ? sub.token.slice(0, 4) + '****' + sub.token.slice(-4) : '',
                        customerName: null
                    }));
                } catch (err) {
                    logger.warn('Failed to load subscriptions', {
                        error: err.message, orgId, apiId
                    });
                }
            }
            let profile = null;
            if (req.user) {
                profile = {
                    imageURL: req.user.imageURL,
                    firstName: req.user.firstName,
                    lastName: req.user.lastName,
                    email: req.user.email,
                    isAdmin: req.user.isAdmin,
                }
            }
            // Build the definition download link from the file that is actually stored, resolved
            // by its doc-type (one definition per API). The stored name is the uploaded basename,
            // not a fixed constant, so guessing a filename here would 404 for any file uploaded
            // under a different name (e.g. a SOAP WSDL named 'service.wsdl'). MCP tools schemas live
            // under SCHEMA_DEFINITION and are served from /mcp-servers; everything else under
            // API_DEFINITION from /apis.
            const definitionDocType = metaData.type === constants.API_TYPE.MCP
                ? constants.DOC_TYPES.SCHEMA_DEFINITION
                : constants.DOC_TYPES.API_DEFINITION;
            const definitionAssetBasePath = metaData.type === constants.API_TYPE.MCP
                ? '/mcp-servers/'
                : constants.ROUTE.API_FILE_PATH;
            let schemaUrl = null;
            try {
                const definitionDoc = await apiFileDao.getByType(definitionDocType, orgId, apiId);
                if (definitionDoc?.file_name) {
                    schemaUrl = `${req.protocol}://${req.get('host')}${constants.DEVPORTAL_API.orgPath(orgId)}`
                        + `${definitionAssetBasePath}${apiId}/assets?type=${definitionDocType}`
                        + `&fileName=${encodeURIComponent(definitionDoc.file_name)}`;
                }
            } catch (schemaErr) {
                logger.debug('Could not resolve definition file for download link', {
                    orgId, apiId, error: schemaErr.message
                });
            }

            let apiDefinitionForNav = null;
            if (metaData?.type !== constants.API_TYPE.GRAPHQL && metaData?.type !== constants.API_TYPE.SOAP && metaData?.type !== constants.API_TYPE.MCP) {
                try {
                    apiDefinitionForNav = await getApiDefinitionFileContent(orgId, apiId);
                } catch (definitionErr) {
                    logger.debug('Could not load API definition for API keys nav check', {
                        orgId,
                        apiId,
                        error: definitionErr.message
                    });
                }
            }

            templateContent = {
                isAuthenticated: req.isAuthenticated(),
                apiMetadata: metaData,
                subscriptionPlans: subscriptionPlans,
                subscriptions: subscriptions,
                baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
                schemaUrl: schemaUrl,
                loadDefault: loadDefault,
                resources: apiDetails,
                orgId: orgId,
                schemaDefinition: schemaDefinition,
                scopes: [],
                devportalMode: devportalMode,
                profile: req.isAuthenticated() ? profile : null,
            };
            templateContent.showApiKeysNav = await resolveShowApiKeysNav(orgId, apiId, metaData.type, metaData, apiDefinitionForNav);
            templateContent.showSubscriptionsNav = (metaData?.subscriptionPlans || []).length > 0;
            templateContent.hasSubscriptionToken = !!findSubscriptionTokenHeader(apiDefinitionForNav);
            if (metaData.type == constants.API_TYPE.MCP) {
                html = await renderTemplateFromAPI(templateContent, orgId, orgName, "pages/mcp-landing", viewName);
            } else {
                html = await renderTemplateFromAPI(templateContent, orgId, orgName, "pages/api-landing", viewName);
            }
        } catch (error) {
            logger.error(`Failed to load api content`, {
                orgName: orgName,
                error: error.message,
                stack: error.stack
            });
            if (Number(error?.statusCode) === 401) {
                const err = Object.assign(new Error(constants.ERROR_MESSAGE.COMMON_AUTH_ERROR_MESSAGE), { status: 401 });
                return next(err);
            } else {
                error.status = 500;
                return next(error);
            }
        }
        res.send(html);
    }
}

const getAPIDefinition = async (orgName, viewName, apiHandle) => {

    let metaData, templateContent = {};
    if (config.designMode?.enabled) {
        metaData = loadAPIMetaDataFromFile(apiHandle);
        templateContent.apiType = metaData.type;
        templateContent.metaData = metaData;
        if (metaData.type === constants.API_TYPE.MCP) {
            const productionURL = metaData.endPoints?.productionURL || '';
            templateContent.swagger = JSON.stringify({ servers: [{ url: productionURL }] });
        } else {
            const definitionContent = sampleApiLoader.getDefinition(apiHandle, resolveSamplesPath(apiHandle));
            if (definitionContent) {
                templateContent.swagger = definitionContent;
            }
        }
    } else {
        const orgId = await orgDao.getId(orgName);
        const apiId = await apiDao.getId(orgId, apiHandle);
        metaData = await apiMetadataService.getMetadataFromDB(orgId, apiId, viewName);
        const data = metaData ? JSON.stringify(metaData) : {};
        metaData = JSON.parse(data);
        const apiType = metaData.type;
        templateContent.apiType = apiType;
        let apiDefinition;
        if (metaData.type === constants.API_TYPE.MCP) {
            const productionURL = metaData.endPoints?.productionURL || '';
            templateContent.swagger = JSON.stringify({ servers: [{ url: productionURL }] });
            // Load MCP schema so loadAPIDefinitionRaw can serve it via SPEC_FORMAT_MAP field:'schema'
            try {
                const rawSchema = await apiFileDao.getByType(constants.DOC_TYPES.SCHEMA_DEFINITION, orgId, apiId);
                if (rawSchema?.file_content) {
                    templateContent.schema = rawSchema.file_content.toString(constants.CHARSET_UTF8);
                }
            } catch (schemaErr) {
                logger.warn('Could not load MCP schema definition for raw spec', { orgId, apiId, error: schemaErr.message });
            }
        } else if (metaData.type === constants.API_TYPE.GRAPHQL) {
            apiDefinition = await apiFileDao.get(constants.FILE_NAME.API_DEFINITION_GRAPHQL, constants.DOC_TYPES.API_DEFINITION, orgId, apiId);
            templateContent.graphql = apiDefinition.file_content.toString(constants.CHARSET_UTF8);
        } else {
            apiDefinition = await getApiDefinitionFileContent(orgId, apiId);
            if (apiType === constants.API_TYPE.WS || apiType === constants.API_TYPE.WEBSUB) {
                templateContent.asyncapi = apiDefinition;
            } else {
                templateContent.swagger = apiDefinition;
            }
        }
        templateContent.metaData = metaData;
    }
    return templateContent;
}

const loadDocsPage = async (req, res, next) => {

    const { orgName, apiHandle, viewName } = req.params;
    let html = "";
    if (config.designMode?.enabled) {
        const layoutPath = config.designMode.pathToLayout;
        const apiMetadata = await loadAPIMetaDataFromFile(apiHandle);
        const docNames = apiMetadata.docTypes;
        const metaForNav = {
            refId: apiMetadata.refId,
        };
        // Load the definition so the API Keys nav is computed the same way as every
        // other API-scoped page — without it the check always returns false and the
        // API Keys item is hidden on the documentation page.
        const definitionResponse = await getAPIDefinition(orgName, viewName, apiHandle);
        const templateContent = {
            apiMD: '',
            baseUrl: constants.ROUTE.VIEWS_PATH + viewName + '/api/' + apiHandle,
            baseDocUrl: constants.ROUTE.VIEWS_PATH + viewName + '/api/' + apiHandle,
            docTypes: docNames,
            apiType: apiMetadata.type,
            apiName: apiMetadata.name || '',
            showApiKeysNav: await resolveShowApiKeysNav(null, null, apiMetadata.type, metaForNav, definitionResponse.swagger ?? null),
        }
        html = renderTemplate(layoutPath + 'pages/docs/page.hbs', layoutPath + 'layout/main.hbs', templateContent, false);
    } else {
        const orgDetails = await orgDao.get(orgName);
        const devportalMode = orgDetails.configuration?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;

        try {
            const orgId = await orgDao.getId(orgName);
            const apiId = await apiDao.getId(orgId, apiHandle);
            const viewName = req.params.viewName;
            const docNames = await apiMetadataService.getAPIDocTypes(orgId, apiId);

            let profile = null;
            if (req.user) {
                profile = {
                    imageURL: req.user.imageURL,
                    firstName: req.user.firstName,
                    lastName: req.user.lastName,
                    email: req.user.email,
                    isAdmin: req.user.isAdmin,
                }
            }

            const apiMetadata = await apiDao.get(orgId, apiId);
            let apiType = apiMetadata[0].dataValues.type;
            const metaForNav = {
                refId: apiMetadata[0].dataValues.ref_id,
            };

            const templateContent = {
                baseUrl: '/' + orgName + '/views/' + viewName + "/api/" + apiHandle,
                baseDocUrl: '/' + orgName + '/views/' + viewName + "/api/" + apiHandle,
                docTypes: docNames,
                apiType: apiType,
                apiName: apiMetadata[0].dataValues.name || '',
                profile: req.isAuthenticated() ? profile : null,
                devportalMode: devportalMode,
                // resolveShowApiKeysNav returns false early for GraphQL/MCP/SOAP and lazily
                // fetches the definition itself for the remaining types, so no preload here.
                showApiKeysNav: await resolveShowApiKeysNav(orgId, apiId, apiType, metaForNav),
            };
            html = await renderTemplateFromAPI(templateContent, orgId, orgName, "pages/docs", viewName);
        } catch (error) {
            logger.error(`Failed to load api docs`, {
                orgName: orgName,
                error: error.message,
                stack: error.stack
            });
            error.status = 500;
            return next(error);
        }
    }
    res.send(html);
}

const loadDocument = async (req, res, next) => {
    const { orgName, apiHandle, viewName, docType, docName } = req.params;

    if (config.designMode?.enabled) {
        const layoutPath = config.designMode.pathToLayout;
        const metaData = loadAPIMetaDataFromFile(apiHandle);
        const isSpecPage = req.originalUrl.includes(constants.FILE_NAME.API_SPECIFICATION_PATH);
        const definitionResponse = await getAPIDefinition(orgName, viewName, apiHandle);
        let templateContent = {
            isAPIDefinition: false,
            isWebSocketTryout: false,
            isGraphQLTryout: false,
        };
        templateContent.apiType = definitionResponse.apiType;
        if (isSpecPage && definitionResponse.swagger) {
            const specType = definitionResponse.apiType;
            const tryoutEnabled = !!req.query.tryout;
            if (specType === constants.API_TYPE.WS || specType === constants.API_TYPE.WEBSUB) {
                templateContent.asyncapi = JSON.stringify(parseApiDefinitionContent(definitionResponse.swagger));
                templateContent.isWebSocketTryout = tryoutEnabled;
            } else if (specType === constants.API_TYPE.GRAPHQL) {
                templateContent.isGraphQLTryout = tryoutEnabled;
                if (tryoutEnabled) {
                    const schemaAsIntrospectionJSON = await convertSDLToIntrospection(definitionResponse.swagger);
                    templateContent.graphqlSchemaAsIntrospectionJSON = schemaAsIntrospectionJSON ? JSON.stringify(schemaAsIntrospectionJSON) : null;
                    templateContent.graphqlSecurityScheme = '[]';
                    templateContent.graphqlApiKeyHeader = config.security?.serviceApiKey?.headerName || 'apikey';
                    templateContent.apiMetadata = metaData;
                } else {
                    templateContent.graphql = JSON.stringify(definitionResponse.swagger);
                    templateContent.apiMetadataJSON = JSON.stringify(metaData || {});
                }
            } else if (specType !== constants.API_TYPE.SOAP) {
                templateContent.swagger = JSON.stringify(parseApiDefinitionContent(definitionResponse.swagger));
            }
            templateContent.isAPIDefinition = true;
        }
        if (!isSpecPage && docType !== undefined && docName !== undefined) {
            const raw = sampleApiLoader.getDocMarkdown(apiHandle, docName, resolveSamplesPath(apiHandle), docType) || '';
            templateContent.apiMD = raw ? require('marked').parse(raw) : '';
        }
        templateContent.baseUrl = constants.ROUTE.VIEWS_PATH + viewName;
        templateContent.baseDocUrl = constants.ROUTE.VIEWS_PATH + viewName + '/api/' + apiHandle;
        templateContent.docTypes = metaData.docTypes;
        templateContent.currentDocName = docName || null;
        templateContent.currentDocType = docType || null;
        templateContent.apiName = metaData.name || '';
        const metaForNav = { refId: metaData.refId };
        // Pass the definition so the API Keys nav is computed the same way as every
        // other API-scoped page — without it the check always returns false and the
        // API Keys item is hidden on the documentation page.
        templateContent.showApiKeysNav = await resolveShowApiKeysNav(null, null, definitionResponse.apiType, metaForNav, definitionResponse.swagger ?? null);
        const html = renderTemplate(layoutPath + 'pages/docs/page.hbs', layoutPath + 'layout/main.hbs', templateContent, false);
        res.send(html);
        return;
    }

    const orgDetails = await orgDao.get(orgName);
    const devportalMode = orgDetails.configuration?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
    let baseDocUrl = '/' + orgName + '/views/' + viewName + "/api/" + apiHandle
    if (req.originalUrl.includes('/mcp')) {
        baseDocUrl = '/' + orgName + '/views/' + viewName + "/mcp/" + apiHandle
    }
    try {
        let templateContent = {
            "isAPIDefinition": false,
            "isWebSocketTryout": false,
            "isGraphQLTryout": false
        };
        const definitionResponse = await getAPIDefinition(orgName, viewName, apiHandle);
        templateContent.apiType = definitionResponse.apiType;
        
        const tryoutEnabled = req.query.tryout ? true : false;
        if (definitionResponse.apiType === constants.API_TYPE.WS || definitionResponse.apiType === constants.API_TYPE.WEBSUB) {
            templateContent.isWebSocketTryout = tryoutEnabled;
        } else if (definitionResponse.apiType === constants.API_TYPE.GRAPHQL) {
            templateContent.isGraphQLTryout = tryoutEnabled;
        }
        let apiMetadata = definitionResponse.metaData;
        
        const isMCPFromRegistry = apiMetadata?.type === constants.API_TYPE.MCP && !apiMetadata?.refId;

        //load API definition
        if (req.originalUrl.includes(constants.FILE_NAME.API_SPECIFICATION_PATH)) {

            if (isMCPFromRegistry) {
                const remotes = apiMetadata?.remotes || [];
                const serverUrl = remotes.length > 0 ? remotes[0].url : '';
                templateContent.swagger = JSON.stringify({ servers: [{ url: serverUrl }] });
            } else if (definitionResponse.apiType === constants.API_TYPE.MCP) {
                // CP-registered MCP: use server URL from endPoints
                templateContent.swagger = definitionResponse.swagger;
            } else if (definitionResponse.apiType !== constants.API_TYPE.WS && definitionResponse.apiType !== constants.API_TYPE.GRAPHQL && definitionResponse.apiType !== constants.API_TYPE.WEBSUB) {
                let modifiedSwagger;
                try {
                    const parsedSwagger = parseApiDefinitionContent(definitionResponse.swagger);
                    modifiedSwagger = replaceEndpointParams(parsedSwagger, apiMetadata.endPoints.productionURL, apiMetadata.endPoints.sandboxURL);
                } catch (error) {
                    if (definitionResponse.apiType === constants.API_TYPE.SOAP) {
                        logger.warn('SOAP XML definition is not supported for interactive spec rendering. Skipping parse step.', {
                            orgName,
                            apiHandle,
                            error: error.message
                        });
                        modifiedSwagger = null;
                    } else {
                        throw error;
                    }
                }

                if (modifiedSwagger) {
                    // Add apiKey security scheme headers as operation parameters
                    // so Stoplight Elements renders input fields in the try-it panel
                    if (modifiedSwagger.components?.securitySchemes) {
                        for (const scheme of Object.values(modifiedSwagger.components.securitySchemes)) {
                            if (scheme.type === 'apiKey' && scheme.in === 'header' && scheme.name) {
                                for (const pathItem of Object.values(modifiedSwagger.paths || {})) {
                                    for (const method of ['get', 'post', 'put', 'delete', 'patch', 'head', 'options']) {
                                        if (pathItem[method]) {
                                            if (!pathItem[method].parameters) {
                                                pathItem[method].parameters = [];
                                            }
                                            const exists = pathItem[method].parameters.some(
                                                p => p.name === scheme.name && p.in === 'header'
                                            );
                                            if (!exists) {
                                                pathItem[method].parameters.push({
                                                    name: scheme.name,
                                                    in: 'header',
                                                    required: false,
                                                    schema: { type: 'string' },
                                                    description: scheme.description || 'API key for subscription-based access'
                                                });
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }

                    templateContent.swagger = JSON.stringify(modifiedSwagger);
                }
            } else if (definitionResponse.apiType === constants.API_TYPE.GRAPHQL) {
                if (templateContent.isGraphQLTryout && definitionResponse.graphql) {
                    const schemaAsIntrospectionJSON = await convertSDLToIntrospection(definitionResponse.graphql);
                    templateContent.graphqlSchemaAsIntrospectionJSON = schemaAsIntrospectionJSON ? JSON.stringify(schemaAsIntrospectionJSON) : null;
                    templateContent.graphqlSecurityScheme = '[]';
                    templateContent.graphqlApiKeyHeader = config.security?.serviceApiKey?.headerName || 'apikey';
                } else {
                    templateContent.graphql = definitionResponse.graphql ? JSON.stringify(definitionResponse.graphql) : '""';
                    templateContent.apiMetadataJSON = JSON.stringify(apiMetadata || {});
                }
                templateContent.apiMetadata = apiMetadata;
            }
             else {
                const parsedAsyncAPI = parseApiDefinitionContent(definitionResponse.asyncapi);
                let modifiedAsyncAPI = replaceEndpointParamsAsyncAPI(parsedAsyncAPI, apiMetadata.endPoints.productionURL, apiMetadata.endPoints.sandboxURL);
                templateContent.asyncapi = JSON.stringify(modifiedAsyncAPI);
            }
            templateContent.isAPIDefinition = true;
        }
        try {
            const orgId = await orgDao.getId(orgName);
            const apiId = await apiDao.getId(orgId, apiHandle);
            const viewName = req.params.viewName;
            let docNames = await apiMetadataService.getAPIDocTypes(orgId, apiId);
            const apiMetadata = await apiDao.get(orgId, apiId);
            let apiType = apiMetadata[0].dataValues.type;
            // All MCPs (registry and CP) need a Specification entry in the sidebar
            if (apiType === constants.API_TYPE.MCP && !docNames.some(d => d.type === constants.DOC_TYPES.DOCS.API_DEFINITION)) {
                docNames = [{ type: constants.DOC_TYPES.DOCS.API_DEFINITION }, ...docNames];
            }
            templateContent.baseUrl = '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName;
            templateContent.baseDocUrl = baseDocUrl;
            templateContent.docTypes = docNames;
            templateContent.currentDocName = docName || null;
            templateContent.currentDocType = docType || null;
            templateContent.apiName = apiMetadata[0].dataValues.name || '';
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
            const row = apiMetadata[0].dataValues;
            const metaForNav = {
                refId: row.ref_id,
            };
            // Compute the API Keys nav the same way as every other API-scoped page.
            templateContent.showApiKeysNav = await resolveShowApiKeysNav(orgId, apiId, apiType, metaForNav, definitionResponse.swagger ?? null);
            html = await renderTemplateFromAPI(templateContent, orgId, orgName, "pages/docs", viewName);
        } catch (error) {
            logger.error('Failed to load api content', {
                error: error.message,
                stack: error.stack
            });
            error.status = 500;
            return next(error);
        }
        res.send(html);
    } catch (error) {
        if (Number(error?.statusCode) === 401) {
            const err = Object.assign(new Error(constants.ERROR_MESSAGE.COMMON_AUTH_ERROR_MESSAGE), { status: 401 });
            return next(err);
        } else {
            error.status = 500;
            return next(error);
        }
    }
}

async function loadAPIMetaDataList(samplesPath = config.designMode.apiSamplesPath) {

    const apis = sampleApiLoader.loadAll(samplesPath);
    apis.forEach(element => {
        const randomNumber = Math.floor(Math.random() * 3) + 3;
        element.ratings = generateArray(randomNumber);
        element.ratingsNoFill = generateArray(5 - randomNumber);
    });
    return apis;
}

async function loadAPIMetaDataListFromAPI(req, orgId, orgName, searchTerm, tags, viewName) {

    let metaData = await apiMetadataService.getMetadataListFromDB(orgId, searchTerm, tags, null, null, viewName);
    metaData.forEach(element => {
        const randomNumber = Math.floor(Math.random() * 3) + 3;
        element.ratings = generateArray(randomNumber);
        element.ratingsNoFill = generateArray(5 - randomNumber);
    });
    util.appendAPIImageURL(metaData, req, orgId);

    let data = JSON.stringify(metaData);
    return JSON.parse(data);
}

async function loadAPIMetaData(req, orgId, apiId, viewName) {

    let metaData = {};
    metaData = await apiMetadataService.getMetadataFromDB(orgId, apiId, viewName);
    if (metaData !== "") {
        const data = metaData ? JSON.stringify(metaData) : {};
        metaData = JSON.parse(data);
        //replace image urls
        // Use the handle (metaData.id), not the raw uuid — the /apis/{apiId}/assets
        // endpoint resolves apiId via getIdExcludingType, which matches on `handle`.
        // A uuid here never matches and the asset request 404s. This mirrors
        // appendAPIImageURL (listing page), which already uses the handle.
        // orgId is appended so the (public) image endpoint can resolve the view for
        // anonymous visitors with no session — mirrors appendAPIImageURL and getOrgAsset.
        let images = metaData.apiImageMetadata;
        for (const key in images) {
            let apiImageUrl = `${constants.DEVPORTAL_API.orgPath(orgId)}${constants.ROUTE.API_FILE_PATH}${metaData.id}${constants.API_TEMPLATE_FILE_NAME}`;
            const modifiedApiImageURL = apiImageUrl + images[key] + `${constants.ORG_ID_PARAM}${orgId}`;
            images[key] = modifiedApiImageURL;
        }
    }
    return metaData;
}

function loadAPIMetaDataFromFile(apiName) {
    // Try the API samples path first, then the MCP samples path
    try {
        return sampleApiLoader.loadOne(apiName, config.designMode.apiSamplesPath);
    } catch (_) {
        return sampleApiLoader.loadOne(apiName, config.designMode.mcpSamplesPath);
    }
}

function resolveSamplesPath(apiHandle) {
    // Returns the samples directory that contains the given handle
    try {
        sampleApiLoader.loadOne(apiHandle, config.designMode.apiSamplesPath);
        return config.designMode.apiSamplesPath;
    } catch (_) {
        return config.designMode.mcpSamplesPath;
    }
}

async function getApiDefinitionFileContent(orgId, apiId) {
    const apiDefinition = await apiFileDao.getDoc(constants.DOC_TYPES.API_DEFINITION, orgId, apiId);
    if (apiDefinition?.file_content) {
        return apiDefinition.file_content.toString(constants.CHARSET_UTF8);
    }

    throw new Error('API definition file not found');
}

/**
 * Single source of truth for the "API Keys" sidebar item visibility on any
 * API-scoped page (landing, documentation, api-keys). It loads the stored API
 * definition and inspects its security schemes so every page derives the flag
 * the same way — previously each page computed it separately and some (e.g. the
 * documentation page) omitted the definition, hiding the nav item inconsistently.
 *
 * GraphQL/MCP/SOAP definitions are not OpenAPI security-scheme documents we can
 * inspect for apiKey security, so they resolve to false without a definition load.
 *
 * @param {string} orgId
 * @param {string} apiId
 * @param {string} apiType - constants.API_TYPE.*
 * @param {object} metaData - any truthy metadata object for the API
 * @param {string|object} [preloadedDefinition] - the API definition if the caller
 *        already loaded it (pass to avoid a second read); omit to load from storage.
 * @returns {Promise<boolean>}
 */
async function resolveShowApiKeysNav(orgId, apiId, apiType, metaData, preloadedDefinition) {
    if (!metaData) {
        return false;
    }
    if (apiType === constants.API_TYPE.GRAPHQL
        || apiType === constants.API_TYPE.MCP
        || apiType === constants.API_TYPE.SOAP) {
        return false;
    }
    let apiDefinition = preloadedDefinition;
    if (apiDefinition === undefined) {
        apiDefinition = null;
        try {
            apiDefinition = await getApiDefinitionFileContent(orgId, apiId);
        } catch (definitionErr) {
            logger.debug('Could not load API definition for API keys nav check', {
                orgId,
                apiId,
                error: definitionErr.message
            });
        }
    }
    return apiUsesApiKeySecurity(metaData, apiDefinition);
}

function parseApiDefinitionContent(apiDefinitionContent) {
    if (!apiDefinitionContent || typeof apiDefinitionContent !== 'string') {
        throw new Error('Invalid API definition content');
    }

    try {
        return JSON.parse(apiDefinitionContent);
    } catch (jsonError) {
        try {
            const parsedYaml = yaml.load(apiDefinitionContent);
            if (!parsedYaml || typeof parsedYaml !== 'object') {
                throw new Error('Parsed API definition is empty or invalid');
            }
            return parsedYaml;
        } catch (yamlError) {
            throw new Error(`Failed to parse API definition as JSON or YAML: ${yamlError.message}`);
        }
    }
}

async function parseSwagger(api) {
    try {
        // Extract general API info
        const title = api.info?.title || "No title";
        const apiDescription = api.info?.description || "No description available";
        //const servers = api.servers || [];

        // Extract endpoints — only recognised HTTP verbs (skip path-level keys like 'parameters', 'summary', 'description', 'servers')
        const HTTP_METHODS = new Set(['get', 'post', 'put', 'delete', 'patch']);
        const endpoints = Object.entries(api.paths || {}).map(([path, methods]) => ({
            path,
            methods: Object.keys(methods)
                .filter(method => HTTP_METHODS.has(method.toLowerCase()))
                .map(method => ({
                    method: method.toUpperCase(),
                    summary: methods[method]?.summary || "No summary",
                    description: methods[method]?.description || "No description",
                })),
        })).filter(entry => entry.methods.length > 0);
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
    prodEndpoint = prodEndpoint || '';
    sandboxEndpoint = sandboxEndpoint || '';

    if (apiDefinition?.swagger?.startsWith('2.')) {
        if (prodEndpoint.trim().length !== 0) {
            apiDefinition.host = prodEndpoint.replace(/https?:\/\//, '');
            apiDefinition.schemes = [prodEndpoint.startsWith('https') ? 'https' : 'http'];
        }
    }
    let servers = [];
    if (prodEndpoint.trim().length !== 0) {
        servers.push({
            description: "Production",
            url: prodEndpoint
        });
    }
    if (sandboxEndpoint.trim().length !== 0) {
        servers.push({
            description: "Sandbox",
            url: sandboxEndpoint
        });
    }
    apiDefinition.servers = servers;
    return apiDefinition;
}


function replaceEndpointParamsAsyncAPI(apiDefinition, prodEndpoint, sandboxEndpoint) {
    prodEndpoint = prodEndpoint || '';
    sandboxEndpoint = sandboxEndpoint || '';
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

async function convertSDLToIntrospection(sdl) {
    try {
        const schema = buildSchema(sdl);
        const introspectionQuery = getIntrospectionQuery();
        const result = await executeGraphQL({
            schema,
            source: introspectionQuery
        });
        
        return result.data;
    } catch (error) {
        logger.error('Error converting SDL to introspection', {
            error: error.message,
            stack: error.stack
        });
        return null;
    }
}


const loadAPIContentMd = async (req, res) => {
    const { orgName, apiHandle, viewName } = req.params;

    try {
        const orgDetails = await orgDao.get(orgName);
        const orgId = orgDetails.uuid;

        if (await isAiDisabledForPortal(orgId, viewName)) {
            return res.status(404).send('# Not Found\n\nThis resource is not available for agents.');
        }

        const apiId = await apiDao.getId(orgId, apiHandle);
        const metaData = await loadAPIMetaData(req, orgId, apiId);

        if (metaData?.agentVisibility === 'HIDDEN') {
            return res.status(404).send('# Not Found\n\nThis API is not available for agents.');
        }

        const subscriptionPlans = await util.appendSubscriptionPlanDetails(orgId, metaData.subscriptionPlans);

        const isMCPFromRegistry = metaData?.type === constants.API_TYPE.MCP && !metaData.refId;
        let showOAuth2 = true;
        let showApiKey = false;
        let noAuth = false;

        // Load API definition
        let apiDefinition = null;
        let specHeading = 'OpenAPI Specification';
        const apiType = metaData?.type;
        try {
            // Resolve the definition by its doc-type rather than a guessed filename: the stored
            // name is the uploaded basename, so a fixed name would miss anything uploaded under a
            // different name. There is one definition per API (SCHEMA_DEFINITION for MCP tools
            // schemas, API_DEFINITION for every other type).
            if (apiType === constants.API_TYPE.GRAPHQL) specHeading = 'GraphQL Schema';
            else if (apiType === constants.API_TYPE.MCP) specHeading = 'Tool Schema';
            else if (apiType === constants.API_TYPE.WS || apiType === constants.API_TYPE.WEBSUB) specHeading = 'AsyncAPI Specification';
            else if (apiType === 'SOAP') specHeading = 'WSDL';
            const definitionDocType = apiType === constants.API_TYPE.MCP
                ? constants.DOC_TYPES.SCHEMA_DEFINITION
                : constants.DOC_TYPES.API_DEFINITION;
            const raw = await apiFileDao.getDoc(definitionDocType, orgId, apiId, null);
            if (raw) apiDefinition = raw.file_content.toString(constants.CHARSET_UTF8);
        } catch (defErr) {
            logger.warn('Could not load API definition for markdown', { orgId, apiId, error: defErr.message });
        }

        // Load docs
        const baseUrl = '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName;
        const linkBase = apiType === constants.API_TYPE.MCP
            ? `${baseUrl}/mcp/${apiHandle}`
            : `${baseUrl}/api/${apiHandle}`;
        const docTypes = await apiMetadataService.getAPIDocTypes(orgId, apiId);
        const docs = [];
        for (const docType of docTypes) {
            if (docType.type === 'API_DEFINITION') continue;
            for (const name of (docType.names || [])) {
                docs.push({ name, type: docType.type, url: `${linkBase}/docs/${docType.type}/${name}` });
            }
        }

        let tokenEndpoint = null;

        // Enrich spec with live server URLs
        if (apiDefinition && apiType !== constants.API_TYPE.GRAPHQL && apiType !== constants.API_TYPE.MCP && apiType !== 'SOAP') {
            try {
                const parsed = parseApiDefinitionContent(apiDefinition);
                const isAsyncAPI = apiType === constants.API_TYPE.WS || apiType === constants.API_TYPE.WEBSUB;
                const enriched = isAsyncAPI
                    ? replaceEndpointParamsAsyncAPI(parsed, metaData.endPoints?.productionURL || '', metaData.endPoints?.sandboxURL || '')
                    : replaceEndpointParams(parsed, metaData.endPoints?.productionURL || '', metaData.endPoints?.sandboxURL || '');
                apiDefinition = JSON.stringify(enriched, null, 2);
            } catch (enrichErr) {
                logger.warn('Could not enrich API spec for markdown', { orgId, apiId, error: enrichErr.message });
            }
        }

        const specExt = apiType === constants.API_TYPE.GRAPHQL ? 'graphql'
            : apiType === 'SOAP' ? 'xml'
            : 'json';
        let schemaDefinition = null;
        if (apiType === constants.API_TYPE.MCP && apiDefinition) {
            try {
                const parsed = JSON.parse(apiDefinition);
                if (Array.isArray(parsed)) {
                    schemaDefinition = {
                        tools: parsed.filter(item => item.type === 'TOOL'),
                        resources: parsed.filter(item => item.type === 'RESOURCE'),
                        prompts: parsed.filter(item => item.type === 'PROMPT'),
                    };
                } else {
                    schemaDefinition = parsed;
                }
            } catch (parseErr) {
                logger.warn('Could not parse MCP schema definition for markdown', { orgId, apiId, error: parseErr.message });
            }
        }
        const templateContent = {
            apiMetadata: metaData,
            subscriptionPlans,
            apiDefinition,
            schemaDefinition,
            specHeading,
            specUrl: `${linkBase}/docs/specification.${specExt}`,
            docs: docs.length > 0 ? docs : null,
            baseUrl,
            tokenEndpoint,
            showOAuth2,
            showApiKey,
            noAuth,
            isMCPFromRegistry,
        };
        const templateDir = apiType === constants.API_TYPE.MCP ? 'pages/mcp-landing' : 'pages/api-landing';
        const md = await util.renderMarkdownTemplateFromAPI(templateContent, orgId, templateDir, viewName);

        res.setHeader('Content-Type', 'text/markdown; charset=utf-8');
        res.send(md);
    } catch (error) {
        logger.error('Error generating API detail markdown', {
            orgName,
            error: error.message,
            stack: error.stack
        });
        res.status(500).send('# Error\n\nFailed to load API details.');
    }
};

async function buildLlmsTxtTemplateContent(req, orgId, orgName, viewName, configOverrides = {}) {
    const metaDataList = await loadAPIMetaDataListFromAPI(req, orgId, orgName, null, null, viewName);
    const agentVisibleAPIs = metaDataList.filter(api => api.agentVisibility !== 'HIDDEN');
    const hiddenAPICount = metaDataList.length - agentVisibleAPIs.length;

    // api.type holds the stored constant (e.g. "RestApi", "Mcp", "WebSubApi" — see
    // constants.API_TYPE), not the enum key used below — map it back or every
    // REST/MCP/WebSub API silently drops out of the discovery index.
    const typeConstantToEnum = Object.fromEntries(
        Object.entries(constants.API_TYPE).map(([enumKey, storedValue]) => [storedValue, enumKey])
    );
    const byType = { REST: [], MCP: [], GRAPHQL: [], WS: [], WEBSUB: [] };
    for (const api of agentVisibleAPIs) {
        const type = typeConstantToEnum[api.type];
        if (byType[type]) byType[type].push(api);
    }

    const viewId = await viewDao.getId(orgId, viewName);
    const allApiWorkflows = await apiWorkflowService.getAllAPIWorkflowsFromDB(orgId, viewId);
    const allPublishedWorkflows = allApiWorkflows.filter(flow => flow.status === 'PUBLISHED');
    const publishedWorkflows = allPublishedWorkflows.filter(flow => flow.agentVisibility !== 'HIDDEN');
    const hiddenWorkflowCount = allPublishedWorkflows.length - publishedWorkflows.length;

    const baseUrl = '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName;
    return {
        orgName: configOverrides.orgName,
        portalName: configOverrides.portalName || null,
        portalDescription: configOverrides.portalDescription || null,
        restAPIs:    byType.REST.length    ? byType.REST    : null,
        mcpAPIs:     byType.MCP.length     ? byType.MCP     : null,
        graphqlAPIs: byType.GRAPHQL.length ? byType.GRAPHQL : null,
        wsAPIs:      byType.WS.length      ? byType.WS      : null,
        websubAPIs:  byType.WEBSUB.length  ? byType.WEBSUB  : null,
        workflows:   publishedWorkflows.length > 0 ? publishedWorkflows : null,
        hiddenAPICount: hiddenAPICount > 0 ? hiddenAPICount : 0,
        hiddenWorkflowCount: hiddenWorkflowCount > 0 ? hiddenWorkflowCount : 0,
        hasHiddenResources: hiddenAPICount > 0 || hiddenWorkflowCount > 0,
        portalUrl: baseUrl,
        baseUrl,
    };
}

const loadLlmsTxt = async (req, res) => {
    const { orgName, viewName } = req.params;
    try {
        const orgDetails = await orgDao.get(orgName);
        const orgId = orgDetails.uuid;

        const configAsset = await orgDao.getContent({
            orgId: orgId, fileType: constants.FILE_TYPE.LLMS_CONFIG, viewName, fileName: constants.FILE_NAME.LLMS_CONFIG
        });
        let llmsConfig = {};
        if (configAsset) {
            try { llmsConfig = JSON.parse(configAsset.file_content.toString('utf8')); } catch (e) { /* ignore */ }
        }
        if (llmsConfig.aiEnabled === false) {
            return res.status(404).send('Not Found');
        }

        const templateContent = await buildLlmsTxtTemplateContent(req, orgId, orgName, viewName, {
            orgName: orgDetails.display_name,
            portalName: llmsConfig.portalName || null,
            portalDescription: llmsConfig.portalDescription || null,
        });

        const md = await util.renderLlmsTxt(templateContent, orgId, viewName);
        res.setHeader('Content-Type', 'text/plain; charset=utf-8');
        res.setHeader('Cache-Control', 'no-cache, no-store, must-revalidate');
        res.setHeader('Pragma', 'no-cache');
        res.setHeader('Expires', '0');
        res.send(md);
    } catch (error) {
        logger.error('Error generating llms.txt', { orgName, error: error.message, stack: error.stack });
        res.status(500).send('# Error\n\nFailed to generate portal index.');
    }
};

const previewLlmsTxt = async (req, res) => {
    const { orgName, viewName } = req.params;
    try {
        const orgDetails = await orgDao.get(orgName);
        const orgId = orgDetails.uuid;
        const { portalName, portalDescription } = req.body;

        const templateContent = await buildLlmsTxtTemplateContent(req, orgId, orgName, viewName, {
            orgName: orgDetails.display_name,
            portalName: portalName || null,
            portalDescription: portalDescription || null,
        });

        const md = await util.renderLlmsTxt(templateContent, orgId, viewName);
        res.setHeader('Content-Type', 'text/plain; charset=utf-8');
        res.setHeader('Cache-Control', 'no-cache, no-store, must-revalidate');
        res.setHeader('Pragma', 'no-cache');
        res.setHeader('Expires', '0');
        res.send(md);
    } catch (error) {
        logger.error('Error previewing llms.txt', { orgName, error: error.message, stack: error.stack });
        res.status(500).send('# Error\n\nFailed to generate preview.');
    }
};

const loadAPIsMd = async (req, res) => {
    const { orgName, viewName } = req.params;

    try {
        const orgDetails = await orgDao.get(orgName);
        const orgId = orgDetails.uuid;

        if (await isAiDisabledForPortal(orgId, viewName)) {
            return res.status(404).send('# Not Found\n\nThis resource is not available for agents.');
        }

        const metaDataList = await loadAPIMetaDataListFromAPI(req, orgId, orgName, null, null, viewName);
        const agentVisibleAPIs = metaDataList.filter(api => api.agentVisibility !== 'HIDDEN');
        const hiddenAPICount = metaDataList.length - agentVisibleAPIs.length;

        const nonMcpAPIs = agentVisibleAPIs.filter(api => api.type !== constants.API_TYPE.MCP);
        const byType = { REST: [], GRAPHQL: [], WS: [], WEBSUB: [] };
        for (const api of nonMcpAPIs) {
            const type = api.type;
            if (byType[type]) byType[type].push(api);
        }
        const baseUrl = '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName;
        const templateContent = {
            restAPIs:    byType.REST.length    ? byType.REST    : null,
            graphqlAPIs: byType.GRAPHQL.length ? byType.GRAPHQL : null,
            wsAPIs:      byType.WS.length      ? byType.WS      : null,
            websubAPIs:  byType.WEBSUB.length  ? byType.WEBSUB  : null,
            baseUrl,
            hiddenAPICount: hiddenAPICount > 0 ? hiddenAPICount : 0,
            hasHiddenAPIs: hiddenAPICount > 0,
            portalUrl: baseUrl,
        };
        const md = await util.renderMarkdownTemplateFromAPI(templateContent, orgId, 'pages/apis', viewName);

        res.setHeader('Content-Type', 'text/markdown; charset=utf-8');
        res.setHeader('Cache-Control', 'no-cache, no-store, must-revalidate');
        res.setHeader('Pragma', 'no-cache');
        res.setHeader('Expires', '0');
        res.send(md);
    } catch (error) {
        logger.error('Error generating APIs markdown', {
            orgName,
            error: error.message,
            stack: error.stack
        });
        res.status(500).send('# Error\n\nFailed to load API list.');
    }
};

const loadMCPsMd = async (req, res) => {
    const { orgName, viewName } = req.params;

    try {
        const orgDetails = await orgDao.get(orgName);
        const orgId = orgDetails.uuid;

        if (await isAiDisabledForPortal(orgId, viewName)) {
            return res.status(404).send('# Not Found\n\nThis resource is not available for agents.');
        }

        const metaDataList = await loadAPIMetaDataListFromAPI(req, orgId, orgName, null, null, viewName);
        const agentVisibleAPIs = metaDataList.filter(api => api.agentVisibility !== 'HIDDEN');
        const mcpAPIs = agentVisibleAPIs.filter(api => api.type === constants.API_TYPE.MCP);
        const hiddenAPICount = metaDataList.filter(api => api.type === constants.API_TYPE.MCP).length - mcpAPIs.length;

        const baseUrl = '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName;
        const templateContent = {
            mcpAPIs: mcpAPIs.length ? mcpAPIs : null,
            baseUrl,
            hiddenAPICount: hiddenAPICount > 0 ? hiddenAPICount : 0,
            hasHiddenAPIs: hiddenAPICount > 0,
            portalUrl: baseUrl,
        };
        const md = await util.renderMarkdownTemplateFromAPI(templateContent, orgId, 'pages/mcps', viewName);

        res.setHeader('Content-Type', 'text/markdown; charset=utf-8');
        res.setHeader('Cache-Control', 'no-cache, no-store, must-revalidate');
        res.setHeader('Pragma', 'no-cache');
        res.setHeader('Expires', '0');
        res.send(md);
    } catch (error) {
        logger.error('Error generating MCPs markdown', {
            orgName,
            error: error.message,
            stack: error.stack
        });
        res.status(500).send('# Error\n\nFailed to load MCP list.');
    }
};

const SPEC_FORMAT_MAP = {
    [constants.API_TYPE.GRAPHQL]: { format: 'graphql', field: 'graphql',  label: 'GraphQL' },
    [constants.API_TYPE.SOAP]:    { format: 'xml',     field: 'swagger',  label: 'SOAP'    },
    [constants.API_TYPE.MCP]:     { format: 'json',    field: 'schema',   label: 'MCP'     },
    [constants.API_TYPE.WS]:      { format: 'json',    field: 'asyncapi', label: 'WS'      },
    [constants.API_TYPE.WEBSUB]:  { format: 'json',    field: 'asyncapi', label: 'WEBSUB'  },
};
const SPEC_FORMAT_DEFAULT = { format: 'json', field: 'swagger', label: 'REST' };

const loadAPIDefinitionRaw = async (req, res) => {
    const { orgName, apiHandle, viewName, format } = req.params;
    try {
        const orgDetails = await orgDao.get(orgName);
        const orgId = orgDetails.uuid;

        if (await isAiDisabledForPortal(orgId, viewName)) {
            return util.sendError(res, 404, 'Not Found');
        }

        const definitionResponse = await getAPIDefinition(orgName, viewName, apiHandle);

        if (definitionResponse.metaData?.agentVisibility === 'HIDDEN') {
            return util.sendError(res, 404, 'API specification not found');
        }

        const typeConfig = SPEC_FORMAT_MAP[definitionResponse.apiType] || SPEC_FORMAT_DEFAULT;

        if (format !== typeConfig.format) {
            return res.status(404).send(`${typeConfig.label} APIs only support specification.${typeConfig.format}.`);
        }

        const raw = definitionResponse[typeConfig.field];
        if (!raw) return util.sendError(res, 404, 'API specification not found');

        const apiType = definitionResponse.apiType;

        if (apiType === constants.API_TYPE.GRAPHQL) {
            const sdl = typeof raw === 'string' ? raw : JSON.stringify(raw);
            res.setHeader('Content-Type', 'application/graphql; charset=utf-8');
            return res.status(200).send(sdl);
        }

        if (apiType === constants.API_TYPE.SOAP) {
            res.setHeader('Content-Type', 'application/xml; charset=utf-8');
            return res.status(200).send(typeof raw === 'string' ? raw : String(raw));
        }

        let spec = typeof raw === 'string' ? parseApiDefinitionContent(raw) : raw;

        const endpoints = definitionResponse.metaData?.endPoints;
        const prodUrl = endpoints?.productionURL || '';
        const sandboxUrl = endpoints?.sandboxURL || '';
        if (apiType === constants.API_TYPE.WS || apiType === constants.API_TYPE.WEBSUB) {
            spec = replaceEndpointParamsAsyncAPI(spec, prodUrl, sandboxUrl);
        } else if (apiType !== constants.API_TYPE.MCP) {
            spec = replaceEndpointParams(spec, prodUrl, sandboxUrl);
        }

        res.status(200).json(spec);
    } catch (error) {
        logger.error('Error loading raw specification', {
            orgName,
            viewName,
            format,
            error: error.message,
            stack: error.stack
        });
        util.sendError(res, 500, 'Failed to load specification.');
    }
};

const loadDocumentMd = async (req, res) => {
    const { orgName, apiHandle, viewName, docType, docName } = req.params;

    try {
        const orgDetails = await orgDao.get(orgName);
        const orgId = orgDetails.uuid;

        if (await isAiDisabledForPortal(orgId, viewName)) {
            return res.status(404).send('# Not Found\n\nThis resource is not available for agents.');
        }

        const apiId = await apiDao.getId(orgId, apiHandle);
        const docMetaData = await loadAPIMetaData(req, orgId, apiId);
        if (docMetaData?.agentVisibility === 'HIDDEN') {
            return res.status(404).send('# Not Found\n\nThis API is not available for agents.');
        }
        // docName here is without the .md suffix (stripped by the route param)
        const fullDocName = docName + '.md';
        const docContentResponse = await apiFileDao.getDocByName(
            constants.DOC_TYPES.DOC_ID + docType,
            fullDocName,
            orgId,
            apiId
        );
        if (!docContentResponse) {
            return res.status(404).send('# Not Found\n\nDocument not found.');
        }
        const content = docContentResponse.file_content.toString(constants.CHARSET_UTF8);
        res.setHeader('Content-Type', 'text/markdown; charset=utf-8');
        res.send(content);
    } catch (error) {
        logger.error('Error loading raw document markdown', {
            orgName,
            error: error.message,
            stack: error.stack
        });
        res.status(500).send('# Error\n\nFailed to load document.');
    }
};

module.exports = {
    loadAPIs,
    loadAPIContent,
    loadDocsPage,
    loadDocument,
    loadAPIsMd,
    loadMCPsMd,
    loadLlmsTxt,
    previewLlmsTxt,
    loadAPIContentMd,
    loadDocumentMd,
    loadSpecificationRaw: loadAPIDefinitionRaw,
    resolveShowApiKeysNav,
};
