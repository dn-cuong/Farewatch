package airlines

import "testing"

func TestParseProviderTimeHandlesAmadeusStyleTimestamps(t *testing.T) {
	withOffset := parseProviderTime("2026-09-15T08:30:00-04:00")
	if withOffset.IsZero() {
		t.Fatal("expected RFC3339 timestamp with offset to parse")
	}

	withoutOffset := parseProviderTime("2026-09-15T08:30:00")
	if withoutOffset.IsZero() {
		t.Fatal("expected timestamp without timezone to parse")
	}
}