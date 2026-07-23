package readonlybot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/eltonsst/tren/internal/parser"
	"github.com/eltonsst/tren/internal/sheetscheck"
	"github.com/eltonsst/tren/internal/telegram"
)

type Store interface {
	CurrentWorkout(context.Context) (sheetscheck.Workout, error)
	CurrentRotation(context.Context) (sheetscheck.RotationProgram, error)
	OpenSession(context.Context) (*sheetscheck.Session, error)
	StartSession(context.Context, sheetscheck.Session) error
	SessionSets(context.Context, string) ([]sheetscheck.LoggedSet, error)
	AppendSets(context.Context, []sheetscheck.Set) error
	UndoLastExercise(context.Context, string) (*sheetscheck.UndoResult, error)
	UndoLastWorkout(context.Context) (*sheetscheck.WorkoutUndoResult, error)
	FinishSessionAndAdvance(context.Context, string, string, time.Time) error
	FinishRotation(context.Context, string, time.Time) (sheetscheck.RotationRollover, error)
}

type Handler struct {
	store Store
	now   func() time.Time
}

func New(store Store) *Handler {
	return &Handler{store: store, now: time.Now}
}

func (h *Handler) CommandKeyboard(ctx context.Context) ([][]string, error) {
	session, err := h.store.OpenSession(ctx)
	if err != nil {
		return nil, err
	}
	if session != nil {
		return [][]string{
			{"/resume", "/rotation"},
			{"/undoexercise", "/finish"},
			{"/howto"},
		}, nil
	}
	return [][]string{
		{"/current", "/rotation"},
		{"/start", "/undo"},
		{"/howto"},
	}, nil
}

func (h *Handler) Handle(ctx context.Context, update telegram.Update) (int64, string, error) {
	if update.Message == nil || update.Message.From == nil || strings.TrimSpace(update.Message.Text) == "" {
		return 0, "", nil
	}
	message := update.Message
	switch command(message.Text) {
	case "/whoami":
		return message.Chat.ID, fmt.Sprintf("Your Telegram user ID is %d.", message.From.ID), nil
	case "/help":
		return message.Chat.ID, help(), nil
	case "/howto":
		return message.Chat.ID, howTo(), nil
	case "/current":
		workout, reply, err := h.authorizedWorkout(ctx, message.From.ID)
		if err != nil || reply != "" {
			return message.Chat.ID, reply, err
		}
		return message.Chat.ID, formatWorkout(workout), nil
	case "/rotation":
		rotation, reply, err := h.authorizedRotation(ctx, message.From.ID)
		if err != nil || reply != "" {
			return message.Chat.ID, reply, err
		}
		return message.Chat.ID, formatRotation(rotation), nil
	case "/start":
		workout, reply, err := h.authorizedWorkout(ctx, message.From.ID)
		if err != nil || reply != "" {
			return message.Chat.ID, reply, err
		}
		existing, err := h.store.OpenSession(ctx)
		if err != nil {
			return message.Chat.ID, "", err
		}
		if existing != nil {
			return message.Chat.ID, fmt.Sprintf("Workout %s is already open. Send /resume.", existing.WorkoutCode), nil
		}
		now := h.now()
		session := sheetscheck.Session{
			ID:               fmt.Sprintf("session-%s-u%d", now.Format("20060102-150405"), update.ID),
			ProgramVersionID: workout.ProgramVersion, RotationNumber: workout.RotationNumber,
			WorkoutCode: workout.Code, StartedAt: now, TelegramChatID: message.Chat.ID,
			StartMessageID: message.ID,
		}
		if err := h.store.StartSession(ctx, session); err != nil {
			return message.Chat.ID, "", err
		}
		return message.Chat.ID, fmt.Sprintf(
			"Workout %s started — rotation %d.\n\n%s\n\n%s",
			workout.Code, workout.RotationNumber, formatWorkout(workout), formatLogPrompt(workout.Exercises[0]),
		), nil
	case "/resume":
		workout, reply, err := h.authorizedWorkout(ctx, message.From.ID)
		if err != nil || reply != "" {
			return message.Chat.ID, reply, err
		}
		existing, err := h.store.OpenSession(ctx)
		if err != nil {
			return message.Chat.ID, "", err
		}
		if existing == nil {
			return message.Chat.ID, "No workout is open. Send /start.", nil
		}
		logged, err := h.store.SessionSets(ctx, existing.ID)
		if err != nil {
			return message.Chat.ID, "", err
		}
		next := nextExercise(workout.Exercises, logged)
		if next == nil {
			return message.Chat.ID, fmt.Sprintf("Workout %s is open and every exercise has logged sets.", existing.WorkoutCode), nil
		}
		return message.Chat.ID, fmt.Sprintf("Workout %s is open.\n\nNext: %s\n\n%s", existing.WorkoutCode, formatExercise(*next), formatReplyExample(*next)), nil
	case "/undo":
		return h.undoWorkout(ctx, update)
	case "/undoexercise":
		return h.undoExercise(ctx, update)
	case "/finish":
		return h.finish(ctx, update)
	default:
		if strings.HasPrefix(strings.TrimSpace(message.Text), "/") {
			return message.Chat.ID, help(), nil
		}
		return h.logExercise(ctx, update)
	}
}

func (h *Handler) undoExercise(ctx context.Context, update telegram.Update) (int64, string, error) {
	message := update.Message
	workout, reply, err := h.authorizedWorkout(ctx, message.From.ID)
	if err != nil || reply != "" {
		return message.Chat.ID, reply, err
	}
	session, err := h.store.OpenSession(ctx)
	if err != nil {
		return message.Chat.ID, "", err
	}
	if session == nil {
		return message.Chat.ID, "No workout is open. Nothing to undo.", nil
	}
	if session.WorkoutCode != workout.Code || session.ProgramVersionID != workout.ProgramVersion {
		return message.Chat.ID, "", fmt.Errorf("open session does not match the active workout")
	}
	undone, err := h.store.UndoLastExercise(ctx, session.ID)
	if err != nil {
		return message.Chat.ID, "", err
	}
	if undone == nil {
		return message.Chat.ID, fmt.Sprintf("Nothing to undo in Workout %s.", session.WorkoutCode), nil
	}
	exercise := exerciseBySlot(workout.Exercises, undone.SlotID)
	name := undone.ExerciseID
	if exercise != nil {
		name = exercise.Name
	}
	reply = fmt.Sprintf("Undid %s: removed %d working %s.", name, undone.RemovedSets, plural(undone.RemovedSets, "set", "sets"))
	if exercise != nil {
		reply += "\n\n" + formatLogPrompt(*exercise)
	}
	return message.Chat.ID, reply, nil
}

func (h *Handler) undoWorkout(ctx context.Context, update telegram.Update) (int64, string, error) {
	message := update.Message
	_, reply, err := h.authorizedWorkout(ctx, message.From.ID)
	if err != nil || reply != "" {
		return message.Chat.ID, reply, err
	}
	session, err := h.store.OpenSession(ctx)
	if err != nil {
		return message.Chat.ID, "", err
	}
	if session != nil {
		return message.Chat.ID, fmt.Sprintf("Workout %s is still open. Finish it before undoing a completed workout.", session.WorkoutCode), nil
	}
	undone, err := h.store.UndoLastWorkout(ctx)
	if err != nil {
		return message.Chat.ID, "", err
	}
	if undone == nil {
		return message.Chat.ID, "There is no completed workout to undo.", nil
	}
	reply = fmt.Sprintf(
		"Undid Workout %s · rotation %d: deleted the session and %d working %s.\nWorkout %s is next again.",
		undone.WorkoutCode, undone.RotationNumber, undone.RemovedSets,
		plural(undone.RemovedSets, "set", "sets"), undone.WorkoutCode,
	)
	if undone.RemovedProgramVersion != "" {
		reply += fmt.Sprintf("\nRemoved generated program %s and reactivated %s.", undone.RemovedProgramVersion, undone.ProgramVersionID)
	}
	return message.Chat.ID, reply, nil
}

func (h *Handler) finish(ctx context.Context, update telegram.Update) (int64, string, error) {
	message := update.Message
	workout, reply, err := h.authorizedWorkout(ctx, message.From.ID)
	if err != nil || reply != "" {
		return message.Chat.ID, reply, err
	}
	session, err := h.store.OpenSession(ctx)
	if err != nil {
		return message.Chat.ID, "", err
	}
	if session == nil {
		return message.Chat.ID, "No workout is open.", nil
	}
	if session.WorkoutCode != workout.Code || session.ProgramVersionID != workout.ProgramVersion {
		return message.Chat.ID, "", fmt.Errorf("open session does not match the active workout")
	}
	logged, err := h.store.SessionSets(ctx, session.ID)
	if err != nil {
		return message.Chat.ID, "", err
	}
	missing := missingExercises(workout.Exercises, logged)
	if session.WorkoutCode == "D" {
		rollover, err := h.store.FinishRotation(ctx, session.ID, h.now())
		if err != nil {
			return message.Chat.ID, "", err
		}
		reply = fmt.Sprintf(
			"Workout D finished. Rotation %d complete.\nCreated %s — next: Workout A.\nProgression: %d prescriptions copied, %d restored to target settings, %d loads increased.",
			rollover.PreviousRotation, rollover.NextVersion, rollover.PrescriptionCount,
			rollover.RestoredPrescriptions, rollover.IncreasedLoads,
		)
		return message.Chat.ID, incompleteWarning(reply, workout.Exercises, missing), nil
	}
	next, ok := nextWorkout(session.WorkoutCode)
	if !ok {
		return message.Chat.ID, "", fmt.Errorf("cannot advance Workout %s", session.WorkoutCode)
	}
	if err := h.store.FinishSessionAndAdvance(ctx, session.ID, next, h.now()); err != nil {
		return message.Chat.ID, "", err
	}
	reply = fmt.Sprintf("Workout %s finished. Next: Workout %s.", session.WorkoutCode, next)
	return message.Chat.ID, incompleteWarning(reply, workout.Exercises, missing), nil
}

func incompleteWarning(reply string, exercises []sheetscheck.WorkoutExercise, missing []string) string {
	if len(missing) > 0 {
		reply += fmt.Sprintf("\nIncomplete workout: %d/%d exercises logged. Missing: %s.", len(exercises)-len(missing), len(exercises), strings.Join(missing, ", "))
	}
	return reply
}

func (h *Handler) logExercise(ctx context.Context, update telegram.Update) (int64, string, error) {
	message := update.Message
	workout, reply, err := h.authorizedWorkout(ctx, message.From.ID)
	if err != nil || reply != "" {
		return message.Chat.ID, reply, err
	}
	session, err := h.store.OpenSession(ctx)
	if err != nil {
		return message.Chat.ID, "", err
	}
	if session == nil {
		return message.Chat.ID, "No workout is open. Send /start.", nil
	}
	logged, err := h.store.SessionSets(ctx, session.ID)
	if err != nil {
		return message.Chat.ID, "", err
	}
	for _, set := range logged {
		if set.TelegramMessageID == message.ID {
			return message.Chat.ID, "That Telegram message is already logged.", nil
		}
	}
	exercise := nextExercise(workout.Exercises, logged)
	if exercise == nil {
		return message.Chat.ID, "Every exercise already has logged sets.", nil
	}
	plannedSets, err := strconv.Atoi(exercise.PlannedSets)
	if err != nil {
		return message.Chat.ID, "", fmt.Errorf("planned sets for %s are invalid: %w", exercise.Name, err)
	}
	parsed, err := parser.Parse(message.Text, parser.Context{
		DefaultLoadKG: exercise.SuggestedLoad,
		ExpectedSets:  plannedSets,
		RequireLoad:   requiresLoad(exercise.LoadMode) && exercise.SuggestedLoad == nil,
	})
	if err != nil {
		return message.Chat.ID, fmt.Sprintf("I couldn't read the %s result: %v\n\n%s", exercise.Name, err, formatLogPrompt(*exercise)), nil
	}
	if parsed.SubstitutionAlias != "" {
		return message.Chat.ID, "Substitutions are not enabled in this increment; edit the Sheet or log the planned exercise for now.", nil
	}

	now := h.now()
	values := make([]sheetscheck.Set, 0, len(parsed.Sets))
	for index, set := range parsed.Sets {
		notes := ""
		if index == 0 && len(parsed.Warnings) > 0 {
			notes = "warning: " + strings.Join(parsed.Warnings, "; ")
		}
		values = append(values, sheetscheck.Set{
			ID: fmt.Sprintf("set-u%d-%02d", update.ID, index+1), SessionID: session.ID,
			ProgramVersionID: session.ProgramVersionID, SlotID: exercise.SlotID,
			ExerciseID: exercise.ExerciseID, Number: index + 1, LoadKG: set.LoadKG,
			Reps: set.Reps, RIR: set.RIR, Pain: set.Pain,
			TelegramMessageID: message.ID, LoggedAt: now, Notes: notes,
		})
	}
	if err := h.store.AppendSets(ctx, values); err != nil {
		return message.Chat.ID, "", err
	}
	reply = formatLogged(exercise.Name, values, parsed.Warnings)
	for _, value := range values {
		logged = append(logged, sheetscheck.LoggedSet{SlotID: value.SlotID, TelegramMessageID: value.TelegramMessageID})
	}
	if next := nextExercise(workout.Exercises, logged); next != nil {
		reply += "\n\nNext: " + formatExercise(*next) + "\n\n" + formatReplyExample(*next)
	} else {
		reply += "\n\nEvery exercise now has logged sets."
	}
	return message.Chat.ID, reply, nil
}

func (h *Handler) authorizedWorkout(ctx context.Context, userID int64) (sheetscheck.Workout, string, error) {
	workout, err := h.store.CurrentWorkout(ctx)
	if err != nil {
		return sheetscheck.Workout{}, "", err
	}
	if workout.AuthorizedTelegramUserID == 0 {
		return sheetscheck.Workout{}, fmt.Sprintf(
			"Your Telegram user ID is %d. Add it to Config → authorized_telegram_user_id, then try again.",
			userID,
		), nil
	}
	if userID != workout.AuthorizedTelegramUserID {
		return sheetscheck.Workout{}, "This Tren bot is private.", nil
	}
	return workout, "", nil
}

func (h *Handler) authorizedRotation(ctx context.Context, userID int64) (sheetscheck.RotationProgram, string, error) {
	rotation, err := h.store.CurrentRotation(ctx)
	if err != nil {
		return sheetscheck.RotationProgram{}, "", err
	}
	if rotation.AuthorizedTelegramUserID == 0 {
		return sheetscheck.RotationProgram{}, fmt.Sprintf(
			"Your Telegram user ID is %d. Add it to Config → authorized_telegram_user_id, then try again.",
			userID,
		), nil
	}
	if userID != rotation.AuthorizedTelegramUserID {
		return sheetscheck.RotationProgram{}, "This Tren bot is private.", nil
	}
	return rotation, "", nil
}

func formatWorkout(workout sheetscheck.Workout) string {
	lines := []string{fmt.Sprintf("Workout %s · %s", workout.Code, workout.ProgramVersion), ""}
	for _, exercise := range workout.Exercises {
		lines = append(lines, formatExercise(exercise))
	}
	return strings.Join(lines, "\n")
}

func formatRotation(rotation sheetscheck.RotationProgram) string {
	lines := []string{
		fmt.Sprintf("Rotation %d · %s", rotation.RotationNumber, rotation.ProgramVersion),
		fmt.Sprintf("Next: Workout %s", rotation.NextWorkout),
	}
	for _, workout := range rotation.Workouts {
		lines = append(lines, "", "WORKOUT "+workout.Code)
		for _, exercise := range workout.Exercises {
			lines = append(lines, formatExercise(exercise))
		}
	}
	return strings.Join(lines, "\n")
}

func formatExercise(exercise sheetscheck.WorkoutExercise) string {
	line := fmt.Sprintf("%d. %s — %s×%s", exercise.Position, exercise.Name, exercise.PlannedSets, formatRange(exercise.RepMin, exercise.RepMax, "reps"))
	if rir := formatRange(exercise.RIRMin, exercise.RIRMax, ""); rir != "" {
		line += " @" + rir + " RIR"
	}
	if exercise.SuggestedLoadKG != "" {
		line += " · " + exercise.SuggestedLoadKG + " kg"
	} else if exercise.LoadGuidance != "" {
		line += " · " + exercise.LoadGuidance
	}
	return line
}

func formatLogPrompt(exercise sheetscheck.WorkoutExercise) string {
	return "Log " + exercise.Name + " next.\n" + formatReplyExample(exercise)
}

func formatReplyExample(exercise sheetscheck.WorkoutExercise) string {
	sets, err := strconv.Atoi(exercise.PlannedSets)
	if err != nil || sets < 1 {
		sets = 1
	}
	if sets > 8 {
		sets = 8
	}
	reps := exercise.RepMin
	if reps == "" {
		reps = "10"
	}
	rir := exercise.RIRMin
	if rir == "" {
		rir = "2"
	}
	set := reps + "@" + rir
	parts := make([]string, sets)
	for index := range parts {
		parts[index] = set
	}
	example := strings.Join(parts, " ")
	if requiresLoad(exercise.LoadMode) && exercise.SuggestedLoad == nil {
		example = "50: " + example
	}
	return "Reply like: " + example + "\nAdd p1–p10 only if there is pain."
}

func nextExercise(exercises []sheetscheck.WorkoutExercise, logged []sheetscheck.LoggedSet) *sheetscheck.WorkoutExercise {
	completed := make(map[string]bool)
	for _, set := range logged {
		completed[set.SlotID] = true
	}
	for index := range exercises {
		if !completed[exercises[index].SlotID] {
			return &exercises[index]
		}
	}
	return nil
}

func exerciseBySlot(exercises []sheetscheck.WorkoutExercise, slotID string) *sheetscheck.WorkoutExercise {
	for index := range exercises {
		if exercises[index].SlotID == slotID {
			return &exercises[index]
		}
	}
	return nil
}

func plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func missingExercises(exercises []sheetscheck.WorkoutExercise, logged []sheetscheck.LoggedSet) []string {
	completed := make(map[string]bool)
	for _, set := range logged {
		completed[set.SlotID] = true
	}
	var missing []string
	for _, exercise := range exercises {
		if !completed[exercise.SlotID] {
			missing = append(missing, exercise.Name)
		}
	}
	return missing
}

func nextWorkout(current string) (string, bool) {
	switch current {
	case "A":
		return "B", true
	case "B":
		return "C", true
	case "C":
		return "D", true
	default:
		return "", false
	}
}

func requiresLoad(loadMode string) bool {
	switch loadMode {
	case "bodyweight", "bodyweight_angle", "duration", "other":
		return false
	default:
		return true
	}
}

func formatLogged(name string, sets []sheetscheck.Set, warnings []string) string {
	parts := make([]string, 0, len(sets))
	for _, set := range sets {
		part := ""
		if set.LoadKG != nil {
			part = strconv.FormatFloat(*set.LoadKG, 'f', -1, 64) + "×"
		}
		part += strconv.Itoa(set.Reps)
		if set.RIR != nil {
			part += "@" + strconv.FormatFloat(*set.RIR, 'f', -1, 64)
		}
		parts = append(parts, part)
	}
	reply := "Logged " + name + ": " + strings.Join(parts, ", ")
	if len(sets) > 0 && sets[0].Pain != nil {
		reply += fmt.Sprintf("; pain %d", *sets[0].Pain)
	}
	if len(warnings) > 0 {
		reply += ".\nWarning: " + strings.Join(warnings, "; ")
	}
	return reply + "."
}

func formatRange(minimum, maximum, fallback string) string {
	if minimum == "" && maximum == "" {
		return fallback
	}
	if minimum == "" {
		return maximum
	}
	if maximum == "" || minimum == maximum {
		return minimum
	}
	return minimum + "–" + maximum
}

func command(text string) string {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return ""
	}
	return strings.ToLower(strings.SplitN(fields[0], "@", 2)[0])
}

func help() string {
	return "Tren workout logger.\n\n/current — show the next workout\n/rotation — show the complete A–D rotation\n/start — open the next workout\n/resume — show the open workout\n/undoexercise — delete the last logged exercise\n/finish — complete and advance the workout\n/undo — delete the latest completed workout\n/howto — show how to log your sets\n/whoami — show your Telegram user ID\n/help — show these commands"
}

func howTo() string {
	return `HOW TO LOG A WORKOUT

1. Send /start.
2. After each exercise, send one message with its working sets.
3. Send /finish when the workout is over.

Made a mistake while the workout is open? Send /undoexercise to delete the most recently logged exercise, then enter it again.

Finished the wrong workout? Send /undo to permanently delete the latest completed session and its sets, and make that workout next again.

SET FORMAT
reps@RIR

RIR means how many more reps you could have done.
Example: 5@2 = 5 reps with 2 reps left.

THREE SETS AT THE PLANNED LOAD
5@3 5@2 4@1

Tren already knows the planned load, so you normally omit it. Spaces, commas, and semicolons all work.

USE A DIFFERENT LOAD FOR EVERY SET
75x5@3 75x5@2 72.5x6@2

USE ONE DIFFERENT LOAD FOR ALL SETS
75: 5@3 5@2 4@1

For weighted pull-ups, added load can be written as:
+10: 5@2 5@2 4@1

PAIN
Add p1 to p10 at the end only when there is pain:
5@3 5@2 4@1 p2

Warm-up sets are not logged. If you complete fewer working sets, just send the sets you completed; Tren will save them with a warning.`
}
