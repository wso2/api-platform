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
const express = require('express');
const { engine } = require('express-handlebars');
const passport = require('passport');
const session = require('express-session');
const pgSession = require('connect-pg-simple')(session);
const path = require('path');
const logger = require('./config/logger');
const { auditMiddleware } = require('./middlewares/auditLogger');
const authRoute = require('./routes/authRoute');
const devportalRoute = require('./routes/devportalRoute');
const orgContent = require('./routes/orgContentRoute');
const apiContent = require('./routes/apiContentRoute');
const applicationContent = require('./routes/applicationsContentRoute');
const customContent = require('./routes/customPageRoute');
const subscriptionsContent = require('./routes/subscriptionsContentRoute');
const mcpRegistryRoute = require('./routes/mcpRegistryRoute');
const { config } = require('./config/configLoader');
const Handlebars = require('handlebars');
const constants = require("./utils/constants");
const designRoute = require('./routes/designModeRoute');
const settingsRoute = require('./routes/configureRoute');
const apiFlowsRoute = require('./routes/apiFlowsRoute');
const { v4: uuidv4 } = require('uuid');
const util = require('./utils/util');
const pool = require('./db/pool');
const { registerHelpers } = require('./helpers/handlebarsHelpers');
const { configurePassport } = require('./middlewares/passport');

const app = express();
// const secret = crypto.randomBytes(64).toString('hex');
const sessionSecret = 'my-secret';
const filePrefix = config.pathToContent;

const SERVER_ID = uuidv4();

logger.info(`Starting server with ID: ${SERVER_ID}`);

app.engine('.hbs', engine({
    extname: '.hbs'
}));

app.set('view engine', 'hbs');

registerHelpers();

app.use(session({
    store: new pgSession({
        pool: pool,
        tableName: 'session',
        pruneSessionInterval: 3600,
        debug: (message) => logger.debug('Session store debug', { message }),
    }),
    secret: sessionSecret,
    resave: false,
    saveUninitialized: true,
    cookie: {
        secure: !config.advanced.http,
        maxAge: 60 * 60 * 1000,
    },
}));

// Stripe webhook endpoint MUST use raw body parser for signature verification
const billingController = require('./controllers/billingController');
app.post('/webhooks/stripe/:orgId', express.raw({ type: 'application/json' }), billingController.handleStripeWebhook);

app.get('/robots.txt', (req, res) => {
    res.type('text/plain').send(
        'User-agent: *\nAllow: /\n\n# AI agent guidance: /{orgName}/views/{viewName}/llms.txt\n'
    );
});

app.get('/llms.txt', (req, res) => {
    const baseUrl = config.baseUrl;
    res.type('text/plain').send(
        `# API Developer Portal — AI Agent Entry Point\n\n` +
        `This portal provides APIs, MCP servers, and API workflows organized by organization and view.\n` +
        `The portal host is the origin you fetched this file from: ${baseUrl}\n\n` +
        `## Exploring APIs\n\n` +
        `To discover APIs, MCP servers, and API workflows published by an organization, fetch the org/view-specific index:\n\n` +
        `  ${baseUrl}/{orgName}/views/{viewName}/llms.txt\n\n` +
        `If the user has provided a URL that contains the organization name and view name, extract them from it.\n\n` +
        `If the organization name is not known, ask the user to provide it — do not guess.\n` +
        `If the view name is not specified, use \`default\`.\n`
    );
});

app.use(express.json());
app.use(express.urlencoded({ extended: true }));

// Add audit logging middleware
app.use(auditMiddleware({
    excludePaths: ['/health', '/metrics', '/favicon.ico', '/styles', '/scripts', '/images', '/technical-styles', '/technical-scripts'],
    sensitiveFields: ['password', 'token', 'secret', 'key', 'authorization', 'idToken', 'accessToken', 'refreshToken']
}));

app.use(passport.initialize());
app.use(passport.session());


configurePassport(SERVER_ID);

app.use(constants.ROUTE.TECHNICAL_STYLES, express.static(path.join(require.main.filename, '../styles')));
app.use(constants.ROUTE.TECHNICAL_SCRIPTS, express.static(path.join(require.main.filename, '../scripts')));

// Redirect unrecognised root-level paths (e.g. /robots.txt, /sitemap.xml) before
// the /:orgName route can treat them as org IDs.
app.use((req, res, next) => {
    const segments = req.path.split('/').filter(Boolean);
    if (segments.length === 1 && segments[0].includes('.')) {
        return res.redirect('/');
    }
    next();
});

//backend routes
if (config.advanced?.openApiValidator?.enabled) {
    logger.info('Mounting spec-driven /devportal router (advanced.useOpenApiValidator=true)');
    const devportalApiRouter = require('./openapi/devportalApiRouter');
    app.use(constants.ROUTE.DEV_PORTAL, devportalApiRouter);
} else {
    app.use(constants.ROUTE.DEV_PORTAL, devportalRoute);
}

// MCP Server Registry (OpenAPI v0.1)
app.use('/registry/:orgHandle', mcpRegistryRoute);
app.use('/:orgHandle/registry', mcpRegistryRoute);

if (config.mode === constants.DEV_MODE) {
    app.use(constants.ROUTE.STYLES, express.static(path.join(process.cwd(), filePrefix + 'styles')));
    app.use(constants.ROUTE.IMAGES, express.static(path.join(process.cwd(), filePrefix + 'images')));
    app.use(constants.ROUTE.MOCK, express.static(path.join(process.cwd(), filePrefix + 'mock')));
    app.use(constants.ROUTE.DEFAULT, designRoute);
} else {
    app.use(constants.ROUTE.STYLES, express.static(path.join(process.cwd(), './src/defaultContent/' + 'styles')));
    app.use(constants.ROUTE.IMAGES, express.static(path.join(process.cwd(), './src/defaultContent/' + 'images')));
    app.use(constants.ROUTE.DEFAULT, authRoute);
    app.use(constants.ROUTE.DEFAULT, apiContent);
    app.use(constants.ROUTE.DEFAULT, applicationContent);
    app.use(constants.ROUTE.DEFAULT, orgContent);
    app.use(constants.ROUTE.DEFAULT, settingsRoute);
    app.use(constants.ROUTE.DEFAULT, apiFlowsRoute);
    app.use(constants.ROUTE.DEFAULT, subscriptionsContent);
    app.use(constants.ROUTE.DEFAULT, customContent);
}


app.use((req, res) => {
    res.redirect('/');
});

app.use( (err, req, res, next) => {
    Handlebars.registerPartial('header', '');
    Handlebars.registerPartial('sidebar', '');
    logger.error('Application error', { 
        error: err.message, 
        stack: err.stack,
        url: req.url,
        method: req.method,
        operation: 'expressErrorHandler'
    });
    let templateContent = {
        devportalMode: 'DEFAULT',
        baseUrl: '/' + req.originalUrl?.split('/')[1] + '/' + constants.ROUTE.VIEWS_PATH + "default",
        errorMessage: "Oops! Something went wrong",
        profile: typeof req.isAuthenticated === 'function' && req.isAuthenticated() ? req.user : null,
    }
    let html = "";
    if (err.status === 401) {
        req.session.destroy((err) => {
            if (err) {
                return res.status(500).send("Logout failed");
            }
        });
        templateContent.errorMessage = constants.ERROR_MESSAGE.COMMON_AUTH_ERROR_MESSAGE;
        html = util.renderTemplate('../pages/error-page/page.hbs', 'src/pages/error-layout/main.hbs', templateContent, true);
    } else {
        html = util.renderTemplate('../pages/error-page/page.hbs', 'src/pages/error-layout/main.hbs', templateContent, true);
    }
    res.status(err.status || 500).send(`
      ${html}
    `);
});


module.exports = app;

