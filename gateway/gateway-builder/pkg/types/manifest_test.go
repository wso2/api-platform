package types

import "testing"

func TestManifestEntry_DeriveFilePath(t *testing.T) {
	tests := []struct {
		name     string
		entry    ManifestEntry
		expected string
	}{
		{
			name:     "BasicAuth policy",
			entry:    ManifestEntry{Name: "BasicAuth", Version: "v1.0.0"},
			expected: "basic-auth/v1.0.0",
		},
		{
			name:     "APIKey policy",
			entry:    ManifestEntry{Name: "APIKey", Version: "v2.1.0"},
			expected: "api-key/v2.1.0",
		},
		{
			name:     "RateLimit policy",
			entry:    ManifestEntry{Name: "RateLimit", Version: "v1.5.2"},
			expected: "rate-limit/v1.5.2",
		},
		{
			name:     "Single word policy",
			entry:    ManifestEntry{Name: "CORS", Version: "v1.0.0"},
			expected: "cors/v1.0.0",
		},
		{
			name:     "Already lowercase",
			entry:    ManifestEntry{Name: "transform", Version: "v3.0.0"},
			expected: "transform/v3.0.0",
		},
		{
			name:     "Multiple consecutive uppercase",
			entry:    ManifestEntry{Name: "HTTPSRedirect", Version: "v1.0.0"},
			expected: "https-redirect/v1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.entry.DeriveFilePath()
			if result != tt.expected {
				t.Errorf("DeriveFilePath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestToKebabCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "PascalCase",
			input:    "BasicAuth",
			expected: "basic-auth",
		},
		{
			name:     "camelCase",
			input:    "apiKey",
			expected: "api-key",
		},
		{
			name:     "ALL_CAPS",
			input:    "CORS",
			expected: "cors",
		},
		{
			name:     "lowercase",
			input:    "transform",
			expected: "transform",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Multiple uppercase",
			input:    "HTTPSRedirect",
			expected: "https-redirect",
		},
		{
			name:     "Single character",
			input:    "A",
			expected: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toKebabCase(tt.input)
			if result != tt.expected {
				t.Errorf("toKebabCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
