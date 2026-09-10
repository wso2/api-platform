package config

import (
	"path/filepath"
	"testing"
)

// TestDebugOverlay covers configs/config-debug.toml, the tracked overlay
// `make bff-run` and .vscode/launch.json layer on top of configs/config.toml. Its
// [auth.oidc] and [auth.oidc.token_exchange] tables are entirely {{ env }} tokens so
// no credential is committed, which means their correctness is invisible until
// someone exports the variables and runs the BFF — hence a test.
//
// Two things are pinned. First, an empty environment leaves the overlay inert:
// basic mode, exchange off, exactly as `make bff-run` behaved before the tables
// existed. Second, the exported-variable path actually reaches the config, including
// the inheritance that lets a single-application deployment omit the exchange
// credentials — and the separate-client case, which is the shape a real STS uses.
func TestDebugOverlay(t *testing.T) {
	base := filepath.Join("..", "..", "..", "configs", "config.toml")
	dbg := filepath.Join("..", "..", "..", "configs", "config-debug.toml")

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
