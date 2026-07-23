package stats

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const demoRotations = 6

// OfflineDemo builds the complete demo catalogue, active program, and history
// without reading credentials or contacting Google Sheets.
func OfflineDemo(now time.Time) Dataset {
	return WithDemoHistory(demoProgram(), now)
}

func demoProgram() Dataset {
	dataset := Dataset{ActiveProgramVersion: "block1-r1"}
	dataset.Exercises = []Exercise{
		demoExercise("bench_press", "Bench press", "bench|bp|panca", "barbell_total", 1),
		demoExercise("weighted_pull_up", "Weighted pull-up", "weighted pullup|wpu|trazioni zavorrate", "external_added", 1),
		demoExercise("barbell_overhead_press", "Barbell overhead press", "ohp|overhead press|military press", "barbell_total", 1),
		demoExercise("strict_barbell_row", "Strict barbell row", "barbell row|row|rematore bilanciere", "barbell_total", 1),
		demoExercise("one_arm_lateral_raise", "One-arm lateral raise", "lateral raise|laterals|alzate laterali", "dumbbell_each", 2),
		demoExercise("barbell_curl", "Barbell curl", "curl|bb curl", "barbell_total", 1),
		demoExercise("back_squat", "Back squat", "squat|sq", "barbell_total", 1),
		demoExercise("romanian_deadlift", "Romanian deadlift", "rdl|romanian", "barbell_total", 1),
		demoExercise("bulgarian_split_squat", "Bulgarian split squat", "bulgarian|bss", "dumbbell_each", 2),
		demoExercise("close_grip_push_up", "Shoulder-width close-grip push-up", "close grip pushup|cg pushup|diamond pushup", "bodyweight", 1),
		demoExercise("band_pushdown", "Band pushdown", "band pushdown|pushdown band", "other", 1),
		demoExercise("cable_pushdown", "Cable pushdown", "pushdown|triceps pushdown|cable pushdown", "machine_stack", 1),
		demoExercise("weighted_sit_up", "Weighted sit-up", "situp|weighted situp", "external_added", 1),
		demoExercise("plank", "Plank", "plank", "duration", 1),
		demoExercise("paused_bench", "Paused bench press", "paused bench|pause bench", "barbell_total", 1),
		demoExercise("chin_up", "Bodyweight chin-up", "chinup|chin up|supinated pullup", "bodyweight", 1),
		demoExercise("inverted_row", "Inverted row from rack", "inverted row|australian row", "bodyweight_angle", 1),
		demoExercise("feet_elevated_push_up", "Feet-elevated push-up", "feet elevated pushup|decline pushup", "bodyweight", 1),
		demoExercise("concentration_curl", "Concentration curl", "concentration curl|conc curl", "dumbbell_each", 2),
		demoExercise("paused_squat", "Paused squat", "pause squat|paused squat", "barbell_total", 1),
		demoExercise("barbell_hip_thrust", "Barbell hip thrust", "hip thrust|bb hip thrust", "barbell_total", 1),
		demoExercise("pronated_pull_up", "Pronated pull-up", "pullup|pull up|pronated pullup", "bodyweight", 1),
		demoExercise("hammer_curl", "Hammer curl", "hammer curl|hammer", "dumbbell_each", 2),
		demoExercise("neutral_grip_one_arm_db_press", "Neutral-grip one-arm dumbbell press", "neutral db press|one arm db press", "dumbbell_each", 2),
		demoExercise("chest_supported_row", "Chest-supported row", "supported row|chest supported row", "machine_stack", 1),
		demoExercise("t_bar_row", "T-bar row", "tbar row|t bar row", "machine_stack", 1),
		demoExercise("seated_cable_row", "Seated cable row", "cable row|seated row", "machine_stack", 1),
		demoExercise("lat_pulldown", "Lat pulldown", "pulldown|lat pull down", "machine_stack", 1),
		demoExercise("cable_lateral_raise", "Cable lateral raise", "cable lateral|cable laterals", "machine_stack", 2),
		demoExercise("lateral_raise_machine", "Lateral-raise machine", "lateral machine|machine lateral", "machine_stack", 1),
	}

	dataset.Prescriptions = []Prescription{
		demoPrescription("A", 1, "bench_press", 3, 3, 5, 3, floatValue(72.5), ""),
		demoPrescription("A", 2, "weighted_pull_up", 3, 3, 5, 3, floatValue(5), "External added load"),
		demoPrescription("A", 3, "barbell_overhead_press", 2, 5, 8, 3, floatValue(35), ""),
		demoPrescription("A", 4, "strict_barbell_row", 2, 6, 10, 3, nil, "Select by RIR"),
		demoPrescription("A", 5, "one_arm_lateral_raise", 2, 12, 20, 2, nil, "Controlled"),
		demoPrescription("A", 6, "barbell_curl", 2, 8, 12, 2, nil, "Select by RIR"),
		demoPrescription("B", 1, "back_squat", 3, 3, 5, 3, floatValue(70), ""),
		demoPrescription("B", 2, "romanian_deadlift", 2, 6, 10, 3, nil, "Select by RIR"),
		demoPrescription("B", 3, "bulgarian_split_squat", 2, 8, 12, 2, nil, "One dumbbell"),
		demoPrescription("B", 4, "close_grip_push_up", 2, 10, 15, 2, nil, "Pain-free"),
		demoPrescription("B", 5, "weighted_sit_up", 2, 10, 15, 2, nil, "Controlled"),
		demoPrescription("C", 1, "paused_bench", 3, 6, 8, 3, floatValue(65), "One-second pause"),
		demoPrescription("C", 2, "chin_up", 3, 5, 8, 3, nil, "Bodyweight"),
		demoPrescription("C", 3, "inverted_row", 2, 8, 15, 3, nil, "Adjust foot position"),
		demoPrescription("C", 4, "feet_elevated_push_up", 2, 8, 15, 2, nil, "Pain-free"),
		demoPrescription("C", 5, "one_arm_lateral_raise", 2, 12, 20, 2, nil, "Controlled"),
		demoPrescription("C", 6, "concentration_curl", 2, 8, 12, 2, nil, "Approximately 14–16 kg"),
		demoPrescription("D", 1, "paused_squat", 2, 6, 8, 3, floatValue(57.5), "Two-second pause"),
		demoPrescription("D", 2, "barbell_hip_thrust", 2, 8, 12, 3, nil, "Select by RIR"),
		demoPrescription("D", 3, "pronated_pull_up", 2, 5, 8, 3, nil, "Bodyweight"),
		demoPrescription("D", 4, "one_arm_lateral_raise", 2, 15, 25, 2, nil, "Controlled"),
		demoPrescription("D", 5, "close_grip_push_up", 2, 10, 15, 2, nil, "Pain-free"),
		demoPrescription("D", 6, "hammer_curl", 2, 10, 15, 2, nil, "Select by RIR"),
	}
	return dataset
}

func demoExercise(id, name, aliasList, loadMode string, multiplier float64) Exercise {
	return Exercise{ID: id, Name: name, Aliases: strings.Split(aliasList, "|"), LoadMode: loadMode, VolumeMultiplier: multiplier, Active: true}
}

func demoPrescription(workout string, position int, exerciseID string, sets, repMin, repMax int, rir float64, load *float64, guidance string) Prescription {
	return Prescription{
		VersionID: "block1-r1", Workout: workout, Position: position,
		SlotID: fmt.Sprintf("%s%02d", workout, position), ExerciseID: exerciseID,
		PlannedSets: sets, RepMin: intValue(repMin), RepMax: intValue(repMax),
		RIRMin: floatValue(rir), RIRMax: floatValue(rir), SuggestedLoadKG: load, LoadGuidance: guidance,
	}
}

// WithDemoHistory returns a copy of the live catalogue and program populated
// with synthetic sessions and sets. It never writes demo rows to Google Sheets.
func WithDemoHistory(source Dataset, now time.Time) Dataset {
	dataset := source
	dataset.Demo = true
	dataset.Sessions = nil
	dataset.Sets = nil

	exercises := make(map[string]Exercise, len(dataset.Exercises))
	for _, exercise := range dataset.Exercises {
		exercises[exercise.ID] = exercise
	}

	workouts := []string{"A", "B", "C", "D"}
	start := time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location()).AddDate(0, 0, -((demoRotations-1)*8 + (len(workouts)-1)*2))
	for rotation := 1; rotation <= demoRotations; rotation++ {
		for workoutIndex, workout := range workouts {
			started := start.AddDate(0, 0, (rotation-1)*8+workoutIndex*2)
			finished := started.Add(75 * time.Minute)
			sessionID := fmt.Sprintf("demo-r%02d-%s", rotation, workout)
			dataset.Sessions = append(dataset.Sessions, Session{
				ID: sessionID, ProgramVersionID: dataset.ActiveProgramVersion,
				RotationNumber: rotation, Workout: workout, StartedAt: started,
				FinishedAt: &finished, Status: "completed",
			})

			for _, prescription := range dataset.Prescriptions {
				if prescription.VersionID != dataset.ActiveProgramVersion || prescription.Workout != workout {
					continue
				}
				exercise := exercises[prescription.ExerciseID]
				for setNumber := 1; setNumber <= prescription.PlannedSets; setNumber++ {
					loggedAt := started.Add(time.Duration(8+prescription.Position*9+setNumber*3) * time.Minute)
					pain := 0
					if (exercise.ID == "bench_press" && rotation == 2 && setNumber == prescription.PlannedSets) ||
						(exercise.ID == "barbell_overhead_press" && rotation == 3 && setNumber == prescription.PlannedSets) {
						pain = 1
					}
					dataset.Sets = append(dataset.Sets, Set{
						SessionID: sessionID, ProgramVersionID: dataset.ActiveProgramVersion,
						SlotID: prescription.SlotID, ExerciseID: exercise.ID, Number: setNumber,
						LoadKG: demoLoad(exercise, prescription, rotation),
						Reps:   demoReps(prescription, rotation, setNumber),
						RIR:    demoRIR(prescription, rotation), Pain: intValue(pain), LoggedAt: loggedAt,
					})
				}
			}
		}
	}
	return dataset
}

func demoLoad(exercise Exercise, prescription Prescription, rotation int) *float64 {
	latest := prescription.SuggestedLoadKG
	if latest == nil {
		fallback, found := map[string]float64{
			"strict_barbell_row": 52.5, "one_arm_lateral_raise": 10,
			"barbell_curl": 27.5, "concentration_curl": 12,
			"hammer_curl": 14, "bulgarian_split_squat": 30,
			"inverted_row": 0, "weighted_sit_up": 15,
			"romanian_deadlift": 70, "barbell_hip_thrust": 75,
		}[exercise.ID]
		if !found || fallback == 0 || strings.Contains(exercise.LoadMode, "bodyweight") {
			return nil
		}
		latest = floatValue(fallback)
	}

	step := 2.5
	if strings.Contains(exercise.LoadMode, "external") {
		step = math.Min(2.5, math.Max(0.5, *latest/5))
	} else if *latest >= 80 {
		step = 5
	} else if *latest < 20 {
		step = 1
	}
	load := *latest - step*float64(demoRotations-rotation)
	if load < 0 {
		load = 0
	}
	return floatValue(math.Round(load*2) / 2)
}

func demoReps(prescription Prescription, rotation, setNumber int) int {
	minimum, maximum := 8, 12
	if prescription.RepMin != nil {
		minimum = *prescription.RepMin
	}
	if prescription.RepMax != nil {
		maximum = *prescription.RepMax
	}
	if maximum < minimum {
		maximum = minimum
	}
	progress := int(math.Round(float64(maximum-minimum) * float64(rotation-1) / float64(demoRotations-1)))
	reps := minimum + progress - (setNumber-1)/2
	if reps < minimum {
		reps = minimum
	}
	return reps
}

func demoRIR(prescription Prescription, rotation int) *float64 {
	target := 2.0
	if prescription.RIRMax != nil {
		target = *prescription.RIRMax
	} else if prescription.RIRMin != nil {
		target = *prescription.RIRMin
	}
	if rotation <= 2 {
		target++
	}
	return floatValue(target)
}

func floatValue(value float64) *float64 { return &value }
func intValue(value int) *int           { return &value }
