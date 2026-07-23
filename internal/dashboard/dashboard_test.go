package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eltonsst/tren/internal/stats"
)

func TestDynamicDashboardReloadsDataset(t *testing.T) {
	loads := 0
	handler, err := NewDynamic(func(context.Context) (stats.Dataset, error) {
		loads++
		return stats.OfflineDemo(time.Date(2026, 7, 17, 18, 0, 0, 0, time.UTC)), nil
	}, "bench")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d", response.Code)
		}
	}
	if loads != 2 {
		t.Fatalf("loads = %d, want 2", loads)
	}
}

func TestExercisePageRendersDashboardAndSVGChart(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/exercise/bench_press", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, expected := range []string{
		"Bench press · Tren", "Demo data", "Current prescription", "72.5 kg",
		"Best estimated 1RM", "<svg", "<polyline", "Recent history", "01 Jun 2026",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("page missing %q", expected)
		}
	}
	if strings.Contains(body, "<script") {
		t.Fatal("dashboard unexpectedly contains JavaScript")
	}
}

func TestSearchFiltersExerciseNavigation(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/exercise/bench_press?q=bench", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	if !strings.Contains(body, "Bench press") || !strings.Contains(body, "Paused bench press") {
		t.Fatalf("bench matches missing:\n%s", body)
	}
	if strings.Contains(body, ">Back squat</strong>") {
		t.Fatal("search did not filter Back squat from navigation")
	}
}

func TestStylesAreEmbeddedAndResponsive(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/assets/styles.css", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("status = %d, content type = %q", response.Code, response.Header().Get("Content-Type"))
	}
	for _, expected := range []string{"@media (max-width: 820px)", ".progress-chart", "grid-template-columns"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("styles missing %q", expected)
		}
	}
}

func TestUnknownDashboardPathReturnsNotFound(t *testing.T) {
	handler := testHandler(t)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
}

func testHandler(t *testing.T) *Handler {
	t.Helper()
	dataset := stats.OfflineDemo(time.Date(2026, 7, 17, 18, 0, 0, 0, time.UTC))
	handler, err := New(dataset, "bench")
	if err != nil {
		t.Fatal(err)
	}
	return handler
}
