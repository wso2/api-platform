package handlers

import (
	"testing"
)

func TestUuidToOpenAPIUUID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid UUID",
			input:   "550e8400-e29b-41d4-a716-446655440000",
			wantErr: false,
		},
		{
			name:    "invalid UUID format",
			input:   "not-a-valid-uuid",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "UUID without hyphens",
			input:   "550e8400e29b41d4a716446655440000",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := uuidToOpenAPIUUID(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("uuidToOpenAPIUUID() expected error but got none")
				}
				if result != nil {
					t.Errorf("uuidToOpenAPIUUID() expected nil result on error, got %v", result)
				}
			} else {
				if err != nil {
					t.Errorf("uuidToOpenAPIUUID() unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("uuidToOpenAPIUUID() expected non-nil result")
				}
			}
		})
	}
}

func TestConvertHandleToUUID(t *testing.T) {
	tests := []struct {
		name    string
		handle  string
		wantNil bool
	}{
		{
			name:    "valid handle UUID",
			handle:  "550e8400-e29b-41d4-a716-446655440000",
			wantNil: false,
		},
		{
			name:    "invalid handle",
			handle:  "invalid-handle",
			wantNil: true,
		},
		{
			name:    "empty handle",
			handle:  "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertHandleToUUID(tt.handle)

			if tt.wantNil {
				if result != nil {
					t.Errorf("convertHandleToUUID() expected nil, got %v", result)
				}
			} else {
				if result == nil {
					t.Errorf("convertHandleToUUID() expected non-nil result")
				}
			}
		})
	}
}

func TestStatusPtr(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "status string",
			input: "active",
		},
		{
			name:  "long string",
			input: "this is a longer status string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := statusPtr(tt.input)

			if result == nil {
				t.Errorf("statusPtr() returned nil")
			}
			if *result != tt.input {
				t.Errorf("statusPtr() = %v, want %v", *result, tt.input)
			}
		})
	}
}

func TestIntPtr(t *testing.T) {
	tests := []struct {
		name  string
		input int
	}{
		{
			name:  "zero",
			input: 0,
		},
		{
			name:  "positive number",
			input: 42,
		},
		{
			name:  "negative number",
			input: -10,
		},
		{
			name:  "large number",
			input: 9999999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := intPtr(tt.input)

			if result == nil {
				t.Errorf("intPtr() returned nil")
			}
			if *result != tt.input {
				t.Errorf("intPtr() = %v, want %v", *result, tt.input)
			}
		})
	}
}
