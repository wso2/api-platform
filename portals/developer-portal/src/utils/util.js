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
const config = require(process.cwd() + '/config.json');
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

    return layout({
        body: template(templateContent),
        portalConfigs: config.portalConfigs,
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

    return layout({
        body: template(templateContent),
        portalConfigs: config.portalConfigs,
    });

}

async function renderGivenTemplate(templatePage, layoutPage, templateContent) {

    const template = Handlebars.compile(templatePage.toString());
    const layout = Handlebars.compile(layoutPage.toString());
    return layout({
        body: template(templateContent),
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

const isTextFile = (fileExtension) => {
    return textFiles.includes(fileExtension)
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

async function readDocFiles(directory, baseDir = '') {

    const files = await fs.promises.readdir(directory, { withFileTypes: true });
    let fileDetails = [];
    for (const file of files) {
        const filePath = path.join(directory, file.name);
        const relativePath = path.join(baseDir, file.name);
        if (file.isDirectory()) {
            const subDirContents = await readDocFiles(filePath, relativePath);
            fileDetails = fileDetails.concat(subDirContents);
        } else {
            if (!(file.name === '.DS_Store')) {
                let content = await fs.promises.readFile(filePath);
                fileDetails.push({
                    type: constants.DOC_TYPES.DOC_ID + baseDir,
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

    const options = {
        method,
        headers,
        httpsAgent,
    };

    try {
        if (!(body == null || body === '' || (Array.isArray(body) && body.length === 0) || (typeof body === 'object' && !Array.isArray(body) && Object.keys(body).length === 0))) {
            options.data = body;
        }
        if (config.advanced.tokenExchanger?.enabled) {
            let orgId = "";
            if (req.cpOrgID) {
                orgId = req.cpOrgID;
                url = url.includes("?") ? `${url}&organizationId=${orgId}` : `${url}?organizationId=${orgId}`;
            } else {
                const decodedToken = jwt.decode(req.user.exchangeToken);
                orgId = decodedToken?.organization.uuid;
                url = url.includes("?") ? `${url}&organizationId=${orgId}` : `${url}?organizationId=${orgId}`;
            }

        }
        const response = await axios(url, options);
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
                options.headers = headers;
                const response = await axios(url, options);
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
            stack: error.stack
        });
        throw new CustomError(error.statusCode || 500, error.message || 'Internal Server Error', error.description || 'Error reading files in directory');
    }
}


function validateScripts(strContent) {
    try {
        const allowedScripts = new Set([
            "<script src='https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/js/bootstrap.bundle.min.js'></script>",
            "<script src='/technical-scripts/search.js' defer></script>",
            "<script src='/technical-scripts/filter.js' defer></script>",
            "<script src='/technical-scripts/common.js' defer></script>",
            "<script src='/technical-scripts/subscription.js' defer></script>",
            "<script src='/technical-scripts/add-application-form.js' defer></script>"
        ]);

        const scriptRegex = /<script(?:\s+[^>]*)?>[\s\S]*?<\/script>/g;
        let match;
        const extractedScripts = new Set();

        while ((match = scriptRegex.exec(strContent)) !== null) {
            extractedScripts.add(match[0].trim());
        }

        for (const script of extractedScripts) {
            if (!allowedScripts.has(script)) {
                throw new CustomError(400, constants.ERROR_CODE[400], `Additional scripts not allowed`);
            }
        }
    } catch (error) {
        logger.error("Error occurred while validating scripts", {
            error: error.message,
            stack: error.stack,
        });
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
            subscriptionPlans.push({
                policyID: subscriptionPlan.policyID,
                displayName: subscriptionPlan.displayName,
                policyName: subscriptionPlan.policyName,
                description: subscriptionPlan.description,
                billingPlan: subscriptionPlan.billingPlan,
                requestCount: subscriptionPlan.requestCount,
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
        util.handleError(res, error);
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

function filterAllowedAPIs(searchResults, allowedAPIs) {

    searchResults = searchResults.filter(api =>
        allowedAPIs.some(allowedAPI => api.apiReferenceID === allowedAPI.id)
    );
    return searchResults;
}

const enforcePortalMode = async (req, res, next) => {
    const orgDetails = await adminDao.getOrganization(req.params.orgName);
    const portalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.API_TYPE.DEFAULT;
    const path = req.originalUrl.split('/')[4];

    if ((path.includes('apis') || path.includes('api')) && (portalMode === constants.API_TYPE.DEFAULT || portalMode === constants.API_TYPE.API_PROXIES) ||
        (path.includes('mcps') || path.includes('mcp')) && (portalMode === constants.API_TYPE.DEFAULT || portalMode === constants.API_TYPE.MCP_ONLY)) {
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

module.exports = {
    loadMarkdown,
    renderTemplate,
    loadLayoutFromAPI,
    loadTemplateFromAPI,
    renderTemplateFromAPI,
    renderGivenTemplate,
    handleError,
    retrieveContentType,
    getAPIFileContent,
    getAPIImages,
    getAPIDocLinks,
    isTextFile,
    invokeApiRequest,
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
    unzipDirectory,
    filterAllowedAPIs,
    enforcePortalMode
}