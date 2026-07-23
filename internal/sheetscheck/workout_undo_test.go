package sheetscheck

import (
	"strings"
	"testing"
)

func TestPlanUndoLastWorkoutRewindsLatestCompletedSession(t *testing.T) {
	plan, err := planUndoLastWorkout(
		[][]any{
			workoutUndoSessionHeaders(),
			{"session-a", "block1-r1", 1, "A", "2026-07-20T10:00:00Z", "completed"},
			{"session-b", "block1-r1", 1, "B", "2026-07-21T10:00:00Z", "completed"},
		},
		[][]any{
			{"session_id"},
			{"session-a"},
			{"session-b"},
			{"session-b"},
		},
		workoutUndoConfig("block1-r1", "C"),
		[][]any{
			workoutUndoVersionHeaders(),
			{"block1-r1", 1, "active", "2026-07-20T09:00:00Z", "", ""},
		},
		[][]any{{"version_id"}, {"block1-r1"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.result.SessionID != "session-b" || plan.result.WorkoutCode != "B" || plan.result.RemovedSets != 2 || plan.result.RemovedProgramVersion != "" {
		t.Fatalf("result = %#v", plan.result)
	}
	if plan.sessionRow != 2 || len(plan.setSpans) != 1 || plan.setSpans[0] != (rowSpan{start: 2, end: 4}) {
		t.Fatalf("session row = %d, set spans = %#v", plan.sessionRow, plan.setSpans)
	}
	if plan.clearPreviousVersionLock {
		t.Fatal("undoing Workout B must not unlock the program")
	}
}

func TestPlanUndoWorkoutAUnlocksVersionWhenNoSessionsRemain(t *testing.T) {
	plan, err := planUndoLastWorkout(
		[][]any{
			workoutUndoSessionHeaders(),
			{"session-a", "block1-r1", 1, "A", "2026-07-21T10:00:00Z", "completed"},
		},
		[][]any{{"session_id"}},
		workoutUndoConfig("block1-r1", "B"),
		[][]any{
			workoutUndoVersionHeaders(),
			{"block1-r1", 1, "active", "2026-07-21T09:00:00Z", "", ""},
		},
		[][]any{{"version_id"}, {"block1-r1"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.clearPreviousVersionLock || plan.result.WorkoutCode != "A" {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestPlanUndoWorkoutDRestoresPreviousVersionAndRemovesGeneratedVersion(t *testing.T) {
	plan, err := planUndoLastWorkout(
		[][]any{
			workoutUndoSessionHeaders(),
			{"session-a", "block1-r1", 1, "A", "2026-07-18T10:00:00Z", "completed"},
			{"session-b", "block1-r1", 1, "B", "2026-07-19T10:00:00Z", "completed"},
			{"session-c", "block1-r1", 1, "C", "2026-07-20T10:00:00Z", "completed"},
			{"session-d", "block1-r1", 1, "D", "2026-07-21T10:00:00Z", "completed"},
		},
		[][]any{{"session_id"}, {"session-d"}, {"session-d"}},
		workoutUndoConfig("block1-r2", "A"),
		[][]any{
			workoutUndoVersionHeaders(),
			{"block1-r1", 1, "completed", "2026-07-18T09:00:00Z", "2026-07-21T10:00:00Z", ""},
			{"block1-r2", 2, "active", "", "", "block1-r1"},
		},
		[][]any{
			{"version_id", "slot_id"},
			{"block1-r1", "A01"},
			{"block1-r2", "A01"},
			{"block1-r2", "B01"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.result.WorkoutCode != "D" || plan.result.ProgramVersionID != "block1-r1" || plan.result.RemovedProgramVersion != "block1-r2" || plan.result.RemovedSets != 2 {
		t.Fatalf("result = %#v", plan.result)
	}
	if plan.generatedVersionRow != 2 || len(plan.generatedProgramSpans) != 1 || plan.generatedProgramSpans[0] != (rowSpan{start: 2, end: 4}) {
		t.Fatalf("generated version row = %d, program spans = %#v", plan.generatedVersionRow, plan.generatedProgramSpans)
	}
}

func TestPlanUndoWorkoutDRefusesToRemoveLockedGeneratedVersion(t *testing.T) {
	_, err := planUndoLastWorkout(
		[][]any{
			workoutUndoSessionHeaders(),
			{"session-d", "block1-r1", 1, "D", "2026-07-21T10:00:00Z", "completed"},
		},
		[][]any{{"session_id"}},
		workoutUndoConfig("block1-r2", "A"),
		[][]any{
			workoutUndoVersionHeaders(),
			{"block1-r1", 1, "completed", "2026-07-18T09:00:00Z", "2026-07-21T10:00:00Z", ""},
			{"block1-r2", 2, "active", "2026-07-21T11:00:00Z", "", "block1-r1"},
		},
		[][]any{{"version_id"}, {"block1-r1"}, {"block1-r2"}},
	)
	if err == nil || !strings.Contains(err.Error(), "no longer safe to remove") {
		t.Fatalf("err = %v", err)
	}
}

func TestPlanUndoLastWorkoutRejectsOpenSession(t *testing.T) {
	_, err := planUndoLastWorkout(
		[][]any{
			workoutUndoSessionHeaders(),
			{"session-a", "block1-r1", 1, "A", "", "open"},
		},
		[][]any{{"session_id"}},
		workoutUndoConfig("block1-r1", "A"),
		[][]any{workoutUndoVersionHeaders()},
		[][]any{{"version_id"}},
	)
	if err == nil || !strings.Contains(err.Error(), "still open") {
		t.Fatalf("err = %v", err)
	}
}

func TestAppendDeleteRowsUsesDescendingOrder(t *testing.T) {
	requests := appendDeleteRows(nil, 42, []rowSpan{{start: 1, end: 3}, {start: 6, end: 8}})
	if len(requests) != 2 ||
		requests[0].DeleteDimension.Range.StartIndex != 6 ||
		requests[1].DeleteDimension.Range.StartIndex != 1 {
		t.Fatalf("requests = %#v", requests)
	}
}

func workoutUndoSessionHeaders() []any {
	return []any{"session_id", "program_version_id", "rotation_number", "workout_code", "finished_at", "status"}
}

func workoutUndoVersionHeaders() []any {
	return []any{"version_id", "rotation_number", "status", "locked_at", "completed_at", "source_version_id"}
}

func workoutUndoConfig(version, workout string) [][]any {
	return [][]any{
		{"key", "value"},
		{"active_program_version", version},
		{"next_workout", workout},
	}
}
