package vendorid

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIdentify_PrefersSNMPOverMAC(t *testing.T) {
	r := Identify(Evidence{MACAddress: "00:27:22:01:02:03", SNMPSysObjectID: "1.3.6.1.4.1.9.1.1", HTTPTitle: "Cisco Catalyst"})
	assert.Equal(t, "Cisco", r.Vendor)
	assert.Equal(t, "high", r.Confidence)
	assert.Contains(t, r.Sources, "snmp_oid")
}

func TestIdentify_CameraAndBiometricFingerprints(t *testing.T) {
	assert.Equal(t, "Hikvision", Identify(Evidence{HTTPTitle: "Hikvision Network Camera"}).Vendor)
	r := Identify(Evidence{SNMPDescription: "ZKTeco biometric attendance terminal"})
	assert.Equal(t, "ZKTeco", r.Vendor)
	assert.Equal(t, "medium", r.Confidence)
}

func TestIdentify_MACFallbackAndUnknown(t *testing.T) {
	r := Identify(Evidence{MACAddress: "24:5A:4C:00:00:01"})
	assert.Equal(t, "Ubiquiti", r.Vendor)
	assert.Equal(t, "low", r.Confidence)
	assert.Empty(t, Identify(Evidence{HTTPTitle: "Internal dashboard"}).Vendor)
}
