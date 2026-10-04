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

	"github.com/wso2/api-platform/platform-api/internal/apperror"
)

// ErrBuildLimitReached stands in for the platform's own refusal so a plugin can exercise
// its handling of it. IsBuildLimitReached recognises this too, which lets a fake capability
// return it where the real one would refuse. The platform never returns it itself.
var ErrBuildLimitReached = errors.New("build limit reached")

// IsBuildLimitReached reports whether err is the platform refusing to prepare another
// build because the artifact already holds as many as it may, with every one of them on a
// gateway.
//
// A plugin that orchestrates deployments needs to tell this refusal apart from a genuine
// failure: it is a caller-fixable state with a choice behind it — free a slot by undeploying
// something, or delete a build that is no longer needed — and a plugin may be able to make
// that choice itself when it knows which build it is about to replace. The platform's own
// error types are internal, so this is how that one question is asked from outside.
func IsBuildLimitReached(err error) bool {
	return errors.Is(err, ErrBuildLimitReached) || apperror.BuildLimitReached.Is(err)
}
