package progression

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/eltonsst/tren/internal/domain"
)

type Decision struct {
	SlotID         string
	ExerciseID     string
	PreviousLoadKG *float64
	NextLoadKG     *float64
	LoadIncreased  bool
	Reason         string
}

type Result struct {
	Version       domain.ProgramVersion
	Prescriptions []domain.Prescription
	Decisions     []Decision
}

func NextVersion(current domain.ProgramVersion, prescriptions []domain.Prescription, sets []domain.WorkingSet, now time.Time) (Result, error) {
	if current.Status != domain.ProgramCompleted {
		return Result{}, fmt.Errorf("program version %s is not completed", current.ID)
	}
	if current.BlockID == "" || current.RotationNumber < 1 {
		return Result{}, fmt.Errorf("program version metadata is incomplete")
	}

	nextID := fmt.Sprintf("%s-r%d", current.BlockID, current.RotationNumber+1)
	activatedAt := now
	result := Result{
		Version: domain.ProgramVersion{
			ID:              nextID,
			BlockID:         current.BlockID,
			RotationNumber:  current.RotationNumber + 1,
			Status:          domain.ProgramActive,
			CreatedAt:       now,
			ActivatedAt:     &activatedAt,
			SourceVersionID: current.ID,
			CreatedBy:       "progression_engine",
		},
	}

	setsBySlot := make(map[string][]domain.WorkingSet)
	for _, set := range sets {
		if set.ProgramVersionID == current.ID {
			setsBySlot[set.SlotID] = append(setsBySlot[set.SlotID], set)
		}
	}
	for slotID := range setsBySlot {
		sort.Slice(setsBySlot[slotID], func(i, j int) bool {
			return setsBySlot[slotID][i].Number < setsBySlot[slotID][j].Number
		})
	}

	for _, prescription := range prescriptions {
		if prescription.VersionID != current.ID {
			continue
		}
		next := clonePrescription(prescription)
		next.VersionID = nextID
		next.PlannedSets = prescription.TargetSets
		next.PlannedRIRMin = cloneFloat(prescription.TargetRIRMin)
		next.PlannedRIRMax = cloneFloat(prescription.TargetRIRMax)

		decision := decide(prescription, setsBySlot[prescription.SlotID])
		next.SuggestedLoadKG = cloneFloat(decision.NextLoadKG)
		result.Prescriptions = append(result.Prescriptions, next)
		result.Decisions = append(result.Decisions, decision)
	}

	if len(result.Prescriptions) == 0 {
		return Result{}, fmt.Errorf("program version %s has no prescriptions", current.ID)
	}
	return result, nil
}

func decide(prescription domain.Prescription, sets []domain.WorkingSet) Decision {
	baseLoad := cloneFloat(prescription.SuggestedLoadKG)
	if baseLoad == nil {
		baseLoad = consistentLoad(sets)
	}
	decision := Decision{
		SlotID:         prescription.SlotID,
		ExerciseID:     prescription.ExerciseID,
		PreviousLoadKG: cloneFloat(baseLoad),
		NextLoadKG:     cloneFloat(baseLoad),
	}

	if prescription.ProgressionRule == domain.ProgressNone {
		decision.Reason = "progression is disabled"
		return decision
	}
	if prescription.PlannedSets < prescription.TargetSets {
		decision.Reason = "temporary reduced volume; restored target sets without increasing load"
		return decision
	}
	if prescription.RepMax == nil {
		decision.Reason = "no upper repetition target"
		return decision
	}
	if len(sets) < prescription.TargetSets {
		decision.Reason = fmt.Sprintf("completed %d of %d target sets", len(sets), prescription.TargetSets)
		return decision
	}

	relevant := sets[:prescription.TargetSets]
	for _, set := range relevant {
		if set.IsSubstitution || set.ExerciseID != prescription.ExerciseID {
			decision.Reason = "a substitution was performed; default exercise load was preserved"
			return decision
		}
		if set.Reps < *prescription.RepMax {
			decision.Reason = "not every set reached the top of the repetition range"
			return decision
		}
		if prescription.TargetRIRMin != nil && (set.RIR == nil || *set.RIR < *prescription.TargetRIRMin) {
			decision.Reason = "target repetitions were not achieved at the prescribed RIR"
			return decision
		}
		if set.Pain != nil && *set.Pain > 0 {
			decision.Reason = "pain was recorded; automatic progression was held"
			return decision
		}
	}

	switch prescription.ProgressionRule {
	case domain.ProgressMakeHarder:
		decision.Reason = "top of range achieved; make the bodyweight variation harder manually"
		return decision
	case domain.ProgressAllSetsTop, domain.ProgressDouble:
		if baseLoad == nil {
			decision.Reason = "top of range achieved but no consistent load was available"
			return decision
		}
		if prescription.IncrementKG == nil || *prescription.IncrementKG <= 0 {
			decision.Reason = "top of range achieved; choose the next practical load manually"
			return decision
		}
		next := *baseLoad + *prescription.IncrementKG
		decision.NextLoadKG = &next
		decision.LoadIncreased = true
		decision.Reason = fmt.Sprintf("all target sets reached %d reps at the prescribed RIR", *prescription.RepMax)
		return decision
	default:
		decision.Reason = fmt.Sprintf("unknown progression rule %q", prescription.ProgressionRule)
		return decision
	}
}

func consistentLoad(sets []domain.WorkingSet) *float64 {
	if len(sets) == 0 || sets[0].LoadKG == nil {
		return nil
	}
	load := *sets[0].LoadKG
	for _, set := range sets[1:] {
		if set.LoadKG == nil || math.Abs(*set.LoadKG-load) > 0.0001 {
			return nil
		}
	}
	return &load
}

func clonePrescription(value domain.Prescription) domain.Prescription {
	copy := value
	copy.RepMin = cloneInt(value.RepMin)
	copy.RepMax = cloneInt(value.RepMax)
	copy.PlannedRIRMin = cloneFloat(value.PlannedRIRMin)
	copy.PlannedRIRMax = cloneFloat(value.PlannedRIRMax)
	copy.TargetRIRMin = cloneFloat(value.TargetRIRMin)
	copy.TargetRIRMax = cloneFloat(value.TargetRIRMax)
	copy.SuggestedLoadKG = cloneFloat(value.SuggestedLoadKG)
	copy.IncrementKG = cloneFloat(value.IncrementKG)
	copy.HomeAlternatives = append([]string(nil), value.HomeAlternatives...)
	copy.GymAlternatives = append([]string(nil), value.GymAlternatives...)
	return copy
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
