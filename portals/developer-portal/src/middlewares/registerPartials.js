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
const config = require(process.cwd() + '/config.json');
const markdown = require('marked');
const adminDao = require('../dao/admin');
const apiDao = require('../dao/apiMetadata');
const constants = require('../utils/constants');
const apiMetadataService = require('../services/apiMetadataService');
const { loadLayoutFromAPI, renderTemplate } = require('../utils/util');
const util = require('../utils/util');
const { validationResult } = require('express-validator');
const logger = require('../config/logger');
const filePrefix = config.pathToContent;
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
  if (config.mode === constants.DEV_MODE) {
    registerAllPartialsFromFile(config.baseUrl + constants.ROUTE.VIEWS_PATH + req.params.viewName, req, filePrefix);
  } else {
    let matchURL = req.originalUrl;
    if (req.session.returnTo) {
      matchURL = req.session.returnTo;
    }
    let devportalMode = constants.API_TYPE.DEFAULT;

    try {
      const orgDetails = await adminDao.getOrganization(req.params.orgName);
      devportalMode = orgDetails.ORG_CONFIG?.devportalMode;
      
      if (req.params.orgName && req.params.orgName !== "portal" && (!(/configure/i.test(matchURL)))) {

        const orgID = await adminDao.getOrgId(req.params.orgName);
        await registerPartialsFromAPI(req);
        await registerAllPartialsFromFile(config.baseUrl + "/" + req.params.orgName + constants.ROUTE.VIEWS_PATH + req.params.viewName, req, './src/defaultContent');
        //register doc page partials
        if (req.originalUrl.includes(constants.ROUTE.API_DOCS_PATH) && req.params.docType && req.params.docName) {
          await registerDocsPageContent(req, orgID, {});
        } else if (req.originalUrl.includes(constants.ROUTE.API_LANDING_PAGE_PATH)) {
          await registerAPILandingContent(req, orgID, {});
        }

      }
    } catch (error) {
      logger.error('Error while loading organization', { 
        error: error.message, 
        stack: error.stack,
        orgName: req.params.orgName,
        operation: 'registerPartials'
      });
      if (error.message === "API not found") {
        let templateContent = {
          errorMessage: constants.ERROR_MESSAGE.API_NOT_FOUND,
          baseUrl: '/' + req.params.orgName + constants.ROUTE.VIEWS_PATH + req.params.viewName,
          devportalMode: devportalMode
        }
        html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
        return res.send(html);
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

  const filePath = req.originalUrl.split(baseURL).pop();

  await registerPartialsFromFile(baseURL, path.join(process.cwd(), filePrefix, "partials"), req);
  await registerPartialsFromFile(baseURL, path.join(process.cwd(), filePrefix, "pages", "home", "partials"), req);
  await registerPartialsFromFile(baseURL, path.join(process.cwd(), filePrefix, "pages", "api-landing", "partials"), req);
  await registerPartialsFromFile(baseURL, path.join(process.cwd(), filePrefix, "pages", "apis", "partials"), req);
  await registerPartialsFromFile(baseURL, path.join(process.cwd(), filePrefix, "pages", "docs", "partials"), req);
  await registerPartialsFromFile(baseURL, path.join(process.cwd(), filePrefix, "pages", "mcp", "partials"), req);
  await registerPartialsFromFile(baseURL, path.join(process.cwd(), filePrefix, "pages", "mcp-landing", "partials"), req);

  if (fs.existsSync(path.join(process.cwd(), filePrefix + "pages", filePath, "partials"))) {
    await registerPartialsFromFile(baseURL, path.join(process.cwd(), filePrefix + "pages", filePath, "partials"), req);
  }
}

const registerPartialsFromAPI = async (req) => {

  const orgName = req.params.orgName;
  const viewName = req.params.viewName;
  const orgID = await adminDao.getOrgId(orgName);

  let partials = await adminDao.getOrgContent({
    orgId: orgID,
    fileType: 'partial',
    viewName: viewName
  });
  let partialObject = {};
  partials.forEach(file => {
    let fileName = file.FILE_NAME.split(".")[0];
    let content = file.FILE_CONTENT.toString(constants.CHARSET_UTF8);
    partialObject[fileName] = content;
  });
  const hbs = exphbs.create({});
  Object.keys(partialObject).forEach((partialName) => {
    if (constants.CUSTOMIZABLE_FILES.includes(partialName)) {
      hbs.handlebars.registerPartial(partialName, partialObject[partialName]);
    }
  });
};

async function registerAPILandingContent(req, orgID, partialObject) {

  const apiHandle = req.params.apiHandle;
  const apiID = await apiDao.getAPIId(orgID, apiHandle);
  if (apiID === undefined || apiID === null) {
    throw new Error("API not found");
  }
  //fetch markdown content for API if exists
  const markdownResponse = await apiDao.getAPIFile(constants.FILE_NAME.API_MD_CONTENT_FILE_NAME, constants.DOC_TYPES.API_LANDING, orgID, apiID);
  const markdownContent = markdownResponse !== null ? markdownResponse.API_FILE.toString("utf8") : "";
  const markdownHtml = markdownContent ? markdown.parse(markdownContent) : "";

  metaData = await apiMetadataService.getMetadataFromDB(orgID, apiID);
  if (metaData !== "") {
    const data = metaData ? JSON.stringify(metaData) : {};
    metaData = JSON.parse(data);
    //replace image urls
    let images = metaData.apiInfo.apiImageMetadata;
    for (const key in images) {
      let apiImageUrl = `${req.protocol}://${req.get('host')}${constants.ROUTE.DEVPORTAL_ASSETS_BASE_PATH}${orgID}${constants.ROUTE.API_FILE_PATH}${apiID}${constants.API_TEMPLATE_FILE_NAME}`
      const modifiedApiImageURL = apiImageUrl + images[key]
      images[key] = modifiedApiImageURL;
    }
  }
  //if hbs content available for API, render the hbs page
  let additionalAPIContentResponse = await apiDao.getAPIFile(constants.FILE_NAME.API_HBS_CONTENT_FILE_NAME, constants.DOC_TYPES.API_LANDING, orgID, apiID);
  if (additionalAPIContentResponse !== null) {
    let additionalAPIContent = additionalAPIContentResponse.API_FILE.toString("utf8");
    partialObject[constants.FILE_NAME.API_CONTENT_PARTIAL_NAME] = additionalAPIContent ? additionalAPIContent : "";
    hbs.handlebars.partials[constants.FILE_NAME.API_CONTENT_PARTIAL_NAME] = hbs.handlebars.compile(
      partialObject[constants.FILE_NAME.API_CONTENT_PARTIAL_NAME])({
        apiContent: markdownHtml,
        apiMetadata: metaData
      });
  }

}

async function registerDocsPageContent(req, orgID, partialObject) {

  const { orgName, apiHandle, viewName, docType, docName } = req.params;
  const apiID = await apiDao.getAPIId(orgID, apiHandle);
  let markdownHtml = "";
  let docContentResponse = await apiDao.getAPIDocByName(constants.DOC_TYPES.DOC_ID + docType, docName, orgID, apiID);
  if (docContentResponse !== null) {
    if (docName.endsWith(".md")) {
      const markdownContent = docContentResponse.API_FILE.toString("utf8");
      markdownHtml = markdownContent ? markdown.parse(markdownContent) : "";
      partialObject[constants.FILE_NAME.API_DOC_PARTIAL_NAME] = hbs.handlebars.partials[constants.FILE_NAME.API_DOC_PARTIAL_NAME];
    } else {
      let additionalDocContent = docContentResponse.API_FILE.toString("utf8");
      partialObject[constants.FILE_NAME.API_DOC_PARTIAL_NAME] = additionalDocContent ? additionalDocContent : "";
    }
  }
  const apiMetadata = await apiDao.getAPIMetadata(orgID, apiID);
  let apiType = apiMetadata[0].dataValues.API_TYPE;
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
  const filenames = fs.readdirSync(dir);

  for (const filename of filenames) {
    if (filename.endsWith(".hbs")) {
      let name = filename.split(".hbs")[0];
      const template = fs.readFileSync(path.join(dir, filename), constants.CHARSET_UTF8);
      if (constants.CUSTOMIZABLE_FILES.includes(name)) {
        const orgID = await adminDao.getOrgId(req.params.orgName);
        const content = await adminDao.getOrgContent({ orgId: orgID, fileType: 'partial', viewName: req.params.viewName, fileName: name + '.hbs' });
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

