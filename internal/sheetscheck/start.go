package sheetscheck

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/sheets/v4"
)

type sessionStartPlan struct {
	sessionRow         []any
	sessionRowNumber   int
	versionRowNumber   int
	versionHeaders     map[string]int
	lockProgramVersion bool
}

func (c *Checker) startSession(ctx context.Context, session Session) error {
	sessionValues, err := c.readValues(ctx, "Sessions")
	if err != nil {
		return err
	}
	configValues, err := c.readValues(ctx, "Config")
	if err != nil {
		return err
	}
	versionValues, err := c.readValues(ctx, "ProgramVersions")
	if err != nil {
		return err
	}
	plan, err := planSessionStart(sessionValues, configValues, versionValues, session)
	if err != nil {
		return err
	}

	sheetIDs, err := c.sheetIDs(ctx, "Sessions", "ProgramVersions")
	if err != nil {
		return err
	}
	requests := []*sheets.Request{
		writeRows(sheetIDs["Sessions"], plan.sessionRowNumber, [][]any{plan.sessionRow}),
	}
	if plan.lockProgramVersion {
		requests = append(requests, updateCell(
			sheetIDs["ProgramVersions"], plan.versionRowNumber,
			plan.versionHeaders["locked_at"], session.StartedAt.Format(time.RFC3339),
		))
	}
	if _, err := c.service.Spreadsheets.BatchUpdate(c.spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{Requests: requests}).Context(ctx).Do(); err != nil {
		return fmt.Errorf("start Workout %s session: %w", session.WorkoutCode, err)
	}
	return nil
}

func planSessionStart(sessionValues, configValues, versionValues [][]any, session Session) (sessionStartPlan, error) {
	if session.ID == "" || session.ProgramVersionID == "" || !validWorkout(session.WorkoutCode) || session.RotationNumber < 1 || session.StartedAt.IsZero() {
		return sessionStartPlan{}, fmt.Errorf("session identity is incomplete")
	}

	sessions, err := newTable("Sessions", sessionValues)
	if err != nil {
		return sessionStartPlan{}, err
	}
	if err := sessions.require(
		"session_id", "program_version_id", "rotation_number", "workout_code", "started_at",
		"finished_at", "status", "telegram_chat_id", "start_message_id", "notes",
	); err != nil {
		return sessionStartPlan{}, err
	}
	for _, source := range sessions.rows {
		if strings.EqualFold(source.value(sessions.index, "status"), "open") {
			return sessionStartPlan{}, fmt.Errorf("session %s is already open", source.value(sessions.index, "session_id"))
		}
	}

	_, configHeaders, nextWorkout, err := findRawRow("Config", configValues, "key", "next_workout")
	if err != nil {
		return sessionStartPlan{}, err
	}
	_, activeHeaders, activeVersion, err := findRawRow("Config", configValues, "key", "active_program_version")
	if err != nil {
		return sessionStartPlan{}, err
	}
	if err := requireHeaders("Config", configHeaders, "value"); err != nil {
		return sessionStartPlan{}, err
	}
	if !strings.EqualFold(rawValue(nextWorkout, configHeaders, "value"), session.WorkoutCode) || rawValue(activeVersion, activeHeaders, "value") != session.ProgramVersionID {
		return sessionStartPlan{}, fmt.Errorf("session does not match the active program and next workout")
	}

	versionRowNumber, versionHeaders, version, err := findRawRow("ProgramVersions", versionValues, "version_id", session.ProgramVersionID)
	if err != nil {
		return sessionStartPlan{}, err
	}
	if err := requireHeaders("ProgramVersions", versionHeaders, "rotation_number", "status", "locked_at"); err != nil {
		return sessionStartPlan{}, err
	}
	if !strings.EqualFold(rawValue(version, versionHeaders, "status"), "active") {
		return sessionStartPlan{}, fmt.Errorf("program version %s is not active", session.ProgramVersionID)
	}
	rotation, err := parseInteger(rawValue(version, versionHeaders, "rotation_number"))
	if err != nil {
		return sessionStartPlan{}, fmt.Errorf("ProgramVersions rotation_number: %w", err)
	}
	if rotation != session.RotationNumber {
		return sessionStartPlan{}, fmt.Errorf("session rotation %d does not match program rotation %d", session.RotationNumber, rotation)
	}
	locked := rawValue(version, versionHeaders, "locked_at") != ""
	if session.WorkoutCode != "A" && !locked {
		return sessionStartPlan{}, fmt.Errorf("program version %s is not locked; start Workout A first", session.ProgramVersionID)
	}

	record, err := sessions.record(map[string]any{
		"session_id": session.ID, "program_version_id": session.ProgramVersionID,
		"rotation_number": session.RotationNumber, "workout_code": session.WorkoutCode,
		"started_at": session.StartedAt.Format(time.RFC3339), "finished_at": "", "status": "open",
		"telegram_chat_id": strconv.FormatInt(session.TelegramChatID, 10),
		"start_message_id": session.StartMessageID, "notes": session.Notes,
	})
	if err != nil {
		return sessionStartPlan{}, err
	}
	return sessionStartPlan{
		sessionRow: record, sessionRowNumber: len(sessionValues) + 1,
		versionRowNumber: versionRowNumber, versionHeaders: versionHeaders,
		lockProgramVersion: session.WorkoutCode == "A" && !locked,
	}, nil
}
