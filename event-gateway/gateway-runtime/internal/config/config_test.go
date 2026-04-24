package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
	t.Setenv("APIP_EGW_SERVER_WEBSUB_PORT", "9090")
	t.Setenv("APIP_EGW_SERVER_WEBSUB_TLS_ENABLED", "true")
	t.Setenv("APIP_EGW_SERVER_WEBSUB_TLS_CERT_FILE", "/tmp/tls.crt")
	t.Setenv("APIP_EGW_SERVER_WEBSUB_TLS_KEY_FILE", "/tmp/tls.key")
	t.Setenv("APIP_EGW_KAFKA_BROKERS", "kafka-1:9092,kafka-2:9092")
	t.Setenv("APIP_EGW_CONTROLPLANE_ENABLED", "true")
	t.Setenv("APIP_EGW_CONTROLPLANE_XDS_ADDRESS", "xds:18001")
	t.Setenv("APIP_EGW_POLICY_ENGINE_CONFIG_FILE", "/tmp/policies.toml")
	t.Setenv("APIP_EGW_LOGGING_LEVEL", "debug")
	t.Setenv("APIP_EGW_LOGGING_FORMAT", "json")

	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte(`
[server]
websub_port = 8080

[kafka]
brokers = ["localhost:9092"]

[controlplane]
enabled = false
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, _, err := Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Server.WebSubPort != 9090 {
		t.Fatalf("expected websub port 9090, got %d", cfg.Server.WebSubPort)
	}
	if !cfg.Server.WebSubTLSEnabled {
		t.Fatalf("expected websub TLS to be enabled")
	}
	if cfg.Server.WebSubTLSCertFile != "/tmp/tls.crt" {
		t.Fatalf("expected websub TLS cert file override, got %q", cfg.Server.WebSubTLSCertFile)
	}
	if cfg.Server.WebSubTLSKeyFile != "/tmp/tls.key" {
		t.Fatalf("expected websub TLS key file override, got %q", cfg.Server.WebSubTLSKeyFile)
	}

	wantBrokers := []string{"kafka-1:9092", "kafka-2:9092"}
	if !reflect.DeepEqual(cfg.Kafka.Brokers, wantBrokers) {
		t.Fatalf("expected brokers %v, got %v", wantBrokers, cfg.Kafka.Brokers)
	}

	if !cfg.ControlPlane.Enabled {
		t.Fatalf("expected control plane to be enabled")
	}

	if cfg.ControlPlane.XDSAddress != "xds:18001" {
		t.Fatalf("expected xds address xds:18001, got %q", cfg.ControlPlane.XDSAddress)
	}

	if cfg.PolicyEngine.ConfigFile != "/tmp/policies.toml" {
		t.Fatalf("expected policy engine config file override, got %q", cfg.PolicyEngine.ConfigFile)
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("expected logging level debug, got %q", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("expected logging format json, got %q", cfg.Logging.Format)
	}
}

func TestLoadRequiresWebSubTLSFilesWhenEnabled(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte(`
[server]
websub_tls_enabled = true
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := Load(configPath)
	if err == nil {
		t.Fatalf("expected load to fail when TLS files are missing")
	}

	want := "server.websub_tls_cert_file is required when server.websub_tls_enabled is true"
	if err.Error() != want {
		t.Fatalf("expected error %q, got %q", want, err.Error())
	}
}

func TestLoadRejectsInvalidLoggingLevel(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte(`
[logging]
level = "trace"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := Load(configPath)
	if err == nil {
		t.Fatalf("expected load to fail for invalid logging level")
	}

	want := "logging.level must be one of debug, info, warn, error"
	if err.Error() != want {
		t.Fatalf("expected error %q, got %q", want, err.Error())
	}
}
