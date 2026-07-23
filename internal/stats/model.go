package stats

import "time"

type Dataset struct {
	ActiveProgramVersion string
	Demo                 bool
	Exercises            []Exercise
	Prescriptions        []Prescription
	Sessions             []Session
	Sets                 []Set
}

type Exercise struct {
	ID               string
	Name             string
	Aliases          []string
	LoadMode         string
	VolumeMultiplier float64
	Active           bool
}

type Prescription struct {
	VersionID       string
	Workout         string
	Position        int
	SlotID          string
	ExerciseID      string
	PlannedSets     int
	RepMin          *int
	RepMax          *int
	RIRMin          *float64
	RIRMax          *float64
	SuggestedLoadKG *float64
	LoadGuidance    string
}

type Session struct {
	ID               string
	ProgramVersionID string
	RotationNumber   int
	Workout          string
	StartedAt        time.Time
	FinishedAt       *time.Time
	Status           string
}

type Set struct {
	SessionID        string
	ProgramVersionID string
	SlotID           string
	ExerciseID       string
	Number           int
	LoadKG           *float64
	Reps             int
	RIR              *float64
	Pain             *int
	IsSubstitution   bool
	LoggedAt         time.Time
}

type Report struct {
	Exercise      Exercise
	Current       []Prescription
	SessionCount  int
	SetCount      int
	TotalReps     int
	LoadVolumeKG  float64
	PainSetCount  int
	BestLoadKG    *float64
	BestEstimated *EstimatedSet
	BestSet       *Set
	History       []SessionPerformance
}

type EstimatedSet struct {
	Set
	EstimatedLoadKG float64
}

type SessionPerformance struct {
	Session Session
	Sets    []Set
	Volume  float64
}
