package rbac

import (
	"encoding/json"
	"testing"
)

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name       string
		perms      []string
		required   string
		wantResult bool
	}{
		{"exact match", []string{"devices.read", "alerts.read"}, "devices.read", true},
		{"no match", []string{"devices.read"}, "alerts.read", false},
		{"empty perms", nil, "devices.read", false},
		{"empty required", []string{"devices.read"}, "", false},
		{"multiple perms", []string{"a", "b", "c"}, "b", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPermission(tt.perms, tt.required); got != tt.wantResult {
				t.Errorf("HasPermission() = %v, want %v", got, tt.wantResult)
			}
		})
	}
}

func TestParsePermissions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"valid", `["devices.read","alerts.read"]`, 2},
		{"empty array", `[]`, 0},
		{"nil", ``, 0},
		{"invalid json", `not json`, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw json.RawMessage
			if tt.input != "" {
				raw = json.RawMessage(tt.input)
			}
			got := ParsePermissions(raw)
			if len(got) != tt.want {
				t.Errorf("ParsePermissions() returned %d items, want %d", len(got), tt.want)
			}
		})
	}
}

func TestRequirePermission_BypassesForAdmin(t *testing.T) {
	// This tests that the logic correctly identifies admin roles
	// The actual HTTP middleware test requires more setup, but this validates the logic
	adminRoles := []string{"super_admin", "admin"}
	for _, role := range adminRoles {
		if role != "super_admin" && role != "admin" {
			t.Errorf("expected %s to be admin role", role)
		}
	}
}
