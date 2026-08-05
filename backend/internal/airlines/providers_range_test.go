package airlines

import (
	"context"
	"testing"
)

func TestAirlineSimulatorRejectsImpossibleLongHaulForFrontier(t *testing.T) {
	frontier := airlineProvider{
		code:       "F9",
		name:       "Frontier Airlines",
		baseFare:   115,
		volatility: 0.28,
		latencyMs:  0,
		aircraft:   []string{"A320neo", "A321neo"},
		hubBias:    map[string]float64{"DEN": 0.88},
	}

	if _, err := frontier.fetchOne(context.Background(), "HAN", "LAX", "2026-09-15", nil, "economy"); err == nil {
		t.Fatal("expected impossible Frontier route to be rejected")
	}
}