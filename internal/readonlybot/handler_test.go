package readonlybot

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/eltonsst/tren/internal/sheetscheck"
	"github.com/eltonsst/tren/internal/telegram"
)

type fakeStore struct {
	workout         sheetscheck.Workout
	rotation        sheetscheck.RotationProgram
	open            *sheetscheck.Session
	started         []sheetscheck.Session
	sets            []sheetscheck.Set
	finishedSession string
	advancedWorkout string
	finishedAt      time.Time
	rollover        sheetscheck.RotationRollover
	rotationSession string
	undoWorkout     *sheetscheck.WorkoutUndoResult
	undoWorkoutRuns int
}

func (store *fakeStore) CurrentWorkout(context.Context) (sheetscheck.Workout, error) {
	return store.workout, nil
}

func (store *fakeStore) CurrentRotation(context.Context) (sheetscheck.RotationProgram, error) {
	return store.rotation, nil
}

func (store *fakeStore) OpenSession(context.Context) (*sheetscheck.Session, error) {
	return store.open, nil
}

func (store *fakeStore) StartSession(_ context.Context, session sheetscheck.Session) error {
	store.started = append(store.started, session)
	store.open = &session
	return nil
}

func (store *fakeStore) SessionSets(_ context.Context, sessionID string) ([]sheetscheck.LoggedSet, error) {
	var result []sheetscheck.LoggedSet
	for _, set := range store.sets {
		if set.SessionID == sessionID {
			result = append(result, sheetscheck.LoggedSet{SlotID: set.SlotID, TelegramMessageID: set.TelegramMessageID})
		}
	}
	return result, nil
}

func (store *fakeStore) AppendSets(_ context.Context, sets []sheetscheck.Set) error {
	store.sets = append(store.sets, sets...)
	return nil
}

func (store *fakeStore) UndoLastExercise(_ context.Context, sessionID string) (*sheetscheck.UndoResult, error) {
	messageID := 0
	for index := len(store.sets) - 1; index >= 0; index-- {
		if store.sets[index].SessionID == sessionID {
			messageID = store.sets[index].TelegramMessageID
			break
		}
	}
	if messageID == 0 {
		return nil, nil
	}
	result := &sheetscheck.UndoResult{TelegramMessageID: messageID}
	kept := make([]sheetscheck.Set, 0, len(store.sets))
	for _, set := range store.sets {
		if set.SessionID == sessionID && set.TelegramMessageID == messageID {
			result.SlotID = set.SlotID
			result.ExerciseID = set.ExerciseID
			result.RemovedSets++
			continue
		}
		kept = append(kept, set)
	}
	store.sets = kept
	return result, nil
}

func (store *fakeStore) UndoLastWorkout(context.Context) (*sheetscheck.WorkoutUndoResult, error) {
	store.undoWorkoutRuns++
	return store.undoWorkout, nil
}

func (store *fakeStore) FinishSessionAndAdvance(_ context.Context, sessionID, nextWorkout string, finishedAt time.Time) error {
	store.finishedSession = sessionID
	store.advancedWorkout = nextWorkout
	store.finishedAt = finishedAt
	store.open = nil
	return nil
}

func (store *fakeStore) FinishRotation(_ context.Context, sessionID string, finishedAt time.Time) (sheetscheck.RotationRollover, error) {
	store.rotationSession = sessionID
	store.finishedAt = finishedAt
	store.open = nil
	return store.rollover, nil
}

func message(text string, userID int64) telegram.Update {
	return telegram.Update{ID: 1, Message: &telegram.Message{
		ID: 2, From: &telegram.User{ID: userID}, Chat: telegram.Chat{ID: userID}, Text: text,
	}}
}

func TestWhoAmI(t *testing.T) {
	handler := New(&fakeStore{})
	_, reply, err := handler.Handle(context.Background(), message("/whoami", 123))
	if err != nil || reply != "Your Telegram user ID is 123." {
		t.Fatalf("reply = %q, err = %v", reply, err)
	}
}

func TestCommandKeyboardChangesWithWorkoutState(t *testing.T) {
	store := &fakeStore{}
	handler := New(store)
	keyboard, err := handler.CommandKeyboard(context.Background())
	if err != nil || len(keyboard) != 3 || keyboard[0][0] != "/current" || keyboard[0][1] != "/rotation" || keyboard[1][0] != "/start" || keyboard[1][1] != "/undo" {
		t.Fatalf("closed keyboard = %#v, err = %v", keyboard, err)
	}
	store.open = &sheetscheck.Session{ID: "session-b", WorkoutCode: "B"}
	keyboard, err = handler.CommandKeyboard(context.Background())
	if err != nil || len(keyboard) != 3 || keyboard[0][0] != "/resume" || keyboard[0][1] != "/rotation" || keyboard[1][0] != "/undoexercise" || keyboard[1][1] != "/finish" {
		t.Fatalf("open keyboard = %#v, err = %v", keyboard, err)
	}
}

func TestHowToExplainsSetGrammar(t *testing.T) {
	handler := New(&fakeStore{})
	_, reply, err := handler.Handle(context.Background(), message("/howto", 123))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"reps@RIR",
		"5@2 = 5 reps with 2 reps left",
		"5@3 5@2 4@1",
		"75: 5@3 5@2 4@1",
		"75x5@3 75x5@2 72.5x6@2",
		"p1 to p10",
		"/finish",
	} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("reply = %q, missing %q", reply, expected)
		}
	}
}

func TestCurrentWorkout(t *testing.T) {
	handler := New(&fakeStore{workout: sheetscheck.Workout{
		ProgramVersion: "block1-r1", RotationNumber: 1, Code: "A", AuthorizedTelegramUserID: 123,
		Exercises: []sheetscheck.WorkoutExercise{{
			Position: 1, SlotID: "A01", ExerciseID: "bench_press", Name: "Bench press", PlannedSets: "3", RepMin: "3", RepMax: "5",
			RIRMin: "3", RIRMax: "3", SuggestedLoadKG: "72.5",
		}},
	}})
	_, reply, err := handler.Handle(context.Background(), message("/current", 123))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reply, "Workout A · block1-r1") || !strings.Contains(reply, "3×3–5 @3 RIR · 72.5 kg") {
		t.Fatalf("reply = %q", reply)
	}
}

func TestRotationShowsCompleteCurrentProgram(t *testing.T) {
	store := &fakeStore{rotation: sheetscheck.RotationProgram{
		ProgramVersion: "block1-r1", RotationNumber: 1, NextWorkout: "B", AuthorizedTelegramUserID: 123,
		Workouts: []sheetscheck.Workout{
			{Code: "A", Exercises: []sheetscheck.WorkoutExercise{{Position: 1, Name: "Bench press", PlannedSets: "3", RepMin: "3", RepMax: "5", RIRMin: "3", RIRMax: "3", SuggestedLoadKG: "72.5"}}},
			{Code: "B", Exercises: []sheetscheck.WorkoutExercise{{Position: 1, Name: "Back squat", PlannedSets: "3", RepMin: "3", RepMax: "5", RIRMin: "3", RIRMax: "3", SuggestedLoadKG: "70"}}},
			{Code: "C", Exercises: []sheetscheck.WorkoutExercise{{Position: 1, Name: "Paused bench press", PlannedSets: "3", RepMin: "5", RepMax: "8", RIRMin: "3", RIRMax: "3"}}},
			{Code: "D", Exercises: []sheetscheck.WorkoutExercise{{Position: 1, Name: "Paused squat", PlannedSets: "3", RepMin: "5", RepMax: "8", RIRMin: "3", RIRMax: "3"}}},
		},
	}}
	handler := New(store)
	_, reply, err := handler.Handle(context.Background(), message("/rotation", 123))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"Rotation 1 · block1-r1", "Next: Workout B", "WORKOUT A", "1. Bench press", "WORKOUT B", "1. Back squat", "WORKOUT C", "1. Paused bench press", "WORKOUT D", "1. Paused squat",
	} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("reply = %q, missing %q", reply, expected)
		}
	}
}

func TestCurrentExplainsHowToClaimUnconfiguredBot(t *testing.T) {
	handler := New(&fakeStore{workout: sheetscheck.Workout{Code: "A"}})
	_, reply, err := handler.Handle(context.Background(), message("/current", 123))
	if err != nil || !strings.Contains(reply, "authorized_telegram_user_id") || !strings.Contains(reply, "123") {
		t.Fatalf("reply = %q, err = %v", reply, err)
	}
}

func TestStartCreatesOneOpenSession(t *testing.T) {
	load := 72.5
	store := &fakeStore{workout: sheetscheck.Workout{
		ProgramVersion: "block1-r1", RotationNumber: 1, Code: "A", AuthorizedTelegramUserID: 123,
		Exercises: []sheetscheck.WorkoutExercise{{
			Position: 1, SlotID: "A01", ExerciseID: "bench_press", Name: "Bench press", LoadMode: "barbell_total",
			PlannedSets: "3", RepMin: "3", RepMax: "5", RIRMin: "3", RIRMax: "3", SuggestedLoad: &load,
		}},
	}}
	now := time.Date(2026, 7, 15, 18, 30, 0, 0, time.FixedZone("CEST", 2*60*60))
	handler := newWithClock(store, func() time.Time { return now })
	_, reply, err := handler.Handle(context.Background(), message("/start", 123))
	if err != nil {
		t.Fatal(err)
	}
	if len(store.started) != 1 || store.started[0].ID != "session-20260715-183000-u1" || store.started[0].WorkoutCode != "A" {
		t.Fatalf("started = %#v", store.started)
	}
	if !strings.Contains(reply, "Workout A started — rotation 1") {
		t.Fatalf("reply = %q", reply)
	}
	if !strings.Contains(reply, "Reply like: 3@3 3@3 3@3") {
		t.Fatalf("reply does not show the logging shorthand: %q", reply)
	}
	_, reply, err = handler.Handle(context.Background(), message("/start", 123))
	if err != nil || len(store.started) != 1 || !strings.Contains(reply, "already open") {
		t.Fatalf("second start reply = %q, err = %v, started = %d", reply, err, len(store.started))
	}
}

func TestSpaceSeparatedExerciseResultIsSaved(t *testing.T) {
	load := 70.0
	store := &fakeStore{
		workout: sheetscheck.Workout{
			ProgramVersion: "block1-r1", RotationNumber: 1, Code: "B", AuthorizedTelegramUserID: 123,
			Exercises: []sheetscheck.WorkoutExercise{{
				Position: 1, SlotID: "B01", ExerciseID: "back_squat", Name: "Back squat", LoadMode: "barbell_total",
				PlannedSets: "3", RepMin: "3", RepMax: "5", RIRMin: "3", RIRMax: "3", SuggestedLoad: &load,
			}},
		},
		open: &sheetscheck.Session{ID: "session-b", ProgramVersionID: "block1-r1", WorkoutCode: "B"},
	}
	handler := New(store)
	_, reply, err := handler.Handle(context.Background(), message("3@3 3@3 3@3", 123))
	if err != nil || len(store.sets) != 3 || !strings.Contains(reply, "Logged Back squat: 70×3@3, 70×3@3, 70×3@3") {
		t.Fatalf("reply = %q, err = %v, sets = %#v", reply, err, store.sets)
	}
}

func TestUndoExerciseRemovesLastExerciseResultAndPromptsForItAgain(t *testing.T) {
	load := 70.0
	store := &fakeStore{
		workout: sheetscheck.Workout{
			ProgramVersion: "block1-r1", RotationNumber: 1, Code: "B", AuthorizedTelegramUserID: 123,
			Exercises: []sheetscheck.WorkoutExercise{
				{Position: 1, SlotID: "B01", ExerciseID: "back_squat", Name: "Back squat", LoadMode: "barbell_total", PlannedSets: "3", RepMin: "3", RepMax: "5", RIRMin: "3", RIRMax: "3", SuggestedLoad: &load},
				{Position: 2, SlotID: "B02", ExerciseID: "romanian_deadlift", Name: "Romanian deadlift", PlannedSets: "2", RepMin: "6", RepMax: "10", RIRMin: "3", RIRMax: "3"},
			},
		},
		open: &sheetscheck.Session{ID: "session-b", ProgramVersionID: "block1-r1", WorkoutCode: "B"},
		sets: []sheetscheck.Set{
			{SessionID: "session-b", SlotID: "B01", ExerciseID: "back_squat", TelegramMessageID: 10},
			{SessionID: "session-b", SlotID: "B01", ExerciseID: "back_squat", TelegramMessageID: 10},
			{SessionID: "session-b", SlotID: "B02", ExerciseID: "romanian_deadlift", TelegramMessageID: 11},
			{SessionID: "session-b", SlotID: "B02", ExerciseID: "romanian_deadlift", TelegramMessageID: 11},
		},
	}

	handler := New(store)
	_, reply, err := handler.Handle(context.Background(), message("/undoexercise", 123))
	if err != nil {
		t.Fatal(err)
	}
	if len(store.sets) != 2 || store.sets[0].SlotID != "B01" {
		t.Fatalf("sets = %#v", store.sets)
	}
	if !strings.Contains(reply, "Undid Romanian deadlift: removed 2 working sets.") ||
		!strings.Contains(reply, "Log Romanian deadlift next.") ||
		!strings.Contains(reply, "Reply like: 50: 6@3 6@3") {
		t.Fatalf("reply = %q", reply)
	}
}

func TestUndoExerciseWithNoLoggedExerciseDoesNothing(t *testing.T) {
	store := &fakeStore{
		workout: sheetscheck.Workout{ProgramVersion: "block1-r1", Code: "B", AuthorizedTelegramUserID: 123},
		open:    &sheetscheck.Session{ID: "session-b", ProgramVersionID: "block1-r1", WorkoutCode: "B"},
	}
	handler := New(store)
	_, reply, err := handler.Handle(context.Background(), message("/undoexercise", 123))
	if err != nil || reply != "Nothing to undo in Workout B." {
		t.Fatalf("reply = %q, err = %v", reply, err)
	}
}

func TestUndoDeletesLatestCompletedWorkoutAndReportsRewind(t *testing.T) {
	store := &fakeStore{
		workout: sheetscheck.Workout{ProgramVersion: "block1-r1", Code: "C", AuthorizedTelegramUserID: 123},
		undoWorkout: &sheetscheck.WorkoutUndoResult{
			SessionID: "session-b", ProgramVersionID: "block1-r1", RotationNumber: 1,
			WorkoutCode: "B", RemovedSets: 11,
		},
	}
	handler := New(store)
	_, reply, err := handler.Handle(context.Background(), message("/undo", 123))
	if err != nil || store.undoWorkoutRuns != 1 {
		t.Fatalf("reply = %q, err = %v, calls = %d", reply, err, store.undoWorkoutRuns)
	}
	if !strings.Contains(reply, "Undid Workout B · rotation 1: deleted the session and 11 working sets.") ||
		!strings.Contains(reply, "Workout B is next again.") {
		t.Fatalf("reply = %q", reply)
	}
}

func TestUndoWorkoutDReportsRemovedGeneratedVersion(t *testing.T) {
	store := &fakeStore{
		workout: sheetscheck.Workout{ProgramVersion: "block1-r2", Code: "A", AuthorizedTelegramUserID: 123},
		undoWorkout: &sheetscheck.WorkoutUndoResult{
			SessionID: "session-d", ProgramVersionID: "block1-r1", RotationNumber: 1,
			WorkoutCode: "D", RemovedSets: 10, RemovedProgramVersion: "block1-r2",
		},
	}
	handler := New(store)
	_, reply, err := handler.Handle(context.Background(), message("/undo", 123))
	if err != nil || !strings.Contains(reply, "Removed generated program block1-r2 and reactivated block1-r1.") {
		t.Fatalf("reply = %q, err = %v", reply, err)
	}
}

func TestUndoCompletedWorkoutRefusesWhileWorkoutIsOpen(t *testing.T) {
	store := &fakeStore{
		workout: sheetscheck.Workout{ProgramVersion: "block1-r1", Code: "B", AuthorizedTelegramUserID: 123},
		open:    &sheetscheck.Session{ID: "session-b", ProgramVersionID: "block1-r1", WorkoutCode: "B"},
	}
	handler := New(store)
	_, reply, err := handler.Handle(context.Background(), message("/undo", 123))
	if err != nil || store.undoWorkoutRuns != 0 || !strings.Contains(reply, "Workout B is still open") {
		t.Fatalf("reply = %q, err = %v, calls = %d", reply, err, store.undoWorkoutRuns)
	}
}

func TestResumeRequiresOpenSession(t *testing.T) {
	store := &fakeStore{workout: sheetscheck.Workout{ProgramVersion: "block1-r1", RotationNumber: 1, Code: "A", AuthorizedTelegramUserID: 123}}
	handler := New(store)
	_, reply, err := handler.Handle(context.Background(), message("/resume", 123))
	if err != nil || reply != "No workout is open. Send /start." {
		t.Fatalf("reply = %q, err = %v", reply, err)
	}
}

func TestPartialExerciseIsSavedWithWarningAndAdvances(t *testing.T) {
	load := 72.5
	store := &fakeStore{
		workout: sheetscheck.Workout{
			ProgramVersion: "block1-r1", RotationNumber: 1, Code: "A", AuthorizedTelegramUserID: 123,
			Exercises: []sheetscheck.WorkoutExercise{
				{Position: 1, SlotID: "A01", ExerciseID: "bench_press", Name: "Bench press", LoadMode: "barbell_total", PlannedSets: "3", RepMin: "3", RepMax: "5", SuggestedLoad: &load},
				{Position: 2, SlotID: "A02", ExerciseID: "weighted_pull_up", Name: "Weighted pull-up", LoadMode: "external_added", PlannedSets: "3", RepMin: "3", RepMax: "5"},
			},
		},
		open: &sheetscheck.Session{ID: "session-1", ProgramVersionID: "block1-r1", WorkoutCode: "A"},
	}
	now := time.Date(2026, 7, 15, 19, 30, 0, 0, time.UTC)
	handler := newWithClock(store, func() time.Time { return now })
	_, reply, err := handler.Handle(context.Background(), message("5@2, 4@1 pain0", 123))
	if err != nil {
		t.Fatal(err)
	}
	if len(store.sets) != 2 || store.sets[0].LoadKG == nil || *store.sets[0].LoadKG != 72.5 {
		t.Fatalf("sets = %#v", store.sets)
	}
	if !strings.Contains(reply, "expected 3 sets, received 2") || !strings.Contains(reply, "Next: 2. Weighted pull-up") {
		t.Fatalf("reply = %q", reply)
	}
}

func TestExerciseMessageIsIdempotent(t *testing.T) {
	store := &fakeStore{
		workout: sheetscheck.Workout{ProgramVersion: "block1-r1", RotationNumber: 1, Code: "A", AuthorizedTelegramUserID: 123},
		open:    &sheetscheck.Session{ID: "session-1", ProgramVersionID: "block1-r1", WorkoutCode: "A"},
		sets:    []sheetscheck.Set{{SessionID: "session-1", SlotID: "A01", TelegramMessageID: 2}},
	}
	handler := New(store)
	_, reply, err := handler.Handle(context.Background(), message("5@2 pain0", 123))
	if err != nil || reply != "That Telegram message is already logged." || len(store.sets) != 1 {
		t.Fatalf("reply = %q, err = %v, sets = %d", reply, err, len(store.sets))
	}
}

func TestFinishIncompleteWorkoutSavesAndAdvances(t *testing.T) {
	store := &fakeStore{
		workout: sheetscheck.Workout{
			ProgramVersion: "block1-r1", RotationNumber: 1, Code: "A", AuthorizedTelegramUserID: 123,
			Exercises: []sheetscheck.WorkoutExercise{
				{Position: 1, SlotID: "A01", ExerciseID: "bench_press", Name: "Bench press"},
				{Position: 2, SlotID: "A02", ExerciseID: "weighted_pull_up", Name: "Weighted pull-up"},
			},
		},
		open: &sheetscheck.Session{ID: "session-1", ProgramVersionID: "block1-r1", WorkoutCode: "A"},
		sets: []sheetscheck.Set{{SessionID: "session-1", SlotID: "A01"}},
	}
	now := time.Date(2026, 7, 15, 20, 0, 0, 0, time.UTC)
	handler := newWithClock(store, func() time.Time { return now })

	_, reply, err := handler.Handle(context.Background(), message("/finish", 123))
	if err != nil {
		t.Fatal(err)
	}
	if store.finishedSession != "session-1" || store.advancedWorkout != "B" || !store.finishedAt.Equal(now) || store.open != nil {
		t.Fatalf("finish = session %q, next %q, at %v, open %#v", store.finishedSession, store.advancedWorkout, store.finishedAt, store.open)
	}
	if !strings.Contains(reply, "Workout A finished. Next: Workout B.") ||
		!strings.Contains(reply, "Incomplete workout: 1/2 exercises logged. Missing: Weighted pull-up.") {
		t.Fatalf("reply = %q", reply)
	}
}

func TestFinishCompleteWorkoutDoesNotWarn(t *testing.T) {
	store := &fakeStore{
		workout: sheetscheck.Workout{
			ProgramVersion: "block1-r1", RotationNumber: 1, Code: "B", AuthorizedTelegramUserID: 123,
			Exercises: []sheetscheck.WorkoutExercise{{Position: 1, SlotID: "B01", ExerciseID: "squat", Name: "Squat"}},
		},
		open: &sheetscheck.Session{ID: "session-2", ProgramVersionID: "block1-r1", WorkoutCode: "B"},
		sets: []sheetscheck.Set{{SessionID: "session-2", SlotID: "B01"}},
	}
	handler := New(store)

	_, reply, err := handler.Handle(context.Background(), message("/finish", 123))
	if err != nil || reply != "Workout B finished. Next: Workout C." || store.advancedWorkout != "C" {
		t.Fatalf("reply = %q, err = %v, next = %q", reply, err, store.advancedWorkout)
	}
}

func TestFinishWorkoutDCreatesNextProgramVersion(t *testing.T) {
	store := &fakeStore{
		workout: sheetscheck.Workout{
			ProgramVersion: "block1-r1", RotationNumber: 1, Code: "D", AuthorizedTelegramUserID: 123,
			Exercises: []sheetscheck.WorkoutExercise{{Position: 1, SlotID: "D01", ExerciseID: "romanian_deadlift", Name: "Romanian deadlift"}},
		},
		open:     &sheetscheck.Session{ID: "session-4", ProgramVersionID: "block1-r1", WorkoutCode: "D"},
		rollover: sheetscheck.RotationRollover{PreviousRotation: 1, NextRotation: 2, PreviousVersion: "block1-r1", NextVersion: "block1-r2", PrescriptionCount: 24, RestoredPrescriptions: 18, IncreasedLoads: 0},
	}
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	handler := newWithClock(store, func() time.Time { return now })

	_, reply, err := handler.Handle(context.Background(), message("/finish", 123))
	if err != nil || store.rotationSession != "session-4" || store.open != nil || !store.finishedAt.Equal(now) {
		t.Fatalf("reply = %q, err = %v, session = %q, open = %#v", reply, err, store.rotationSession, store.open)
	}
	for _, expected := range []string{
		"Workout D finished. Rotation 1 complete.",
		"Created block1-r2 — next: Workout A.",
		"24 prescriptions copied, 18 restored to target settings, 0 loads increased.",
		"Incomplete workout: 0/1 exercises logged. Missing: Romanian deadlift.",
	} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("reply = %q, missing %q", reply, expected)
		}
	}
}
