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
	"log/slog"
	"slices"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// MonetizationService keeps the copies of the meters made in the API Portal, which sends every
// change as a webhook event. A plan's pricing comes with the plan instead
// (SubscriptionPlanService.UpsertPlanFromWebhook).
//   - A change older than the stored copy is ignored, and every operation can be repeated safely.
//   - A meter that a plan's pricing here uses is not changed or deleted by an event, since that
//     plan's prices were made from it.
type MonetizationService struct {
	meterRepo   repository.MeterRepository
	pricingRepo repository.SubscriptionPlanPricingRepository
	slogger     *slog.Logger
}

// NewMonetizationService creates a monetization service.
func NewMonetizationService(
	meterRepo repository.MeterRepository,
	pricingRepo repository.SubscriptionPlanPricingRepository,
	slogger *slog.Logger,
) *MonetizationService {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &MonetizationService{meterRepo: meterRepo, pricingRepo: pricingRepo, slogger: slogger}
}

// UpsertMeterFromWebhook stores the API Portal's meter, unless the stored copy is newer.
func (s *MonetizationService) UpsertMeterFromWebhook(meter *model.Meter) error {
	existing, err := s.meterRepo.GetByHandle(meter.OrganizationUUID, meter.Handle)
	if err != nil {
		return err
	}
	if existing != nil && meter.SourceUpdatedAt.Before(existing.SourceUpdatedAt) {
		s.slogger.Info("Ignoring a stale meter event", "orgId", meter.OrganizationUUID, "meter", meter.Handle)
		return nil
	}
	if existing != nil {
		priced, err := s.pricedHere(meter.OrganizationUUID, meter.Handle)
		if err != nil {
			return err
		}
		if priced {
			s.slogger.Warn("Ignoring an API Portal change to a meter a plan here prices",
				"orgId", meter.OrganizationUUID, "meter", meter.Handle)
			return nil
		}
		meter.UUID = existing.UUID
	}
	return s.meterRepo.Upsert(meter)
}

// pricedHere reports whether any plan's pricing here has a price on the meter.
func (s *MonetizationService) pricedHere(orgUUID, handle string) (bool, error) {
	pricings, err := s.pricingRepo.ListByOrganization(orgUUID)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(pricings, func(p *model.SubscriptionPlanPricing) bool {
		return slices.ContainsFunc(p.Prices, func(price model.SubscriptionPlanPrice) bool { return price.MeterHandle == handle })
	}), nil
}

// DeleteMeterFromWebhook deletes the meter, unless it was changed after the deletion or a plan's
// pricing here uses it. A meter that does not exist counts as deleted.
func (s *MonetizationService) DeleteMeterFromWebhook(orgUUID, handle string, occurredAt time.Time) error {
	existing, err := s.meterRepo.GetByHandle(orgUUID, handle)
	if err != nil || existing == nil {
		return err
	}
	if occurredAt.Before(existing.SourceUpdatedAt) {
		s.slogger.Info("Ignoring a stale meter deletion", "orgId", orgUUID, "meter", handle)
		return nil
	}
	priced, err := s.pricedHere(orgUUID, handle)
	if err != nil {
		return err
	}
	if priced {
		s.slogger.Warn("Ignoring an API Portal deletion of a meter a plan here prices", "orgId", orgUUID, "meter", handle)
		return nil
	}
	return s.meterRepo.Delete(orgUUID, handle)
}
