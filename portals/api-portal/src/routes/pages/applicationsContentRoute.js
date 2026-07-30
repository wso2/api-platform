const express = require('express');
const router = express.Router();
const applicationsController = require('../../controllers/applicationsContentController');
const registerPartials = require('../../middlewares/registerPartials');
const { ensureAuthenticated } = require('../../middlewares/ensureAuthenticated');
const { attachOrgGuard } = require('../../middlewares/orgGuard');

// Pin every ':orgName' in this router to the organization this instance serves;
// anything else is a 404 before the route's own handlers run.
attachOrgGuard(router);

router.get('/:orgName/views/:viewName/applications', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, registerPartials, ensureAuthenticated, applicationsController.loadApplications);



router.get('/:orgName/views/:viewName/applications/:applicationId', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, registerPartials, ensureAuthenticated, applicationsController.loadApplication);




module.exports = router;
