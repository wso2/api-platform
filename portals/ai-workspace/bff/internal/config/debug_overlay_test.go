package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// hasOIDCTables reports whether the base config carries the tables the OIDC subtests
// below need. They cannot be supplied as a fixture: this test's whole purpose is to
// pin the SHIPPED files, so a locally-built config would assert nothing about them.
//
// A deployment without those tables is a valid one — it runs in basic mode — so their
// absence is a reason to skip those subtests, not to fail. The consequence is worth
// stating plainly: while they are absent, nothing exercises the OIDC or token-exchange
// config path, and `APIP_AIW_AUTH_OIDC_*` variables bind to nothing at runtime, since
// environment values reach the config only through {{ env }} tokens written in a file.
// The skip message says so, so the gap is visible in test output rather than silent.
func hasOIDCTables(t *testing.T, base string) bool {
	t.Helper()
	raw, err := os.ReadFile(base)
	if err != nil {
		t.Fatalf("read %s: %v", base, err)
	}
	for _, table := range []string{
		"[ai_workspace.auth.oidc]",
		"[ai_workspace.auth.oidc.token_exchange]",
	} {
		if !bytes.Contains(raw, []byte(table)) {
			return false
		}
	}
	return true
}

// skipWithoutOIDCTables keeps the reason in one place, so a reader of a skipped run
// learns what is not being covered rather than just that something was skipped.
func skipWithoutOIDCTables(t *testing.T, present bool, base string) {
	t.Helper()
	if !present {
		t.Skipf("%s has no [ai_workspace.auth.oidc] / [ai_workspace.auth.oidc.token_exchange] "+
			"tables, so there is nothing for APIP_AIW_AUTH_OIDC_* to bind to and this "+
			"subtest cannot assert anything. Add the tables to the shipped config to cover "+
			"the OIDC and token-exchange paths.", base)
	}
}

// TestDebugOverlay covers configs/config-debug.toml, the tracked overlay
// `make bff-run` and .vscode/launch.json layer on top of configs/config.toml, and
// with it the merged result the host-run BFF actually sees. The [auth.oidc] and
// [auth.oidc.token_exchange] tables live in the base file and are entirely {{ env }}
// tokens so no credential is committed, which means their correctness is invisible
// until someone exports the variables and runs the BFF — hence a test.
//
// Two things are pinned. First, an empty environment leaves the overlay inert:
// basic mode, exchange off, exactly as `make bff-run` behaved before the tables
// existed. Second, the exported-variable path actually reaches the config, including
// the inheritance that lets a single-application deployment omit the exchange
// credentials — and the separate-client case, which is the shape a real STS uses.
func TestDebugOverlay(t *testing.T) {
	base := filepath.Join("..", "..", "..", "configs", "config.toml")
	dbg := filepath.Join("..", "..", "..", "configs", "config-debug.toml")
	// Computed once; each OIDC subtest skips on it rather than the whole test, so the
	// inert-environment case below keeps running against any config.
	oidcTables := hasOIDCTables(t, base)

	t.Run("inert with an empty environment", func(t *testing.T) {
		cfg, err := Load(base, dbg)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Auth.Mode != "basic" {
			t.Errorf("mode = %q, want basic", cfg.Auth.Mode)
		}
		if cfg.Auth.TokenExchangeEnabled() {
			t.Error("token exchange must be off with an empty environment")
		}
	})

	t.Run("enabled via env", func(t *testing.T) {
		skipWithoutOIDCTables(t, oidcTables, base)
		t.Setenv("APIP_AIW_AUTH_MODE", "oidc")
		t.Setenv("APIP_AIW_AUTH_OIDC_AUTHORITY", "https://idp.example.com")
		t.Setenv("APIP_AIW_AUTH_OIDC_CLIENT_ID", "login-client")
		t.Setenv("APIP_AIW_AUTH_OIDC_CLIENT_SECRET", "login-secret")
		t.Setenv("APIP_AIW_AUTH_OIDC_TOKEN_EXCHANGE_ENABLED", "true")
		t.Setenv("APIP_AIW_AUTH_OIDC_TOKEN_EXCHANGE_AUDIENCE", "platform-api")
		cfg, err := Load(base, dbg)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		te := cfg.Auth.OIDC.TokenExchange
		if !cfg.Auth.TokenExchangeEnabled() {
			t.Fatal("token exchange must be enabled")
		}
		if te.Audience != "platform-api" {
			t.Errorf("audience = %q", te.Audience)
		}
		if te.ClientID != "login-client" || te.ClientSecret != "login-secret" {
			t.Errorf("credentials must inherit the login client: %q / %q", te.ClientID, te.ClientSecret)
		}
		if te.GrantType != GrantTokenExchange {
			t.Errorf("grant_type = %q", te.GrantType)
		}
		if !te.CacheEnabled || te.MinValidity <= 0 {
			t.Errorf("cache defaults lost: %v / %s", te.CacheEnabled, te.MinValidity)
		}
		if te.Scopes == "" {
			t.Error("scope must inherit the login scope set")
		}
		if te.SubjectTokenType == "" || te.RequestedTokenType == "" {
			t.Errorf("token-type defaults lost: %q / %q", te.SubjectTokenType, te.RequestedTokenType)
		}
	})

	t.Run("separate exchange client", func(t *testing.T) {
		skipWithoutOIDCTables(t, oidcTables, base)
		t.Setenv("APIP_AIW_AUTH_MODE", "oidc")
		t.Setenv("APIP_AIW_AUTH_OIDC_AUTHORITY", "https://idp.example.com")
		t.Setenv("APIP_AIW_AUTH_OIDC_CLIENT_ID", "oBnbNu4N")
		t.Setenv("APIP_AIW_AUTH_OIDC_CLIENT_SECRET", "login-secret")
		t.Setenv("APIP_AIW_AUTH_OIDC_TOKEN_EXCHANGE_ENABLED", "true")
		t.Setenv("APIP_AIW_AUTH_OIDC_TOKEN_EXCHANGE_CLIENT_ID", "F8ytfAHD")
		t.Setenv("APIP_AIW_AUTH_OIDC_TOKEN_EXCHANGE_CLIENT_SECRET", "exchange-secret")
		t.Setenv("APIP_AIW_AUTH_OIDC_TOKEN_EXCHANGE_AUDIENCE", "platform-api")
		cfg, err := Load(base, dbg)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Auth.OIDC.ClientID != "oBnbNu4N" || cfg.Auth.OIDC.TokenExchange.ClientID != "F8ytfAHD" {
			t.Errorf("the two clients must stay distinct: %q / %q",
				cfg.Auth.OIDC.ClientID, cfg.Auth.OIDC.TokenExchange.ClientID)
		}
	})

	t.Run("jwt_bearer entra shape", func(t *testing.T) {
		skipWithoutOIDCTables(t, oidcTables, base)
		t.Setenv("APIP_AIW_AUTH_MODE", "oidc")
		t.Setenv("APIP_AIW_AUTH_OIDC_AUTHORITY", "https://idp.example.com")
		t.Setenv("APIP_AIW_AUTH_OIDC_CLIENT_ID", "login-client")
		t.Setenv("APIP_AIW_AUTH_OIDC_CLIENT_SECRET", "login-secret")
		t.Setenv("APIP_AIW_AUTH_OIDC_TOKEN_EXCHANGE_ENABLED", "true")
		t.Setenv("APIP_AIW_AUTH_OIDC_TOKEN_EXCHANGE_GRANT_TYPE", "jwt_bearer")
		t.Setenv("APIP_AIW_AUTH_OIDC_TOKEN_EXCHANGE_SCOPE", "api://platform-api/.default")
		if _, err := Load(base, dbg); err != nil {
			t.Fatalf("Load: %v", err)
		}
	})
}
