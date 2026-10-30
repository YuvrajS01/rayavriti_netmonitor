package logging

import (
	"net/url"
	"strings"
	"testing"
)

func TestRedactQuery(t *testing.T) {
	t.Parallel()
	q := url.Values{
		"search":    {"core-switch"},
		"token":     {"abc"},
		"api_key":   {"k"},
		"community": {"public"},
	}
	got := redactQuery(q)
	for _, secret := range []string{"abc", "=k", "public"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redactQuery leaked %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, "search=core-switch") {
		t.Fatalf("redactQuery dropped a non-sensitive param: %s", got)
	}
}
