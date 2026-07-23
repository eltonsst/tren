package sheetscheck

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/eltonsst/tren/internal/domain"
	"github.com/eltonsst/tren/internal/progression"
	"google.golang.org/api/sheets/v4"
)

type RotationRollover struct {
	PreviousVersion       string
	NextVersion           string
	PreviousRotation      int
	NextRotation          int
	PrescriptionCount     int
	RestoredPrescriptions int
	IncreasedLoads        int
}

func (c *Checker) FinishRotation(ctx context.Context, sessionID string, finishedAt time.Time) (RotationRollover, error) {
	if sessionID == "" {
		return RotationRollover{}, fmt.Errorf("session ID is required")
	}

	sessionValues, err := c.readValues(ctx, "Sessions")
	if err != nil {
		return RotationRollover{}, err
	}
	sessionRow, sessionHeaders, session, err := findRawRow("Sessions", sessionValues, "session_id", sessionID)
	if err != nil {
		return RotationRollover{}, err
	}
	if !strings.EqualFold(rawValue(session, sessionHeaders, "status"), "open") {
		return RotationRollover{}, fmt.Errorf("session %s is not open", sessionID)
	}
	if !strings.EqualFold(rawValue(session, sessionHeaders, "workout_code"), "D") {
		return RotationRollover{}, fmt.Errorf("session %s is not Workout D", sessionID)
	}
	if err := requireHeaders("Sessions", sessionHeaders, "finished_at", "status", "program_version_id", "rotation_number"); err != nil {
		return RotationRollover{}, err
	}
	currentVersionID := rawValue(session, sessionHeaders, "program_version_id")
	sessionRotation, err := parseInteger(rawValue(session, sessionHeaders, "rotation_number"))
	if err != nil {
		return RotationRollover{}, fmt.Errorf("Sessions rotation_number: %w", err)
	}

	configValues, err := c.readValues(ctx, "Config")
	if err != nil {
		return RotationRollover{}, err
	}
	nextWorkoutRow, configHeaders, nextWorkout, err := findRawRow("Config", configValues, "key", "next_workout")
	if err != nil {
		return RotationRollover{}, err
	}
	activeVersionRow, activeConfigHeaders, activeVersion, err := findRawRow("Config", configValues, "key", "active_program_version")
	if err != nil {
		return RotationRollover{}, err
	}
	if err := requireHeaders("Config", configHeaders, "value"); err != nil {
		return RotationRollover{}, err
	}
	if !strings.EqualFold(rawValue(nextWorkout, configHeaders, "value"), "D") {
		return RotationRollover{}, fmt.Errorf("Config next_workout is not D")
	}
	if rawValue(activeVersion, activeConfigHeaders, "value") != currentVersionID {
		return RotationRollover{}, fmt.Errorf("open session does not match the active program version")
	}

	versionValues, err := c.readFormulaValues(ctx, "ProgramVersions")
	if err != nil {
		return RotationRollover{}, err
	}
	currentVersionRow, versionHeaders, versionRow, err := findRawRow("ProgramVersions", versionValues, "version_id", currentVersionID)
	if err != nil {
		return RotationRollover{}, err
	}
	versionTable, err := newTable("ProgramVersions", versionValues)
	if err != nil {
		return RotationRollover{}, err
	}
	if err := versionTable.require("version_id", "block_id", "rotation_number", "status", "created_at", "activated_at", "locked_at", "completed_at", "source_version_id", "created_by", "notes"); err != nil {
		return RotationRollover{}, err
	}
	if !strings.EqualFold(rawValue(versionRow, versionHeaders, "status"), "active") {
		return RotationRollover{}, fmt.Errorf("program version %s is not active", currentVersionID)
	}
	currentRotation, err := parseInteger(rawValue(versionRow, versionHeaders, "rotation_number"))
	if err != nil {
		return RotationRollover{}, fmt.Errorf("ProgramVersions rotation_number: %w", err)
	}
	if currentRotation != sessionRotation {
		return RotationRollover{}, fmt.Errorf("session rotation %d does not match program rotation %d", sessionRotation, currentRotation)
	}
	current := domain.ProgramVersion{
		ID: currentVersionID, BlockID: rawValue(versionRow, versionHeaders, "block_id"),
		RotationNumber: currentRotation, Status: domain.ProgramCompleted, CompletedAt: &finishedAt,
	}

	programValues, err := c.readFormulaValues(ctx, "Program")
	if err != nil {
		return RotationRollover{}, err
	}
	programTable, err := newTable("Program", programValues)
	if err != nil {
		return RotationRollover{}, err
	}
	prescriptions, err := prescriptionsForVersion(programTable, currentVersionID)
	if err != nil {
		return RotationRollover{}, err
	}

	setValues, err := c.readValues(ctx, "Sets")
	if err != nil {
		return RotationRollover{}, err
	}
	setTable, err := newTable("Sets", setValues)
	if err != nil {
		return RotationRollover{}, err
	}
	workingSets, err := workingSetsForVersion(setTable, currentVersionID)
	if err != nil {
		return RotationRollover{}, err
	}

	result, err := progression.NextVersion(current, prescriptions, workingSets, finishedAt)
	if err != nil {
		return RotationRollover{}, err
	}
	if tableContains(versionTable, "version_id", result.Version.ID) || tableContains(programTable, "version_id", result.Version.ID) {
		return RotationRollover{}, fmt.Errorf("next program version %s already exists", result.Version.ID)
	}

	nextVersionRow, err := clonedVersionRow(versionTable, row(versionRow), result.Version)
	if err != nil {
		return RotationRollover{}, err
	}
	nextProgramRows, restored, err := clonedProgramRows(programTable, currentVersionID, result.Prescriptions)
	if err != nil {
		return RotationRollover{}, err
	}
	increased := 0
	for _, decision := range result.Decisions {
		if decision.LoadIncreased {
			increased++
		}
	}

	sheetIDs, err := c.sheetIDs(ctx, "Config", "ProgramVersions", "Program", "Sessions")
	if err != nil {
		return RotationRollover{}, err
	}
	requests := []*sheets.Request{
		updateCell(sheetIDs["Sessions"], sessionRow, sessionHeaders["finished_at"], finishedAt.Format(time.RFC3339)),
		updateCell(sheetIDs["Sessions"], sessionRow, sessionHeaders["status"], "completed"),
		updateCell(sheetIDs["ProgramVersions"], currentVersionRow, versionHeaders["status"], "completed"),
		updateCell(sheetIDs["ProgramVersions"], currentVersionRow, versionHeaders["completed_at"], finishedAt.Format(time.RFC3339)),
		writeRows(sheetIDs["ProgramVersions"], len(versionValues)+1, [][]any{nextVersionRow}),
		writeRows(sheetIDs["Program"], len(programValues)+1, nextProgramRows),
		updateCell(sheetIDs["Config"], activeVersionRow, activeConfigHeaders["value"], result.Version.ID),
		updateCell(sheetIDs["Config"], nextWorkoutRow, configHeaders["value"], "A"),
	}
	if _, err := c.service.Spreadsheets.BatchUpdate(c.spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{Requests: requests}).Context(ctx).Do(); err != nil {
		return RotationRollover{}, fmt.Errorf("finish rotation and create %s: %w", result.Version.ID, err)
	}

	return RotationRollover{
		PreviousVersion: currentVersionID, NextVersion: result.Version.ID,
		PreviousRotation: currentRotation, NextRotation: result.Version.RotationNumber,
		PrescriptionCount: len(result.Prescriptions), RestoredPrescriptions: restored, IncreasedLoads: increased,
	}, nil
}

func prescriptionsForVersion(program table, versionID string) ([]domain.Prescription, error) {
	required := []string{"version_id", "workout_code", "position", "slot_id", "exercise_id", "planned_sets", "target_sets", "rep_min", "rep_max", "planned_rir_min", "planned_rir_max", "target_rir_min", "target_rir_max", "suggested_load_kg", "load_guidance", "progression_rule", "increment_kg", "home_alternatives", "gym_alternatives", "notes"}
	if err := program.require(required...); err != nil {
		return nil, err
	}
	var result []domain.Prescription
	for _, source := range program.rows {
		if source.value(program.index, "version_id") != versionID {
			continue
		}
		workout := domain.WorkoutCode(strings.ToUpper(source.value(program.index, "workout_code")))
		if !workout.Valid() {
			return nil, fmt.Errorf("Program workout_code %q is invalid", workout)
		}
		position, err := parseInteger(source.value(program.index, "position"))
		if err != nil {
			return nil, fmt.Errorf("Program position: %w", err)
		}
		plannedSets, err := parseInteger(source.value(program.index, "planned_sets"))
		if err != nil {
			return nil, fmt.Errorf("Program planned_sets: %w", err)
		}
		targetSets, err := parseInteger(source.value(program.index, "target_sets"))
		if err != nil {
			return nil, fmt.Errorf("Program target_sets: %w", err)
		}
		repMin, err := parseOptionalInteger(source.value(program.index, "rep_min"))
		if err != nil {
			return nil, fmt.Errorf("Program rep_min: %w", err)
		}
		repMax, err := parseOptionalInteger(source.value(program.index, "rep_max"))
		if err != nil {
			return nil, fmt.Errorf("Program rep_max: %w", err)
		}
		plannedRIRMin, err := parseOptionalFloat(source.value(program.index, "planned_rir_min"))
		if err != nil {
			return nil, fmt.Errorf("Program planned_rir_min: %w", err)
		}
		plannedRIRMax, err := parseOptionalFloat(source.value(program.index, "planned_rir_max"))
		if err != nil {
			return nil, fmt.Errorf("Program planned_rir_max: %w", err)
		}
		targetRIRMin, err := parseOptionalFloat(source.value(program.index, "target_rir_min"))
		if err != nil {
			return nil, fmt.Errorf("Program target_rir_min: %w", err)
		}
		targetRIRMax, err := parseOptionalFloat(source.value(program.index, "target_rir_max"))
		if err != nil {
			return nil, fmt.Errorf("Program target_rir_max: %w", err)
		}
		load, err := parseOptionalFloat(source.value(program.index, "suggested_load_kg"))
		if err != nil {
			return nil, fmt.Errorf("Program suggested_load_kg: %w", err)
		}
		increment, err := parseOptionalFloat(source.value(program.index, "increment_kg"))
		if err != nil {
			return nil, fmt.Errorf("Program increment_kg: %w", err)
		}
		result = append(result, domain.Prescription{
			VersionID: versionID, Workout: workout, Position: position,
			SlotID: source.value(program.index, "slot_id"), ExerciseID: source.value(program.index, "exercise_id"),
			PlannedSets: plannedSets, TargetSets: targetSets, RepMin: repMin, RepMax: repMax,
			PlannedRIRMin: plannedRIRMin, PlannedRIRMax: plannedRIRMax, TargetRIRMin: targetRIRMin, TargetRIRMax: targetRIRMax,
			SuggestedLoadKG: load, LoadGuidance: source.value(program.index, "load_guidance"),
			ProgressionRule: domain.ProgressionRule(source.value(program.index, "progression_rule")), IncrementKG: increment,
			HomeAlternatives: splitIDs(source.value(program.index, "home_alternatives")), GymAlternatives: splitIDs(source.value(program.index, "gym_alternatives")),
			Notes: source.value(program.index, "notes"),
		})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("Program contains no prescriptions for version %s", versionID)
	}
	return result, nil
}

func workingSetsForVersion(sets table, versionID string) ([]domain.WorkingSet, error) {
	if err := sets.require("program_version_id", "slot_id", "exercise_id", "set_number", "load_kg", "reps", "rir", "pain_0_10", "is_substitution"); err != nil {
		return nil, err
	}
	var result []domain.WorkingSet
	for _, source := range sets.rows {
		if source.value(sets.index, "program_version_id") != versionID {
			continue
		}
		number, err := parseInteger(source.value(sets.index, "set_number"))
		if err != nil {
			return nil, fmt.Errorf("Sets set_number: %w", err)
		}
		reps, err := parseInteger(source.value(sets.index, "reps"))
		if err != nil {
			return nil, fmt.Errorf("Sets reps: %w", err)
		}
		load, err := parseOptionalFloat(source.value(sets.index, "load_kg"))
		if err != nil {
			return nil, fmt.Errorf("Sets load_kg: %w", err)
		}
		rir, err := parseOptionalFloat(source.value(sets.index, "rir"))
		if err != nil {
			return nil, fmt.Errorf("Sets rir: %w", err)
		}
		pain, err := parseOptionalInteger(source.value(sets.index, "pain_0_10"))
		if err != nil {
			return nil, fmt.Errorf("Sets pain_0_10: %w", err)
		}
		substitution, err := parseOptionalBool(source.value(sets.index, "is_substitution"))
		if err != nil {
			return nil, fmt.Errorf("Sets is_substitution: %w", err)
		}
		result = append(result, domain.WorkingSet{
			ProgramVersionID: versionID, SlotID: source.value(sets.index, "slot_id"), ExerciseID: source.value(sets.index, "exercise_id"),
			Number: number, LoadKG: load, Reps: reps, RIR: rir, Pain: pain, IsSubstitution: substitution,
		})
	}
	return result, nil
}

func clonedVersionRow(versions table, source row, version domain.ProgramVersion) ([]any, error) {
	return versions.mergeRecord(source, map[string]any{
		"version_id": version.ID, "block_id": version.BlockID, "rotation_number": version.RotationNumber,
		"status": string(version.Status), "created_at": version.CreatedAt.Format(time.RFC3339),
		"activated_at": optionalTime(version.ActivatedAt), "locked_at": optionalTime(version.LockedAt),
		"completed_at": optionalTime(version.CompletedAt), "source_version_id": version.SourceVersionID,
		"created_by": version.CreatedBy, "notes": version.Notes,
	})
}

func clonedProgramRows(program table, currentVersion string, prescriptions []domain.Prescription) ([][]any, int, error) {
	bySlot := make(map[string]domain.Prescription, len(prescriptions))
	for _, prescription := range prescriptions {
		if _, duplicate := bySlot[prescription.SlotID]; duplicate {
			return nil, 0, fmt.Errorf("next program has duplicate slot %s", prescription.SlotID)
		}
		bySlot[prescription.SlotID] = prescription
	}
	var rows [][]any
	restored := 0
	for _, source := range program.rows {
		if source.value(program.index, "version_id") != currentVersion {
			continue
		}
		slotID := source.value(program.index, "slot_id")
		prescription, found := bySlot[slotID]
		if !found {
			return nil, 0, fmt.Errorf("next program is missing slot %s", slotID)
		}
		plannedSets, _ := parseInteger(source.value(program.index, "planned_sets"))
		if plannedSets != prescription.PlannedSets || !sameOptionalFloatText(source.value(program.index, "planned_rir_min"), prescription.PlannedRIRMin) || !sameOptionalFloatText(source.value(program.index, "planned_rir_max"), prescription.PlannedRIRMax) {
			restored++
		}
		row, err := program.mergeRecord(source, prescriptionRecord(prescription))
		if err != nil {
			return nil, 0, err
		}
		rows = append(rows, row)
		delete(bySlot, slotID)
	}
	if len(bySlot) != 0 || len(rows) != len(prescriptions) {
		return nil, 0, fmt.Errorf("next program does not match the source program")
	}
	return rows, restored, nil
}

func prescriptionRecord(value domain.Prescription) map[string]any {
	return map[string]any{
		"version_id": value.VersionID, "workout_code": string(value.Workout), "position": value.Position,
		"slot_id": value.SlotID, "exercise_id": value.ExerciseID, "planned_sets": value.PlannedSets, "target_sets": value.TargetSets,
		"rep_min": optionalInt(value.RepMin), "rep_max": optionalInt(value.RepMax),
		"planned_rir_min": optionalFloat(value.PlannedRIRMin), "planned_rir_max": optionalFloat(value.PlannedRIRMax),
		"target_rir_min": optionalFloat(value.TargetRIRMin), "target_rir_max": optionalFloat(value.TargetRIRMax),
		"suggested_load_kg": optionalFloat(value.SuggestedLoadKG), "load_guidance": value.LoadGuidance,
		"progression_rule": string(value.ProgressionRule), "increment_kg": optionalFloat(value.IncrementKG),
		"home_alternatives": strings.Join(value.HomeAlternatives, "|"), "gym_alternatives": strings.Join(value.GymAlternatives, "|"),
		"notes": value.Notes,
	}
}

func (t table) mergeRecord(source row, values map[string]any) ([]any, error) {
	width := 0
	for _, column := range t.index {
		if column+1 > width {
			width = column + 1
		}
	}
	result := make([]any, width)
	copy(result, source)
	for header, value := range values {
		column, found := t.index[header]
		if !found {
			return nil, fmt.Errorf("sheet %s is missing column %q", t.name, header)
		}
		result[column] = value
	}
	return result, nil
}

func tableContains(value table, header, expected string) bool {
	for _, source := range value.rows {
		if source.value(value.index, header) == expected {
			return true
		}
	}
	return false
}

func requireHeaders(sheet string, headers map[string]int, required ...string) error {
	for _, header := range required {
		if _, found := headers[header]; !found {
			return fmt.Errorf("sheet %s is missing column %q", sheet, header)
		}
	}
	return nil
}

func parseOptionalInteger(value string) (*int, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := parseInteger(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseOptionalBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "false", "no", "0":
		return false, nil
	case "true", "yes", "1":
		return true, nil
	default:
		return false, fmt.Errorf("%q is not a boolean", value)
	}
}

func splitIDs(value string) []string {
	var result []string
	for _, part := range strings.Split(value, "|") {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func optionalTime(value *time.Time) any {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339)
}

func sameOptionalFloatText(text string, value *float64) bool {
	parsed, err := parseOptionalFloat(text)
	if err != nil {
		return false
	}
	if parsed == nil || value == nil {
		return parsed == nil && value == nil
	}
	return *parsed == *value
}

func (c *Checker) readFormulaValues(ctx context.Context, name string) ([][]any, error) {
	result, err := c.service.Spreadsheets.Values.Get(c.spreadsheetID, quoteSheet(name)+"!A:ZZ").ValueRenderOption("FORMULA").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return result.Values, nil
}

func (c *Checker) sheetIDs(ctx context.Context, names ...string) (map[string]int64, error) {
	spreadsheet, err := c.service.Spreadsheets.Get(c.spreadsheetID).Fields("sheets.properties").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("read sheet IDs: %w", err)
	}
	result := make(map[string]int64)
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[name] = true
	}
	for _, sheet := range spreadsheet.Sheets {
		if wanted[sheet.Properties.Title] {
			result[sheet.Properties.Title] = sheet.Properties.SheetId
		}
	}
	for _, name := range names {
		if _, found := result[name]; !found {
			return nil, fmt.Errorf("sheet %s was not found", name)
		}
	}
	return result, nil
}

func updateCell(sheetID int64, rowNumber, column int, value any) *sheets.Request {
	return writeRows(sheetID, rowNumber, [][]any{{value}}, column)
}

func writeRows(sheetID int64, rowNumber int, values [][]any, startColumn ...int) *sheets.Request {
	column := 0
	if len(startColumn) > 0 {
		column = startColumn[0]
	}
	rows := make([]*sheets.RowData, 0, len(values))
	for _, value := range values {
		cells := make([]*sheets.CellData, len(value))
		for index, cell := range value {
			cells[index] = cellData(cell)
		}
		rows = append(rows, &sheets.RowData{Values: cells})
	}
	return &sheets.Request{UpdateCells: &sheets.UpdateCellsRequest{
		Start: &sheets.GridCoordinate{SheetId: sheetID, RowIndex: int64(rowNumber - 1), ColumnIndex: int64(column)},
		Rows:  rows, Fields: "userEnteredValue",
	}}
}

func cellData(value any) *sheets.CellData {
	cell := &sheets.CellData{}
	switch typed := value.(type) {
	case nil:
		return cell
	case string:
		if strings.HasPrefix(typed, "=") {
			cell.UserEnteredValue = &sheets.ExtendedValue{FormulaValue: &typed}
		} else {
			cell.UserEnteredValue = &sheets.ExtendedValue{StringValue: &typed}
		}
	case bool:
		cell.UserEnteredValue = &sheets.ExtendedValue{BoolValue: &typed}
	case int:
		number := float64(typed)
		cell.UserEnteredValue = &sheets.ExtendedValue{NumberValue: &number}
	case int64:
		number := float64(typed)
		cell.UserEnteredValue = &sheets.ExtendedValue{NumberValue: &number}
	case float64:
		cell.UserEnteredValue = &sheets.ExtendedValue{NumberValue: &typed}
	default:
		text := fmt.Sprint(value)
		cell.UserEnteredValue = &sheets.ExtendedValue{StringValue: &text}
	}
	return cell
}
