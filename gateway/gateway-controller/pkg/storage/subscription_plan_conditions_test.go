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

package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"gotest.tools/v3/assert"
)

// A plan's conditions are stored and read back as they are: none recorded (nil) stays nil, an empty
// list stays empty, and an update replaces them.
func TestSubscriptionPlan_ConditionsRoundTrip(t *testing.T) {
	store := setupTestStorage(t)
	defer store.db.Close()

	conditions := []models.SubscriptionPlanCondition{
		{ID: "requests", Type: "request-count", Params: []byte(`{"limits":[{"count":100,"timeAmount":1,"timeUnit":"MINUTE"}],"onExceed":"reject"}`)},
		{ID: "ip-ranges", Type: "custom", Params: []byte(`{"cidr":"10.0.0.0/8"}`)},
	}
	for _, plan := range []*models.SubscriptionPlan{
		{ID: "p-gold", PlanName: "Gold", Status: models.SubscriptionPlanStatusActive, Conditions: conditions},
		{ID: "p-free", PlanName: "Free", Status: models.SubscriptionPlanStatusActive, Conditions: []models.SubscriptionPlanCondition{}},
		{ID: "p-old", PlanName: "Old", Status: models.SubscriptionPlanStatusActive},
	} {
		assert.NilError(t, store.SaveSubscriptionPlan(plan))
	}

	gold, err := store.GetSubscriptionPlanByID("p-gold", "")
	assert.NilError(t, err)
	assert.Equal(t, len(gold.Conditions), 2)
	assert.Equal(t, gold.Conditions[1].ID, "ip-ranges")
	assert.Equal(t, string(gold.Conditions[1].Params), `{"cidr":"10.0.0.0/8"}`)

	plans, err := store.ListSubscriptionPlans("")
	assert.NilError(t, err)
	byID := map[string]*models.SubscriptionPlan{}
	for _, p := range plans {
		byID[p.ID] = p
	}
	assert.Assert(t, byID["p-free"].Conditions != nil && len(byID["p-free"].Conditions) == 0, "an empty list must stay empty")
	assert.Assert(t, byID["p-old"].Conditions == nil, "a plan with no conditions recorded must stay nil")

	gold.Conditions = conditions[1:]
	assert.NilError(t, store.UpdateSubscriptionPlan(gold))
	gold, err = store.GetSubscriptionPlanByID("p-gold", "")
	assert.NilError(t, err)
	assert.Equal(t, len(gold.Conditions), 1)
	assert.Equal(t, gold.Conditions[0].ID, "ip-ranges")

	var column sql.NullString
	assert.NilError(t, store.db.QueryRow(`SELECT conditions FROM subscription_plans WHERE uuid = 'p-old'`).Scan(&column))
	assert.Assert(t, !column.Valid, "no conditions recorded is stored as NULL")
}

// A version 6 database gets the conditions column; its plans keep their throttle limit and read with
// no conditions recorded.
func TestSQLite_UpgradeFromV6AddsPlanConditions(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "v6.db")
	createReleasedV5Database(t, path)

	rawDB, err := sql.Open("sqlite3", path)
	assert.NilError(t, err)
	for _, statement := range []string{
		`ALTER TABLE certificates ADD COLUMN usage TEXT NOT NULL DEFAULT 'upstream'`,
		`ALTER TABLE certificates ADD COLUMN role TEXT NOT NULL DEFAULT 'client'`,
		`ALTER TABLE certificates ADD COLUMN match_json TEXT`,
		`ALTER TABLE certificates ADD COLUMN private_key_ciphertext TEXT`,
		`ALTER TABLE certificates ADD COLUMN key_algorithm TEXT`,
		`INSERT INTO subscription_plans (uuid, gateway_id, plan_name, stop_on_quota_reach, throttle_limit_count, throttle_limit_unit, status)
			VALUES ('p-gold', 'platform-gateway-id', 'Gold', 1, 50, 'Min', 'ACTIVE')`,
		fmt.Sprintf("PRAGMA user_version = %d", conditionsSchemaVersion),
	} {
		_, err = rawDB.Exec(statement)
		assert.NilError(t, err, statement)
	}
	assert.NilError(t, rawDB.Close())

	upgraded, err := NewStorage(BackendConfig{Type: "sqlite", SQLitePath: path, GatewayID: "platform-gateway-id"},
		slog.New(slog.NewTextHandler(os.Stdout, nil)))
	assert.NilError(t, err)
	defer upgraded.(*sqlStore).db.Close()

	var version int
	assert.NilError(t, upgraded.(*sqlStore).db.QueryRow("PRAGMA user_version").Scan(&version))
	assert.Equal(t, version, currentSchemaVersion)

	gold, err := upgraded.GetSubscriptionPlanByID("p-gold", "")
	assert.NilError(t, err)
	assert.Assert(t, gold.Conditions == nil)
	assert.Equal(t, *gold.ThrottleLimitCount, 50)
	effective := gold.EffectiveConditions()
	assert.Equal(t, len(effective), 1)
	assert.Equal(t, string(effective[0].Params), `{"limits":[{"count":50,"timeAmount":1,"timeUnit":"MINUTE"}],"onExceed":"reject"}`)
}

// A conditions column that can't be read fails the read of that plan, rather than reading it as no
// conditions.
func TestSubscriptionPlan_UnreadableConditionsColumn(t *testing.T) {
	store := setupTestStorage(t)
	defer store.db.Close()
	assert.NilError(t, store.SaveSubscriptionPlan(&models.SubscriptionPlan{ID: "p-bad", PlanName: "Bad", Status: models.SubscriptionPlanStatusActive}))
	_, err := store.db.Exec(`UPDATE subscription_plans SET conditions = 'not json' WHERE uuid = 'p-bad'`)
	assert.NilError(t, err)

	_, err = store.GetSubscriptionPlanByID("p-bad", "")
	assert.ErrorContains(t, err, "unreadable conditions")
	_, err = store.ListSubscriptionPlans("")
	assert.ErrorContains(t, err, "unreadable conditions")
}
