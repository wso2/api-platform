/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package steps

import (
	"bytes"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

func TestFeatureStepsAreAllDefined(t *testing.T) {
	var out bytes.Buffer
	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) { Register(sc, nil, &httpx.Funnel{}) },
		Options:             &godog.Options{Format: "progress", Paths: []string{"../features"}, NoColors: true, Output: &out},
	}
	suite.Run()
	text := out.String()
	require.NotContains(t, text, "undefined", "a step line of a feature file has no definition:\n"+text)
	require.NotContains(t, text, "ambiguous", "a step line of a feature file matches two definitions:\n"+text)
}

// stepsWithoutFeature are defined and unit tested but no feature runs them yet.
var stepsWithoutFeature = map[string]bool{
	`^I rotate the synthetic secret$`:                               true,
	`^I read the synthetic secret$`:                                 true,
	`^I list the secrets$`:                                          true,
	`^the response should not reveal the synthetic secret's value$`: true,
	`^the synthetic secret should be described by a hash$`:          true,
	`^a "([^"]*)" request to "([^"]*)" should echo the header "([^"]*)" with the rotated secret value$`: true,
}

func TestEveryFeatureStepMatchesExactlyOneDefinition(t *testing.T) {
	definitions := StepDefinitions()
	patterns := make([]*regexp.Regexp, len(definitions))
	seen := map[string]bool{}
	for i, definition := range definitions {
		require.False(t, seen[definition.Pattern], "duplicate step pattern %s", definition.Pattern)
		seen[definition.Pattern] = true
		patterns[i] = regexp.MustCompile(definition.Pattern)
	}
	features, err := filepath.Glob(filepath.Join("..", "features", "*.feature"))
	require.NoError(t, err)
	require.NotEmpty(t, features)
	used := make([]bool, len(patterns))
	for _, feature := range features {
		for _, step := range cloudbase.FeatureSteps(t, feature) {
			matches := 0
			for i, pattern := range patterns {
				if pattern.MatchString(step) {
					matches++
					used[i] = true
				}
			}
			require.Equal(t, 1, matches, "%s: step %q matches %d definitions", filepath.Base(feature), step, matches)
		}
	}
	for i, definition := range definitions {
		require.True(t, used[i] || stepsWithoutFeature[definition.Pattern], "step %s is used by no feature", definition.Pattern)
	}
}
