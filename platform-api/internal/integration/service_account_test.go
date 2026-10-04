/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// TestServiceAccount_LifecycleAndLedger checks the two new tables behave the
// same on every engine: the account write, the monotonic watermark upsert,
// the ledger outliving a delete, and pruning.
func TestServiceAccount_LifecycleAndLedger(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgID, _ := seedOrgProject(t, it, "sa")
	repo := repository.NewServiceAccountRepo(it.db)

	accountUUID := id()
	sa := &model.ServiceAccount{
		UUID: accountUUID, OrganizationID: orgID, Handle: "ci-bot-" + accountUUID[:6], DisplayName: "CI bot",
		Owner: "team", Description: "deploys", ClientID: "sa_it_" + accountUUID, ClientSecretHash: "h1",
		MaskedSecret: "***abcde", Roles: "ap_sa_reader",
	}
	if err := repo.Create(sa, sa.Subject("it")); err != nil {
		t.Fatalf("[%s] Create: %v", it.driver, err)
	}
	if err := repo.TouchLastUsed(accountUUID, time.Now(), "2001:db8::1"); err != nil {
		t.Fatalf("[%s] TouchLastUsed: %v", it.driver, err)
	}
	got, err := repo.GetByClientID(sa.ClientID)
	if err != nil || got.LastUsedIP != "2001:db8::1" || got.LastUsedAt == nil || got.Roles != "ap_sa_reader" {
		t.Fatalf("[%s] GetByClientID = %+v, %v", it.driver, got, err)
	}

	t0 := time.Now().UTC().Truncate(time.Millisecond)
	rev := func(minVersion int64) *model.ServiceAccountRevocation {
		return &model.ServiceAccountRevocation{AccountUUID: accountUUID, OrganizationID: orgID, MinTokenVersion: minVersion, ExpiresAt: t0.Add(time.Hour)}
	}
	got.ClientSecretHash, got.MaskedSecret, got.TokenVersion = "h2", "***fghij", 2
	if err := repo.UpdateSecret(got, 1, rev(2)); err != nil {
		t.Fatalf("[%s] UpdateSecret: %v", it.driver, err)
	}
	stale := *got
	stale.Status = model.ServiceAccountStatusDisabled
	if err := repo.Update(&stale, 1, model.ServiceAccountStatusActive, nil); !apperror.Conflict.Is(err) {
		t.Fatalf("[%s] stale Update: want conflict, got %v", it.driver, err)
	}
	if err := repo.Revoke(rev(1)); err != nil { // lower: must not win
		t.Fatalf("[%s] Revoke: %v", it.driver, err)
	}
	if err := repo.Delete(orgID, accountUUID, 2, rev(3)); err != nil {
		t.Fatalf("[%s] Delete: %v", it.driver, err)
	}
	if _, err := repo.GetByHandle(orgID, sa.Handle); !apperror.ServiceAccountNotFound.Is(err) {
		t.Fatalf("[%s] want not found after delete, got %v", it.driver, err)
	}

	active, err := repo.ListActive(t0)
	if err != nil {
		t.Fatalf("[%s] ListActive: %v", it.driver, err)
	}
	var found *model.ServiceAccountRevocation
	for _, r := range active {
		if r.AccountUUID == accountUUID {
			found = r
		}
	}
	if found == nil || found.MinTokenVersion != 3 {
		t.Fatalf("[%s] ledger row = %+v, want the latest watermark to survive the delete", it.driver, found)
	}
	if foreign, err := repo.ForeignReservedIdentities(); err != nil || len(foreign) != 0 {
		t.Fatalf("[%s] minted identity reported as foreign: %v, %v", it.driver, foreign, err)
	}
	if _, err := repo.PruneExpired(t0.Add(3 * time.Hour)); err != nil {
		t.Fatalf("[%s] PruneExpired: %v", it.driver, err)
	}
}
