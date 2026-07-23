package sheetscheck

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/eltonsst/tren/internal/stats"
)

func (c *Checker) StatsDataset(ctx context.Context) (stats.Dataset, error) {
	configValues, err := c.readValues(ctx, "Config")
	if err != nil {
		return stats.Dataset{}, err
	}
	exerciseValues, err := c.readValues(ctx, "Exercises")
	if err != nil {
		return stats.Dataset{}, err
	}
	programValues, err := c.readValues(ctx, "Program")
	if err != nil {
		return stats.Dataset{}, err
	}
	sessionValues, err := c.readValues(ctx, "Sessions")
	if err != nil {
		return stats.Dataset{}, err
	}
	setValues, err := c.readValues(ctx, "Sets")
	if err != nil {
		return stats.Dataset{}, err
	}
	return statsDatasetFromValues(configValues, exerciseValues, programValues, sessionValues, setValues)
}

func statsDatasetFromValues(configValues, exerciseValues, programValues, sessionValues, setValues [][]any) (stats.Dataset, error) {
	config, err := newTable("Config", configValues)
	if err != nil {
		return stats.Dataset{}, err
	}
	if err := config.require("key", "value"); err != nil {
		return stats.Dataset{}, err
	}
	settings := make(map[string]string)
	for _, source := range config.rows {
		settings[source.value(config.index, "key")] = source.value(config.index, "value")
	}
	if settings["active_program_version"] == "" {
		return stats.Dataset{}, fmt.Errorf("Config active_program_version is blank")
	}
	location := time.UTC
	if name := settings["timezone"]; name != "" {
		location, err = time.LoadLocation(name)
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Config timezone %q is invalid: %w", name, err)
		}
	}
	dataset := stats.Dataset{ActiveProgramVersion: settings["active_program_version"]}

	exercises, err := newTable("Exercises", exerciseValues)
	if err != nil {
		return stats.Dataset{}, err
	}
	if err := exercises.require("exercise_id", "name", "aliases", "load_mode", "volume_multiplier", "active"); err != nil {
		return stats.Dataset{}, err
	}
	for _, source := range exercises.rows {
		volume, err := parseOptionalFloat(source.value(exercises.index, "volume_multiplier"))
		if err != nil || volume == nil || *volume <= 0 {
			return stats.Dataset{}, fmt.Errorf("Exercises volume_multiplier for %s is invalid", source.value(exercises.index, "exercise_id"))
		}
		active, err := parseOptionalBool(source.value(exercises.index, "active"))
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Exercises active for %s: %w", source.value(exercises.index, "exercise_id"), err)
		}
		dataset.Exercises = append(dataset.Exercises, stats.Exercise{
			ID: source.value(exercises.index, "exercise_id"), Name: source.value(exercises.index, "name"),
			Aliases: splitIDs(source.value(exercises.index, "aliases")), LoadMode: source.value(exercises.index, "load_mode"),
			VolumeMultiplier: *volume, Active: active,
		})
	}

	program, err := newTable("Program", programValues)
	if err != nil {
		return stats.Dataset{}, err
	}
	if err := program.require(
		"version_id", "workout_code", "position", "slot_id", "exercise_id", "planned_sets",
		"rep_min", "rep_max", "planned_rir_min", "planned_rir_max", "suggested_load_kg", "load_guidance",
	); err != nil {
		return stats.Dataset{}, err
	}
	for _, source := range program.rows {
		position, err := parseInteger(source.value(program.index, "position"))
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Program position for %s: %w", source.value(program.index, "slot_id"), err)
		}
		plannedSets, err := parseInteger(source.value(program.index, "planned_sets"))
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Program planned_sets for %s: %w", source.value(program.index, "slot_id"), err)
		}
		repMin, err := parseOptionalInteger(source.value(program.index, "rep_min"))
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Program rep_min for %s: %w", source.value(program.index, "slot_id"), err)
		}
		repMax, err := parseOptionalInteger(source.value(program.index, "rep_max"))
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Program rep_max for %s: %w", source.value(program.index, "slot_id"), err)
		}
		rirMin, err := parseOptionalFloat(source.value(program.index, "planned_rir_min"))
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Program planned_rir_min for %s: %w", source.value(program.index, "slot_id"), err)
		}
		rirMax, err := parseOptionalFloat(source.value(program.index, "planned_rir_max"))
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Program planned_rir_max for %s: %w", source.value(program.index, "slot_id"), err)
		}
		load, err := parseOptionalFloat(source.value(program.index, "suggested_load_kg"))
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Program suggested_load_kg for %s: %w", source.value(program.index, "slot_id"), err)
		}
		dataset.Prescriptions = append(dataset.Prescriptions, stats.Prescription{
			VersionID: source.value(program.index, "version_id"), Workout: strings.ToUpper(source.value(program.index, "workout_code")),
			Position: position, SlotID: source.value(program.index, "slot_id"), ExerciseID: source.value(program.index, "exercise_id"),
			PlannedSets: plannedSets, RepMin: repMin, RepMax: repMax, RIRMin: rirMin, RIRMax: rirMax,
			SuggestedLoadKG: load, LoadGuidance: source.value(program.index, "load_guidance"),
		})
	}

	sessions, err := newTable("Sessions", sessionValues)
	if err != nil {
		return stats.Dataset{}, err
	}
	if err := sessions.require("session_id", "program_version_id", "rotation_number", "workout_code", "started_at", "finished_at", "status"); err != nil {
		return stats.Dataset{}, err
	}
	for _, source := range sessions.rows {
		rotation, err := parseInteger(source.value(sessions.index, "rotation_number"))
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Sessions rotation_number for %s: %w", source.value(sessions.index, "session_id"), err)
		}
		started, err := parseStatsTime(source.value(sessions.index, "started_at"), location)
		if err != nil || started == nil {
			return stats.Dataset{}, fmt.Errorf("Sessions started_at for %s is invalid", source.value(sessions.index, "session_id"))
		}
		finished, err := parseStatsTime(source.value(sessions.index, "finished_at"), location)
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Sessions finished_at for %s: %w", source.value(sessions.index, "session_id"), err)
		}
		dataset.Sessions = append(dataset.Sessions, stats.Session{
			ID: source.value(sessions.index, "session_id"), ProgramVersionID: source.value(sessions.index, "program_version_id"),
			RotationNumber: rotation, Workout: strings.ToUpper(source.value(sessions.index, "workout_code")),
			StartedAt: *started, FinishedAt: finished, Status: source.value(sessions.index, "status"),
		})
	}

	sets, err := newTable("Sets", setValues)
	if err != nil {
		return stats.Dataset{}, err
	}
	if err := sets.require("session_id", "program_version_id", "slot_id", "exercise_id", "set_number", "load_kg", "reps", "rir", "pain_0_10", "is_substitution", "logged_at"); err != nil {
		return stats.Dataset{}, err
	}
	for _, source := range sets.rows {
		number, err := parseInteger(source.value(sets.index, "set_number"))
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Sets set_number: %w", err)
		}
		reps, err := parseInteger(source.value(sets.index, "reps"))
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Sets reps: %w", err)
		}
		load, err := parseOptionalFloat(source.value(sets.index, "load_kg"))
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Sets load_kg: %w", err)
		}
		rir, err := parseOptionalFloat(source.value(sets.index, "rir"))
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Sets rir: %w", err)
		}
		pain, err := parseOptionalInteger(source.value(sets.index, "pain_0_10"))
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Sets pain_0_10: %w", err)
		}
		substitution, err := parseOptionalBool(source.value(sets.index, "is_substitution"))
		if err != nil {
			return stats.Dataset{}, fmt.Errorf("Sets is_substitution: %w", err)
		}
		logged, err := parseStatsTime(source.value(sets.index, "logged_at"), location)
		if err != nil || logged == nil {
			return stats.Dataset{}, fmt.Errorf("Sets logged_at is invalid")
		}
		dataset.Sets = append(dataset.Sets, stats.Set{
			SessionID: source.value(sets.index, "session_id"), ProgramVersionID: source.value(sets.index, "program_version_id"),
			SlotID: source.value(sets.index, "slot_id"), ExerciseID: source.value(sets.index, "exercise_id"),
			Number: number, LoadKG: load, Reps: reps, RIR: rir, Pain: pain, IsSubstitution: substitution, LoggedAt: *logged,
		})
	}
	return dataset, nil
}

func parseStatsTime(value string, location *time.Location) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return &parsed, nil
	}
	formats := []string{
		"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02",
		"02/01/2006 15:04:05", "02/01/2006 15:04", "02/01/2006",
		"1/2/2006 15:04:05", "1/2/2006 15:04", "1/2/2006",
	}
	for _, format := range formats {
		if parsed, err := time.ParseInLocation(format, value, location); err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("%q is not a supported timestamp", value)
}
