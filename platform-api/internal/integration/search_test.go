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
	"sort"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// The `query` collection filter (openapi.yaml query-Q -> ListOptions.Search ->
// repository.handleSearchClause) must match the resource display name as well
// as its handle: the console lists resources by display name, and handles are
// URL-safe slugs users neither choose nor see. These tests run against a real
// engine because three properties of the filter cannot be observed from the
// clause string that pagination_test.go asserts on:
//
//   - Grouping. handleSearchClause returns a fragment; GatewayRepo.ListPaginated
//     strips the leading " AND " and joins it into its own AND-separated
//     condition list. An unparenthesized "a OR b" binds as
//     "(organization_uuid = ? AND display_name LIKE ?) OR handle LIKE ?" and
//     returns gateways from other organizations.
//   - Collation. SQL Server defaults to a case-insensitive collation while
//     PostgreSQL is case-sensitive; LOWER() on both sides is what makes the
//     filter behave identically, and only executing it proves that.
//   - List/count parity. ListXxx and CountXxx call handleSearchClause
//     independently, so a filtered page can pair with an unfiltered total.

// searchPageLimit is large enough that every seeded fixture lands on one page,
// keeping these tests about filtering rather than pagination.
const searchPageLimit = 1000

// seedSearchOrg inserts a bare organization and returns its UUID.
func seedSearchOrg(t *testing.T, it *itDB) string {
	t.Helper()
	org := id()
	it.exec(t, `INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid) VALUES (?, ?, ?, ?, ?)`,
		org, "so-"+org[:8], "search org", "us", "idp-ref")
	return org
}

// seedSearchGateway inserts a gateway with an explicit handle/display-name pair
// and returns its UUID. Seeding goes through SQL rather than GatewayRepo.Create
// so the two columns are written exactly as given — the divergence between them
// is the whole point of these fixtures. description is nullable in the schema
// but GatewayRepo scans it into a plain string, so it is written as ” here,
// matching what Create persists for a gateway with no description.
func seedSearchGateway(t *testing.T, it *itDB, orgID, handle, displayName string) string {
	t.Helper()
	gw := id()
	it.exec(t, `INSERT INTO gateways (uuid, organization_uuid, handle, display_name, description, properties) VALUES (?, ?, ?, ?, ?, ?)`,
		gw, orgID, handle, displayName, "", []byte("{}"))
	return gw
}

// seedSearchProject inserts a project with an explicit handle/display-name pair
// and returns its UUID. As above, description is written as ” rather than left
// NULL because ProjectRepo scans it into a plain string.
func seedSearchProject(t *testing.T, it *itDB, orgID, handle, displayName string) string {
	t.Helper()
	proj := id()
	it.exec(t, `INSERT INTO projects (uuid, handle, display_name, organization_uuid, description) VALUES (?, ?, ?, ?, ?)`,
		proj, handle, displayName, orgID, "")
	return proj
}

// searchGateways runs one search through GatewayRepo and returns the matched
// UUIDs, failing if the paged result and CountGateways disagree.
func searchGateways(t *testing.T, it *itDB, repo repository.GatewayRepository, orgID, term string) []string {
	t.Helper()
	page, err := repo.ListPaginated(orgID, repository.ListOptions{Limit: searchPageLimit, Search: term})
	if err != nil {
		t.Fatalf("[%s] ListPaginated(query=%q) failed: %v", it.driver, term, err)
	}
	total, err := repo.CountGateways(orgID, term)
	if err != nil {
		t.Fatalf("[%s] CountGateways(query=%q) failed: %v", it.driver, term, err)
	}
	if total != len(page) {
		t.Errorf("[%s] gateway query=%q: CountGateways=%d but the page holds %d rows — the list and count filters disagree",
			it.driver, term, total, len(page))
	}
	ids := make([]string, 0, len(page))
	for _, gw := range page {
		ids = append(ids, gw.ID)
	}
	return ids
}

// searchProjects runs one search through ProjectRepo and returns the matched
// UUIDs, failing if the paged result and CountProjects disagree.
func searchProjects(t *testing.T, it *itDB, repo repository.ProjectRepository, orgID, term string) []string {
	t.Helper()
	page, err := repo.ListProjects(orgID, repository.ListOptions{Limit: searchPageLimit, Search: term})
	if err != nil {
		t.Fatalf("[%s] ListProjects(query=%q) failed: %v", it.driver, term, err)
	}
	total, err := repo.CountProjects(orgID, term)
	if err != nil {
		t.Fatalf("[%s] CountProjects(query=%q) failed: %v", it.driver, term, err)
	}
	if total != len(page) {
		t.Errorf("[%s] project query=%q: CountProjects=%d but the page holds %d rows — the list and count filters disagree",
			it.driver, term, total, len(page))
	}
	ids := make([]string, 0, len(page))
	for _, p := range page {
		ids = append(ids, p.ID)
	}
	return ids
}

// assertMatches compares a search result against the exact set of expected
// UUIDs, order-independently.
func assertMatches(t *testing.T, it *itDB, term string, got []string, want ...string) {
	t.Helper()
	gotSorted := append([]string(nil), got...)
	wantSorted := append([]string(nil), want...)
	sort.Strings(gotSorted)
	sort.Strings(wantSorted)
	if len(gotSorted) != len(wantSorted) {
		t.Fatalf("[%s] query=%q matched %d rows %v, want %d %v", it.driver, term, len(gotSorted), gotSorted, len(wantSorted), wantSorted)
	}
	for i := range gotSorted {
		if gotSorted[i] != wantSorted[i] {
			t.Fatalf("[%s] query=%q matched %v, want %v", it.driver, term, gotSorted, wantSorted)
		}
	}
}

// TestSearch_GatewayQueryMatchesDisplayNameAndHandle drives GatewayRepo, the
// riskiest of the four searchable collections: it is the only one that
// hand-assembles its WHERE clause from the shared fragment, so it is where a
// missing OR group leaks rows across organizations.
func TestSearch_GatewayQueryMatchesDisplayNameAndHandle(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	repo := repository.NewGatewayRepo(it.db)

	orgA := seedSearchOrg(t, it)
	orgB := seedSearchOrg(t, it)

	// The handle is deliberately unrelated to the display name, so a
	// handle-only filter cannot match "Payment API" by coincidence.
	payment := seedSearchGateway(t, it, orgA, "legacy-svc-01", "Payment API Gateway")
	seedSearchGateway(t, it, orgA, "billing-gw", "Billing Gateway")
	// Same display name, different organization: the cross-tenant guard.
	otherOrgPayment := seedSearchGateway(t, it, orgB, "orgb-gw", "Payment API Gateway")

	t.Run("multi-word display name matches", func(t *testing.T) {
		// The reported bug: the console shows "Payment API Gateway", the user
		// types "Payment API", and a handle-only filter can never match it —
		// the space cannot line up with the hyphen in a slug.
		assertMatches(t, it, "Payment API", searchGateways(t, it, repo, orgA, "Payment API"), payment)
	})

	t.Run("match is case-insensitive across engines", func(t *testing.T) {
		assertMatches(t, it, "payment api", searchGateways(t, it, repo, orgA, "payment api"), payment)
		assertMatches(t, it, "PAYMENT API", searchGateways(t, it, repo, orgA, "PAYMENT API"), payment)
	})

	t.Run("handle search is preserved", func(t *testing.T) {
		// Existing id-based callers (CLI, scripts) must keep working.
		assertMatches(t, it, "legacy-svc", searchGateways(t, it, repo, orgA, "legacy-svc"), payment)
	})

	t.Run("search does not cross organizations", func(t *testing.T) {
		// If the OR group is unparenthesized, "organization_uuid = ? AND
		// display_name LIKE ? OR handle LIKE ?" binds as
		// "(org AND display_name) OR handle" and orgB's gateway appears here.
		assertMatches(t, it, "Payment API", searchGateways(t, it, repo, orgA, "Payment API"), payment)
		assertMatches(t, it, "Payment API", searchGateways(t, it, repo, orgB, "Payment API"), otherOrgPayment)
		// Same check via the handle leg, which is the term the bad grouping
		// would leave unscoped by organization.
		assertMatches(t, it, "orgb-gw", searchGateways(t, it, repo, orgA, "orgb-gw"))
	})

	t.Run("non-matching term returns nothing", func(t *testing.T) {
		assertMatches(t, it, "no-such-resource", searchGateways(t, it, repo, orgA, "no-such-resource"))
	})

	t.Run("wildcard metacharacters stay literal", func(t *testing.T) {
		// ESCAPE '\' must hold on every engine: a bare "%" is a literal to
		// match, not a wildcard that returns the whole collection.
		assertMatches(t, it, "%", searchGateways(t, it, repo, orgA, "%"))
		assertMatches(t, it, "_", searchGateways(t, it, repo, orgA, "_"))
	})

	t.Run("empty term does not filter", func(t *testing.T) {
		all := searchGateways(t, it, repo, orgA, "")
		if len(all) != 2 {
			t.Fatalf("[%s] empty query returned %d gateways, want both seeded rows", it.driver, len(all))
		}
	})
}

// TestSearch_ProjectQueryMatchesDisplayNameAndHandle covers the other clause
// shape: ProjectRepo appends the fragment to an existing WHERE, so it stands in
// for rest_apis and applications, which assemble their queries the same way.
func TestSearch_ProjectQueryMatchesDisplayNameAndHandle(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	repo := repository.NewProjectRepo(it.db)

	orgA := seedSearchOrg(t, it)
	orgB := seedSearchOrg(t, it)

	payment := seedSearchProject(t, it, orgA, "legacy-proj-01", "Payment API Project")
	seedSearchProject(t, it, orgA, "billing-proj", "Billing Project")
	otherOrgPayment := seedSearchProject(t, it, orgB, "orgb-proj", "Payment API Project")

	t.Run("multi-word display name matches", func(t *testing.T) {
		assertMatches(t, it, "Payment API", searchProjects(t, it, repo, orgA, "Payment API"), payment)
	})

	t.Run("handle search is preserved", func(t *testing.T) {
		assertMatches(t, it, "legacy-proj", searchProjects(t, it, repo, orgA, "legacy-proj"), payment)
	})

	t.Run("search does not cross organizations", func(t *testing.T) {
		assertMatches(t, it, "Payment API", searchProjects(t, it, repo, orgB, "Payment API"), otherOrgPayment)
		assertMatches(t, it, "orgb-proj", searchProjects(t, it, repo, orgA, "orgb-proj"))
	})

	t.Run("wildcard metacharacters stay literal", func(t *testing.T) {
		assertMatches(t, it, "%", searchProjects(t, it, repo, orgA, "%"))
	})
}
