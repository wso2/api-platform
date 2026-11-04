/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package gateway

import (
	"testing"

	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/utils"
)

// runAdd runs `gateway add` against a throwaway config in a temp HOME.
func runAdd(t *testing.T, name, server, adminServer string) *config.Gateway {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	addName, addPlatform, addServer, addAdminServer = name, "", server, adminServer
	addAuth, addUsername, addPassword, addToken = utils.AuthTypeNone, "", "", ""
	addNoInteractive = true

	if err := runAddCommand(); err != nil {
		t.Fatalf("runAddCommand: %v", err)
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	gw, err := cfg.GetGateway(name)
	if err != nil {
		t.Fatalf("GetGateway(%q): %v", name, err)
	}
	return gw
}

func TestAddCommand_PersistsAdminServer(t *testing.T) {
	gw := runAdd(t, "local", "http://localhost:9090", "http://localhost:9094")
	if gw.AdminServer != "http://localhost:9094" {
		t.Fatalf("--admin-server was not saved: got %q", gw.AdminServer)
	}
}

func TestAddCommand_OmittedAdminServerStaysEmpty(t *testing.T) {
	gw := runAdd(t, "local", "http://localhost:9090", "")
	if gw.AdminServer != "" {
		t.Fatalf("expected no admin server so health defaults to --server, got %q", gw.AdminServer)
	}
}
