package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDevice_JSONSerialization(t *testing.T) {
	t.Parallel()
	now := time.Now().Truncate(time.Second)
	locID := int64(5)
	parentID := int64(1)
	device := Device{
		ID:                 1,
		Name:               "Server-1",
		IPAddress:          "192.168.1.100",
		Protocol:           "ping",
		Port:               0,
		Enabled:            true,
		Status:             "up",
		Tags:               []string{"production", "web"},
		SNMPCommunity:      "public",
		SNMPVersion:        "2c",
		SNMPPort:           161,
		HTTPPath:           "/health",
		HTTPExpectedStatus: 200,
		Interval:           60,
		CreatedAt:          now,
		UpdatedAt:          now,
		LocationID:         &locID,
		ParentDeviceID:     &parentID,
		RackPosition:       "A1",
		AssetTag:           "IT-001",
		MACAddress:         "AA:BB:CC:DD:EE:FF",
		Manufacturer:       "Cisco",
		Model:              "2960",
		DeviceCategory:     "switch",
		Notes:              "Main switch",
	}

	b, err := json.Marshal(device)
	require.NoError(t, err)

	var decoded Device
	require.NoError(t, json.Unmarshal(b, &decoded))
	assert.Equal(t, device.ID, decoded.ID)
	assert.Equal(t, device.Name, decoded.Name)
	assert.Equal(t, device.IPAddress, decoded.IPAddress)
	assert.Equal(t, device.Tags, decoded.Tags)
	assert.Equal(t, device.LocationID, decoded.LocationID)
	assert.Equal(t, device.ParentDeviceID, decoded.ParentDeviceID)
}

func TestDevice_Tags_NilSlice(t *testing.T) {
	t.Parallel()
	device := Device{ID: 1, Name: "Server-1"}
	b, err := json.Marshal(device)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"tags":null`)
}

func TestMetric_NilOptionalFields(t *testing.T) {
	t.Parallel()
	metric := Metric{ID: 1, DeviceID: 1, Status: "up"}
	b, err := json.Marshal(metric)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "responseTime")
	assert.NotContains(t, string(b), "packetLoss")
}

func TestSensor_ConfigMap(t *testing.T) {
	t.Parallel()
	sensor := Sensor{
		ID:       1,
		DeviceID: 1,
		Name:     "CPU Sensor",
		Type:     "system",
		Config:   map[string]any{"threshold": 5, "interval": 30},
	}
	b, err := json.Marshal(sensor)
	require.NoError(t, err)

	var decoded Sensor
	require.NoError(t, json.Unmarshal(b, &decoded))
	assert.Equal(t, float64(5), decoded.Config["threshold"])
	assert.Equal(t, float64(30), decoded.Config["interval"])
}

func TestUser_PasswordHash_NotSerialized(t *testing.T) {
	t.Parallel()
	user := User{ID: 1, Username: "admin", PasswordHash: "secret-hash"}
	b, err := json.Marshal(user)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "secret-hash")
	assert.NotContains(t, string(b), "passwordHash")
}

func TestAPIKey_KeyHash_NotSerialized(t *testing.T) {
	t.Parallel()
	key := APIKey{ID: 1, KeyHash: "secret-hash"}
	b, err := json.Marshal(key)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "secret-hash")
}

func TestFlow_JSONSerialization(t *testing.T) {
	t.Parallel()
	flow := Flow{
		ID:       1,
		SrcIP:    "10.0.0.1",
		DstIP:    "192.168.1.1",
		SrcPort:  12345,
		DstPort:  80,
		Protocol: "TCP",
		Bytes:    1024,
		Packets:  10,
		Duration: 1.5,
	}
	b, err := json.Marshal(flow)
	require.NoError(t, err)

	var decoded Flow
	require.NoError(t, json.Unmarshal(b, &decoded))
	assert.Equal(t, flow.SrcIP, decoded.SrcIP)
	assert.Equal(t, flow.Bytes, decoded.Bytes)
}

func TestDashboard_RawMessage(t *testing.T) {
	t.Parallel()
	layout := json.RawMessage(`{"grid":{"cols":12,"rows":6}}`)
	dashboard := Dashboard{ID: 1, Name: "Main", Layout: layout}
	b, err := json.Marshal(dashboard)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"grid"`)
}

func TestPagedResult(t *testing.T) {
	t.Parallel()
	result := PagedResult[Device]{
		Data:     []Device{{ID: 1, Name: "Server-1"}, {ID: 2, Name: "Server-2"}},
		Total:    10,
		Page:     1,
		PageSize: 2,
	}
	b, err := json.Marshal(result)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"total":10`)
	assert.Contains(t, string(b), `"page":1`)
}
