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
 
const express = require('express');
const { engine } = require('express-handlebars');
const passport = require('passport');
const session = require('express-session');
const path = require('path');
const logger = require('./config/logger');
const { auditMiddleware } = require('./middlewares/auditLogger');
const authRoute = require('./routes/pages/authRoute');
const orgContent = require('./routes/pages/orgContentRoute');
const apiContent = require('./routes/pages/apiContentRoute');
const applicationContent = require('./routes/pages/applicationsContentRoute');
const customContent = require('./routes/pages/customPageRoute');
const subscriptionsContent = require('./routes/pages/subscriptionsContentRoute');
const apiKeysOverviewContent = require('./routes/pages/apiKeysOverviewRoute');
const mcpRegistryRoute = require('./routes/pages/mcpRegistryRoute');
const { config } = require('./config/configLoader');
const Handlebars = require('handlebars');
const constants = require("./utils/constants");
const designRoute = require('./routes/pages/designModeRoute');
const settingsRoute = require('./routes/pages/settingsRoute');
const apiWorkflowsRoute = require('./routes/pages/apiWorkflowsRoute');
const crypto = require('crypto');
const util = require('./utils/util');
const orgContext = require('./utils/orgContext');
const sessionStore = require('./db/sessionStoreConfig');
const { registerHelpers } = require('./helpers/handlebarsHelpers');
const { configurePassport } = require('./middlewares/passportConfig');

const app = express();
// Do not advertise Express in response headers.
app.disable('x-powered-by');
const sessionSecret = config.security.sessionSecret;

const SERVER_ID = crypto.randomUUID();

app.engine('.hbs', engine({
    extname: '.hbs'
}));

app.set('view engine', 'hbs');

registerHelpers();

app.use(session({
    store: sessionStore,
    secret: sessionSecret,
    resave: false,
    saveUninitialized: true,
    cookie: {
        secure: config.server.https.enabled && !config.designMode?.enabled,
        maxAge: 60 * 60 * 1000,
        // Scoped to the mount prefix: the whole portal (and every route that reads the
        // session) lives under BASE_PATH, so a root-scoped cookie would be sent to paths
        // this service never serves and — more importantly — must match the paths the
        // browser actually requests. Mismatched cookie path is a silent login loop.
        path: constants.ROUTE.BASE_PATH,
    },
}));

app.get('/health', (req, res) => {
    res.status(200).json({ status: 'ok' });
});

app.get('/robots.txt', (req, res) => {
    res.type('text/plain').send(
        'User-agent: *\nAllow: /\n\n' +
        `# AI agent guidance: ${constants.ROUTE.BASE_PATH}/${orgContext.getHandle()}/views/{viewName}/llms.txt\n`
    );
});

app.get('/llms.txt', (req, res) => {
    const baseUrl = `${req.protocol}://${req.get('host')}`;
    // This portal serves exactly one organization, so name it outright rather than
    // asking the agent to supply a handle it would have to guess.
    const orgHandle = orgContext.getHandle();
    res.type('text/plain').send(
        `# API Portal & MCP Hub — AI Agent Entry Point\n\n` +
        `This portal serves a single organization, \`${orgHandle}\`, whose APIs, MCP servers,\n` +
        `and API workflows are organized into views.\n` +
        `The portal host is the origin you fetched this file from: ${baseUrl}\n\n` +
        `## Exploring APIs\n\n` +
        `To discover APIs, MCP servers, and API workflows published in a view, fetch the\n` +
        `view-specific index:\n\n` +
        `  ${baseUrl}${constants.ROUTE.BASE_PATH}/${orgHandle}/views/{viewName}/llms.txt\n\n` +
        `If the user has provided a URL that contains the view name, extract it from there.\n` +
        `If the view name is not specified, use \`default\`.\n`
    );
});

// Bound JSON/urlencoded body size (config-sourced).
const bodyLimit = config.uploads?.maxBytes || 10485760;

// The try-it proxy relays its request body verbatim, so neither parser may run
// for it — either one would consume the stream and leave only a re-serialised
// approximation of what the caller actually sent. Skipping them here (rather
// than mounting the proxy ahead of them) lets the proxy be registered after
// passport, so it can apply the same authentication gate as the page it serves.
// The pattern is owned by the route module so it tracks that route's mount path.
const tryoutProxy = require('./routes/pages/tryoutProxyRoute');
const skipForTryoutProxy = (parser) => (req, res, next) =>
    (tryoutProxy.BODY_PARSER_SKIP_PATTERN.test(req.path) ? next() : parser(req, res, next));

app.use(skipForTryoutProxy(express.json({ limit: bodyLimit })));
app.use(skipForTryoutProxy(express.urlencoded({ extended: true, limit: bodyLimit })));

// Add audit logging middleware. Runs at the app level (before the BASE_PATH parent
// router strips the prefix), so the static-asset exclude paths carry BASE_PATH to match
// the real request paths; /health and /metrics stay at the true root.
const BP = constants.ROUTE.BASE_PATH;
app.use(auditMiddleware({
    excludePaths: ['/health', '/metrics', '/favicon.ico',
        `${BP}/styles`, `${BP}/scripts`, `${BP}/images`, `${BP}/technical-styles`, `${BP}/technical-scripts`],
    sensitiveFields: ['password', 'token', 'secret', 'key', 'authorization', 'idToken', 'accessToken', 'refreshToken']
}));

app.use(passport.initialize());
app.use(passport.session());

// Expose the per-session CSRF token as a browser-readable cookie (double-submit
// pattern). Mutating fetches echo it back as X-CSRF-Token; the value matches
// what requireCsrfForMutatingApi expects (getSessionCsrfToken).
const { getSessionCsrfToken } = require('./middlewares/csrfProtection');
app.use((req, res, next) => {
    if (req.session) {
        res.cookie('XSRF-TOKEN', getSessionCsrfToken(req), {
            sameSite: 'lax',
            secure: config.server.https.enabled && !config.designMode?.enabled,
            // Same scope as the session cookie above — the double-submit token is only
            // ever echoed back from pages served under BASE_PATH.
            path: constants.ROUTE.BASE_PATH,
        });
    }
    next();
});


configurePassport(SERVER_ID);

// Everything below is mounted on a single parent router at BASE_PATH: a reverse proxy
// forwards only `${BASE_PATH}/*` here, so the portal mounts *itself* under the prefix.
// Inside `portal`, req.path/req.url are already stripped of BASE_PATH (Express strips a
// mount path), so the sub-routers below keep their bare paths ('/', '/:orgName/...') and
// need no knowledge of the prefix. Only the app-level middleware above (session, body
// parsers, passport, XSRF) — plus /health, /robots.txt, /llms.txt — stay at the true
// root; /health in particular is probed unprefixed by the Docker/compose healthchecks.
const portal = express.Router();

// API try-it proxy (Stoplight Elements `tryItCorsProxy`). Mounted first, after
// passport.initialize()/passport.session() (app-level), so req.user and
// req.isAuthenticated() are populated and the route can apply the same authentication
// gate the specification page it serves goes through. Its raw body survives because the
// parsers above skip this path (BODY_PARSER_SKIP_PATTERN includes BASE_PATH).
portal.use(constants.ROUTE.DEFAULT, tryoutProxy.router);

portal.use(constants.ROUTE.TECHNICAL_STYLES, express.static(path.join(require.main.filename, '../styles')));
portal.use(constants.ROUTE.TECHNICAL_SCRIPTS, express.static(path.join(require.main.filename, '../scripts')));

// Dev live-reload SSE endpoint — must be registered before org-resolution routes
if (process.env.NODE_ENV === 'development') {
    require('./liveReload').setup(portal);
}

// Redirect unrecognised prefix-level paths (e.g. ${BASE_PATH}/sitemap.xml) before
// the /:orgName route can treat them as org IDs. Redirects to the portal front door.
portal.use((req, res, next) => {
    const segments = req.path.split('/').filter(Boolean);
    if (segments.length === 1 && segments[0].includes('.')) {
        return res.redirect(constants.ROUTE.BASE_PATH + '/');
    }
    next();
});

//backend routes
// Spec-driven portal router (express-openapi-validator): request validation +
// fine-grained OAuth2 scope enforcement, dispatching by operationId to
// src/routes/api/handlers (/api/v0.9/..., /organizations, /login, ...).
// Registered before the page route tree so unmatched requests fall through to it.
const apiPortalRouter = require('./routes/api/apiPortalRouter');
portal.use(constants.ROUTE.DEFAULT, apiPortalRouter);

// MCP Server Registry (OpenAPI v0.1)
portal.use('/registry/:orgHandle', mcpRegistryRoute);
portal.use('/:orgHandle/registry', mcpRegistryRoute);

if (config.designMode?.enabled) {
    const sampleApiLoader = require('./utils/sampleApiLoader');
    const layoutPath = config.designMode.pathToLayout;
    // Serve styles/images from pathToLayout first, fall back to src/defaultContent/
    portal.use(constants.ROUTE.STYLES, express.static(path.resolve(process.cwd(), layoutPath, 'styles')));
    portal.use(constants.ROUTE.STYLES, express.static(path.join(process.cwd(), './src/defaultContent/styles')));
    portal.use(constants.ROUTE.IMAGES, express.static(path.resolve(process.cwd(), layoutPath, 'images')));
    portal.use(constants.ROUTE.IMAGES, express.static(path.join(process.cwd(), './src/defaultContent/images')));
    portal.use(constants.ROUTE.MOCK, express.static(path.join(process.cwd(), config.designMode.apiSamplesPath)));
    // Serve API definition files by resolving the handle to the actual directory
    portal.get('/mock/:apiHandle/definition.yml', (req, res) => {
        const content = sampleApiLoader.getDefinition(req.params.apiHandle, config.designMode.apiSamplesPath);
        if (!content) return res.status(404).send('Not found');
        res.type('text/yaml').send(content);
    });
    portal.use(constants.ROUTE.DEFAULT, designRoute);
} else {
    portal.use(constants.ROUTE.STYLES, express.static(path.join(process.cwd(), './src/defaultContent/' + 'styles')));
    portal.use(constants.ROUTE.IMAGES, express.static(path.join(process.cwd(), './src/defaultContent/' + 'images')));
    portal.use(constants.ROUTE.DEFAULT, authRoute);
    portal.use(constants.ROUTE.DEFAULT, apiContent);
    portal.use(constants.ROUTE.DEFAULT, applicationContent);
    portal.use(constants.ROUTE.DEFAULT, orgContent);
    portal.use(constants.ROUTE.DEFAULT, settingsRoute);
    portal.use(constants.ROUTE.DEFAULT, apiWorkflowsRoute);
    portal.use(constants.ROUTE.DEFAULT, subscriptionsContent);
    portal.use(constants.ROUTE.DEFAULT, apiKeysOverviewContent);
    portal.use(constants.ROUTE.DEFAULT, customContent);
}

app.use(constants.ROUTE.BASE_PATH, portal);


// 404 catch-all — must come after all page routes
app.use((req, res, next) => {
    const err = new Error('Not Found');
    err.status = 404;
    next(err);
});

// Central error handler
app.use(async (err, req, res, next) => {
    if (res.headersSent) return;
    const status = err.status || 500;

    if (status >= 500) {
        logger.error('Application error', {
            error: err.message,
            stack: err.stack,
            url: req.url,
            method: req.method,
            operation: 'expressErrorHandler'
        });
    }

    // Destroy session on auth errors
    if (status === 401 && req.session) {
        req.session.destroy(() => {});
    }

    // Ensure chrome partials exist — registered by registerPartials for normal requests,
    // but may be absent for early-pipeline errors (unmatched route, startup crash).
    ['header', 'sidebar', 'footer', 'delete-confirmation'].forEach(name => {
        if (!Handlebars.partials[name]) Handlebars.registerPartial(name, '');
    });

    const errorType = status === 404 ? '404' : status === 403 ? '403' : '500';
    // Always this instance's own organization — never the first path segment of the
    // failed request. A 404 from orgGuard means that segment named some *other*
    // organization, and echoing it back would point the error page's "home" link
    // outside this portal.
    // Resolved rather than hardcoded to 'default', so the "home" link still points at a
    // view that exists after that one has been renamed or deleted. Never throws — see
    // orgContext.getFallbackViewHandle — which matters on this path above all others.
    const baseUrl = constants.ROUTE.BASE_PATH + '/' + orgContext.getHandle() + constants.ROUTE.VIEWS_PATH + await orgContext.getFallbackViewHandle();
    const templateContent = {
        baseUrl,
        errorType,
        profile: typeof req.isAuthenticated === 'function' && req.isAuthenticated() ? req.user : null,
    };

    const html = util.renderTemplate('../pages/error-page/page.hbs', './src/defaultContent/layout/main.hbs', templateContent, true);
    res.status(status).send(html);
});


module.exports = app;

