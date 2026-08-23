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

package migrationcore

import (
	"fmt"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// sarg returns a nullable-string insert arg (value or nil for NULL).
func sarg(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// insertArtifact writes the parent artifacts(uuid, type, organization_uuid) row.
func insertArtifact(ex Execer, opts Options, uuid, kind, org string) error {
	return upsert(ex, opts, "artifacts", []string{"uuid", "type", "organization_uuid"},
		[]any{uuid, kind, org}, []string{"uuid"})
}

// ---- organizations ----

// OrganizationRow mirrors the v1 organizations columns needed for the v2 write.
// Handle is pre-resolved by the caller (batch: persist-and-replay; live: slug+uuid).
type OrganizationRow struct {
	UUID, Handle, DisplayName, Region string
	CreatedAt, UpdatedAt              *time.Time
	CreatedBy                         string // raw v1 actor ("" ⇒ migration actor)
}

func UpsertOrganization(ex Execer, r OrganizationRow, opts Options, rep Reporter) error {
	// idp_organization_ref_uuid = org uuid (deterministic placeholder).
	rep.Flag("organizations", r.UUID, FlagPlaceholderIDP, nil,
		map[string]any{"idp_organization_ref_uuid": r.UUID})
	createdBy, err := audit(ex, opts, rep, "organizations", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	return upsert(ex, opts, "organizations",
		[]string{"uuid", "handle", "display_name", "region", "idp_organization_ref_uuid",
			"data_version", "created_by", "created_at", "updated_by", "updated_at"},
		[]any{r.UUID, r.Handle, r.DisplayName, r.Region, r.UUID, DataVersion, createdBy,
			tsArg(r.CreatedAt, opts), createdBy, tsArg(r.UpdatedAt, opts)},
		[]string{"uuid"})
}

// ---- projects ----

type ProjectRow struct {
	UUID, Handle, DisplayName, Org string
	Description                    *string
	CreatedAt, UpdatedAt           *time.Time
	CreatedBy                      string
}

func UpsertProject(ex Execer, r ProjectRow, opts Options, rep Reporter) error {
	createdBy, err := audit(ex, opts, rep, "projects", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	return upsert(ex, opts, "projects",
		[]string{"uuid", "handle", "display_name", "organization_uuid", "description",
			"data_version", "created_by", "created_at", "updated_by", "updated_at"},
		[]any{r.UUID, r.Handle, r.DisplayName, r.Org, sarg(r.Description), DataVersion, createdBy,
			tsArg(r.CreatedAt, opts), createdBy, tsArg(r.UpdatedAt, opts)},
		[]string{"uuid"})
}

// ---- applications ----

type ApplicationRow struct {
	UUID, Handle, ProjectUUID, Org, DisplayName, Type string
	Description                                        *string
	CreatedAt, UpdatedAt                               *time.Time
	CreatedBy                                          string
}

func UpsertApplication(ex Execer, r ApplicationRow, opts Options, rep Reporter) error {
	createdBy, err := audit(ex, opts, rep, "applications", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	return upsert(ex, opts, "applications",
		[]string{"uuid", "handle", "project_uuid", "organization_uuid", "display_name", "description",
			"type", "data_version", "created_by", "created_at", "updated_by", "updated_at"},
		[]any{r.UUID, r.Handle, r.ProjectUUID, r.Org, r.DisplayName, sarg(r.Description), r.Type,
			DataVersion, createdBy, tsArg(r.CreatedAt, opts), createdBy, tsArg(r.UpdatedAt, opts)},
		[]string{"uuid"})
}

// ---- rest_apis (RestApi) ----

type RestAPIRow struct {
	UUID, Handle, DisplayName, Version, Org, ProjectUUID string
	Description, Lifecycle, Transport                    *string
	Configuration                                        []byte // v1 JSONB
	CreatedAt, UpdatedAt                                 *time.Time
	CreatedBy                                            string
}

func UpsertRestAPI(ex Execer, r RestAPIRow, opts Options, rep Reporter) error {
	blob, _, unknown, err := ReshapeRestAPIConfig(r.Configuration, deref(r.Transport))
	if err != nil {
		rep.Quarantine("rest_apis", r.UUID, ReasonBlobUnparseable, err.Error(),
			map[string]any{"uuid": r.UUID})
		return ErrBlobUnparseable
	}
	rep.DroppedFields("RestAPIConfig", unknown)
	lc := lifecycleOr(r.Lifecycle)
	createdBy, err := audit(ex, opts, rep, "rest_apis", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	if err := insertArtifact(ex, opts, r.UUID, constants.RestApi, r.Org); err != nil {
		return err
	}
	return upsert(ex, opts, "rest_apis",
		[]string{"uuid", "organization_uuid", "handle", "display_name", "version", "project_uuid",
			"description", "lifecycle_status", "configuration", "data_version", "origin",
			"created_by", "created_at", "updated_by", "updated_at"},
		[]any{r.UUID, r.Org, r.Handle, r.DisplayName, r.Version, r.ProjectUUID, sarg(r.Description), lc, blob,
			DataVersion, OriginCP, createdBy, tsArg(r.CreatedAt, opts), createdBy, tsArg(r.UpdatedAt, opts)},
		[]string{"uuid"})
}

// ---- llm_provider_templates ----

type LLMTemplateRow struct {
	UUID, Org, Handle, DisplayName string
	Description                    *string
	Configuration                 []byte // v1 TEXT
	CreatedAt, UpdatedAt           *time.Time
	CreatedBy                      string
}

func UpsertLLMProviderTemplate(ex Execer, r LLMTemplateRow, opts Options, rep Reporter) error {
	groupID := r.Handle // group_id = the template's v1 handle (matches v2 create path)
	rep.Flag("llm_provider_templates", r.UUID, FlagSynthesized, nil, map[string]any{
		"group_id": groupID, "version": "v1.0", "managed_by": constants.TemplateManagedByOrganization,
		"is_latest": 1, "enabled": 1, "openapi_spec": nil})
	createdBy, err := audit(ex, opts, rep, "llm_provider_templates", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	return upsert(ex, opts, "llm_provider_templates",
		[]string{"uuid", "organization_uuid", "handle", "group_id", "display_name", "managed_by",
			"version", "description", "configuration", "openapi_spec", "is_latest", "enabled",
			"data_version", "origin", "created_by", "created_at", "updated_by", "updated_at"},
		[]any{r.UUID, r.Org, r.Handle, groupID, r.DisplayName, constants.TemplateManagedByOrganization,
			"v1.0", sarg(r.Description), r.Configuration, nil, 1, 1,
			DataVersion, OriginCP, createdBy, tsArg(r.CreatedAt, opts), createdBy, tsArg(r.UpdatedAt, opts)},
		[]string{"uuid"})
}

// ---- llm_providers (LlmProvider) ----

type LLMProviderRow struct {
	UUID, Handle, DisplayName, Version, Org, TemplateUUID string
	Description, OpenAPISpec, ModelList, Status           *string
	Configuration                                         []byte
	CreatedAt, UpdatedAt                                  *time.Time
	CreatedBy                                             string
}

func UpsertLLMProvider(ex Execer, r LLMProviderRow, opts Options, rep Reporter) error {
	var cfg model.LLMProviderConfig
	blob, unknown, err := RemarshalConfig(r.Configuration, &cfg)
	if err != nil {
		rep.Quarantine("llm_providers", r.UUID, ReasonBlobUnparseable, err.Error(), map[string]any{"uuid": r.UUID})
		return ErrBlobUnparseable
	}
	rep.DroppedFields("LLMProviderConfig", unknown)
	flagPlaintextCredential(rep, "llm_providers", r.UUID, cfg.Security)
	if r.Status != nil && *r.Status != "" {
		rep.Dropped("field", "llm_providers", r.UUID, DropLLMProviderStatus, *r.Status)
	}
	createdBy, err := audit(ex, opts, rep, "llm_providers", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	if err := insertArtifact(ex, opts, r.UUID, constants.LLMProvider, r.Org); err != nil {
		return err
	}
	return upsert(ex, opts, "llm_providers",
		[]string{"uuid", "handle", "display_name", "version", "description", "template_uuid",
			"openapi_spec", "model_list", "configuration", "data_version", "origin",
			"created_by", "created_at", "updated_by", "updated_at", "organization_uuid"},
		[]any{r.UUID, r.Handle, r.DisplayName, r.Version, sarg(r.Description), r.TemplateUUID,
			bytesOrNilP(r.OpenAPISpec), bytesOrNilP(r.ModelList), blob, DataVersion, OriginCP,
			createdBy, tsArg(r.CreatedAt, opts), createdBy, tsArg(r.UpdatedAt, opts), r.Org},
		[]string{"uuid"})
}

// ---- llm_proxies (LlmProxy) ----

type LLMProxyRow struct {
	UUID, Handle, DisplayName, Version, ProjectUUID, Org, ProviderUUID string
	Description, OpenAPISpec, Status                                   *string
	Configuration                                                     []byte
	CreatedAt, UpdatedAt                                              *time.Time
	CreatedBy                                                         string
}

func UpsertLLMProxy(ex Execer, r LLMProxyRow, opts Options, rep Reporter) error {
	var cfg model.LLMProxyConfig
	blob, unknown, err := RemarshalConfig(r.Configuration, &cfg)
	if err != nil {
		rep.Quarantine("llm_proxies", r.UUID, ReasonBlobUnparseable, err.Error(), map[string]any{"uuid": r.UUID})
		return ErrBlobUnparseable
	}
	rep.DroppedFields("LLMProxyConfig", unknown)
	flagPlaintextCredential(rep, "llm_proxies", r.UUID, cfg.Security)
	if r.Status != nil && *r.Status != "" {
		rep.Dropped("field", "llm_proxies", r.UUID, DropLLMProxyStatus, *r.Status)
	}
	createdBy, err := audit(ex, opts, rep, "llm_proxies", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	if err := insertArtifact(ex, opts, r.UUID, constants.LLMProxy, r.Org); err != nil {
		return err
	}
	return upsert(ex, opts, "llm_proxies",
		[]string{"uuid", "handle", "display_name", "version", "project_uuid", "description", "provider_uuid",
			"openapi_spec", "configuration", "data_version", "origin",
			"created_by", "created_at", "updated_by", "updated_at", "organization_uuid"},
		[]any{r.UUID, r.Handle, r.DisplayName, r.Version, r.ProjectUUID, sarg(r.Description), r.ProviderUUID,
			bytesOrNilP(r.OpenAPISpec), blob, DataVersion, OriginCP,
			createdBy, tsArg(r.CreatedAt, opts), createdBy, tsArg(r.UpdatedAt, opts), r.Org},
		[]string{"uuid"})
}

// ---- mcp_proxies (Mcp) ----

type MCPProxyRow struct {
	UUID, Handle, DisplayName, Version, Org string
	ProjectUUID, Description, Status        *string
	Configuration                          []byte
	CreatedAt, UpdatedAt                    *time.Time
	CreatedBy                              string
}

func UpsertMCPProxy(ex Execer, r MCPProxyRow, opts Options, rep Reporter) error {
	var cfg model.MCPProxyConfiguration
	blob, unknown, err := RemarshalConfig(r.Configuration, &cfg)
	if err != nil {
		rep.Quarantine("mcp_proxies", r.UUID, ReasonBlobUnparseable, err.Error(), map[string]any{"uuid": r.UUID})
		return ErrBlobUnparseable
	}
	rep.DroppedFields("MCPProxyConfiguration", unknown)
	if r.Status != nil && *r.Status != "" {
		rep.Dropped("field", "mcp_proxies", r.UUID, DropMCPStatus, *r.Status)
	}
	createdBy, err := audit(ex, opts, rep, "mcp_proxies", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	if err := insertArtifact(ex, opts, r.UUID, constants.MCPProxy, r.Org); err != nil {
		return err
	}
	return upsert(ex, opts, "mcp_proxies",
		[]string{"uuid", "handle", "display_name", "version", "project_uuid", "description",
			"configuration", "data_version", "origin",
			"created_by", "created_at", "updated_by", "updated_at", "organization_uuid"},
		[]any{r.UUID, r.Handle, r.DisplayName, r.Version, sarg(r.ProjectUUID), sarg(r.Description),
			blob, DataVersion, OriginCP,
			createdBy, tsArg(r.CreatedAt, opts), createdBy, tsArg(r.UpdatedAt, opts), r.Org},
		[]string{"uuid"})
}

// ---- websub_apis (WebSubApi, plugin table) ----

type WebSubRow struct {
	UUID, Handle, DisplayName, Version, Org, ProjectUUID string
	Description, Lifecycle, Transport                    *string
	Configuration                                        []byte
	CreatedAt, UpdatedAt                                 *time.Time
	CreatedBy                                            string
}

func UpsertWebSubAPI(ex Execer, r WebSubRow, opts Options, rep Reporter) error {
	blob, _, unknown, notes, err := ReshapeWebSubConfig(r.Configuration, deref(r.Transport))
	if err != nil {
		rep.Quarantine("websub_apis", r.UUID, ReasonBlobUnparseable, err.Error(), map[string]any{"uuid": r.UUID})
		return ErrBlobUnparseable
	}
	rep.DroppedFields("WebSubAPIConfiguration", unknown)
	if len(notes) > 0 {
		rep.Flag("websub_apis", r.UUID, FlagSynthesized, nil, map[string]any{"structural_reshape": notes})
	}
	lc := lifecycleOr(r.Lifecycle)
	createdBy, err := audit(ex, opts, rep, "websub_apis", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	if err := insertArtifact(ex, opts, r.UUID, constants.WebSubApi, r.Org); err != nil {
		return err
	}
	return upsert(ex, opts, "websub_apis",
		[]string{"uuid", "organization_uuid", "handle", "display_name", "version", "project_uuid",
			"description", "lifecycle_status", "configuration", "data_version", "origin",
			"created_by", "created_at", "updated_by", "updated_at"},
		[]any{r.UUID, r.Org, r.Handle, r.DisplayName, r.Version, r.ProjectUUID, sarg(r.Description), lc, blob,
			DataVersion, OriginCP, createdBy, tsArg(r.CreatedAt, opts), createdBy, tsArg(r.UpdatedAt, opts)},
		[]string{"uuid"})
}

// ---- webbroker_apis (WebBrokerApi, plugin table) ----

type WebBrokerRow struct {
	UUID, Handle, DisplayName, Version, Org, ProjectUUID string
	Description, Lifecycle, Transport                    *string
	Configuration                                        []byte
	CreatedAt, UpdatedAt                                 *time.Time
	CreatedBy                                            string
}

func UpsertWebBrokerAPI(ex Execer, r WebBrokerRow, opts Options, rep Reporter) error {
	blob, _, unknown, err := ReshapeWebBrokerConfig(r.Configuration, deref(r.Transport))
	if err != nil {
		rep.Quarantine("webbroker_apis", r.UUID, ReasonBlobUnparseable, err.Error(), map[string]any{"uuid": r.UUID})
		return ErrBlobUnparseable
	}
	rep.DroppedFields("WebBrokerAPIConfiguration", unknown)
	lc := lifecycleOr(r.Lifecycle)
	createdBy, err := audit(ex, opts, rep, "webbroker_apis", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	if err := insertArtifact(ex, opts, r.UUID, constants.WebBrokerApi, r.Org); err != nil {
		return err
	}
	return upsert(ex, opts, "webbroker_apis",
		[]string{"uuid", "organization_uuid", "handle", "display_name", "version", "project_uuid",
			"description", "lifecycle_status", "configuration", "data_version", "origin",
			"created_by", "created_at", "updated_by", "updated_at"},
		[]any{r.UUID, r.Org, r.Handle, r.DisplayName, r.Version, r.ProjectUUID, sarg(r.Description), lc, blob,
			DataVersion, OriginCP, createdBy, tsArg(r.CreatedAt, opts), createdBy, tsArg(r.UpdatedAt, opts)},
		[]string{"uuid"})
}

// ---- subscription_plans (+ subscription_plan_limits) ----

type SubscriptionPlanRow struct {
	UUID, Handle, DisplayName, Org, Status string
	BillingPlan, ThrottleUnit              *string
	StopOnQuota                            *bool
	ThrottleCount                          *int64
	ExpiryTime, CreatedAt, UpdatedAt       *time.Time
	CreatedBy                              string
}

// ErrUnmappedThrottleUnit is returned when a v1 throttle unit has no v2 mapping.
type ErrUnmappedThrottleUnit struct{ Unit string }

func (e ErrUnmappedThrottleUnit) Error() string {
	return "unmapped throttle_limit_unit " + e.Unit
}

func UpsertSubscriptionPlan(ex Execer, r SubscriptionPlanRow, opts Options, rep Reporter) error {
	if r.BillingPlan != nil && *r.BillingPlan != "" {
		rep.Dropped("field", "subscription_plans", r.UUID, DropBillingPlan, *r.BillingPlan)
	}
	createdBy, err := audit(ex, opts, rep, "subscription_plans", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	if err := upsert(ex, opts, "subscription_plans",
		[]string{"uuid", "handle", "display_name", "expiry_time", "organization_uuid", "status",
			"data_version", "created_by", "created_at", "updated_by", "updated_at"},
		[]any{r.UUID, r.Handle, r.DisplayName, tsArg(r.ExpiryTime, opts), r.Org, r.Status, DataVersion,
			createdBy, tsArg(r.CreatedAt, opts), createdBy, tsArg(r.UpdatedAt, opts)},
		[]string{"uuid"}); err != nil {
		return err
	}

	// Throttle → exactly one subscription_plan_limits row. On a live UPDATE (§8.3) the
	// throttle unit may have changed (which shifts the row's natural key and would leave
	// the old limit behind) or the throttle may have been cleared: upsert the desired
	// limit, then delete any superseded limit rows so the v2 child state matches the v1
	// plan. Batch is InsertOnly, so the replacement deletes are skipped → byte-identical.
	if r.ThrottleCount != nil && r.ThrottleUnit != nil && *r.ThrottleUnit != "" {
		timeUnit, ok := CaseConvertThrottleUnit(*r.ThrottleUnit)
		if !ok {
			return ErrUnmappedThrottleUnit{Unit: *r.ThrottleUnit}
		}
		limitTS := opts.Epoch
		if r.CreatedAt != nil {
			limitTS = ReinterpretTZ(*r.CreatedAt, opts.loc())
		}
		limitUUID := DeterministicUUID(r.UUID+"|"+constants.LimitTypeRequestCount+"|"+timeUnit, limitTS)
		stop := 1
		if r.StopOnQuota != nil {
			stop = BoolToSmallint(*r.StopOnQuota)
		}
		if err := upsert(ex, opts, "subscription_plan_limits",
			[]string{"uuid", "subscription_plan_uuid", "limit_type", "time_unit", "time_amount",
				"limit_count", "limit_count_unit", "stop_on_quota_reach"},
			[]any{limitUUID, r.UUID, constants.LimitTypeRequestCount, timeUnit, 1, *r.ThrottleCount, nil, stop},
			[]string{"subscription_plan_uuid", "limit_type", "time_amount", "time_unit"}); err != nil {
			return err
		}
		if !opts.InsertOnly && !opts.DryRun {
			if _, err := ex.Exec("DELETE FROM subscription_plan_limits WHERE subscription_plan_uuid = $1 AND uuid <> $2", r.UUID, limitUUID); err != nil {
				return fmt.Errorf("replace subscription_plan_limits for %s: %w", r.UUID, err)
			}
		}
	} else if !opts.InsertOnly && !opts.DryRun {
		// Throttle cleared on a live UPDATE → drop any stale limit rows.
		if _, err := ex.Exec("DELETE FROM subscription_plan_limits WHERE subscription_plan_uuid = $1", r.UUID); err != nil {
			return fmt.Errorf("clear subscription_plan_limits for %s: %w", r.UUID, err)
		}
	}
	return nil
}

// CaseConvertThrottleUnit maps a v1 PascalCase throttle unit to v2's UPPERCASE.
func CaseConvertThrottleUnit(u string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(u)) {
	case "min", "minute":
		return constants.ThrottleLimitUnitMinute, true
	case "hour":
		return constants.ThrottleLimitUnitHour, true
	case "day":
		return constants.ThrottleLimitUnitDay, true
	case "month":
		return constants.ThrottleLimitUnitMonth, true
	default:
		return "", false
	}
}

// ---- subscriptions ----

type SubscriptionRow struct {
	UUID, ArtifactUUID, SubscriberID, Token, Hash, Org, Status string
	ApplicationID, PlanUUID                                    *string
	CreatedAt, UpdatedAt                                       *time.Time
	CreatedBy                                                  string
}

func UpsertSubscription(ex Execer, r SubscriptionRow, opts Options, rep Reporter) error {
	createdBy, err := audit(ex, opts, rep, "subscriptions", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	return upsert(ex, opts, "subscriptions",
		[]string{"uuid", "artifact_uuid", "subscriber_id", "application_id", "subscription_token",
			"subscription_token_hash", "subscription_plan_uuid", "organization_uuid", "status",
			"data_version", "created_by", "created_at", "updated_by", "updated_at"},
		[]any{r.UUID, r.ArtifactUUID, r.SubscriberID, sarg(r.ApplicationID), r.Token, r.Hash, sarg(r.PlanUUID), r.Org, r.Status,
			DataVersion, createdBy, tsArg(r.CreatedAt, opts), createdBy, tsArg(r.UpdatedAt, opts)},
		[]string{"uuid"})
}

// ---- gateways (+ gateway_endpoints) ----

type GatewayRow struct {
	UUID, Org, Handle, DisplayName, Version, FuncType, Vhost string
	Description                                              *string
	Properties, Manifest                                    []byte
	IsCritical, IsActive                                     bool
	CreatedAt, UpdatedAt                                     *time.Time
	CreatedBy                                               string
}

func UpsertGateway(ex Execer, r GatewayRow, opts Options, rep Reporter) error {
	ver, cut := TruncateStr(r.Version, 30)
	if cut {
		rep.Flag("gateways", r.UUID, FlagTruncated, map[string]any{"version": r.Version}, map[string]any{"version": ver})
	}
	createdBy, err := audit(ex, opts, rep, "gateways", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	if err := upsert(ex, opts, "gateways",
		[]string{"uuid", "organization_uuid", "handle", "display_name", "description", "version",
			"gateway_functionality_type", "properties", "manifest", "is_active", "is_critical",
			"data_version", "created_by", "created_at", "updated_by", "updated_at"},
		[]any{r.UUID, r.Org, r.Handle, r.DisplayName, sarg(r.Description), ver, r.FuncType, r.Properties, r.Manifest,
			BoolToSmallint(r.IsActive), BoolToSmallint(r.IsCritical),
			DataVersion, createdBy, tsArg(r.CreatedAt, opts), createdBy, tsArg(r.UpdatedAt, opts)},
		[]string{"uuid"}); err != nil {
		return err
	}
	// vhost → exactly one gateway_endpoints row. On a live UPDATE (§8.3) the vhost may
	// have changed (gateway_endpoints has a SERIAL id and no natural key, so the changed
	// vhost inserts a second row) or been cleared: insert the desired endpoint, then
	// delete any superseded ones so the v2 child state matches the v1 gateway. Batch is
	// InsertOnly, so the replacement deletes are skipped → batch output is byte-identical.
	if strings.TrimSpace(r.Vhost) != "" {
		if err := UpsertGatewayEndpoint(ex, r.UUID, r.Vhost, opts); err != nil {
			return err
		}
		if !opts.InsertOnly && !opts.DryRun {
			if _, err := ex.Exec("DELETE FROM gateway_endpoints WHERE gateway_uuid = $1 AND url <> $2", r.UUID, r.Vhost); err != nil {
				return fmt.Errorf("replace gateway_endpoints for %s: %w", r.UUID, err)
			}
		}
		return nil
	}
	// vhost cleared on a live UPDATE → drop any stale endpoint rows.
	if !opts.InsertOnly && !opts.DryRun {
		if _, err := ex.Exec("DELETE FROM gateway_endpoints WHERE gateway_uuid = $1", r.UUID); err != nil {
			return fmt.Errorf("clear gateway_endpoints for %s: %w", r.UUID, err)
		}
	}
	return nil
}

// UpsertGatewayEndpoint inserts (gateway_uuid, url) once; gateway_endpoints has a
// SERIAL id and no natural unique key, so idempotency is an explicit existence check.
// Returns whether a row was inserted.
func UpsertGatewayEndpoint(ex Execer, gatewayUUID, url string, opts Options) error {
	if opts.DryRun {
		return nil
	}
	var one int
	err := ex.QueryRow("SELECT 1 FROM gateway_endpoints WHERE gateway_uuid = $1 AND url = $2", gatewayUUID, url).Scan(&one)
	if err == nil {
		return nil // already present
	}
	if err.Error() != "sql: no rows in result set" {
		return err
	}
	return upsert(ex, opts, "gateway_endpoints", []string{"gateway_uuid", "url"}, []any{gatewayUUID, url}, nil)
}

// ---- artifact_gateway_mappings ----

type ArtifactGatewayMappingRow struct {
	ArtifactUUID, Org, GatewayUUID string
	CreatedAt, UpdatedAt           *time.Time
	CreatedBy                      string
}

func UpsertArtifactGatewayMapping(ex Execer, r ArtifactGatewayMappingRow, opts Options, rep Reporter) error {
	key := r.ArtifactUUID + "|" + r.GatewayUUID
	createdBy, err := audit(ex, opts, rep, "artifact_gateway_mappings", key, r.CreatedBy)
	if err != nil {
		return err
	}
	return upsert(ex, opts, "artifact_gateway_mappings",
		[]string{"artifact_uuid", "organization_uuid", "gateway_uuid", "metadata",
			"created_by", "created_at", "updated_by", "updated_at"},
		[]any{r.ArtifactUUID, r.Org, r.GatewayUUID, nil, createdBy, tsArg(r.CreatedAt, opts), createdBy, tsArg(r.UpdatedAt, opts)},
		[]string{"organization_uuid", "artifact_uuid", "gateway_uuid"})
}

// ---- gateway_custom_policies ----

type GatewayCustomPolicyRow struct {
	UUID, Org, Name, Version string
	DisplayName, Description  *string
	PolicyDefinition          []byte
	CreatedAt, UpdatedAt      *time.Time
	CreatedBy                 string
}

func UpsertGatewayCustomPolicy(ex Execer, r GatewayCustomPolicyRow, opts Options, rep Reporter) error {
	var descVal any
	if r.Description != nil {
		d, cut := TruncateStr(*r.Description, 1023)
		if cut {
			rep.Flag("gateway_custom_policies", r.UUID, FlagTruncated,
				map[string]any{"description_len": len(*r.Description)}, map[string]any{"description_len": len(d)})
		}
		descVal = d
	}
	createdBy, err := audit(ex, opts, rep, "gateway_custom_policies", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	return upsert(ex, opts, "gateway_custom_policies",
		[]string{"uuid", "organization_uuid", "name", "display_name", "version", "description",
			"policy_definition", "data_version", "created_by", "created_at", "updated_by", "updated_at"},
		[]any{r.UUID, r.Org, r.Name, sarg(r.DisplayName), r.Version, descVal, r.PolicyDefinition, DataVersion,
			createdBy, tsArg(r.CreatedAt, opts), createdBy, tsArg(r.UpdatedAt, opts)},
		[]string{"uuid"})
}

// ---- gateway_custom_policy_usages ----

type PolicyUsageRow struct{ PolicyUUID, ArtifactUUID string }

func UpsertPolicyUsage(ex Execer, r PolicyUsageRow, opts Options, rep Reporter) error {
	return upsert(ex, opts, "gateway_custom_policy_usages",
		[]string{"policy_uuid", "artifact_uuid"}, []any{r.PolicyUUID, r.ArtifactUUID},
		[]string{"policy_uuid", "artifact_uuid"})
}

// ---- gateway_tokens ----

type GatewayTokenRow struct {
	UUID, GatewayUUID, TokenHash, Salt, Status string
	CreatedAt, RevokedAt                       *time.Time
	CreatedBy                                  string
}

func UpsertGatewayToken(ex Execer, r GatewayTokenRow, opts Options, rep Reporter) error {
	createdBy, err := audit(ex, opts, rep, "gateway_tokens", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	var revokedBy any
	if r.RevokedAt != nil {
		actor, _, e := ResolveIdentity(ex, MigrationActorIDPID, opts)
		if e != nil {
			return e
		}
		revokedBy = actor
	}
	return upsert(ex, opts, "gateway_tokens",
		[]string{"uuid", "gateway_uuid", "token_hash", "salt", "status", "data_version",
			"created_by", "created_at", "revoked_by", "revoked_at"},
		[]any{r.UUID, r.GatewayUUID, r.TokenHash, r.Salt, r.Status, DataVersion,
			createdBy, tsArg(r.CreatedAt, opts), revokedBy, tsArg(r.RevokedAt, opts)},
		[]string{"uuid"})
}

// ---- deployments ----

type DeploymentRow struct {
	UUID, DisplayName, ArtifactUUID, Org, GatewayUUID string
	BaseDeploymentUUID, Metadata                      *string
	Content                                           []byte
	CreatedAt                                         *time.Time
	CreatedBy                                         string
}

func UpsertDeployment(ex Execer, r DeploymentRow, opts Options, rep Reporter) error {
	createdBy, err := audit(ex, opts, rep, "deployments", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	return upsert(ex, opts, "deployments",
		[]string{"uuid", "display_name", "artifact_uuid", "organization_uuid", "gateway_uuid",
			"base_deployment_uuid", "content", "metadata", "data_version", "created_by", "created_at"},
		[]any{r.UUID, r.DisplayName, r.ArtifactUUID, r.Org, r.GatewayUUID, sarg(r.BaseDeploymentUUID),
			r.Content, bytesOrNilP(r.Metadata), DataVersion, createdBy, tsArg(r.CreatedAt, opts)},
		[]string{"uuid"})
}

// ---- deployment_status ----

type DeploymentStatusRow struct {
	ArtifactUUID, Org, GatewayUUID, DeploymentUUID, Status string
	StatusDesired, StatusReason                            *string
	PerformedAt, UpdatedAt                                 *time.Time
	PerformedBy                                            string
}

func UpsertDeploymentStatus(ex Execer, r DeploymentStatusRow, opts Options, rep Reporter) error {
	key := r.ArtifactUUID + "|" + r.Org + "|" + r.GatewayUUID
	performedBy, err := audit(ex, opts, rep, "deployment_status", key, r.PerformedBy)
	if err != nil {
		return err
	}
	return upsert(ex, opts, "deployment_status",
		[]string{"artifact_uuid", "organization_uuid", "gateway_uuid", "deployment_uuid", "status",
			"status_desired", "performed_at", "performed_by", "status_reason", "updated_at"},
		[]any{r.ArtifactUUID, r.Org, r.GatewayUUID, r.DeploymentUUID, r.Status, sarg(r.StatusDesired),
			tsArg(r.PerformedAt, opts), performedBy, sarg(r.StatusReason), tsArg(r.UpdatedAt, opts)},
		[]string{"organization_uuid", "artifact_uuid", "gateway_uuid"})
}

// ---- api_keys ----

type APIKeyRow struct {
	UUID, ArtifactUUID, Handle, DisplayName, MaskedKey, APIKeyHashes, Status, AllowedTargets string
	Issuer                                                                                   *string
	CreatedAt, UpdatedAt, ExpiresAt                                                          *time.Time
	CreatedBy                                                                                string
}

func UpsertAPIKey(ex Execer, r APIKeyRow, opts Options, rep Reporter) error {
	var issuerVal any
	if r.Issuer != nil {
		iv, cut := TruncateStr(*r.Issuer, 255)
		if cut {
			rep.Flag("api_keys", r.UUID, FlagTruncated, map[string]any{"issuer_len": len(*r.Issuer)}, map[string]any{"issuer_len": len(iv)})
		}
		issuerVal = iv
	}
	at, cut := TruncateStr(r.AllowedTargets, 255)
	if cut {
		rep.Flag("api_keys", r.UUID, FlagTruncated, map[string]any{"allowed_targets_len": len(r.AllowedTargets)}, map[string]any{"allowed_targets_len": len(at)})
	}
	createdBy, err := audit(ex, opts, rep, "api_keys", r.UUID, r.CreatedBy)
	if err != nil {
		return err
	}
	return upsert(ex, opts, "api_keys",
		[]string{"uuid", "artifact_uuid", "handle", "display_name", "masked_api_key", "api_key_hashes",
			"status", "data_version", "created_by", "created_at", "updated_by", "updated_at",
			"expires_at", "issuer", "allowed_targets"},
		[]any{r.UUID, r.ArtifactUUID, r.Handle, r.DisplayName, r.MaskedKey, []byte(r.APIKeyHashes), r.Status, DataVersion,
			createdBy, tsArg(r.CreatedAt, opts), createdBy, tsArg(r.UpdatedAt, opts), tsArg(r.ExpiresAt, opts), issuerVal, at},
		[]string{"uuid"})
}

// ---- application_api_key_mappings ----

type ApplicationAPIKeyMappingRow struct {
	ApplicationUUID, APIKeyID string
	CreatedAt                 *time.Time
	CreatedBy                 string
}

func UpsertApplicationAPIKeyMapping(ex Execer, r ApplicationAPIKeyMappingRow, opts Options, rep Reporter) error {
	key := r.ApplicationUUID + "|" + r.APIKeyID
	createdBy, err := audit(ex, opts, rep, "application_api_key_mappings", key, r.CreatedBy)
	if err != nil {
		return err
	}
	return upsert(ex, opts, "application_api_key_mappings",
		[]string{"application_uuid", "api_key_id", "created_by", "created_at"},
		[]any{r.ApplicationUUID, r.APIKeyID, createdBy, tsArg(r.CreatedAt, opts)},
		[]string{"application_uuid", "api_key_id"})
}

// ---- application_artifact_mappings ----

type ApplicationArtifactMappingRow struct {
	ApplicationUUID, ArtifactUUID string
	CreatedAt                     *time.Time
	CreatedBy                     string
}

func UpsertApplicationArtifactMapping(ex Execer, r ApplicationArtifactMappingRow, opts Options, rep Reporter) error {
	key := r.ApplicationUUID + "|" + r.ArtifactUUID
	createdBy, err := audit(ex, opts, rep, "application_artifact_mappings", key, r.CreatedBy)
	if err != nil {
		return err
	}
	return upsert(ex, opts, "application_artifact_mappings",
		[]string{"application_uuid", "artifact_uuid", "created_by", "created_at"},
		[]any{r.ApplicationUUID, r.ArtifactUUID, createdBy, tsArg(r.CreatedAt, opts)},
		[]string{"application_uuid", "artifact_uuid"})
}

// ---- shared helpers ----

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func lifecycleOr(p *string) string {
	if p != nil && *p != "" {
		return *p
	}
	return "CREATED"
}

func bytesOrNilP(p *string) any {
	if p == nil {
		return nil
	}
	return []byte(*p)
}

func flagPlaintextCredential(rep Reporter, table, key string, sec *model.SecurityConfig) {
	if sec != nil && sec.APIKey != nil && sec.APIKey.Key != "" {
		rep.Flag(table, key, FlagPlaintextCredential, nil,
			map[string]any{"note": "upstream apiKey.key stored in plaintext in the config blob"})
	}
}
