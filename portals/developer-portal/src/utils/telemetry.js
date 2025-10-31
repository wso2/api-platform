const { trackEventWithDefaults } = require('./telemetryHelper');

/**
 * Send telemetry for login trigger
 */
function trackLoginTrigger({ orgName }) {
    trackEventWithDefaults({
        name: 'login',
        properties: {
            organization: orgName || 'unknown',
            timestamp: new Date().toISOString()
        }
    });
}

/**
 * Send telemetry for logout trigger
 */
function trackLogoutTrigger({ orgName }) {
    trackEventWithDefaults({
        name: 'logout',
        properties: {
            organization: orgName || 'unknown',
            timestamp: new Date().toISOString()
        }
    });
}

/**
 * Send telemetry for start to create an application
 */
function trackAppCreationStart({ orgId, appName, idpId }) {
    trackEventWithDefaults({
        name: 'application-create-start',
        properties: {
            orgId: orgId || 'unknown',
            appName: appName || 'unknown',
            idpId: idpId || 'unknown',
            timestamp: new Date().toISOString()
        }
    });
}

/**
 * Send telemetry for create an application
 */
function trackAppCreationEnd({ orgId, appName, idpId }) {
    trackEventWithDefaults({
        name: 'application-create-end',
        properties: {
            orgId: orgId || 'unknown',
            appName: appName || 'unknown',
            idpId: idpId || 'unknown',
            timestamp: new Date().toISOString()
        }
    });
}

/**
 * Send telemetry for delete an application
 */
function trackAppDeletion({ orgId, appId, idpId }) {
    trackEventWithDefaults({
        name: 'application-delete',
        properties: {
            orgId: orgId || 'unknown',
            appId: appId || 'unknown',
            idpId: idpId || 'unknown',
            timestamp: new Date().toISOString()
        }
    });
}

/**
 * Send telemetry for generate credentials
 */
function trackGenerateCredentials({ orgId, appName, idpId }) {
    trackEventWithDefaults({
        name: 'generate-credentials',
        properties: {
            orgId: orgId || 'unknown',
            appName: appName || 'unknown',
            idpId: idpId || 'unknown',
            timestamp: new Date().toISOString()
        }
    });
}

/**
 * Send telemetry for generate key
 */
function trackGenerateKey({ orgId, appId, idpId }) {
    trackEventWithDefaults({
        name: 'generate-key',
        properties: {
            orgId: orgId || 'unknown',
            appId: appId || 'unknown',
            idpId: idpId || 'unknown',
            timestamp: new Date().toISOString()
        }
    });
}

/**
 * Send telemetry for subscribe to an API
 */
function trackSubscribeApi({ orgId, appId, apiId, idpId }) {
    trackEventWithDefaults({
        name: 'subscribe-api',
        properties: {
            orgId: orgId || 'unknown',
            appId: appId || 'unknown',
            apiId: apiId || 'unknown',
            idpId: idpId || 'unknown',
            timestamp: new Date().toISOString()
        }
    });
}

/**
 * Send telemetry for unsubscribe to an API
 */
function trackUnsubscribeApi({ orgId, appId, apiRefId, idpId }) {
    trackEventWithDefaults({
        name: 'unsubscribe-api',
        properties: {
            orgId: orgId || 'unknown',
            appId: appId || 'unknown',
            apiRefId: apiRefId || 'unknown',
            idpId: idpId || 'unknown',
            timestamp: new Date().toISOString()
        }
    });
}

/**
 * Send telemetry for SDK generation start
 */
function trackSDKGenerationStart({ orgName, appId, idpId }) {
    trackEventWithDefaults({
        name: 'generate-sdk-start',
        properties: {
            orgName: orgName || 'unknown',
            appId: appId || 'unknown',
            idpId: idpId || 'unknown',
            timestamp: new Date().toISOString()
        }
    });
}

/**
 * Send telemetry for SDK generation end
 */
function trackSDKGenerationEnd() {
    trackEventWithDefaults({
        name: 'generate-sdk-end',
        properties: {
            timestamp: new Date().toISOString()
        }
    });
}

/**
 * Send telemetry for home page visit
 */
function trackHomePageVisit({ orgId, idpId }) {
    trackEventWithDefaults({
        name: 'home-page-visit',
        properties: {
            orgId: orgId || 'unknown',
            idpId: idpId || 'unknown',
            timestamp: new Date().toISOString()
        }
    });
}

module.exports = {
    trackLoginTrigger,
    trackLogoutTrigger,
    trackAppCreationStart,
    trackAppCreationEnd,
    trackAppDeletion,
    trackGenerateCredentials,
    trackGenerateKey,
    trackSubscribeApi,
    trackUnsubscribeApi,
    trackSDKGenerationStart,
    trackSDKGenerationEnd,
    trackHomePageVisit
};
