package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
	tempDir := t.TempDir()
	certPath := filepath.Join(tempDir, "tls.crt")
	keyPath := filepath.Join(tempDir, "tls.key")
	if err := os.WriteFile(certPath, []byte("cert"), 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	t.Setenv("APIP_EGW_SERVER_WEBSUB_ENABLED", "false")
	t.Setenv("APIP_EGW_SERVER_WEBSUB_HTTP_PORT", "9090")
	t.Setenv("APIP_EGW_SERVER_WEBSUB_HTTPS_PORT", "9443")
	t.Setenv("APIP_EGW_SERVER_WEBSUB_TLS_ENABLED", "true")
	t.Setenv("APIP_EGW_SERVER_WEBSUB_TLS_CERT_FILE", certPath)
	t.Setenv("APIP_EGW_SERVER_WEBSUB_TLS_KEY_FILE", keyPath)
	t.Setenv("APIP_EGW_KAFKA_BROKERS", "kafka-1:9092,kafka-2:9092")
	t.Setenv("APIP_EGW_CONTROLPLANE_ENABLED", "true")
	t.Setenv("APIP_EGW_CONTROLPLANE_XDS_ADDRESS", "xds:18001")
	t.Setenv("APIP_EGW_POLICY_ENGINE_CONFIG_FILE", "/tmp/policies.toml")
	t.Setenv("APIP_EGW_LOGGING_LEVEL", "debug")
	t.Setenv("APIP_EGW_LOGGING_FORMAT", "json")

	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte(`
[server]
websub_http_port = 8080
websub_https_port = 8443

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

	if cfg.Server.WebSubEnabled {
		t.Fatalf("expected websub to be disabled")
	}
	if cfg.Server.WebSubHTTPPort != 9090 {
		t.Fatalf("expected websub http port 9090, got %d", cfg.Server.WebSubHTTPPort)
	}
	if cfg.Server.WebSubHTTPSPort != 9443 {
		t.Fatalf("expected websub https port 9443, got %d", cfg.Server.WebSubHTTPSPort)
	}
	if !cfg.Server.WebSubTLSEnabled {
		t.Fatalf("expected websub TLS to be enabled")
	}
	if cfg.Server.WebSubTLSCertFile != certPath {
		t.Fatalf("expected websub TLS cert file override, got %q", cfg.Server.WebSubTLSCertFile)
	}
	if cfg.Server.WebSubTLSKeyFile != keyPath {
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
websub_enabled = true
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

func TestLoadRequiresTLSFilesEvenWhenWebSubDisabled(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte(`
[server]
websub_enabled = false
websub_tls_enabled = true
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := Load(configPath)
	if err == nil {
		t.Fatalf("expected load to fail when TLS is enabled without readable files")
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

func TestLoadRejectsMissingTLSFilesWhenEnabled(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	missingCert := filepath.Join(t.TempDir(), "missing.crt")
	missingKey := filepath.Join(t.TempDir(), "missing.key")
	if err := os.WriteFile(configPath, []byte(`
[server]
websub_enabled = true
websub_tls_enabled = true
websub_tls_cert_file = "`+missingCert+`"
websub_tls_key_file = "`+missingKey+`"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := Load(configPath)
	if err == nil {
		t.Fatalf("expected load to fail when TLS cert file is missing")
	}

	if !strings.Contains(err.Error(), `server.websub_tls_cert_file file "`) || !strings.Contains(err.Error(), `does not exist`) {
		t.Fatalf("expected missing cert file error, got %q", err.Error())
	}
}

func TestLoadRejectsUnreadableTLSKeyFileWhenEnabled(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-based readability checks are unreliable when running as root")
	}

	tempDir := t.TempDir()
	certPath := filepath.Join(tempDir, "tls.crt")
	keyPath := filepath.Join(tempDir, "tls.key")
	configPath := filepath.Join(tempDir, "config.toml")

	if err := os.WriteFile(certPath, []byte("cert"), 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	if err := os.Chmod(keyPath, 0o000); err != nil {
		t.Fatalf("chmod key unreadable: %v", err)
	}
	defer func() {
		_ = os.Chmod(keyPath, 0o600)
	}()

	if err := os.WriteFile(configPath, []byte(`
[server]
websub_enabled = true
websub_tls_enabled = true
websub_tls_cert_file = "`+certPath+`"
websub_tls_key_file = "`+keyPath+`"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := Load(configPath)
	if err == nil {
		t.Fatalf("expected load to fail when TLS key file is unreadable")
	}

	if !strings.Contains(err.Error(), `server.websub_tls_key_file file "`) || !strings.Contains(err.Error(), `is not readable`) {
		t.Fatalf("expected unreadable key file error, got %q", err.Error())
	}
}

func TestLoadRejectsNonPositiveServerPorts(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte(`
[server]
websub_http_port = 0
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := Load(configPath)
	if err == nil {
		t.Fatalf("expected load to fail for non-positive port")
	}

	want := "server.websub_http_port must be a positive integer, got 0"
	if err.Error() != want {
		t.Fatalf("expected error %q, got %q", want, err.Error())
	}
}

func TestLoadRejectsDuplicateServerPorts(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte(`
[server]
websub_http_port = 8080
websub_https_port = 8443
websocket_port = 8080
admin_port = 9002
metrics_port = 9003
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := Load(configPath)
	if err == nil {
		t.Fatalf("expected load to fail for duplicate ports")
	}

	want := "server.websocket_port port 8080 conflicts with server.websub_http_port"
	if err.Error() != want {
		t.Fatalf("expected error %q, got %q", want, err.Error())
	}
}
