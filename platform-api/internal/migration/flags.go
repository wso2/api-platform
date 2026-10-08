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
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// ParseFlags builds a Config from CLI args and the environment. Passwords and
// the encryption key are read ONLY from the environment (V1_DB_PASSWORD,
// V2_DB_PASSWORD, V2_ENCRYPTION_KEY) — never from a flag.
func ParseFlags(args []string, out io.Writer) (*Config, error) {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(out)

	var (
		v1Host = fs.String("v1-host", "", "v1 (source) Postgres host")
		v1Port = fs.Int("v1-port", 5432, "v1 Postgres port")
		v1DB   = fs.String("v1-db", "", "v1 database name")
		v1User = fs.String("v1-user", "", "v1 database user")
		v1SSL  = fs.String("v1-sslmode", "require", "v1 sslmode: require|verify-full|disable|…")

		v2Host = fs.String("v2-host", "", "v2 (target) Postgres host")
		v2Port = fs.Int("v2-port", 5432, "v2 Postgres port")
		v2DB   = fs.String("v2-db", "", "v2 database name")
		v2User = fs.String("v2-user", "", "v2 database user")
		v2SSL  = fs.String("v2-sslmode", "require", "v2 sslmode: require|verify-full|disable|…")

		v1TZ = fs.String("v1-timezone", "", "REQUIRED: v1 app server IANA zone (e.g. Asia/Colombo); "+
			"'UTC' only if the v1 fleet provably ran TZ=UTC (§B.3)")
		orgCSV = fs.String("org-handles-csv", "", "REQUIRED (forward): Choreo ORG handle export CSV — org_uuid,org_handle[,org_status,org_name] "+
			"(cr/test/export-org-handles.sh). v2 addresses orgs by handle and v1's handles are not Choreo's (§B.14)")
		projCSV = fs.String("project-handles-csv", "", "REQUIRED (forward): Choreo PROJECT handle export CSV — project_uuid,project_handler "+
			"(cr/test/export-project-handlers.js). Projects Choreo does not know fall back to a handle minted from the v1 name (§B.14)")

		actor = fs.String("migration-actor-uuid", DefaultMigrationActorUUID,
			"synthetic actor UUID for v2 NEW audit columns with no v1 source (§B.4)")

		direction  = fs.String("direction", string(DirectionForward), "forward (v1→v2) | reverse (v2→v1, §13)")
		dryRun     = fs.Bool("dry-run", false, "run the full transform + verify with ZERO writes")
		verifyOnly = fs.Bool("verify-only", false, "run §9 verification against an already-migrated target, no writes")
		resume     = fs.Bool("resume", false, "resume a prior run, reusing checkpointed minted handles/UUIDs")
		ckpt       = fs.String("checkpoint-file", "migration-checkpoint.json", "checkpoint store path (minted handles/UUIDs + progress)")
		validateK  = fs.Bool("validate-keys", true, "preflight: sample-decrypt a real v1 token + round-trip encrypt/HMAC; abort on failure (§B.0 Gates 2 & 4)")
		contOnErr  = fs.Bool("continue-on-error", false, "do not fail-fast; log and continue past a failed row/batch")

		resources = fs.String("resources", "", "optional CSV of resources to include (default = all, FK order §7)")
		skip      = fs.String("skip", "", "optional CSV of resources to skip")
		batch     = fs.Int("batch-size", 0, "optional rows per tx batch (0 = default ~750; never 1)")

		logLevel  = fs.String("log-level", "info", "debug|info|warn|error")
		logFormat = fs.String("log-format", "text", "json|text")

		handlesReport = fs.String("handles-report", "migration-handles.csv", "CSV written at the end of every forward run (dry-run too): "+
			"one row per organization/project with uuid, organization_uuid, v1 value, v2 handle, display name and the handle's source "+
			"(choreo-export | v1-handle-kept | minted-from-v1-name[-suffixed]) — the non-Choreo rows are the ones to resolve (§B.14)")

		showVersion = fs.Bool("version", false, "print build information (version, commit, build date) and exit")
	)

	fs.Usage = func() {
		fmt.Fprintf(out, "Usage: migrate [flags]\n\n"+
			"platform-api v1 → v2 database migration client.\n"+
			"Build: %s\n"+
			"Passwords/keys come from the environment ONLY: %s, %s, %s.\n\nFlags:\n",
			BuildInfo(), EnvV1DBPassword, EnvV2DBPassword, EnvV2EncryptionKey)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if *showVersion {
		return nil, ErrVersionRequested
	}

	cfg := &Config{
		V1: DBConn{Host: *v1Host, Port: *v1Port, Name: *v1DB, User: *v1User, SSLMode: *v1SSL, passwordEnv: EnvV1DBPassword},
		V2: DBConn{Host: *v2Host, Port: *v2Port, Name: *v2DB, User: *v2User, SSLMode: *v2SSL, passwordEnv: EnvV2DBPassword},

		MigrationActorUUID: *actor,
		Direction:          Direction(*direction),
		DryRun:             *dryRun,
		VerifyOnly:         *verifyOnly,
		Resume:             *resume,
		CheckpointFile:     *ckpt,
		ValidateKeys:       *validateK,
		ContinueOnError:    *contOnErr,
		Resources:          splitCSV(*resources),
		Skip:               splitCSV(*skip),
		BatchSize:          *batch,
		LogLevel:           *logLevel,
		LogFormat:          *logFormat,
		V1TimezoneName:     *v1TZ,
		OrgHandlesCSV:      *orgCSV,
		ProjectHandlesCSV:  *projCSV,
		HandlesReport:      *handlesReport,
	}

	// Resolve the v1 timezone (required).
	if *v1TZ != "" {
		loc, err := time.LoadLocation(*v1TZ)
		if err != nil {
			return nil, fmt.Errorf("--v1-timezone %q is not a valid IANA zone: %w", *v1TZ, err)
		}
		cfg.V1Location = loc
	}

	// Load the Choreo handle maps (§B.14). Validate() enforces that a forward
	// run has both, so a missing flag is reported by name.
	if *orgCSV != "" {
		m, err := LoadOrgHandleMap(*orgCSV)
		if err != nil {
			return nil, fmt.Errorf("--org-handles-csv: %w", err)
		}
		cfg.OrgHandles = m
	}
	if *projCSV != "" {
		m, err := LoadProjectHandleMap(*projCSV)
		if err != nil {
			return nil, fmt.Errorf("--project-handles-csv: %w", err)
		}
		cfg.ProjectHandles = m
	}

	// Resolve the encryption key from the environment (never a flag).
	if raw := os.Getenv(EnvV2EncryptionKey); raw != "" {
		key, err := utils.DeriveEncryptionKey(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", EnvV2EncryptionKey, err)
		}
		cfg.EncryptionKey = key
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
