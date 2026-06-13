package logging

import (
	"testing"
)

func TestRedactPasswords(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "password field",
			input:    `{"password":"hunter2"}`,
			expected: `{"password":"***"}`,
		},
		{
			name:     "no password field",
			input:    `{"username":"admin"}`,
			expected: `{"username":"admin"}`,
		},
		{
			name:     "multiple fields with password",
			input:    `{"username":"admin","password":"secret123","extra":"data"}`,
			expected: `{"username":"admin","password":"***","extra":"data"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := redactPasswords(tt.input)
			if result != tt.expected {
				t.Errorf("redactPasswords(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDBLogger_LogQuery(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	dbLogger := NewDBLogger(logger, 100)
	if dbLogger == nil {
		t.Fatal("NewDBLogger returned nil")
	}
}
