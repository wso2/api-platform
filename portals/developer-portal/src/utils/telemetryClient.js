// process.env.APPLICATIONINSIGHTS_LOGGING_LEVEL = 'verbose';
const appInsights = require('applicationinsights');
const { config, secrets: secret } = require('../config/configLoader');

let telemetryClient = null;

if (config.telemetry && secret.azureInsightsConnectionString) {
    appInsights.setup(secret.azureInsightsConnectionString);
    telemetryClient = appInsights.defaultClient;
} else {
    // Provide a no-op client when telemetry is disabled
    telemetryClient = {
        trackEvent: () => {},
        trackException: () => {},
        trackTrace: () => {},
        trackMetric: () => {},
        trackRequest: () => {},
        trackDependency: () => {},
        flush: () => {}
    };
}

module.exports = {
    telemetryClient
};
