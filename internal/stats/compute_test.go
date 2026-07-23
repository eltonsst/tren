package stats

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestBuildReportCalculatesExerciseHistory(t *testing.T) {
	dataset := testDataset()
	report, err := BuildReport(dataset, "bp")
	if err != nil {
		t.Fatal(err)
	}
	if report.Exercise.ID != "bench_press" || len(report.Current) != 1 {
		t.Fatalf("exercise = %#v, current = %#v", report.Exercise, report.Current)
	}
	if report.SessionCount != 2 || report.SetCount != 4 || report.TotalReps != 19 || report.PainSetCount != 1 {
		t.Fatalf("report counts = %#v", report)
	}
	wantVolume := 70.0*10 + 72.5*9
	if math.Abs(report.LoadVolumeKG-wantVolume) > 0.001 {
		t.Fatalf("volume = %v, want %v", report.LoadVolumeKG, wantVolume)
	}
	if report.BestLoadKG == nil || *report.BestLoadKG != 72.5 {
		t.Fatalf("best load = %v", report.BestLoadKG)
	}
	wantEstimate := 72.5 * (1 + 5.0/30)
	if report.BestEstimated == nil || math.Abs(report.BestEstimated.EstimatedLoadKG-wantEstimate) > 0.001 {
		t.Fatalf("best estimate = %#v", report.BestEstimated)
	}
	if report.History[0].Session.ID != "session-new" || report.History[1].Session.ID != "session-old" {
		t.Fatalf("history = %#v", report.History)
	}
}

func TestFindExerciseReportsAmbiguousPartialMatch(t *testing.T) {
	dataset := testDataset()
	dataset.Exercises = append(dataset.Exercises, Exercise{ID: "incline_bench_press", Name: "Incline bench press", Active: true, VolumeMultiplier: 1})
	_, err := FindExercise(dataset, "press")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") || !strings.Contains(err.Error(), "Incline bench press") {
		t.Fatalf("error = %v", err)
	}
}

func TestRenderReportIncludesPrescriptionSummaryAndHistory(t *testing.T) {
	dataset := testDataset()
	report, err := BuildReport(dataset, "bench_press")
	if err != nil {
		t.Fatal(err)
	}
	output := RenderReport(dataset, report)
	for _, expected := range []string{
		"BENCH PRESS", "A1  3×3–5 @3 RIR · 72.5 kg", "Working sets   4",
		"Best e1RM", "2026-07-17 · Workout A · completed", "72.5×5 @2 p1",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("output missing %q:\n%s", expected, output)
		}
	}
}

func TestCataloguePutsCurrentProgramFirst(t *testing.T) {
	dataset := testDataset()
	dataset.Exercises = append(dataset.Exercises, Exercise{ID: "curl", Name: "Barbell curl", Active: true, VolumeMultiplier: 1})
	exercises := Catalogue(dataset)
	if len(exercises) != 2 || exercises[0].ID != "bench_press" || exercises[1].ID != "curl" {
		t.Fatalf("catalogue = %#v", exercises)
	}
}

func testDataset() Dataset {
	repMin, repMax := 3, 5
	rir := 3.0
	load := 72.5
	oldDate := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	newDate := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	zero, pain := 0, 1
	rirTwo := 2.0
	return Dataset{
		ActiveProgramVersion: "block1-r1",
		Exercises: []Exercise{{
			ID: "bench_press", Name: "Bench press", Aliases: []string{"bench", "bp"},
			LoadMode: "barbell_total", VolumeMultiplier: 1, Active: true,
		}},
		Prescriptions: []Prescription{{
			VersionID: "block1-r1", Workout: "A", Position: 1, SlotID: "A01", ExerciseID: "bench_press",
			PlannedSets: 3, RepMin: &repMin, RepMax: &repMax, RIRMin: &rir, RIRMax: &rir, SuggestedLoadKG: &load,
		}},
		Sessions: []Session{
			{ID: "session-old", ProgramVersionID: "block1-r1", Workout: "A", StartedAt: oldDate, Status: "completed"},
			{ID: "session-new", ProgramVersionID: "block1-r1", Workout: "A", StartedAt: newDate, Status: "completed"},
		},
		Sets: []Set{
			{SessionID: "session-old", ProgramVersionID: "block1-r1", ExerciseID: "bench_press", Number: 1, LoadKG: floatPointer(70), Reps: 5, RIR: &rirTwo, Pain: &zero, LoggedAt: oldDate},
			{SessionID: "session-old", ProgramVersionID: "block1-r1", ExerciseID: "bench_press", Number: 2, LoadKG: floatPointer(70), Reps: 5, RIR: &rirTwo, Pain: &zero, LoggedAt: oldDate},
			{SessionID: "session-new", ProgramVersionID: "block1-r1", ExerciseID: "bench_press", Number: 1, LoadKG: &load, Reps: 4, RIR: &rirTwo, Pain: &zero, LoggedAt: newDate},
			{SessionID: "session-new", ProgramVersionID: "block1-r1", ExerciseID: "bench_press", Number: 2, LoadKG: &load, Reps: 5, RIR: &rirTwo, Pain: &pain, LoggedAt: newDate},
		},
	}
}

func floatPointer(value float64) *float64 { return &value }
