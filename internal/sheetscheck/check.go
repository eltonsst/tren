package sheetscheck

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const supportedSchemaVersion = "1"

var requiredTabs = []string{
	"Config",
	"Exercises",
	"ProgramVersions",
	"Program",
	"Sessions",
	"Sets",
	"Measurements",
	"Inbox",
}

type Summary struct {
	SchemaVersion        string
	ActiveProgramVersion string
	NextWorkout          string
	NextWorkoutExercises int
	ExerciseCount        int
}

type Workout struct {
	ProgramVersion           string
	RotationNumber           int
	Code                     string
	AuthorizedTelegramUserID int64
	Exercises                []WorkoutExercise
}

type RotationProgram struct {
	ProgramVersion           string
	RotationNumber           int
	NextWorkout              string
	AuthorizedTelegramUserID int64
	Workouts                 []Workout
}

type Session struct {
	ID               string
	ProgramVersionID string
	RotationNumber   int
	WorkoutCode      string
	StartedAt        time.Time
	TelegramChatID   int64
	StartMessageID   int
	Notes            string
}

type LoggedSet struct {
	SlotID            string
	TelegramMessageID int
}

type Set struct {
	ID                string
	SessionID         string
	ProgramVersionID  string
	SlotID            string
	ExerciseID        string
	Number            int
	LoadKG            *float64
	Reps              int
	RIR               *float64
	Pain              *int
	IsSubstitution    bool
	TelegramMessageID int
	LoggedAt          time.Time
	Notes             string
}

type WorkoutExercise struct {
	Position        int
	SlotID          string
	ExerciseID      string
	Name            string
	LoadMode        string
	PlannedSets     string
	RepMin          string
	RepMax          string
	RIRMin          string
	RIRMax          string
	SuggestedLoadKG string
	SuggestedLoad   *float64
	LoadGuidance    string
}

type Checker struct {
	service       *sheets.Service
	spreadsheetID string
}

func NewReadOnly(ctx context.Context, spreadsheetID, credentialsSource string) (*Checker, error) {
	return newChecker(ctx, spreadsheetID, credentialsSource, sheets.SpreadsheetsReadonlyScope)
}

func NewReadWrite(ctx context.Context, spreadsheetID, credentialsSource string) (*Checker, error) {
	return newChecker(ctx, spreadsheetID, credentialsSource, sheets.SpreadsheetsScope)
}

func newChecker(ctx context.Context, spreadsheetID, credentialsSource, scope string) (*Checker, error) {
	if strings.TrimSpace(spreadsheetID) == "" {
		return nil, fmt.Errorf("spreadsheet ID is required")
	}
	credentialsSource = strings.TrimSpace(credentialsSource)
	if credentialsSource == "" {
		return nil, fmt.Errorf("Google service-account credentials are required")
	}
	credentialsOption := option.WithCredentialsFile(credentialsSource)
	if strings.HasPrefix(credentialsSource, "{") {
		credentialsOption = option.WithCredentialsJSON([]byte(credentialsSource))
	}
	service, err := sheets.NewService(
		ctx,
		credentialsOption,
		option.WithScopes(scope),
	)
	if err != nil {
		return nil, fmt.Errorf("open Google credentials: %w", err)
	}
	return &Checker{service: service, spreadsheetID: spreadsheetID}, nil
}

func (c *Checker) Check(ctx context.Context) (Summary, error) {
	if err := c.checkTabs(ctx); err != nil {
		return Summary{}, err
	}
	config, err := c.readTable(ctx, "Config")
	if err != nil {
		return Summary{}, err
	}
	if err := config.require("key", "value"); err != nil {
		return Summary{}, err
	}
	configValues := make(map[string]string)
	for _, row := range config.rows {
		configValues[row.value(config.index, "key")] = row.value(config.index, "value")
	}

	summary := Summary{
		SchemaVersion:        configValues["schema_version"],
		ActiveProgramVersion: configValues["active_program_version"],
		NextWorkout:          strings.ToUpper(configValues["next_workout"]),
	}
	if summary.SchemaVersion != supportedSchemaVersion {
		return Summary{}, fmt.Errorf("unsupported Sheets schema %q; Tren expects %s", summary.SchemaVersion, supportedSchemaVersion)
	}
	if summary.ActiveProgramVersion == "" {
		return Summary{}, fmt.Errorf("Config active_program_version is blank")
	}
	if !validWorkout(summary.NextWorkout) {
		return Summary{}, fmt.Errorf("Config next_workout %q is invalid", summary.NextWorkout)
	}

	exercises, err := c.readTable(ctx, "Exercises")
	if err != nil {
		return Summary{}, err
	}
	if err := exercises.require("exercise_id", "active"); err != nil {
		return Summary{}, err
	}
	for _, row := range exercises.rows {
		if row.value(exercises.index, "exercise_id") != "" {
			summary.ExerciseCount++
		}
	}

	program, err := c.readTable(ctx, "Program")
	if err != nil {
		return Summary{}, err
	}
	if err := program.require("version_id", "workout_code", "slot_id", "exercise_id"); err != nil {
		return Summary{}, err
	}
	for _, row := range program.rows {
		if row.value(program.index, "version_id") == summary.ActiveProgramVersion &&
			strings.EqualFold(row.value(program.index, "workout_code"), summary.NextWorkout) &&
			row.value(program.index, "exercise_id") != "" {
			summary.NextWorkoutExercises++
		}
	}
	if summary.NextWorkoutExercises == 0 {
		return Summary{}, fmt.Errorf("Program contains no exercises for version %s Workout %s", summary.ActiveProgramVersion, summary.NextWorkout)
	}
	return summary, nil
}

func (c *Checker) CurrentWorkout(ctx context.Context) (Workout, error) {
	rotation, err := c.currentRotationProgram(ctx, false)
	if err != nil {
		return Workout{}, err
	}
	if len(rotation.Workouts) != 1 {
		return Workout{}, fmt.Errorf("current rotation did not contain exactly one next workout")
	}
	return rotation.Workouts[0], nil
}

func (c *Checker) CurrentRotation(ctx context.Context) (RotationProgram, error) {
	return c.currentRotationProgram(ctx, true)
}

func (c *Checker) currentRotationProgram(ctx context.Context, includeAll bool) (RotationProgram, error) {
	configValues, err := c.config(ctx)
	if err != nil {
		return RotationProgram{}, err
	}
	rotation := RotationProgram{
		ProgramVersion: configValues["active_program_version"],
		NextWorkout:    strings.ToUpper(configValues["next_workout"]),
	}
	if rotation.ProgramVersion == "" || !validWorkout(rotation.NextWorkout) {
		return RotationProgram{}, fmt.Errorf("Config does not identify a valid current workout")
	}
	versions, err := c.readTable(ctx, "ProgramVersions")
	if err != nil {
		return RotationProgram{}, err
	}
	if err := versions.require("version_id", "rotation_number"); err != nil {
		return RotationProgram{}, err
	}
	for _, row := range versions.rows {
		if row.value(versions.index, "version_id") != rotation.ProgramVersion {
			continue
		}
		rotation.RotationNumber, err = parseInteger(row.value(versions.index, "rotation_number"))
		if err != nil {
			return RotationProgram{}, fmt.Errorf("ProgramVersions rotation_number: %w", err)
		}
		break
	}
	if rotation.RotationNumber < 1 {
		return RotationProgram{}, fmt.Errorf("program version %s has no valid rotation number", rotation.ProgramVersion)
	}
	if value := configValues["authorized_telegram_user_id"]; value != "" {
		rotation.AuthorizedTelegramUserID, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			return RotationProgram{}, fmt.Errorf("Config authorized_telegram_user_id %q is invalid", value)
		}
	}

	exerciseTable, err := c.readTable(ctx, "Exercises")
	if err != nil {
		return RotationProgram{}, err
	}
	if err := exerciseTable.require("exercise_id", "name", "load_mode"); err != nil {
		return RotationProgram{}, err
	}
	type exerciseDefinition struct {
		name     string
		loadMode string
	}
	exerciseDefinitions := make(map[string]exerciseDefinition)
	for _, row := range exerciseTable.rows {
		exerciseDefinitions[row.value(exerciseTable.index, "exercise_id")] = exerciseDefinition{
			name:     row.value(exerciseTable.index, "name"),
			loadMode: row.value(exerciseTable.index, "load_mode"),
		}
	}

	program, err := c.readTable(ctx, "Program")
	if err != nil {
		return RotationProgram{}, err
	}
	if err := program.require(
		"version_id", "workout_code", "position", "slot_id", "exercise_id", "planned_sets",
		"rep_min", "rep_max", "planned_rir_min", "planned_rir_max",
		"suggested_load_kg", "load_guidance",
	); err != nil {
		return RotationProgram{}, err
	}
	wanted := map[string]bool{rotation.NextWorkout: true}
	if includeAll {
		wanted = map[string]bool{"A": true, "B": true, "C": true, "D": true}
	}
	byCode := make(map[string]*Workout, len(wanted))
	for code := range wanted {
		byCode[code] = &Workout{
			ProgramVersion: rotation.ProgramVersion, RotationNumber: rotation.RotationNumber,
			Code: code, AuthorizedTelegramUserID: rotation.AuthorizedTelegramUserID,
		}
	}
	for _, row := range program.rows {
		code := strings.ToUpper(row.value(program.index, "workout_code"))
		workout, found := byCode[code]
		if row.value(program.index, "version_id") != rotation.ProgramVersion || !found {
			continue
		}
		exerciseID := row.value(program.index, "exercise_id")
		position, err := parseInteger(row.value(program.index, "position"))
		if err != nil {
			return RotationProgram{}, fmt.Errorf("Program position for %s: %w", exerciseID, err)
		}
		definition, found := exerciseDefinitions[exerciseID]
		if !found || definition.name == "" {
			return RotationProgram{}, fmt.Errorf("Program references unknown exercise %q", exerciseID)
		}
		suggestedLoadText := row.value(program.index, "suggested_load_kg")
		suggestedLoad, err := parseOptionalFloat(suggestedLoadText)
		if err != nil {
			return RotationProgram{}, fmt.Errorf("Program suggested_load_kg for %s: %w", exerciseID, err)
		}
		workout.Exercises = append(workout.Exercises, WorkoutExercise{
			Position: position, SlotID: row.value(program.index, "slot_id"), ExerciseID: exerciseID,
			Name: definition.name, LoadMode: definition.loadMode,
			PlannedSets: row.value(program.index, "planned_sets"),
			RepMin:      row.value(program.index, "rep_min"), RepMax: row.value(program.index, "rep_max"),
			RIRMin: row.value(program.index, "planned_rir_min"), RIRMax: row.value(program.index, "planned_rir_max"),
			SuggestedLoadKG: suggestedLoadText, SuggestedLoad: suggestedLoad,
			LoadGuidance: row.value(program.index, "load_guidance"),
		})
	}
	codes := []string{rotation.NextWorkout}
	if includeAll {
		codes = []string{"A", "B", "C", "D"}
	}
	for _, code := range codes {
		workout := byCode[code]
		sort.Slice(workout.Exercises, func(i, j int) bool {
			return workout.Exercises[i].Position < workout.Exercises[j].Position
		})
		if len(workout.Exercises) == 0 {
			return RotationProgram{}, fmt.Errorf("Program contains no exercises for version %s Workout %s", workout.ProgramVersion, code)
		}
		rotation.Workouts = append(rotation.Workouts, *workout)
	}
	return rotation, nil
}

func (c *Checker) OpenSession(ctx context.Context) (*Session, error) {
	sessions, err := c.readTable(ctx, "Sessions")
	if err != nil {
		return nil, err
	}
	if err := sessions.require(
		"session_id", "program_version_id", "rotation_number", "workout_code",
		"started_at", "status", "telegram_chat_id", "start_message_id", "notes",
	); err != nil {
		return nil, err
	}
	var open *Session
	for _, row := range sessions.rows {
		if !strings.EqualFold(row.value(sessions.index, "status"), "open") {
			continue
		}
		if open != nil {
			return nil, fmt.Errorf("Sessions contains more than one open workout")
		}
		rotation, err := parseInteger(row.value(sessions.index, "rotation_number"))
		if err != nil {
			return nil, fmt.Errorf("Sessions rotation_number: %w", err)
		}
		chatID, err := strconv.ParseInt(row.value(sessions.index, "telegram_chat_id"), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Sessions telegram_chat_id: %w", err)
		}
		messageID, err := parseInteger(row.value(sessions.index, "start_message_id"))
		if err != nil {
			return nil, fmt.Errorf("Sessions start_message_id: %w", err)
		}
		open = &Session{
			ID:               row.value(sessions.index, "session_id"),
			ProgramVersionID: row.value(sessions.index, "program_version_id"),
			RotationNumber:   rotation, WorkoutCode: strings.ToUpper(row.value(sessions.index, "workout_code")),
			TelegramChatID: chatID, StartMessageID: messageID, Notes: row.value(sessions.index, "notes"),
		}
	}
	return open, nil
}

func (c *Checker) StartSession(ctx context.Context, session Session) error {
	return c.startSession(ctx, session)
}

func (c *Checker) SessionSets(ctx context.Context, sessionID string) ([]LoggedSet, error) {
	sets, err := c.readTable(ctx, "Sets")
	if err != nil {
		return nil, err
	}
	if err := sets.require("session_id", "slot_id", "telegram_message_id"); err != nil {
		return nil, err
	}
	var result []LoggedSet
	for _, row := range sets.rows {
		if row.value(sets.index, "session_id") != sessionID {
			continue
		}
		messageID, err := parseInteger(row.value(sets.index, "telegram_message_id"))
		if err != nil {
			return nil, fmt.Errorf("Sets telegram_message_id: %w", err)
		}
		result = append(result, LoggedSet{
			SlotID: row.value(sets.index, "slot_id"), TelegramMessageID: messageID,
		})
	}
	return result, nil
}

func (c *Checker) AppendSets(ctx context.Context, values []Set) error {
	if len(values) == 0 {
		return fmt.Errorf("at least one set is required")
	}
	sets, err := c.readTable(ctx, "Sets")
	if err != nil {
		return err
	}
	rows := make([][]any, 0, len(values))
	for _, value := range values {
		record := map[string]any{
			"set_id": value.ID, "session_id": value.SessionID,
			"program_version_id": value.ProgramVersionID, "slot_id": value.SlotID,
			"exercise_id": value.ExerciseID, "set_number": value.Number,
			"load_kg": optionalFloat(value.LoadKG), "reps": value.Reps,
			"rir": optionalFloat(value.RIR), "pain_0_10": optionalInt(value.Pain),
			"is_substitution":     value.IsSubstitution,
			"telegram_message_id": value.TelegramMessageID,
			"logged_at":           value.LoggedAt.Format(time.RFC3339), "notes": value.Notes,
		}
		row, err := sets.record(record)
		if err != nil {
			return err
		}
		rows = append(rows, row)
	}
	_, err = c.service.Spreadsheets.Values.Append(
		c.spreadsheetID,
		quoteSheet("Sets")+"!A:ZZ",
		&sheets.ValueRange{Values: rows},
	).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("append Sets rows: %w", err)
	}
	return nil
}

func (c *Checker) FinishSessionAndAdvance(ctx context.Context, sessionID, nextWorkout string, finishedAt time.Time) error {
	if sessionID == "" || !validWorkout(nextWorkout) {
		return fmt.Errorf("finish request is incomplete")
	}
	sessionValues, err := c.readValues(ctx, "Sessions")
	if err != nil {
		return err
	}
	sessionRow, sessionHeaders, session, err := findRawRow("Sessions", sessionValues, "session_id", sessionID)
	if err != nil {
		return err
	}
	if !strings.EqualFold(rawValue(session, sessionHeaders, "status"), "open") {
		return fmt.Errorf("session %s is not open", sessionID)
	}
	for _, header := range []string{"finished_at", "status"} {
		if _, found := sessionHeaders[header]; !found {
			return fmt.Errorf("sheet Sessions is missing column %q", header)
		}
	}

	configValues, err := c.readValues(ctx, "Config")
	if err != nil {
		return err
	}
	configRow, configHeaders, _, err := findRawRow("Config", configValues, "key", "next_workout")
	if err != nil {
		return err
	}
	valueColumn, found := configHeaders["value"]
	if !found {
		return fmt.Errorf("sheet Config is missing column %q", "value")
	}

	request := &sheets.BatchUpdateValuesRequest{
		ValueInputOption: "USER_ENTERED",
		Data: []*sheets.ValueRange{
			{Range: cellRange("Sessions", sessionHeaders["finished_at"], sessionRow), Values: [][]any{{finishedAt.Format(time.RFC3339)}}},
			{Range: cellRange("Sessions", sessionHeaders["status"], sessionRow), Values: [][]any{{"completed"}}},
			{Range: cellRange("Config", valueColumn, configRow), Values: [][]any{{nextWorkout}}},
		},
	}
	if _, err := c.service.Spreadsheets.Values.BatchUpdate(c.spreadsheetID, request).Context(ctx).Do(); err != nil {
		return fmt.Errorf("finish session and advance workout: %w", err)
	}
	return nil
}

func (c *Checker) config(ctx context.Context) (map[string]string, error) {
	config, err := c.readTable(ctx, "Config")
	if err != nil {
		return nil, err
	}
	if err := config.require("key", "value"); err != nil {
		return nil, err
	}
	values := make(map[string]string)
	for _, row := range config.rows {
		values[row.value(config.index, "key")] = row.value(config.index, "value")
	}
	return values, nil
}

func (c *Checker) checkTabs(ctx context.Context) error {
	spreadsheet, err := c.service.Spreadsheets.Get(c.spreadsheetID).
		Fields("sheets.properties.title").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("read Google Sheet: %w", err)
	}
	found := make(map[string]bool)
	for _, sheet := range spreadsheet.Sheets {
		found[sheet.Properties.Title] = true
	}
	var missing []string
	for _, name := range requiredTabs {
		if !found[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("Google Sheet is missing tabs: %s", strings.Join(missing, ", "))
	}
	return nil
}

func (c *Checker) readTable(ctx context.Context, name string) (table, error) {
	values, err := c.readValues(ctx, name)
	if err != nil {
		return table{}, err
	}
	return newTable(name, values)
}

func (c *Checker) readValues(ctx context.Context, name string) ([][]any, error) {
	result, err := c.service.Spreadsheets.Values.Get(c.spreadsheetID, quoteSheet(name)+"!A:ZZ").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return result.Values, nil
}

type table struct {
	name  string
	index map[string]int
	rows  []row
}

type row []any

func newTable(name string, values [][]any) (table, error) {
	if len(values) == 0 {
		return table{}, fmt.Errorf("sheet %s has no header row", name)
	}
	result := table{name: name, index: make(map[string]int)}
	for column, value := range values[0] {
		header := strings.TrimSpace(fmt.Sprint(value))
		if header == "" {
			continue
		}
		if _, exists := result.index[header]; exists {
			return table{}, fmt.Errorf("sheet %s has duplicate header %q", name, header)
		}
		result.index[header] = column
	}
	for _, values := range values[1:] {
		if !blank(values) {
			result.rows = append(result.rows, row(values))
		}
	}
	return result, nil
}

func (t table) require(headers ...string) error {
	var missing []string
	for _, header := range headers {
		if _, found := t.index[header]; !found {
			missing = append(missing, header)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("sheet %s is missing columns: %s", t.name, strings.Join(missing, ", "))
	}
	return nil
}

func (t table) record(values map[string]any) ([]any, error) {
	width := 0
	for _, column := range t.index {
		if column+1 > width {
			width = column + 1
		}
	}
	result := make([]any, width)
	for header, value := range values {
		column, found := t.index[header]
		if !found {
			return nil, fmt.Errorf("sheet %s is missing column %q", t.name, header)
		}
		result[column] = value
	}
	return result, nil
}

func (r row) value(index map[string]int, header string) string {
	column, found := index[header]
	if !found || column >= len(r) || r[column] == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(r[column]))
}

func blank(values []any) bool {
	for _, value := range values {
		if strings.TrimSpace(fmt.Sprint(value)) != "" {
			return false
		}
	}
	return true
}

func quoteSheet(name string) string {
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

func findRawRow(sheet string, values [][]any, keyHeader, key string) (int, map[string]int, []any, error) {
	if len(values) == 0 {
		return 0, nil, nil, fmt.Errorf("sheet %s has no header row", sheet)
	}
	headers := make(map[string]int)
	for column, value := range values[0] {
		header := strings.TrimSpace(fmt.Sprint(value))
		if header != "" {
			headers[header] = column
		}
	}
	if _, found := headers[keyHeader]; !found {
		return 0, nil, nil, fmt.Errorf("sheet %s is missing column %q", sheet, keyHeader)
	}
	var foundRow int
	var foundValues []any
	for index, row := range values[1:] {
		if rawValue(row, headers, keyHeader) != key {
			continue
		}
		if foundRow != 0 {
			return 0, nil, nil, fmt.Errorf("sheet %s has duplicate %s %q", sheet, keyHeader, key)
		}
		foundRow = index + 2
		foundValues = row
	}
	if foundRow == 0 {
		return 0, nil, nil, fmt.Errorf("sheet %s has no row with %s %q", sheet, keyHeader, key)
	}
	return foundRow, headers, foundValues, nil
}

func rawValue(row []any, headers map[string]int, header string) string {
	column, found := headers[header]
	if !found || column >= len(row) || row[column] == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(row[column]))
}

func cellRange(sheet string, column, row int) string {
	return fmt.Sprintf("%s!%s%d", quoteSheet(sheet), columnName(column), row)
}

func columnName(index int) string {
	index++
	var result string
	for index > 0 {
		index--
		result = string(rune('A'+index%26)) + result
		index /= 26
	}
	return result
}

func validWorkout(value string) bool {
	switch value {
	case "A", "B", "C", "D":
		return true
	default:
		return false
	}
}

func parseInteger(value string) (int, error) {
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.Trunc(number) != number {
		return 0, fmt.Errorf("%q is not an integer", value)
	}
	return int(number), nil
}

func parseOptionalFloat(value string) (*float64, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", "."))
	if value == "" {
		return nil, nil
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, fmt.Errorf("%q is not a number", value)
	}
	return &number, nil
}

func optionalFloat(value *float64) any {
	if value == nil {
		return ""
	}
	return *value
}

func optionalInt(value *int) any {
	if value == nil {
		return ""
	}
	return *value
}
