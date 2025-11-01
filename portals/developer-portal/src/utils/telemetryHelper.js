const { telemetryClient } = require('./telemetryClient');
const config = require(process.cwd() + '/config.json');

const DEFAULT_PROPERTIES = {
    context: 'devportal',
    origin: 'bijira',
};

function trackEventWithDefaults(event) {
    if (!config.telemetry) {
        return;
    }
    
    event.properties = {
        ...DEFAULT_PROPERTIES,
        ...(event.properties || {})
    };
    telemetryClient.trackEvent(event);
}

module.exports = {
    trackEventWithDefaults,
};
