package workout

import (
	"testing"
	"time"

	"github.com/eltonsst/tren/internal/domain"
)

func TestStartResumeFinish(t *testing.T) {
	now := time.Date(2026, 7, 15, 18, 0, 0, 0, time.UTC)
	state := State{NextWorkout: domain.WorkoutA, RotationNumber: 1}
	state, started, err := Start(state, "session-1", "block1-r1", nil, now)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if started.Workout != domain.WorkoutA {
		t.Fatalf("Workout = %q", started.Workout)
	}
	if _, _, err := Start(state, "session-2", "block1-r1", nil, now); err == nil {
		t.Fatal("second Start() error = nil")
	}
	if resumed, err := Resume(state); err != nil || resumed.ID != started.ID {
		t.Fatalf("Resume() = %#v, %v", resumed, err)
	}

	state, finished, err := Finish(state, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	if finished.NextWorkout != domain.WorkoutB || finished.RotationCompleted || state.OpenSession != nil {
		t.Fatalf("Finish() = %#v, state = %#v", finished, state)
	}
}

func TestFinishWorkoutDCompletesRotation(t *testing.T) {
	now := time.Date(2026, 7, 15, 18, 0, 0, 0, time.UTC)
	state := State{NextWorkout: domain.WorkoutD, RotationNumber: 3}
	state, _, err := Start(state, "session-d", "block1-r3", nil, now)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	state, result, err := Finish(state, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	if !result.RotationCompleted || result.NextWorkout != domain.WorkoutA || result.NextRotation != 4 {
		t.Fatalf("Finish() = %#v", result)
	}
	if state.RotationNumber != 4 {
		t.Fatalf("RotationNumber = %d", state.RotationNumber)
	}
}

func TestStartOverrideDoesNotAdvanceWithoutFinish(t *testing.T) {
	now := time.Date(2026, 7, 15, 18, 0, 0, 0, time.UTC)
	override := domain.WorkoutC
	state := State{NextWorkout: domain.WorkoutA, RotationNumber: 1}
	state, started, err := Start(state, "session-c", "block1-r1", &override, now)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if started.Workout != domain.WorkoutC || state.NextWorkout != domain.WorkoutA {
		t.Fatalf("started = %#v, state = %#v", started, state)
	}
}
