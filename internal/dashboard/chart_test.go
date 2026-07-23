package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/eltonsst/tren/internal/stats"
)

func TestChartUsesExerciseAppropriateMetric(t *testing.T) {
	dataset := stats.OfflineDemo(time.Date(2026, 7, 17, 18, 0, 0, 0, time.UTC))
	tests := []struct {
		query string
		label string
		unit  string
	}{
		{"bench", "Best estimated 1RM", "kg"},
		{"weighted pull-up", "Top working load", "kg"},
		{"chin up", "Best set", "reps"},
	}
	for _, test := range tests {
		report, err := stats.BuildReport(dataset, test.query)
		if err != nil {
			t.Fatal(err)
		}
		chart, ok := buildChart(report)
		if !ok || chart.Label != test.label || chart.Unit != test.unit {
			t.Fatalf("%s chart = %#v", test.query, chart)
		}
		if len(chart.Points) != 6 || len(chart.YTicks) != 4 || len(chart.XTicks) != 3 {
			t.Fatalf("%s chart dimensions = %#v", test.query, chart)
		}
		if !strings.HasPrefix(chart.Change, "+") {
			t.Fatalf("%s change = %q", test.query, chart.Change)
		}
	}
}
