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
const path = require('path');
const fs = require('fs');
const marked = require('marked');
const Handlebars = require('handlebars');
const logger = require('../config/logger');
const { CustomError } = require('../utils/errors/customErrors');
const adminDao = require('../dao/admin');
const constants = require('../utils/constants');
const unzipper = require('unzipper');
const axios = require('axios');
const qs = require('qs');
const https = require('https');
const { config } = require('../config/configLoader');
const { body, param, query } = require('express-validator');
const { Sequelize } = require('sequelize');
const apiDao = require('../dao/apiMetadata');
const subscriptionPolicyDTO = require('../dto/subscriptionPolicy');
const jwt = require('jsonwebtoken');
const filePrefix = '/src/defaultContent/';

// Function to load and convert markdown file to HTML
async function loadMarkdown(filename, dirName) {

    const filePath = path.join(process.cwd(), dirName, filename);
    if (fs.existsSync(filePath)) {
        const fileContent = fs.readFileSync(filePath, constants.CHARSET_UTF8);
        return marked.parse(fileContent);
    } else {
        return null;
    }
};


function renderTemplate(templatePath, layoutPath, templateContent, isTechnical) {

    let completeTemplatePath;
    if (isTechnical) {
        completeTemplatePath = path.join(require.main.filename, templatePath);
    } else {
        completeTemplatePath = path.join(process.cwd(), templatePath);
    }

    const templateResponse = fs.readFileSync(completeTemplatePath, constants.CHARSET_UTF8);
    const completeLayoutPath = path.join(process.cwd(), layoutPath);
    const layoutResponse = fs.readFileSync(completeLayoutPath, constants.CHARSET_UTF8)

    const template = Handlebars.compile(templateResponse.toString());
    const layout = Handlebars.compile(layoutResponse.toString());

    const showApiWorkflowsNav = config.features?.apiWorkflows?.enabled === true;
    const enrichedContent = { ...templateContent, showApiWorkflowsNav };
    return layout({
        body: template(enrichedContent),
        portalConfigs: config.portalConfigs,
        profile: templateContent.profile,
        showApiWorkflowsNav,
    });
}

async function loadLayoutFromAPI(orgID, viewName) {

    var layoutContent = await adminDao.getOrgContent({
        orgId: orgID,
        fileType: constants.FILE_TYPE.LAYOUT,
        fileName: constants.FILE_NAME.MAIN,
        viewName: viewName
    });
    if (layoutContent) {
        return layoutContent.FILE_CONTENT.toString(constants.CHARSET_UTF8);
    } else {
        return "";
    }
}

async function loadTemplateFromAPI(orgID, filePath, viewName) {

    var templateContent = await adminDao.getOrgContent({
        orgId: orgID,
        filePath: filePath,
        fileType: constants.FILE_TYPE.TEMPLATE,
        fileName: constants.FILE_NAME.PAGE,
        viewName: viewName
    });
    return templateContent ? templateContent.FILE_CONTENT.toString(constants.CHARSET_UTF8) : "";
}

async function renderTemplateFromAPI(templateContent, orgID, orgName, filePath, viewName) {

    const templateResponse = fs.readFileSync(path.join(process.cwd(), filePrefix + filePath + '/page.hbs'), constants.CHARSET_UTF8);
    const completeLayoutPath = path.join(process.cwd(), filePrefix + 'layout/main.hbs');

    layoutResponse = fs.readFileSync(completeLayoutPath, constants.CHARSET_UTF8);
    const styleContent = await adminDao.getOrgContent({ orgId: orgID, fileType: 'style', viewName: viewName, fileName: 'main.css' });
    if (styleContent) {
        layoutResponse = layoutResponse.replace(/\/styles\//g, `${constants.ROUTE.DEVPORTAL_ASSETS_BASE_PATH}${orgID}/views/${viewName}/layout?fileType=style&fileName=`);
    }

    const template = Handlebars.compile(templateResponse.toString());
    const layout = Handlebars.compile(layoutResponse.toString());

    const showApiWorkflowsNav = config.features?.apiWorkflows?.enabled === true;
    const enrichedContent = { ...templateContent, showApiWorkflowsNav };
    return layout({
        body: template(enrichedContent),
        portalConfigs: config.portalConfigs,
        profile: templateContent.profile,
        showApiWorkflowsNav,
    });

}

async function renderLlmsTxt(templateContent, orgID, viewName) {

    const dbPartial = await adminDao.getOrgContent({
        orgId: orgID,
        fileType: 'partial',
        viewName: viewName,
        fileName: 'llms-txt.hbs'
    });
    const partialSource = dbPartial
        ? dbPartial.FILE_CONTENT.toString(constants.CHARSET_UTF8)
        : fs.readFileSync(
            path.join(process.cwd(), filePrefix + 'pages/llms-txt/partials/llms-txt.hbs'),
            constants.CHARSET_UTF8
        );
    Handlebars.registerPartial('llms-txt', partialSource);

    const pageSource = fs.readFileSync(
        path.join(process.cwd(), filePrefix + 'pages/llms-txt/page.hbs'),
        constants.CHARSET_UTF8
    );
    return Handlebars.compile(pageSource)(templateContent);
}

async function renderMarkdownTemplateFromAPI(templateContent, orgID, filePath, viewName) {

    const partialName = path.basename(filePath) + '-md';
    const dbPartial = await adminDao.getOrgContent({
        orgId: orgID,
        fileType: 'partial',
        viewName: viewName,
        fileName: partialName + '.hbs'
    });
    const partialSource = dbPartial
        ? dbPartial.FILE_CONTENT.toString(constants.CHARSET_UTF8)
        : fs.readFileSync(
            path.join(process.cwd(), filePrefix + filePath + '/partials/' + partialName + '.hbs'),
            constants.CHARSET_UTF8
        );
    Handlebars.registerPartial(partialName, partialSource);

    const pageSource = fs.readFileSync(
        path.join(process.cwd(), filePrefix + filePath + '/page-md.hbs'),
        constants.CHARSET_UTF8
    );
    return Handlebars.compile(pageSource)(templateContent);
}

async function renderGivenTemplate(templatePage, layoutPage, templateContent) {

    const template = Handlebars.compile(templatePage.toString());
    const layout = Handlebars.compile(layoutPage.toString());
    const showApiWorkflowsNav = config.features?.apiWorkflows?.enabled === true;
    const enrichedContent = { ...templateContent, showApiWorkflowsNav };
    return layout({
        body: template(enrichedContent),
        portalConfigs: config.portalConfigs,
        profile: templateContent.profile,
        showApiWorkflowsNav,
    });
}

function getErrors(errors) {

    const errorList = [];
    errors.errors.forEach(element => {
        errorList.push({
            code: '400',
            message: 'input validation failed',
            description: element.msg
        })
    });
    return errorList;
}

function handleError(res, error) {
    if (error instanceof Sequelize.UniqueConstraintError) {
        return res.status(409).json({
            code: "409",
            message: "Conflict",
            description: error.errors ? error.errors[0].message : error.message.replaceAll('"', ''),
        });
    } else if (error instanceof Sequelize.ValidationError) {
        return res.status(400).json({
            code: "400",
            message: "Bad Request",
            description: error.message
        });
    } else if (error instanceof Sequelize.EmptyResultError) {
        return res.status(404).json({
            code: "404",
            message: "Resource Not Found",
            description: error.message
        });
    } else if (error instanceof CustomError) {
        return res.status(error.statusCode).json({
            code: error.statusCode,
            message: error.message,
            description: error.description
        });
    } else {
        let errorDescription = error.message;
        if (error instanceof Sequelize.DatabaseError) {
            errorDescription = "Internal Server Error";
        }
        return res.status(500).json({
            "code": "500",
            "message": "Internal Server Error",
            "description": errorDescription
        });
    }
};

const unzipDirectory = async (zipPath, extractPath) => {
    if (typeof zipPath !== 'string' || typeof extractPath !== 'string' || !zipPath || !extractPath) {
        throw new CustomError(400, 'Error unzipping directory', 'Invalid zip path or extract path.');
    }
    const extractedFiles = [];
    const maxFileSize = 10 * 1024 * 1024; // 10MB (limit for individual file size)
    const maxTotalSize = 50 * 1024 * 1024; // 50MB (limit for total extracted data)
    const maxDepth = 10; // Limit to prevent excessive nesting
    let totalExtractedSize = 0; // Total extracted data size

    await new Promise((resolve, reject) => {
        const streams = [];
        fs.createReadStream(zipPath)
            .pipe(unzipper.Parse())
            .on('entry', entry => {
                try {
                    const entryPath = entry.path;
                    const entrySize = entry.size;
                    const entryDepth = entryPath.split(path.sep).length;

                    if (!entryPath.includes('__MACOSX')) {
                        const filePath = path.resolve(extractPath, entryPath);
                        // Prevent path traversal
                        const normalizedFilePath = path.normalize(filePath);
                        if (!normalizedFilePath.startsWith(path.resolve(extractPath))) {
                            entry.autodrain();
                            return reject(new CustomError(400, 'Error unzipping directory'
                                , 'File access outside working directory detected.'));
                        }

                        // Validate depth (to avoid zip bombs with excessive nesting)
                        // and reject files that are too large
                        // and check if adding this file would exceed the total size limit
                        if ((entryDepth > maxDepth) || (entrySize > maxFileSize)
                            || (totalExtractedSize + entrySize > maxTotalSize)) {
                            entry.autodrain();
                            return reject(new CustomError(400, 'Error unzipping directory'
                                , 'File size exceeded the limit of 50 MB'));
                        }

                        const dirName = path.dirname(normalizedFilePath);
                        fs.mkdirSync(dirName, { recursive: true });
                        if (entry.type === 'Directory') {
                            entry.autodrain();
                        } else {
                            extractedFiles.push(normalizedFilePath);
                            const stream = new Promise((resolve, reject) => {
                                entry.pipe(fs.createWriteStream(normalizedFilePath))
                                    .on('finish', resolve)
                                    .on('error', reject);
                            });
                            streams.push(stream);
                            // Update the total extracted size
                            totalExtractedSize += entrySize;
                        }
                    } else {
                        entry.autodrain();
                    }
                } catch (err) {
                    logger.error("Error processing entry", { error: err.message, stack: err.stack });
                    entry.autodrain();
                    reject(new Error('Error processing entry.'));
                }
            })
            .on('close', async () => {
                try {
                    await Promise.all(streams); // Wait for all files to finish writing
                    extractedFiles.length > 0 ? resolve() : reject(new Error('No files were extracted'));
                } catch (err) {
                    reject(new Error(`Unzip failed: ${err.message}`));
                }
            })
            .on('error', err => {
                reject(new Error(`Unzip failed: ${err.message}`));
            });
    }).catch((err) => {
        throw err;
    });
}

const imageMapping = {
    [constants.FILE_EXTENSIONS.SVG]: constants.MIME_TYPES.SVG,
    [constants.FILE_EXTENSIONS.JPG]: constants.MIME_TYPES.JPEG,
    [constants.FILE_EXTENSIONS.JPEG]: constants.MIME_TYPES.JPEG,
    [constants.FILE_EXTENSIONS.PNG]: constants.MIME_TYPES.PNG,
    [constants.FILE_EXTENSIONS.GIF]: constants.MIME_TYPES.GIF,
};
const fileMapping = {
    [constants.FILE_EXTENSIONS.JSON]: constants.MIME_TYPES.JSON,
    [constants.FILE_EXTENSIONS.YAML]: constants.MIME_TYPES.YAML,
    [constants.FILE_EXTENSIONS.YML]: constants.MIME_TYPES.YAML,
    [constants.FILE_EXTENSIONS.XML]: constants.MIME_TYPES.XML
}

const textFiles = [
    constants.FILE_EXTENSIONS.HTML, constants.FILE_EXTENSIONS.HBS, constants.FILE_EXTENSIONS.MD,
    constants.FILE_EXTENSIONS.JSON, constants.FILE_EXTENSIONS.YAML, constants.FILE_EXTENSIONS.YML
]

const imageFiles = [
    constants.FILE_EXTENSIONS.SVG, constants.FILE_EXTENSIONS.JPG,
    constants.FILE_EXTENSIONS.JPEG, constants.FILE_EXTENSIONS.PNG,
    constants.FILE_EXTENSIONS.GIF
]

const isTextFile = (fileExtension) => {
    return textFiles.includes(fileExtension)
}

const isImageFile = (fileExtension) => {
    return imageFiles.includes(fileExtension)
}

const retrieveContentType = (fileName, fileType) => {

    if (fileType === constants.STYLE)
        return constants.MIME_TYPES.CSS;

    const extension = path.extname(fileName).toLowerCase();

    if (fileType === constants.IMAGE) {
        return imageMapping[extension] || constants.MIME_TYPES.CONYEMT_TYPE_OCT;
    }
    if (fileType === constants.TEXT) {
        return fileMapping[extension] || constants.MIME_TYPES.TEXT;
    }
    return constants.MIME_TYPES.TEXT;
};

const getAPIFileContent = (directory) => {
    let files = [];
    const filenames = fs.readdirSync(directory);
    filenames.forEach((filename) => {
        if (!(filename === '.DS_Store')) {
            let fileContent = fs.readFileSync(path.join(directory, filename), 'utf8');
            files.push({ fileName: filename, content: fileContent, type: constants.DOC_TYPES.API_LANDING });
        }
    });
    return files;
};

const getAPIImages = async (directory) => {
    let files = [];
    const filenames = await fs.promises.readdir(directory, { withFileTypes: true });
    for (const filename of filenames) {
        if (!(filename === '.DS_Store')) {
            let fileContent = await fs.promises.readFile(path.join(directory, filename.name));
            files.push({ fileName: filename.name, content: fileContent, type: constants.DOC_TYPES.IMAGES });
        }
    }
    return files;
};

const getAPIDocLinks = (documentMetadata) => {

    let files = [];
    documentMetadata.forEach((doc) => {
        doc.links.forEach((link) => {
            files.push({ fileName: constants.DOC_TYPES.DOCLINK_ID + link.displayName, content: link.url, type: doc.type });
        });
    });
    return files;
};

async function readDocFiles(directory, baseDir = '', topLevelOnly = false) {

    const files = await fs.promises.readdir(directory, { withFileTypes: true });
    let fileDetails = [];
    for (const file of files) {
        const filePath = path.join(directory, file.name);
        const relativePath = path.join(baseDir, file.name);
        if (file.isDirectory()) {
            const subDirContents = await readDocFiles(filePath, relativePath, topLevelOnly);
            fileDetails = fileDetails.concat(subDirContents);
        } else {
            if (!(file.name === '.DS_Store')) {
                let content = await fs.promises.readFile(filePath);
                let docType;
                if (topLevelOnly) {
                    docType = baseDir
                        ? baseDir.split(path.sep)[0]
                        : constants.DOC_TYPES.DOCS.OTHER;
                } else {
                    docType = baseDir;
                }
                fileDetails.push({
                    type: constants.DOC_TYPES.DOC_ID + docType,
                    fileName: file.name,
                    content: content,
                });
            }
        }
    }
    return fileDetails;
}


const invokeGraphQLRequest = async (req, url, query, variables, headers) => {
    logger.info(`Invoking GraphQL API: ${url}`, {
        userId: req.user?.id || req.user?.username || 'anonymous',
        method: 'GraphQL',
        endpoint: url
    });

    headers = {
        ...headers,
        'Content-Type': 'application/json',
        Authorization: req.user?.exchangeToken
            ? `Bearer ${req.user.exchangeToken}`
            : req.user
                ? `Bearer ${req.user.accessToken}`
                : req.headers.authorization
    };

    let httpsAgent;

    if (config.controlPlane.disableCertValidation) {
        httpsAgent = new https.Agent({
            rejectUnauthorized: false,
        });
    } else {
        const certPath = path.join(process.cwd(), config.controlPlane.pathToCertificate);
        httpsAgent = new https.Agent({
            ca: fs.readFileSync(certPath),
            rejectUnauthorized: false,
        });
    }

    let graphqlPayload = {
        query,
        variables
    };

    try {
        if (config.advanced.tokenExchanger?.enabled) {
            const decodedToken = jwt.decode(req.user.exchangeToken);
            const orgId = decodedToken.organization.uuid;
            url = url.includes("?") ? `${url}&organizationId=${orgId}` : `${url}?organizationId=${orgId}`;
        }

        const response = await axios.post(url, graphqlPayload, {
            headers,
            httpsAgent
        });

        return response.data;
    } catch (error) {
        if (error.response?.status === 401 && req.user?.exchangeToken) {
            try {
                const newExchangedToken = await tokenExchanger(req.user.accessToken, req.user.returnTo.split("/")[1]);
                req.user.exchangeToken = newExchangedToken;
                headers.Authorization = `Bearer ${newExchangedToken}`;

                const retryResponse = await axios.post(url, graphqlPayload, {
                    headers,
                    httpsAgent
                });

                return retryResponse.data;
            } catch (retryError) {
                let retryMessage = retryError.response?.data?.description || retryError.message;
                logger.error('GraphQL request retry failed', {
                    url: url,
                    error: retryMessage,
                    statusCode: retryError.response?.status,
                    userId: req.user?.id || req.user?.username || 'anonymous'
                });
                throw new CustomError(retryError.response?.status || 500, "Request retry failed", retryMessage);
            }
        } else {
            logger.error('GraphQL request error', {
                url: url,
                error: error.message,
                statusCode: error.response?.status,
                responseData: error.response?.data,
                userId: req.user?.id || req.user?.username || 'anonymous'
            });
            let message = error.response?.data?.description || error.message;
            throw new CustomError(error.response?.status || 500, 'GraphQL request failed', message);
        }
    }
};

const apiRequest = async (method, url, headers, body, organizationId) => {
    let httpsAgent;
    url = url.includes("?") ? `${url}&organizationId=${organizationId}` : `${url}?organizationId=${organizationId}`;

    if (config.controlPlane.disableCertValidation) {
        httpsAgent = new https.Agent({
            rejectUnauthorized: false,
        });
    } else {
        const certPath = path.join(process.cwd(), config.controlPlane.pathToCertificate);
        httpsAgent = new https.Agent({
            ca: fs.readFileSync(certPath),
            rejectUnauthorized: false,
        });
    }
    const response = await axios({
        method,
        url,
        headers,
        data: body,
        httpsAgent
    });
    return response;
};

const invokeApiRequest = async (req, method, url, headers, body, publicMode = false) => {

    logger.info(`Invoking API: ${url}`, {
        method: method,
        userId: req.user?.id || req.user?.username || 'anonymous',
        endpoint: url
    });
    if (!publicMode) {
        headers = headers || {};
        headers.Authorization = req.user?.exchangeToken ? `Bearer ${req.user.exchangeToken}` : req.user ? `Bearer ${req.user.accessToken}` : req.headers.authorization;
    }
    let orgId = "";
    try {
        if (config.advanced.tokenExchanger?.enabled) {
            if (req.cpOrgID) {
                orgId = req.cpOrgID;
            } else {
                const decodedToken = jwt.decode(req.user.exchangeToken);
                orgId = decodedToken?.organization.uuid;
            }
        } else if (req.cpOrgID) {
            orgId = req.cpOrgID;
        }
        const response = await apiRequest(method, url, headers, body, orgId);
        return response.data;
    } catch (error) {
        logger.error('Error while invoking API', {
            url: url,
            method: method,
            error: error.message,
            statusCode: error.response?.status,
            responseData: error.response?.data,
            userId: req.user?.id || req.user?.username || 'anonymous'
        });
        if (error.response?.status === 401) {
            try {
                const newExchangedToken = await tokenExchanger(req.user.accessToken, req.originalUrl.split("/")[1]);
                req.user.exchangeToken = newExchangedToken;
                headers.Authorization = `Bearer ${newExchangedToken}`;
                const response = await apiRequest(method, url, headers, body, orgId);
                return response.data;
            } catch (retryError) {
                let retryMessage;
                if (retryError.response) {
                    retryMessage = retryError.response.data.description;
                }
                logger.error('API request retry failed', {
                    url: url,
                    method: method,
                    error: retryMessage,
                    userId: req.user?.id || req.user?.username || 'anonymous'
                });
                throw new CustomError(error.response.status, "Access denied", error.message || error.response?.data?.description || constants.ERROR_MESSAGE.UNAUTHENTICATED);
            }
        } else {
            let message = error.message;
            if (error.response) {
                message = error.response.data.description;
            }
            logger.error('API request failed', {
                url: url,
                method: method,
                error: message,
                statusCode: error.status,
                userId: req.user?.id || req.user?.username || 'anonymous'
            });
            throw new CustomError(error.status, 'Request failed', message);
        }
    }
};


const validateIDP = () => {

    const validations = [

        body('authorizationURL')
            .notEmpty()
            .isURL({
                protocols: ['http', 'https'], // Allow both http and https
                require_tld: false
            }).withMessage('authorizationURL must be a valid URL'),
        body('tokenURL')
            .notEmpty()
            .isURL({
                protocols: ['http', 'https'], // Allow both http and https
                require_tld: false
            }).withMessage('tokenURL must be a valid URL'),
        body('clientId')
            .notEmpty()
            .escape(),
        body('userInfoURL')
            .optional()
            .isURL({
                protocols: ['http', 'https'], // Allow both http and https
                require_tld: false
            }).withMessage('userInfoURL must be a valid URL'),
        body('callbackURL')
            .notEmpty()
            .isURL({
                protocols: ['http', 'https'], // Allow both http and https
                require_tld: false
            }).withMessage('callbackURL must be a valid URL'),
        body('logoutURL')
            .notEmpty()
            .isURL({
                protocols: ['http', 'https'], // Allow both http and https
                require_tld: false
            }).withMessage('logoutURL must be a valid URL'),
        body('logoutRedirectURI')
            .notEmpty()
            .isURL({
                protocols: ['http', 'https'], // Allow both http and https
                require_tld: false
            }).withMessage('logoutRedirectURI must be a valid URL'),
        body('signUpURL')
            .optional()
            .isURL({
                protocols: ['http', 'https'], // Allow both http and https
                require_tld: false
            }).withMessage('signUpURL must be a valid URL'),
        body('name')
            .notEmpty()
            .escape(),
        body('*')
            .if(body('*').isString())
            .trim()
    ];
    return validations;
}

const validateOrganization = () => {

    const validations = [
        body('businessOwnerEmail')
            .optional({ checkFalsy: true })
            .isEmail(),
        body().customSanitizer((value) => {
            for (const key in value) {
                if (['orgHandle', 'orgConfiguration'].includes(key)) {
                    continue;
                } else if (typeof value[key] === 'string') {
                    value[key] = value[key].replace(/[<>"'&]/g, '').trim();
                }
            }
            return value;
        })
    ]
    return validations;
}

const validateProvider = () => {

    const validations = [
        body('name')
            .notEmpty()
            .escape()
            .trim(),
        body('providerURL')
            .notEmpty()
            .isURL({
                protocols: ['http', 'https'], // Allow both http and https
                require_tld: false
            }).withMessage('providerUrl must be a valid URL')
    ]
    return validations;
}

const validateRequestParameters = () => {

    const validations = [
        param('*')
            .trim()
            .escape(),
        query('*')
            .trim()
            .escape(),
    ]
    return validations;
}

const rejectExtraProperties = (allowedKeys, payload) => {

    const extraKeys = Object.keys(payload).filter(
        (key) => !allowedKeys.includes(key)
    );
    return extraKeys;
};

async function readFilesInDirectory(directory, orgId, protocol, host, viewName, baseDir = '') {
    try {
        const files = await fs.promises.readdir(directory, { withFileTypes: true });
        let fileDetails = [];
        for (const file of files) {
            const filePath = path.join(directory, file.name);
            const relativePath = path.join(baseDir, file.name);

            // Normalize and resolve filePath to ensure it stays within the intended directory
            const resolvedFilePath = path.resolve(filePath);
            const resolvedBaseDir = path.resolve(directory);

            // Ensure the file path is within the target directory
            if (!resolvedFilePath.startsWith(resolvedBaseDir + path.sep)) {
                throw new Error(`Invalid file path: ${filePath}`);
            }

            if (file.isDirectory()) {
                const subDirContents = await readFilesInDirectory(filePath, orgId, protocol, host, viewName, relativePath);
                fileDetails = fileDetails.concat(subDirContents);
            } else {
                let content = await fs.promises.readFile(filePath);
                let strContent = await fs.promises.readFile(filePath, constants.CHARSET_UTF8);
                let dir = baseDir.replace(/^[^/]+\/?/, '') || '/';
                const imageExtensions = ['.jpg', '.jpeg', '.png', '.gif', '.svg'];
                const fileExtension = path.extname(file.name).toLowerCase();
                let fileType;
                if (file.name.endsWith(".css")) {
                    fileType = "style"
                    if (file.name === "main.css") {
                        strContent = strContent.replace(/@import\s*['"]\/styles\/api-content\.css['"];/g, `@import url("${constants.ROUTE.DEVPORTAL_ASSETS_BASE_PATH}${orgId}/views/${viewName}/layout?fileType=style&fileName=api-content.css");`);
                        strContent = strContent.replace(/@import\s*['"]\/styles\/home\.css['"];/g, `@import url("${constants.ROUTE.DEVPORTAL_ASSETS_BASE_PATH}${orgId}/views/${viewName}/layout?fileType=style&fileName=home.css");`);
                        strContent = strContent.replace(/@import\s*['"]\/styles\/main\.css['"];/g, `@import url("${constants.ROUTE.DEVPORTAL_ASSETS_BASE_PATH}${orgId}/views/${viewName}/layout?fileType=style&fileName=main.css");`);
                    }
                    strContent = strContent.replace(/"\/images\/(devportalLogo\.[^"]+)/g, `"${constants.ROUTE.DEVPORTAL_ASSETS_BASE_PATH}${orgId}/views/${viewName}/layout?fileType=image&fileName=$1`);
                    strContent = strContent.replace(/'\/images\/(devportalLogo\.[^']+)/g, `"${constants.ROUTE.DEVPORTAL_ASSETS_BASE_PATH}${orgId}/views/${viewName}/layout?fileType=image&fileName=$1`);
                    content = Buffer.from(strContent, constants.CHARSET_UTF8);
                } else if (file.name.endsWith(".hbs") && dir.endsWith("layout")) {
                    fileType = "layout"
                    if (file.name === "main.hbs") {
                        strContent = strContent.replace(/\/styles\//g, `${constants.ROUTE.DEVPORTAL_ASSETS_BASE_PATH}${orgId}/views/${viewName}/layout?fileType=style&fileName=`);
                        content = Buffer.from(strContent, constants.CHARSET_UTF8);
                    }
                    validateScripts(strContent);
                } else if (file.name.endsWith(".hbs") && dir.endsWith("partials")) {
                    strContent = strContent.replace(/"\/images\/([^"]+)/g, `"${constants.ROUTE.DEVPORTAL_ASSETS_BASE_PATH}${orgId}/views/${viewName}/layout?fileType=image&fileName=$1`);
                    strContent = strContent.replace(/'\/images\/([^']+)/g, `'${constants.ROUTE.DEVPORTAL_ASSETS_BASE_PATH}${orgId}/views/${viewName}/layout?fileType=image&fileName=$1`);
                    content = Buffer.from(strContent, constants.CHARSET_UTF8);
                    validateScripts(strContent);
                    fileType = "partial"
                } else if (file.name.endsWith(".md") && dir.endsWith("content")) {
                    fileType = "markDown";
                } else if (file.name.endsWith(".hbs")) {
                    validateScripts(strContent);
                    fileType = "template";
                } else if (imageExtensions.includes(fileExtension)) {
                    fileType = "image";
                } else {
                    // Unexpected file type
                    logger.error(`Unexpected file type detected: ${file.name}`, {
                        fileName: file.name,
                        fileExtension: fileExtension,
                        directory: directory,
                        orgId: orgId
                    });
                    throw new CustomError(400, `Bad Request`, `Unexpected file type: ${file.name}`);
                }

                fileDetails.push({
                    filePath: dir,
                    fileName: file.name,
                    fileContent: content,
                    fileType: fileType
                });
            }
        }
        return fileDetails;
    } catch (error) {
        logger.error("Error occurred while reading files in directory", {
            directory: directory,
            orgId: orgId,
            viewName: viewName,
            error: error.message,
            description: error.description,
            stack: error.stack
        });
        throw new CustomError(error.statusCode || 500, error.message || 'Internal Server Error', error.description || 'Error reading files in directory');
    }
}


function validateScripts(strContent) {
    try {
        const allowedScripts = new Set([
            "<script src='https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/js/bootstrap.bundle.min.js'></script>",
            '<script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/js/bootstrap.bundle.min.js"></script>',
            '<script src="https://js.stripe.com/v3/"></script>',
            "<script src='/technical-scripts/search.js' defer></script>",
            "<script src='/technical-scripts/filter.js' defer></script>",
            "<script src='/technical-scripts/common.js' defer></script>",
            "<script src='/technical-scripts/subscription.js' defer></script>",
            "<script src='/technical-scripts/add-application-form.js' defer></script>",
            "<script src='/technical-scripts/platform-subscription.js' defer></script>",
            "<script src='/technical-scripts/subscription-modal.js' defer></script>",
            "<script src='/technical-scripts/subscriptions-page.js' defer></script>",
            "<script src='/technical-scripts/platform-api-keys-page.js' defer></script>",
            '<script src="/technical-scripts/oauth2-key-generation.js" defer></script>',
            '<script src="/technical-scripts/api-key-generation.js" defer></script>',
            '<script src="/technical-scripts/billing.js" defer></script>',
            "<script src='/technical-scripts/delete-confirmation-modal.js' defer></script>",
            "<script src='/technical-scripts/api-flow-detail.js' defer></script>",
            "<script src='/technical-scripts/api-workflows.js' defer></script>",
            "<script src='/technical-scripts/api-agent-prompt.js' defer></script>",
            '<script src="/technical-scripts/home-discover.js" defer></script>',
            '<script src="https://cdn.jsdelivr.net/npm/@jentic/arazzo-ui@1.0.0-alpha.30/dist/arazzo-ui.js" integrity="sha256-OYzURPQLK+lup5rGo+IQmVbjWOjVgjURBWDDtMHIOaw=" crossorigin="anonymous"></script>',
            '<script src="https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.min.js" integrity="sha256-Rdw90D3AegZwWiwpibjH9wkBPwS9U4bjJ51ORH8H69c=" crossorigin="anonymous"></script>',
            '<script src="https://cdn.jsdelivr.net/npm/marked@13.0.3/marked.min.js" integrity="sha256-Wt6n2O5BpwD8zBS7nVAxBPBHDMF6hK0+Fn0/UlHq4No=" crossorigin="anonymous"></script>',
            '<script src="https://cdnjs.cloudflare.com/ajax/libs/dompurify/3.2.7/purify.min.js" integrity="sha512-78KH17QLT5e55GJqP76vutp1D2iAoy06WcYBXB6iBCsmO6wWzx0Qdg8EDpm8mKXv68BcvHOyeeP4wxAL0twJGQ==" crossorigin="anonymous"></script>',
        ]);
        const allowedInlineScripts = new Set([
            // Reo analytics loader (src/defaultContent/layout/main.hbs)
            "<script type=\"text/javascript\">\n      !function(){var e,t,n;e=\"{{portalConfigs.reoClientID}}\",t=function(){Reo.init({clientID:\"{{portalConfigs.reoClientID}}\"})},(n=document.createElement(\"script\")).src=\"https://static.reo.dev/\"+e+\"/reo.js\",n.defer=!0,n.onload=t,document.head.appendChild(n)}();\n    </script>",
            // Token-map JSON data island (api-landing/partials/api-subscription-plans.hbs)
            "<script id=\"token-map-data\" type=\"application/json\">{{{jsonSafePlatformSubscriptions ../platformSubscriptions}}}</script>",
            // Token-meta bootstrap (api-landing/partials/api-subscription-plans.hbs)
            "<script>\n                    (function() {\n                        var data = JSON.parse(document.getElementById('token-map-data').textContent || '[]');\n                        window.__tokenMeta = window.__tokenMeta || {};\n                        data.forEach(function(sub) {\n                            // store only non-sensitive metadata and masked token\n                            window.__tokenMeta[sub.subscriptionId] = {\n                                maskedToken: sub.maskedToken,\n                                customerName: sub.customerName,\n                                subscriptionPlanName: sub.subscriptionPlanName,\n                                status: sub.status\n                            };\n                        });\n                        // expose orgID for on-demand fetches\n                        window.__subscriptionOrgID = \"{{@root.orgID}}\";\n                    })();\n                </script>",
            // Existing-subs JSON data island (api-landing/partials/api-subscription-plans.hbs)
            "<script id=\"existing-subs-data\" type=\"application/json\">{{{json platformSubscriptions}}}</script>",
            // API flows JSON data island (pages/api-flows/page.hbs)
            "<script type=\"application/json\" id=\"apiFlowsDataContainer\">{{{json apiFlows}}}</script>",
            // AI agent data island (pages/api-landing/page.hbs)
            "<script type=\"application/json\" id=\"apiAgentData\">{\"baseUrl\":\"{{baseUrl}}\",\"apiHandle\":\"{{apiMetadata.apiHandle}}\"}</script>",
            // Home discover data island (pages/home/page.hbs)
            "<script type=\"application/json\" id=\"homeDiscoverData\">{\"baseUrl\":\"{{baseUrl}}\"}</script>",
            // Existing-subs bootstrap (api-landing/partials/api-subscription-plans.hbs)
            "<script>\n                (function() {\n                    window.__subscriptionOrgID = window.__subscriptionOrgID || \"{{@root.orgID}}\";\n                    var raw = document.getElementById('existing-subs-data').textContent || '[]';\n                    try {\n                        var parsed = JSON.parse(raw);\n                        window.existingPlatformSubscriptions = parsed.map(function(sub) {\n                            return { subscriptionId: sub.subscriptionId, subscriptionPlanName: sub.subscriptionPlanName, status: sub.status };\n                        });\n                    } catch (e) {\n                        window.existingPlatformSubscriptions = [];\n                    }\n                })();\n            </script>",
            // tokenMap + orgID bootstrap (api-subscriptions/partials/api-subscription-list.hbs
            // and subscriptions/partials/subscription-list.hbs)
            "<script>\n                window.__tokenMap = window.__tokenMap || {};\n                window.__subscriptionOrgID = \"{{@root.orgID}}\";\n            </script>",
            // Modal click handler (apis/partials/api-listing.hbs)
            "<script>\n    (function(){\n      function findClosest(el, selector){\n        while(el && el !== document){\n          if(el.matches && el.matches(selector)) return el;\n          el = el.parentNode;\n        }\n        return null;\n      }\n\n      document.addEventListener('click', function(e){\n        var modalTrigger = findClosest(e.target, '[data-modal]');\n        if(modalTrigger){\n          e.preventDefault();\n          if(modalTrigger.classList.contains('is-readonly') || modalTrigger.getAttribute('aria-disabled') === 'true'){\n            return;\n          }\n          if(typeof loadModal === 'function'){\n            loadModal(modalTrigger.getAttribute('data-modal'));\n          } else {\n            var id = modalTrigger.getAttribute('data-modal');\n            var el = document.getElementById(id);\n            if(el) {\n              el.style.display = 'flex';\n              document.body.classList.add('modal-open');\n              if(typeof prepareSubscriptionModal === 'function') {\n                try { prepareSubscriptionModal(id); } catch(err) { /* noop */ }\n              }\n            }\n          }\n          return;\n        }\n\n        var nav = findClosest(e.target, '[data-href]');\n        if(nav){\n          var href = nav.getAttribute('data-href');\n          if(href){ window.location.href = href; }\n        }\n      }, false);\n    })();\n  </script>",
        ]);

        const scriptRegex = /<script(?:\s+[^>]*)?>[\s\S]*?<\/script>/gi;
        let match;

        while ((match = scriptRegex.exec(strContent)) !== null) {
            const script = match[0].trim();
            const openingTag = script.match(/^<script(?:\s+[^>]*)?>/i)?.[0] || '';
            const hasSrc = /\bsrc\s*=/i.test(openingTag);

            if (!hasSrc) {
                const isEmpty = /^<script[^>]*>\s*<\/script>$/i.test(script);
                if (isEmpty || allowedInlineScripts.has(script)) {
                    continue;
                }
                logger.error("Script validation failed: inline scripts are not allowed", { script });
                throw new CustomError(400, constants.ERROR_CODE[400], `Inline scripts are not allowed in uploaded themes: ${script}`);
            }
            if (!allowedScripts.has(script)) {
                logger.error("Script validation failed: disallowed script tag found", { script });
                throw new CustomError(400, constants.ERROR_CODE[400], `Additional scripts not allowed: ${script}`);
            }
        }
    } catch (error) {
        if (!(error instanceof CustomError)) {
            logger.error("Error occurred while validating scripts", {
                error: error.message,
                description: error.description,
                stack: error.stack,
            });
        }
        throw error;
    }
}

function appendAPIImageURL(subList, req, orgID) {

    subList.forEach(element => {
        const images = element.apiInfo.apiImageMetadata;
        let apiImageUrl = '';
        for (const key in images) {
            apiImageUrl = `${constants.ROUTE.DEVPORTAL_ASSETS_BASE_PATH}${orgID}${constants.ROUTE.API_FILE_PATH}${element.apiID}${constants.API_TEMPLATE_FILE_NAME}`;
            const modifiedApiImageURL = apiImageUrl + images[key];
            element.apiInfo.apiImageMetadata[key] = modifiedApiImageURL;
        }
    });
}

async function appendSubscriptionPlanDetails(orgID, subscriptionPolicies) {
    let subscriptionPlans = [];
    if (subscriptionPolicies) {
        for (const policy of subscriptionPolicies) {
            const subscriptionPlan = await loadSubscriptionPlan(orgID, policy.policyName);
            if (!subscriptionPlan) {
                logger.warn('[appendSubscriptionPlanDetails] Plan not found, skipping', {
                    orgID,
                    policyName: policy.policyName
                });
                continue;
            }
            const billingPlanRaw = subscriptionPlan.billingPlan;
            const billingPlan = (typeof billingPlanRaw === 'string') ? billingPlanRaw.trim().toUpperCase() : '';
            const isPaid = billingPlan === 'COMMERCIAL';
            logger.debug('[appendSubscriptionPlanDetails] Plan:', {
                policyID: subscriptionPlan.policyID,
                policyName: subscriptionPlan.policyName,
                billingPlanRaw,
                billingPlan,
                isPaid
            });
            subscriptionPlans.push({
                policyID: subscriptionPlan.policyID,
                displayName: subscriptionPlan.displayName,
                policyName: subscriptionPlan.policyName,
                description: subscriptionPlan.description,
                billingPlan: subscriptionPlan.billingPlan,
                requestCount: subscriptionPlan.requestCount,
                pricingModel: subscriptionPlan.pricingModel,
                currency: subscriptionPlan.currency,
                billingPeriod: subscriptionPlan.billingPeriod,
                flatAmount: subscriptionPlan.flatAmount,
                unitAmount: subscriptionPlan.unitAmount,
                pricingMetadata: subscriptionPlan.pricingMetadata,
                isPaid: isPaid
            });
        }
    }
    return subscriptionPlans;
}

const loadSubscriptionPlan = async (orgID, policyName) => {

    try {
        const policyData = await apiDao.getSubscriptionPolicyByName(orgID, policyName);
        if (policyData) {
            return new subscriptionPolicyDTO(policyData);
        } else {
            throw new CustomError(404, constants.ERROR_CODE[404], constants.ERROR_MESSAGE.SUBSCRIPTION_POLICY_NOT_FOUND);
        }
    } catch (error) {
        logger.error("Error occurred while loading subscription plans", {
            orgID: orgID,
            policyName: policyName,
            error: error.message,
            stack: error.stack
        });
        return null;
    }
}

async function tokenExchanger(token, orgName) {
    logger.info(`Exchanging token for organization: ${orgName}`, {
        orgName: orgName,
        action: 'token_exchange'
    });
    const url = config.advanced.tokenExchanger.url;
    const maxRetries = 3;
    let delay = 1000;
    const orgDetails = await adminDao.getOrganization(orgName);
    if (!orgDetails) {
        throw new Error('Organization not found');
    } else if (!orgDetails.ORGANIZATION_IDENTIFIER) {
        throw new Error('Organization Identifier not found');
    }

    const data = qs.stringify({
        client_id: config.advanced.tokenExchanger.client_id,
        grant_type: config.advanced.tokenExchanger.grant_type,
        subject_token_type: config.advanced.tokenExchanger.subject_token_type,
        requested_token_type: config.advanced.tokenExchanger.requested_token_type,
        scope: config.advanced.tokenExchanger.scope,
        subject_token: token,
        orgHandle: orgDetails.ORG_HANDLE
    });

    for (let attempt = 0; attempt <= maxRetries; attempt++) {
        try {
            const response = await axios.post(url, data, {
                headers: {
                    'Referer': '',
                    'Accept': 'application/json',
                    'Content-Type': 'application/x-www-form-urlencoded'
                }
            });

            return response.data.access_token;
        } catch (error) {
            if (error.response?.status >= 500 && error.response?.status < 600 && attempt < maxRetries) {
                logger.warn(`Token exchange failed. Retrying in ${delay}ms... (Attempt ${attempt + 1}/${maxRetries})`, {
                    orgName: orgName,
                    statusCode: error.response?.status,
                    error: error.message
                });
                await new Promise(resolve => setTimeout(resolve, delay));
                delay *= 2;
            } else {
                logger.error('Token exchange failed', {
                    orgName: orgName,
                    attempt: attempt + 1,
                    error: error.message,
                    statusCode: error.response?.status,
                    responseData: error.response?.data
                });
                throw new Error('Failed to exchange token');
            }
        }
    }
}

async function listFiles(path) {

    let files = [];
    fs.promises.readdir(path, (err, files) => {
        if (err) {
            logger.error('Error reading directory', {
                path: path,
                error: err.message
            });
            return;
        }
        logger.debug('Files in directory', {
            path: path,
            fileCount: files.length,
            files: files
        });
    });
    return files;
}

async function findFileByNameRecursive(rootPath, targetNames) {
    const normalizedTargetNames = new Set(Array.from(targetNames).map(name => String(name).toLowerCase()));
    const stack = [rootPath];

    while (stack.length > 0) {
        const currentPath = stack.pop();
        const entries = await fs.promises.readdir(currentPath, { withFileTypes: true });
        for (const entry of entries) {
            if (entry.name === '.DS_Store' || entry.name === '__MACOSX') {
                continue;
            }
            const fullPath = path.join(currentPath, entry.name);
            if (entry.isDirectory()) {
                stack.push(fullPath);
                continue;
            }
            if (normalizedTargetNames.has(entry.name.toLowerCase())) {
                return fullPath;
            }
        }
    }
    return null;
}

function normalizeStringArray(value) {
    if (!Array.isArray(value)) {
        return [];
    }
    return value
        .filter(item => item !== undefined && item !== null && String(item).trim() !== '')
        .map(item => String(item).trim());
}

function resolveApiType(apiType) {
    if (!apiType || typeof apiType !== 'string') {
        return constants.API_TYPE.REST;
    }

    const resolvedType = apiType.replace(/\s+/g, '').toUpperCase();
    if (!Object.values(constants.API_TYPE).includes(resolvedType)) {
        throw new Sequelize.ValidationError(
            "Invalid api type. Supported values: REST, WS, GRAPHQL, SOAP, WEBSUB, MCP"
        );
    }
    return resolvedType;
}

function filterAllowedAPIs(searchResults, allowedAPIs) {

    searchResults = searchResults.filter(api => {
        const gatewayVendor = api?.apiInfo?.gatewayVendor || 'wso2';
        if (constants.FEDERATED_GATEWAY_VENDORS.includes(gatewayVendor)) {
            return true;
        }
        // MCP servers published via the registry have no referenceID – skip control plane check
        if (api?.apiInfo?.apiType === constants.API_TYPE.MCP && !api.apiReferenceID) {
            return true;
        }
        return allowedAPIs.some(allowedAPI => api.apiReferenceID === allowedAPI.id);
    });
    return searchResults;
}

const enforcePortalMode = async (req, res, next) => {
    const orgDetails = await adminDao.getOrganization(req.params.orgName);
    const portalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
    const path = req.originalUrl.split('/')[4];

    if ((path.includes('apis') || path.includes('api')) && (portalMode === constants.DEVPORTAL_MODE.DEFAULT || portalMode === constants.DEVPORTAL_MODE.API_PROXIES) ||
        (path.includes('mcps') || path.includes('mcp')) && (portalMode === constants.DEVPORTAL_MODE.DEFAULT || portalMode === constants.DEVPORTAL_MODE.MCP_ONLY)) {
        next();
    } else {
        const templateContent = {
            errorMessage: constants.ERROR_MESSAGE.COMMON_PAGE_NOT_FOUND_ERROR_MESSAGE,
            devportalMode: portalMode,
            baseUrl: '/' + req.params.orgName + constants.ROUTE.VIEWS_PATH + req.params.viewName,
        }
        const html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
        res.send(html);
    }
}

async function isAiDisabledForPortal(orgID, viewName) {
    const configAsset = await adminDao.getOrgContent({
        orgId: orgID, fileType: constants.FILE_TYPE.LLMS_CONFIG, viewName, fileName: constants.FILE_NAME.LLMS_CONFIG
    });
    if (!configAsset) return false;
    try {
        const llmsConfig = JSON.parse(configAsset.FILE_CONTENT.toString('utf8'));
        return llmsConfig.aiEnabled === false;
    } catch (e) {
        return false;
    }
}

module.exports = {
    loadMarkdown,
    renderTemplate,
    loadLayoutFromAPI,
    loadTemplateFromAPI,
    renderTemplateFromAPI,
    renderMarkdownTemplateFromAPI,
    renderLlmsTxt,
    renderGivenTemplate,
    handleError,
    retrieveContentType,
    getAPIFileContent,
    getAPIImages,
    getAPIDocLinks,
    isTextFile,
    invokeApiRequest,
    apiRequest,
    invokeGraphQLRequest,
    validateIDP,
    validateOrganization,
    getErrors,
    validateProvider,
    validateRequestParameters,
    rejectExtraProperties,
    readFilesInDirectory,
    appendAPIImageURL,
    appendSubscriptionPlanDetails,
    tokenExchanger,
    listFiles,
    readDocFiles,
    findFileByNameRecursive,
    unzipDirectory,
    filterAllowedAPIs,
    enforcePortalMode,
    isAiDisabledForPortal,
    isImageFile,
    normalizeStringArray,
    resolveApiType
}
