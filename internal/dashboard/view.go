package dashboard

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/eltonsst/tren/internal/stats"
)

type pageView struct {
	Title            string
	Demo             bool
	Version          string
	Query            string
	CurrentExercises []exerciseView
	OtherExercises   []exerciseView
	Selected         exerciseDetailView
}

type exerciseView struct {
	Name      string
	Workouts  string
	Href      string
	Selected  bool
	HasRecord bool
}

type exerciseDetailView struct {
	Name          string
	ID            string
	LoadMode      string
	Prescriptions []prescriptionView
	Metrics       []metricView
	Chart         *chartView
	History       []historyView
	HasHistory    bool
}

type prescriptionView struct {
	Slot   string
	Detail string
	Load   string
}

type metricView struct {
	Label string
	Value string
	Tone  string
}

type historyView struct {
	Date    string
	Workout string
	Status  string
	Sets    []string
}

func buildPage(dataset stats.Dataset, selectedID, query string) (pageView, error) {
	exercises := stats.Catalogue(dataset)
	if selectedID == "" && len(exercises) > 0 {
		selectedID = exercises[0].ID
	}
	report, err := stats.BuildReport(dataset, selectedID)
	if err != nil {
		return pageView{}, err
	}
	workouts := make(map[string][]string)
	for _, prescription := range dataset.Prescriptions {
		if prescription.VersionID == dataset.ActiveProgramVersion {
			workouts[prescription.ExerciseID] = appendUnique(workouts[prescription.ExerciseID], prescription.Workout)
		}
	}
	normalizedQuery := normalize(query)
	page := pageView{
		Title: report.Exercise.Name + " · Tren", Demo: dataset.Demo,
		Version: dataset.ActiveProgramVersion, Query: query,
		Selected: buildExerciseDetail(report),
	}
	for _, exercise := range exercises {
		if normalizedQuery != "" && !exerciseMatches(exercise, normalizedQuery) {
			continue
		}
		item := exerciseView{
			Name: exercise.Name, Workouts: strings.Join(workouts[exercise.ID], "/"),
			Href: "/exercise/" + url.PathEscape(exercise.ID), Selected: exercise.ID == report.Exercise.ID,
		}
		item.HasRecord = hasExerciseSets(dataset, exercise.ID)
		if item.Workouts != "" {
			page.CurrentExercises = append(page.CurrentExercises, item)
		} else {
			page.OtherExercises = append(page.OtherExercises, item)
		}
	}
	return page, nil
}

func buildExerciseDetail(report stats.Report) exerciseDetailView {
	detail := exerciseDetailView{
		Name: report.Exercise.Name, ID: report.Exercise.ID,
		LoadMode:   strings.ReplaceAll(report.Exercise.LoadMode, "_", " "),
		HasHistory: len(report.History) > 0,
	}
	for _, prescription := range report.Current {
		detail.Prescriptions = append(detail.Prescriptions, prescriptionView{
			Slot:   fmt.Sprintf("%s%d", prescription.Workout, prescription.Position),
			Detail: fmt.Sprintf("%d × %s%s", prescription.PlannedSets, intRange(prescription.RepMin, prescription.RepMax), rirText(prescription.RIRMin, prescription.RIRMax)),
			Load:   prescriptionLoad(prescription),
		})
	}
	detail.Metrics = []metricView{
		{Label: "Sessions", Value: strconv.Itoa(report.SessionCount), Tone: "blue"},
		{Label: "Working sets", Value: strconv.Itoa(report.SetCount), Tone: "blue"},
		{Label: "Total reps", Value: strconv.Itoa(report.TotalReps), Tone: "green"},
		{Label: "Pain sets", Value: strconv.Itoa(report.PainSetCount), Tone: painTone(report.PainSetCount)},
		{Label: "Load volume", Value: optionalNumber(report.LoadVolumeKG, " kg"), Tone: "green"},
		{Label: "Top load", Value: pointerNumber(report.BestLoadKG, " kg"), Tone: "yellow"},
		{Label: "Best e1RM", Value: estimatedNumber(report.BestEstimated), Tone: "pink"},
	}
	if chart, ok := buildChart(report); ok {
		detail.Chart = &chart
	}
	limit := len(report.History)
	if limit > 10 {
		limit = 10
	}
	for _, performance := range report.History[:limit] {
		row := historyView{
			Date:    performance.Session.StartedAt.Format("02 Jan 2006"),
			Workout: "Workout " + fallback(performance.Session.Workout, "?"),
			Status:  fallback(performance.Session.Status, "unknown"),
		}
		for _, set := range performance.Sets {
			row.Sets = append(row.Sets, formatSet(set))
		}
		detail.History = append(detail.History, row)
	}
	return detail
}

func exerciseMatches(exercise stats.Exercise, query string) bool {
	values := append([]string{exercise.Name, exercise.ID}, exercise.Aliases...)
	for _, value := range values {
		if strings.Contains(normalize(value), query) {
			return true
		}
	}
	return false
}

func hasExerciseSets(dataset stats.Dataset, exerciseID string) bool {
	for _, set := range dataset.Sets {
		if set.ExerciseID == exerciseID {
			return true
		}
	}
	return false
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("_", " ", "-", " ").Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func intRange(minimum, maximum *int) string {
	if minimum == nil && maximum == nil {
		return "reps"
	}
	if minimum == nil {
		return strconv.Itoa(*maximum)
	}
	if maximum == nil || *minimum == *maximum {
		return strconv.Itoa(*minimum)
	}
	return fmt.Sprintf("%d–%d", *minimum, *maximum)
}

func rirText(minimum, maximum *float64) string {
	if minimum == nil && maximum == nil {
		return ""
	}
	if minimum == nil {
		return " · @" + number(*maximum) + " RIR"
	}
	if maximum == nil || *minimum == *maximum {
		return " · @" + number(*minimum) + " RIR"
	}
	return " · @" + number(*minimum) + "–" + number(*maximum) + " RIR"
}

func prescriptionLoad(value stats.Prescription) string {
	if value.SuggestedLoadKG != nil {
		return number(*value.SuggestedLoadKG) + " kg"
	}
	return strings.TrimSpace(value.LoadGuidance)
}

func optionalNumber(value float64, suffix string) string {
	if value <= 0 {
		return "—"
	}
	return number(value) + suffix
}

func pointerNumber(value *float64, suffix string) string {
	if value == nil {
		return "—"
	}
	return number(*value) + suffix
}

func estimatedNumber(value *stats.EstimatedSet) string {
	if value == nil {
		return "—"
	}
	return number(value.EstimatedLoadKG) + " kg"
}

func number(value float64) string {
	return strings.TrimSuffix(strconv.FormatFloat(value, 'f', 1, 64), ".0")
}

func painTone(count int) string {
	if count > 0 {
		return "pink"
	}
	return "green"
}

func formatSet(value stats.Set) string {
	result := ""
	if value.LoadKG != nil {
		result += number(*value.LoadKG) + " × "
	}
	result += strconv.Itoa(value.Reps)
	if value.RIR != nil {
		result += " @" + number(*value.RIR)
	}
	if value.Pain != nil && *value.Pain > 0 {
		result += " · pain " + strconv.Itoa(*value.Pain)
	}
	return result
}

func fallback(value, replacement string) string {
	if strings.TrimSpace(value) == "" {
		return replacement
	}
	return value
}
