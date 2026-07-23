package stats

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func FindExercise(dataset Dataset, query string) (Exercise, error) {
	query = normalize(query)
	if query == "" {
		return Exercise{}, fmt.Errorf("exercise query is blank")
	}
	var exact []Exercise
	var partial []Exercise
	for _, exercise := range dataset.Exercises {
		values := append([]string{exercise.ID, exercise.Name}, exercise.Aliases...)
		matchedExact := false
		matchedPartial := false
		for _, value := range values {
			normalized := normalize(value)
			if normalized == query {
				matchedExact = true
			}
			if strings.Contains(normalized, query) {
				matchedPartial = true
			}
		}
		if matchedExact {
			exact = append(exact, exercise)
		} else if matchedPartial {
			partial = append(partial, exercise)
		}
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) > 1 {
		return Exercise{}, ambiguousError(query, exact)
	}
	if len(partial) == 1 {
		return partial[0], nil
	}
	if len(partial) > 1 {
		return Exercise{}, ambiguousError(query, partial)
	}
	return Exercise{}, fmt.Errorf("no exercise matches %q; run `tren stats` to list available exercises", query)
}

func BuildReport(dataset Dataset, query string) (Report, error) {
	exercise, err := FindExercise(dataset, query)
	if err != nil {
		return Report{}, err
	}
	report := Report{Exercise: exercise}
	for _, prescription := range dataset.Prescriptions {
		if prescription.VersionID == dataset.ActiveProgramVersion && prescription.ExerciseID == exercise.ID {
			report.Current = append(report.Current, prescription)
		}
	}
	sort.Slice(report.Current, func(i, j int) bool {
		if report.Current[i].Workout == report.Current[j].Workout {
			return report.Current[i].Position < report.Current[j].Position
		}
		return report.Current[i].Workout < report.Current[j].Workout
	})

	sessionsByID := make(map[string]Session, len(dataset.Sessions))
	for _, session := range dataset.Sessions {
		sessionsByID[session.ID] = session
	}
	historyByID := make(map[string]*SessionPerformance)
	for _, set := range dataset.Sets {
		if set.ExerciseID != exercise.ID {
			continue
		}
		report.SetCount++
		report.TotalReps += set.Reps
		if set.Pain != nil && *set.Pain > 0 {
			report.PainSetCount++
		}
		if set.LoadKG != nil {
			volume := *set.LoadKG * float64(set.Reps) * exercise.VolumeMultiplier
			report.LoadVolumeKG += volume
			if report.BestLoadKG == nil || *set.LoadKG > *report.BestLoadKG {
				value := *set.LoadKG
				report.BestLoadKG = &value
			}
			if estimate, ok := estimatedLoad(set); ok && (report.BestEstimated == nil || estimate > report.BestEstimated.EstimatedLoadKG) {
				copy := set
				report.BestEstimated = &EstimatedSet{Set: copy, EstimatedLoadKG: estimate}
			}
		}
		if betterSet(set, report.BestSet) {
			copy := set
			report.BestSet = &copy
		}
		performance := historyByID[set.SessionID]
		if performance == nil {
			session, found := sessionsByID[set.SessionID]
			if !found {
				session = Session{ID: set.SessionID, ProgramVersionID: set.ProgramVersionID, StartedAt: set.LoggedAt, Status: "unknown"}
			}
			performance = &SessionPerformance{Session: session}
			historyByID[set.SessionID] = performance
		}
		performance.Sets = append(performance.Sets, set)
		if set.LoadKG != nil {
			performance.Volume += *set.LoadKG * float64(set.Reps) * exercise.VolumeMultiplier
		}
	}
	for _, performance := range historyByID {
		sort.Slice(performance.Sets, func(i, j int) bool { return performance.Sets[i].Number < performance.Sets[j].Number })
		report.History = append(report.History, *performance)
	}
	sort.Slice(report.History, func(i, j int) bool {
		return report.History[i].Session.StartedAt.After(report.History[j].Session.StartedAt)
	})
	report.SessionCount = len(report.History)
	return report, nil
}

func Catalogue(dataset Dataset) []Exercise {
	current := make(map[string]bool)
	for _, prescription := range dataset.Prescriptions {
		if prescription.VersionID == dataset.ActiveProgramVersion {
			current[prescription.ExerciseID] = true
		}
	}
	result := make([]Exercise, 0, len(dataset.Exercises))
	for _, exercise := range dataset.Exercises {
		if exercise.Active || current[exercise.ID] {
			result = append(result, exercise)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		leftCurrent, rightCurrent := current[result[i].ID], current[result[j].ID]
		if leftCurrent != rightCurrent {
			return leftCurrent
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result
}

func estimatedLoad(set Set) (float64, bool) {
	if set.LoadKG == nil || set.Reps < 1 || set.Reps > 15 {
		return 0, false
	}
	return *set.LoadKG * (1 + float64(set.Reps)/30), true
}

func betterSet(candidate Set, current *Set) bool {
	if current == nil {
		return true
	}
	candidateLoad, currentLoad := -1.0, -1.0
	if candidate.LoadKG != nil {
		candidateLoad = *candidate.LoadKG
	}
	if current.LoadKG != nil {
		currentLoad = *current.LoadKG
	}
	if math.Abs(candidateLoad-currentLoad) > 0.0001 {
		return candidateLoad > currentLoad
	}
	return candidate.Reps > current.Reps
}

func normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("_", " ", "-", " ").Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func ambiguousError(query string, exercises []Exercise) error {
	names := make([]string, len(exercises))
	for index, exercise := range exercises {
		names[index] = exercise.Name + " (" + exercise.ID + ")"
	}
	sort.Strings(names)
	return fmt.Errorf("exercise query %q is ambiguous: %s", query, strings.Join(names, ", "))
}
