package sheetscheck

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/sheets/v4"
)

type WorkoutUndoResult struct {
	SessionID             string
	ProgramVersionID      string
	RotationNumber        int
	WorkoutCode           string
	RemovedSets           int
	RemovedProgramVersion string
}

type workoutUndoPlan struct {
	result WorkoutUndoResult

	sessionRow int64
	setSpans   []rowSpan

	nextWorkoutConfigRow   int
	activeVersionConfigRow int
	configValueColumn      int

	previousVersionRow       int
	versionStatusColumn      int
	versionCompletedAtColumn int
	versionLockedAtColumn    int
	clearPreviousVersionLock bool
	generatedVersionRow      int64
	generatedProgramSpans    []rowSpan
}

// UndoLastWorkout permanently removes the latest completed session and its
// working sets, then rewinds Config so the same workout is next again. If the
// session completed Workout D, the untouched generated version is removed and
// the previous version is reactivated in the same Sheets batch transaction.
func (c *Checker) UndoLastWorkout(ctx context.Context) (*WorkoutUndoResult, error) {
	sessionValues, err := c.readValues(ctx, "Sessions")
	if err != nil {
		return nil, err
	}
	setValues, err := c.readValues(ctx, "Sets")
	if err != nil {
		return nil, err
	}
	configValues, err := c.readValues(ctx, "Config")
	if err != nil {
		return nil, err
	}
	versionValues, err := c.readValues(ctx, "ProgramVersions")
	if err != nil {
		return nil, err
	}
	programValues, err := c.readValues(ctx, "Program")
	if err != nil {
		return nil, err
	}

	plan, err := planUndoLastWorkout(sessionValues, setValues, configValues, versionValues, programValues)
	if err != nil || plan == nil {
		return nil, err
	}

	names := []string{"Sessions", "Sets", "Config", "ProgramVersions"}
	if plan.result.RemovedProgramVersion != "" {
		names = append(names, "Program")
	}
	sheetIDs, err := c.sheetIDs(ctx, names...)
	if err != nil {
		return nil, err
	}

	requests := make([]*sheets.Request, 0, len(plan.setSpans)+len(plan.generatedProgramSpans)+8)
	requests = appendDeleteRows(requests, sheetIDs["Sets"], plan.setSpans)
	requests = append(requests, updateCell(
		sheetIDs["Config"], plan.nextWorkoutConfigRow, plan.configValueColumn, plan.result.WorkoutCode,
	))

	if plan.result.RemovedProgramVersion == "" {
		if plan.clearPreviousVersionLock {
			requests = append(requests, updateCell(
				sheetIDs["ProgramVersions"], plan.previousVersionRow, plan.versionLockedAtColumn, "",
			))
		}
	} else {
		requests = append(requests,
			updateCell(sheetIDs["ProgramVersions"], plan.previousVersionRow, plan.versionStatusColumn, "active"),
			updateCell(sheetIDs["ProgramVersions"], plan.previousVersionRow, plan.versionCompletedAtColumn, ""),
			updateCell(sheetIDs["Config"], plan.activeVersionConfigRow, plan.configValueColumn, plan.result.ProgramVersionID),
		)
		requests = appendDeleteRows(requests, sheetIDs["Program"], plan.generatedProgramSpans)
		requests = append(requests, deleteRowsRequest(sheetIDs["ProgramVersions"], rowSpan{
			start: plan.generatedVersionRow, end: plan.generatedVersionRow + 1,
		}))
	}
	requests = append(requests, deleteRowsRequest(sheetIDs["Sessions"], rowSpan{
		start: plan.sessionRow, end: plan.sessionRow + 1,
	}))

	if _, err := c.service.Spreadsheets.BatchUpdate(
		c.spreadsheetID,
		&sheets.BatchUpdateSpreadsheetRequest{Requests: requests},
	).Context(ctx).Do(); err != nil {
		return nil, fmt.Errorf("undo Workout %s session: %w", plan.result.WorkoutCode, err)
	}
	return &plan.result, nil
}

func planUndoLastWorkout(sessionValues, setValues, configValues, versionValues, programValues [][]any) (*workoutUndoPlan, error) {
	sessions, err := newTable("Sessions", sessionValues)
	if err != nil {
		return nil, err
	}
	if err := sessions.require("session_id", "program_version_id", "rotation_number", "workout_code", "finished_at", "status"); err != nil {
		return nil, err
	}

	latestRow := 0
	var latestFinished time.Time
	for index := 1; index < len(sessionValues); index++ {
		status := strings.ToLower(rawValue(sessionValues[index], sessions.index, "status"))
		if status == "open" {
			return nil, fmt.Errorf("Workout %s is still open; finish it before undoing a completed workout", strings.ToUpper(rawValue(sessionValues[index], sessions.index, "workout_code")))
		}
		if status != "completed" {
			continue
		}
		finished, parseErr := time.Parse(time.RFC3339, rawValue(sessionValues[index], sessions.index, "finished_at"))
		if parseErr != nil {
			return nil, fmt.Errorf("Sessions finished_at: %w", parseErr)
		}
		if latestRow == 0 || finished.After(latestFinished) || (finished.Equal(latestFinished) && index > latestRow) {
			latestRow = index
			latestFinished = finished
		}
	}
	if latestRow == 0 {
		return nil, nil
	}

	session := sessionValues[latestRow]
	rotation, err := parseInteger(rawValue(session, sessions.index, "rotation_number"))
	if err != nil {
		return nil, fmt.Errorf("Sessions rotation_number: %w", err)
	}
	workoutCode := strings.ToUpper(rawValue(session, sessions.index, "workout_code"))
	if !validWorkout(workoutCode) {
		return nil, fmt.Errorf("Sessions workout_code %q is invalid", workoutCode)
	}
	plan := &workoutUndoPlan{
		result: WorkoutUndoResult{
			SessionID:        rawValue(session, sessions.index, "session_id"),
			ProgramVersionID: rawValue(session, sessions.index, "program_version_id"),
			RotationNumber:   rotation,
			WorkoutCode:      workoutCode,
		},
		// The values slice index equals the zero-based row index in Sheets.
		sessionRow: int64(latestRow),
	}
	if plan.result.SessionID == "" || plan.result.ProgramVersionID == "" {
		return nil, fmt.Errorf("latest completed session has incomplete identity")
	}

	sets, err := newTable("Sets", setValues)
	if err != nil {
		return nil, err
	}
	if err := sets.require("session_id"); err != nil {
		return nil, err
	}
	var setRows []int64
	for index := 1; index < len(setValues); index++ {
		if rawValue(setValues[index], sets.index, "session_id") == plan.result.SessionID {
			setRows = append(setRows, int64(index))
		}
	}
	plan.result.RemovedSets = len(setRows)
	plan.setSpans = groupRows(setRows)

	nextWorkoutRow, configHeaders, nextWorkout, err := findRawRow("Config", configValues, "key", "next_workout")
	if err != nil {
		return nil, err
	}
	activeVersionRow, activeHeaders, activeVersion, err := findRawRow("Config", configValues, "key", "active_program_version")
	if err != nil {
		return nil, err
	}
	if err := requireHeaders("Config", configHeaders, "value"); err != nil {
		return nil, err
	}
	plan.nextWorkoutConfigRow = nextWorkoutRow
	plan.activeVersionConfigRow = activeVersionRow
	plan.configValueColumn = configHeaders["value"]
	configuredWorkout := strings.ToUpper(rawValue(nextWorkout, configHeaders, "value"))
	configuredVersion := rawValue(activeVersion, activeHeaders, "value")

	versions, err := newTable("ProgramVersions", versionValues)
	if err != nil {
		return nil, err
	}
	if err := versions.require("version_id", "rotation_number", "status", "locked_at", "completed_at", "source_version_id"); err != nil {
		return nil, err
	}
	previousRowNumber, versionHeaders, previousVersion, err := findRawRow("ProgramVersions", versionValues, "version_id", plan.result.ProgramVersionID)
	if err != nil {
		return nil, err
	}
	plan.previousVersionRow = previousRowNumber
	plan.versionStatusColumn = versionHeaders["status"]
	plan.versionCompletedAtColumn = versionHeaders["completed_at"]
	plan.versionLockedAtColumn = versionHeaders["locked_at"]

	if workoutCode != "D" {
		expectedNext, _ := workoutAfter(workoutCode)
		if configuredWorkout != expectedNext || configuredVersion != plan.result.ProgramVersionID {
			return nil, fmt.Errorf("latest completed Workout %s does not match the current program position", workoutCode)
		}
		if !strings.EqualFold(rawValue(previousVersion, versionHeaders, "status"), "active") {
			return nil, fmt.Errorf("program version %s is not active", plan.result.ProgramVersionID)
		}
		if workoutCode == "A" {
			plan.clearPreviousVersionLock = !hasOtherSessionForVersion(sessionValues, sessions.index, latestRow, plan.result.ProgramVersionID)
		}
		return plan, nil
	}

	if configuredWorkout != "A" || configuredVersion == plan.result.ProgramVersionID {
		return nil, fmt.Errorf("latest completed Workout D does not match the current program position")
	}
	if !strings.EqualFold(rawValue(previousVersion, versionHeaders, "status"), "completed") {
		return nil, fmt.Errorf("program version %s is not completed", plan.result.ProgramVersionID)
	}
	generatedRowNumber, generatedHeaders, generatedVersion, err := findRawRow("ProgramVersions", versionValues, "version_id", configuredVersion)
	if err != nil {
		return nil, err
	}
	generatedRotation, err := parseInteger(rawValue(generatedVersion, generatedHeaders, "rotation_number"))
	if err != nil {
		return nil, fmt.Errorf("ProgramVersions rotation_number: %w", err)
	}
	if rawValue(generatedVersion, generatedHeaders, "source_version_id") != plan.result.ProgramVersionID ||
		generatedRotation != rotation+1 ||
		!strings.EqualFold(rawValue(generatedVersion, generatedHeaders, "status"), "active") ||
		rawValue(generatedVersion, generatedHeaders, "locked_at") != "" {
		return nil, fmt.Errorf("generated program version %s is no longer safe to remove", configuredVersion)
	}
	if hasAnySessionForVersion(sessionValues, sessions.index, configuredVersion) {
		return nil, fmt.Errorf("generated program version %s already has a workout session", configuredVersion)
	}

	program, err := newTable("Program", programValues)
	if err != nil {
		return nil, err
	}
	if err := program.require("version_id"); err != nil {
		return nil, err
	}
	var programRows []int64
	for index := 1; index < len(programValues); index++ {
		if rawValue(programValues[index], program.index, "version_id") == configuredVersion {
			programRows = append(programRows, int64(index))
		}
	}
	if len(programRows) == 0 {
		return nil, fmt.Errorf("Program contains no rows for generated version %s", configuredVersion)
	}
	plan.result.RemovedProgramVersion = configuredVersion
	plan.generatedVersionRow = int64(generatedRowNumber - 1)
	plan.generatedProgramSpans = groupRows(programRows)
	return plan, nil
}

func workoutAfter(code string) (string, bool) {
	switch code {
	case "A":
		return "B", true
	case "B":
		return "C", true
	case "C":
		return "D", true
	case "D":
		return "A", true
	default:
		return "", false
	}
}

func hasOtherSessionForVersion(values [][]any, headers map[string]int, excludedRow int, versionID string) bool {
	for index := 1; index < len(values); index++ {
		if index != excludedRow && rawValue(values[index], headers, "program_version_id") == versionID {
			return true
		}
	}
	return false
}

func hasAnySessionForVersion(values [][]any, headers map[string]int, versionID string) bool {
	return hasOtherSessionForVersion(values, headers, -1, versionID)
}

func groupRows(rows []int64) []rowSpan {
	spans := make([]rowSpan, 0, len(rows))
	for _, row := range rows {
		if len(spans) == 0 || spans[len(spans)-1].end != row {
			spans = append(spans, rowSpan{start: row, end: row + 1})
			continue
		}
		spans[len(spans)-1].end = row + 1
	}
	return spans
}

func appendDeleteRows(requests []*sheets.Request, sheetID int64, spans []rowSpan) []*sheets.Request {
	for index := len(spans) - 1; index >= 0; index-- {
		requests = append(requests, deleteRowsRequest(sheetID, spans[index]))
	}
	return requests
}

func deleteRowsRequest(sheetID int64, span rowSpan) *sheets.Request {
	return &sheets.Request{DeleteDimension: &sheets.DeleteDimensionRequest{Range: &sheets.DimensionRange{
		SheetId: sheetID, Dimension: "ROWS", StartIndex: span.start, EndIndex: span.end,
	}}}
}
