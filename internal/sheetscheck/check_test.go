package sheetscheck

import "testing"

func TestNewTableUsesHeaderNames(t *testing.T) {
	table, err := newTable("Config", [][]any{
		{"description", "value", "key"},
		{"", "1", "schema_version"},
	})
	if err != nil {
		t.Fatalf("newTable() error = %v", err)
	}
	if got := table.rows[0].value(table.index, "key"); got != "schema_version" {
		t.Fatalf("key = %q", got)
	}
	if got := table.rows[0].value(table.index, "value"); got != "1" {
		t.Fatalf("value = %q", got)
	}
}

func TestRequiredColumnError(t *testing.T) {
	table, err := newTable("Program", [][]any{{"version_id", "workout_code"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := table.require("version_id", "exercise_id"); err == nil {
		t.Fatal("require() did not reject a missing column")
	}
}

func TestParseIntegerAcceptsSheetsNumbers(t *testing.T) {
	for _, value := range []string{"1", "1.0"} {
		got, err := parseInteger(value)
		if err != nil || got != 1 {
			t.Fatalf("parseInteger(%q) = %d, %v", value, got, err)
		}
	}
}

func TestRecordAlignsSessionWithReorderedHeaders(t *testing.T) {
	table, err := newTable("Sessions", [][]any{{"status", "custom", "session_id"}})
	if err != nil {
		t.Fatal(err)
	}
	row, err := table.record(map[string]any{"session_id": "session-1", "status": "open"})
	if err != nil {
		t.Fatal(err)
	}
	if row[0] != "open" || row[1] != nil || row[2] != "session-1" {
		t.Fatalf("row = %#v", row)
	}
}

func TestParseOptionalFloatAcceptsSheetLocale(t *testing.T) {
	value, err := parseOptionalFloat("72,50")
	if err != nil || value == nil || *value != 72.5 {
		t.Fatalf("value = %v, err = %v", value, err)
	}
}

func TestFindRawRowPreservesSheetRowNumberAcrossBlanks(t *testing.T) {
	values := [][]any{
		{"status", "session_id"},
		{},
		{"completed", "old"},
		{"open", "wanted"},
	}
	rowNumber, headers, row, err := findRawRow("Sessions", values, "session_id", "wanted")
	if err != nil {
		t.Fatal(err)
	}
	if rowNumber != 4 || rawValue(row, headers, "status") != "open" {
		t.Fatalf("row = %d, status = %q", rowNumber, rawValue(row, headers, "status"))
	}
}

func TestColumnName(t *testing.T) {
	for index, expected := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z", "AA"} {
		if got := columnName(index); got != expected {
			t.Fatalf("columnName(%d) = %q, want %q", index, got, expected)
		}
	}
}
