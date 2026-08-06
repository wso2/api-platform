package kafka

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
		CompactTopicPartitions:        1,
		CompactTopicReplicationFactor: 1,
		TLS:                           true,
		TLSCAFile:                     caPath,
		TLSServerName:                 "global-kafka",
		SASLMechanism:                 "plain",
		SASLUsername:                  "global-user",
		SASLPassword:                  "global-pass",
	}

	resolved, err := ResolveConnectionConfig(global, map[string]interface{}{
		"brokers":         []interface{}{"broker-2:9093", " broker-3:9094 "},
		"tls_server_name": "binding-kafka",
		"sasl_username":   "binding-user",
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
}

func TestResolveConnectionConfig_PreservesOpaqueCredentials(t *testing.T) {
	resolved, err := ResolveConnectionConfig(runtimeconfig.KafkaConfig{
		CompactTopicPartitions:        1,
		CompactTopicReplicationFactor: 1,
	}, map[string]interface{}{
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
		Brokers:                       []string{"broker:9092"},
		CompactTopicPartitions:        1,
		CompactTopicReplicationFactor: 1,
		TLSCAFile:                     "/tmp/ca.crt",
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
		Brokers:                       []string{"broker:9092"},
		CompactTopicPartitions:        1,
		CompactTopicReplicationFactor: 1,
		TLS:                           true,
		TLSCAFile:                     caPath,
	}, nil)
	if err != nil {
		t.Fatalf("expected readable CA file to validate, got %v", err)
	}

	_, err = ResolveConnectionConfig(runtimeconfig.KafkaConfig{
		Brokers:                       []string{"broker:9092"},
		CompactTopicPartitions:        1,
		CompactTopicReplicationFactor: 1,
		TLS:                           true,
		TLSCAFile:                     filepath.Join(tempDir, "missing.crt"),
	}, nil)
	if err == nil {
		t.Fatalf("expected missing CA file to fail validation")
	}
}

func TestResolveConnectionConfig_RequiresSASLCredentials(t *testing.T) {
	_, err := ResolveConnectionConfig(runtimeconfig.KafkaConfig{
		CompactTopicPartitions:        1,
		CompactTopicReplicationFactor: 1,
	}, map[string]interface{}{
		"brokers":        []interface{}{"broker:9092"},
		"sasl_mechanism": "scram-sha-512",
		"sasl_username":  "user",
	})
	if err == nil {
		t.Fatalf("expected missing SASL password to fail validation")
	}
}

func TestIntOverride_AcceptsIntegerFloat64(t *testing.T) {
	got, ok, err := intOverride(float64(3))
	if err != nil {
		t.Fatalf("expected integer float64 override to succeed, got %v", err)
	}
	if !ok {
		t.Fatalf("expected integer float64 override to be accepted")
	}
	if got != 3 {
		t.Fatalf("expected integer float64 override to convert to 3, got %d", got)
	}
}

func TestIntOverride_RejectsInvalidFloat64(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		wantErr string
	}{
		{
			name:    "non integer",
			value:   3.5,
			wantErr: "non-integer",
		},
		{
			name:    "out of bounds",
			value:   float64(math.MaxInt32) + 1,
			wantErr: "within",
		},
		{
			name:    "non finite",
			value:   math.NaN(),
			wantErr: "non-finite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok, err := intOverride(tt.value)
			if err == nil {
				t.Fatalf("expected float64 override %v to fail", tt.value)
			}
			if ok {
				t.Fatalf("expected invalid float64 override %v to be rejected", tt.value)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error %q to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
