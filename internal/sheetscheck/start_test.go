package sheetscheck

import (
	"strings"
	"testing"
	"time"
)

func TestPlanSessionStartLocksWorkoutAAndPreservesSheetRowNumber(t *testing.T) {
	session := testStartSession("A")
	plan, err := planSessionStart(
		[][]any{
			sessionHeaders(),
			{},
			{"old", "block1-r1", 1, "D", "2026-07-15T10:00:00Z", "2026-07-15T11:00:00Z", "completed", "123", 1, ""},
		},
		configValues("block1-r1", "A"),
		versionValues("block1-r1", 1, "active", ""),
		session,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.lockProgramVersion || plan.sessionRowNumber != 4 || plan.versionRowNumber != 2 {
		t.Fatalf("plan = %#v", plan)
	}
	table, err := newTable("Sessions", [][]any{sessionHeaders()})
	if err != nil {
		t.Fatal(err)
	}
	if got := row(plan.sessionRow).value(table.index, "session_id"); got != "session-1" {
		t.Fatalf("session_id = %q", got)
	}
	if got := row(plan.sessionRow).value(table.index, "status"); got != "open" {
		t.Fatalf("status = %q", got)
	}
}

func TestPlanSessionStartAllowsAlreadyLockedVersion(t *testing.T) {
	for _, workout := range []string{"A", "B"} {
		t.Run(workout, func(t *testing.T) {
			session := testStartSession(workout)
			plan, err := planSessionStart(
				[][]any{sessionHeaders()}, configValues("block1-r1", workout),
				versionValues("block1-r1", 1, "active", "2026-07-16T09:00:00Z"), session,
			)
			if err != nil {
				t.Fatal(err)
			}
			if plan.lockProgramVersion {
				t.Fatalf("already locked version planned another lock: %#v", plan)
			}
		})
	}
}

func TestPlanSessionStartRejectsWorkoutBAgainstUnlockedVersion(t *testing.T) {
	session := testStartSession("B")
	_, err := planSessionStart(
		[][]any{sessionHeaders()}, configValues("block1-r1", "B"),
		versionValues("block1-r1", 1, "active", ""), session,
	)
	if err == nil || !strings.Contains(err.Error(), "start Workout A first") {
		t.Fatalf("error = %v", err)
	}
}

func TestPlanSessionStartRejectsStaleOrConcurrentState(t *testing.T) {
	tests := []struct {
		name     string
		sessions [][]any
		config   [][]any
		error    string
	}{
		{
			name: "open session",
			sessions: [][]any{
				sessionHeaders(),
				{"existing", "block1-r1", 1, "A", "2026-07-16T09:00:00Z", "", "open", "123", 1, ""},
			},
			config: configValues("block1-r1", "A"), error: "already open",
		},
		{
			name: "config changed", sessions: [][]any{sessionHeaders()},
			config: configValues("block1-r1", "B"), error: "does not match",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := planSessionStart(test.sessions, test.config, versionValues("block1-r1", 1, "active", ""), testStartSession("A"))
			if err == nil || !strings.Contains(err.Error(), test.error) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func testStartSession(workout string) Session {
	return Session{
		ID: "session-1", ProgramVersionID: "block1-r1", RotationNumber: 1, WorkoutCode: workout,
		StartedAt: time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC), TelegramChatID: 123,
		StartMessageID: 9,
	}
}

func sessionHeaders() []any {
	return []any{"session_id", "program_version_id", "rotation_number", "workout_code", "started_at", "finished_at", "status", "telegram_chat_id", "start_message_id", "notes"}
}

func configValues(version, workout string) [][]any {
	return [][]any{
		{"key", "value"},
		{"active_program_version", version},
		{"next_workout", workout},
	}
}

func versionValues(version string, rotation int, status, lockedAt string) [][]any {
	return [][]any{
		{"version_id", "rotation_number", "status", "locked_at"},
		{version, rotation, status, lockedAt},
	}
}
