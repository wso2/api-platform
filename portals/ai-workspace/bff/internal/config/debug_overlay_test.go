package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// requireOIDCTables fails with a diagnosis instead of a validation error when the base
// config is missing the tables this test exists to cover.
//
// Without it the three OIDC subtests below fail with "OIDC mode requires [auth.oidc]
// authority, client_id, client_secret and redirect_url" — which reads as a broken test
// environment, and sends the reader looking at the subtests' t.Setenv calls (which are
// fine) rather than at the file. The tables are all {{ env }} tokens carrying no
// credential, so the usual reason they are absent is that the file has not been
// committed yet: the test then passes for whoever has the edits locally and fails for
// everyone else and in CI, which is the confusing shape this message short-circuits.
func requireOIDCTables(t *testing.T, base string) {
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
			t.Fatalf("%s has no %s table.\n"+
				"This test pins the SHIPPED config files, so the table has to exist there — "+
				"it is not a fixture this test can supply for itself. If it is present in your "+
				"working tree, it is not committed: commit it (every value in it is an {{ env }} "+
				"token, so no credential is committed with it).", base, table)
		}
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
	requireOIDCTables(t, base)

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
