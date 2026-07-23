package sheetscheck

import (
	"testing"

	"github.com/eltonsst/tren/internal/domain"
)

func TestClonedProgramRowsRestoresTargetsAndPreservesUnknownColumns(t *testing.T) {
	headers := []any{
		"version_id", "workout_code", "position", "slot_id", "exercise_id", "planned_sets", "target_sets",
		"rep_min", "rep_max", "planned_rir_min", "planned_rir_max", "target_rir_min", "target_rir_max",
		"suggested_load_kg", "load_guidance", "progression_rule", "increment_kg", "home_alternatives",
		"gym_alternatives", "notes", "custom_formula",
	}
	values := [][]any{
		headers,
		{"block1-r1", "A", 1, "A01", "bench_press", 3, 4, 3, 5, 3, 3, 2, 2, 72.5, "", "all_sets_top", 2.5, "", "", "", "=A2&\" custom\""},
	}
	program, err := newTable("Program", values)
	if err != nil {
		t.Fatal(err)
	}
	prescriptions, err := prescriptionsForVersion(program, "block1-r1")
	if err != nil {
		t.Fatal(err)
	}
	next := prescriptions[0]
	next.VersionID = "block1-r2"
	next.PlannedSets = next.TargetSets
	next.PlannedRIRMin = next.TargetRIRMin
	next.PlannedRIRMax = next.TargetRIRMax

	rows, restored, err := clonedProgramRows(program, "block1-r1", []domain.Prescription{next})
	if err != nil {
		t.Fatal(err)
	}
	if restored != 1 || len(rows) != 1 {
		t.Fatalf("restored = %d, rows = %d", restored, len(rows))
	}
	row := rows[0]
	if row[program.index["version_id"]] != "block1-r2" || row[program.index["planned_sets"]] != 4 || row[program.index["planned_rir_min"]] != float64(2) {
		t.Fatalf("row = %#v", row)
	}
	if row[program.index["custom_formula"]] != "=A2&\" custom\"" {
		t.Fatalf("custom formula = %#v", row[program.index["custom_formula"]])
	}
}

func TestWorkingSetsForVersionParsesLocaleAndOptionalValues(t *testing.T) {
	sets, err := newTable("Sets", [][]any{
		{"program_version_id", "slot_id", "exercise_id", "set_number", "load_kg", "reps", "rir", "pain_0_10", "is_substitution"},
		{"block1-r1", "A01", "bench_press", 1, "72,5", 5, 2, 0, false},
		{"other", "A01", "bench_press", 1, 80, 5, 2, 0, false},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := workingSetsForVersion(sets, "block1-r1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].LoadKG == nil || *result[0].LoadKG != 72.5 || result[0].Pain == nil || *result[0].Pain != 0 {
		t.Fatalf("sets = %#v", result)
	}
}

func TestWriteRowsUsesOneBasedSheetRowsAndPreservesFormulas(t *testing.T) {
	request := writeRows(42, 3, [][]any{{"block1-r2", "=A3&\"!\"", 2}})
	update := request.UpdateCells
	if update == nil || update.Start.SheetId != 42 || update.Start.RowIndex != 2 || update.Start.ColumnIndex != 0 {
		t.Fatalf("update = %#v", update)
	}
	cells := update.Rows[0].Values
	if cells[0].UserEnteredValue.StringValue == nil || *cells[0].UserEnteredValue.StringValue != "block1-r2" {
		t.Fatalf("string cell = %#v", cells[0])
	}
	if cells[1].UserEnteredValue.FormulaValue == nil || *cells[1].UserEnteredValue.FormulaValue != "=A3&\"!\"" {
		t.Fatalf("formula cell = %#v", cells[1])
	}
	if cells[2].UserEnteredValue.NumberValue == nil || *cells[2].UserEnteredValue.NumberValue != 2 {
		t.Fatalf("number cell = %#v", cells[2])
	}
}
