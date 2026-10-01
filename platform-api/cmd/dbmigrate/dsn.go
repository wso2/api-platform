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

package main

import (
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/database"
)

// dsnToConfig parses a PostgreSQL URL DSN (postgres://user:pass@host:port/db?sslmode=...)
// into the config.Database struct that database.NewConnection expects. Only the
// PostgreSQL driver is in scope for this tool.
func dsnToConfig(dsn string) (*config.Database, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		// Do NOT wrap err — url.Parse echoes the raw input, which includes the password.
		return nil, fmt.Errorf("could not parse DSN (expected postgres://user:pass@host:port/db?sslmode=...)")
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return nil, fmt.Errorf("unsupported DSN scheme %q (only postgres:// is supported)", u.Scheme)
	}

	port := 5432
	if p := u.Port(); p != "" {
		port, err = strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid port %q: %w", p, err)
		}
	}

	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		return nil, fmt.Errorf("DSN is missing a database name")
	}

	sslMode := u.Query().Get("sslmode")
	if sslMode == "" {
		sslMode = "disable"
	}

	pass, _ := u.User.Password()
	return &config.Database{
		Driver:          database.DriverPostgres,
		Host:            u.Hostname(),
		Port:            port,
		Name:            name,
		User:            u.User.Username(),
		Password:        pass,
		SSLMode:         sslMode,
		SSLRootCert:     u.Query().Get("sslrootcert"),
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 0,
	}, nil
}

// openDB opens a *database.DB for the given DSN, reusing the product's connection
// helper so we inherit its pgx wiring, Rebind, and IsDuplicateKeyError behavior.
func openDB(dsn string, logger *slog.Logger) (*database.DB, error) {
	cfg, err := dsnToConfig(dsn)
	if err != nil {
		return nil, err
	}
	if cfg.SSLMode == "" || cfg.SSLMode == "disable" {
		logger.Warn("database TLS is disabled (sslmode=disable) — credentials and data traverse the network in plaintext; use sslmode=require or verify-full for a remote database",
			"host", cfg.Host)
	}
	db, err := database.NewConnection(cfg, logger)
	if err != nil {
		return nil, err
	}
	return db, nil
}
