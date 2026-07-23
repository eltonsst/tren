package sheetscheck

import "testing"

func TestStatsDatasetFromValuesUsesHeadersAndSheetLocale(t *testing.T) {
	dataset, err := statsDatasetFromValues(
		[][]any{
			{"value", "key"},
			{"block1-r1", "active_program_version"},
			{"Europe/Rome", "timezone"},
		},
		[][]any{
			{"active", "volume_multiplier", "load_mode", "aliases", "name", "exercise_id"},
			{true, "1,0", "barbell_total", "bench|bp", "Bench press", "bench_press"},
		},
		[][]any{
			{"load_guidance", "suggested_load_kg", "planned_rir_max", "planned_rir_min", "rep_max", "rep_min", "planned_sets", "exercise_id", "slot_id", "position", "workout_code", "version_id"},
			{"", "72,5", 3, 3, 5, 3, 3, "bench_press", "A01", 1, "A", "block1-r1"},
		},
		[][]any{
			{"status", "finished_at", "started_at", "workout_code", "rotation_number", "program_version_id", "session_id"},
			{"completed", "17/07/2026 11:00", "17/07/2026 10:00", "A", 1, "block1-r1", "session-1"},
		},
		[][]any{
			{"logged_at", "is_substitution", "pain_0_10", "rir", "reps", "load_kg", "set_number", "exercise_id", "slot_id", "program_version_id", "session_id"},
			{"17/07/2026 10:30", false, 0, 2, 5, "72,5", 1, "bench_press", "A01", "block1-r1", "session-1"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if dataset.ActiveProgramVersion != "block1-r1" || len(dataset.Exercises) != 1 || len(dataset.Prescriptions) != 1 || len(dataset.Sessions) != 1 || len(dataset.Sets) != 1 {
		t.Fatalf("dataset = %#v", dataset)
	}
	if dataset.Exercises[0].Aliases[1] != "bp" || dataset.Sets[0].LoadKG == nil || *dataset.Sets[0].LoadKG != 72.5 {
		t.Fatalf("exercise = %#v, set = %#v", dataset.Exercises[0], dataset.Sets[0])
	}
	if dataset.Sessions[0].StartedAt.Location().String() != "Europe/Rome" {
		t.Fatalf("session timezone = %s", dataset.Sessions[0].StartedAt.Location())
	}
}
