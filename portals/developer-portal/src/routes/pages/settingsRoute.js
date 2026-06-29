const express = require('express');
const router = express.Router();
const settingsController = require('../../controllers/settingsController');
const viewConfigureController = require('../../controllers/viewConfigureController');
const apiContentController = require('../../controllers/apiContentController');
const registerPartials = require('../../middlewares/registerPartials');
const { ensureAuthenticated } = require('../../middlewares/ensureAuthenticated');
const authController = require('../../controllers/authController');
const { requireCsrfForMutatingApi } = require('../../middlewares/csrfProtection');
const constants = require('../../utils/constants');

const noFavicon = (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') return res.status(404).send('Not Found');
    next();
};

const requireAdmin = (req, res, next) => {
    if (!req.user || !req.user.isAdmin) {
        const err = new Error('Access Denied');
        err.status = 403;
        return next(err);
    }
    next();
};

// Org-level settings: Organizations, Views, Labels, IDP
router.get('/:orgName/settings', noFavicon,
    authController.handleSilentSSO, registerPartials, ensureAuthenticated, requireAdmin, settingsController.loadOrgSettingsPage);

// View-level settings: LLM Instructions + API Workflows
router.get('/:orgName/views/:viewName/settings', noFavicon,
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
