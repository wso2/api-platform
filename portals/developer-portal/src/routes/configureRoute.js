const express = require('express');
const router = express.Router();
const settingsController = require('../controllers//settingsController');
const registerPartials = require('../middlewares/registerPartials');
const { ensureAuthenticated } = require('../middlewares/ensureAuthenticated');

//TODO: Comment organization configuration routes for SAAS scenrio
// router.get('/(((?!favicon.ico|images)):orgName/configure)', ensureAuthenticated, registerPartials, settingsController.loadSettingPage);

// router.get('/(((?!favicon.ico|images))portal)', registerPartials, ensureAuthenticated, settingsController.loadPortalPage);

// router.get('/(((?!favicon.ico|images)):orgName/configure/edit)', registerPartials, ensureAuthenticated, settingsController.loadEditOrganizationPage);

// router.get('/(((?!favicon.ico|images)):orgName/configure/create)', registerPartials, ensureAuthenticated, settingsController.loadCreateOrganizationPage);

module.exports = router;
