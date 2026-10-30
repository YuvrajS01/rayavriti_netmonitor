package handlers

import (
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestNeighborLinks(t *testing.T) {
	id := func(v int64) *int64 { return &v }
	links := neighborLinks([]models.LLDPNeighbor{
		// Device 1 has two parallel links to device 2 and one unmanaged neighbour.
		{DeviceID: 1, LocalPort: "Gi0/1", RemoteDeviceID: id(2), RemoteDevice: "access-1", RemotePortID: "Gi1/0/47"},
		{DeviceID: 1, LocalPort: "Gi0/2", RemoteDeviceID: id(2), RemoteDevice: "access-1", RemotePortID: "Gi1/0/48", RemotePortDesc: "uplink-b"},
		{DeviceID: 1, LocalPort: "Gi0/9", RemoteSysName: "ap-lobby", RemotePortID: "eth0"},
		{DeviceID: 1, LocalPort: "Gi0/10", RemoteChassisID: "00:11:22:33:44:55", RemotePortID: "1"},
		// Device 2's reports of the same links are dropped.
		{DeviceID: 2, LocalPort: "Gi1/0/47", RemoteDeviceID: id(1), RemoteDevice: "core", RemotePortID: "Gi0/1"},
		{DeviceID: 2, LocalPort: "Gi1/0/48", RemoteDeviceID: id(1), RemoteDevice: "core", RemotePortID: "Gi0/2"},
	})
	if len(links) != 4 {
		t.Fatalf("expected 4 links, got %d: %+v", len(links), links)
	}
	if links[1].ToPort != "uplink-b" || links[2].ToName != "ap-lobby" || links[2].ToDeviceID != nil || links[3].ToName != "00:11:22:33:44:55" {
		t.Fatalf("unexpected links: %+v", links)
	}
}
