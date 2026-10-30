package handlers

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestMaskChannelConfig_HidesTelegramAndHeaders(t *testing.T) {
	cfg := map[string]any{
		"bot_token": "123:ABC",
		"headers":   map[string]any{"Authorization": "Bearer x"},
		"chat_name": "noc",
	}
	masked := maskChannelConfig(cfg)
	if masked["bot_token"] != secretPlaceholder || masked["headers"] != secretPlaceholder {
		t.Fatalf("secrets not masked: %+v", masked)
	}
	if masked["chat_name"] != "noc" {
		t.Fatalf("non-secret changed: %+v", masked)
	}
}

func TestRestoreMaskedSecrets_KeepsStoredValueWhenPlaceholderSentBack(t *testing.T) {
	stored := map[string]any{"password": "real", "smtp_host": "mail"}
	incoming := map[string]any{"password": secretPlaceholder, "smtp_host": "mail2"}
	restoreMaskedSecrets(incoming, stored)
	if incoming["password"] != "real" {
		t.Fatalf("stored secret was overwritten by the placeholder: %v", incoming["password"])
	}
	if incoming["smtp_host"] != "mail2" {
		t.Fatalf("non-secret edit lost: %v", incoming["smtp_host"])
	}

	incoming = map[string]any{"password": "new-secret"}
	restoreMaskedSecrets(incoming, stored)
	if incoming["password"] != "new-secret" {
		t.Fatal("a real new secret must be kept")
	}
}

func TestRedactDeviceSecrets(t *testing.T) {
	devices := func() []models.Device { return []models.Device{{ID: 1, SNMPCommunity: "s3cret"}} }
	viewer := httptest.NewRequest("GET", "/api/v1/devices", nil)
	viewer = viewer.WithContext(context.WithValue(viewer.Context(), auth.ClaimsKey, &auth.Claims{Role: "viewer", Permissions: []string{models.PermDevicesRead}}))
	d := devices()
	redactDeviceSecrets(viewer, d)
	if d[0].SNMPCommunity != "" {
		t.Fatal("viewer received the SNMP community")
	}

	writer := httptest.NewRequest("GET", "/api/v1/devices", nil)
	writer = writer.WithContext(context.WithValue(writer.Context(), auth.ClaimsKey, &auth.Claims{Role: "network_admin", Permissions: []string{models.PermDevicesWrite}}))
	d = devices()
	redactDeviceSecrets(writer, d)
	if d[0].SNMPCommunity != "s3cret" {
		t.Fatal("device writers need the community to edit the device")
	}
}
