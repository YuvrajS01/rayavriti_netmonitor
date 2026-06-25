package rbac

import (
	"testing"
)

func TestFilterDeviceQuery_NilScope(t *testing.T) {
	base := "SELECT * FROM devices d"
	got := FilterDeviceQuery(nil, base)
	if got != base {
		t.Errorf("nil scope should return base query, got %q", got)
	}
}

func TestFilterDeviceQuery_NotScoped(t *testing.T) {
	base := "SELECT * FROM devices d"
	sc := &ScopeContext{IsScoped: false}
	got := FilterDeviceQuery(sc, base)
	if got != base {
		t.Errorf("unscoped context should return base query, got %q", got)
	}
}

func TestFilterDeviceQuery_EmptyScopes(t *testing.T) {
	base := "SELECT * FROM devices d"
	sc := &ScopeContext{IsScoped: true, Scopes: []UserScope{}}
	got := FilterDeviceQuery(sc, base)
	if got != base {
		t.Errorf("empty scopes should return base query, got %q", got)
	}
}

func TestFilterDeviceQuery_LocationOnly(t *testing.T) {
	base := "SELECT * FROM devices d"
	sc := &ScopeContext{
		IsScoped: true,
		Scopes:   []UserScope{{Type: "location", Value: "1"}},
	}
	got := FilterDeviceQuery(sc, base)
	want := base + " AND (d.location_id IN (1))"
	if got != want {
		t.Errorf("location scope:\n  got  %q\n  want %q", got, want)
	}
}

func TestFilterDeviceQuery_MultipleLocations(t *testing.T) {
	base := "SELECT * FROM devices d"
	sc := &ScopeContext{
		IsScoped: true,
		Scopes: []UserScope{
			{Type: "location", Value: "1"},
			{Type: "location", Value: "2"},
			{Type: "location", Value: "3"},
		},
	}
	got := FilterDeviceQuery(sc, base)
	want := base + " AND (d.location_id IN (1,2,3))"
	if got != want {
		t.Errorf("multiple locations:\n  got  %q\n  want %q", got, want)
	}
}

func TestFilterDeviceQuery_SubnetOnly(t *testing.T) {
	base := "SELECT * FROM devices d"
	sc := &ScopeContext{
		IsScoped: true,
		Scopes:   []UserScope{{Type: "subnet", Value: "10.0.0.0/24"}},
	}
	got := FilterDeviceQuery(sc, base)
	want := base + " AND (d.ip_address >>= '10.0.0.0/24')"
	if got != want {
		t.Errorf("subnet scope:\n  got  %q\n  want %q", got, want)
	}
}

func TestFilterDeviceQuery_MultipleSubnets(t *testing.T) {
	base := "SELECT * FROM devices d"
	sc := &ScopeContext{
		IsScoped: true,
		Scopes: []UserScope{
			{Type: "subnet", Value: "10.0.0.0/24"},
			{Type: "subnet", Value: "192.168.1.0/24"},
		},
	}
	got := FilterDeviceQuery(sc, base)
	want := base + " AND (d.ip_address >>= '10.0.0.0/24' OR d.ip_address >>= '192.168.1.0/24')"
	if got != want {
		t.Errorf("multiple subnets:\n  got  %q\n  want %q", got, want)
	}
}

func TestFilterDeviceQuery_MixedScopes(t *testing.T) {
	base := "SELECT * FROM devices d"
	sc := &ScopeContext{
		IsScoped: true,
		Scopes: []UserScope{
			{Type: "location", Value: "5"},
			{Type: "subnet", Value: "10.0.0.0/24"},
		},
	}
	got := FilterDeviceQuery(sc, base)
	want := base + " AND (d.location_id IN (5) OR d.ip_address >>= '10.0.0.0/24')"
	if got != want {
		t.Errorf("mixed scopes:\n  got  %q\n  want %q", got, want)
	}
}

func TestFilterDeviceQuery_UnrecognizedScopeType(t *testing.T) {
	base := "SELECT * FROM devices d"
	sc := &ScopeContext{
		IsScoped: true,
		Scopes:   []UserScope{{Type: "unknown", Value: "x"}},
	}
	got := FilterDeviceQuery(sc, base)
	want := base + " AND 1=0"
	if got != want {
		t.Errorf("unrecognized scope type should produce AND 1=0:\n  got  %q\n  want %q", got, want)
	}
}

func TestFilterDeviceQuery_SubnetEscapesQuotes(t *testing.T) {
	base := "SELECT * FROM devices d"
	sc := &ScopeContext{
		IsScoped: true,
		Scopes:   []UserScope{{Type: "subnet", Value: "10.0.0.0/24'; DROP TABLE--"}},
	}
	got := FilterDeviceQuery(sc, base)
	if got != base+" AND (d.ip_address >>= '10.0.0.0/24''; DROP TABLE--')" {
		t.Errorf("subnet with quotes not properly escaped: %q", got)
	}
}

func TestFilterAlertQuery_NilScope(t *testing.T) {
	base := "SELECT * FROM alerts a"
	got := FilterAlertQuery(nil, base)
	if got != base {
		t.Errorf("nil scope should return base query, got %q", got)
	}
}

func TestFilterAlertQuery_NotScoped(t *testing.T) {
	base := "SELECT * FROM alerts a"
	sc := &ScopeContext{IsScoped: false}
	got := FilterAlertQuery(sc, base)
	if got != base {
		t.Errorf("unscoped context should return base query, got %q", got)
	}
}

func TestFilterAlertQuery_EmptyScopes(t *testing.T) {
	base := "SELECT * FROM alerts a"
	sc := &ScopeContext{IsScoped: true, Scopes: []UserScope{}}
	got := FilterAlertQuery(sc, base)
	if got != base {
		t.Errorf("empty scopes should return base query, got %q", got)
	}
}

func TestFilterAlertQuery_LocationOnly(t *testing.T) {
	base := "SELECT * FROM alerts a"
	sc := &ScopeContext{
		IsScoped: true,
		Scopes:   []UserScope{{Type: "location", Value: "7"}},
	}
	got := FilterAlertQuery(sc, base)
	want := base + " AND (a.location_id IN (7))"
	if got != want {
		t.Errorf("location scope:\n  got  %q\n  want %q", got, want)
	}
}

func TestFilterAlertQuery_MultipleLocations(t *testing.T) {
	base := "SELECT * FROM alerts a"
	sc := &ScopeContext{
		IsScoped: true,
		Scopes: []UserScope{
			{Type: "location", Value: "1"},
			{Type: "location", Value: "2"},
		},
	}
	got := FilterAlertQuery(sc, base)
	want := base + " AND (a.location_id IN (1,2))"
	if got != want {
		t.Errorf("multiple locations:\n  got  %q\n  want %q", got, want)
	}
}

func TestFilterAlertQuery_SubnetOnly(t *testing.T) {
	base := "SELECT * FROM alerts a"
	sc := &ScopeContext{
		IsScoped: true,
		Scopes:   []UserScope{{Type: "subnet", Value: "10.0.0.0/24"}},
	}
	got := FilterAlertQuery(sc, base)
	want := base + " AND (a.device_id IN (SELECT id FROM devices WHERE ip_address >>= '10.0.0.0/24'))"
	if got != want {
		t.Errorf("subnet scope:\n  got  %q\n  want %q", got, want)
	}
}

func TestFilterAlertQuery_MultipleSubnets(t *testing.T) {
	base := "SELECT * FROM alerts a"
	sc := &ScopeContext{
		IsScoped: true,
		Scopes: []UserScope{
			{Type: "subnet", Value: "10.0.0.0/24"},
			{Type: "subnet", Value: "192.168.1.0/24"},
		},
	}
	got := FilterAlertQuery(sc, base)
	want := base + " AND (a.device_id IN (SELECT id FROM devices WHERE ip_address >>= '10.0.0.0/24') OR a.device_id IN (SELECT id FROM devices WHERE ip_address >>= '192.168.1.0/24'))"
	if got != want {
		t.Errorf("multiple subnets:\n  got  %q\n  want %q", got, want)
	}
}

func TestFilterAlertQuery_MixedScopes(t *testing.T) {
	base := "SELECT * FROM alerts a"
	sc := &ScopeContext{
		IsScoped: true,
		Scopes: []UserScope{
			{Type: "location", Value: "3"},
			{Type: "subnet", Value: "10.0.0.0/24"},
		},
	}
	got := FilterAlertQuery(sc, base)
	want := base + " AND (a.location_id IN (3) OR a.device_id IN (SELECT id FROM devices WHERE ip_address >>= '10.0.0.0/24'))"
	if got != want {
		t.Errorf("mixed scopes:\n  got  %q\n  want %q", got, want)
	}
}

func TestFilterAlertQuery_UnrecognizedScopeType(t *testing.T) {
	base := "SELECT * FROM alerts a"
	sc := &ScopeContext{
		IsScoped: true,
		Scopes:   []UserScope{{Type: "unknown", Value: "x"}},
	}
	got := FilterAlertQuery(sc, base)
	want := base + " AND 1=0"
	if got != want {
		t.Errorf("unrecognized scope type should produce AND 1=0:\n  got  %q\n  want %q", got, want)
	}
}

func TestFilterAlertQuery_SubnetEscapesQuotes(t *testing.T) {
	base := "SELECT * FROM alerts a"
	sc := &ScopeContext{
		IsScoped: true,
		Scopes:   []UserScope{{Type: "subnet", Value: "10.0.0.0/24'; DROP TABLE--"}},
	}
	got := FilterAlertQuery(sc, base)
	want := base + " AND (a.device_id IN (SELECT id FROM devices WHERE ip_address >>= '10.0.0.0/24''; DROP TABLE--'))"
	if got != want {
		t.Errorf("subnet with quotes not properly escaped:\n  got  %q\n  want %q", got, want)
	}
}
