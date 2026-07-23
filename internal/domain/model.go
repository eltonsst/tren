package domain

import "time"

type WorkoutCode string

const (
	WorkoutA WorkoutCode = "A"
	WorkoutB WorkoutCode = "B"
	WorkoutC WorkoutCode = "C"
	WorkoutD WorkoutCode = "D"
)

func (w WorkoutCode) Valid() bool {
	switch w {
	case WorkoutA, WorkoutB, WorkoutC, WorkoutD:
		return true
	default:
		return false
	}
}

func (w WorkoutCode) Next() (WorkoutCode, bool) {
	switch w {
	case WorkoutA:
		return WorkoutB, false
	case WorkoutB:
		return WorkoutC, false
	case WorkoutC:
		return WorkoutD, false
	case WorkoutD:
		return WorkoutA, true
	default:
		return "", false
	}
}

type ProgramVersionStatus string

const (
	ProgramDraft     ProgramVersionStatus = "draft"
	ProgramActive    ProgramVersionStatus = "active"
	ProgramCompleted ProgramVersionStatus = "completed"
	ProgramArchived  ProgramVersionStatus = "archived"
)

type ProgramVersion struct {
	ID              string
	BlockID         string
	RotationNumber  int
	Status          ProgramVersionStatus
	CreatedAt       time.Time
	ActivatedAt     *time.Time
	LockedAt        *time.Time
	CompletedAt     *time.Time
	SourceVersionID string
	CreatedBy       string
	Notes           string
}

type ProgressionRule string

const (
	ProgressAllSetsTop ProgressionRule = "all_sets_top"
	ProgressDouble     ProgressionRule = "double_progression"
	ProgressMakeHarder ProgressionRule = "make_harder"
	ProgressNone       ProgressionRule = "none"
)

type Prescription struct {
	VersionID        string
	Workout          WorkoutCode
	Position         int
	SlotID           string
	ExerciseID       string
	PlannedSets      int
	TargetSets       int
	RepMin           *int
	RepMax           *int
	PlannedRIRMin    *float64
	PlannedRIRMax    *float64
	TargetRIRMin     *float64
	TargetRIRMax     *float64
	SuggestedLoadKG  *float64
	LoadGuidance     string
	ProgressionRule  ProgressionRule
	IncrementKG      *float64
	HomeAlternatives []string
	GymAlternatives  []string
	Notes            string
}

type SessionStatus string

const (
	SessionOpen      SessionStatus = "open"
	SessionCompleted SessionStatus = "completed"
	SessionAbandoned SessionStatus = "abandoned"
)

type Session struct {
	ID               string
	ProgramVersionID string
	RotationNumber   int
	Workout          WorkoutCode
	StartedAt        time.Time
	FinishedAt       *time.Time
	Status           SessionStatus
	TelegramChatID   int64
	StartMessageID   int
	Notes            string
}

type WorkingSet struct {
	ID                string
	SessionID         string
	ProgramVersionID  string
	SlotID            string
	ExerciseID        string
	Number            int
	LoadKG            *float64
	Reps              int
	RIR               *float64
	Pain              *int
	IsSubstitution    bool
	TelegramMessageID int
	LoggedAt          time.Time
	Notes             string
}
