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

package apperror

import (
	"errors"
	"fmt"
)

// Def is a catalog entry: the code, HTTP status, and client-facing message
// template are bound together at declaration time (see catalog.go) so call
// sites cannot pair a code with the wrong status or a drifting message. It is
// the Go equivalent of a Java ExceptionCodes enum constant. Errors are
// created only through Def.New / Def.Wrap (plus NewValidation for
// validator.ValidationErrors), never by picking a code/status/message triple
// at the call site.
type Def struct {
	Code       string // stable, client-visible catalog code, e.g. "REST_API_EXISTS"
	HTTPStatus int
	MessageFmt string // client-facing message template; may contain fmt verbs
}

// New instantiates the catalog entry for a business-rule failure with no
// underlying error. args fill the MessageFmt verbs, if any; they must be
// user-supplied values (names, handles, IDs), never internal error text. The
// call stack is captured automatically.
func (d Def) New(args ...any) *Error {
	return newWithSkip(3, d.Code, d.HTTPStatus, d.message(args))
}

// Wrap instantiates the catalog entry for a failure caused by a lower-level
// error (DB, IO, downstream call). The cause is a required positional
// parameter — not a fluent afterthought — so it cannot be forgotten; it feeds
// the mapper's log line and errors.Is/As chains, and is never sent to the
// client.
func (d Def) Wrap(cause error, args ...any) *Error {
	e := newWithSkip(3, d.Code, d.HTTPStatus, d.message(args))
	e.Cause = cause
	return e
}

// Is reports whether err is (or wraps) an *Error carrying this Def's code,
// letting callers branch on catalog entries without sentinel errors.
func (d Def) Is(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == d.Code
}

func (d Def) message(args []any) string {
	if len(args) == 0 {
		return d.MessageFmt
	}
	return fmt.Sprintf(d.MessageFmt, args...)
}
