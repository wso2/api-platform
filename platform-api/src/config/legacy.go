/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package config

import (
	"log"
	"os"
)

// legacyEnvAliases maps deprecated env var names to their current equivalents.
// When the new name is unset but the old name has a value, the old value is
// injected under the new name before config loading runs.
var legacyEnvAliases = [][2]string{
	// JWT (local HMAC mode) — renamed under AUTH_JWT_* namespace
	{"JWT_SECRET_KEY", "AUTH_JWT_SECRET_KEY"},
	{"JWT_ISSUER", "AUTH_JWT_ISSUER"},
	{"JWT_SKIP_VALIDATION", "AUTH_JWT_SKIP_VALIDATION"},
	{"JWT_SKIP_PATHS", "AUTH_SKIP_PATHS"},
}

// applyLegacyEnvAliases copies deprecated env var values to their new names
// so that operators who have not yet migrated their config continue to work.
// A deprecation warning is logged for each old var that is still in use.
func applyLegacyEnvAliases() {
	for _, pair := range legacyEnvAliases {
		old, newName := pair[0], pair[1]
		if os.Getenv(newName) == "" {
			if val := os.Getenv(old); val != "" {
				_ = os.Setenv(newName, val)
				log.Printf("[DEPRECATED] environment variable %s has been renamed to %s — please update your configuration", old, newName)
			}
		}
	}
}
