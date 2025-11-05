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
const { renderGivenTemplate } = require('../utils/util');
const fs = require('fs');
const path = require('path');
const logger = require('../config/logger');
const { logUserAction } = require('../middlewares/auditLogger');
const adminDao = require('../dao/admin');
const IdentityProviderDTO = require("../dto/identityProvider");
const config = require(process.cwd() + '/config.json');
const constants = require('../utils/constants');
const adminService = require('../services/adminService');
const apiMetadataService = require('../services/apiMetadataService');
const devPortalService = require('../services/devportalService');

const filePrefix = config.pathToContent;


const loadSettingPage = async (req, res) => {

    let orgID;
    const completeTemplatePath = path.join(require.main.filename, '..', 'pages', 'configure', 'page.hbs');
    const layoutPath = path.join(require.main.filename, '..', 'pages', 'layout', 'main.hbs');

    let templateContent = {
        baseUrl: req.params.orgName+'/views/default',
        orgContent: true
    }
    let layoutResponse = "";
    let views;
    try {
        if (config.mode === constants.DEV_MODE) {
            const retrievedIDP = await getMockIdentityProvider();
            templateContent.idp = retrievedIDP;
            views = [{
                'name': 'Default'
            }]
            templateContent.createIDP = true
            templateContent.content = true;
            templateContent.orgContent = true;
            templateContent.views = views;
            templateContent.apiProviders = getMockAPIProviders();
        } else {
            let orgName = req.params.orgName;
            templateContent.loggedOrg = orgName;
            //retrieve orgID from the user object
            if (req.user) {
                orgID = req.user[constants.ORG_ID];
            } else {
                orgID = await adminDao.getOrgId(orgName);
            }
            templateContent.orgID = orgID;
            const organizations = await adminService.getAllOrganizations();
            let orgs = organizations.length;
            if (orgs !== 0) {
                templateContent.organizations = organizations;
            }
            const retrievedIDP = await adminDao.getIdentityProvider(orgID);
            if (retrievedIDP.length > 0) {
                templateContent.idp = new IdentityProviderDTO(retrievedIDP[0]);
            } else {
                templateContent.createIDP = true;
            }
            templateContent.viewCreate = true;
            const views = await apiMetadataService.getViewsFromDB(orgID);
            if (views.length > 0) {
                templateContent.content = true;
                templateContent.views = views;
                templateContent.viewCreate = false;
                templateContent.orgContent = false;
            }
            const orgLabels = await apiMetadataService.getOrgLabels(orgID);
            templateContent.orgLabels = orgLabels;
            //get api providers
            const apiProviders = await getAPIProviders(orgID);
            if (apiProviders.length > 0) {
                templateContent.apiProviders = apiProviders;
            }
        }
        layoutResponse = fs.readFileSync(layoutPath, constants.CHARSET_UTF8);
        const templateResponse = fs.readFileSync(completeTemplatePath, constants.CHARSET_UTF8);
        let html = await renderGivenTemplate(templateResponse, layoutResponse, templateContent);
        res.send(html);
    } catch (error) {
        logger.error(`Error while loading content from DB`, {
            error: error.message,
            stack: error.stack
        });
    }
}

async function getMockAPIProviders() {

    const mockAPIProvidersDataPath = path.join(process.cwd(), filePrefix + '../mock/APIProviders/APIProviders.json');
    const mockAPIProvidersData = JSON.parse(fs.readFileSync(mockAPIProvidersDataPath, 'utf-8'));
    return mockAPIProvidersData;
}

async function getAPIProviders(orgID) {

    const apiProviders = await adminService.getAllProviders(orgID);
    return apiProviders;
}

async function getMockIdentityProvider() {

    const mockIdentityProviderDataPath = path.join(process.cwd(), filePrefix + '../mock/IdentityProvider/identityProvider.json');
    const mockIDPaData = JSON.parse(fs.readFileSync(mockIdentityProviderDataPath, 'utf-8'));
    return mockIDPaData;
}

const loadPortalPage = async (req, res) => {

    let templateContent = {};
    try {
        if (config.mode === constants.DEV_MODE) {
            const organizations = await getMockOrganizations();
            templateContent.organizations = organizations;
        } else {
            templateContent = {
                'profile': req.user
            }
            //fetch all created organizations
            const organizations = await adminService.getAllOrganizations();
            let orgs = organizations.length;
            if (orgs !== 0) {
                templateContent.organizations = organizations;
                templateContent.create = true;
            }
        }
        const completeTemplatePath = path.join(require.main.filename, '..', 'pages', 'portal', 'page.hbs');
        const layoutPath = path.join(require.main.filename, '..', 'pages', 'layout', 'main.hbs');
        const templateResponse = fs.readFileSync(completeTemplatePath, constants.CHARSET_UTF8);
        const layoutResponse = fs.readFileSync(layoutPath, constants.CHARSET_UTF8);
        const html = await renderGivenTemplate(templateResponse, layoutResponse, templateContent);
        res.send(html);

    } catch (error) {
        logger.error(`Error while loading setting page`, {
            error: error.message,
            stack: error.stack
        });
    }
}

const loadEditOrganizationPage = async (req, res) => {

    let templateContent = {};
    let orgID = "";
    try {
        if (config.mode === constants.DEV_MODE) {
            const organizations = await getMockOrganizations();
            templateContent.organizations = organizations;
        } else {
            const orgName = req.params.orgName;
            if (req.params.orgId !== 'create') {
                orgID = await adminDao.getOrgId(orgName);

                //orgID = req.params.orgId;
                const organization = await devPortalService.getOrganizationDetails(orgID);
                templateContent = {
                    'orgID': orgID,
                    'profile': req.user,
                    'organization': organization,
                    'edit': true
                }
            }
        }
        const completeTemplatePath = path.join(require.main.filename, '..', 'pages', 'edit-organization', 'page.hbs');
        const layoutPath = path.join(require.main.filename, '..', 'pages', 'layout', 'main.hbs');
        const templateResponse = fs.readFileSync(completeTemplatePath, constants.CHARSET_UTF8);
        const layoutResponse = fs.readFileSync(layoutPath, constants.CHARSET_UTF8);
        const html = await renderGivenTemplate(templateResponse, layoutResponse, templateContent);
        res.send(html);
    } catch (error) {
        logger.error(`Error while loading edit organization setting page`, {
            orgName: req.params?.orgName,
            error: error.message,
            stack: error.stack
        });
    }
}

const loadCreateOrganizationPage = async (req, res) => {

    let templateContent = {};
    try {
        const completeTemplatePath = path.join(require.main.filename, '..', 'pages', 'add-organization', 'page.hbs');
        const layoutPath = path.join(require.main.filename, '..', 'pages', 'layout', 'main.hbs');
        const templateResponse = fs.readFileSync(completeTemplatePath, constants.CHARSET_UTF8);
        const layoutResponse = fs.readFileSync(layoutPath, constants.CHARSET_UTF8);
        const html = await renderGivenTemplate(templateResponse, layoutResponse, templateContent);
        res.send(html);
    } catch (error) {
        logger.error(`Error while loading create organization setting page`, {
            error: error.message,
            stack: error.stack
        });
    }
}

async function getMockOrganizations() {

    const mockOrganizationPath = path.join(process.cwd(), filePrefix + '../mock/Organization',
        '/organizations.json');
    const mockOrgData = JSON.parse(fs.readFileSync(mockOrganizationPath, 'utf-8'));
    return mockOrgData;
}

module.exports = {
    loadSettingPage,
    loadPortalPage,
    loadEditOrganizationPage,
    loadCreateOrganizationPage
};