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
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestDSN_QuotesSpecialCharacters(t *testing.T) {
	const pw = `it's a p\ss word`
	t.Setenv("TEST_MIGRATE_DB_PASSWORD", pw)
	c := DBConn{Host: "db.example", Port: 5432, Name: "my db", User: "o'brien", SSLMode: "disable", passwordEnv: "TEST_MIGRATE_DB_PASSWORD"}
	dsn, err := c.DSN()
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	cfg, err := pgconn.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	if cfg.Password != pw || cfg.User != "o'brien" || cfg.Database != "my db" || cfg.Host != "db.example" || cfg.Port != 5432 {
		t.Fatalf("round-trip mismatch: user=%q db=%q host=%q port=%d password-ok=%v",
			cfg.User, cfg.Database, cfg.Host, cfg.Port, cfg.Password == pw)
	}
}
