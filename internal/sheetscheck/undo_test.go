package sheetscheck

import "testing"

func TestPlanUndoLastExerciseSelectsLatestMessageInSession(t *testing.T) {
	values := [][]any{
		{"set_id", "session_id", "slot_id", "exercise_id", "telegram_message_id"},
		{"s1", "session-b", "B01", "back_squat", 10},
		{"s2", "session-b", "B01", "back_squat", 10},
		{"other", "another-session", "A01", "bench_press", 99},
		{"s3", "session-b", "B02", "romanian_deadlift", 11},
		{"s4", "session-b", "B02", "romanian_deadlift", 11},
	}

	result, spans, err := planUndoLastExercise(values, "session-b")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.SlotID != "B02" || result.ExerciseID != "romanian_deadlift" || result.TelegramMessageID != 11 || result.RemovedSets != 2 {
		t.Fatalf("result = %#v", result)
	}
	if len(spans) != 1 || spans[0] != (rowSpan{start: 4, end: 6}) {
		t.Fatalf("spans = %#v", spans)
	}
}

func TestPlanUndoLastExerciseGroupsSeparatedRowsForSafeDeletion(t *testing.T) {
	values := [][]any{
		{"session_id", "slot_id", "exercise_id", "telegram_message_id"},
		{"session-b", "B01", "back_squat", 10},
		{"another-session", "A01", "bench_press", 90},
		{"session-b", "B01", "back_squat", 10},
	}

	result, spans, err := planUndoLastExercise(values, "session-b")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.RemovedSets != 2 {
		t.Fatalf("result = %#v", result)
	}
	if len(spans) != 2 || spans[0] != (rowSpan{start: 1, end: 2}) || spans[1] != (rowSpan{start: 3, end: 4}) {
		t.Fatalf("spans = %#v", spans)
	}
}

func TestPlanUndoLastExerciseReturnsNilWhenSessionHasNoSets(t *testing.T) {
	values := [][]any{
		{"session_id", "slot_id", "exercise_id", "telegram_message_id"},
		{"another-session", "A01", "bench_press", 90},
	}
	result, spans, err := planUndoLastExercise(values, "session-b")
	if err != nil || result != nil || spans != nil {
		t.Fatalf("result = %#v, spans = %#v, err = %v", result, spans, err)
	}
}
