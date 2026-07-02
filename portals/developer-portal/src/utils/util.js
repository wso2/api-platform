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
const orgDao = require('../dao/organizationDao');
const constants = require('../utils/constants');
const unzipper = require('unzipper');
const axios = require('axios');
const qs = require('qs');
const https = require('https');
const { config } = require('../config/configLoader');
const { body, param, query } = require('express-validator');
const { Sequelize } = require('sequelize');
const apiDao = require('../dao/apiDao');
const subscriptionPlanDao = require('../dao/subscriptionPlanDao');
const subscriptionPlanDTO = require('../dto/subscriptionPlanDto');
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


/**
 * In design mode, if a template/layout file doesn't exist at the given path
 * (which may be under a custom pathToLayout), fall back to the same relative
 * path under src/defaultContent/.
 */
function resolveDesignFallback(filePath) {
    if (!config.designMode?.enabled) return filePath;
    // Resolve relative to cwd so both relative and absolute pathToLayout values work
    const abs = path.resolve(process.cwd(), filePath);
    if (fs.existsSync(abs)) return abs;
    const designRoot = path.resolve(process.cwd(), config.designMode.pathToLayout);
    if (abs.startsWith(designRoot)) {
        // Strip the leading path separator so the relative part doesn't look absolute
        const relative = abs.slice(designRoot.length).replace(/^[/\\]/, '');
        return path.resolve(process.cwd(), './src/defaultContent', relative);
    }
    return abs;
}

function renderTemplate(templatePath, layoutPath, templateContent, isTechnical) {

    let completeTemplatePath;
    if (isTechnical) {
        completeTemplatePath = path.join(require.main.filename, templatePath);
    } else {
        completeTemplatePath = resolveDesignFallback(templatePath);
    }

    const templateResponse = fs.readFileSync(completeTemplatePath, constants.CHARSET_UTF8);
    const completeLayoutPath = resolveDesignFallback(layoutPath);
    const layoutResponse = fs.readFileSync(completeLayoutPath, constants.CHARSET_UTF8)

    const template = Handlebars.compile(templateResponse.toString());
    const layout = Handlebars.compile(layoutResponse.toString());

    const slots = {};
    const showApiWorkflowsNav = config.features?.apiWorkflows?.enabled === true;
    const enrichedContent = { devportalMode: constants.DEVPORTAL_MODE.DEFAULT, ...templateContent, showApiWorkflowsNav, slots };
    return layout({
        ...enrichedContent,
        body: template(enrichedContent),
        portalConfigs: config.portalConfigs,
        devportalApiConfig: {
            base: constants.DEVPORTAL_API.BASE_SEGMENT,
            version: constants.DEVPORTAL_API.VERSION,
        },
        devReloadEnabled: process.env.NODE_ENV === 'development',
    });
}

async function loadLayoutFromAPI(orgId, viewName) {

    var layoutContent = await orgDao.getContent({
        orgId: orgId,
        fileType: constants.FILE_TYPE.LAYOUT,
        fileName: constants.FILE_NAME.MAIN,
        viewName: viewName
    });
    if (layoutContent) {
        return layoutContent.file_content.toString(constants.CHARSET_UTF8);
    } else {
        return "";
    }
}

async function loadTemplateFromAPI(orgId, filePath, viewName) {

    var templateContent = await orgDao.getContent({
        orgId: orgId,
        filePath: filePath,
        fileType: constants.FILE_TYPE.TEMPLATE,
        fileName: constants.FILE_NAME.PAGE,
        viewName: viewName
    });
    return templateContent ? templateContent.file_content.toString(constants.CHARSET_UTF8) : "";
}

async function renderTemplateFromAPI(templateContent, orgId, orgName, filePath, viewName) {

    const templateResponse = fs.readFileSync(path.join(process.cwd(), filePrefix + filePath + '/page.hbs'), constants.CHARSET_UTF8);
    const completeLayoutPath = path.join(process.cwd(), filePrefix + 'layout/main.hbs');

    layoutResponse = fs.readFileSync(completeLayoutPath, constants.CHARSET_UTF8);
    const styleContent = await orgDao.getContent({ orgId: orgId, fileType: 'style', viewName: viewName, fileName: 'main.css' });
    if (styleContent) {
        layoutResponse = layoutResponse.replace(/\/styles\//g, `${constants.DEVPORTAL_API.orgPath(orgId)}/views/${viewName}/asset?fileType=style&fileName=`);
    }

    const template = Handlebars.compile(templateResponse.toString());
    const layout = Handlebars.compile(layoutResponse.toString());

    const slots = {};
    const showApiWorkflowsNav = config.features?.apiWorkflows?.enabled === true;
    const enrichedContent = { devportalMode: constants.DEVPORTAL_MODE.DEFAULT, ...templateContent, showApiWorkflowsNav, slots };
    return layout({
        ...enrichedContent,
        body: template(enrichedContent),
        portalConfigs: config.portalConfigs,
        devportalApiConfig: {
            base: constants.DEVPORTAL_API.BASE_SEGMENT,
            version: constants.DEVPORTAL_API.VERSION,
        },
        devReloadEnabled: process.env.NODE_ENV === 'development',
    });

}

async function renderLlmsTxt(templateContent, orgId, viewName) {

    const dbPartial = await orgDao.getContent({
        orgId: orgId,
        fileType: 'partial',
        viewName: viewName,
        fileName: 'llms-txt.hbs'
    });
    const partialSource = dbPartial
        ? dbPartial.file_content.toString(constants.CHARSET_UTF8)
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

async function renderMarkdownTemplateFromAPI(templateContent, orgId, filePath, viewName) {

    const partialName = path.basename(filePath) + '-md';
    const dbPartial = await orgDao.getContent({
        orgId: orgId,
        fileType: 'partial',
        viewName: viewName,
        fileName: partialName + '.hbs'
    });
    const partialSource = dbPartial
        ? dbPartial.file_content.toString(constants.CHARSET_UTF8)
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
    const slots = {};
    const showApiWorkflowsNav = config.features?.apiWorkflows?.enabled === true;
    const enrichedContent = { devportalMode: constants.DEVPORTAL_MODE.DEFAULT, ...templateContent, showApiWorkflowsNav, slots };
    return layout({
        ...enrichedContent,
        body: template(enrichedContent),
        portalConfigs: config.portalConfigs,
        devportalApiConfig: {
            base: constants.DEVPORTAL_API.BASE_SEGMENT,
            version: constants.DEVPORTAL_API.VERSION,
        },
        devReloadEnabled: process.env.NODE_ENV === 'development',
    });
}

const HTTP_CODE_TO_CATALOG = {
    400: 'COMMON_VALIDATION_ERROR',
    401: 'UNAUTHORIZED',
    403: 'FORBIDDEN',
    404: 'RESOURCE_NOT_FOUND',
    409: 'CONFLICT',
    500: 'INTERNAL_SERVER_ERROR',
};

function getErrors(errors) {
    const errorList = [];
    errors.errors.forEach(element => {
        errorList.push({ field: element.path || element.param || undefined, message: element.msg });
    });
    return errorList;
}

function handleError(res, error) {
    if (error instanceof Sequelize.UniqueConstraintError) {
        const msg = error.errors ? error.errors[0].message : error.message.replaceAll('"', '');
        return res.status(409).json({
            status: 'error',
            code: 'CONFLICT',
            message: 'Conflict',
            errors: [{ message: msg }],
        });
    } else if (error instanceof Sequelize.ValidationError) {
        return res.status(400).json({
            status: 'error',
            code: 'COMMON_VALIDATION_ERROR',
            message: 'Bad Request',
            errors: [{ message: error.message }],
        });
    } else if (error instanceof Sequelize.EmptyResultError) {
        return res.status(404).json({
            status: 'error',
            code: 'RESOURCE_NOT_FOUND',
            message: 'Resource Not Found',
            errors: [],
        });
    } else if (error instanceof CustomError) {
        const code = HTTP_CODE_TO_CATALOG[error.statusCode] || 'INTERNAL_SERVER_ERROR';
        return res.status(error.statusCode).json({
            status: 'error',
            code,
            message: error.message,
            errors: error.description ? [{ message: error.description }] : [],
        });
    } else {
        return res.status(500).json({
            status: 'error',
            code: 'INTERNAL_SERVER_ERROR',
            message: 'Internal Server Error',
            errors: [],
        });
    }
}

function toPaginatedList(list, req) {
    const limit = Math.min(parseInt((req.query && req.query.limit) || '20', 10) || 20, 100);
    const offset = parseInt((req.query && req.query.offset) || '0', 10) || 0;
    return {
        list,
        pagination: { total: list.length, limit, offset },
    };
}

/**
 * Resolve the acting user identity from a request for audit columns
 * (CREATED_BY / UPDATED_BY). Covers every auth mode wired in the portal:
 *   - OpenAPI router (authResolver): oauth2 / platform-jwt set req.auth.userId
 *   - enforceSecurity router (e.g. MCP registry): sets req[constants.USER_ID]
 *   - session/passport users: req.user.sub
 * Machine credentials (API key, mTLS) carry no user identity, so fall back to
 * the SYSTEM actor to keep the NOT NULL audit columns satisfiable instead of
 * throwing a constraint violation.
 */
function resolveActor(req) {
    return req?.auth?.userId
        || req?.[constants.USER_ID]
        || req?.user?.[constants.USER_ID]
        || constants.SYSTEM_ACTOR;
}

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
                        strContent = strContent.replace(/@import\s*['"]\/styles\/api-content\.css['"];/g, `@import url("${constants.DEVPORTAL_API.orgPath(orgId)}/views/${viewName}/asset?fileType=style&fileName=api-content.css");`);
                        strContent = strContent.replace(/@import\s*['"]\/styles\/home\.css['"];/g, `@import url("${constants.DEVPORTAL_API.orgPath(orgId)}/views/${viewName}/asset?fileType=style&fileName=home.css");`);
                        strContent = strContent.replace(/@import\s*['"]\/styles\/main\.css['"];/g, `@import url("${constants.DEVPORTAL_API.orgPath(orgId)}/views/${viewName}/asset?fileType=style&fileName=main.css");`);
                    }
                    strContent = strContent.replace(/"\/images\/(devportal-logo\.[^"]+)/g, `"${constants.DEVPORTAL_API.orgPath(orgId)}/views/${viewName}/asset?fileType=image&fileName=$1`);
                    strContent = strContent.replace(/'\/images\/(devportal-logo\.[^']+)/g, `'${constants.DEVPORTAL_API.orgPath(orgId)}/views/${viewName}/asset?fileType=image&fileName=$1`);
                    content = Buffer.from(strContent, constants.CHARSET_UTF8);
                } else if (file.name.endsWith(".hbs") && dir.endsWith("layout")) {
                    fileType = "layout"
                    if (file.name === "main.hbs") {
                        strContent = strContent.replace(/\/styles\//g, `${constants.DEVPORTAL_API.orgPath(orgId)}/views/${viewName}/asset?fileType=style&fileName=`);
                        content = Buffer.from(strContent, constants.CHARSET_UTF8);
                    }
                    validateScripts(strContent);
                } else if (file.name.endsWith(".hbs") && dir.endsWith("partials")) {
                    strContent = strContent.replace(/"\/images\/([^"]+)/g, `"${constants.DEVPORTAL_API.orgPath(orgId)}/views/${viewName}/asset?fileType=image&fileName=$1`);
                    strContent = strContent.replace(/'\/images\/([^']+)/g, `'${constants.DEVPORTAL_API.orgPath(orgId)}/views/${viewName}/asset?fileType=image&fileName=$1`);
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
            "<script src='/technical-scripts/search.js' defer></script>",
            "<script src='/technical-scripts/filter.js' defer></script>",
            "<script src='/technical-scripts/common.js' defer></script>",
            "<script src='/technical-scripts/subscription.js' defer></script>",
            "<script src='/technical-scripts/add-application-form.js' defer></script>",
            "<script src='/technical-scripts/subscription.js' defer></script>",
            "<script src='/technical-scripts/subscription-modal.js' defer></script>",
            "<script src='/technical-scripts/subscriptions-page.js' defer></script>",
            "<script src='/technical-scripts/api-keys-page.js' defer></script>",
            '<script src="/technical-scripts/oauth2-key-generation.js" defer></script>',
            "<script src='/technical-scripts/delete-confirmation-modal.js' defer></script>",
            "<script src='/technical-scripts/api-workflow-detail.js' defer></script>",
            "<script src='/technical-scripts/api-workflows.js' defer></script>",
            "<script src='/technical-scripts/api-agent-prompt.js' defer></script>",
            '<script src="/technical-scripts/home-discover.js" defer></script>',
            '<script src="https://cdn.jsdelivr.net/npm/@jentic/arazzo-ui@1.0.0-alpha.30/dist/arazzo-ui.js" integrity="sha256-OYzURPQLK+lup5rGo+IQmVbjWOjVgjURBWDDtMHIOaw=" crossorigin="anonymous"></script>',
            '<script src="https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.min.js" integrity="sha256-Rdw90D3AegZwWiwpibjH9wkBPwS9U4bjJ51ORH8H69c=" crossorigin="anonymous"></script>',
            '<script src="https://cdn.jsdelivr.net/npm/marked@13.0.3/marked.min.js" integrity="sha256-Wt6n2O5BpwD8zBS7nVAxBPBHDMF6hK0+Fn0/UlHq4No=" crossorigin="anonymous"></script>',
            '<script src="https://cdnjs.cloudflare.com/ajax/libs/dompurify/3.2.7/purify.min.js" integrity="sha512-78KH17QLT5e55GJqP76vutp1D2iAoy06WcYBXB6iBCsmO6wWzx0Qdg8EDpm8mKXv68BcvHOyeeP4wxAL0twJGQ==" crossorigin="anonymous"></script>',
        ]);
        const allowedInlineScripts = new Set([
            // Token-map JSON data island (api-landing/partials/api-subscription-plans.hbs)
            "<script id=\"token-map-data\" type=\"application/json\">{{{jsonSafeSubscriptions ../subscriptions}}}</script>",
            // Token-meta bootstrap (api-landing/partials/api-subscription-plans.hbs)
            "<script>\n                    (function() {\n                        var data = JSON.parse(document.getElementById('token-map-data').textContent || '[]');\n                        window.__tokenMeta = window.__tokenMeta || {};\n                        data.forEach(function(sub) {\n                            // store only non-sensitive metadata and masked token\n                            window.__tokenMeta[sub.subscriptionId] = {\n                                maskedToken: sub.maskedToken,\n                                customerName: sub.customerName,\n                                subscriptionPlanName: sub.subscriptionPlanName,\n                                status: sub.status\n                            };\n                        });\n                        // expose orgId for on-demand fetches\n                        window.__subscriptionOrgId = \"{{@root.orgId}}\";\n                    })();\n                </script>",
            // Existing-subs JSON data island (api-landing/partials/api-subscription-plans.hbs)
            "<script id=\"existing-subs-data\" type=\"application/json\">{{{json subscriptions}}}</script>",
            // API workflows JSON data island (pages/api-workflows/page.hbs)
            "<script type=\"application/json\" id=\"apiWorkflowsDataContainer\">{{{json apiWorkflows}}}</script>",
            // AI agent data island (pages/api-landing/page.hbs)
            "<script type=\"application/json\" id=\"apiAgentData\">{\"baseUrl\":\"{{baseUrl}}\",\"handle\":\"{{apiMetadata.handle}}\"}</script>",
            // Home discover data island (pages/home/page.hbs)
            "<script type=\"application/json\" id=\"homeDiscoverData\">{\"baseUrl\":\"{{baseUrl}}\"}</script>",
            // Existing-subs bootstrap (api-landing/partials/api-subscription-plans.hbs)
            "<script>\n                (function() {\n                    window.__subscriptionOrgId = window.__subscriptionOrgId || \"{{@root.orgId}}\";\n                    var raw = document.getElementById('existing-subs-data').textContent || '[]';\n                    try {\n                        var parsed = JSON.parse(raw);\n                        window.existingSubscriptions = parsed.map(function(sub) {\n                            return { subscriptionId: sub.subscriptionId, subscriptionPlanName: sub.subscriptionPlanName, status: sub.status };\n                        });\n                    } catch (e) {\n                        window.existingSubscriptions = [];\n                    }\n                })();\n            </script>",
            // tokenMap + orgId bootstrap (api-subscriptions/partials/api-subscription-list.hbs
            // and subscriptions/partials/subscription-list.hbs)
            "<script>\n                window.__tokenMap = window.__tokenMap || {};\n                window.__subscriptionOrgId = \"{{@root.orgId}}\";\n            </script>",
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

function appendAPIImageURL(subList, req, orgId) {

    subList.forEach(element => {
        const images = element.apiImageMetadata;
        let apiImageUrl = '';
        for (const key in images) {
            apiImageUrl = `${constants.DEVPORTAL_API.orgPath(orgId)}${constants.ROUTE.API_FILE_PATH}${element.id}${constants.API_TEMPLATE_FILE_NAME}`;
            const modifiedApiImageURL = apiImageUrl + images[key];
            element.apiImageMetadata[key] = modifiedApiImageURL;
        }
    });
}

async function appendSubscriptionPlanDetails(orgId, subscriptionPlans) {
    const enrichedPlans = [];
    if (subscriptionPlans) {
        for (const plan of subscriptionPlans) {
            const subscriptionPlan = await loadSubscriptionPlan(orgId, plan.handle);
            if (!subscriptionPlan) {
                logger.warn('Subscription plan not found, skipping', {
                    orgId,
                    handle: plan.handle
                });
                continue;
            }
            enrichedPlans.push({
                id: subscriptionPlan.id,
                name: subscriptionPlan.name,
                handle: subscriptionPlan.handle,
                description: subscriptionPlan.description,
                limits: subscriptionPlan.limits || [],
            });
        }
    }
    return enrichedPlans;
}

const loadSubscriptionPlan = async (orgId, planName) => {

    try {
        const planData = await subscriptionPlanDao.getByName(orgId, planName);
        if (planData) {
            return new subscriptionPlanDTO(planData);
        } else {
            throw new CustomError(404, constants.ERROR_CODE[404], constants.ERROR_MESSAGE.SUBSCRIPTION_PLAN_NOT_FOUND);
        }
    } catch (error) {
        logger.error("Error occurred while loading subscription plans", {
            orgId: orgId,
            planName: planName,
            error: error.message,
            stack: error.stack
        });
        return null;
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

    // Accept the stored value as-is (e.g. "RestApi", sent by devportal's own UI),
    // otherwise fall back to the short authoring keyword (e.g. "REST" in an uploaded api.yaml).
    if (Object.values(constants.API_TYPE).includes(apiType)) {
        return apiType;
    }

    const keyword = apiType.replace(/\s+/g, '').toUpperCase();
    if (!Object.prototype.hasOwnProperty.call(constants.API_TYPE, keyword)) {
        throw new Sequelize.ValidationError(
            "Invalid api type. Supported values: REST, WS, GRAPHQL, SOAP, WEBSUB, MCP"
        );
    }
    return constants.API_TYPE[keyword];
}

function filterAllowedAPIs(searchResults, allowedAPIs) {
    searchResults = searchResults.filter(api => {
        // MCP servers published via the registry have no referenceId
        if (api?.type === constants.API_TYPE.MCP && !api.refId) {
            return true;
        }
        return allowedAPIs.some(allowedAPI => api.refId === allowedAPI.id);
    });
    return searchResults;
}

const enforcePortalMode = async (req, res, next) => {
    const orgDetails = await orgDao.get(req.params.orgName);
    const portalMode = orgDetails.configuration?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
    const path = req.originalUrl.split('/')[4];

    if ((path.includes('apis') || path.includes('api')) && (portalMode === constants.DEVPORTAL_MODE.DEFAULT || portalMode === constants.DEVPORTAL_MODE.APIS_ONLY) ||
        (path.includes('mcps') || path.includes('mcp')) && (portalMode === constants.DEVPORTAL_MODE.DEFAULT || portalMode === constants.DEVPORTAL_MODE.MCP_SERVERS_ONLY)) {
        next();
    } else {
        const err = new Error('Page not found');
        err.status = 404;
        next(err);
    }
}

async function isAiDisabledForPortal(orgId, viewName) {
    const configAsset = await orgDao.getContent({
        orgId: orgId, fileType: constants.FILE_TYPE.LLMS_CONFIG, viewName, fileName: constants.FILE_NAME.LLMS_CONFIG
    });
    if (!configAsset) return false;
    try {
        const llmsConfig = JSON.parse(configAsset.file_content.toString('utf8'));
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
    validateIDP,
    validateOrganization,
    getErrors,
    validateRequestParameters,
    rejectExtraProperties,
    readFilesInDirectory,
    appendAPIImageURL,
    appendSubscriptionPlanDetails,
    listFiles,
    readDocFiles,
    findFileByNameRecursive,
    unzipDirectory,
    filterAllowedAPIs,
    enforcePortalMode,
    isAiDisabledForPortal,
    isImageFile,
    normalizeStringArray,
    resolveApiType,
    toPaginatedList,
    resolveActor,
    filePrefix,
}
