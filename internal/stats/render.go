package stats

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func RenderCatalogue(dataset Dataset) string {
	exercises := Catalogue(dataset)
	current := make(map[string][]string)
	for _, prescription := range dataset.Prescriptions {
		if prescription.VersionID == dataset.ActiveProgramVersion {
			current[prescription.ExerciseID] = append(current[prescription.ExerciseID], prescription.Workout)
		}
	}
	nameWidth, idWidth := 0, 0
	for _, exercise := range exercises {
		if len(exercise.Name) > nameWidth {
			nameWidth = len(exercise.Name)
		}
		if len(exercise.ID) > idWidth {
			idWidth = len(exercise.ID)
		}
	}
	var output strings.Builder
	fmt.Fprintf(&output, "Exercise statistics · %s\n\n", dataset.ActiveProgramVersion)
	output.WriteString("Use `tren stats <name or alias>` for a detailed report.\n\n")
	output.WriteString("CURRENT PROGRAM\n")
	for _, exercise := range exercises {
		workouts := current[exercise.ID]
		if len(workouts) == 0 {
			continue
		}
		fmt.Fprintf(&output, "  %-*s  %-*s  %s\n", nameWidth, exercise.Name, idWidth, exercise.ID, strings.Join(workouts, "/"))
	}
	output.WriteString("\nOTHER ACTIVE EXERCISES\n")
	for _, exercise := range exercises {
		if len(current[exercise.ID]) != 0 {
			continue
		}
		fmt.Fprintf(&output, "  %-*s  %s\n", nameWidth, exercise.Name, exercise.ID)
	}
	return strings.TrimRight(output.String(), "\n") + "\n"
}

func RenderReport(dataset Dataset, report Report) string {
	var output strings.Builder
	fmt.Fprintf(&output, "%s\n%s · %s\n", strings.ToUpper(report.Exercise.Name), report.Exercise.ID, humanize(report.Exercise.LoadMode))

	output.WriteString("\nCURRENT PRESCRIPTION")
	if dataset.ActiveProgramVersion != "" {
		fmt.Fprintf(&output, " · %s", dataset.ActiveProgramVersion)
	}
	output.WriteString("\n")
	if len(report.Current) == 0 {
		output.WriteString("  Not present in the active program.\n")
	} else {
		for _, prescription := range report.Current {
			fmt.Fprintf(&output, "  %s%d  %s", prescription.Workout, prescription.Position, formatPrescription(prescription))
			if load := formatLoad(prescription.SuggestedLoadKG, prescription.LoadGuidance); load != "" {
				fmt.Fprintf(&output, " · %s", load)
			}
			output.WriteString("\n")
		}
	}

	output.WriteString("\nSUMMARY\n")
	fmt.Fprintf(&output, "  Sessions       %d\n", report.SessionCount)
	fmt.Fprintf(&output, "  Working sets   %d\n", report.SetCount)
	fmt.Fprintf(&output, "  Total reps     %d\n", report.TotalReps)
	if report.LoadVolumeKG > 0 {
		fmt.Fprintf(&output, "  Load volume    %s kg\n", formatNumber(report.LoadVolumeKG))
	}
	if report.BestLoadKG != nil {
		fmt.Fprintf(&output, "  Highest load   %s kg\n", formatNumber(*report.BestLoadKG))
	}
	if report.BestEstimated != nil {
		fmt.Fprintf(&output, "  Best e1RM      %s kg (%s)\n", formatNumber(report.BestEstimated.EstimatedLoadKG), formatSet(report.BestEstimated.Set))
	}
	fmt.Fprintf(&output, "  Pain sets      %d\n", report.PainSetCount)

	output.WriteString("\nHISTORY\n")
	if len(report.History) == 0 {
		output.WriteString("  No logged working sets yet.\n")
		return output.String()
	}
	limit := len(report.History)
	if limit > 10 {
		limit = 10
	}
	for _, performance := range report.History[:limit] {
		fmt.Fprintf(&output, "  %s · Workout %s · %s\n", formatDate(performance.Session.StartedAt), fallback(performance.Session.Workout, "?"), fallback(performance.Session.Status, "unknown"))
		output.WriteString("    ")
		for index, set := range performance.Sets {
			if index > 0 {
				output.WriteString("  |  ")
			}
			output.WriteString(formatSet(set))
		}
		output.WriteString("\n")
	}
	return output.String()
}

func formatPrescription(value Prescription) string {
	reps := formatRange(value.RepMin, value.RepMax, "reps")
	rir := formatFloatRange(value.RIRMin, value.RIRMax)
	result := fmt.Sprintf("%d×%s", value.PlannedSets, reps)
	if rir != "" {
		result += " @" + rir + " RIR"
	}
	return result
}

func formatSet(value Set) string {
	result := ""
	if value.LoadKG != nil {
		result = formatNumber(*value.LoadKG) + "×"
	}
	result += strconv.Itoa(value.Reps)
	if value.RIR != nil {
		result += " @" + formatNumber(*value.RIR)
	}
	if value.Pain != nil {
		result += " p" + strconv.Itoa(*value.Pain)
	}
	if value.IsSubstitution {
		result += " sub"
	}
	return result
}

func formatLoad(load *float64, guidance string) string {
	if load != nil {
		return formatNumber(*load) + " kg"
	}
	return strings.TrimSpace(guidance)
}

func formatRange(minimum, maximum *int, fallback string) string {
	if minimum == nil && maximum == nil {
		return fallback
	}
	if minimum == nil {
		return strconv.Itoa(*maximum)
	}
	if maximum == nil || *minimum == *maximum {
		return strconv.Itoa(*minimum)
	}
	return fmt.Sprintf("%d–%d", *minimum, *maximum)
}

func formatFloatRange(minimum, maximum *float64) string {
	if minimum == nil && maximum == nil {
		return ""
	}
	if minimum == nil {
		return formatNumber(*maximum)
	}
	if maximum == nil || *minimum == *maximum {
		return formatNumber(*minimum)
	}
	return formatNumber(*minimum) + "–" + formatNumber(*maximum)
}

func formatNumber(value float64) string {
	return strings.TrimSuffix(strconv.FormatFloat(value, 'f', 1, 64), ".0")
}

func formatDate(value time.Time) string {
	if value.IsZero() {
		return "unknown date"
	}
	return value.Format("2006-01-02")
}

func humanize(value string) string {
	return strings.ReplaceAll(value, "_", " ")
}

func fallback(value, replacement string) string {
	if strings.TrimSpace(value) == "" {
		return replacement
	}
	return value
}
