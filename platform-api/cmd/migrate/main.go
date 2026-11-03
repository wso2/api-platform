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

// Command migrate is the platform-api v1 -> v2 database migration client.
//
// It reads the v1 Postgres schema and writes the transformed rows into the v2
// schema (including the EventGateway plugin), reusing v2's own internal/ helpers
// for deterministic UUIDs, handle slugging, and vault crypto so the migrated
// bytes match what v2's create path would have produced.
//
// Passwords and the encryption key are read from the environment only
// (V1_DB_PASSWORD, V2_DB_PASSWORD, V2_ENCRYPTION_KEY) — never CLI flags.
//
// Run --dry-run first, then a real run. See internal/migration/README.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/wso2/api-platform/platform-api/internal/migration"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := migration.ParseFlags(os.Args[1:], os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		if errors.Is(err, migration.ErrVersionRequested) {
			fmt.Println("migrate", migration.BuildInfo())
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(2)
	}

	if err := migration.Run(ctx, cfg); err != nil {
		fmt.Fprintln(os.Stderr, "migration failed:", err)
		os.Exit(1)
	}
}
