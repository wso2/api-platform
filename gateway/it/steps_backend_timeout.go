/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package it

import (
	"fmt"
	"strconv"
	"time"

	"github.com/cucumber/godog"
)

// elapsedTimeToleranceSeconds is the tolerance when asserting minimum elapsed time (e.g. clock skew, scheduling).
const elapsedTimeToleranceSeconds = 1

// RegisterBackendTimeoutSteps registers step definitions for backend timeout scenarios
func RegisterBackendTimeoutSteps(ctx *godog.ScenarioContext, state *TestState) {
	ctx.Step(`^I record the current time as "([^"]*)"$`, func(key string) error {
		state.SetContextValue(key, time.Now())
		return nil
	})

	ctx.Step(`^the request should have taken at least "(\d+)" seconds$`, func(expectedSecondsStr string) error {
		expectedSeconds, err := strconv.Atoi(expectedSecondsStr)
		if err != nil {
			return fmt.Errorf("expected seconds must be a number, got: %s", expectedSecondsStr)
		}
		val, ok := state.GetContextValue("request_start")
		if !ok {
			return fmt.Errorf("no start time recorded; record the current time as \"request_start\" before sending the request")
		}
		start, ok := val.(time.Time)
		if !ok {
			return fmt.Errorf("context value \"request_start\" is not a time.Time")
		}
		elapsed := time.Since(start)
		minElapsed := time.Duration(expectedSeconds-elapsedTimeToleranceSeconds) * time.Second
		if elapsed < minElapsed {
			return fmt.Errorf("request should have taken at least %d seconds (with %ds tolerance), but elapsed time was %s",
				expectedSeconds, elapsedTimeToleranceSeconds, elapsed.Round(time.Millisecond))
		}
		return nil
	})
}
