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

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// portalUndoTimeout bounds removing what a failed publish created in the API Portal. The undo has
// its own time limit, since the publish may have failed because its own time ran out.
const portalUndoTimeout = 30 * time.Second

// portalMeter is a meter in the API Portal's shape, as its REST API takes and gives it.
type portalMeter struct {
	ID           string                   `json:"id"`
	DisplayName  string                   `json:"displayName"`
	Description  *string                  `json:"description,omitempty"`
	Type         string                   `json:"type"`
	TypeKey      *string                  `json:"typeKey,omitempty"`
	FilterFields []portalMeterFilterField `json:"filterFields"`
}

type portalMeterFilterField struct {
	Field    string `json:"field"`
	EventKey string `json:"eventKey"`
}

func toPortalMeter(m *model.Meter) portalMeter {
	out := portalMeter{ID: m.Handle, DisplayName: m.DisplayName, Type: m.Type, TypeKey: m.TypeKey,
		FilterFields: make([]portalMeterFilterField, 0, len(m.FilterFields))}
	if m.Description != "" {
		description := m.Description
		out.Description = &description
	}
	for _, f := range m.FilterFields {
		out.FilterFields = append(out.FilterFields, portalMeterFilterField{Field: f.Field, EventKey: f.EventKey})
	}
	return out
}

// meterKey is a meter's settings in a form that does not depend on how they were written: a name
// trimmed, no description equal to an empty one, the filter fields in any order.
func meterKey(m portalMeter) string {
	description, typeKey := "", ""
	if m.Description != nil {
		description = strings.TrimSpace(*m.Description)
	}
	if m.TypeKey != nil {
		typeKey = *m.TypeKey
	}
	fields := make([]string, 0, len(m.FilterFields))
	for _, f := range m.FilterFields {
		fields = append(fields, f.Field+"|"+f.EventKey)
	}
	slices.Sort(fields)
	return fmt.Sprintf("%s|%s|%s|%s|%q", strings.TrimSpace(m.DisplayName), description, m.Type, typeKey, fields)
}

func metersURL(portal *model.APIPortal) string {
	return strings.TrimRight(portal.URL, "/") + portalRESTBase + "/meters"
}

// pricedMeters returns the meters of sync.Meters that the prices of plans charge for, each once, in
// the order the prices name them.
func pricedMeters(plans []*model.SubscriptionPlan, sync PlanSync) []*model.Meter {
	var meters []*model.Meter
	for _, plan := range plans {
		pricing := sync.Pricing[plan.UUID]
		if pricing == nil {
			continue
		}
		for _, price := range pricing.Prices {
			meter, ok := sync.Meters[price.MeterHandle]
			if ok && !slices.Contains(meters, meter) {
				meters = append(meters, meter)
			}
		}
	}
	return meters
}

// missingMeters returns those of meters the portal lacks. A copy with other settings refuses the
// publish with a *PortalMeterConflictError before anything is written.
func (p *HTTPPortalPublisher) missingMeters(ctx context.Context, portal *model.APIPortal, meters []*model.Meter) ([]*model.Meter, error) {
	var missing []*model.Meter
	for _, meter := range meters {
		portalCopy, err := p.getMeter(ctx, portal, meter.Handle)
		if err != nil {
			return nil, err
		}
		switch {
		case portalCopy == nil:
			missing = append(missing, meter)
		case meterKey(*portalCopy) != meterKey(toPortalMeter(meter)):
			return nil, &PortalMeterConflictError{Handle: meter.Handle}
		}
	}
	return missing, nil
}

// getMeter returns the portal's meter with the handle, or nil when it has none.
func (p *HTTPPortalPublisher) getMeter(ctx context.Context, portal *model.APIPortal, handle string) (*portalMeter, error) {
	resp, err := p.portalRequest(ctx, portal, http.MethodGet, metersURL(portal)+"/"+url.PathEscape(handle), nil)
	if err != nil {
		return nil, fmt.Errorf("portal meter read failed: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
		var meter portalMeter
		if err := json.NewDecoder(io.LimitReader(resp.Body, portalPlansMaxResponseBytes)).Decode(&meter); err != nil {
			return nil, fmt.Errorf("portal meter read returned an unreadable body: %w", err)
		}
		return &meter, nil
	case resp.StatusCode == http.StatusNotFound:
		return nil, nil
	case isPortalAuthFailure(resp.StatusCode):
		return nil, portalAuthError(resp.StatusCode)
	case isPortalRejection(resp.StatusCode):
		return nil, portalRejection(resp, "the API Portal rejected the meter read")
	default:
		return nil, fmt.Errorf("portal meter read failed: unexpected status %d", resp.StatusCode)
	}
}

// createMeter creates the meter in the portal, as it is kept here.
func (p *HTTPPortalPublisher) createMeter(ctx context.Context, portal *model.APIPortal, meter *model.Meter) error {
	body, err := json.Marshal(toPortalMeter(meter))
	if err != nil {
		return fmt.Errorf("failed to encode portal meter: %w", err)
	}
	resp, err := p.portalRequest(ctx, portal, http.MethodPost, metersURL(portal), body)
	if err != nil {
		return fmt.Errorf("portal meter create failed: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusCreated:
		return nil
	case isPortalAuthFailure(resp.StatusCode):
		return portalAuthError(resp.StatusCode)
	case isPortalRejection(resp.StatusCode):
		return portalRejection(resp, "the API Portal rejected the meter")
	default:
		return fmt.Errorf("portal meter create failed: unexpected status %d", resp.StatusCode)
	}
}

// UndoPlanSync implements PortalPublisher: it removes the plans a sync created, then its meters,
// each with its own request, newest first. One the portal no longer has counts as removed; every
// failure is reported.
func (p *HTTPPortalPublisher) UndoPlanSync(ctx context.Context, portal *model.APIPortal, created PlanSyncResult) error {
	var errs []error
	for i := len(created.Plans) - 1; i >= 0; i-- {
		errs = append(errs, p.remove(ctx, portal, plansURL(portal)+"/"+url.PathEscape(created.Plans[i]), "plan "+created.Plans[i]))
	}
	for i := len(created.Meters) - 1; i >= 0; i-- {
		errs = append(errs, p.remove(ctx, portal, metersURL(portal)+"/"+url.PathEscape(created.Meters[i]), "meter "+created.Meters[i]))
	}
	return errors.Join(errs...)
}

// remove deletes the portal object at target; what names it in the error. One the portal no longer
// has (404) counts as removed.
func (p *HTTPPortalPublisher) remove(ctx context.Context, portal *model.APIPortal, target, what string) error {
	resp, err := p.portalRequest(ctx, portal, http.MethodDelete, target, nil)
	if err != nil {
		return fmt.Errorf("removing %s from the API Portal failed: %w", what, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 300 || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return fmt.Errorf("the API Portal did not remove %s (status %d)", what, resp.StatusCode)
}

// undoAfter removes what a sync created before failing with err, with its own time limit, and
// returns err, joined with what could not be removed.
func (p *HTTPPortalPublisher) undoAfter(ctx context.Context, portal *model.APIPortal, created PlanSyncResult, err error) error {
	if len(created.Plans) == 0 && len(created.Meters) == 0 {
		return err
	}
	undoCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), portalUndoTimeout)
	defer cancel()
	if undoErr := p.UndoPlanSync(undoCtx, portal, created); undoErr != nil {
		return fmt.Errorf("%w; what the publish created could not all be removed: %v", err, undoErr)
	}
	return err
}

// portalRequest sends one authenticated request to the portal's REST API, with a JSON body when
// body is not nil.
func (p *HTTPPortalPublisher) portalRequest(ctx context.Context, portal *model.APIPortal, method, target string, body []byte) (*http.Response, error) {
	authHeader, err := p.authHeader(ctx, portal)
	if err != nil {
		return nil, err
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to build portal request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", authHeader)
	return p.client.Do(req)
}
