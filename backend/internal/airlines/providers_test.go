package airlines

import "testing"

func TestAllProvidersWrapAirlineProvidersWithLiveBackends(t *testing.T) {
	ignav := NewIgnav("test-key")
	amadeus := NewAmadeus("client-id", "client-secret", "https://test.api.amadeus.com")

	providers := AllProviders(ignav, amadeus)
	if len(providers) == 0 {
		t.Fatal("expected providers")
	}

	wrapped, ok := providers[0].(*IgnavBackedProvider)
	if !ok {
		t.Fatalf("expected first provider to be Ignav-backed, got %T", providers[0])
	}
	amadeusWrapped, ok := wrapped.inner.(*AmadeusBackedProvider)
	if !ok {
		t.Fatalf("expected Ignav wrapper to wrap Amadeus-backed provider, got %T", wrapped.inner)
	}
	if _, ok := amadeusWrapped.inner.(*AirlineAPI); !ok {
		t.Fatalf("expected live wrapper chain to end in AirlineAPI fallback, got %T", amadeusWrapped.inner)
	}
}