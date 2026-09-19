/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package pdk

import (
	"errors"
	"fmt"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
)

// A plugin sees this refusal through whatever context it wrapped the call in, so it has to
// be recognisable through wrapping.
func TestIsBuildLimitReached_RecognisesTheRefusalThroughWrapping(t *testing.T) {
	err := apperror.BuildLimitReached.New(5)
	if !IsBuildLimitReached(err) {
		t.Fatal("the refusal itself should be recognised")
	}
	if !IsBuildLimitReached(fmt.Errorf("deploy to gateway %q: %w", "eu-gw", err)) {
		t.Error("the refusal should be recognised through a wrap")
	}
}

// The stand-in a plugin's own tests use is recognised the same way, including through a wrap.
func TestIsBuildLimitReached_RecognisesTheStandInForPluginTests(t *testing.T) {
	if !IsBuildLimitReached(ErrBuildLimitReached) {
		t.Fatal("the stand-in should be recognised")
	}
	if !IsBuildLimitReached(fmt.Errorf("deploy: %w", ErrBuildLimitReached)) {
		t.Error("the stand-in should be recognised through a wrap")
	}
}

func TestIsBuildLimitReached_IgnoresEverythingElse(t *testing.T) {
	for _, err := range []error{nil, errors.New("boom"), apperror.DeploymentNotFound.New()} {
		if IsBuildLimitReached(err) {
			t.Errorf("%v should not read as a build-limit refusal", err)
		}
	}
}
