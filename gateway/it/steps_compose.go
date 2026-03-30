package it

import (
	"fmt"

	"github.com/cucumber/godog"
)

func RegisterComposeSteps(ctx *godog.ScenarioContext, composeManager *ComposeManager) {
	ctx.Step(`^I restart the "([^"]*)" service$`, func(service string) error {
		if composeManager == nil {
			return fmt.Errorf("compose manager is not initialized")
		}
		return composeManager.RestartService(service)
	})
}
