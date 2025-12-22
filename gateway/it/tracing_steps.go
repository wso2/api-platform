package it

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
)

// RegisterTracingSteps registers the tracing-related steps
func RegisterTracingSteps(ctx *godog.ScenarioContext, cm *ComposeManager) {
	ctx.Step(`^I should see a trace for "([^"]*)" in the OpenTelemetry collector logs$`, func(serviceName string) error {
		return verifyTraceInLogs(cm, serviceName)
	})

	ctx.Step(`^the Gateway is running with tracing enabled$`, func() error {
		// This is just a readability step, the @config-tracing tag handles the setup.
		// We could assert here that the config is correct if we want to be strict.
		return nil
	})
}

func verifyTraceInLogs(cm *ComposeManager, text string) error {
	// Retry for a few seconds as logs might be delayed
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for trace logs containing '%s'", text)
		case <-ticker.C:
			found, err := cm.CheckLogsForText(ctx, "it-otel-collector", text)
			if err != nil {
				// Don't fail immediately on log retrieval error (container might be starting?)
				continue
			}
			if found {
				return nil
			}
		}
	}
}
