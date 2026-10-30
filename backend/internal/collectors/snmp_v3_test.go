package collectors

import (
	"testing"

	"github.com/gosnmp/gosnmp"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestNewSNMPClient_V3(t *testing.T) {
	d := &models.Device{IPAddress: "10.0.0.1", SNMPVersion: "3", SNMPv3: &models.SNMPv3Config{
		SecurityName: "monitor", SecurityLevel: models.SNMPv3AuthPriv,
		AuthProtocol: "SHA256", AuthPassphrase: "auth-secret-1",
		PrivProtocol: "AES256", PrivPassphrase: "priv-secret-1", ContextName: "ctx",
	}}
	g, err := newSNMPClient(d)
	if err != nil {
		t.Fatal(err)
	}
	if g.Version != gosnmp.Version3 || g.SecurityModel != gosnmp.UserSecurityModel || g.MsgFlags != gosnmp.AuthPriv || g.ContextName != "ctx" {
		t.Fatalf("unexpected client: version=%v model=%v flags=%v", g.Version, g.SecurityModel, g.MsgFlags)
	}
	sp := g.SecurityParameters.(*gosnmp.UsmSecurityParameters)
	if sp.UserName != "monitor" || sp.AuthenticationProtocol != gosnmp.SHA256 || sp.PrivacyProtocol != gosnmp.AES256 ||
		sp.AuthenticationPassphrase != "auth-secret-1" || sp.PrivacyPassphrase != "priv-secret-1" {
		t.Fatalf("unexpected USM params: %+v", sp)
	}
	if g.Community != "" {
		t.Fatal("v3 client must not carry a community")
	}
}

func TestNewSNMPClient_V3Errors(t *testing.T) {
	if _, err := newSNMPClient(&models.Device{SNMPVersion: "3"}); err == nil {
		t.Fatal("expected error without credentials")
	}
	bad := &models.Device{SNMPVersion: "3", SNMPv3: &models.SNMPv3Config{SecurityName: "m", SecurityLevel: models.SNMPv3AuthNoPriv, AuthProtocol: "SHA"}}
	if _, err := newSNMPClient(bad); err == nil {
		t.Fatal("expected error for a missing auth passphrase")
	}
}

func TestNewSNMPClient_V1V2c(t *testing.T) {
	g, _ := newSNMPClient(&models.Device{SNMPVersion: "1", SNMPCommunity: "ro"})
	if g.Version != gosnmp.Version1 || g.Community != "ro" {
		t.Fatalf("v1: %v %q", g.Version, g.Community)
	}
	g, _ = newSNMPClient(&models.Device{SNMPPort: 1161})
	if g.Version != gosnmp.Version2c || g.Community != "public" || g.Port != 1161 {
		t.Fatalf("v2c default: %v %q %d", g.Version, g.Community, g.Port)
	}
}
