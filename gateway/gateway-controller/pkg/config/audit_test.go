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

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func validAuditConfig() AuditConfig {
	return AuditConfig{
		Enabled: true,
		Outputs: []string{AuditOutputStdout, AuditOutputFile, AuditOutputHTTP},
		File:    AuditFileConfig{Path: "/var/log/wso2/audit/audit.log", MaxSizeMB: 100},
		HTTP: AuditHTTPConfig{
			Endpoint:       "https://logs.example.com/ingest",
			ContentType:    "application/x-ndjson",
			BatchMaxEvents: 100,
			BatchMaxBytes:  1 << 20,
			FlushInterval:  5 * time.Second,
			QueueCapacity:  10000,
			RequestTimeout: 10 * time.Second,
			MaxRetries:     3,
			RetryBackoff:   time.Second,
		},
	}
}

func TestValidateAuditConfig(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*AuditConfig)
		wantErr string // empty means valid
	}{
		{"every output", func(*AuditConfig) {}, ""},
		{"disabled skips validation", func(c *AuditConfig) { c.Enabled = false; c.Outputs = []string{"syslog"} }, ""},
		{"no outputs", func(c *AuditConfig) { c.Outputs = nil; c.File.Path = ""; c.HTTP.Endpoint = "" }, ""},
		{"unselected outputs are not validated", func(c *AuditConfig) { c.Outputs = []string{"stdout"}; c.File.Path = ""; c.HTTP.Endpoint = "" }, ""},
		{"unknown output", func(c *AuditConfig) { c.Outputs = []string{"syslog"} }, `unknown output "syslog"`},
		{"repeated output", func(c *AuditConfig) { c.Outputs = []string{"file", " FILE "} }, `lists "file" more than once`},

		{"file path missing", func(c *AuditConfig) { c.File.Path = " " }, "controller.audit.file: path is required"},
		{"file path relative", func(c *AuditConfig) { c.File.Path = "logs/audit.log" }, "path must be absolute"},
		{"file path with a .. segment", func(c *AuditConfig) { c.File.Path = "/var/log/../audit.log" }, `must not contain ".." segments`},
		{"file path with a null byte", func(c *AuditConfig) { c.File.Path = "/var/log/audit\x00.log" }, "null byte"},
		{"max_size_mb negative", func(c *AuditConfig) { c.File.MaxSizeMB = -1 }, "max_size_mb must be between 0 and"},
		{"max_size_mb too large", func(c *AuditConfig) { c.File.MaxSizeMB = maxAuditFileSizeMB + 1 }, "max_size_mb must be between 0 and"},
		{"max_size_mb 0 disables rotation", func(c *AuditConfig) { c.File.MaxSizeMB = 0 }, ""},
		{"max_size_mb at the maximum", func(c *AuditConfig) { c.File.MaxSizeMB = maxAuditFileSizeMB }, ""},

		{"endpoint missing", func(c *AuditConfig) { c.HTTP.Endpoint = "" }, "controller.audit.http: endpoint is required"},
		{"endpoint not a URL", func(c *AuditConfig) { c.HTTP.Endpoint = "https://logs example.com/" }, "endpoint is not a valid URL"},
		{"plaintext endpoint", func(c *AuditConfig) { c.HTTP.Endpoint = "http://logs.example.com/" }, "allow_insecure_transport is false"},
		{"plaintext endpoint allowed", func(c *AuditConfig) {
			c.HTTP.Endpoint = "http://logs.example.com/"
			c.HTTP.AllowInsecureTransport = true
		}, ""},
		{"unsupported scheme", func(c *AuditConfig) { c.HTTP.Endpoint = "ftp://logs.example.com/" }, `scheme must be https`},
		{"endpoint without a host", func(c *AuditConfig) { c.HTTP.Endpoint = "https:///ingest" }, "must include a host"},
		{"port 0", func(c *AuditConfig) { c.HTTP.Endpoint = "https://logs.example.com:0/" }, "port must be between 1 and 65535"},
		{"port above 65535", func(c *AuditConfig) { c.HTTP.Endpoint = "https://logs.example.com:70000/" }, "port must be between 1 and 65535"},
		{"insecure_skip_verify", func(c *AuditConfig) { c.HTTP.TLS.InsecureSkipVerify = true }, ""},

		{"batch_max_events 0", func(c *AuditConfig) { c.HTTP.BatchMaxEvents = 0 }, "batch_max_events must be positive"},
		{"batch_max_bytes 0", func(c *AuditConfig) { c.HTTP.BatchMaxBytes = 0 }, "batch_max_bytes must be positive"},
		{"flush_interval 0", func(c *AuditConfig) { c.HTTP.FlushInterval = 0 }, "flush_interval must be positive"},
		{"queue_capacity 0", func(c *AuditConfig) { c.HTTP.QueueCapacity = 0 }, "queue_capacity must be positive"},
		{"request_timeout 0", func(c *AuditConfig) { c.HTTP.RequestTimeout = 0 }, "request_timeout must be positive"},
		{"max_retries negative", func(c *AuditConfig) { c.HTTP.MaxRetries = -1 }, "max_retries must be >= 0"},
		{"retry_backoff 0 with retries", func(c *AuditConfig) { c.HTTP.RetryBackoff = 0 }, "retry_backoff must be positive"},
		{"retry_backoff 0 without retries", func(c *AuditConfig) { c.HTTP.MaxRetries = 0; c.HTTP.RetryBackoff = 0 }, ""},
		{"retry_backoff above 1h", func(c *AuditConfig) { c.HTTP.RetryBackoff = time.Hour + time.Second }, "retry_backoff must be at most 1h0m0s"},
		{"retry_backoff of 1h", func(c *AuditConfig) { c.HTTP.RetryBackoff = time.Hour }, ""},
		{"content_type empty", func(c *AuditConfig) { c.HTTP.ContentType = "" }, "content_type must be a valid Content-Type"},
		{"content_type with a line break", func(c *AuditConfig) { c.HTTP.ContentType = "application/json\r\nX-Injected: 1" }, "content_type must be a valid Content-Type"},
		{"auth error", func(c *AuditConfig) { c.HTTP.Auth.Type = AuditAuthBearer }, "auth: bearer.token is required"},
		{"cert_file without key_file", func(c *AuditConfig) { c.HTTP.TLS.CertFile = "/etc/gateway-controller/client.pem" }, "cert_file and key_file must be set together"},
		{"key_file without cert_file", func(c *AuditConfig) { c.HTTP.TLS.KeyFile = "/etc/gateway-controller/client.key" }, "cert_file and key_file must be set together"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validAuditConfig()
			tc.mutate(&cfg)
			err := validateAuditConfig(&cfg)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateAuditConfig() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("validateAuditConfig() error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestNormalizeAuditOutputs(t *testing.T) {
	got, err := NormalizeAuditOutputs([]string{" STDOUT ", "", "Http", "file"})
	if err != nil {
		t.Fatalf("NormalizeAuditOutputs() error = %v", err)
	}
	if want := []string{"stdout", "http", "file"}; !slices.Equal(got, want) {
		t.Fatalf("NormalizeAuditOutputs() = %v, want %v", got, want)
	}

	got, err = NormalizeAuditOutputs(nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("NormalizeAuditOutputs(nil) = %v, %v; want no outputs", got, err)
	}
}

func TestResolveAuditFilePath(t *testing.T) {
	got, err := ResolveAuditFilePath("/var/log/./wso2//audit.log")
	if err != nil {
		t.Fatalf("ResolveAuditFilePath() error = %v", err)
	}
	if want := "/var/log/wso2/audit.log"; got != want {
		t.Fatalf("ResolveAuditFilePath() = %q, want %q", got, want)
	}
}

func TestAuditHTTPAuthHeader(t *testing.T) {
	auth := func(set func(*AuditHTTPAuthConfig)) AuditHTTPAuthConfig {
		var a AuditHTTPAuthConfig
		set(&a)
		return a
	}
	for _, tc := range []struct {
		name      string
		auth      AuditHTTPAuthConfig
		wantName  string
		wantValue string
		wantErr   string
	}{
		{name: "no type", auth: AuditHTTPAuthConfig{}},
		{name: "none", auth: AuditHTTPAuthConfig{Type: "none"}},
		{name: "bearer, any case and spacing", auth: auth(func(a *AuditHTTPAuthConfig) { a.Type = " Bearer "; a.Bearer.Token = "t0k" }),
			wantName: "Authorization", wantValue: "Bearer t0k"},
		{name: "bearer without a token", auth: AuditHTTPAuthConfig{Type: "bearer"}, wantErr: "bearer.token is required"},
		{name: "basic", auth: auth(func(a *AuditHTTPAuthConfig) { a.Type = "basic"; a.Basic.Username = "gw"; a.Basic.Password = "p:w" }),
			wantName: "Authorization", wantValue: "Basic Z3c6cDp3"},
		{name: "basic without a password", auth: auth(func(a *AuditHTTPAuthConfig) { a.Type = "basic"; a.Basic.Username = "gw" }),
			wantErr: "basic.username and basic.password are both required"},
		{name: "basic username with a colon", auth: auth(func(a *AuditHTTPAuthConfig) { a.Type = "basic"; a.Basic.Username = "g:w"; a.Basic.Password = "pw" }),
			wantErr: "must not contain a colon"},
		{name: "header", auth: auth(func(a *AuditHTTPAuthConfig) {
			a.Type = "header"
			a.Header.Name = "Authorization"
			a.Header.Value = "Splunk abc"
		}),
			wantName: "Authorization", wantValue: "Splunk abc"},
		{name: "header without a value", auth: auth(func(a *AuditHTTPAuthConfig) { a.Type = "header"; a.Header.Name = "X-Key" }),
			wantErr: "header.name and header.value are both required"},
		{name: "header with an invalid name", auth: auth(func(a *AuditHTTPAuthConfig) { a.Type = "header"; a.Header.Name = "X Key"; a.Header.Value = "v" }),
			wantErr: "is not a valid HTTP header name"},
		{name: "value with a line break", auth: auth(func(a *AuditHTTPAuthConfig) { a.Type = "bearer"; a.Bearer.Token = "t0k\n" }),
			wantErr: "contains characters not allowed in HTTP"},
		{name: "unknown type", auth: AuditHTTPAuthConfig{Type: "digest"}, wantErr: `unknown type "digest"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name, value, err := tc.auth.AuthHeader()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("AuthHeader() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("AuthHeader() error = %v", err)
			}
			if name != tc.wantName || value != tc.wantValue {
				t.Fatalf("AuthHeader() = %q: %q, want %q: %q", name, value, tc.wantName, tc.wantValue)
			}
		})
	}
}
