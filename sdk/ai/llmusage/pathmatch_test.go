package llmusage

import "testing"

func TestPathsMatch(t *testing.T) {
	tests := []struct {
		name        string
		requestPath string
		pattern     string
		want        bool
	}{
		{"root wildcard matches anything", "/chat/completions", "/*", true},
		{"exact match", "/responses", "/responses", true},
		{"prefix wildcard matches", "/chat/completions", "/chat/*", true},
		{"prefix wildcard matches deeper", "/chat/completions/stream", "/chat/*", true},
		{"different path does not match", "/responses", "/chat/completions", false},
		{"specific pattern does not match wildcard request", "/chat/*", "/chat/completions", false},
		{"empty pattern does not match", "/responses", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pathsMatch(tt.requestPath, tt.pattern); got != tt.want {
				t.Errorf("pathsMatch(%q, %q) = %v, want %v", tt.requestPath, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestPreferMoreSpecificPath(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		current   string
		want      bool
	}{
		{"exact beats wildcard", "/v1/exact", "/v1/*", true},
		{"wildcard loses to exact", "/v1/*", "/v1/exact", false},
		{"longer exact wins", "/v1/chat/completions", "/v1/chat", true},
		{"shorter exact loses", "/v1/chat", "/v1/chat/completions", false},
		{"longer wildcard wins over shorter wildcard", "/v1/chat/*", "/v1/*", true},
		{"equal length does not displace", "/v1/aaa", "/v1/bbb", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := preferMoreSpecificPath(tt.candidate, tt.current); got != tt.want {
				t.Errorf("preferMoreSpecificPath(%q, %q) = %v, want %v",
					tt.candidate, tt.current, got, tt.want)
			}
		})
	}
}
