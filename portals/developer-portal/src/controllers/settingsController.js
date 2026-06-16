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
const adminDao = require('../dao/adminDao');
const IdentityProviderDTO = require("../dto/identityProviderDto");
const { config } = require('../config/configLoader');
const constants = require('../utils/constants');
const adminService = require('../services/adminService');
const apiMetadataService = require('../services/apiMetadataService');
const devPortalService = require('../services/devportalService');



const loadOrgSettingsPage = async (req, res) => {

    let orgID;
    const completeTemplatePath = path.join(require.main.filename, '..', 'pages', 'configure', 'page.hbs');
    const layoutPath = path.join(require.main.filename, '..', 'pages', 'layout', 'main.hbs');

    let templateContent = {
        baseUrl: req.params.orgName,
        orgContent: true,
    };
    let layoutResponse = "";
    let views;
    try {
        let orgName = req.params.orgName;
        templateContent.loggedOrg = orgName;
        orgID = await adminDao.getOrgId(orgName);
        templateContent.orgID = orgID;

        const organizations = await adminService.getAllOrganizations();
        if (organizations.length > 0) {
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
        const apiProviders = await getAPIProviders(orgID);
        if (apiProviders.length > 0) {
            templateContent.apiProviders = apiProviders;
        }

        templateContent.profile = req.user;
        layoutResponse = fs.readFileSync(layoutPath, constants.CHARSET_UTF8);
        const templateResponse = fs.readFileSync(completeTemplatePath, constants.CHARSET_UTF8);
        let html = await renderGivenTemplate(templateResponse, layoutResponse, templateContent);
        res.send(html);
    } catch (error) {
        logger.error(`Error while loading org settings page`, {
            error: error.message,
            stack: error.stack
        });
        res.status(500).send('Error loading configuration page');
    }
}


async function getAPIProviders(orgID) {

    const apiProviders = await adminService.getAllProviders(orgID);
    return apiProviders;
}


const loadPortalPage = async (req, res) => {

    let templateContent = {};
    try {
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


module.exports = {
    loadOrgSettingsPage,
    loadPortalPage,
    loadEditOrganizationPage,
    loadCreateOrganizationPage
};