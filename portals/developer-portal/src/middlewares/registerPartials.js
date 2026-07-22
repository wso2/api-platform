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
const exphbs = require('express-handlebars');
const { config } = require('../config/configLoader');
const markdown = require('marked');
const orgDao = require('../dao/organizationDao');
const apiDao = require('../dao/apiDao');
const apiFileDao = require('../dao/apiFileDao');
const constants = require('../utils/constants');
const apiMetadataService = require('../services/apiMetadataService');
const util = require('../utils/util');
const { validationResult } = require('express-validator');
const logger = require('../config/logger');
const hbs = exphbs.create({});

const registerPartials = async (req, res, next) => {

  const rules = util.validateRequestParameters();
  for (let validation of rules) {
    await validation.run(req);
  }
  const errors = validationResult(req);
  if (!errors.isEmpty()) {
    return res.status(400).json(util.getErrors(errors));
  }
  registerInternalPartials(req);
  if (config.designMode?.enabled) {
    const baseUrl = config.server.baseUrl + constants.ROUTE.VIEWS_PATH + req.params.viewName;
    // Always load the full set of defaults first so no partial is missing
    await registerAllPartialsFromFile(baseUrl, req, './src/defaultContent');
    // Then override with the designer's custom files (skip if pathToLayout is already src/defaultContent)
    const layoutPath = path.resolve(config.designMode.pathToLayout);
    const defaultPath = path.resolve('./src/defaultContent');
    if (layoutPath !== defaultPath) {
      await registerAllPartialsFromFile(baseUrl, req, config.designMode.pathToLayout);
    }
  } else {
    let matchURL = req.originalUrl;
    if (req.session.returnTo) {
      matchURL = req.session.returnTo;
    }
    let devportalMode = constants.DEVPORTAL_MODE.DEFAULT;

    try {
      const orgDetails = await orgDao.get(req.params.orgName);
      devportalMode = orgDetails.configuration?.devportalMode || devportalMode;
      
      // Org-scoped settings page (/:orgName/settings) has no view segment, but still
      // renders the default-content chrome (sidebar/header/footer) and its own partials.
      // Register them against the default view's baseUrl.
      const isOrgSettings = req.params.orgName && req.params.orgName !== "portal"
        && !req.params.viewName && /^\/[^/]+\/settings(?:[/?#]|$)/i.test(matchURL);
      const isNonConfigure = req.params.orgName && req.params.orgName !== "portal"
        && req.params.viewName;

      if (isNonConfigure || isOrgSettings) {
        // The org-scoped settings route carries no view segment. Downstream partial
        // resolution (registerPartialsFromFile) reads req.params.viewName to look up
        // per-view custom overrides, so default it to 'default' — the settings page
        // renders the default view's chrome and its own default-content partials.
        if (isOrgSettings && !req.params.viewName) {
          req.params.viewName = 'default';
        }
        const viewSegment = req.params.viewName || 'default';
        const baseUrl = config.server.baseUrl + "/" + req.params.orgName + constants.ROUTE.VIEWS_PATH + viewSegment;
        await registerAllPartialsFromFile(baseUrl, req, './src/defaultContent');

        if (isNonConfigure) {
          const orgId = await orgDao.getId(req.params.orgName);
          await registerPartialsFromAPI(req, orgId);
          //register doc page partials
          if (req.originalUrl.includes(constants.ROUTE.API_DOCS_PATH) && req.params.docType && req.params.docName) {
            await registerDocsPageContent(req, orgId, {});
          } else if (req.originalUrl.includes(constants.ROUTE.API_LANDING_PAGE_PATH)) {
            await registerAPILandingContent(req, orgId, {});
          }
        }
      }
    } catch (error) {
      logger.error('Error while loading organization', { 
        error: error.message, 
        stack: error.stack,
        orgName: req.params.orgName,
        operation: 'registerPartials'
      });
      if (error.message === "Organization not found") {
        return res.redirect('/?error=org_not_found&org=' + encodeURIComponent(req.params.orgName));
      }
      if (error.message === "API not found") {
        const notFound = new Error('API not found');
        notFound.status = 404;
        return next(notFound);
      }
      next(error);
    }
  }
  next();
};

const registerInternalPartials = async (req) => {

  let isAdmin, isSuperAdmin = false;
  if (req.user) {
    isAdmin = req.user["isAdmin"];
    isSuperAdmin = req.user["isSuperAdmin"];
  }
  const partialsDir = path.join(path.join(require.main.filename, '..', '/pages/partials'));
  const getDirectories = source =>
    fs.readdirSync(source, { withFileTypes: true })
      .filter(dirent => dirent.isDirectory())
      .map(dirent => path.join(source, dirent.name));

  const partialsDirs = [partialsDir, ...getDirectories(path.join(require.main.filename, '..', '/pages')).map(dir => path.join(dir, 'partials'))];
  for (const dir of partialsDirs) {
    if (fs.existsSync(dir)) {
      fs.readdirSync(dir).forEach(file => {
        if (file.endsWith('.hbs')) {
          const partialName = path.basename(file, '.hbs');
          const partialContent = fs.readFileSync(path.join(dir, file), 'utf8');
          hbs.handlebars.registerPartial(partialName, partialContent);
        }
      });
    }
  };
}

const registerAllPartialsFromFile = async (baseURL, req, filePrefix) => {

  // Use path.resolve so both relative ("./my-theme/") and absolute ("/abs/path/")
  // values of filePrefix work correctly.
  const base = (...parts) => path.resolve(process.cwd(), filePrefix, ...parts);

  const filePath = req.originalUrl.split(baseURL).pop();

  await registerPartialsFromFile(baseURL, base("partials"), req);
  await registerPartialsFromFile(baseURL, base("pages", "home", "partials"), req);
  await registerPartialsFromFile(baseURL, base("pages", "api-landing", "partials"), req);
  await registerPartialsFromFile(baseURL, base("pages", "apis", "partials"), req);
  await registerPartialsFromFile(baseURL, base("pages", "docs", "partials"), req);
  await registerPartialsFromFile(baseURL, base("pages", "mcp", "partials"), req);
  await registerPartialsFromFile(baseURL, base("pages", "mcp-landing", "partials"), req);
  await registerPartialsFromFile(baseURL, base("pages", "api-workflows", "partials"), req);
  await registerPartialsFromFile(baseURL, base("pages", "api-workflows", "detail", "partials"), req);

  if (fs.existsSync(base("pages", filePath, "partials"))) {
    await registerPartialsFromFile(baseURL, base("pages", filePath, "partials"), req);
  }
}

const registerPartialsFromAPI = async (req, orgId) => {

  const viewName = req.params.viewName;

  let partials = await orgDao.getContent({
    orgId: orgId,
    fileType: 'partial',
    viewName: viewName
  });
  let partialObject = {};
  partials.forEach(file => {
    let fileName = file.file_name.split(".")[0];
    let content = file.file_content.toString(constants.CHARSET_UTF8);
    partialObject[fileName] = content;
  });
  Object.keys(partialObject).forEach((partialName) => {
    if (constants.CUSTOMIZABLE_FILES.includes(partialName)) {
      hbs.handlebars.registerPartial(partialName, partialObject[partialName]);
    }
  });
};

async function registerAPILandingContent(req, orgId, partialObject) {

  const apiHandle = req.params.apiHandle;
  const apiId = await apiDao.getId(orgId, apiHandle);
  if (apiId === undefined || apiId === null) {
    throw new Error("API not found");
  }
  //fetch markdown content for API if exists
  const markdownResponse = await apiFileDao.get(constants.FILE_NAME.API_MD_CONTENT_FILE_NAME, constants.DOC_TYPES.API_LANDING, orgId, apiId);
  const markdownContent = markdownResponse !== null ? markdownResponse.file_content.toString("utf8") : "";
  const markdownHtml = markdownContent ? markdown.parse(markdownContent) : "";

  let metaData = await apiMetadataService.getMetadataFromDB(orgId, apiId);
  if (metaData !== "") {
    const data = metaData ? JSON.stringify(metaData) : {};
    metaData = JSON.parse(data);
    //replace image urls
    let images = metaData.apiImageMetadata;
    for (const key in images) {
      let apiImageUrl = `${req.protocol}://${req.get('host')}${constants.DEVPORTAL_API.orgPath(orgId)}${constants.ROUTE.API_FILE_PATH}${apiId}${constants.API_TEMPLATE_FILE_NAME}`
      const modifiedApiImageURL = apiImageUrl + images[key]
      images[key] = modifiedApiImageURL;
    }
  }
  //if hbs content available for API, render the hbs page
  let additionalAPIContentResponse = await apiFileDao.get(constants.FILE_NAME.API_HBS_CONTENT_FILE_NAME, constants.DOC_TYPES.API_LANDING, orgId, apiId);
  if (additionalAPIContentResponse !== null) {
    let additionalAPIContent = additionalAPIContentResponse.file_content.toString("utf8");
    partialObject[constants.FILE_NAME.API_CONTENT_PARTIAL_NAME] = additionalAPIContent ? additionalAPIContent : "";
    hbs.handlebars.partials[constants.FILE_NAME.API_CONTENT_PARTIAL_NAME] = hbs.handlebars.compile(
      partialObject[constants.FILE_NAME.API_CONTENT_PARTIAL_NAME])({
        apiContent: markdownHtml,
        apiMetadata: metaData
      });
  }

}

async function registerDocsPageContent(req, orgId, partialObject) {

  const { orgName, apiHandle, viewName, docType, docName } = req.params;
  const apiId = await apiDao.getId(orgId, apiHandle);
  let markdownHtml = "";
  const docContentResponse = await apiFileDao.getDocByName(constants.DOC_TYPES.DOC_ID + docType, docName + ".md", orgId, apiId);
  if (docContentResponse !== null) {
    const markdownContent = docContentResponse.file_content.toString("utf8");
    markdownHtml = markdownContent ? markdown.parse(markdownContent) : "";
    partialObject[constants.FILE_NAME.API_DOC_PARTIAL_NAME] = hbs.handlebars.partials[constants.FILE_NAME.API_DOC_PARTIAL_NAME];
  }
  const apiMetadata = await apiDao.get(orgId, apiId);
  let apiType = apiMetadata[0].type;
  let baseUrl;

  if (apiType === constants.API_TYPE.MCP) {
    baseUrl = '/' + orgName + '/views/' + viewName + "/mcp/" + apiHandle;
  } else {
    baseUrl = '/' + orgName + '/views/' + viewName + "/api/" + apiHandle;
  }

  hbs.handlebars.partials[constants.FILE_NAME.API_DOC_PARTIAL_NAME] = hbs.handlebars.compile(
    partialObject[constants.FILE_NAME.API_DOC_PARTIAL_NAME])({
      baseUrl: baseUrl,
      apiMD: markdownHtml
    });
}

async function registerPartialsFromFile(baseURL, dir, req) {
  if (!dir || !fs.existsSync(dir)) return;
  const filenames = fs.readdirSync(dir);

  let orgId;
  if (req.params.orgName) {
    orgId = await orgDao.getId(req.params.orgName);
  }

  for (const filename of filenames) {
    if (filename.endsWith(".hbs")) {
      let name = filename.split(".hbs")[0];
      const template = fs.readFileSync(path.join(dir, filename), constants.CHARSET_UTF8);
      if (constants.CUSTOMIZABLE_FILES.includes(name) && orgId) {
        const content = await orgDao.getContent({ orgId: orgId, fileType: 'partial', viewName: req.params.viewName, fileName: name + '.hbs' });
        if (!(content)) {
          hbs.handlebars.registerPartial(name, template);
        }
      } else {
        hbs.handlebars.registerPartial(name, template);
      }
    }
  }
}

module.exports = registerPartials;

