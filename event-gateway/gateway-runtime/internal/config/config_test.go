package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
	t.Setenv("APIP_EGW_SERVER_WEBSUB_PORT", "9090")
	t.Setenv("APIP_EGW_KAFKA_BROKERS", "kafka-1:9092,kafka-2:9092")
	t.Setenv("APIP_EGW_CONTROLPLANE_ENABLED", "true")
	t.Setenv("APIP_EGW_CONTROLPLANE_XDS_ADDRESS", "xds:18001")
	t.Setenv("APIP_EGW_POLICY_ENGINE_CONFIG_FILE", "/tmp/policies.toml")

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
}
