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

package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// MeterRepo implements MeterRepository.
type MeterRepo struct {
	db *database.DB
}

// NewMeterRepo creates a meter repository.
func NewMeterRepo(db *database.DB) MeterRepository {
	return &MeterRepo{db: db}
}

const meterColumns = `uuid, organization_uuid, handle, display_name, description, type,
	type_key, filter_fields, source_updated_at, created_at`

func scanMeter(scanner rowScanner) (*model.Meter, error) {
	m := &model.Meter{}
	var (
		description, typeKey sql.NullString
		filterFields         []byte
	)
	if err := scanner.Scan(&m.UUID, &m.OrganizationUUID, &m.Handle, &m.DisplayName, &description, &m.Type,
		&typeKey, &filterFields, &m.SourceUpdatedAt, &m.CreatedAt); err != nil {
		return nil, err
	}
	m.Description = description.String
	m.TypeKey = nullableString(typeKey)
	if err := json.Unmarshal(filterFields, &m.FilterFields); err != nil {
		return nil, fmt.Errorf("failed to decode filter fields of meter %s: %w", m.Handle, err)
	}
	if m.FilterFields == nil {
		m.FilterFields = []model.MeterFilterField{}
	}
	return m, nil
}

// GetByHandle returns the meter, or nil when the organization has no meter with that handle.
func (r *MeterRepo) GetByHandle(orgUUID, handle string) (*model.Meter, error) {
	m, err := scanMeter(r.db.QueryRow(r.db.Rebind(`SELECT `+meterColumns+` FROM meters
		WHERE organization_uuid = ? AND handle = ?`), orgUUID, handle))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get meter: %w", err)
	}
	return m, nil
}

// ListByOrganization returns the organization's meters ordered by handle.
func (r *MeterRepo) ListByOrganization(orgUUID string) ([]*model.Meter, error) {
	rows, err := r.db.Query(r.db.Rebind(`SELECT `+meterColumns+` FROM meters
		WHERE organization_uuid = ? ORDER BY handle`), orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list meters: %w", err)
	}
	defer rows.Close()
	meters := []*model.Meter{}
	for rows.Next() {
		m, err := scanMeter(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan meter: %w", err)
		}
		meters = append(meters, m)
	}
	return meters, rows.Err()
}

// ErrMeterExists is returned by Create when the organization already has a meter with the handle.
var ErrMeterExists = errors.New("a meter with this handle already exists")

// Create stores a new meter; ErrMeterExists when the organization has one with the same handle.
func (r *MeterRepo) Create(m *model.Meter) error {
	filterFields := m.FilterFields
	if filterFields == nil {
		filterFields = []model.MeterFilterField{}
	}
	filterFieldsJSON, err := json.Marshal(filterFields)
	if err != nil {
		return fmt.Errorf("failed to encode filter fields: %w", err)
	}
	if m.UUID == "" {
		m.UUID = uuid.New().String()
	}
	now := time.Now().UTC()
	_, err = r.db.Exec(r.db.Rebind(`INSERT INTO meters (uuid, organization_uuid, handle, display_name, description,
			type, type_key, filter_fields, source_updated_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		m.UUID, m.OrganizationUUID, m.Handle, m.DisplayName, optionalString(m.Description), m.Type,
		m.TypeKey, filterFieldsJSON, m.SourceUpdatedAt.UTC(), now)
	if err != nil && IsUniqueViolation(err) {
		return ErrMeterExists
	}
	if err != nil {
		return fmt.Errorf("failed to insert meter: %w", err)
	}
	m.CreatedAt = now
	return nil
}

// Upsert creates the meter or replaces the stored meter with the same handle.
func (r *MeterRepo) Upsert(m *model.Meter) error {
	filterFields := m.FilterFields
	if filterFields == nil {
		filterFields = []model.MeterFilterField{}
	}
	filterFieldsJSON, err := json.Marshal(filterFields)
	if err != nil {
		return fmt.Errorf("failed to encode filter fields: %w", err)
	}
	update := func() (int64, error) {
		result, err := r.db.Exec(r.db.Rebind(`UPDATE meters
			SET display_name = ?, description = ?, type = ?, type_key = ?, filter_fields = ?,
				source_updated_at = ?
			WHERE organization_uuid = ? AND handle = ?`),
			m.DisplayName, optionalString(m.Description), m.Type, m.TypeKey, filterFieldsJSON,
			m.SourceUpdatedAt.UTC(), m.OrganizationUUID, m.Handle)
		if err != nil {
			return 0, err
		}
		return result.RowsAffected()
	}
	updated, err := update()
	if err != nil {
		return fmt.Errorf("failed to update meter: %w", err)
	}
	if updated > 0 {
		return nil
	}
	if m.UUID == "" {
		m.UUID = uuid.New().String()
	}
	_, err = r.db.Exec(r.db.Rebind(`INSERT INTO meters (uuid, organization_uuid, handle, display_name, description,
			type, type_key, filter_fields, source_updated_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		m.UUID, m.OrganizationUUID, m.Handle, m.DisplayName, optionalString(m.Description), m.Type,
		m.TypeKey, filterFieldsJSON, m.SourceUpdatedAt.UTC(), time.Now().UTC())
	if err != nil && IsUniqueViolation(err) {
		// A concurrent delivery inserted the same meter between the update and the insert.
		_, err = update()
	}
	if err != nil {
		return fmt.Errorf("failed to insert meter: %w", err)
	}
	return nil
}

// Delete removes the meter; a meter that does not exist is not an error.
func (r *MeterRepo) Delete(orgUUID, handle string) error {
	if _, err := r.db.Exec(r.db.Rebind(`DELETE FROM meters WHERE organization_uuid = ? AND handle = ?`),
		orgUUID, handle); err != nil {
		return fmt.Errorf("failed to delete meter: %w", err)
	}
	return nil
}

// SubscriptionPlanPricingRepo implements SubscriptionPlanPricingRepository.
type SubscriptionPlanPricingRepo struct {
	db *database.DB
}

// NewSubscriptionPlanPricingRepo creates a subscription plan pricing repository.
func NewSubscriptionPlanPricingRepo(db *database.DB) SubscriptionPlanPricingRepository {
	return &SubscriptionPlanPricingRepo{db: db}
}

const subscriptionPlanPricingColumns = `subscription_plan_uuid, organization_uuid, currency, billing_period,
	prices, created_at`

func scanSubscriptionPlanPricing(scanner rowScanner) (*model.SubscriptionPlanPricing, error) {
	p := &model.SubscriptionPlanPricing{}
	var prices []byte
	if err := scanner.Scan(&p.SubscriptionPlanUUID, &p.OrganizationUUID, &p.Currency, &p.BillingPeriod,
		&prices, &p.CreatedAt); err != nil {
		return nil, err
	}
	if err := decodePrices(p, prices); err != nil {
		return nil, fmt.Errorf("failed to decode prices of plan %s: %w", p.SubscriptionPlanUUID, err)
	}
	return p, nil
}

// GetByPlan returns the pricing of the plan, or nil when the plan is free.
func (r *SubscriptionPlanPricingRepo) GetByPlan(planUUID, orgUUID string) (*model.SubscriptionPlanPricing, error) {
	p, err := scanSubscriptionPlanPricing(r.db.QueryRow(r.db.Rebind(`SELECT `+subscriptionPlanPricingColumns+`
		FROM subscription_plan_pricing WHERE subscription_plan_uuid = ? AND organization_uuid = ?`), planUUID, orgUUID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription plan pricing: %w", err)
	}
	return p, nil
}

// ListByOrganization returns the pricing of each of the organization's paid plans.
func (r *SubscriptionPlanPricingRepo) ListByOrganization(orgUUID string) ([]*model.SubscriptionPlanPricing, error) {
	rows, err := r.db.Query(r.db.Rebind(`SELECT `+subscriptionPlanPricingColumns+`
		FROM subscription_plan_pricing WHERE organization_uuid = ?`), orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscription plan pricing: %w", err)
	}
	defer rows.Close()
	pricings := []*model.SubscriptionPlanPricing{}
	for rows.Next() {
		p, err := scanSubscriptionPlanPricing(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subscription plan pricing: %w", err)
		}
		pricings = append(pricings, p)
	}
	return pricings, rows.Err()
}

// storedFlatPrice is the one stored price of a FLAT_RATE plan: the amount charged every billing
// period, with the pricing model model.PricingModelFlat.
type storedFlatPrice struct {
	PricingModel string  `json:"pricing_model"`
	Amount       *string `json:"amount"`
}

// encodePrices returns a plan's prices as stored, a JSON array: for a FLAT_RATE plan one
// model.PricingModelFlat price with its amount, for a USAGE_BASED plan its usage prices in order.
func encodePrices(p *model.SubscriptionPlanPricing) ([]byte, error) {
	var stored any
	switch p.PricingType {
	case model.PricingTypeFlatRate:
		if p.FlatAmount == nil || len(p.Prices) > 0 {
			return nil, errors.New("a flat rate plan has an amount and no usage prices")
		}
		stored = []storedFlatPrice{{PricingModel: model.PricingModelFlat, Amount: p.FlatAmount}}
	case model.PricingTypeUsageBased:
		if p.FlatAmount != nil {
			return nil, errors.New("a usage based plan has no flat amount")
		}
		prices := p.Prices
		if prices == nil {
			prices = []model.SubscriptionPlanPrice{}
		}
		for _, price := range prices {
			if price.PricingModel == model.PricingModelFlat {
				return nil, fmt.Errorf("a usage price can't have the pricing model %s", model.PricingModelFlat)
			}
		}
		stored = prices
	default:
		return nil, fmt.Errorf("unknown pricing type %q", p.PricingType)
	}
	pricesJSON, err := json.Marshal(stored)
	if err != nil {
		return nil, fmt.Errorf("failed to encode prices: %w", err)
	}
	return pricesJSON, nil
}

// decodePrices reads a plan's stored prices into p: a single model.PricingModelFlat price is a
// FLAT_RATE plan, anything else a USAGE_BASED plan.
func decodePrices(p *model.SubscriptionPlanPricing, stored []byte) error {
	var entries []storedFlatPrice
	if err := json.Unmarshal(stored, &entries); err != nil {
		return err
	}
	flat := slices.IndexFunc(entries, func(e storedFlatPrice) bool { return e.PricingModel == model.PricingModelFlat })
	switch {
	case flat < 0:
		var prices []model.SubscriptionPlanPrice
		if err := json.Unmarshal(stored, &prices); err != nil {
			return err
		}
		if prices == nil {
			prices = []model.SubscriptionPlanPrice{}
		}
		p.PricingType, p.FlatAmount, p.Prices = model.PricingTypeUsageBased, nil, prices
		return nil
	case len(entries) > 1:
		return fmt.Errorf("a %s price is stored beside other prices", model.PricingModelFlat)
	case entries[flat].Amount == nil:
		return fmt.Errorf("the %s price has no amount", model.PricingModelFlat)
	default:
		p.PricingType, p.FlatAmount = model.PricingTypeFlatRate, entries[flat].Amount
		p.Prices = []model.SubscriptionPlanPrice{}
		return nil
	}
}

// insertSubscriptionPlanPricing inserts a plan's pricing through exec, the transaction that creates
// the plan. The pricing is dated with the CreatedAt the caller sets, or now when none is set.
func insertSubscriptionPlanPricing(exec sqlExecutor, db *database.DB, p *model.SubscriptionPlanPricing, now time.Time) error {
	pricesJSON, err := encodePrices(p)
	if err != nil {
		return err
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if _, err := exec.Exec(db.Rebind(`INSERT INTO subscription_plan_pricing (subscription_plan_uuid, organization_uuid,
			currency, billing_period, prices, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`),
		p.SubscriptionPlanUUID, p.OrganizationUUID, p.Currency, p.BillingPeriod, pricesJSON, p.CreatedAt.UTC()); err != nil {
		return fmt.Errorf("failed to insert subscription plan pricing: %w", err)
	}
	return nil
}

// nullableString reads a nullable column as a pointer: nil for NULL.
func nullableString(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

// optionalString stores an empty string as NULL.
func optionalString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// SubscriptionBillingRepo implements SubscriptionBillingRepository.
type SubscriptionBillingRepo struct {
	db *database.DB
}

// NewSubscriptionBillingRepo creates a subscription billing repository.
func NewSubscriptionBillingRepo(db *database.DB) SubscriptionBillingRepository {
	return &SubscriptionBillingRepo{db: db}
}

const subscriptionBillingColumns = `subscription_uuid, organization_uuid, billing_status, billing_customer_id,
	billing_subscription_id, billing_updated_at`

func scanSubscriptionBilling(scanner rowScanner) (*model.SubscriptionBilling, error) {
	b := &model.SubscriptionBilling{}
	err := scanner.Scan(&b.SubscriptionUUID, &b.OrganizationUUID, &b.Status, &b.CustomerID, &b.SubscriptionID, &b.UpdatedAt)
	return b, err
}

// Get returns the billing state of the subscription, or nil for a free subscription.
func (r *SubscriptionBillingRepo) Get(subscriptionUUID, orgUUID string) (*model.SubscriptionBilling, error) {
	b, err := scanSubscriptionBilling(r.db.QueryRow(r.db.Rebind(`SELECT `+subscriptionBillingColumns+`
		FROM subscription_billing WHERE subscription_uuid = ? AND organization_uuid = ?`), subscriptionUUID, orgUUID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription billing: %w", err)
	}
	return b, nil
}

// subscriptionBillingBatchSize is how many subscriptions GetMany asks for in one query, well
// below the parameter limit of each database (SQL Server allows 2100).
const subscriptionBillingBatchSize = 500

// GetMany returns the billing state of each given subscription that has one, by subscription UUID.
// The subscriptions are read in batches of subscriptionBillingBatchSize.
func (r *SubscriptionBillingRepo) GetMany(subscriptionUUIDs []string, orgUUID string) (map[string]*model.SubscriptionBilling, error) {
	result := map[string]*model.SubscriptionBilling{}
	for batch := range slices.Chunk(subscriptionUUIDs, subscriptionBillingBatchSize) {
		if err := r.getBatch(batch, orgUUID, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// getBatch adds the billing state of each subscription in batch that has one to result.
func (r *SubscriptionBillingRepo) getBatch(batch []string, orgUUID string, result map[string]*model.SubscriptionBilling) error {
	placeholders := make([]string, len(batch))
	args := make([]any, 0, len(batch)+1)
	for i, id := range batch {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, orgUUID)
	rows, err := r.db.Query(r.db.Rebind(`SELECT `+subscriptionBillingColumns+` FROM subscription_billing
		WHERE subscription_uuid IN (`+strings.Join(placeholders, ", ")+`) AND organization_uuid = ?`), args...)
	if err != nil {
		return fmt.Errorf("failed to list subscription billing: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		b, err := scanSubscriptionBilling(rows)
		if err != nil {
			return fmt.Errorf("failed to scan subscription billing: %w", err)
		}
		result[b.SubscriptionUUID] = b
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to list subscription billing: %w", err)
	}
	return nil
}

// Upsert stores the billing state of a subscription.
func (r *SubscriptionBillingRepo) Upsert(b *model.SubscriptionBilling) error {
	update := func() (int64, error) {
		result, err := r.db.Exec(r.db.Rebind(`UPDATE subscription_billing
			SET billing_status = ?, billing_customer_id = ?, billing_subscription_id = ?, billing_updated_at = ?
			WHERE subscription_uuid = ? AND organization_uuid = ?`),
			b.Status, b.CustomerID, b.SubscriptionID, b.UpdatedAt.UTC(), b.SubscriptionUUID, b.OrganizationUUID)
		if err != nil {
			return 0, err
		}
		return result.RowsAffected()
	}
	updated, err := update()
	if err != nil {
		return fmt.Errorf("failed to update subscription billing: %w", err)
	}
	if updated > 0 {
		return nil
	}
	_, err = r.db.Exec(r.db.Rebind(`INSERT INTO subscription_billing (subscription_uuid, organization_uuid,
			billing_status, billing_customer_id, billing_subscription_id, billing_updated_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`),
		b.SubscriptionUUID, b.OrganizationUUID, b.Status, b.CustomerID, b.SubscriptionID, b.UpdatedAt.UTC(), time.Now().UTC())
	if err != nil && IsUniqueViolation(err) {
		_, err = update()
	}
	if err != nil {
		return fmt.Errorf("failed to insert subscription billing: %w", err)
	}
	return nil
}
