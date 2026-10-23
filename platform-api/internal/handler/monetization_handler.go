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
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	api "github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"

	"github.com/wso2/api-platform/httpkit/httputil"
)

// maxMonetizationBodyBytes caps a meter request; it is a few kilobytes at most.
const maxMonetizationBodyBytes = 64 << 10

// MonetizationHandler serves meters and reads subscription plan pricing. Meters made here are kept
// here and reach an API Portal with the plans published there (see service.MonetizationAdminService).
// Pricing is set when a plan is created (SubscriptionPlanHandler) and does not change.
type MonetizationHandler struct {
	service *service.MonetizationAdminService
	slogger *slog.Logger
}

// NewMonetizationHandler creates the handler.
func NewMonetizationHandler(svc *service.MonetizationAdminService, slogger *slog.Logger) *MonetizationHandler {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &MonetizationHandler{service: svc, slogger: slogger}
}

// RegisterRoutes registers the meter routes and the plan pricing read.
func (h *MonetizationHandler) RegisterRoutes(mux router.Router) {
	mux.HandleFunc("GET "+constants.APIBasePath+"/meters", middleware.MapErrors(h.slogger, h.ListMeters))
	mux.HandleFunc("POST "+constants.APIBasePath+"/meters", middleware.MapErrors(h.slogger, h.CreateMeter))
	mux.HandleFunc("GET "+constants.APIBasePath+"/meters/{meterId}", middleware.MapErrors(h.slogger, h.GetMeter))
	mux.HandleFunc("PUT "+constants.APIBasePath+"/meters/{meterId}", middleware.MapErrors(h.slogger, h.UpdateMeter))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/meters/{meterId}", middleware.MapErrors(h.slogger, h.DeleteMeter))
	mux.HandleFunc("GET "+constants.APIBasePath+"/subscription-plans/{subscriptionPlanId}/pricing", middleware.MapErrors(h.slogger, h.GetSubscriptionPlanPricing))
}

// organization returns the caller's organization, from the token.
func organization(r *http.Request) (string, error) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return "", apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	return orgID, nil
}

// readJSONBody reads a bounded request body that must be a JSON object.
func readJSONBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxMonetizationBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, apperror.PayloadTooLarge.New("request body exceeds the maximum allowed size")
		}
		return nil, apperror.ValidationFailed.New("The request body could not be read")
	}
	var object map[string]any
	// A JSON null decodes without error into a nil map: it is not an object either.
	if err := json.Unmarshal(body, &object); err != nil || object == nil {
		return nil, apperror.ValidationFailed.New("The request body must be a JSON object")
	}
	return body, nil
}

// meterResponse is a meter as the REST API returns it, with its stored event keys: typeKey
// is null for a REQUEST_COUNT meter, and filterFields is empty when its prices take no filters.
// planCount is how many plans charge for it: a count, not their names, as a meter may be priced by
// many plans.
func meterResponse(m *model.Meter, plans []string) map[string]any {
	filterFields := make([]map[string]any, 0, len(m.FilterFields))
	for _, f := range m.FilterFields {
		filterFields = append(filterFields, map[string]any{"field": f.Field, "eventKey": f.EventKey})
	}
	return map[string]any{
		"id":           m.Handle,
		"displayName":  m.DisplayName,
		"description":  m.Description,
		"type":         m.Type,
		"typeKey":      m.TypeKey,
		"filterFields": filterFields,
		"updatedAt":    m.SourceUpdatedAt.UTC().Format(time.RFC3339),
		"planCount":    len(plans),
	}
}

// ListMeters handles GET /meters.
func (h *MonetizationHandler) ListMeters(w http.ResponseWriter, r *http.Request) error {
	orgID, err := organization(r)
	if err != nil {
		return err
	}
	opts := parseListOptions(r)
	meters, err := h.service.ListMeters(orgID, opts.Search)
	if err != nil {
		return err
	}
	usage, err := h.service.MeterUsage(orgID)
	if err != nil {
		return err
	}
	page := pageWindow(meters, opts.Limit, opts.Offset)
	items := make([]map[string]any, 0, len(page))
	for _, m := range page {
		items = append(items, meterResponse(m, usage[m.Handle]))
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"list":       items,
		"count":      len(items),
		"pagination": api.Pagination{Total: len(meters), Offset: opts.Offset, Limit: opts.Limit},
	})
	return nil
}

// GetMeter handles GET /meters/{meterId}.
func (h *MonetizationHandler) GetMeter(w http.ResponseWriter, r *http.Request) error {
	orgID, err := organization(r)
	if err != nil {
		return err
	}
	meter, err := h.service.GetMeter(orgID, r.PathValue("meterId"))
	if err != nil {
		return err
	}
	usage, err := h.service.MeterUsage(orgID)
	if err != nil {
		return err
	}
	httputil.WriteJSON(w, http.StatusOK, meterResponse(meter, usage[meter.Handle]))
	return nil
}

// CreateMeter handles POST /meters.
func (h *MonetizationHandler) CreateMeter(w http.ResponseWriter, r *http.Request) error {
	orgID, err := organization(r)
	if err != nil {
		return err
	}
	body, err := readJSONBody(w, r)
	if err != nil {
		return err
	}
	meter, err := h.service.CreateMeter(orgID, body)
	if err != nil {
		return err
	}
	setLocation(w, "meters", meter.Handle)
	httputil.WriteJSON(w, http.StatusCreated, meterResponse(meter, nil))
	return nil
}

// UpdateMeter handles PUT /meters/{meterId}.
func (h *MonetizationHandler) UpdateMeter(w http.ResponseWriter, r *http.Request) error {
	orgID, err := organization(r)
	if err != nil {
		return err
	}
	body, err := readJSONBody(w, r)
	if err != nil {
		return err
	}
	meter, err := h.service.UpdateMeter(orgID, r.PathValue("meterId"), body)
	if err != nil {
		return err
	}
	// Only a meter no plan prices can change, so nothing uses it.
	httputil.WriteJSON(w, http.StatusOK, meterResponse(meter, nil))
	return nil
}

// DeleteMeter handles DELETE /meters/{meterId}.
func (h *MonetizationHandler) DeleteMeter(w http.ResponseWriter, r *http.Request) error {
	orgID, err := organization(r)
	if err != nil {
		return err
	}
	if err := h.service.DeleteMeter(orgID, r.PathValue("meterId")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// GetSubscriptionPlanPricing handles GET /subscription-plans/{subscriptionPlanId}/pricing.
func (h *MonetizationHandler) GetSubscriptionPlanPricing(w http.ResponseWriter, r *http.Request) error {
	orgID, err := organization(r)
	if err != nil {
		return err
	}
	planHandle := r.PathValue("subscriptionPlanId")
	pricing, err := h.service.GetPlanPricing(orgID, planHandle)
	if err != nil {
		return err
	}
	httputil.WriteJSON(w, http.StatusOK, pricingResponse(planHandle, pricing))
	return nil
}

// pricingResponse is a plan's pricing as the REST API returns it: a FLAT_RATE plan's amount (null for
// USAGE_BASED) and a USAGE_BASED plan's prices (none for FLAT_RATE), each with the fields its pricing
// model does not use as null.
func pricingResponse(planHandle string, pricing *model.SubscriptionPlanPricing) map[string]any {
	prices := make([]map[string]any, 0, len(pricing.Prices))
	for _, p := range pricing.Prices {
		prices = append(prices, priceResponse(p))
	}
	return map[string]any{
		"subscriptionPlanId": planHandle,
		"pricingType":        pricing.PricingType,
		"currency":           pricing.Currency,
		"billingPeriod":      pricing.BillingPeriod,
		"flatAmount":         pricing.FlatAmount,
		"prices":             prices,
		"updatedAt":          pricing.SourceUpdatedAt.UTC().Format(time.RFC3339),
	}
}

func priceResponse(p model.SubscriptionPlanPrice) map[string]any {
	filters := make([]map[string]any, 0, len(p.Filters))
	for _, f := range p.Filters {
		filter := map[string]any{"field": f.Field, "operator": f.Operator, "values": f.Values}
		if f.EventKey != "" {
			filter["eventKey"] = f.EventKey
		}
		filters = append(filters, filter)
	}
	var tiers any
	if p.Tiers != nil {
		list := make([]map[string]any, 0, len(p.Tiers))
		for _, t := range p.Tiers {
			list = append(list, map[string]any{"upTo": t.UpTo, "unitAmount": t.UnitAmount, "flatAmount": t.FlatAmount})
		}
		tiers = list
	}
	return map[string]any{
		"meterId":      p.MeterHandle,
		"filters":      filters,
		"pricingModel": p.PricingModel,
		"unitAmount":   p.UnitAmount,
		"unitQuantity": p.UnitQuantity,
		"tiers":        tiers,
	}
}
