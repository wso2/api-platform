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
	"net/http"
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

// A plugin gets the platform's refusal through its own wrapping and must see what the
// platform itself would have answered.
func TestAsClientError_ReportsThePlatformAnswerThroughWrapping(t *testing.T) {
	err := fmt.Errorf("read portal %q: %w", "acme", apperror.APIPortalNotFound.New())
	got, ok := AsClientError(err)
	if !ok {
		t.Fatal("a platform refusal should be recognised through a wrap")
	}
	want := ClientError{Status: http.StatusNotFound, Code: apperror.CodeAPIPortalNotFound, Message: "The specified API Portal could not be found."}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// Validation failures keep the message that names the field.
func TestAsClientError_KeepsTheValidationMessage(t *testing.T) {
	got, ok := AsClientError(apperror.ValidationFailed.New("The name field cannot be empty."))
	if !ok || got.Status != http.StatusBadRequest || got.Code != apperror.CodeCommonValidationFailed ||
		got.Message != "The name field cannot be empty." {
		t.Errorf("got %+v ok=%v", got, ok)
	}
}

// Only the client-facing message is reported, never the wrapped cause or internal log detail.
func TestAsClientError_LeavesOutInternalDetail(t *testing.T) {
	err := apperror.Internal.Wrap(errors.New("dial tcp 10.0.0.5:5432: connection refused")).
		WithLogMessage("db unreachable")
	got, ok := AsClientError(err)
	if !ok {
		t.Fatal("an internal platform error is still classified")
	}
	if got.Status != http.StatusInternalServerError || got.Message != "An unexpected error occurred." {
		t.Errorf("got %+v", got)
	}
}

// The stand-in a plugin's tests return from a fake is recognised the same way.
func TestAsClientError_RecognisesTheStandInForPluginTests(t *testing.T) {
	stand := &ClientError{Status: http.StatusConflict, Code: "X", Message: "taken"}
	got, ok := AsClientError(fmt.Errorf("create: %w", stand))
	if !ok || got != *stand {
		t.Errorf("got %+v ok=%v", got, ok)
	}
}

// A typed nil stored in an error must not panic when printed.
func TestClientError_NilReceiverDoesNotPanic(t *testing.T) {
	var stand *ClientError
	var err error = stand
	if err.Error() == "" {
		t.Error("want a stable message")
	}
	if _, ok := AsClientError(err); ok {
		t.Error("a nil *ClientError should not be classified")
	}
}

func TestAsClientError_IgnoresEverythingElse(t *testing.T) {
	for _, err := range []error{nil, errors.New("boom"), fmt.Errorf("wrap: %w", errors.New("x"))} {
		if got, ok := AsClientError(err); ok {
			t.Errorf("%v should not be classified, got %+v", err, got)
		}
	}
}
