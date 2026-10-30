package handlers

import (
	"testing"
	"time"
)

func TestParseIDList(t *testing.T) {
	ids, err := parseIDList("3, 1,3,,2")
	if err != nil || len(ids) != 3 || ids[0] != 3 || ids[1] != 1 || ids[2] != 2 {
		t.Fatalf("parseIDList = %v, %v; want [3 1 2] deduplicated in order", ids, err)
	}
	for _, bad := range []string{"1,x", "-4", "0"} {
		if _, err := parseIDList(bad); err == nil {
			t.Errorf("parseIDList(%q) should fail", bad)
		}
	}
}

func TestPickBucket(t *testing.T) {
	tests := map[time.Duration]string{
		time.Hour: "1m", 12 * time.Hour: "5m", 2 * 24 * time.Hour: "15m", 7 * 24 * time.Hour: "1h", 90 * 24 * time.Hour: "1d",
	}
	for span, want := range tests {
		if got := pickBucket(span); got != want {
			t.Errorf("pickBucket(%v) = %s, want %s", span, got, want)
		}
	}
}
