package progression

import (
	"strings"
	"testing"
	"time"

	"github.com/eltonsst/tren/internal/domain"
)

func TestNextVersionRestoresFirstRotationTargets(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	current := completedVersion("block1-r1", 1)
	prescription := basePrescription()
	prescription.VersionID = current.ID
	prescription.PlannedSets = 3
	prescription.TargetSets = 4
	prescription.PlannedRIRMin = float64Pointer(3)
	prescription.PlannedRIRMax = float64Pointer(3)
	prescription.TargetRIRMin = float64Pointer(2)
	prescription.TargetRIRMax = float64Pointer(2)
	prescription.SuggestedLoadKG = nil
	sets := successfulSets(current.ID, prescription, 3, 72.5)

	got, err := NextVersion(current, []domain.Prescription{prescription}, sets, now)
	if err != nil {
		t.Fatalf("NextVersion() error = %v", err)
	}
	if got.Version.ID != "block1-r2" || got.Version.Status != domain.ProgramActive {
		t.Fatalf("Version = %#v", got.Version)
	}
	next := got.Prescriptions[0]
	if next.PlannedSets != 4 || *next.PlannedRIRMin != 2 || *next.SuggestedLoadKG != 72.5 {
		t.Fatalf("Prescription = %#v", next)
	}
	if got.Decisions[0].LoadIncreased || !strings.Contains(got.Decisions[0].Reason, "reduced volume") {
		t.Fatalf("Decision = %#v", got.Decisions[0])
	}
}

func TestNextVersionIncreasesEligibleLoad(t *testing.T) {
	current := completedVersion("block1-r2", 2)
	prescription := basePrescription()
	sets := successfulSets(current.ID, prescription, 4, 72.5)

	got, err := NextVersion(current, []domain.Prescription{prescription}, sets, time.Now())
	if err != nil {
		t.Fatalf("NextVersion() error = %v", err)
	}
	decision := got.Decisions[0]
	if !decision.LoadIncreased || *decision.NextLoadKG != 75 {
		t.Fatalf("Decision = %#v", decision)
	}
}

func TestNextVersionHoldsLoad(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]domain.WorkingSet)
		reason string
	}{
		{name: "reps", mutate: func(sets []domain.WorkingSet) { sets[3].Reps = 4 }, reason: "repetition range"},
		{name: "rir", mutate: func(sets []domain.WorkingSet) { sets[3].RIR = float64Pointer(1) }, reason: "prescribed RIR"},
		{name: "pain", mutate: func(sets []domain.WorkingSet) { sets[3].Pain = intPointer(1) }, reason: "pain"},
		{name: "substitution", mutate: func(sets []domain.WorkingSet) { sets[0].IsSubstitution = true }, reason: "substitution"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			current := completedVersion("block1-r2", 2)
			prescription := basePrescription()
			sets := successfulSets(current.ID, prescription, 4, 72.5)
			test.mutate(sets)
			got, err := NextVersion(current, []domain.Prescription{prescription}, sets, time.Now())
			if err != nil {
				t.Fatalf("NextVersion() error = %v", err)
			}
			decision := got.Decisions[0]
			if decision.LoadIncreased || !strings.Contains(decision.Reason, test.reason) {
				t.Fatalf("Decision = %#v", decision)
			}
		})
	}
}

func TestNextVersionManualProgressions(t *testing.T) {
	tests := []struct {
		name   string
		rule   domain.ProgressionRule
		reason string
	}{
		{name: "unknown increment", rule: domain.ProgressDouble, reason: "next practical load"},
		{name: "bodyweight difficulty", rule: domain.ProgressMakeHarder, reason: "make the bodyweight variation harder"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			current := completedVersion("block1-r2", 2)
			prescription := basePrescription()
			prescription.ProgressionRule = test.rule
			prescription.IncrementKG = nil
			sets := successfulSets(current.ID, prescription, 4, 72.5)
			got, err := NextVersion(current, []domain.Prescription{prescription}, sets, time.Now())
			if err != nil {
				t.Fatalf("NextVersion() error = %v", err)
			}
			if !strings.Contains(got.Decisions[0].Reason, test.reason) {
				t.Fatalf("Decision = %#v", got.Decisions[0])
			}
		})
	}
}

func TestNextVersionRequiresCompletedProgram(t *testing.T) {
	current := completedVersion("block1-r2", 2)
	current.Status = domain.ProgramActive
	if _, err := NextVersion(current, []domain.Prescription{basePrescription()}, nil, time.Now()); err == nil {
		t.Fatal("NextVersion() error = nil")
	}
}

func basePrescription() domain.Prescription {
	return domain.Prescription{
		VersionID:       "block1-r2",
		Workout:         domain.WorkoutA,
		Position:        1,
		SlotID:          "A01",
		ExerciseID:      "bench_press",
		PlannedSets:     4,
		TargetSets:      4,
		RepMin:          intPointer(3),
		RepMax:          intPointer(5),
		PlannedRIRMin:   float64Pointer(2),
		PlannedRIRMax:   float64Pointer(2),
		TargetRIRMin:    float64Pointer(2),
		TargetRIRMax:    float64Pointer(2),
		SuggestedLoadKG: float64Pointer(72.5),
		ProgressionRule: domain.ProgressAllSetsTop,
		IncrementKG:     float64Pointer(2.5),
	}
}

func completedVersion(id string, rotation int) domain.ProgramVersion {
	return domain.ProgramVersion{
		ID:             id,
		BlockID:        "block1",
		RotationNumber: rotation,
		Status:         domain.ProgramCompleted,
	}
}

func successfulSets(versionID string, prescription domain.Prescription, count int, load float64) []domain.WorkingSet {
	sets := make([]domain.WorkingSet, count)
	for index := range sets {
		sets[index] = domain.WorkingSet{
			ProgramVersionID: versionID,
			SlotID:           prescription.SlotID,
			ExerciseID:       prescription.ExerciseID,
			Number:           index + 1,
			LoadKG:           float64Pointer(load),
			Reps:             *prescription.RepMax,
			RIR:              float64Pointer(2),
			Pain:             intPointer(0),
		}
	}
	return sets
}

func float64Pointer(value float64) *float64 { return &value }
func intPointer(value int) *int             { return &value }
