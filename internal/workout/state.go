package workout

import (
	"fmt"
	"time"

	"github.com/eltonsst/tren/internal/domain"
)

type State struct {
	NextWorkout    domain.WorkoutCode
	RotationNumber int
	OpenSession    *domain.Session
}

type FinishResult struct {
	Session           domain.Session
	NextWorkout       domain.WorkoutCode
	RotationCompleted bool
	NextRotation      int
}

func Start(state State, sessionID, versionID string, override *domain.WorkoutCode, now time.Time) (State, domain.Session, error) {
	if state.OpenSession != nil && state.OpenSession.Status == domain.SessionOpen {
		return state, domain.Session{}, fmt.Errorf("session %s is already open", state.OpenSession.ID)
	}
	workout := state.NextWorkout
	if override != nil {
		workout = *override
	}
	if !workout.Valid() {
		return state, domain.Session{}, fmt.Errorf("invalid workout %q", workout)
	}
	if state.RotationNumber < 1 {
		state.RotationNumber = 1
	}

	session := domain.Session{
		ID:               sessionID,
		ProgramVersionID: versionID,
		RotationNumber:   state.RotationNumber,
		Workout:          workout,
		StartedAt:        now,
		Status:           domain.SessionOpen,
	}
	state.OpenSession = &session
	return state, session, nil
}

func Resume(state State) (domain.Session, error) {
	if state.OpenSession == nil || state.OpenSession.Status != domain.SessionOpen {
		return domain.Session{}, fmt.Errorf("no open session")
	}
	return *state.OpenSession, nil
}

func Finish(state State, now time.Time) (State, FinishResult, error) {
	if state.OpenSession == nil || state.OpenSession.Status != domain.SessionOpen {
		return state, FinishResult{}, fmt.Errorf("no open session")
	}

	session := *state.OpenSession
	session.Status = domain.SessionCompleted
	session.FinishedAt = &now
	next, rotationCompleted := session.Workout.Next()
	if !next.Valid() {
		return state, FinishResult{}, fmt.Errorf("invalid workout %q", session.Workout)
	}

	nextRotation := state.RotationNumber
	if rotationCompleted {
		nextRotation++
	}
	state.OpenSession = nil
	state.NextWorkout = next
	state.RotationNumber = nextRotation

	return state, FinishResult{
		Session:           session,
		NextWorkout:       next,
		RotationCompleted: rotationCompleted,
		NextRotation:      nextRotation,
	}, nil
}
