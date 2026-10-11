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

package migration

import "context"

func init() {
	register(&secretsMigrator{baseMigrator{name: "secrets", dependsOn: []string{"artifacts"}}})
}

// secretsMigrator is a verify-only pass. The secrets / secret_scopes /
// artifact_secret_refs rows are written INLINE while each artifact's config is
// externalized (§B.10), so there is nothing to migrate as a separate step — this
// migrator only reports integrity: every artifact_secret_refs.secret_handle must
// resolve to a secrets row, and every secret must have an org scope.
type secretsMigrator struct{ baseMigrator }

func (m *secretsMigrator) Migrate(_ context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	rep.Status = StatusSkip
	rep.addf("secrets are emitted inline during artifact externalization (§B.10) — no separate pass")
	return rep, nil
}

func (m *secretsMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	if rc.Tgt == nil || rc.Direction == DirectionReverse {
		// v2-only tables (secrets / scopes / refs): in reverse the target is v1.
		rep.Status = StatusSkip
		if rc.Direction == DirectionReverse {
			rep.addf("reverse: v2-only tables, nothing to verify in v1 (secret re-inline is unsupported, §13)")
		}
		return rep, nil
	}
	if n, err := scanCount(ctx, rc.Tgt, "SELECT COUNT(*) FROM secrets"); err == nil {
		rep.TgtCount = n
	}
	// Every artifact_secret_refs handle must resolve to a secrets row (org-scoped).
	dangling, err := scanCount(ctx, rc.Tgt, `
		SELECT COUNT(*) FROM artifact_secret_refs r
		LEFT JOIN secrets s ON s.organization_uuid = r.organization_uuid AND s.handle = r.secret_handle
		WHERE s.uuid IS NULL`)
	if err != nil {
		rep.warn("dangling-ref check failed: %v", err)
		return rep, nil
	}
	if dangling > 0 {
		rep.fail("%d artifact_secret_refs point to a missing secrets row", dangling)
	}
	// Every secret must have at least one scope row.
	scopeless, err := scanCount(ctx, rc.Tgt, `
		SELECT COUNT(*) FROM secrets s
		LEFT JOIN secret_scopes sc ON sc.secret_uuid = s.uuid
		WHERE sc.secret_uuid IS NULL`)
	if err == nil && scopeless > 0 {
		rep.warn("%d secrets have no scope row", scopeless)
	}
	return rep, nil
}
