package kafka

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	runtimeconfig "github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/config"
)

func TestResolveConnectionConfig_MergesGlobalAndOverrides(t *testing.T) {
	tempDir := t.TempDir()
	caPath := filepath.Join(tempDir, "ca.crt")
	if err := os.WriteFile(caPath, []byte("ca"), 0o644); err != nil {
		t.Fatalf("write ca: %v", err)
	}

	global := runtimeconfig.KafkaConfig{
		Brokers:                       []string{"broker-1:9092"},
		CompactTopicPartitions:        3,
		CompactTopicReplicationFactor: 2,
		TLS:                           true,
		TLSCAFile:                     caPath,
		TLSServerName:                 "global-kafka",
		SASLMechanism:                 "plain",
		SASLUsername:                  "global-user",
		SASLPassword:                  "global-pass",
	}

	resolved, err := ResolveConnectionConfig(global, map[string]interface{}{
		"brokers":                          []interface{}{"broker-2:9093", " broker-3:9094 "},
		"tls_server_name":                  "binding-kafka",
		"sasl_username":                    "binding-user",
		"compact_topic_partitions":         float64(5),
		"compact_topic_replication_factor": float64(4),
	})
	if err != nil {
		t.Fatalf("ResolveConnectionConfig returned error: %v", err)
	}

	wantBrokers := []string{"broker-2:9093", "broker-3:9094"}
	if !reflect.DeepEqual(resolved.Brokers, wantBrokers) {
		t.Fatalf("expected brokers %v, got %v", wantBrokers, resolved.Brokers)
	}
	if !resolved.TLS {
		t.Fatalf("expected TLS to remain enabled")
	}
	if resolved.TLSCAFile != caPath {
		t.Fatalf("expected global CA file to be preserved, got %q", resolved.TLSCAFile)
	}
	if resolved.TLSServerName != "binding-kafka" {
		t.Fatalf("expected binding TLS server name override, got %q", resolved.TLSServerName)
	}
	if resolved.SASLUsername != "binding-user" {
		t.Fatalf("expected binding SASL username override, got %q", resolved.SASLUsername)
	}
	if resolved.SASLPassword != "global-pass" {
		t.Fatalf("expected global SASL password fallback, got %q", resolved.SASLPassword)
	}
	if resolved.CompactTopicPartitions != 5 {
		t.Fatalf("expected compact topic partitions override, got %d", resolved.CompactTopicPartitions)
	}
	if resolved.CompactTopicReplicationFactor != 4 {
		t.Fatalf("expected compact topic replication override, got %d", resolved.CompactTopicReplicationFactor)
	}
}

func TestResolveConnectionConfig_PreservesOpaqueCredentials(t *testing.T) {
	resolved, err := ResolveConnectionConfig(runtimeconfig.KafkaConfig{}, map[string]interface{}{
		"brokers":        []interface{}{"broker:9092"},
		"sasl_mechanism": "plain",
		"sasl_username":  "  user-with-spaces  ",
		"sasl_password":  "  secret-with-spaces  ",
	})
	if err != nil {
		t.Fatalf("ResolveConnectionConfig returned error: %v", err)
	}

	if resolved.SASLUsername != "  user-with-spaces  " {
		t.Fatalf("expected username to be preserved verbatim, got %q", resolved.SASLUsername)
	}
	if resolved.SASLPassword != "  secret-with-spaces  " {
		t.Fatalf("expected password to be preserved verbatim, got %q", resolved.SASLPassword)
	}
}

func TestResolveConnectionConfig_RequiresTLSWhenTLSFilesAreConfigured(t *testing.T) {
	_, err := ResolveConnectionConfig(runtimeconfig.KafkaConfig{
		Brokers:   []string{"broker:9092"},
		TLSCAFile: "/tmp/ca.crt",
	}, nil)
	if err == nil {
		t.Fatalf("expected error when TLS files are set with TLS disabled")
	}
}

func TestResolveConnectionConfig_ValidatesReadableTLSFiles(t *testing.T) {
	tempDir := t.TempDir()
	caPath := filepath.Join(tempDir, "ca.crt")
	if err := os.WriteFile(caPath, []byte("ca"), 0o644); err != nil {
		t.Fatalf("write ca: %v", err)
	}

	_, err := ResolveConnectionConfig(runtimeconfig.KafkaConfig{
		Brokers:   []string{"broker:9092"},
		TLS:       true,
		TLSCAFile: caPath,
	}, nil)
	if err != nil {
		t.Fatalf("expected readable CA file to validate, got %v", err)
	}

	_, err = ResolveConnectionConfig(runtimeconfig.KafkaConfig{
		Brokers:   []string{"broker:9092"},
		TLS:       true,
		TLSCAFile: filepath.Join(tempDir, "missing.crt"),
	}, nil)
	if err == nil {
		t.Fatalf("expected missing CA file to fail validation")
	}
}

func TestResolveConnectionConfig_RequiresSASLCredentials(t *testing.T) {
	_, err := ResolveConnectionConfig(runtimeconfig.KafkaConfig{}, map[string]interface{}{
		"brokers":        []interface{}{"broker:9092"},
		"sasl_mechanism": "scram-sha-512",
		"sasl_username":  "user",
	})
	if err == nil {
		t.Fatalf("expected missing SASL password to fail validation")
	}
}

func TestResolveConnectionConfig_RequiresPositiveCompactedTopicSettings(t *testing.T) {
	_, err := ResolveConnectionConfig(runtimeconfig.KafkaConfig{
		Brokers: []string{"broker:9092"},
	}, map[string]interface{}{
		"compact_topic_partitions": float64(0),
	})
	if err == nil {
		t.Fatalf("expected compact topic partitions validation to fail")
	}

	_, err = ResolveConnectionConfig(runtimeconfig.KafkaConfig{
		Brokers:                []string{"broker:9092"},
		CompactTopicPartitions: 1,
	}, map[string]interface{}{
		"compact_topic_replication_factor": float64(0),
	})
	if err == nil {
		t.Fatalf("expected compact topic replication validation to fail")
	}
}
