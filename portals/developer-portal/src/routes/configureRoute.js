const express = require('express');
const router = express.Router();
const settingsController = require('../controllers/settingsController');
const viewConfigureController = require('../controllers/viewConfigureController');
const apiContentController = require('../controllers/apiContentController');
const registerPartials = require('../middlewares/registerPartials');
const { ensureAuthenticated } = require('../middlewares/ensureAuthenticated');
const authController = require('../controllers/authController');
const { requireCsrfForMutatingApi } = require('../middlewares/csrfProtection');
const util = require('../utils/util');
const constants = require('../utils/constants');

const noFavicon = (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') return res.status(404).send('Not Found');
    next();
};

const requireAdmin = (req, res, next) => {
    if (!req.user || !req.user.isAdmin) {
        const viewName = req.params.viewName || 'default';
        const baseUrl = req.params.viewName
            ? '/' + req.params.orgName + '/views/' + viewName
            : '/' + req.params.orgName;
        const templateContent = {
            errorMessage: "Access Denied",
            baseUrl,
            devportalMode: constants.API_TYPE.DEFAULT,
            profile: req.user || null,
            showApiWorkflowsNav: false,
        };
        const html = util.renderTemplate('../pages/error-page/page.hbs', './src/defaultContent/layout/main.hbs', templateContent, true);
        return res.status(403).send(html);
    }
    next();
};

// Org-level settings: Organizations, Views, Labels, IDP, API Provider
router.get('/:orgName/configure', noFavicon,
    authController.handleSilentSSO, registerPartials, ensureAuthenticated, requireAdmin, settingsController.loadOrgSettingsPage);

// View-level settings: LLM Instructions + API Workflows
router.get('/:orgName/views/:viewName/configure', noFavicon,
    authController.handleSilentSSO, registerPartials, ensureAuthenticated, requireAdmin, viewConfigureController.loadViewSettingsPage);

// LLM config CRUD
router.get('/:orgName/views/:viewName/llms-config', noFavicon,
    authController.handleSilentSSO, ensureAuthenticated, viewConfigureController.getLlmsConfig);

router.put('/:orgName/views/:viewName/llms-config', noFavicon,
    ensureAuthenticated, requireCsrfForMutatingApi, viewConfigureController.saveLlmsConfig);

// llms.txt preview (uses real API data + submitted overrides)
router.post('/:orgName/views/:viewName/llms.txt/preview', noFavicon,
    ensureAuthenticated, requireCsrfForMutatingApi, apiContentController.previewLlmsTxt);

module.exports = router;
