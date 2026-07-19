package remote

import "testing"

func TestSummarize(t *testing.T) {
	snapshot := summarize(7,
		[]byte(`{"data":{"version":"3.7.5"}}`),
		[]byte(`{"data":[{"status":"up"},{"status":"down"},{"status":"online"}]}`),
		[]byte(`{"data":{"alerts":[{"severity":"critical"},{"severity":"warning"}]}}`),
		12.5,
	)
	if snapshot.InstanceID != 7 || snapshot.DeviceCount != 3 || snapshot.DeviceUpCount != 2 || snapshot.DeviceDownCount != 1 {
		t.Fatalf("unexpected device summary: %#v", snapshot)
	}
	if snapshot.AlertCount != 2 || snapshot.CriticalAlerts != 1 || snapshot.Version != "3.7.5" {
		t.Fatalf("unexpected alert/version summary: %#v", snapshot)
	}
	if snapshot.HealthScore < 66 || snapshot.HealthScore > 67 || snapshot.LatencyMS != 12.5 {
		t.Fatalf("unexpected health summary: %#v", snapshot)
	}
}
