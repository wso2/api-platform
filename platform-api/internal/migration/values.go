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

import (
	"database/sql"
	"strings"
	"time"
)

// nullOrString returns the string value or nil for an INSERT bind, so a v1 NULL
// stays NULL in v2 instead of becoming an empty string.
func nullOrString(ns sql.NullString) any {
	if ns.Valid {
		return ns.String
	}
	return nil
}

// nullOrEmpty returns nil for an empty string (so a NULL column stays NULL),
// otherwise the string. Used on the reverse path where a recovered actor may be "".
func nullOrEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// textToBytea copies a v1 TEXT column into a v2 BYTEA column as an opaque byte
// copy (api_key_hashes, etc. — §B.1), preserving NULL.
func textToBytea(ns sql.NullString) any {
	if !ns.Valid {
		return nil
	}
	return []byte(ns.String)
}

// nullOrBytes returns the bytes or nil for an INSERT bind, so a v1 NULL/empty
// blob stays NULL rather than an empty byte string.
func nullOrBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

// nullTimePtr converts a sql.NullTime to *time.Time (nil when NULL).
func nullTimePtr(nt sql.NullTime) *time.Time {
	if nt.Valid {
		return &nt.Time
	}
	return nil
}

// tstzPtrArg converts a nullable naive v1 timestamp to a target-ready value
// (nil-safe), applying the zone conversion.
func tstzPtrArg(nt sql.NullTime, zone *time.Location) any {
	if p := tsToTstzPtr(nullTimePtr(nt), zone); p != nil {
		return *p
	}
	return nil
}

// mapThrottleUnit maps a v1 throttle_limit_unit to a v2 subscription_plan_limits
// time_unit (§ plans §6: Min→MINUTE, Hour→HOUR, Day→DAY, Month→MONTH). Unknown
// values pass through upper-cased.
func mapThrottleUnit(ns sql.NullString) string {
	if !ns.Valid {
		return "MINUTE"
	}
	switch strings.ToLower(strings.TrimSpace(ns.String)) {
	case "min", "minute", "minutes":
		return "MINUTE"
	case "hour", "hours":
		return "HOUR"
	case "day", "days":
		return "DAY"
	case "month", "months":
		return "MONTH"
	default:
		return strings.ToUpper(ns.String)
	}
}

// boolToSmallint maps a v1 BOOLEAN to a v2 SMALLINT (true->1, false/NULL->0).
func boolToSmallint(nb sql.NullBool) int {
	if nb.Valid && nb.Bool {
		return 1
	}
	return 0
}
