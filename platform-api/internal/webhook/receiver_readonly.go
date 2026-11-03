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

// TEMP-READ-ONLY-MODE: this whole file is part of the temporary organization-scoped
// read-only mode used while Bijira migrates from Platform API v1 to v2. Delete it
// when the mode is removed — see README.md "Read-only (maintenance) mode — temporary"
// for the removal checklist.

package webhook

import (
	"log/slog"
	"net/http"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
)

// readOnlyMode is the one thing the receiver needs from config.ReadOnly. It is
// declared here, not in receiver.go, so that file's single marked field needs
// nothing else; config.ReadOnly satisfies it with a nil-safe method, so a
// never-wired (nil) field means "mode off".
type readOnlyMode interface {
	IsReadOnlyOrg(orgUUID string) bool
}

// SetReadOnly wires the read-only (maintenance) mode configuration into the
// receiver. Every event it handles mutates the organization's rows, so an event
// for a read-only organization is refused once the organization is resolved.
func (r *Receiver) SetReadOnly(ro *config.ReadOnly) {
	r.readOnly = ro
}

// rejectIfReadOnly writes a 503 and reports true when the event's (already
// resolved) organization is read-only. 503 — the same status the user-facing
// routes return — lets the producer retry once maintenance ends rather than
// silently losing the event. It is written directly rather than returned
// through the error mapper: this is an expected maintenance outcome, not a 5xx
// system fault that warrants an ERROR log with a stack.
func (r *Receiver) rejectIfReadOnly(w http.ResponseWriter, env *Envelope, log *slog.Logger) bool {
	if r.readOnly == nil || !r.readOnly.IsReadOnlyOrg(env.OrgID) {
		return false
	}
	log.Warn("Rejected webhook event in read-only mode")
	apperror.WriteHTTP(w, apperror.OrganizationReadOnly.New(), "")
	return true
}
