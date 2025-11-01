// process.env.APPLICATIONINSIGHTS_LOGGING_LEVEL = 'verbose';
const appInsights = require('applicationinsights');
const secret = require(process.cwd() + '/secret.json');
const config = require(process.cwd() + '/config.json');

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
