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

package config

import "testing"

// The AI Workspace config has no list/array-typed keys, so koanf's array-replace
// semantics are exercised by the gateway-controller, policy-engine, and platform-api
// suites instead. These tests cover the merge behaviour that does apply here:
// deep-merge/last-wins on nested tables, cross-file type-mismatch (caught at
// unmarshal), a numeric field overridden by an {{ env }} token, interpolation after
// merge, argument-order precedence, and the fail-closed no-files contract.

const cpURL = "\n[ai_workspace.control_plane]\nurl = \"https://platform-api:9243\"\n"

// TestLoad_MultiFile_DeepMergeLastWins verifies that when the same nested table is
// set across two files, keys deep-merge and a key present in both files takes the
// later file's value (last-wins), while a key only in the base survives.
func TestLoad_MultiFile_DeepMergeLastWins(t *testing.T) {
	base := writeConfig(t, "[ai_workspace.logging]\nlevel = \"info\"\nformat = \"json\"\n"+cpURL)
	overlay := writeConfig(t, "[ai_workspace.logging]\nlevel = \"debug\"\n")

	cfg, err := Load(base, overlay)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want %q (later file must win)", cfg.Logging.Level, "debug")
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Logging.Format = %q, want %q (sibling key in a merged table must survive)", cfg.Logging.Format, "json")
	}
}

// TestLoad_MultiFile_TypeMismatchFails verifies that a genuinely invalid cross-file
// override — a non-numeric string for a numeric field — still fails. The loader does
// not use koanf StrictMerge (see loadConfigKoanf), so this is caught by the
// weakly-typed unmarshal after the merge rather than at merge time.
func TestLoad_MultiFile_TypeMismatchFails(t *testing.T) {
	base := writeConfig(t, "[ai_workspace.server.http]\nport = 9643\n")
	overlay := writeConfig(t, "[ai_workspace.server.http]\nport = \"not-a-number\"\n")

	if _, err := Load(base, overlay); err == nil {
		t.Fatal("Load() succeeded, want an error for a non-coercible cross-file override")
	}
}

// TestLoad_MultiFile_NumericOverriddenByEnvToken verifies that a numeric field set
// natively in the base can be overridden by an {{ env }} interpolation token (a TOML
// string) in an overlay — the token resolves and coerces to the numeric field. This
// is the reason the loader does not use koanf StrictMerge: a strict type check would
// reject the int-vs-string collision before interpolation runs.
func TestLoad_MultiFile_NumericOverriddenByEnvToken(t *testing.T) {
	t.Setenv("APIP_TEST_AIW_PORT", "9644")
	base := writeConfig(t, "[ai_workspace.server.http]\nport = 9643\n"+cpURL)
	overlay := writeConfig(t, "[ai_workspace.server.http]\nport = '{{ env \"APIP_TEST_AIW_PORT\" \"9643\" }}'\n")

	cfg, err := Load(base, overlay)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.HTTP.Port != 9644 {
		t.Errorf("Server.HTTP.Port = %d, want 9644 (an {{ env }} token in an overlay must override a native numeric base value)", cfg.Server.HTTP.Port)
	}
}

// TestLoad_MultiFile_EnvTokenResolvingToNonNumberFails verifies that interpolation
// does not bypass the type check: a numeric field whose {{ env }} token resolves to
// a non-numeric value still fails loudly, at the weakly-typed unmarshal after the
// token is expanded. (An unset/empty required token is a separate fail-closed path,
// covered by config_test.go.)
func TestLoad_MultiFile_EnvTokenResolvingToNonNumberFails(t *testing.T) {
	t.Setenv("APIP_TEST_AIW_PORT", "bar")
	base := writeConfig(t, "[ai_workspace.server.http]\nport = 9643\n"+cpURL)
	overlay := writeConfig(t, "[ai_workspace.server.http]\nport = '{{ env \"APIP_TEST_AIW_PORT\" }}'\n")

	if _, err := Load(base, overlay); err == nil {
		t.Fatal("Load() succeeded, want an error for an env token resolving to a non-number")
	}
}

// TestLoad_MultiFile_InterpolationAfterMerge verifies that {{ env }} interpolation
// runs once, after all files are merged (and after the ai_workspace subtree Cut): a
// token declared in the base resolves even when a later overlay overrides an
// unrelated key.
func TestLoad_MultiFile_InterpolationAfterMerge(t *testing.T) {
	t.Setenv("APIP_TEST_AIW_LOG", "warn")
	base := writeConfig(t, "[ai_workspace.logging]\nlevel = '{{ env \"APIP_TEST_AIW_LOG\" \"info\" }}'\n"+cpURL)
	overlay := writeConfig(t, "[ai_workspace.logging]\nformat = \"text\"\n")

	cfg, err := Load(base, overlay)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("Logging.Level = %q, want %q (token in base must resolve after merge)", cfg.Logging.Level, "warn")
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("Logging.Format = %q, want %q (overlay override must apply alongside interpolation)", cfg.Logging.Format, "text")
	}
}

// TestLoad_MultiFile_OrderMatters verifies precedence follows argument order:
// swapping the two files flips which value wins. The required control_plane.url lives
// in file a, so both orderings validate because a is present in both.
func TestLoad_MultiFile_OrderMatters(t *testing.T) {
	a := writeConfig(t, "[ai_workspace.logging]\nlevel = \"debug\"\n"+cpURL)
	b := writeConfig(t, "[ai_workspace.logging]\nlevel = \"error\"\n")

	cfgAB, err := Load(a, b)
	if err != nil {
		t.Fatalf("Load(a, b) error = %v", err)
	}
	if cfgAB.Logging.Level != "error" {
		t.Errorf("Logging.Level = %q, want %q (last file b must win)", cfgAB.Logging.Level, "error")
	}

	cfgBA, err := Load(b, a)
	if err != nil {
		t.Fatalf("Load(b, a) error = %v", err)
	}
	if cfgBA.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want %q (last file a must win when order is swapped)", cfgBA.Logging.Level, "debug")
	}
}

// TestLoad_NoFiles verifies the fail-closed contract: with no config file path, Load
// must error rather than silently run on built-in defaults.
func TestLoad_NoFiles(t *testing.T) {
	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded, want an error when no config file path is given")
	}
}

// TestLoad_MissingFileFails verifies that an explicitly-passed missing file is a hard
// error, not a silent no-op (there is no default fallback).
func TestLoad_MissingFileFails(t *testing.T) {
	if _, err := Load("/nonexistent/path/config.toml"); err == nil {
		t.Fatal("Load() succeeded, want an error for a missing config file")
	}
}
