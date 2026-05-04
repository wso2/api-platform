package kafka

import (
	"reflect"
	"strings"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestValidateConnectionConfig(t *testing.T) {
	tests := []struct {
		name        string
		cfg         ConnectionConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "valid plain",
			cfg: ConnectionConfig{
				Brokers:       []string{"kafka:9092"},
				SASLMechanism: SASLMechanismPlain,
				SASLUsername:  "user",
				SASLPassword:  "pass",
			},
		},
		{
			name: "valid scram sha 256",
			cfg: ConnectionConfig{
				Brokers:       []string{"kafka:9092"},
				SASLMechanism: SASLMechanismSCRAMSHA256,
				SASLUsername:  "user",
				SASLPassword:  "pass",
			},
		},
		{
			name: "valid scram sha 512",
			cfg: ConnectionConfig{
				Brokers:       []string{"kafka:9092"},
				SASLMechanism: SASLMechanismSCRAMSHA512,
				SASLUsername:  "user",
				SASLPassword:  "pass",
			},
		},
		{
			name: "invalid mechanism",
			cfg: ConnectionConfig{
				Brokers:       []string{"kafka:9092"},
				SASLMechanism: "oauth",
				SASLUsername:  "user",
				SASLPassword:  "pass",
			},
			wantErr:     true,
			errContains: "must be one of",
		},
		{
			name: "credentials without mechanism",
			cfg: ConnectionConfig{
				Brokers:      []string{"kafka:9092"},
				SASLUsername: "user",
				SASLPassword: "pass",
			},
			wantErr:     true,
			errContains: "require sasl_mechanism",
		},
		{
			name: "mechanism without username",
			cfg: ConnectionConfig{
				Brokers:       []string{"kafka:9092"},
				SASLMechanism: SASLMechanismPlain,
				SASLPassword:  "pass",
			},
			wantErr:     true,
			errContains: "requires both",
		},
		{
			name: "mechanism without password",
			cfg: ConnectionConfig{
				Brokers:       []string{"kafka:9092"},
				SASLMechanism: SASLMechanismSCRAMSHA256,
				SASLUsername:  "user",
			},
			wantErr:     true,
			errContains: "requires both",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConnectionConfig(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestResolveConnectionConfig(t *testing.T) {
	defaults := ConnectionConfig{
		Brokers:       []string{"global-1:9092", "global-2:9092"},
		TLS:           true,
		SASLMechanism: SASLMechanismSCRAMSHA512,
		SASLUsername:  "global-user",
		SASLPassword:  "global-pass",
	}

	tests := []struct {
		name      string
		overrides map[string]interface{}
		want      ConnectionConfig
	}{
		{
			name:      "global only config",
			overrides: nil,
			want:      defaults,
		},
		{
			name: "broker override",
			overrides: map[string]interface{}{
				"brokers": []interface{}{"binding:29092"},
			},
			want: ConnectionConfig{
				Brokers:       []string{"binding:29092"},
				TLS:           true,
				SASLMechanism: SASLMechanismSCRAMSHA512,
				SASLUsername:  "global-user",
				SASLPassword:  "global-pass",
			},
		},
		{
			name: "tls and sasl override",
			overrides: map[string]interface{}{
				"tls":            false,
				"sasl_mechanism": SASLMechanismPlain,
				"sasl_username":  "binding-user",
				"sasl_password":  "binding-pass",
			},
			want: ConnectionConfig{
				Brokers:       []string{"global-1:9092", "global-2:9092"},
				TLS:           false,
				SASLMechanism: SASLMechanismPlain,
				SASLUsername:  "binding-user",
				SASLPassword:  "binding-pass",
			},
		},
		{
			name: "partial merge semantics",
			overrides: map[string]interface{}{
				"sasl_username": "binding-user",
			},
			want: ConnectionConfig{
				Brokers:       []string{"global-1:9092", "global-2:9092"},
				TLS:           true,
				SASLMechanism: SASLMechanismSCRAMSHA512,
				SASLUsername:  "binding-user",
				SASLPassword:  "global-pass",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveConnectionConfig(defaults, tt.overrides)
			if err != nil {
				t.Fatalf("ResolveConnectionConfig returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("expected %+v, got %+v", tt.want, got)
			}
		})
	}
}

func TestBuildClientOptions(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ConnectionConfig
		wantErr bool
	}{
		{
			name: "no auth",
			cfg: ConnectionConfig{
				Brokers: []string{"kafka:9092"},
			},
		},
		{
			name: "tls only",
			cfg: ConnectionConfig{
				Brokers: []string{"kafka:9092"},
				TLS:     true,
			},
		},
		{
			name: "sasl plain",
			cfg: ConnectionConfig{
				Brokers:       []string{"kafka:9092"},
				SASLMechanism: SASLMechanismPlain,
				SASLUsername:  "user",
				SASLPassword:  "pass",
			},
		},
		{
			name: "invalid combination",
			cfg: ConnectionConfig{
				Brokers:       []string{"kafka:9092"},
				SASLMechanism: SASLMechanismPlain,
				SASLUsername:  "user",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := BuildClientOptions(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildClientOptions returned error: %v", err)
			}
			client, err := kgo.NewClient(opts...)
			if err != nil {
				t.Fatalf("kgo.NewClient returned error: %v", err)
			}
			client.Close()
		})
	}
}
