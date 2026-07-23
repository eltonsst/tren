package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	painPattern       = regexp.MustCompile(`(?i)(?:^|[\s,;])(?:pain|p)\s*(10|[0-9])(?:/10)?\s*$`)
	loadPrefixPattern = regexp.MustCompile(`(?i)^\s*([+-]?\d+(?:[.,]\d+)?)\s*(?:kg)?\s*:\s*`)
	setPattern        = regexp.MustCompile(`(?i)^\s*(?:([+-]?\d+(?:[.,]\d+)?)\s*(?:kg)?\s*[x×]\s*)?(\d+)\s*(?:@\s*(\d+(?:[.,]\d+)?))?\s*$`)
	setTokenPattern   = regexp.MustCompile(`(?i)(?:[+-]?\d+(?:[.,]\d+)?\s*(?:kg)?\s*[x×]\s*)?\d+\s*(?:@\s*\d+(?:[.,]\d+)?)?`)
	separatorPattern  = regexp.MustCompile(`^[\s,;]*$`)
	setStartPattern   = regexp.MustCompile(`(?i)([+-]?\d+(?:[.,]\d+)?\s*(?:kg)?\s*:|\d+\s*@)`)
)

func Parse(input string, ctx Context) (Result, error) {
	text := strings.TrimSpace(input)
	if text == "" {
		return Result{}, fmt.Errorf("empty exercise result")
	}

	result := Result{}
	pain, stripped, err := extractPain(text)
	if err != nil {
		return Result{}, err
	}
	text = stripped

	if rest, ok := stripCommandPrefix(text, "swap", "sub"); ok {
		start := setStartPattern.FindStringIndex(rest)
		if start == nil {
			return Result{}, fmt.Errorf("substitution is missing set results")
		}
		result.SubstitutionAlias = strings.TrimSpace(rest[:start[0]])
		if result.SubstitutionAlias == "" {
			return Result{}, fmt.Errorf("substitution exercise is missing")
		}
		text = strings.TrimSpace(rest[start[0]:])
	}

	defaultLoad := cloneFloat(ctx.DefaultLoadKG)
	if match := loadPrefixPattern.FindStringSubmatch(text); match != nil {
		load, parseErr := parseDecimal(match[1])
		if parseErr != nil {
			return Result{}, fmt.Errorf("invalid load %q", match[1])
		}
		defaultLoad = &load
		text = strings.TrimSpace(text[len(match[0]):])
	}

	parts, err := splitSets(text)
	if err != nil {
		return Result{}, err
	}
	if len(parts) == 0 {
		return Result{}, fmt.Errorf("no sets found")
	}

	missingRIR := false
	for index, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		match := setPattern.FindStringSubmatch(part)
		if match == nil {
			return Result{}, fmt.Errorf("set %d has invalid format %q", index+1, part)
		}

		reps, parseErr := strconv.Atoi(match[2])
		if parseErr != nil || reps < 0 || reps > 100 {
			return Result{}, fmt.Errorf("set %d has invalid reps %q", index+1, match[2])
		}

		load := cloneFloat(defaultLoad)
		if match[1] != "" {
			parsed, parseErr := parseDecimal(match[1])
			if parseErr != nil || parsed < -500 || parsed > 1000 {
				return Result{}, fmt.Errorf("set %d has invalid load %q", index+1, match[1])
			}
			load = &parsed
		}
		if ctx.RequireLoad && load == nil {
			return Result{}, fmt.Errorf("set %d is missing a load", index+1)
		}

		var rir *float64
		if match[3] != "" {
			parsed, parseErr := parseDecimal(match[3])
			if parseErr != nil || parsed < 0 || parsed > 10 {
				return Result{}, fmt.Errorf("set %d has invalid RIR %q", index+1, match[3])
			}
			rir = &parsed
		} else {
			missingRIR = true
		}

		result.Sets = append(result.Sets, Set{
			LoadKG: load,
			Reps:   reps,
			RIR:    rir,
			Pain:   cloneInt(pain),
		})
	}

	if len(result.Sets) == 0 {
		return Result{}, fmt.Errorf("no sets found")
	}
	if ctx.ExpectedSets > 0 && len(result.Sets) != ctx.ExpectedSets {
		result.Warnings = append(result.Warnings, fmt.Sprintf("expected %d sets, received %d", ctx.ExpectedSets, len(result.Sets)))
	}
	if missingRIR {
		result.Warnings = append(result.Warnings, "one or more sets are missing RIR")
	}
	if pain == nil {
		result.Warnings = append(result.Warnings, "pain was not recorded")
	}

	return result, nil
}

func splitSets(input string) ([]string, error) {
	matches := setTokenPattern.FindAllStringIndex(input, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("set 1 has invalid format %q", strings.TrimSpace(input))
	}
	parts := make([]string, 0, len(matches))
	cursor := 0
	for _, match := range matches {
		if separator := input[cursor:match[0]]; !separatorPattern.MatchString(separator) {
			return nil, fmt.Errorf("set %d has invalid format %q", len(parts)+1, strings.TrimSpace(separator))
		}
		parts = append(parts, strings.TrimSpace(input[match[0]:match[1]]))
		cursor = match[1]
	}
	if remainder := input[cursor:]; !separatorPattern.MatchString(remainder) {
		return nil, fmt.Errorf("set %d has invalid format %q", len(parts)+1, strings.TrimSpace(remainder))
	}
	return parts, nil
}

func extractPain(input string) (*int, string, error) {
	location := painPattern.FindStringIndex(input)
	if location == nil {
		return nil, input, nil
	}
	match := painPattern.FindStringSubmatch(input[location[0]:location[1]])
	value, err := strconv.Atoi(match[1])
	if err != nil || value < 0 || value > 10 {
		return nil, "", fmt.Errorf("invalid pain value")
	}
	stripped := strings.TrimSpace(input[:location[0]])
	stripped = strings.TrimRight(stripped, " ,;")
	return &value, stripped, nil
}

func stripCommandPrefix(input string, commands ...string) (string, bool) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return input, false
	}
	for _, command := range commands {
		if strings.EqualFold(fields[0], command) {
			return strings.TrimSpace(input[len(fields[0]):]), true
		}
	}
	return input, false
}

func parseDecimal(value string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
