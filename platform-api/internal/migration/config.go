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

// Package migration implements the one-shot platform-api v1 -> v2 database
// migration client. It reads the v1 Postgres schema, transforms every row per
// db-migration-client/DB_MAPPING.md, and writes the v2 schema (including the
// EventGateway plugin tables).
//
// It lives inside the v2 module on purpose: the transform reuses v2's own
// internal/ helpers (deterministic uuidv7, handle slugging, vault AES-GCM/HMAC,
// key derivation) by import, so the migrated bytes are identical to what v2's
// own create path would have produced. See DB_MAPPING.md §8 for the rationale.
package migration

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Direction selects the flow of a run.
type Direction string

const (
	// DirectionForward is the primary v1 -> v2 migration.
	DirectionForward Direction = "forward"
	// DirectionReverse reconciles post-cutover v2 changes back into v1 (best-effort, §13).
	DirectionReverse Direction = "reverse"
)

// DefaultMigrationActorUUID is the synthetic actor stamped into v2 NEW audit
// columns (updated_by, revoked_by, performed_by, …) that have no v1 source
// (DB_MAPPING.md §B.4).
const DefaultMigrationActorUUID = "019b76da-a800-7a6c-aad4-385ed4394247"

// Env var names — every password / key is read ONLY from the environment,
// never a CLI flag (flags leak into `ps` and shell history), and never logged.
const (
	EnvV1DBPassword    = "V1_DB_PASSWORD"
	EnvV2DBPassword    = "V2_DB_PASSWORD"
	EnvV2EncryptionKey = "V2_ENCRYPTION_KEY"
)

// DBConn is one Postgres endpoint. The password is intentionally absent — it is
// pulled from the environment at connect time and held only transiently.
type DBConn struct {
	Host    string
	Port    int
	Name    string
	User    string
	SSLMode string // require | verify-full | disable | …

	passwordEnv string // env var to read the password from (not the password itself)
}

// Config is the fully-resolved run configuration.
type Config struct {
	V1 DBConn // source
	V2 DBConn // target

	// EncryptionKey is v2's security.encryption_key — the SINGLE consolidated key
	// used for ALL at-rest crypto (subscription-token decrypt, secret-vault
	// AES-GCM, secrets.hash HMAC). Per the Gate-2 decision these bytes ARE v1's
	// subscription-token key, so copied token ciphertext decrypts by design.
	// 32 bytes, decoded from 64-hex or base64 by utils.DeriveEncryptionKey.
	// Held here transiently; never logged.
	EncryptionKey []byte

	// V1Location is the v1 *app server's* local zone (the Go-process time.Local).
	// pgx stores a bare time.Now() as the app's local wall-clock digits with the
	// zone discarded, so tsToTstz interprets the ~130 app-bound TIMESTAMP columns
	// in this zone before converting to UTC (DB_MAPPING.md §B.3). The 4 .UTC()
	// tables key off the writer, not this zone.
	V1Location     *time.Location
	V1TimezoneName string

	MigrationActorUUID string

	// OrgHandles / ProjectHandles are the AUTHORITATIVE Choreo handle maps
	// (§B.14) loaded from --org-handles-csv / --project-handles-csv. v2 (REST
	// path ids + the AI Workspace URL) addresses orgs and projects BY HANDLE and
	// v1's values are not Choreo's, so a forward run requires both. nil for a
	// reverse run or a bare --verify-only (which then skips handle parity).
	OrgHandlesCSV, ProjectHandlesCSV string
	// HandlesReport is the CSV path the runner writes every organization/project
	// uuid → v2 handle (+ source) to at the end of a forward run, dry-run included.
	HandlesReport              string
	OrgHandles, ProjectHandles *HandleMap

	Direction Direction

	DryRun          bool
	VerifyOnly      bool
	Resume          bool
	CheckpointFile  string
	ValidateKeys    bool
	ContinueOnError bool

	// Resources / Skip are optional per-resource selectors (default = all, FK
	// order). Not required for a correct full run; operator convenience for
	// per-resource dry-run, targeted re-migration, staged rollout. A selection is
	// FK-dependency-checked before running.
	Resources []string
	Skip      []string

	// BatchSize is an optional throughput / Prod-pressure knob (rows per tx
	// batch). Inserts are idempotent so any size is correct; never 1.
	BatchSize int

	LogLevel  string
	LogFormat string // json | text
}

// DSN builds a database/sql connection string for the pgx stdlib driver,
// reading the password from the endpoint's env var. The returned string
// contains the password and MUST NOT be logged.
func (c DBConn) DSN() (string, error) {
	if c.Host == "" || c.Name == "" || c.User == "" {
		return "", fmt.Errorf("db connection requires host, db and user (got host=%q db=%q user=%q)", c.Host, c.Name, c.User)
	}
	pw := os.Getenv(c.passwordEnv)
	if pw == "" {
		return "", fmt.Errorf("environment variable %s is required (passwords are never passed as flags)", c.passwordEnv)
	}
	ssl := c.SSLMode
	if ssl == "" {
		ssl = "require"
	}
	// key=value DSN form; pgx stdlib parses it. TimeZone is deliberately NOT set
	// here — the v1 zone handling is done in Go (tsToTstz), not by the session.
	// Every string value is quoted so whitespace, quotes and backslashes in a
	// password (or user/db name) stay part of the value.
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		dsnQuote(c.Host), c.Port, dsnQuote(c.Name), dsnQuote(c.User), dsnQuote(pw), dsnQuote(ssl),
	), nil
}

// dsnQuote renders v as a single-quoted libpq key/value DSN value, escaping
// backslashes and single quotes.
func dsnQuote(v string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(v) + "'"
}

// Redacted returns a log-safe description of the endpoint (no password).
func (c DBConn) Redacted() string {
	return fmt.Sprintf("%s@%s:%d/%s sslmode=%s", c.User, c.Host, c.Port, c.Name, c.SSLMode)
}

// Validate checks the resolved config for the invariants the run depends on.
func (c *Config) Validate() error {
	if c.Direction != DirectionForward && c.Direction != DirectionReverse {
		return fmt.Errorf("--direction must be forward or reverse, got %q", c.Direction)
	}
	if _, err := c.V1.DSN(); err != nil {
		return fmt.Errorf("v1 (source) connection: %w", err)
	}
	if !c.DryRun && !c.VerifyOnly {
		if _, err := c.V2.DSN(); err != nil {
			return fmt.Errorf("v2 (target) connection: %w", err)
		}
	}
	if c.V1Location == nil {
		return fmt.Errorf("--v1-timezone is required (v1 timestamps are not all UTC; §B.3) — " +
			"set it to the v1 app server's zone, or 'UTC' only if the v1 fleet provably ran TZ=UTC")
	}
	if c.MigrationActorUUID == "" {
		return fmt.Errorf("--migration-actor-uuid must not be empty")
	}
	// The Choreo handle maps are mandatory for any forward run (dry-run
	// included): without them every org/project handle would be v1's random
	// string and every AI Workspace v2 URL would break (§B.14 / §B.0 Gate 5).
	if c.Direction == DirectionForward && !c.VerifyOnly {
		if c.OrgHandles == nil {
			return fmt.Errorf("--org-handles-csv is required for a forward run: v2 addresses organizations by handle and v1's handles are not Choreo's (§B.14 / §B.0 Gate 5)")
		}
		if c.ProjectHandles == nil {
			return fmt.Errorf("--project-handles-csv is required for a forward run: v2 addresses projects by handle and v1 has none (§B.14 / §B.0 Gate 5)")
		}
	}
	// The encryption key is required for any real forward write (secret
	// externalization + token parity smoke-test) and for reverse (secret
	// re-inline). Only a source-only dry-run/verify can proceed without it, but we
	// still warn there.
	if len(c.EncryptionKey) == 0 && !c.DryRun && !c.VerifyOnly {
		return fmt.Errorf("%s is required (v2's single at-rest key; §B.0 Gates 2 & 4)", EnvV2EncryptionKey)
	}
	if len(c.EncryptionKey) != 0 && len(c.EncryptionKey) != 32 {
		return fmt.Errorf("%s must decode to 32 bytes, got %d", EnvV2EncryptionKey, len(c.EncryptionKey))
	}
	if c.BatchSize < 0 {
		return fmt.Errorf("--batch-size must be >= 0 (0 = default)")
	}
	if c.LogFormat != "" && c.LogFormat != "json" && c.LogFormat != "text" {
		return fmt.Errorf("--log-format must be json or text, got %q", c.LogFormat)
	}
	return nil
}

// EffectiveBatchSize returns the batch size to use, applying the default when
// unset. Never returns 1 — a commit per row is needlessly slow (§12).
func (c *Config) EffectiveBatchSize() int {
	if c.BatchSize <= 1 {
		return 750 // middle of the ~500–1000 default band
	}
	return c.BatchSize
}

// selected reports whether a resource name is in scope given --resources / --skip.
func (c *Config) selected(name string) bool {
	for _, s := range c.Skip {
		if strings.EqualFold(strings.TrimSpace(s), name) {
			return false
		}
	}
	if len(c.Resources) == 0 {
		return true
	}
	for _, r := range c.Resources {
		if strings.EqualFold(strings.TrimSpace(r), name) {
			return true
		}
	}
	return false
}
