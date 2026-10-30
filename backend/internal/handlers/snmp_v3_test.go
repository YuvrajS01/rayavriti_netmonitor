package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func storedV3() *models.SNMPv3Config {
	return &models.SNMPv3Config{
		SecurityName: "monitor", SecurityLevel: models.SNMPv3AuthPriv,
		AuthProtocol: "SHA256", AuthPassphrase: "auth-secret-1",
		PrivProtocol: "AES", PrivPassphrase: "priv-secret-1",
	}
}

func withRole(r *http.Request, role, perm string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), auth.ClaimsKey, &auth.Claims{Role: role, Permissions: []string{perm}}))
}

func TestRedactDevice_SNMPv3(t *testing.T) {
	v3 := storedV3()
	d := &models.Device{ID: 1, SNMPv3: v3}
	redactDevice(withRole(httptest.NewRequest("GET", "/", nil), "network_admin", models.PermDevicesWrite), d)
	if d.SNMPv3.AuthPassphrase != secretPlaceholder || d.SNMPv3.PrivPassphrase != secretPlaceholder {
		t.Fatalf("writer received v3 passphrases: %+v", d.SNMPv3)
	}
	if d.SNMPv3.SecurityName != "monitor" || d.SNMPv3.AuthProtocol != "SHA256" {
		t.Fatalf("non-secret v3 fields lost: %+v", d.SNMPv3)
	}
	if v3.AuthPassphrase != "auth-secret-1" {
		t.Fatal("redaction mutated the caller's config")
	}

	d = &models.Device{ID: 1, SNMPv3: storedV3()}
	redactDevice(withRole(httptest.NewRequest("GET", "/", nil), "viewer", models.PermDevicesRead), d)
	if d.SNMPv3 != nil {
		t.Fatal("viewer received the SNMPv3 block")
	}
}

func TestMergeSNMPv3_PlaceholderKeepsStoredSecret(t *testing.T) {
	in := storedV3()
	in.AuthPassphrase = secretPlaceholder
	in.PrivPassphrase = "new-priv-secret"
	got := mergeSNMPv3(in, storedV3())
	if got.AuthPassphrase != "auth-secret-1" || got.PrivPassphrase != "new-priv-secret" {
		t.Fatalf("unexpected merge: %+v", got)
	}
	// With nothing stored, a placeholder is not a passphrase.
	if got := mergeSNMPv3(in, nil); got.AuthPassphrase != "" {
		t.Fatalf("placeholder stored as a passphrase: %q", got.AuthPassphrase)
	}
}

func TestDeviceUpdate_SNMPv3RoundTrip(t *testing.T) {
	var saved *models.Device
	db := &mockDB{
		getDeviceFn: func(ctx context.Context, id int64) (*models.Device, error) {
			return &models.Device{ID: id, Name: "sw", IPAddress: "10.0.0.1", Protocol: "snmp", SNMPVersion: "3", SNMPv3: storedV3()}, nil
		},
		updateDeviceFn: func(ctx context.Context, id int64, d *models.Device) (*models.Device, error) {
			cp := *d
			saved = &cp
			return d, nil
		},
	}
	h := NewDeviceHandler(db)
	// The edit form sends the masked block back with only the context changed.
	body := `{"snmpV3":{"securityName":"monitor","securityLevel":"authPriv","authProtocol":"SHA256","authPassphrase":"********","privProtocol":"AES","privPassphrase":"********","contextName":"vlan10"}}`
	w, req := makeRequestWithParams("PUT", "/api/v1/devices/1", body, "id", "1")
	h.Update(w, withRole(req, "admin", models.PermDevicesWrite))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if saved.SNMPv3.AuthPassphrase != "auth-secret-1" || saved.SNMPv3.PrivPassphrase != "priv-secret-1" || saved.SNMPv3.ContextName != "vlan10" {
		t.Fatalf("stored config wrong: %+v", saved.SNMPv3)
	}
	if strings.Contains(w.Body.String(), "secret-1") {
		t.Fatalf("response leaked a passphrase: %s", w.Body.String())
	}
}

func TestDeviceUpdate_SNMPv3Validation(t *testing.T) {
	db := &mockDB{
		getDeviceFn: func(ctx context.Context, id int64) (*models.Device, error) {
			return &models.Device{ID: id, Name: "sw", IPAddress: "10.0.0.1", Protocol: "snmp", SNMPVersion: "2c"}, nil
		},
		updateDeviceFn: func(ctx context.Context, id int64, d *models.Device) (*models.Device, error) {
			return d, nil
		},
	}
	h := NewDeviceHandler(db)
	cases := map[string]string{
		"v3 without credentials": `{"snmpVersion":"3"}`,
		"short passphrase":       `{"snmpVersion":"3","snmpV3":{"securityName":"m","securityLevel":"authNoPriv","authProtocol":"SHA","authPassphrase":"short"}}`,
		"unknown protocol":       `{"snmpVersion":"3","snmpV3":{"securityName":"m","securityLevel":"authNoPriv","authProtocol":"SHA1","authPassphrase":"long-enough"}}`,
		"unknown version":        `{"snmpVersion":"4"}`,
	}
	for name, body := range cases {
		w, req := makeRequestWithParams("PUT", "/api/v1/devices/1", body, "id", "1")
		h.Update(w, withRole(req, "admin", models.PermDevicesWrite))
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", name, w.Code)
		}
	}

	ok := `{"snmpVersion":"3","snmpV3":{"securityName":"m","securityLevel":"authNoPriv","authProtocol":"SHA","authPassphrase":"long-enough"}}`
	w, req := makeRequestWithParams("PUT", "/api/v1/devices/1", ok, "id", "1")
	h.Update(w, withRole(req, "admin", models.PermDevicesWrite))
	if w.Code != http.StatusOK {
		t.Fatalf("valid authNoPriv: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data models.Device `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.SNMPv3 == nil || resp.Data.SNMPv3.AuthPassphrase != secretPlaceholder {
		t.Fatalf("expected masked v3 in response, got %+v", resp.Data.SNMPv3)
	}
}
