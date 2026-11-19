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

package platformgateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cucumber/godog"

	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// registerPlanStorageSteps binds the steps that read how gateway-controller stored a subscription
// plan's conditions.
func (g *Gateway) registerPlanStorageSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the gateway database should store subscription plan "([^"]*)" without conditions, as gateways before plan conditions stored every plan$`,
		g.planStoredWithoutConditions)
	sc.Step(`^the gateway database should store (\d+) conditions? for subscription plan "([^"]*)"$`,
		g.planStoredConditions)
}

// storedPlanConditions reads the plan's conditions column: NULL for a plan stored before plan
// conditions, or a JSON array.
func (g *Gateway) storedPlanConditions(ctx context.Context, planName string) (sql.NullString, error) {
	store, ok := g.topo.Storage.Plan.StoreFor("platform-gateway", 0)
	if !ok {
		return sql.NullString{}, errors.New("platform-gateway has no assigned store")
	}
	db, closeDB, err := g.topo.OpenComponentDB(ctx, "platform-gateway", "gateway-controller")
	if err != nil {
		return sql.NullString{}, err
	}
	defer func() { _ = closeDB() }()
	p1, _ := sqlPlaceholders(store.Type)
	var conditions sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT conditions FROM subscription_plans WHERE plan_name = "+p1, planName).Scan(&conditions); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.NullString{}, fmt.Errorf("no subscription plan named %q is stored", planName)
		}
		return sql.NullString{}, fmt.Errorf("reading the stored conditions of plan %q: %w", planName, err)
	}
	return conditions, nil
}

func (g *Gateway) planStoredWithoutConditions(ctx context.Context, planName string) error {
	planName, err := stepscommon.Expand(ctx, planName)
	if err != nil {
		return err
	}
	conditions, err := g.storedPlanConditions(ctx, planName)
	if err != nil {
		return err
	}
	if conditions.Valid {
		return fmt.Errorf("plan %q is stored with conditions %s, want none (NULL)", planName, conditions.String)
	}
	return nil
}

func (g *Gateway) planStoredConditions(ctx context.Context, want int, planName string) error {
	planName, err := stepscommon.Expand(ctx, planName)
	if err != nil {
		return err
	}
	conditions, err := g.storedPlanConditions(ctx, planName)
	if err != nil {
		return err
	}
	if !conditions.Valid {
		return fmt.Errorf("plan %q is stored without conditions (NULL), want %d", planName, want)
	}
	var list []json.RawMessage
	if err := json.Unmarshal([]byte(conditions.String), &list); err != nil {
		return fmt.Errorf("plan %q has unreadable stored conditions %q: %w", planName, conditions.String, err)
	}
	if len(list) != want {
		return fmt.Errorf("plan %q is stored with %d conditions, want %d: %s", planName, len(list), want, conditions.String)
	}
	return nil
}
