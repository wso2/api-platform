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

package webhook

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
)

// Application event types. apikey.application_updated is intentionally handled with the other
// apikey.* events (see handlers_apikey.go): it targets an API key, not an application.
const (
	EventApplicationCreated = "application.created"
	EventApplicationUpdated = "application.updated"
	EventApplicationDeleted = "application.deleted"
)

// applicationData is the data payload for application.* events. description and type are absent on
// application.deleted. handle is the Developer Portal application handle, which the Platform API
// stores as the application handle and uses as the resolution key across all application events.
// application_id (the Developer Portal's internal id) is received but not used for resolution. type
// is the application type (e.g. "genai", "web"), validated by the service layer.
type applicationData struct {
	ApplicationID string `json:"application_id"`
	DisplayName   string `json:"display_name"`
	Handle        string `json:"handle"`
	Description   string `json:"description"`
	Type          string `json:"type"`
}

// handleApplicationCreated reconciles an application created in the Developer Portal.
func (r *Receiver) handleApplicationCreated(ctx context.Context, env *Envelope) error {
	var d applicationData
	if err := env.decodeData(&d); err != nil {
		return err
	}
	if strings.TrimSpace(d.Handle) == "" {
		return fmt.Errorf("%w: data.handle is required", ErrInvalidEnvelope)
	}
	if strings.TrimSpace(d.DisplayName) == "" {
		return fmt.Errorf("%w: data.display_name is required", ErrInvalidEnvelope)
	}
	_, err := r.apps.CreateApplicationFromWebhook(d.Handle, d.DisplayName, d.Description, d.Type, env.OrgID)
	if err != nil {
		// Domain-level idempotency: a duplicate delivery whose application already exists is success.
		if errors.Is(err, constants.ErrApplicationExists) || errors.Is(err, constants.ErrHandleExists) {
			return nil
		}
		return err
	}
	return nil
}

// handleApplicationUpdated reconciles an application renamed/updated in the Developer Portal.
// Deliveries are at-least-once and fired once, so if the create event was missed this upserts.
func (r *Receiver) handleApplicationUpdated(ctx context.Context, env *Envelope) error {
	var d applicationData
	if err := env.decodeData(&d); err != nil {
		return err
	}
	handle := strings.TrimSpace(d.Handle)
	if handle == "" {
		return fmt.Errorf("%w: data.handle is required", ErrInvalidEnvelope)
	}
	name := strings.TrimSpace(d.DisplayName)
	desc := strings.TrimSpace(d.Description)
	// handle is the application handle and is immutable: it must match the application being updated,
	// so it is both the lookup key and the body id. Type is applied only when present
	// (UpdateApplication ignores an empty type).
	req := &api.Application{
		Id:          handle,
		DisplayName: name,
		Description: &desc,
		Type:        api.ApplicationType(strings.TrimSpace(d.Type)),
	}
	_, err := r.apps.UpdateApplication(handle, req, env.OrgID, "")
	if err == nil {
		return nil
	}
	if !errors.Is(err, constants.ErrApplicationNotFound) || name == "" {
		return err
	}
	// Upsert: the create event was likely missed.
	if _, cerr := r.apps.CreateApplicationFromWebhook(handle, name, d.Description, d.Type, env.OrgID); cerr != nil &&
		!errors.Is(cerr, constants.ErrApplicationExists) && !errors.Is(cerr, constants.ErrHandleExists) {
		return cerr
	}
	return nil
}

// handleApplicationDeleted reconciles an application deleted in the Developer Portal.
func (r *Receiver) handleApplicationDeleted(ctx context.Context, env *Envelope) error {
	var d applicationData
	if err := env.decodeData(&d); err != nil {
		return err
	}
	if strings.TrimSpace(d.Handle) == "" {
		return fmt.Errorf("%w: data.handle is required", ErrInvalidEnvelope)
	}
	if err := r.apps.DeleteApplication(d.Handle, env.OrgID, ""); err != nil {
		// Domain-level idempotency: already gone is success.
		if errors.Is(err, constants.ErrApplicationNotFound) {
			return nil
		}
		return err
	}
	return nil
}
