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

package handler

import (
	"errors"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
)

// serviceError adapts an error returned by the service layer for the error
// mapper (middleware.MapErrors).
//
// Services construct their failures from the catalog (apperror.Def.New /
// Def.Wrap), so a catalog error already carries the code, HTTP status, and
// client-facing message the mapper needs — it passes straight through, and the
// handler neither restates the message nor re-derives the status. Handlers used
// to translate service-layer sentinel errors here with errors.Is ladders, which
// meant a condition the ladder forgot silently degraded to a 500 (or, worse,
// leaked the sentinel's internal text as the client message).
//
// Anything else is an error the service did not classify — a driver error, an
// IO failure, a bug. Per the "zero internal details" rule in error-handling.md
// it collapses to a generic 500 with logMsg kept as internal-only context; the
// original error travels as the wrapped cause for the log line only.
func serviceError(err error, logMsg string) error {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return err
	}
	return apperror.Internal.Wrap(err).WithLogMessage(logMsg)
}
