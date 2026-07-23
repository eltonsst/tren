package parser

import (
	"strings"
	"testing"
)

func TestParseContextShorthand(t *testing.T) {
	load := 72.5
	got, err := Parse("5@2, 5@2, 5@1, 4@1 pain0", Context{DefaultLoadKG: &load, ExpectedSets: 4, RequireLoad: true})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(got.Sets) != 4 {
		t.Fatalf("len(Sets) = %d, want 4", len(got.Sets))
	}
	if *got.Sets[0].LoadKG != 72.5 || got.Sets[0].Reps != 5 || *got.Sets[0].RIR != 2 || *got.Sets[0].Pain != 0 {
		t.Fatalf("first set = %#v", got.Sets[0])
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("warnings = %v", got.Warnings)
	}
}

func TestParseLoadOverrideWithDecimalComma(t *testing.T) {
	got, err := Parse("77,5kg: 8@2; 7@1 p1", Context{ExpectedSets: 2, RequireLoad: true})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if *got.Sets[1].LoadKG != 77.5 || *got.Sets[1].Pain != 1 {
		t.Fatalf("second set = %#v", got.Sets[1])
	}
}

func TestParseSpaceSeparatedSets(t *testing.T) {
	load := 70.0
	got, err := Parse("3@3 3@3 3@3", Context{DefaultLoadKG: &load, ExpectedSets: 3, RequireLoad: true})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(got.Sets) != 3 || got.Sets[2].Reps != 3 || got.Sets[2].RIR == nil || *got.Sets[2].RIR != 3 {
		t.Fatalf("sets = %#v", got.Sets)
	}
}

func TestParseSpaceSeparatedSetsWithLoadAndPain(t *testing.T) {
	got, err := Parse("77,5kg: 8@2 7@1 p1", Context{ExpectedSets: 2, RequireLoad: true})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(got.Sets) != 2 || got.Sets[0].LoadKG == nil || *got.Sets[0].LoadKG != 77.5 || got.Sets[1].Pain == nil || *got.Sets[1].Pain != 1 {
		t.Fatalf("sets = %#v", got.Sets)
	}
}

func TestParseMixedSetSeparators(t *testing.T) {
	load := 70.0
	got, err := Parse("3@3, 3@3; 3@3", Context{DefaultLoadKG: &load, ExpectedSets: 3})
	if err != nil || len(got.Sets) != 3 {
		t.Fatalf("sets = %#v, err = %v", got.Sets, err)
	}
}

func TestParseExplicitLoads(t *testing.T) {
	got, err := Parse("75x5@2, 75x5@1, 72.5x6@1 pain0", Context{ExpectedSets: 3, RequireLoad: true})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if *got.Sets[2].LoadKG != 72.5 || got.Sets[2].Reps != 6 {
		t.Fatalf("third set = %#v", got.Sets[2])
	}
}

func TestParseBodyweight(t *testing.T) {
	got, err := Parse("8@2, 7@1, 6@1 pain0", Context{ExpectedSets: 3})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Sets[0].LoadKG != nil {
		t.Fatalf("load = %v, want nil", got.Sets[0].LoadKG)
	}
}

func TestParseSubstitution(t *testing.T) {
	got, err := Parse("swap cable pushdown 25: 12@2, 11@1 pain0", Context{ExpectedSets: 2, RequireLoad: true})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.SubstitutionAlias != "cable pushdown" {
		t.Fatalf("SubstitutionAlias = %q", got.SubstitutionAlias)
	}
	if *got.Sets[0].LoadKG != 25 {
		t.Fatalf("load = %v", *got.Sets[0].LoadKG)
	}
}

func TestParseWarnings(t *testing.T) {
	load := 30.0
	got, err := Parse("10, 9", Context{DefaultLoadKG: &load, ExpectedSets: 3, RequireLoad: true})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	joined := strings.Join(got.Warnings, "|")
	for _, wanted := range []string{"expected 3 sets, received 2", "missing RIR", "pain was not recorded"} {
		if !strings.Contains(joined, wanted) {
			t.Fatalf("warnings %q do not contain %q", joined, wanted)
		}
	}
}

func TestParseRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		ctx   Context
	}{
		{name: "empty", input: ""},
		{name: "bad set", input: "five@two"},
		{name: "bad rir", input: "5@11"},
		{name: "missing load", input: "5@2 pain0", ctx: Context{RequireLoad: true}},
		{name: "missing substitution sets", input: "swap cable pushdown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Parse(test.input, test.ctx); err == nil {
				t.Fatal("Parse() error = nil")
			}
		})
	}
}
