package dashboard

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/eltonsst/tren/internal/stats"
)

type chartView struct {
	Label      string
	Unit       string
	Change     string
	Polyline   string
	Points     []chartPointView
	YTicks     []chartTickView
	XTicks     []chartTickView
	Accessible string
}

type chartPointView struct {
	X     string
	Y     string
	Title string
}

type chartTickView struct {
	Position string
	Label    string
}

type seriesPoint struct {
	date  time.Time
	value float64
}

func buildChart(report stats.Report) (chartView, bool) {
	label, unit, sessionValue := chartMetric(report.Exercise)
	var points []seriesPoint
	for index := len(report.History) - 1; index >= 0; index-- {
		performance := report.History[index]
		value, ok := sessionValue(performance)
		if ok {
			points = append(points, seriesPoint{date: performance.Session.StartedAt, value: value})
		}
	}
	if len(points) < 2 {
		return chartView{}, false
	}
	if len(points) > 24 {
		points = points[len(points)-24:]
	}
	return scaleChart(label, unit, points), true
}

func scaleChart(label, unit string, source []seriesPoint) chartView {
	const left, right, top, bottom = 72.0, 24.0, 24.0, 48.0
	const width, height = 1000.0, 320.0
	plotWidth, plotHeight := width-left-right, height-top-bottom
	minimum, maximum := source[0].value, source[0].value
	for _, point := range source[1:] {
		minimum = math.Min(minimum, point.value)
		maximum = math.Max(maximum, point.value)
	}
	span := maximum - minimum
	if span == 0 {
		span = math.Max(1, math.Abs(maximum)*0.1)
	}
	minimum = math.Max(0, minimum-span*0.12)
	maximum += span * 0.12
	start, end := source[0].date, source[len(source)-1].date
	timeSpan := end.Sub(start).Seconds()
	if timeSpan <= 0 {
		timeSpan = 1
	}
	valueSpan := maximum - minimum
	if valueSpan <= 0 {
		valueSpan = 1
	}
	chart := chartView{Label: label, Unit: unit, Accessible: label + " progression over time"}
	var coordinates []string
	for _, point := range source {
		x := left + point.date.Sub(start).Seconds()/timeSpan*plotWidth
		y := top + (maximum-point.value)/valueSpan*plotHeight
		coordinates = append(coordinates, fmt.Sprintf("%.1f,%.1f", x, y))
		chart.Points = append(chart.Points, chartPointView{
			X: fmt.Sprintf("%.1f", x), Y: fmt.Sprintf("%.1f", y),
			Title: point.date.Format("02 Jan 2006") + " · " + number(point.value) + " " + unit,
		})
	}
	chart.Polyline = strings.Join(coordinates, " ")
	change := source[len(source)-1].value - source[0].value
	if change >= 0 {
		chart.Change = "+" + number(change) + " " + unit
	} else {
		chart.Change = number(change) + " " + unit
	}
	for index := 0; index < 4; index++ {
		ratio := float64(index) / 3
		value := maximum - ratio*valueSpan
		y := top + ratio*plotHeight
		chart.YTicks = append(chart.YTicks, chartTickView{Position: fmt.Sprintf("%.1f", y), Label: number(value)})
	}
	for index, ratio := range []float64{0, 0.5, 1} {
		x := left + ratio*plotWidth
		date := start.Add(time.Duration(ratio * float64(end.Sub(start))))
		label := date.Format("02 Jan")
		if index == 1 && len(source) < 3 {
			continue
		}
		chart.XTicks = append(chart.XTicks, chartTickView{Position: fmt.Sprintf("%.1f", x), Label: label})
	}
	return chart
}

type sessionMetric func(stats.SessionPerformance) (float64, bool)

func chartMetric(exercise stats.Exercise) (string, string, sessionMetric) {
	if usesEstimatedOneRepMax(exercise.ID) {
		return "Best estimated 1RM", "kg", sessionEstimatedOneRepMax
	}
	if !strings.Contains(exercise.LoadMode, "bodyweight") && exercise.LoadMode != "duration" && exercise.LoadMode != "other" {
		return "Top working load", "kg", sessionTopLoad
	}
	return "Best set", "reps", sessionBestReps
}

func usesEstimatedOneRepMax(exerciseID string) bool {
	switch exerciseID {
	case "bench_press", "paused_bench", "back_squat", "paused_squat", "romanian_deadlift",
		"barbell_hip_thrust", "barbell_overhead_press", "strict_barbell_row":
		return true
	default:
		return false
	}
}

func sessionEstimatedOneRepMax(performance stats.SessionPerformance) (float64, bool) {
	best, found := 0.0, false
	for _, set := range performance.Sets {
		if set.LoadKG == nil || set.Reps < 1 || set.Reps > 15 {
			continue
		}
		estimate := *set.LoadKG * (1 + float64(set.Reps)/30)
		if !found || estimate > best {
			best, found = estimate, true
		}
	}
	return best, found
}

func sessionTopLoad(performance stats.SessionPerformance) (float64, bool) {
	best, found := 0.0, false
	for _, set := range performance.Sets {
		if set.LoadKG != nil && (!found || *set.LoadKG > best) {
			best, found = *set.LoadKG, true
		}
	}
	return best, found
}

func sessionBestReps(performance stats.SessionPerformance) (float64, bool) {
	best := 0
	for _, set := range performance.Sets {
		if set.Reps > best {
			best = set.Reps
		}
	}
	return float64(best), best > 0
}
