package stats

import (
	"testing"
	"time"
)

func TestWithDemoHistoryBuildsProgressionWithoutChangingSource(t *testing.T) {
	source := testDataset()
	source.Sessions = nil
	source.Sets = nil
	now := time.Date(2026, 7, 17, 18, 0, 0, 0, time.FixedZone("Europe/Rome", 2*60*60))

	demo := WithDemoHistory(source, now)
	if !demo.Demo || len(demo.Sessions) != demoRotations*4 {
		t.Fatalf("demo = %t, sessions = %d", demo.Demo, len(demo.Sessions))
	}
	if len(source.Sessions) != 0 || len(source.Sets) != 0 {
		t.Fatal("source dataset was mutated")
	}
	report, err := BuildReport(demo, "bench")
	if err != nil {
		t.Fatal(err)
	}
	if report.SessionCount != demoRotations || report.SetCount != demoRotations*3 {
		t.Fatalf("sessions = %d, sets = %d", report.SessionCount, report.SetCount)
	}
	if report.History[0].Session.RotationNumber != demoRotations || report.History[len(report.History)-1].Session.RotationNumber != 1 {
		t.Fatalf("history order = %#v", report.History)
	}
	if report.History[0].Sets[0].Reps <= report.History[len(report.History)-1].Sets[0].Reps {
		t.Fatal("demo reps did not progress")
	}
	if report.BestLoadKG == nil || *report.BestLoadKG != 72.5 || report.PainSetCount != 1 {
		t.Fatalf("best load = %v, pain sets = %d", report.BestLoadKG, report.PainSetCount)
	}
}

func TestOfflineDemoIncludesCompleteProgram(t *testing.T) {
	demo := OfflineDemo(time.Date(2026, 7, 17, 18, 0, 0, 0, time.UTC))
	if !demo.Demo || demo.ActiveProgramVersion != "block1-r1" {
		t.Fatalf("demo metadata = %#v", demo)
	}
	if len(demo.Exercises) != 30 || len(demo.Prescriptions) != 23 {
		t.Fatalf("exercises = %d, prescriptions = %d", len(demo.Exercises), len(demo.Prescriptions))
	}
	if len(demo.Sessions) != demoRotations*4 || len(demo.Sets) == 0 {
		t.Fatalf("sessions = %d, sets = %d", len(demo.Sessions), len(demo.Sets))
	}
	report, err := BuildReport(demo, "bench")
	if err != nil || report.SessionCount != demoRotations {
		t.Fatalf("bench report = %#v, err = %v", report, err)
	}
}
